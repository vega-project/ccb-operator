package workerpools

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
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
			workerPool: workerPool,
			namespace:  namespace,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to construct controller: %w", err)
	}

	predicateFuncs := predicate.Funcs{
		CreateFunc:  func(e event.CreateEvent) bool { return e.Object.GetNamespace() == namespace },
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		UpdateFunc:  func(e event.UpdateEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
	}

	if err := c.Watch(source.NewKindWithCache(&workersv1.WorkerPool{}, mgr.GetCache()), workerPoolsHandler(), predicateFuncs); err != nil {
		return fmt.Errorf("failed to create watch for WorkerPools: %w", err)
	}

	return nil
}
func registerWorkerInPool(ctx context.Context, logger *logrus.Entry, client ctrlruntimeclient.Client, workerPool, nodename, hostname, namespace string) error {
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		pool := &workersv1.WorkerPool{}
		err := client.Get(ctx, ctrlruntimeclient.ObjectKey{Namespace: namespace, Name: workerPool}, pool)
		if err != nil {
			return fmt.Errorf("failed to get workerpool %s in namespace %s: %w", workerPool, namespace, err)
		}

		now := time.Now()
		if value, exists := pool.Spec.Workers[nodename]; exists {
			if value.LastUpdateTime != nil {
				value.LastUpdateTime.Time = now
			} else {
				value.LastUpdateTime = &metav1.Time{Time: now}
			}
			value.State = workersv1.WorkerAvailableState
			value.Name = hostname
			pool.Spec.Workers[nodename] = value
		} else {
			if pool.Spec.Workers == nil {
				pool.Spec.Workers = make(map[string]workersv1.Worker)
			}

			pool.Spec.Workers[nodename] = workersv1.Worker{
				Name:                  hostname,
				RegisteredTime:        &metav1.Time{Time: now},
				LastUpdateTime:        &metav1.Time{Time: now},
				CalculationsProcessed: 0,
				State:                 workersv1.WorkerAvailableState,
			}
		}

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

func workerPoolsHandler() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(o ctrlruntimeclient.Object) []reconcile.Request {
		pool, ok := o.(*workersv1.WorkerPool)
		if !ok {
			logrus.WithField("type", fmt.Sprintf("%T", o)).Error("Got object that was not a WorkerPool")
			return nil
		}

		return []reconcile.Request{
			{NamespacedName: types.NamespacedName{Namespace: pool.Namespace, Name: pool.Name}},
		}
	})
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
