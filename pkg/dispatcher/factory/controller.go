package factory

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ghodss/yaml"
	"github.com/sirupsen/logrus"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
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

	bulkv1 "github.com/vega-project/ccb-operator/pkg/apis/calculationbulk/v1"
	v1 "github.com/vega-project/ccb-operator/pkg/apis/calculationbulkfactory/v1"
	calcv1 "github.com/vega-project/ccb-operator/pkg/apis/calculations/v1"
	"github.com/vega-project/ccb-operator/pkg/util"
)

const (
	controllerName = "factory"
)

func AddToManager(ctx context.Context, mgr manager.Manager, ns string, calculationCh chan calcv1.Calculation, nfsPath string) error {
	c, err := controller.New(controllerName, mgr, controller.Options{
		MaxConcurrentReconciles: 1,
		Reconciler: &reconciler{
			logger:        logrus.WithField("controller", controllerName),
			client:        mgr.GetClient(),
			calculationCh: calculationCh,
			nfsPath:       nfsPath,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to construct controller: %w", err)
	}

	if err := c.Watch(source.Kind(mgr.GetCache(), &v1.CalculationBulkFactory{}, &calculationBulkFactoryHandler{namespace: ns})); err != nil {
		return fmt.Errorf("failed to create watch for clusterpools: %w", err)
	}

	return nil
}

type calculationBulkFactoryHandler struct {
	namespace string
}

func (h *calculationBulkFactoryHandler) Create(ctx context.Context, e event.TypedCreateEvent[*v1.CalculationBulkFactory], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	if h.namespace != e.Object.Namespace {
		return
	}
	q.Add(reconcile.Request{NamespacedName: types.NamespacedName{Namespace: e.Object.Namespace, Name: e.Object.Name}})
}

func (h *calculationBulkFactoryHandler) Update(ctx context.Context, e event.TypedUpdateEvent[*v1.CalculationBulkFactory], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	if h.namespace != e.ObjectNew.Namespace {
		return
	}
	q.Add(reconcile.Request{NamespacedName: types.NamespacedName{Namespace: e.ObjectNew.Namespace, Name: e.ObjectNew.Name}})
}

func (h *calculationBulkFactoryHandler) Delete(ctx context.Context, e event.TypedDeleteEvent[*v1.CalculationBulkFactory], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
}

func (h *calculationBulkFactoryHandler) Generic(ctx context.Context, e event.TypedGenericEvent[*v1.CalculationBulkFactory], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
}

type reconciler struct {
	logger        *logrus.Entry
	client        ctrlruntimeclient.Client
	calculationCh chan calcv1.Calculation
	nfsPath       string
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
	r.logger.Info("Starting reconciliation")

	factory := &v1.CalculationBulkFactory{}
	err := r.client.Get(ctx, ctrlruntimeclient.ObjectKey{Namespace: req.Namespace, Name: req.Name}, factory)
	if err != nil {
		return fmt.Errorf("failed to get calculationbulkfactory %s in namespace %s: %w", req.Name, req.Namespace, err)
	}
	if kerrors.IsNotFound(err) {
		return nil
	}

	if factory.Status.CompletionTime != nil && !factory.Status.BulkCreated {
		bulkFile := filepath.Join(r.nfsPath, factory.RootFolder, factory.BulkOutput)
		b, err := os.ReadFile(bulkFile)
		if err != nil {
			return err
		}

		var bulk bulkv1.CalculationBulk
		if err := yaml.Unmarshal(b, &bulk); err != nil {
			return nil
		}

		r.logger.WithField("bulk", bulk.Name).Info("Creating calculation bulk")
		if err := r.client.Create(ctx, &bulk); err != nil {
			return err
		}

		if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			f := &v1.CalculationBulkFactory{}
			err := r.client.Get(ctx, ctrlruntimeclient.ObjectKey{Namespace: req.Namespace, Name: req.Name}, f)
			if err != nil {
				return fmt.Errorf("failed to get calculationbulkfactory %s in namespace %s: %w", req.Name, req.Namespace, err)
			}

			f.Status.BulkCreated = true
			if err := r.client.Update(ctx, f); err != nil {
				return fmt.Errorf("failed to update calculationbulkfactory %s: %w", f.Name, err)
			}
			return nil
		}); err != nil {
			return err
		}

		return nil
	}

	calc := calcv1.Calculation{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: req.Namespace,
			Name:      fmt.Sprintf("calc-factory-%s", req.Name),
			Labels: map[string]string{
				util.FactoryLabel:   req.Name,
				util.CalcRootFolder: factory.RootFolder,
			},
		},
		Phase:      calcv1.CreatedPhase,
		InputFiles: factory.InputFiles,
		Spec: calcv1.CalculationSpec{
			Steps: []calcv1.Step{
				{
					Command: factory.Command,
					Args:    factory.Args,
				},
			},
		},
		WorkerPool: factory.WorkerPool,
	}

	r.calculationCh <- calc

	return nil
}
