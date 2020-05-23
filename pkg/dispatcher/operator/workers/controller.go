package workers

import (
	"crypto/sha256"
	"encoding/base32"
	"fmt"
	"strconv"
	"time"

	redigo "github.com/garyburd/redigo/redis"
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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	controllerName  = "WorkerPods"
	vegaPodLabel    = "vega-worker"
	phaseCreated    = "Created"
	phaseCompleted  = "Completed"
	phaseProcessing = "Processing"
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
			fmt.Errorf("Pod '%s' in work queue no longer exists", key)
			return nil
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
		if c.Phase == phaseCreated || c.Phase == phaseProcessing {
			return true
		}
	}
	return false
}

func (c *Controller) assignCalulationDB() (string, []string, []string) {
	// TODO: --flag vz
	vzList, err := c.redisClient.LRange("vz", 0, 100000).Result()

	if err != nil {
		c.logger.WithError(err).Error("redis error")
	}

	toUpdate := make(map[string]interface{})

	for _, vz := range vzList {
		status, _ := redigo.Strings(c.redisClient.HMGet(vz, "status").Result())
		if len(status) > 0 && status[0] == "" {
			teff, _ := redigo.Strings(c.redisClient.HMGet(vz, "teff").Result())
			logG, _ := redigo.Strings(c.redisClient.HMGet(vz, "logG").Result())

			// set status
			toUpdate["status"] = "Processing"

			c.logger.WithFields(logrus.Fields{"vz": vz, "teff": teff, "logG": logG, "toUpdate": toUpdate}).Info("Updating database...")
			c.redisClient.HMSet(vz, toUpdate)

			return vz, teff, logG
		}
	}
	return "", nil, nil
}

func (c *Controller) createCalculationForPod(vegaPodName string) error {
	dbKey, teff, logG := c.assignCalulationDB()

	if len(teff) == 0 {
		// There is no calculation to be processed in the database.
		return nil
	}

	t, err := strconv.ParseFloat(teff[0], 64)
	if err != nil {
		return fmt.Errorf("couldn't parse teff [%s] as float: %v", teff, err)
	}
	l, err := strconv.ParseFloat(logG[0], 64)
	if err != nil {
		return fmt.Errorf("couldn't parse logG [%s] as float: %v", logG, err)
	}

	calcSpec := calculationsv1.CalculationSpec{
		Teff: t,
		LogG: l,
		Steps: []calculationsv1.Step{
			{
				Command: "atlas12_ada",
				Args:    []string{"s"},
			},
			{
				Command: "atlas12_ada",
				Args:    []string{"r"},
			},
			{
				Command: "synspec49",
				Args:    []string{"<", "input_tlusty_fortfive"},
			},
		},
	}

	calcName := fmt.Sprintf("calc-%s", inputHash([]byte(teff[0]), []byte(logG[0])))
	calculation := &calculationsv1.Calculation{
		TypeMeta: metav1.TypeMeta{Kind: "Calculation", APIVersion: "vega.io/v1"},
		ObjectMeta: metav1.ObjectMeta{
			Name:   calcName,
			Labels: map[string]string{"assign": vegaPodName},
		},
		Assign: vegaPodName,
		DBKey:  dbKey,
		Phase:  phaseCreated,
		Spec:   calcSpec,
	}

	c.logger.WithFields(logrus.Fields{"name": calcName, "for-pod": vegaPodName}).Info("Creating new calculation...")
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() (err error) {
		_, err = c.calculationClient.CalculationsV1().Calculations().Create(calculation)
		return err
	}); err != nil {
		c.logger.WithField("calculation", calculation.Name).WithError(err).Error("Couldn't create new calculation.")
		return err
	}

	return nil
}

// oneWayEncoding can be used to encode hex to a 62-character set (0 and 1 are duplicates) for use in
// short display names that are safe for use in kubernetes as resource names.
var oneWayNameEncoding = base32.NewEncoding("bcdfghijklmnpqrstvwxyz0123456789").WithPadding(base32.NoPadding)

// inputHash returns a string that hashes the unique parts of the input to avoid collisions.
func inputHash(inputs ...[]byte) string {
	hash := sha256.New()

	// the inputs form a part of the hash
	for _, s := range inputs {
		hash.Write(s)
	}

	// Object names can't be too long so we truncate the hash.
	return oneWayNameEncoding.EncodeToString(hash.Sum(nil)[:16])
}
