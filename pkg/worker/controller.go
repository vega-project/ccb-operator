package worker

import (
	"context"
	"fmt"
	"time"

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

	calculationsv1 "github.com/vega-project/ccb-operator/pkg/apis/calculations/v1"
	v1 "github.com/vega-project/ccb-operator/pkg/apis/calculations/v1"
	workersv1 "github.com/vega-project/ccb-operator/pkg/apis/workers/v1"
	"github.com/vega-project/ccb-operator/pkg/worker/executor"
	"github.com/vega-project/ccb-operator/pkg/worker/workerpools"
)

const (
	controllerName = "calculations"
)

func AddToManager(ctx context.Context, mgr manager.Manager, ns, hostname string, executeChan chan *calculationsv1.Calculation, workerPool, namespace string) error {
	logger := logrus.WithField("controller", controllerName)
	c, err := controller.New(controllerName, mgr, controller.Options{
		MaxConcurrentReconciles: 1,
		Reconciler: &reconciler{
			logger:      logger,
			client:      mgr.GetClient(),
			hostname:    hostname,
			executeChan: executeChan,
			workerPool:  workerPool,
			namespace:   namespace,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to construct controller: %w", err)
	}

	predicateFuncs := predicate.Funcs{
		CreateFunc:  func(e event.CreateEvent) bool { return e.Object.GetNamespace() == ns },
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		UpdateFunc:  func(e event.UpdateEvent) bool { return false },
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
	logger      *logrus.Entry
	client      ctrlruntimeclient.Client
	executeChan chan *calculationsv1.Calculation

	hostname   string
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
	if err != nil {
		return fmt.Errorf("failed to get pod: %s in namespace %s: %w", req.Name, req.Namespace, err)
	}

	if calculation.Assign == r.hostname {
		switch calculation.Phase {
		case v1.CreatedPhase:
			r.logger.WithField("calculation", calculation.Name).Info("Processing assigned calculation")

			if err := r.updateWorkerStatusInPool(ctx, workersv1.WorkerProcessingState); err != nil {
				return fmt.Errorf("failed to update worker's state in worker pool: %w", err)
			}

			r.logger.Info("Sent for execution")
			r.executeChan <- calculation

			if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				calculation := &v1.Calculation{}
				if err := r.client.Get(ctx, ctrlruntimeclient.ObjectKey{Namespace: req.Namespace, Name: req.Name}, calculation); err != nil {
					return fmt.Errorf("failed to get the calculation: %w", err)
				}

				calculation.Phase = calculationsv1.ProcessingPhase
				calculation.Status.PendingTime = &metav1.Time{Time: time.Now()}

				r.logger.WithField("calculation", calculation.Name).Info("Updating calculation phase...")
				if err := r.client.Update(ctx, calculation); err != nil {
					return fmt.Errorf("failed to update calculation %s: %w", calculation.Name, err)
				}
				return nil
			}); err != nil {
				return err
			}
		case calculationsv1.ProcessingPhase:
			if isFinishedCalculation(calculation.Spec.Steps) {
				if err := r.updateCalculationPhase(ctx, calculation, getCalculationFinalPhase(calculation.Spec.Steps)); err != nil {
					return fmt.Errorf("failed to update the calculation phase: %w", err)
				}
				if err := r.updateWorkerStatusInPool(ctx, workersv1.WorkerAvailableState); err != nil {
					return fmt.Errorf("failed to update worker's state in worker pool: %w", err)
				}
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
	stepUpdaterChan chan executor.Result
	calcErrorChan   chan string
	hostname        string
	nodename        string
	namespace       string
}

func NewController(
	ctx context.Context,
	mgr manager.Manager,
	executeChan chan *calculationsv1.Calculation,
	calcErrorChan chan string,
	stepUpdaterChan chan executor.Result,
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
	}

	if err := AddToManager(ctx, mgr, namespace, hostname, executeChan, workerPool, namespace); err != nil {
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

func isFinishedCalculation(steps []calculationsv1.Step) bool {
	for _, step := range steps {
		if step.Status == "" {
			return false
		}
	}
	return true
}

func hasFailedStep(steps []calculationsv1.Step) bool {
	for _, step := range steps {
		if step.Status == "Failed" {
			return true
		}
	}
	return false
}

func getCalculationFinalPhase(steps []calculationsv1.Step) calculationsv1.CalculationPhase {
	if hasFailedStep(steps) {
		return calculationsv1.FailedPhase
	}
	return calculationsv1.CompletedPhase
}

func (r *reconciler) updateCalculationPhase(ctx context.Context, calc *calculationsv1.Calculation, phase calculationsv1.CalculationPhase) error {
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
		return err
	}
	return nil
}

func (r *reconciler) updateWorkerStatusInPool(ctx context.Context, state workersv1.WorkerState) error {
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		pool := &workersv1.WorkerPool{}
		err := r.client.Get(ctx, ctrlruntimeclient.ObjectKey{Namespace: r.namespace, Name: r.workerPool}, pool)
		if err != nil {
			return fmt.Errorf("failed to get workerpool %s in namespace %s: %w", r.workerPool, r.namespace, err)
		}

		now := time.Now()
		if value, exists := pool.Spec.Workers[r.hostname]; exists {
			if value.LastUpdateTime != nil {
				value.LastUpdateTime.Time = now
			} else {
				value.LastUpdateTime = &metav1.Time{Time: now}
			}
			value.State = state
		}

		r.logger.Info("Updating WorkerPool...")
		if err := r.client.Update(ctx, pool); err != nil {
			return fmt.Errorf("failed to update WorkerPool %s: %w", pool.Name, err)
		}
		return nil
	}); err != nil {
		return err
	}

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
		}
	}
}

func (c *Controller) updateErrorCalculation(name string) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {

		calculation := &v1.Calculation{}
		if err := c.client.Get(c.ctx, ctrlruntimeclient.ObjectKey{Namespace: c.namespace, Name: name}, calculation); err != nil {
			return fmt.Errorf("failed to get the calculation: %w", err)
		}

		calculation.Phase = calculationsv1.FailedPhase
		calculation.Status.CompletionTime = &metav1.Time{Time: time.Now()}

		if err := c.client.Update(c.ctx, calculation); err != nil {
			return fmt.Errorf("failed to update calculation %s: %w", calculation.Name, err)
		}
		return nil
	})
}

func (c *Controller) updateCalculation(r executor.Result) error {
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
