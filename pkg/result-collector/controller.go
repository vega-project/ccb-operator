package resultcollector

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"

	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"

	calculationsv1 "github.com/vega-project/ccb-operator/pkg/apis/calculations/v1"
	calculationsclient "github.com/vega-project/ccb-operator/pkg/client/clientset/versioned"
	informers "github.com/vega-project/ccb-operator/pkg/client/informers/externalversions/calculations/v1"
	listers "github.com/vega-project/ccb-operator/pkg/client/listers/calculations/v1"
	"github.com/vega-project/ccb-operator/pkg/util"
)

const (
	controllerName = "result-collector"
)

type Controller struct {
	calculationLister    listers.CalculationLister
	calculationClientSet calculationsclient.Interface
	logger               *logrus.Entry
	calculationsSynced   cache.InformerSynced
	taskQueue            *util.TaskQueue
	calculationsDir      string
	resultsDir           string
}

func NewController(calculationClientSet calculationsclient.Interface, calculationInformer informers.CalculationInformer, calculationsDir, resultsDir string) *Controller {
	logger := logrus.WithField("controller", "calculations")
	logger.Level = logrus.DebugLevel
	controller := &Controller{
		calculationLister:    calculationInformer.Lister(),
		calculationsSynced:   calculationInformer.Informer().HasSynced,
		calculationClientSet: calculationClientSet,
		logger:               logger,
		calculationsDir:      calculationsDir,
		resultsDir:           resultsDir,
	}

	controller.taskQueue = util.NewTaskQueue(
		workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Calculations"),
		controller.syncHandler, controllerName, logger)

	logger.Info("Setting up the Calculations event handlers")
	calculationInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			mObj := obj.(*calculationsv1.Calculation)
			controller.taskQueue.Enqueue(mObj)
		},
		UpdateFunc: func(old, changed interface{}) {
			controller.taskQueue.Enqueue(changed)
		},
		DeleteFunc: func(obj interface{}) {
			controller.taskQueue.Enqueue(obj)
		},
	})

	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()
	defer c.taskQueue.Workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	c.logger.Info("Starting Calculations controller")

	// Wait for the caches to be synced before starting workers
	c.logger.Info("Waiting for calculations informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.calculationsSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	c.logger.Info("Starting calculation worker")
	go wait.Until(c.taskQueue.RunWorker, time.Second, stopCh)
	c.logger.Info("Started calculation worker")

	<-stopCh
	return nil
}

func (c *Controller) syncHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the calculation resource with this namespace/name
	calculation, err := c.calculationLister.Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			fmt.Errorf("calculation '%s' in work queue no longer exists", key)
			return nil
		}
		return err
	}

	if isCompletedCalculation(calculation.Phase) {
		logger := c.logger.WithField("calculation", calculation.Name)
		resultPath := filepath.Join(c.resultsDir, fmt.Sprintf("%.1f___%.2f", calculation.Spec.Teff, calculation.Spec.LogG))

		if _, err := os.Stat(resultPath); os.IsNotExist(err) {
			logger.Info("Creating folder with results")
			if err := os.MkdirAll(resultPath, os.ModePerm); err != nil {
				return fmt.Errorf("couldn't create result's folder %v", err)
			}

			calcPath := filepath.Join(c.calculationsDir, calculation.Name)

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
					c.logger.WithError(err).Error("couldn't remove calculation folder")
					return fmt.Errorf("%v", err)
				}
			}
		}
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

func isCompletedCalculation(phase calculationsv1.CalculationPhase) bool {
	return phase == calculationsv1.CompletedPhase
}
