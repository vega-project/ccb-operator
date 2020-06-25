package calculations

import (
	"fmt"
	"time"

	redigo "github.com/garyburd/redigo/redis"
	"github.com/go-redis/redis"
	"github.com/sirupsen/logrus"

	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"

	calculationsv1 "github.com/vega-project/ccb-operator/pkg/apis/calculations/v1"
	calculationsclient "github.com/vega-project/ccb-operator/pkg/client/clientset/versioned"
	informers "github.com/vega-project/ccb-operator/pkg/client/informers/externalversions/calculations/v1"
	listers "github.com/vega-project/ccb-operator/pkg/client/listers/calculations/v1"
	"github.com/vega-project/ccb-operator/pkg/util"
)

const (
	controllerName = "Calculations"
)

// Controller ...
type Controller struct {
	calculationLister  listers.CalculationLister
	calculationClient  calculationsclient.Interface
	logger             *logrus.Entry
	calculationsSynced cache.InformerSynced
	taskQueue          *util.TaskQueue
	redisClient        *redis.Client
}

// NewController ...
func NewController(calculationClient calculationsclient.Interface, calculationInformer informers.CalculationInformer, redisClient *redis.Client) *Controller {
	logger := logrus.WithField("controller", "calculations")
	logger.Level = logrus.DebugLevel
	controller := &Controller{
		calculationLister:  calculationInformer.Lister(),
		calculationsSynced: calculationInformer.Informer().HasSynced,
		calculationClient:  calculationClient,
		logger:             logger,
		redisClient:        redisClient,
	}

	controller.taskQueue = util.NewTaskQueue(
		workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Calculations"),
		controller.syncHandler, controllerName, logger)

	logger.Info("Setting up the Calculations event handlers")
	calculationInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			mObj := obj.(*calculationsv1.Calculation)
			logger.Infof("Created object: %q", mObj.ObjectMeta.Name)
			controller.taskQueue.Enqueue(mObj)
		},
		UpdateFunc: func(old, changed interface{}) {
			if !equality.Semantic.DeepEqual(old, changed) {
				controller.logger.Debugf("Updating object: %q", diff.ObjectReflectDiff(old, changed))
			}
			controller.taskQueue.Enqueue(changed)
		},
		DeleteFunc: func(obj interface{}) {
			calc := obj.(*calculationsv1.Calculation)
			controller.logger.WithField("calculation", calc.Name).Warn("Deleting...")
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

	c.logger.Info("Starting calculation workers")
	go wait.Until(c.taskQueue.RunWorker, time.Second, stopCh)

	<-stopCh
	return nil
}

func (c *Controller) syncHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		fmt.Errorf("invalid resource key: %s", key)
		return nil
	}

	// Get the calculation resource with this namespace/name
	calc, err := c.calculationLister.Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			runtime.HandleError(fmt.Errorf("calculation '%s' in work queue no longer exists", key))
			return nil
		}
		return err
	}

	toUpdate := make(map[string]interface{})
	status, _ := redigo.Strings(c.redisClient.HMGet(calc.DBKey, "status").Result())

	if len(status) > 0 {
		if calc.Phase == "Completed" && status[0] != "Completed" {
			toUpdate["status"] = "Completed"
			c.logger.WithFields(logrus.Fields{"dbkey": calc.DBKey, "for-calculation": calc.Name}).Info("Updating database with results...")
			c.redisClient.HMSet(calc.DBKey, toUpdate)
		}
	}
	return nil
}
