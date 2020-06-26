package workers

import (
	"fmt"
	"strconv"
	"time"

	redis "github.com/go-redis/redis"
	"github.com/sirupsen/logrus"

	corev1 "k8s.io/api/core/v1"

	informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
	"k8s.io/client-go/util/workqueue"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"

	calculationsv1 "github.com/vega-project/ccb-operator/pkg/apis/calculations/v1"
	calculationsclient "github.com/vega-project/ccb-operator/pkg/client/clientset/versioned"
	calclisters "github.com/vega-project/ccb-operator/pkg/client/listers/calculations/v1"
	"github.com/vega-project/ccb-operator/pkg/util"
)

const (
	controllerName = "WorkerPods"
)

// Controller ...
type Controller struct {
	podLister         listers.PodLister
	calculationLister calclisters.CalculationLister
	kubeClient        kubernetes.Interface
	calculationClient calculationsclient.Interface
	logger            *logrus.Entry
	podsSynced        cache.InformerSynced
	taskQueue         *util.TaskQueue
	redisClient       *redis.Client
}

// NewController ...
func NewController(kubeClient kubernetes.Interface, podInformer informers.PodInformer, calculationClient calculationsclient.Interface, calculationLister calclisters.CalculationLister, redisClient *redis.Client) *Controller {
	logger := logrus.WithField("controller", "pod-workers")
	logger.Level = logrus.DebugLevel
	controller := &Controller{
		podLister:         podInformer.Lister(),
		calculationClient: calculationClient,
		calculationLister: calculationLister,
		podsSynced:        podInformer.Informer().HasSynced,
		kubeClient:        kubeClient,
		logger:            logger,
		redisClient:       redisClient,
	}

	controller.taskQueue = util.NewTaskQueue(
		workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Pods"),
		controller.syncHandler, controllerName, logger)

	logger.Info("Setting up the Pods event handlers")
	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			mObj := obj.(*corev1.Pod)
			logger.Infof("Created object: %q", mObj.ObjectMeta.Name)
			controller.taskQueue.Enqueue(mObj)
		},
		UpdateFunc: func(old, changed interface{}) {
			// TODO:
			// IF the pod's status changes to NodeLost, we want to reassign the calculation
			// to a different worker, otherwise the calculation will be deadlocked.

			pod := changed.(*corev1.Pod)
			if !equality.Semantic.DeepEqual(old, changed) {
				controller.logger.Debugf("Updating object: %q", diff.ObjectReflectDiff(old, changed))
			}
			controller.taskQueue.Enqueue(pod)
		},
		DeleteFunc: func(obj interface{}) {
			pod := obj.(*corev1.Pod)
			controller.logger.WithField("pod-name", pod.Name).Warn("Deleting...")

			// TODO: When pod is deleted, we want to unassigned the uncompleted calculation that
			// its assigned to it.
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
	c.logger.Info("Starting Pods controller")

	// Wait for the caches to be synced before starting workers
	c.logger.Info("Waiting for pods informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.podsSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	go wait.Until(c.taskQueue.RunWorker, time.Second, stopCh)

	<-stopCh
	return nil
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Pod resource
// with the current status of the resource.
func (c *Controller) syncHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the Pod resource with this namespace/name
	pod, err := c.podLister.Pods(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			return fmt.Errorf("Pod %s in work queue no longer exists", key)
		}
		return err
	}

	if pod.Status.Phase != corev1.PodRunning {
		return fmt.Errorf("Pod '%s' is not in Ready phase", key)
	}

	// Get a list of the calculations that are assinged to this pod
	calculations, _ := c.calculationLister.List(labels.Set{"assign": name}.AsSelector())
	if calculations == nil {
		if err := c.createCalculationForPod(name); err != nil {
			c.logger.WithError(err).Error("couldn't create calculation")
		}
	} else {
		if !hasAssignedCalculation(calculations) {
			if err := c.createCalculationForPod(name); err != nil {
				c.logger.WithError(err).Error("couldn't create calculation")
			}
		}
	}
	return nil
}

func hasAssignedCalculation(calculations []*calculationsv1.Calculation) bool {
	for _, c := range calculations {
		if c.Phase == calculationsv1.CreatedPhase || c.Phase == calculationsv1.ProcessingPhase {
			return true
		}
	}
	return false
}

func (c *Controller) assignCalulationDB() (string, string, string) {
	// TODO: --flag vz
	vzList, err := c.redisClient.ZRange("vz", 0, -1).Result()
	if err != nil {
		c.logger.WithError(err).Error("redis error")
	}

	toUpdate := make(map[string]interface{})

	for _, vz := range vzList {
		status := c.redisClient.HMGet(vz, "status").Val()[0]
		if status == nil {
			teff := fmt.Sprintf("%v", c.redisClient.HMGet(vz, "teff").Val()[0])
			logG := fmt.Sprintf("%v", c.redisClient.HMGet(vz, "logG").Val()[0])

			// set status
			toUpdate["status"] = "Processing"

			c.logger.WithFields(logrus.Fields{"vz": vz, "teff": teff, "logG": logG, "toUpdate": toUpdate}).Info("Updating database...")
			c.redisClient.HMSet(vz, toUpdate)

			return vz, teff, logG
		}
	}
	return "", "", ""
}

func (c *Controller) createCalculationForPod(vegaPodName string) error {
	// TODO: first check for created_by_human calculations
	// then check in database
	dbKey, teff, logG := c.assignCalulationDB()

	if len(teff) == 0 {
		// There is no calculation to be processed in the database.
		return nil
	}

	t, err := strconv.ParseFloat(teff, 64)
	if err != nil {
		return fmt.Errorf("couldn't parse teff [%s] as float: %v", teff, err)
	}
	l, err := strconv.ParseFloat(logG, 64)
	if err != nil {
		return fmt.Errorf("couldn't parse logG [%s] as float: %v", logG, err)
	}

	calculation := util.NewCalculation(t, l)
	calculation.Labels = map[string]string{"assign": vegaPodName}
	calculation.Assign = vegaPodName
	calculation.DBKey = dbKey

	c.logger.WithFields(logrus.Fields{"name": calculation.Name, "for-pod": vegaPodName}).Info("Creating new calculation...")
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() (err error) {
		_, err = c.calculationClient.CalculationsV1().Calculations().Create(calculation)
		return err
	}); err != nil {
		c.logger.WithField("calculation", calculation.Name).WithError(err).Error("Couldn't create new calculation.")
		return err
	}

	return nil
}
