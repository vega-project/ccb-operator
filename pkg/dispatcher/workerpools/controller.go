package workerpools

import (
	"context"
	"fmt"
	"sort"

	"github.com/sirupsen/logrus"

	ctrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"

	bulkv1 "github.com/vega-project/ccb-operator/pkg/apis/calculationbulk/v1"
	v1 "github.com/vega-project/ccb-operator/pkg/apis/calculations/v1"
	workersv1 "github.com/vega-project/ccb-operator/pkg/apis/workers/v1"
	"github.com/vega-project/ccb-operator/pkg/util"
)

const (
	controllerName = "worker_pools"
)

func AddToManager(mgr manager.Manager, ns string) error {
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
		CreateFunc: func(e event.CreateEvent) bool { return e.Object.GetNamespace() == ns },
		DeleteFunc: func(e event.DeleteEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {

			// Object is marked for deletion
			if e.ObjectNew.GetDeletionTimestamp() != nil {
				return false
			}

			return e.ObjectNew.GetNamespace() == ns
		},
		GenericFunc: func(e event.GenericEvent) bool { return false },
	}
	if err := c.Watch(source.NewKindWithCache(&workersv1.WorkerPool{}, mgr.GetCache()), poolHandler(), predicateFuncs); err != nil {
		return fmt.Errorf("failed to create watch for WorkerPools: %w", err)
	}

	return nil
}

func poolHandler() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(o ctrlruntimeclient.Object) []reconcile.Request {
		pool, ok := o.(*workersv1.WorkerPool)
		if !ok {
			logrus.WithField("type", fmt.Sprintf("%T", o)).Error("got object that was not a WorkerPool")
			return nil
		}
		return []reconcile.Request{
			{NamespacedName: types.NamespacedName{Namespace: pool.Namespace, Name: pool.Name}},
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

	workerpool := &workersv1.WorkerPool{}
	err := r.client.Get(ctx, ctrlruntimeclient.ObjectKey{Namespace: req.Namespace, Name: req.Name}, workerpool)
	if err != nil && !kerrors.IsNotFound(err) {
		return fmt.Errorf("failed to get workerpool: %s in namespace %s: %w", req.Name, req.Namespace, err)
	}

	for _, worker := range sortWorkers(workerpool.Spec.Workers) {
		if worker.State == workersv1.WorkerAvailableState {
			calculationBulks := &bulkv1.CalculationBulkList{}
			if err := r.client.List(ctx, calculationBulks); err != nil {
				return fmt.Errorf("couldn't get the list of calculationbulks %w", err)
			}

			if len(calculationBulks.Items) > 0 {
				// Sorting the calculationbulks by the creation time
				if len(calculationBulks.Items) > 1 {
					sort.Slice(calculationBulks.Items, func(i, j int) bool {
						return calculationBulks.Items[i].Status.CreatedTime.Before(&calculationBulks.Items[j].Status.CreatedTime)
					})
				}

				bulk := calculationBulks.Items[0]
				for name, calculation := range bulk.Calculations {
					// we assume that if the phase is empty, then the calculation haven't yet been processed.
					if calculation.Phase == "" {
						calc := util.NewCalculation(calculation.Params.Teff, calculation.Params.LogG)
						calc.Assign = worker.Name
						calc.Namespace = req.Namespace
						calc.Labels = map[string]string{
							"vegaproject.io/bulk":            bulk.Name,
							"vegaproject.io/calculationName": name,
							"vegaproject.io/assign":          worker.Name,
						}

						if err := r.client.Create(ctx, calc); err != nil {
							return fmt.Errorf("couldn't create calculation: %w", err)
						}

						// Update bulk with the calculation phase
						if err := r.updateWorkerCalculationBulk(ctx, name, bulk.Name, bulk.Namespace); err != nil {
							return fmt.Errorf("couldn't update calculation in bulk %s: %w", bulk.Name, err)
						}

						// We don't want to continue assigning calculations to that worker
						break
					}

				}
			}
		}
	}
	return nil
}

func (r *reconciler) updateWorkerCalculationBulk(ctx context.Context, calcName, bulkName, bulkNamespace string) error {
	bulk := &bulkv1.CalculationBulk{}
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err := r.client.Get(ctx, ctrlruntimeclient.ObjectKey{Namespace: bulkNamespace, Name: bulkName}, bulk); err != nil {
			return fmt.Errorf("failed to get the calculation: %w", err)
		}

		calc := bulk.Calculations[calcName]
		calc.Phase = v1.ProcessingPhase
		bulk.Calculations[calcName] = calc

		r.logger.WithField("bulk", bulkName).Info("Updating calculation bulk...")
		if err := r.client.Update(ctx, bulk); err != nil {
			return fmt.Errorf("failed to update calculation bulk %s: %w", bulk.Name, err)
		}
		return nil
	}); err != nil {
		return err
	}

	return nil
}

func sortWorkers(workers map[string]workersv1.Worker) []workersv1.Worker {
	var ret []workersv1.Worker

	for _, v := range workers {
		ret = append(ret, v)

	}
	sort.Slice(ret, func(i, j int) bool {
		return ret[i].LastUpdateTime.Before(ret[j].LastUpdateTime)
	})
	return ret
}
