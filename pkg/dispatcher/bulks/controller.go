package bulks

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"

	"k8s.io/apimachinery/pkg/types"
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
	controllerName = "bulks"
)

func AddToManager(ctx context.Context, mgr manager.Manager, ns string, calculationCh chan v1.Calculation) error {
	c, err := controller.New(controllerName, mgr, controller.Options{
		MaxConcurrentReconciles: 1,
		Reconciler: &reconciler{
			logger:        logrus.WithField("controller", controllerName),
			client:        mgr.GetClient(),
			calculationCh: calculationCh,
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
	logger        *logrus.Entry
	client        ctrlruntimeclient.Client
	calculationCh chan v1.Calculation
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

	for _, item := range util.GetSortedCreatedCalculations(bulk.Calculations).Items {
		if item.Calculation.Phase == "" {
			calc := util.NewCalculation(&item.Calculation)
			calc.InputFiles = item.Calculation.InputFiles
			calc.Pipeline = item.Calculation.Pipeline
			calc.Namespace = req.Namespace
			calc.WorkerPool = bulk.WorkerPool
			calc.Labels = map[string]string{
				util.BulkLabel:            bulk.Name,
				util.CalculationNameLabel: item.Name,
				util.CalcRootFolder:       bulk.RootFolder,
			}
			r.calculationCh <- *calc
		}
	}
	return nil
}
