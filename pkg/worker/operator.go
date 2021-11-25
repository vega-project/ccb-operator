package worker

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"

	controllerruntime "sigs.k8s.io/controller-runtime"
	ctrlruntimelog "sigs.k8s.io/controller-runtime/pkg/log"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	calculationsv1 "github.com/vega-project/ccb-operator/pkg/apis/calculations/v1"
	clientset "github.com/vega-project/ccb-operator/pkg/client/clientset/versioned"
	"github.com/vega-project/ccb-operator/pkg/worker/executor"
)

type Operator struct {
	ctx                      context.Context
	logger                   *logrus.Logger
	cfg                      *rest.Config
	kubeclientset            kubernetes.Interface
	vegaclientset            clientset.Interface
	calculationsController   *Controller
	executor                 *executor.Executor
	hostname                 string
	namespace                string
	nfsPath                  string
	atlasControlFiles        string
	atlasDataFiles           string
	kuruzModelTemplateFile   string
	synspecInputTemplateFile string
	dryRun                   bool
}

func NewMainOperator(
	ctx context.Context,
	kubeclientset kubernetes.Interface,
	vegaclientset clientset.Interface,
	hostname,
	namespace,
	nfsPath,
	atlasControlFiles,
	atlasDataFiles,
	kuruzModelTemplateFile,
	synspecInputTemplateFile string,
	cfg *rest.Config,
	dryRun bool) *Operator {
	logger := logrus.New()
	logger.Level = logrus.DebugLevel
	return &Operator{
		ctx:                      ctx,
		logger:                   logger,
		cfg:                      cfg,
		dryRun:                   dryRun,
		hostname:                 hostname,
		namespace:                namespace,
		kubeclientset:            kubeclientset,
		vegaclientset:            vegaclientset,
		nfsPath:                  nfsPath,
		atlasControlFiles:        atlasControlFiles,
		atlasDataFiles:           atlasDataFiles,
		kuruzModelTemplateFile:   kuruzModelTemplateFile,
		synspecInputTemplateFile: synspecInputTemplateFile,
	}
}

func (op *Operator) Initialize() {
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
		logrus.WithError(err).Fatal("failed to construct manager")
	}

	op.calculationsController = NewController(op.ctx, mgr, executeChan, calcErrorChan, stepUpdaterChan, op.hostname, op.namespace)
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
