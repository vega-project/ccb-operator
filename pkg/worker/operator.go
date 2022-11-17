package worker

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"

	controllerruntime "sigs.k8s.io/controller-runtime"

	"k8s.io/client-go/rest"

	calculationsv1 "github.com/vega-project/ccb-operator/pkg/apis/calculations/v1"
	"github.com/vega-project/ccb-operator/pkg/util"
	"github.com/vega-project/ccb-operator/pkg/worker/executor"
)

type Operator struct {
	ctx                    context.Context
	logger                 *logrus.Entry
	cfg                    *rest.Config
	calculationsController *Controller
	executor               *executor.Executor
	hostname               string
	nodename               string
	namespace              string
	workerPool             string
	nfsPath                string
	dryRun                 bool
}

func NewMainOperator(ctx context.Context, hostname, nodename, namespace, workerPool, nfsPath string, cfg *rest.Config, dryRun bool) *Operator {
	return &Operator{
		ctx:        ctx,
		logger:     logrus.WithField("name", "operator"),
		cfg:        cfg,
		dryRun:     dryRun,
		hostname:   hostname,
		nodename:   nodename,
		namespace:  namespace,
		workerPool: workerPool,
		nfsPath:    nfsPath,
	}
}

func (op *Operator) Initialize() error {
	executeChan := make(chan *calculationsv1.Calculation)
	stepUpdaterChan := make(chan util.Result)
	calcErrorChan := make(chan string)

	mgr, err := controllerruntime.NewManager(op.cfg, controllerruntime.Options{DryRunClient: op.dryRun})
	if err != nil {
		return fmt.Errorf("failed to construct manager: %w", err)
	}

	op.executor = executor.NewExecutor(op.ctx, mgr.GetClient(), executeChan, calcErrorChan, stepUpdaterChan, op.nfsPath, op.nodename, op.namespace, op.workerPool)
	op.calculationsController = NewController(op.ctx, mgr, executeChan, calcErrorChan, stepUpdaterChan, op.hostname, op.nodename, op.namespace, op.workerPool)
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
