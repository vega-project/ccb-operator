package workers

import (
	"context"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/util/retry"
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

	v1 "github.com/vega-project/ccb-operator/pkg/apis/calculations/v1"
	workersv1 "github.com/vega-project/ccb-operator/pkg/apis/workers/v1"
)

const (
	controllerName = "worker_pods"
)

var podStatusValue = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: "vega",
	Name:      "pod_status",
	Help:      "Status of a worker pod",
},
	[]string{
		"pod_name",
		"pod_status",
	})

func init() {
	prometheus.Register(podStatusValue)
}

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
		DeleteFunc: func(e event.DeleteEvent) bool { return e.Object.GetNamespace() == ns },
		UpdateFunc: func(e event.UpdateEvent) bool {

			// Object is marked for deletion
			if e.ObjectNew.GetDeletionTimestamp() != nil {
				return false
			}

			return e.ObjectNew.GetNamespace() == ns
		},
		GenericFunc: func(e event.GenericEvent) bool { return false },
	}
	if err := c.Watch(source.NewKindWithCache(&corev1.Pod{}, mgr.GetCache()), podHandler(), predicateFuncs); err != nil {
		return fmt.Errorf("failed to create watch for Pods: %w", err)
	}

	return nil
}

func podHandler() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(o ctrlruntimeclient.Object) []reconcile.Request {
		pod, ok := o.(*corev1.Pod)
		if !ok {
			logrus.WithField("type", fmt.Sprintf("%T", o)).Error("got object that was not a Pod")
			return nil
		}

		if pod.ObjectMeta.Labels != nil {
			v, ok := pod.ObjectMeta.Labels["name"]
			if !ok {
				return nil
			}
			if v != "vega-worker" {
				return nil
			}
		}

		return []reconcile.Request{
			{NamespacedName: types.NamespacedName{Namespace: pod.Namespace, Name: pod.Name}},
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

	pod := &corev1.Pod{}
	err := r.client.Get(ctx, ctrlruntimeclient.ObjectKey{Namespace: req.Namespace, Name: req.Name}, pod)
	if err != nil && !kerrors.IsNotFound(err) {
		return fmt.Errorf("failed to get pod: %s in namespace %s: %w", req.Name, req.Namespace, err)
	}

	if kerrors.IsNotFound(err) {
		if err := r.reconcileWorkerInPools(ctx, pod.Spec.NodeName); err != nil {
			return err
		}
		if err := r.deleteAssignedCalculations(ctx, pod.Name); err != nil {
			return err
		}
	}

	if pod.Status.Phase != corev1.PodRunning {
		logrus.WithField("pod_name", pod.Name).Error("Pod is not in Ready phase")
		return nil
	}

	podStatusValue.With(prometheus.Labels{"pod_name": pod.Name, "pod_status": string(pod.Status.Phase)}).Inc()

	// Get a list of the calculations that are assinged to this pod
	calcList := &v1.CalculationList{}
	if err := r.client.List(ctx, calcList, ctrlruntimeclient.MatchingLabels{"assign": req.Name}); err != nil {
		return fmt.Errorf("couldn't get a list of calculations: %v", err)
	}

	return nil
}

func (r *reconciler) reconcileWorkerInPools(ctx context.Context, podNode string) error {
	workerPools := &workersv1.WorkerPoolList{}
	if err := r.client.List(ctx, workerPools); err != nil {
		return fmt.Errorf("couldn't get a list of worker pools: %v", err)
	}

	for _, pool := range workerPools.Items {
		for name, worker := range pool.Spec.Workers {
			if worker.Name == podNode {
				if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
					workerPool := &workersv1.WorkerPool{}
					if err := r.client.Get(ctx, ctrlruntimeclient.ObjectKey{Namespace: pool.Namespace, Name: pool.Name}, workerPool); err != nil {
						return fmt.Errorf("failed to get the calculation: %w", err)
					}

					workerToUpdate := workerPool.Spec.Workers[worker.Name]
					workerToUpdate.State = workersv1.WorkerUnknownState

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
	if err := r.client.List(ctx, calcList, ctrlruntimeclient.MatchingLabels{"assign": assigned}); err != nil {
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
	if len(assignedCalculations) > 1 {
		return fmt.Errorf("more than one calculations found assigned to pod %s", assigned)
	}

	for _, calc := range assignedCalculations {
		if err := r.client.Delete(ctx, &calc); err != nil {
			return fmt.Errorf("couldn't delete the calculation: %v", err)
		}
	}

	// TODO Clean-up Phase for the corresponding calculation in bulk.
	return nil
}
