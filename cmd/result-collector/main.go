package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"

	client "gitlab.physics.muni.cz/vega-project/ccb-operator/pkg/client/clientset/versioned"
	informers "gitlab.physics.muni.cz/vega-project/ccb-operator/pkg/client/informers/externalversions"
	resultcollector "gitlab.physics.muni.cz/vega-project/ccb-operator/pkg/result-collector"
	"gitlab.physics.muni.cz/vega-project/ccb-operator/pkg/util"
)

type options struct {
	calculationsDir string
	resultsDir      string
}

func gatherOptions() options {
	o := options{}
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	fs.StringVar(&o.calculationsDir, "calculations-dir", "", "The directory that contains the calculations.")
	fs.StringVar(&o.resultsDir, "results-dir", "", "Path were the results will be exported.")

	fs.Parse(os.Args[1:])
	return o
}

func validateOptions(o options) error {
	if len(o.calculationsDir) == 0 {
		return fmt.Errorf("--calculations-dir was not provided")
	}

	if len(o.resultsDir) == 0 {
		return fmt.Errorf("--results-dir was not provided")
	}
	return nil
}

func main() {
	logger := logrus.New()

	o := gatherOptions()

	if err := validateOptions(o); err != nil {
		logger.WithError(err).Error("Invalid configuration")
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

	informer := informers.NewSharedInformerFactory(vegaClient, 30*time.Second)
	controller := resultcollector.NewController(vegaClient, informer.Calculations().V1().Calculations(), o.calculationsDir, o.resultsDir)

	stopCh := make(chan struct{})
	defer close(stopCh)

	informer.Start(stopCh)

	go func() { err = controller.Run(2, stopCh) }()
	if err != nil {
		logger.WithError(err).Errorf("failed to run Calculations controller")
	}

	sigTerm := make(chan os.Signal, 1)
	signal.Notify(sigTerm, syscall.SIGTERM)
	signal.Notify(sigTerm, syscall.SIGINT)
	for {
		select {
		case <-sigTerm:
			logger.Infof("Shutdown signal received, exiting...")
			close(stopCh)
			os.Exit(0)
		}
	}
}
