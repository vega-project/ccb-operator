package bulks

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"

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

	bulkv1 "github.com/vega-project/ccb-operator/pkg/apis/calculationbulk/v1"
	workersv1 "github.com/vega-project/ccb-operator/pkg/apis/workers/v1"
	"github.com/vega-project/ccb-operator/pkg/util"
)

const (
	controllerName = "bulks"
)

func AddToManager(ctx context.Context, mgr manager.Manager, ns string) error {
	c, err := controller.New(controllerName, mgr, controller.Options{
		MaxConcurrentReconciles: 1,
		Reconciler: &reconciler{
			logger: logrus.WithField("controller", controllerName),
			client: mgr.GetClient(),
		},
	})
	if err != nil {
		return fmt.Errorf("failed to construct controller: %w", err)
	}

	predicateFuncs := predicate.Funcs{
		CreateFunc:  func(e event.CreateEvent) bool { return e.Object.GetNamespace() == ns },
		UpdateFunc:  func(e event.UpdateEvent) bool { return e.ObjectNew.GetNamespace() == ns },
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
	}

	if err := c.Watch(source.NewKindWithCache(&bulkv1.CalculationBulk{}, mgr.GetCache()), calculationBulkHandler(), predicateFuncs); err != nil {
		return fmt.Errorf("failed to create watch for Calculations: %w", err)
	}

	return nil
}

func calculationBulkHandler() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(o ctrlruntimeclient.Object) []reconcile.Request {
		bulk, ok := o.(*bulkv1.CalculationBulk)
		if !ok {
			logrus.WithField("type", fmt.Sprintf("%T", o)).Error("Got object that was not a CalculationBulk")
			return nil
		}

		return []reconcile.Request{
			{NamespacedName: types.NamespacedName{Namespace: bulk.Namespace, Name: bulk.Name}},
		}
	})
}

type reconciler struct {
	logger *logrus.Entry
	client ctrlruntimeclient.Client
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

	bulk := &bulkv1.CalculationBulk{}
	err := r.client.Get(ctx, ctrlruntimeclient.ObjectKey{Namespace: req.Namespace, Name: req.Name}, bulk)
	if err != nil {
		return fmt.Errorf("failed to get calculation bulk: %s in namespace %s: %w", req.Name, req.Namespace, err)
	}

	sortedCalculations := util.GetSortedCreatedCalculations(bulk.Calculations)
	for _, item := range sortedCalculations.Items {
		workerpool := &workersv1.WorkerPool{}
		if err := r.client.Get(ctx, ctrlruntimeclient.ObjectKey{Namespace: req.Namespace, Name: bulk.WorkerPool}, workerpool); err != nil {
			return fmt.Errorf("failed to get workerpool: %s in namespace %s: %w", bulk.WorkerPool, req.Namespace, err)
		}

		workerToReserve := util.GetFirstAvailableWorker(workerpool.Spec.Workers)
		if workerToReserve == nil {
			return nil
		}

		if err := r.reserveWorkerInPool(ctx, bulk.WorkerPool, req.Namespace, workerToReserve.Node); err != nil {
			return fmt.Errorf("couldn't reserve worker %s in pool %s: %w", workerToReserve.Node, bulk.WorkerPool, err)
		}

		calculation := item.Calculation
		name := item.Name
		// we assume that if the phase is empty, then the calculation haven't yet been processed.
		calc := util.NewCalculation(&calculation)
		logger = logger.WithField("calc-name", calc.Name).WithField("calc-bulk-name", name)

		calc.InputFiles = calculation.InputFiles
		calc.Pipeline = calculation.Pipeline
		calc.Assign = workerToReserve.Name
		calc.Namespace = req.Namespace
		calc.Labels = map[string]string{
			util.BulkLabel:            bulk.Name,
			util.CalculationNameLabel: name,
			"vegaproject.io/assign":   workerToReserve.Name,
			util.CalcRootFolder:       bulk.RootFolder,
		}

		logger.Info("Creating calculation.")
		if err := r.client.Create(ctx, calc); err != nil {
			return fmt.Errorf("couldn't create calculation: %w", err)
		}

	}

	return nil
}

func (r *reconciler) reserveWorkerInPool(ctx context.Context, workerPool, namespace, workerName string) error {
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		pool := &workersv1.WorkerPool{}
		if err := r.client.Get(ctx, ctrlruntimeclient.ObjectKey{Namespace: namespace, Name: workerPool}, pool); err != nil {
			return fmt.Errorf("failed to get the calculation bulk: %w", err)
		}

		worker := pool.Spec.Workers[workerName]
		worker.State = workersv1.WorkerReservedState
		pool.Spec.Workers[workerName] = worker

		r.logger.WithField("worker-pool", pool.Name).WithField("worker", workerName).Info("Updating worker pool...")
		if err := r.client.Update(ctx, pool); err != nil {
			return fmt.Errorf("failed to update worker pool %s: %w", pool.Name, err)
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}
