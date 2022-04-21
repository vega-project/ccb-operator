package calculations

import (
	"context"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
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

	bulkv1 "github.com/vega-project/ccb-operator/pkg/apis/calculationbulk/v1"
	v1 "github.com/vega-project/ccb-operator/pkg/apis/calculations/v1"
	"github.com/vega-project/ccb-operator/pkg/util"
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
	if err := prometheus.Register(calculationValues); err != nil {
		logrus.Errorf("couldn't register calculation values in prometheus")
	}
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
		UpdateFunc:  func(e event.UpdateEvent) bool { return e.ObjectNew.GetNamespace() == ns },
		GenericFunc: func(e event.GenericEvent) bool { return false },
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
		return fmt.Errorf("failed to get calculation: %s in namespace %s: %w", req.Name, req.Namespace, err)
	}

	if calc.Phase == v1.ProcessingPhase {
		if util.IsFinishedCalculation(calc.Spec.Steps) {
			phase := util.GetCalculationFinalPhase(calc.Spec.Steps)
			if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				calculation := &v1.Calculation{}
				if err := r.client.Get(ctx, ctrlruntimeclient.ObjectKey{Namespace: calc.Namespace, Name: calc.Name}, calculation); err != nil {
					return fmt.Errorf("failed to get the calculation: %w", err)
				}

				calculation.Phase = phase
				calculation.Status.CompletionTime = &metav1.Time{Time: time.Now()}

				r.logger.WithField("calculation", calculation.Name).Info("Updating calculation phase...")
				if err := r.client.Update(ctx, calculation); err != nil {
					return fmt.Errorf("failed to update calculation %s: %w", calculation.Name, err)
				}
				return nil
			}); err != nil {
				return fmt.Errorf("failed to update the calculation phase: %w", err)
			}
		}
	}

	var bulkName string
	if value, exists := calc.Labels[util.BulkLabel]; exists {
		bulkName = value
	} else {
		return fmt.Errorf("no `%s` label found in calculation: %s/%s", util.BulkLabel, req.Namespace, req.Name)
	}

	var calcName string
	if value, exists := calc.Labels[util.CalculationNameLabel]; exists {
		calcName = value
	} else {
		return fmt.Errorf("no `%s` label found in calculation: %s/%s", util.CalculationNameLabel, req.Namespace, req.Name)
	}

	// Update the calculation Bulk that holds this calculation
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		bulk := &bulkv1.CalculationBulk{}
		if err := r.client.Get(ctx, ctrlruntimeclient.ObjectKey{Namespace: req.Namespace, Name: bulkName}, bulk); err != nil {
			return fmt.Errorf("failed to get the calculation bulk: %w", err)
		}

		bulkCalc := bulk.Calculations[calcName]
		bulkCalc.Phase = calc.Phase

		bulk.Calculations[calcName] = bulkCalc

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
