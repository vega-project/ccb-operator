package workers

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/util/retry"
	"k8s.io/client-go/util/workqueue"
	ctrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	bulkv1 "github.com/vega-project/ccb-operator/pkg/apis/calculationbulk/v1"
	v1 "github.com/vega-project/ccb-operator/pkg/apis/calculations/v1"
	workersv1 "github.com/vega-project/ccb-operator/pkg/apis/workers/v1"
)

const (
	controllerName = "worker_pods"
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

	if err := c.Watch(source.Kind(mgr.GetCache(), &corev1.Pod{}, &podHandler{namespace: ns})); err != nil {
		return fmt.Errorf("failed to create watch for clusterpools: %w", err)
	}

	return nil
}

type podHandler struct {
	namespace string
}

func (h *podHandler) Create(ctx context.Context, e event.TypedCreateEvent[*corev1.Pod], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	if h.namespace != e.Object.Namespace {
		return
	}
	if e.Object.ObjectMeta.Labels != nil {
		v, ok := e.Object.ObjectMeta.Labels["name"]
		if !ok {
			return
		}
		if v != "vega-worker" {
			return
		}
	}
	q.Add(reconcile.Request{NamespacedName: types.NamespacedName{Namespace: e.Object.Namespace, Name: e.Object.Name}})
}

func (h *podHandler) Update(ctx context.Context, e event.TypedUpdateEvent[*corev1.Pod], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	if h.namespace != e.ObjectNew.Namespace {
		return
	}

	if e.ObjectNew.GetDeletionTimestamp() != nil {
		return
	}

	if e.ObjectNew.ObjectMeta.Labels != nil {
		v, ok := e.ObjectNew.ObjectMeta.Labels["name"]
		if !ok {
			return
		}
		if v != "vega-worker" {
			return
		}
	}

	q.Add(reconcile.Request{NamespacedName: types.NamespacedName{Namespace: e.ObjectNew.Namespace, Name: e.ObjectNew.Name}})
}

func (h *podHandler) Delete(ctx context.Context, e event.TypedDeleteEvent[*corev1.Pod], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
}

func (h *podHandler) Generic(ctx context.Context, e event.TypedGenericEvent[*corev1.Pod], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
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

	pod := &corev1.Pod{}
	err := r.client.Get(ctx, ctrlruntimeclient.ObjectKey{Namespace: req.Namespace, Name: req.Name}, pod)
	if err != nil && !kerrors.IsNotFound(err) {
		return fmt.Errorf("failed to get pod: %s in namespace %s: %w", req.Name, req.Namespace, err)
	}

	if kerrors.IsNotFound(err) {
		if err := r.reconcileWorkerInPools(ctx, req.Name); err != nil {
			return err
		}
		if err := r.deleteAssignedCalculations(ctx, req.Name); err != nil {
			return err
		}
	} else {
		if pod.Status.Phase != corev1.PodRunning {
			logrus.WithField("pod_name", req.Name).Error("Pod is not in Ready phase")
			return nil
		}
	}
	return nil
}

func (r *reconciler) reconcileWorkerInPools(ctx context.Context, podName string) error {
	workerPools := &workersv1.WorkerPoolList{}
	if err := r.client.List(ctx, workerPools); err != nil {
		return fmt.Errorf("couldn't get a list of worker pools: %v", err)
	}

	for _, pool := range workerPools.Items {
		for name, worker := range pool.Spec.Workers {
			if worker.Name == podName {
				if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
					workerPool := &workersv1.WorkerPool{}
					if err := r.client.Get(ctx, ctrlruntimeclient.ObjectKey{Namespace: pool.Namespace, Name: pool.Name}, workerPool); err != nil {
						return fmt.Errorf("failed to get the calculation: %w", err)
					}

					workerToUpdate := workerPool.Spec.Workers[name]
					workerToUpdate.State = workersv1.WorkerUnknownState

					workerPool.Spec.Workers[name] = workerToUpdate

					r.logger.WithField("worker-name", worker.Name).WithField("worker", name).Info("Updating worker pool")
					if err := r.client.Update(ctx, workerPool); err != nil {
						return fmt.Errorf("failed to update worker pool %s: %w", workerPool.Name, err)
					}
					return nil
				}); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (r *reconciler) deleteAssignedCalculations(ctx context.Context, assigned string) error {
	calcList := &v1.CalculationList{}
	if err := r.client.List(ctx, calcList, ctrlruntimeclient.MatchingLabels{"vegaproject.io/assign": assigned}); err != nil {
		return fmt.Errorf("couldn't get a list of calculations: %v", err)
	}

	getAssignedCalculations := func(calculations []v1.Calculation) []v1.Calculation {
		var ret []v1.Calculation
		for _, c := range calculations {
			if c.Phase == v1.CreatedPhase || c.Phase == v1.ProcessingPhase {
				ret = append(ret, c)
			}
		}
		return ret
	}
	assignedCalculations := getAssignedCalculations(calcList.Items)
	if len(assignedCalculations) == 0 {
		r.logger.WithField("pod-name", assigned).Info("there were no calculations assigned to pod to delete...")
		return nil
	}

	for _, calc := range assignedCalculations {
		if err := r.client.Delete(ctx, &calc); err != nil {
			return fmt.Errorf("couldn't delete the calculation: %v", err)
		}

		bulkName, exist := calc.Labels["vegaproject.io/bulk"]
		if !exist {
			continue
		}

		calcBulkName, exist := calc.Labels["vegaproject.io/calculationName"]
		if !exist {
			continue
		}

		if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			bulk := &bulkv1.CalculationBulk{}
			if err := r.client.Get(ctx, ctrlruntimeclient.ObjectKey{Namespace: calc.Namespace, Name: bulkName}, bulk); err != nil {
				return fmt.Errorf("failed to get the calculation: %w", err)
			}

			calculation := bulk.Calculations[calcBulkName]
			calculation.Phase = ""
			bulk.Calculations[calcBulkName] = calculation

			r.logger.WithField("bulk_calc_name", calcBulkName).WithField("bulk_name", bulkName).Info("Updating calculation bulk")
			if err := r.client.Update(ctx, bulk); err != nil {
				return fmt.Errorf("failed to update calculation bulk %s: %w", bulk.Name, err)
			}
			return nil
		}); err != nil {
			return err
		}

	}

	return nil
}
