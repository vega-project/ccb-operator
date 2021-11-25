package calculations

import (
	"context"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/retry"
	ctrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	v1 "github.com/vega-project/ccb-operator/pkg/apis/calculations/v1"
)

const (
	controllerName = "calculations"
)

var calculationValues = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: "vega",
	Name:      "calculations",
	Help:      "Calculation ID, status and time of creation",
},
	[]string{
		"calc_id",
		"status",
		"creation_time",
	})

func init() {
	prometheus.Register(calculationValues)
}

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
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		UpdateFunc:  func(e event.UpdateEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return e.Object.GetNamespace() == ns },
	}

	cache := mgr.GetCache()
	indexFunc := func(obj ctrlruntimeclient.Object) []string {
		return []string{string(obj.(*v1.Calculation).Phase)}
	}

	if err := cache.IndexField(ctx, &v1.Calculation{}, "phase", indexFunc); err != nil {
		return fmt.Errorf("failed to construct the indexing fields for the cache")
	}

	if err := c.Watch(source.NewKindWithCache(&v1.Calculation{}, cache), calculationHandler(), predicateFuncs); err != nil {
		return fmt.Errorf("failed to create watch for Calculations: %w", err)
	}

	return nil
}

func calculationHandler() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(o ctrlruntimeclient.Object) []reconcile.Request {
		calc, ok := o.(*v1.Calculation)
		if !ok {
			logrus.WithField("type", fmt.Sprintf("%T", o)).Error("Got object that was not a Calculation")
			return nil
		}

		calculationValues.With(prometheus.Labels{"calc_id": calc.Name, "status": string(calc.Phase), "creation_time": calc.Status.StartTime.Time.String()}).Inc()

		return []reconcile.Request{
			{NamespacedName: types.NamespacedName{Namespace: calc.Namespace, Name: calc.Name}},
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

	calc := &v1.Calculation{}
	err := r.client.Get(ctx, ctrlruntimeclient.ObjectKey{Namespace: req.Namespace, Name: req.Name}, calc)
	if err != nil {
		return fmt.Errorf("failed to get pod: %s in namespace %s: %w", req.Name, req.Namespace, err)
	}

	processingCalculations := &v1.CalculationList{}
	if err := r.client.List(ctx, processingCalculations, ctrlruntimeclient.MatchingFields{"phase": "Processing"}); err != nil {
		return fmt.Errorf("couldn't get a list of calculations: %v", err)
	}

	podList := &corev1.PodList{}
	if err := r.client.List(ctx, podList, ctrlruntimeclient.MatchingLabels{"name": "vega-worker"}); err != nil {
		return fmt.Errorf("couldn't get a list of vega-worker pods: %v", err)
	}

	freeWorkers := func() sets.String {
		ret := sets.NewString()
		for _, pod := range podList.Items {
			ret.Insert(pod.Name)
		}
		for _, calc := range processingCalculations.Items {
			ret.Delete(calc.Assign)
		}
		return ret
	}()

	// TODO: if there are no free workers, we should requeue the calculation

	if freeWorkers.Len() > 0 {
		podName := freeWorkers.List()[0]
		if err := r.assignCalculationToPod(ctx, calc, podName); err != nil {
			r.logger.WithError(err).Errorf("couldn't create calculation for pod '%s'", podName)
		}
	}

	return nil
}

func (r *reconciler) assignCalculationToPod(ctx context.Context, calc *v1.Calculation, podName string) error {
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		calculation := &v1.Calculation{}
		if err := r.client.Get(ctx, ctrlruntimeclient.ObjectKey{Namespace: calc.Namespace, Name: calc.Name}, calculation); err != nil {
			return fmt.Errorf("failed to get the calculation: %w", err)
		}

		calculation.Assign = podName

		r.logger.WithField("pod", podName).Info("Updating calculation...")
		if err := r.client.Update(ctx, calculation); err != nil {
			return fmt.Errorf("failed to update calculation %s: %w", calculation.Name, err)
		}
		return nil
	}); err != nil {
		return err
	}

	return nil
}
