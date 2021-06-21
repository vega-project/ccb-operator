package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	"k8s.io/apimachinery/pkg/util/runtime"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"

	calculationsv1 "github.com/vega-project/ccb-operator/pkg/apis/calculations/v1"
	clientset "github.com/vega-project/ccb-operator/pkg/client/clientset/versioned"
	calculationscheme "github.com/vega-project/ccb-operator/pkg/client/clientset/versioned/scheme"
	informers "github.com/vega-project/ccb-operator/pkg/client/informers/externalversions"
	"github.com/vega-project/ccb-operator/pkg/worker/executor"
)

type Operator struct {
	ctx                      context.Context
	logger                   *logrus.Logger
	kubeclientset            kubernetes.Interface
	vegaclientset            clientset.Interface
	informer                 informers.SharedInformerFactory
	calculationsController   *Controller
	executor                 *executor.Executor
	hostname                 string
	nfsPath                  string
	atlasControlFiles        string
	atlasDataFiles           string
	kuruzModelTemplateFile   string
	synspecInputTemplateFile string
}

func NewMainOperator(ctx context.Context, kubeclientset kubernetes.Interface, vegaclientset clientset.Interface, hostname, nfsPath, atlasControlFiles, atlasDataFiles, kuruzModelTemplateFile, synspecInputTemplateFile string) *Operator {
	logger := logrus.New()
	logger.Level = logrus.DebugLevel
	return &Operator{
		ctx:                      ctx,
		logger:                   logger,
		hostname:                 hostname,
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
	op.informer = informers.NewSharedInformerFactory(op.vegaclientset, 30*time.Second)
	runtime.Must(calculationscheme.AddToScheme(scheme.Scheme))

	executeChan := make(chan *calculationsv1.Calculation)
	stepUpdaterChan := make(chan executor.Result)
	calcErrorChan := make(chan string)

	op.executor = executor.NewExecutor(executeChan, calcErrorChan, stepUpdaterChan, op.nfsPath,
		op.atlasControlFiles, op.atlasDataFiles, op.kuruzModelTemplateFile, op.synspecInputTemplateFile)

	op.calculationsController = NewController(op.ctx, op.vegaclientset, op.informer.Vega().V1().Calculations(), executeChan, calcErrorChan, stepUpdaterChan, op.hostname)
}

func (op *Operator) Run(stopCh <-chan struct{}) error {
	op.informer.Start(stopCh)

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
