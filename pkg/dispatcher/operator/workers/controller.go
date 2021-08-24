package workers

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"time"

	redis "github.com/go-redis/redis"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"

	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/fields"
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
	controllerName = "WorkerPods"
)

var podStatusValue = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: "vega",
	Name:      "pod_status",
	Help:      "Status of a worker pod",
},
	[]string{
		"pod_name",
		"pod_status",
	})

func init() {
	prometheus.Register(podStatusValue)
}

// Controller ...
type Controller struct {
	ctx                context.Context
	podLister          listers.PodLister
	calculationLister  calclisters.CalculationLister
	kubeClient         kubernetes.Interface
	calculationClient  calculationsclient.Interface
	logger             *logrus.Entry
	podsSynced         cache.InformerSynced
	taskQueue          *util.TaskQueue
	redisClient        *redis.Client
	redisSortedSetName string
}

// NewController ...
func NewController(ctx context.Context, kubeClient kubernetes.Interface, podInformer informers.PodInformer, calculationClient calculationsclient.Interface, calculationLister calclisters.CalculationLister, redisClient *redis.Client, redisSortedSetName string) *Controller {
	logger := logrus.WithField("controller", "pod-workers")
	logger.Level = logrus.DebugLevel
	controller := &Controller{
		ctx:                ctx,
		podLister:          podInformer.Lister(),
		calculationClient:  calculationClient,
		calculationLister:  calculationLister,
		podsSynced:         podInformer.Informer().HasSynced,
		kubeClient:         kubeClient,
		logger:             logger,
		redisClient:        redisClient,
		redisSortedSetName: redisSortedSetName,
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
			controller.taskQueue.Enqueue(obj)
		},
	})

	return controller
}

