package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	controllerruntime "sigs.k8s.io/controller-runtime"
	ctrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	ctrlruntimelog "sigs.k8s.io/controller-runtime/pkg/log"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/retry"

	calculationsv1 "github.com/vega-project/ccb-operator/pkg/apis/calculations/v1"
	workersv1 "github.com/vega-project/ccb-operator/pkg/apis/workers/v1"
	"github.com/vega-project/ccb-operator/pkg/worker/executor"
)

type Operator struct {
	ctx                      context.Context
	logger                   *logrus.Entry
	cfg                      *rest.Config
	calculationsController   *Controller
	executor                 *executor.Executor
	hostname                 string
	namespace                string
	workerPool               string
	nfsPath                  string
	atlasControlFiles        string
	atlasDataFiles           string
	kuruzModelTemplateFile   string
	synspecInputTemplateFile string
	dryRun                   bool
}

func NewMainOperator(
	ctx context.Context,
	hostname,
	namespace,
	workerPool,
	nfsPath,
	atlasControlFiles,
	atlasDataFiles,
	kuruzModelTemplateFile,
	synspecInputTemplateFile string,
	cfg *rest.Config,
	dryRun bool) *Operator {
	return &Operator{
		ctx:                      ctx,
		logger:                   logrus.WithField("name", "operator"),
		cfg:                      cfg,
		dryRun:                   dryRun,
		hostname:                 hostname,
		namespace:                namespace,
		workerPool:               workerPool,
		nfsPath:                  nfsPath,
		atlasControlFiles:        atlasControlFiles,
		atlasDataFiles:           atlasDataFiles,
		kuruzModelTemplateFile:   kuruzModelTemplateFile,
		synspecInputTemplateFile: synspecInputTemplateFile,
	}
}

func (op *Operator) Initialize() error {
	executeChan := make(chan *calculationsv1.Calculation)
	stepUpdaterChan := make(chan executor.Result)
	calcErrorChan := make(chan string)

	op.executor = executor.NewExecutor(executeChan, calcErrorChan, stepUpdaterChan, op.nfsPath,
		op.atlasControlFiles, op.atlasDataFiles, op.kuruzModelTemplateFile, op.synspecInputTemplateFile)

	mgr, err := controllerruntime.NewManager(op.cfg, controllerruntime.Options{
		DryRunClient: op.dryRun,
		Logger:       ctrlruntimelog.NullLogger{},
	})
	if err != nil {
		return fmt.Errorf("failed to construct manager: %w", err)
	}

	if err := op.registerWorkerInPool(mgr.GetClient()); err != nil {
		return fmt.Errorf("couldn't register worker in worker pool: %w", err)
	}
	op.calculationsController = NewController(op.ctx, mgr, executeChan, calcErrorChan, stepUpdaterChan, op.hostname, op.namespace, op.workerPool)
	return nil
}

func (op *Operator) registerWorkerInPool(client ctrlruntimeclient.Client) error {
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		pool := &workersv1.WorkerPool{}
		err := client.Get(op.ctx, ctrlruntimeclient.ObjectKey{Namespace: op.namespace, Name: op.workerPool}, pool)
		if err != nil {
			return fmt.Errorf("failed to get workerpool %s in namespace %s: %w", op.workerPool, op.namespace, err)
		}

		now := time.Now()
		if value, exists := pool.Spec.Workers[op.hostname]; exists {
			value.LastUpdateTime.Time = now
			value.State = workersv1.WorkerAvailableState
			pool.Spec.Workers[op.hostname] = value
		} else {
			if pool.Spec.Workers == nil {
				pool.Spec.Workers = make(map[string]workersv1.Worker)
			}

			pool.Spec.Workers[op.hostname] = workersv1.Worker{
				Name:                  op.hostname,
				RegisteredTime:        &metav1.Time{Time: now},
				LastUpdateTime:        &metav1.Time{Time: now},
				CalculationsProcessed: 0,
				State:                 workersv1.WorkerAvailableState,
			}
		}

		op.logger.Info("Updating WorkerPool...")
		if err := client.Update(op.ctx, pool); err != nil {
			return fmt.Errorf("failed to update WorkerPool %s: %w", pool.Name, err)
		}
		return nil
	}); err != nil {
		return err
	}

	return nil
}

func (op *Operator) Run(stopCh <-chan struct{}) error {
	var err error
	// TODO pass waitgroup
	go func() { err = op.calculationsController.Run(stopCh) }()
	if err != nil {
		return fmt.Errorf("failed to run Calculations controller: %s", err.Error())
	}

	// TODO pass waitgroup
	go func() { op.executor.Run() }()

	<-stopCh
	op.logger.Info("Shutting down controllers")
	return nil
}
