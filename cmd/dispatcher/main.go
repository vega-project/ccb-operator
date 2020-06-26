package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"

	"k8s.io/client-go/kubernetes"
	coordination "k8s.io/client-go/kubernetes/typed/coordination/v1"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"

	client "github.com/vega-project/ccb-operator/pkg/client/clientset/versioned"
	"github.com/vega-project/ccb-operator/pkg/dispatcher/operator"
	"github.com/vega-project/ccb-operator/pkg/util"
)

type options struct {
	namespace string
	redisURL  string
}

func gatherOptions() options {
	o := options{}
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	fs.StringVar(&o.namespace, "namespace", "", "Namespace where the calculations exists")
	fs.StringVar(&o.redisURL, "redis-url", "", "Redis database url host")

	fs.Parse(os.Args[1:])
	return o
}

func validateOptions(o options) error {
	if len(o.namespace) == 0 {
		return fmt.Errorf("--namespace was not provided")
	}
	return nil
}

func main() {
	logger := logrus.New()

	o := gatherOptions()
	err := validateOptions(o)
	if err != nil {
		logger.WithError(err).Fatal("invalid options")
		os.Exit(1)
	}

	clusterConfig, err := util.LoadClusterConfig()
	if err != nil {
		logger.WithError(err).Error("could not load cluster clusterConfig")
	}

	vegaClient, err := client.NewForConfig(clusterConfig)
	if err != nil {
		logger.WithError(err).Error("could not create client")
	}

	kubeclient, err := kubernetes.NewForConfig(clusterConfig)
	if err != nil {
		logger.Fatalf("Failed to build Kubernetes client: %s", err.Error())
	}

	coordinationClient, err := coordination.NewForConfig(clusterConfig)
	if err != nil {
		logger.Fatalf("Failed to build coordination client: %s", err.Error())
	}

	stopCh := make(chan struct{}, 1)
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)

	id, err := os.Hostname()
	if err != nil {
		logger.WithError(err).Fatal("failed to get hostname")
	}
	lock, err := resourcelock.New(resourcelock.EndpointsResourceLock,
		o.namespace,
		"calculations",
		kubeclient.CoreV1(),
		coordinationClient,
		resourcelock.ResourceLockConfig{
			Identity: id,
		})
	if err != nil {
		logger.Fatalf("Failed to create lock: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
		Lock:          lock,
		LeaseDuration: 15 * time.Second,
		RenewDeadline: 10 * time.Second,
		RetryPeriod:   2 * time.Second,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				logger.Info("Started leading.")
				ctx, cancel = context.WithCancel(ctx)
				defer cancel()

				op := operator.NewMainOperator(ctx, kubeclient, vegaClient, o.redisURL)

				// Initialize the operator
				op.Initialize()

				if err := op.Run(stopCh); err != nil {
					logger.Fatalf("Error starting operator: %s", err.Error())
				}
				logger.Infoln("close.")
			},
			OnStoppedLeading: func() {
				logger.Fatalf("The leader election lost.")
			},
		},
	})

	for {
		select {
		case <-signalCh:
			logger.Infof("Shutdown signal received, exiting...")
			close(stopCh)
			os.Exit(0)
		}
	}
}