func (c *Controller) deleteAssignedCalculation(assigned string) error {
	calculationList, err := c.calculationClient.VegaV1().Calculations().List(context.TODO(), metav1.ListOptions{LabelSelector: fields.Set{"assign": assigned}.AsSelector().String()})
	if err != nil {
		return fmt.Errorf("Couldn't get a list of calculations: %v", err)
	}
	getAssignedCalculations := func(calculations []calculationsv1.Calculation) []calculationsv1.Calculation {
		var ret []calculationsv1.Calculation
		for _, c := range calculations {
			if (c.Phase == calculationsv1.CreatedPhase || c.Phase == calculationsv1.ProcessingPhase) && assigned == c.Assign {
				ret = append(ret, c)
			}
		}
		return ret
	}
	assignedCalculations := getAssignedCalculations(calculationList.Items)
	if len(assignedCalculations) == 0 {
		c.logger.WithField("pod-name", assigned).Warn("There were no calculations assigned to a certain pod to delete...")
		return nil
	}
	if len(assignedCalculations) > 1 {
		return fmt.Errorf("More than one calculations found: %#v", assignedCalculations)
	}

	toUpdate := make(map[string]interface{})
	toUpdate["status"] = ""

	for _, calc := range assignedCalculations {
		if (calc.Phase == calculationsv1.ProcessingPhase || calc.Phase == calculationsv1.CreatedPhase) && calc.Assign == assigned {
			if err := c.calculationClient.VegaV1().Calculations().Delete(context.TODO(), calc.Name, metav1.DeleteOptions{}); err != nil {
				return fmt.Errorf("Couldn't delete the calculation: %v", err)
			}
			c.logger.WithFields(logrus.Fields{"dbKey": calc.DBKey, "toUpdate": toUpdate}).Info("Updating database...")
			if cmd := c.redisClient.HMSet(calc.DBKey, toUpdate); cmd.Err() != nil {
				return fmt.Errorf("Couldn't update status in database: %v", cmd.Err())
			}
		}
	}
	return nil
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
			c.logger.WithError(err).Warningf("pod %s no longer exists. Removed from queue", key)
			return nil
		}
		return err
	}

	if pod.Status.Phase != corev1.PodRunning {
		return fmt.Errorf("Pod '%s' is not in Ready phase", key)
	}

	// Get a list of the calculations that are assinged to this pod
	calculations, err := c.calculationLister.List(labels.Set{"assign": name}.AsSelector())
	podStatusValue.With(prometheus.Labels{"pod_name": pod.Name, "pod_status": string(pod.Status.Phase)}).Inc()
	if err != nil {
		return fmt.Errorf("couldn't get the list of calculations that are assigned to %s: %w", name, err)
	}

	if calculations == nil || !hasAssignedCalculation(calculations) {
		if err := c.createCalculationForPod(name); err != nil {
			c.logger.WithError(err).WithField("pod", name).Error("couldn't create calculation")
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
	vzList, err := c.redisClient.ZRange(c.redisSortedSetName, 0, -1).Result()
	if err != nil {
		c.logger.WithError(err).Error("redis error")
	}

	for _, vz := range vzList {
		status := c.redisClient.HMGet(vz, "status").Val()[0]
		if status == nil {
			teff := fmt.Sprintf("%v", c.redisClient.HMGet(vz, "teff").Val()[0])
			logG := fmt.Sprintf("%v", c.redisClient.HMGet(vz, "logG").Val()[0])
			return vz, teff, logG
		}
	}
	return "", "", ""
}

func (c *Controller) createCalculationForPod(vegaPodName string) error {
	labelSelector := labels.Set{"assign": "\"\"", "created_by_human": "\"true\""}.AsSelector().String()
	humanCalculations, err := c.calculationClient.VegaV1().Calculations().List(c.ctx, metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return fmt.Errorf("couldn't get the list of created_by_human calculations %w", err)
	}

	var createdPhaseCalculations []calculationsv1.Calculation

	for _, c := range humanCalculations.Items {
		if c.Phase == calculationsv1.CreatedPhase {
			createdPhaseCalculations = append(createdPhaseCalculations, c)
		}
	}

	calculation := &calculationsv1.Calculation{}
	if len(createdPhaseCalculations) > 0 {
		if len(createdPhaseCalculations) > 1 {
			sort.Slice(createdPhaseCalculations, func(i, j int) bool {
				return createdPhaseCalculations[i].Status.StartTime.Before(&createdPhaseCalculations[j].Status.StartTime)
			})
		}

		for i, calc := range createdPhaseCalculations {
			if calc.Phase == calculationsv1.CreatedPhase {
				calculation = createdPhaseCalculations[i].DeepCopy()
				calculation.Assign = vegaPodName
				break
			}
		}

		if _, err = c.calculationClient.VegaV1().Calculations().Update(c.ctx, calculation, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("couldn't update calculation: %v", err)
		}

	} else {
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

		calculation = util.NewCalculation(t, l)
		calculation.Labels = map[string]string{"assign": vegaPodName}
		calculation.Assign = vegaPodName
		calculation.DBKey = dbKey

		c.logger.WithFields(logrus.Fields{"name": calculation.Name, "for-pod": vegaPodName}).Info("Creating new calculation...")
		if err := retry.RetryOnConflict(retry.DefaultRetry, func() (err error) {
			_, err = c.calculationClient.VegaV1().Calculations().Create(c.ctx, calculation, metav1.CreateOptions{})
			return err
		}); err != nil {
			c.logger.WithField("calculation", calculation.Name).WithError(err).Error("Couldn't create new calculation.")
			return err
		}

		toUpdate := make(map[string]interface{})
		toUpdate["status"] = "Processing"

		c.logger.WithFields(logrus.Fields{"dbKey": dbKey, "teff": teff, "logG": logG, "toUpdate": toUpdate}).Info("Updating database...")
		if boolCmd := c.redisClient.HMSet(dbKey, toUpdate); boolCmd.Err() != nil {
			return fmt.Errorf("couldn't update status in database: %v", boolCmd.Err())
		}
	}

	return nil
}
