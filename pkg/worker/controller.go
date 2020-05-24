package worker

import (
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
	"k8s.io/client-go/util/workqueue"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"

	calculationsv1 "github.com/vega-project/ccb-operator/pkg/apis/calculations/v1"
	calculationsclient "github.com/vega-project/ccb-operator/pkg/client/clientset/versioned"
	informers "github.com/vega-project/ccb-operator/pkg/client/informers/externalversions/calculations/v1"
	listers "github.com/vega-project/ccb-operator/pkg/client/listers/calculations/v1"
	"github.com/vega-project/ccb-operator/pkg/util"
	"github.com/vega-project/ccb-operator/pkg/worker/executor"
)

const (
	controllerName = "Calculations"
)

type Controller struct {
	calculationLister    listers.CalculationLister
	calculationClientSet calculationsclient.Interface
	logger               *logrus.Entry
	calculationsSynced   cache.InformerSynced
	taskQueue            *util.TaskQueue
	executeChan          chan *calculationsv1.Calculation
	stepUpdaterChan      chan executor.Result
	hostname             string
}

func NewController(calculationClientSet calculationsclient.Interface, calculationInformer informers.CalculationInformer, executeChan chan *calculationsv1.Calculation, stepUpdaterChan chan executor.Result, hostname string) *Controller {
	logger := logrus.WithField("controller", "calculations")
	logger.Level = logrus.DebugLevel
	controller := &Controller{
		calculationLister:    calculationInformer.Lister(),
		calculationsSynced:   calculationInformer.Informer().HasSynced,
		calculationClientSet: calculationClientSet,
		logger:               logger,
		executeChan:          executeChan,
		stepUpdaterChan:      stepUpdaterChan,
		hostname:             hostname,
	}

	controller.taskQueue = util.NewTaskQueue(
		workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), controllerName),
		controller.syncHandler, controllerName, logger)

	logger.Info("Setting up the Calculations event handlers")
	calculationInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			mObj := obj.(*calculationsv1.Calculation)
			controller.taskQueue.Enqueue(mObj)
		},
		UpdateFunc: func(old, changed interface{}) {
			if !equality.Semantic.DeepEqual(old, changed) {
				controller.logger.Debugf("Updating object: %q", diff.ObjectReflectDiff(old, changed))
			}
			controller.taskQueue.Enqueue(changed)
		},
		DeleteFunc: func(obj interface{}) {
			calc := obj.(*calculationsv1.Calculation)
			controller.logger.WithField("calculation", calc.Name).Warn("Deleting...")
			controller.taskQueue.Enqueue(obj)
		},
	})

	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.

// TODO add waitgroup
func (c *Controller) Run(stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()
	defer c.taskQueue.Workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	c.logger.Info("Starting Calculations controller")

	// Wait for the caches to be synced before starting workers
	c.logger.Info("Waiting for calculations informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.calculationsSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	c.logger.Info("Starting calculation worker")
	go wait.Until(c.taskQueue.RunWorker, time.Second, stopCh)

	c.logger.Info("Starting calculation results updater")
	go c.resultUpdater(stopCh)

	<-stopCh
	return nil
}

func (c *Controller) syncHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the calculation resource with this namespace/name
	calculation, err := c.calculationLister.Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			fmt.Errorf("calculation '%s' in work queue no longer exists", key)
			return nil
		}
		return err
	}

	if calculation.Assign == c.hostname {
		switch calculation.Phase {
		case calculationsv1.CreatedPhase:
			c.logger.WithField("calculation", calculation.Name).Info("Processing assigned calculation")
			calculation.Phase = calculationsv1.ProcessingPhase
			calculation.Status.PendingTime = &metav1.Time{Time: time.Now()}

			c.logger.Info("Sent for execution")
			c.executeChan <- calculation

			if err := retry.RetryOnConflict(retry.DefaultRetry, func() (err error) {
				_, err = c.calculationClientSet.CalculationsV1().Calculations().Update(calculation)
				return err
			}); err != nil {
				c.logger.WithField("calculation", calculation.Name).WithError(err).Error("Couldn't update calculation.")
			}
		case calculationsv1.ProcessingPhase:
			if isFinishedCalculation(calculation.Spec.Steps) {
				c.updateCalculationPhase(calculation, getCalculationFinalPhase(calculation.Spec.Steps))
			}
		}
	}

	return nil
}

func isFinishedCalculation(steps []calculationsv1.Step) bool {
	for _, step := range steps {
		if step.Status == "" {
			return false
		}
	}
	return true
}

func hasFailedStep(steps []calculationsv1.Step) bool {
	for _, step := range steps {
		if step.Status == "Failed" {
			return true
		}
	}
	return false
}

func getCalculationFinalPhase(steps []calculationsv1.Step) calculationsv1.CalculationPhase {
	if hasFailedStep(steps) {
		return calculationsv1.FailedPhase
	}
	return calculationsv1.CompletedPhase
}

func (c *Controller) updateCalculationPhase(calc *calculationsv1.Calculation, phase calculationsv1.CalculationPhase) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		newCalc, err := c.calculationClientSet.CalculationsV1().Calculations().Get(calc.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		newCalc.Phase = phase
		newCalc.Status.CompletionTime = &metav1.Time{Time: time.Now()}

		_, err = c.calculationClientSet.CalculationsV1().Calculations().Update(newCalc)
		return err
	})
}

func (c *Controller) resultUpdater(stopCh <-chan struct{}) {
	for {
		select {
		case stepResult := <-c.stepUpdaterChan:
			c.logger.Info("Updating calculation")
			if err := c.updateCalculation(stepResult); err != nil {
				c.logger.WithError(err).Error("Couldn't update calculation's results.")
			}
		case <-stopCh:
			c.logger.Info("Stopping resultUpdater")
			break
		}
	}
}

func (c *Controller) updateCalculation(r executor.Result) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {

		// We need to Get the calculcation again because there is a chance that it will be changed in the cluster.
		newCalc, err := c.calculationClientSet.CalculationsV1().Calculations().Get(r.CalcName, metav1.GetOptions{})
		if err != nil {
			return err
		}

		newCalc.Spec.Steps[r.Step].Status = r.Status // TODO add the rest of the resutls here

		_, err = c.calculationClientSet.CalculationsV1().Calculations().Update(newCalc)
		return err
	})
}
