package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	"k8s.io/client-go/util/retry"
	"k8s.io/client-go/util/workqueue"

	ctrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	v1 "github.com/vega-project/ccb-operator/pkg/apis/calculations/v1"
	workersv1 "github.com/vega-project/ccb-operator/pkg/apis/workers/v1"
	"github.com/vega-project/ccb-operator/pkg/util"
	"github.com/vega-project/ccb-operator/pkg/worker/workerpools"
)

const (
	controllerName = "calculations"
)

func AddToManager(ctx context.Context, mgr manager.Manager, ns, hostname, nodename string, executeChan chan *v1.Calculation, workerPool, namespace string) error {
	logger := logrus.WithField("controller", controllerName)
	c, err := controller.New(controllerName, mgr, controller.Options{
		MaxConcurrentReconciles: 1,
		Reconciler: &reconciler{
			logger:      logger,
			client:      mgr.GetClient(),
			hostname:    hostname,
			nodename:    nodename,
			executeChan: executeChan,
			workerPool:  workerPool,
			namespace:   namespace,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to construct controller: %w", err)
	}

	if err := c.Watch(source.Kind(mgr.GetCache(), &v1.Calculation{}, &calculationHandler{namespace: ns})); err != nil {
		return fmt.Errorf("failed to create watch for clusterpools: %w", err)
	}
	return nil
}

type calculationHandler struct {
	namespace string
}

func (h *calculationHandler) Create(ctx context.Context, e event.TypedCreateEvent[*v1.Calculation], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	if h.namespace != e.Object.Namespace {
		return
	}
	q.Add(reconcile.Request{NamespacedName: types.NamespacedName{Namespace: e.Object.Namespace, Name: e.Object.Name}})
}

func (h *calculationHandler) Update(ctx context.Context, e event.TypedUpdateEvent[*v1.Calculation], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
}

func (h *calculationHandler) Delete(ctx context.Context, e event.TypedDeleteEvent[*v1.Calculation], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
}

func (h *calculationHandler) Generic(ctx context.Context, e event.TypedGenericEvent[*v1.Calculation], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
}

type reconciler struct {
	logger      *logrus.Entry
	client      ctrlruntimeclient.Client
	executeChan chan *v1.Calculation

	hostname   string
	nodename   string
	namespace  string
	workerPool string
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
	if err != nil && !kerrors.IsNotFound(err) {
		return fmt.Errorf("failed to get calculation: %s in namespace %s: %w", req.Name, req.Namespace, err)
	}
	if kerrors.IsNotFound(err) {
		r.logger.WithError(err).Info("couln't find calculation. Ignoring...")
		return nil
	}

	if calculation.Assign == r.hostname {
		if calculation.Phase == v1.CreatedPhase {
			r.logger.WithField("calculation", calculation.Name).Info("Processing assigned calculation")

			if err := util.UpdateWorkerStatusInPool(ctx, r.client, r.workerPool, r.nodename, r.namespace, workersv1.WorkerProcessingState); err != nil {
				return fmt.Errorf("failed to update worker's state in worker pool: %w", err)
			}

			r.logger.Info("Sent for execution")
			r.executeChan <- calculation

			if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				calculation := &v1.Calculation{}
				if err := r.client.Get(ctx, ctrlruntimeclient.ObjectKey{Namespace: req.Namespace, Name: req.Name}, calculation); err != nil {
					return fmt.Errorf("failed to get the calculation: %w", err)
				}

				calculation.Phase = v1.ProcessingPhase
				calculation.Status.PendingTime = &metav1.Time{Time: time.Now()}

				r.logger.WithField("calculation", calculation.Name).Info("Updating calculation phase...")
				if err := r.client.Update(ctx, calculation); err != nil {
					return fmt.Errorf("failed to update calculation %s: %w", calculation.Name, err)
				}
				return nil
			}); err != nil {
				return err
			}
		}
	}

	return nil
}

