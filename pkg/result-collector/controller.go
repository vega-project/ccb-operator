package resultcollector

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"

	"k8s.io/client-go/util/retry"

	ctrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"k8s.io/apimachinery/pkg/types"

	v1 "github.com/vega-project/ccb-operator/pkg/apis/calculations/v1"
	"github.com/vega-project/ccb-operator/pkg/util"
)

const (
	controllerName = "result-collector"
)

func AddToManager(ctx context.Context, mgr manager.Manager, ns string, calculationsDir, resultsDir string) error {
	c, err := controller.New(controllerName, mgr, controller.Options{
		MaxConcurrentReconciles: 1,
		Reconciler: &reconciler{
			logger:          logrus.WithField("controller", controllerName),
			client:          mgr.GetClient(),
			namespace:       ns,
			calculationsDir: calculationsDir,
			resultsDir:      resultsDir,
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

	if err := c.Watch(source.NewKindWithCache(&v1.Calculation{}, mgr.GetCache()), calculationHandler(), predicateFuncs); err != nil {
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

		return []reconcile.Request{
			{NamespacedName: types.NamespacedName{Namespace: calc.Namespace, Name: calc.Name}},
		}
	})
}

type reconciler struct {
	logger          *logrus.Entry
	client          ctrlruntimeclient.Client
	calculationsDir string
	resultsDir      string
	namespace       string
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

	calculation := &v1.Calculation{}
	err := r.client.Get(ctx, ctrlruntimeclient.ObjectKey{Namespace: req.Namespace, Name: req.Name}, calculation)
	if err != nil {
		return fmt.Errorf("failed to get calculation: %s in namespace %s: %w", req.Name, req.Namespace, err)
	}
	logger = logger.WithField("calc", calculation.Name)

	if !isCompletedCalculation(calculation.Phase) {
		logger.Infof("Ignoring calculation with phase: %s", calculation.Phase)
		return nil
	}

	logger = logger.WithField("calculation", calculation.Name)
	resultPath := filepath.Join(r.resultsDir, fmt.Sprintf("%.1f___%.2f", calculation.Spec.Teff, calculation.Spec.LogG))

	if _, err := os.Stat(resultPath); os.IsNotExist(err) {
		logger.Info("Creating folder with results")
		if err := os.MkdirAll(resultPath, os.ModePerm); err != nil {
			return fmt.Errorf("couldn't create result's folder %v", err)
		}

		calcPath := filepath.Join(r.calculationsDir, calculation.Name)

		resultsCopied := true
		logger.Info("Copying fort-8 result file.")
		if _, err := copy(filepath.Join(calcPath, "fort.8"), filepath.Join(resultPath, "fort.8")); err != nil {
			logger.WithError(err).Error("error while copying file")
			resultsCopied = false
		}

		logger.Info("Copying fort-7 result file.")
		if _, err := copy(filepath.Join(calcPath, "fort.7"), filepath.Join(resultPath, "fort.7")); err != nil {
			logger.WithError(err).Error("error while copying file")
			resultsCopied = false
		}

		if resultsCopied {
			logger.Warn("Deleting calculation folder")
			// Remove calculation folder
			if err := os.RemoveAll(calcPath); err != nil {
				r.logger.WithError(err).Error("couldn't remove calculation folder")
				return fmt.Errorf("%v", err)
			}

			labels := map[string]string{util.ResultsCollected: "true"}
			if err := r.updateCalculationLabels(ctx, calculation.Name, labels); err != nil {
				r.logger.WithError(err).Error("couldn't update calculation labels")
				return fmt.Errorf("%v", err)
			}
		}

	}
	return nil
}

func (r *reconciler) updateCalculationLabels(ctx context.Context, calcName string, labels map[string]string) error {
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		calculation := &v1.Calculation{}
		if err := r.client.Get(ctx, ctrlruntimeclient.ObjectKey{Namespace: r.namespace, Name: calcName}, calculation); err != nil {
			return fmt.Errorf("failed to get the calculation: %w", err)
		}

		if labels != nil && calculation.Labels == nil {
			calculation.Labels = make(map[string]string)
		}
		for k, v := range labels {
			calculation.Labels[k] = v
		}

		if err := r.client.Update(ctx, calculation); err != nil {
			return fmt.Errorf("failed to update calculation %s: %w", calculation.Name, err)
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func copy(src, dst string) (int64, error) {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return 0, err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return 0, fmt.Errorf("%s is not a regular file", src)
	}

	source, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	defer destination.Close()
	nBytes, err := io.Copy(destination, source)
	return nBytes, err
}

func isCompletedCalculation(phase v1.CalculationPhase) bool {
	return phase == v1.CompletedPhase
}
