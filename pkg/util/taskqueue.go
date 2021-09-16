package util

import (
	"fmt"

	"github.com/sirupsen/logrus"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

// TaskQueue holds all the information needed to create a task queue.
type TaskQueue struct {
	controllerName string
	logger         *logrus.Entry

	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	Workqueue   workqueue.RateLimitingInterface
	syncHandler func(string) error
}

// NewTaskQueue returns a new TaskQueue
func NewTaskQueue(workqueue workqueue.RateLimitingInterface, syncHandler func(string) error, controllerName string, logger *logrus.Entry) *TaskQueue {
	return &TaskQueue{logger: logger,
		Workqueue:      workqueue,
		syncHandler:    syncHandler,
		controllerName: controllerName,
	}
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (t *TaskQueue) RunWorker() {
	for t.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (t *TaskQueue) processNextWorkItem() bool {
	obj, shutdown := t.Workqueue.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer t.Workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer t.Workqueue.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			t.Workqueue.Forget(obj)
			runtime.HandleError(fmt.Errorf("%q controller expected string in workqueue but got %#v", t.controllerName, obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		if err := t.syncHandler(key); err != nil {
			if !kerrors.IsNotFound(err) {
				// Put the item back on the workqueue to handle any transient errors.
				t.Workqueue.AddRateLimited(key)
			}
			return fmt.Errorf("%q controller error syncing '%s': %s, requeuing", t.controllerName, key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		t.Workqueue.Forget(obj)
		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
		return true
	}

	return true
}

// Enqueue takes a resource and converts it into a namespace/name
// string which is then put onto the work queue.
func (t *TaskQueue) Enqueue(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}
	t.Workqueue.Add(key)
}