type Controller struct {
	ctx             context.Context
	logger          *logrus.Entry
	mgr             manager.Manager
	client          ctrlruntimeclient.Client
	stepUpdaterChan chan util.Result
	calcErrorChan   chan string
	hostname        string
	nodename        string
	namespace       string
	workerPool      string
}

func NewController(
	ctx context.Context,
	mgr manager.Manager,
	executeChan chan *v1.Calculation,
	calcErrorChan chan string,
	stepUpdaterChan chan util.Result,
	hostname, nodename, namespace, workerPool string) *Controller {
	logger := logrus.WithField("controller", "calculations")
	logger.Level = logrus.DebugLevel
	controller := &Controller{
		ctx:             ctx,
		logger:          logger,
		client:          mgr.GetClient(),
		stepUpdaterChan: stepUpdaterChan,
		calcErrorChan:   calcErrorChan,
		hostname:        hostname,
		nodename:        nodename,
		mgr:             mgr,
		namespace:       namespace,
		workerPool:      workerPool,
	}

	if err := AddToManager(ctx, mgr, namespace, hostname, nodename, executeChan, workerPool, namespace); err != nil {
		logrus.WithError(err).Fatal("Failed to add calculations controller to manager")
	}

	if err := workerpools.AddToManager(ctx, mgr, namespace, hostname, nodename, workerPool, namespace); err != nil {
		logrus.WithError(err).Fatal("Failed to add workerpools controller to manager")
	}

	return controller
}

func (c *Controller) Run(stopCh <-chan struct{}) error {
	c.logger.Info("Starting calculation results updater")
	go c.resultUpdater(stopCh)

	if err := c.mgr.Start(c.ctx); err != nil {
		logrus.WithError(err).Fatal("Manager ended with error")
	}

	<-stopCh
	return nil
}

func (c *Controller) resultUpdater(stopCh <-chan struct{}) {
	for {
		select {
		case stepResult := <-c.stepUpdaterChan:
			c.logger.Info("Updating calculation")
			if err := c.updateCalculation(stepResult); err != nil {
				c.logger.WithError(err).Error("Couldn't update calculation's results.")
			}
		case <-stopCh:
			c.logger.Info("Stopping resultUpdater")
			return
		case calcName := <-c.calcErrorChan:
			if err := c.updateErrorCalculation(calcName); err != nil {
				c.logger.WithError(err).WithField("calc-name", calcName).Error("Error updating calculation")
			}

			if err := util.UpdateWorkerStatusInPool(c.ctx, c.client, c.workerPool, c.nodename, c.namespace, workersv1.WorkerAvailableState); err != nil {
				c.logger.WithError(err).Error("failed to update worker's state in worker pool: %w", err)
			}
		}
	}
}

func (c *Controller) updateErrorCalculation(name string) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {

		calculation := &v1.Calculation{}
		if err := c.client.Get(c.ctx, ctrlruntimeclient.ObjectKey{Namespace: c.namespace, Name: name}, calculation); err != nil {
			return fmt.Errorf("failed to get the calculation: %w", err)
		}

		calculation.Phase = v1.FailedPhase
		calculation.Status.CompletionTime = &metav1.Time{Time: time.Now()}

		if err := c.client.Update(c.ctx, calculation); err != nil {
			return fmt.Errorf("failed to update calculation %s: %w", calculation.Name, err)
		}
		return nil
	})
}

func (c *Controller) updateCalculation(r util.Result) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		calculation := &v1.Calculation{}
		if err := c.client.Get(c.ctx, ctrlruntimeclient.ObjectKey{Namespace: c.namespace, Name: r.CalcName}, calculation); err != nil {
			return fmt.Errorf("failed to get the calculation: %w", err)
		}

		calculation.Spec.Steps[r.Step].Status = r.Status // TODO add the rest of the results here

		if err := c.client.Update(c.ctx, calculation); err != nil {
			return fmt.Errorf("failed to update calculation %s: %w", calculation.Name, err)
		}
		return nil
	})
}
