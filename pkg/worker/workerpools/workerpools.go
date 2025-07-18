package workerpools

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"k8s.io/client-go/util/workqueue"
	ctrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	workersv1 "github.com/vega-project/ccb-operator/pkg/apis/workers/v1"
)

const (
	controllerName = "workerpools"
)

func AddToManager(ctx context.Context, mgr manager.Manager, ns, hostname, nodename string, workerPool, namespace string) error {
	logger := logrus.WithField("controller", controllerName)
	c, err := controller.New(controllerName, mgr, controller.Options{
		MaxConcurrentReconciles: 1,
		Reconciler: &reconciler{
			logger:     logger,
			client:     mgr.GetClient(),
			nodename:   nodename,
			hostname:   hostname,
			workerPool: workerPool,
			namespace:  namespace,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to construct controller: %w", err)
	}

	if err := c.Watch(source.Kind(mgr.GetCache(), &workersv1.WorkerPool{}, &workerPoolsHandler{namespace: ns})); err != nil {
		return fmt.Errorf("failed to create watch for clusterpools: %w", err)
	}

	return nil
}

type workerPoolsHandler struct {
	namespace string
}

func (h *workerPoolsHandler) Create(ctx context.Context, e event.TypedCreateEvent[*workersv1.WorkerPool], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	if h.namespace != e.Object.Namespace {
		return
	}
	q.Add(reconcile.Request{NamespacedName: types.NamespacedName{Namespace: e.Object.Namespace, Name: e.Object.Name}})
}

func (h *workerPoolsHandler) Update(ctx context.Context, e event.TypedUpdateEvent[*workersv1.WorkerPool], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
}

func (h *workerPoolsHandler) Delete(ctx context.Context, e event.TypedDeleteEvent[*workersv1.WorkerPool], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
}

func (h *workerPoolsHandler) Generic(ctx context.Context, e event.TypedGenericEvent[*workersv1.WorkerPool], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
}

func registerWorkerInPool(ctx context.Context, logger *logrus.Entry, client ctrlruntimeclient.Client, workerPool, nodename, hostname, namespace string) error {
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		pool := &workersv1.WorkerPool{}
		err := client.Get(ctx, ctrlruntimeclient.ObjectKey{Namespace: namespace, Name: workerPool}, pool)
		if err != nil {
			return fmt.Errorf("failed to get workerpool %s in namespace %s: %w", workerPool, namespace, err)
		}
		if pool.Spec.Workers == nil {
			pool.Spec.Workers = make(map[string]workersv1.Worker)
		}

		now := time.Now()
		worker := workersv1.Worker{}
		if value, exists := pool.Spec.Workers[nodename]; exists {
			worker = value
			if worker.LastUpdateTime != nil {
				worker.LastUpdateTime.Time = now
			} else {
				worker.LastUpdateTime = &metav1.Time{Time: now}
			}
			worker.State = workersv1.WorkerAvailableState
			worker.Name = hostname
		} else {
			worker = workersv1.Worker{
				Name:                  hostname,
				Node:                  nodename,
				RegisteredTime:        &metav1.Time{Time: now},
				LastUpdateTime:        &metav1.Time{Time: now},
				CalculationsProcessed: 0,
				State:                 workersv1.WorkerAvailableState,
			}
		}
		pool.Spec.Workers[nodename] = worker

		logger.WithField("pod-name", hostname).WithField("node-name", nodename).Info("Updating WorkerPool...")
		if err := client.Update(ctx, pool); err != nil {
			return fmt.Errorf("failed to update WorkerPool %s: %w", pool.Name, err)
		}
		return nil
	}); err != nil {
		return err
	}

	return nil
}

func RemoveWorkerFromPool(ctx context.Context, logger *logrus.Entry, client ctrlruntimeclient.Client, workerPool, nodename, namespace string) error {
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		pool := &workersv1.WorkerPool{}
		err := client.Get(ctx, ctrlruntimeclient.ObjectKey{Namespace: namespace, Name: workerPool}, pool)
		if err != nil {
			return fmt.Errorf("failed to get workerpool %s in namespace %s: %w", workerPool, namespace, err)
		}

		if pool.Spec.Workers != nil {
			delete(pool.Spec.Workers, nodename)
			logger.WithField("node-name", nodename).Info("Removing worker from WorkerPool...")
			if err := client.Update(ctx, pool); err != nil {
				return fmt.Errorf("failed to update WorkerPool %s: %w", pool.Name, err)
			}
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

type reconciler struct {
	logger     *logrus.Entry
	client     ctrlruntimeclient.Client
	hostname   string
	nodename   string
	namespace  string
	workerPool string
}

func (r *reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	logger := r.logger.WithField("request", req.String())
	err := r.reconcile(ctx, req, logger)
	if err != nil {
		logger.WithError(err).Error("Reconciliation failed")
	} else {
		logger.Info("Finished reconciliation")
	}
	return reconcile.Result{}, err
}

func (r *reconciler) reconcile(ctx context.Context, req reconcile.Request, logger *logrus.Entry) error {
	logger.Info("Starting reconciliation")

	if err := registerWorkerInPool(ctx, logger, r.client, r.workerPool, r.nodename, r.hostname, r.namespace); err != nil {
		return fmt.Errorf("couldn't register worker in worker pool: %w", err)
	}

	return nil
}
