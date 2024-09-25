package bulks

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	ctrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	bulkv1 "github.com/vega-project/ccb-operator/pkg/apis/calculationbulk/v1"
	v1 "github.com/vega-project/ccb-operator/pkg/apis/calculations/v1"
	workersv1 "github.com/vega-project/ccb-operator/pkg/apis/workers/v1"
	"github.com/vega-project/ccb-operator/pkg/grpc"
	"github.com/vega-project/ccb-operator/pkg/util"
)

const (
	controllerName = "bulks"
)

func AddToManager(ctx context.Context, mgr manager.Manager, ns string, calculationCh chan v1.Calculation, gRPCClient *grpc.Client) error {
	c, err := controller.New(controllerName, mgr, controller.Options{
		MaxConcurrentReconciles: 1,
		Reconciler: &reconciler{
			logger:        logrus.WithField("controller", controllerName),
			client:        mgr.GetClient(),
			calculationCh: calculationCh,
			gRPCClient:    gRPCClient,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to construct controller: %w", err)
	}

	if err := c.Watch(source.Kind(mgr.GetCache(), &bulkv1.CalculationBulk{}, &calculationBulkHandler{namespace: ns})); err != nil {
		return fmt.Errorf("failed to create watch for clusterpools: %w", err)
	}

	return nil
}

type calculationBulkHandler struct {
	namespace string
}

func (h *calculationBulkHandler) Create(ctx context.Context, e event.TypedCreateEvent[*bulkv1.CalculationBulk], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	if h.namespace != e.Object.Namespace {
		return
	}
	q.Add(reconcile.Request{NamespacedName: types.NamespacedName{Namespace: e.Object.Namespace, Name: e.Object.Name}})
}

func (h *calculationBulkHandler) Update(ctx context.Context, e event.TypedUpdateEvent[*bulkv1.CalculationBulk], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	if h.namespace != e.ObjectNew.Namespace {
		return
	}
	q.Add(reconcile.Request{NamespacedName: types.NamespacedName{Namespace: e.ObjectNew.Namespace, Name: e.ObjectNew.Name}})
}

func (h *calculationBulkHandler) Delete(ctx context.Context, e event.TypedDeleteEvent[*bulkv1.CalculationBulk], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
}

func (h *calculationBulkHandler) Generic(ctx context.Context, e event.TypedGenericEvent[*bulkv1.CalculationBulk], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
}

type reconciler struct {
	logger        *logrus.Entry
	client        ctrlruntimeclient.Client
	calculationCh chan v1.Calculation
	gRPCClient    *grpc.Client
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
	if err := r.client.Get(ctx, ctrlruntimeclient.ObjectKey{Namespace: req.Namespace, Name: req.Name}, bulk); err != nil {
		return fmt.Errorf("failed to get calculation bulk: %s in namespace %s: %w", req.Name, req.Namespace, err)
	}

	workerpool := &workersv1.WorkerPool{}
	if err := r.client.Get(ctx, ctrlruntimeclient.ObjectKey{Namespace: bulk.Namespace, Name: bulk.WorkerPool}, workerpool); err != nil {
		return fmt.Errorf("failed to get workerpool: %s in namespace %s: %w", bulk.WorkerPool, bulk.Namespace, err)
	}

	if err := r.reconcileCalculations(bulk.Calculations); err != nil {
		return fmt.Errorf("failed to reconcile calculations: %w", err)
	}

	if err := r.client.Update(ctx, bulk); err != nil {
		return fmt.Errorf("failed to update calculation bulk: %w", err)
	}

	// If the bulk is finished and the post-calculation is not yet created, create it
	if util.IsAllFinishedCalculations(bulk.Calculations) && bulk.PostCalculation != nil && bulk.PostCalculation.Phase == "" {
		for _, worker := range workerpool.Spec.Workers {
			if worker.State == workersv1.WorkerAvailableState {

				calc := *newCalculationForBulk(*bulk, *bulk.PostCalculation, req.Namespace, worker.Name, bulk.WorkerPool, map[string]string{
					util.BulkLabel:            bulk.Name,
					util.PostCalculationLabel: "",
					util.CalcRootFolder:       bulk.RootFolder,
					util.AssignWorkerLabel:    worker.Name,
				})

				r.logger.WithField("calc-name", calc.Name).Info("Creating post calculation.")
				if err := r.client.Create(ctx, &calc); err != nil {
					r.logger.WithError(err).Error("couldn't create post calculation")
					return err
				}
				return nil
			}
		}
	}

	calculations := assignCalculationsToWorkers(bulk, workerpool, req.Namespace)
	for _, calc := range calculations {
		r.logger.WithField("calc-name", calc.Name).Info("Creating calculation.")
		if err := r.client.Create(ctx, &calc); err != nil {
			r.logger.WithError(err).Error("couldn't create calculation")
		}
	}

	return nil
}

func (r *reconciler) reconcileCalculations(calcs map[string]bulkv1.Calculation) error {
	for key, calc := range calcs {
		params := map[string]string{
			"log_g": fmt.Sprintf("%f", calc.Params.LogG),
			"teff":  fmt.Sprintf("%f", calc.Params.Teff),
		}

		_, err := r.gRPCClient.GetData(params)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			logrus.WithError(err).WithField("calc", key).Error("couldn't get results for calculation")
			continue
		}

		calc.Phase = v1.CachedPhase
		calcs[key] = calc

	}

	return nil
}

func assignCalculationsToWorkers(bulk *bulkv1.CalculationBulk, workerpool *workersv1.WorkerPool, namespace string) []v1.Calculation {
	var calculations []v1.Calculation
	calculationItems := util.GetSortedCreatedCalculations(bulk.Calculations).Items
	calculationIndex := 0

	// Sort workers
	workerKeys := make([]string, 0, len(workerpool.Spec.Workers))
	for k := range workerpool.Spec.Workers {
		workerKeys = append(workerKeys, k)
	}
	sort.Strings(workerKeys)

	for _, w := range workerKeys {
		worker := workerpool.Spec.Workers[w]
		if worker.State == workersv1.WorkerAvailableState && calculationIndex < len(calculationItems) {
			item := calculationItems[calculationIndex]
			if item.Calculation.Phase == "" {
				calculation := newCalculationForBulk(*bulk, item.Calculation, namespace, worker.Name, bulk.WorkerPool, map[string]string{
					util.BulkLabel:            bulk.Name,
					util.CalculationNameLabel: item.Name,
					util.CalcRootFolder:       bulk.RootFolder,
					util.AssignWorkerLabel:    worker.Name,
				})
				calculations = append(calculations, *calculation)
				calculationIndex++
			}
		}
	}
	return calculations
}

func newCalculationForBulk(bulk bulkv1.CalculationBulk, calcBulkCalculation bulkv1.Calculation, namespace, assignWorker, workerPool string, labels map[string]string) *v1.Calculation {
	calc := util.NewCalculation(&calcBulkCalculation)

	if calc.InputFiles == nil {
		calc.InputFiles = calcBulkCalculation.InputFiles
	}

	if calc.OutputFilesRegex == "" {
		calc.OutputFilesRegex = bulk.OutputFilesRegex
	}

	if calc.Pipeline == "" {
		calc.Pipeline = calcBulkCalculation.Pipeline
	}

	calc.Namespace = namespace
	calc.WorkerPool = workerPool
	calc.Labels = labels
	calc.Assign = assignWorker

	return calc
}
