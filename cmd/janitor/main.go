package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/vega-project/ccb-operator/pkg/apis/calculations/v1"
	client "github.com/vega-project/ccb-operator/pkg/client/clientset/versioned"
	calculationsv1 "github.com/vega-project/ccb-operator/pkg/client/clientset/versioned/typed/calculations/v1"

	"github.com/vega-project/ccb-operator/pkg/util"
)

type options struct {
	retention       time.Duration
	retentionString string
}

func gatherOptions() options {
	o := options{}
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	fs.StringVar(&o.retentionString, "retention", "24h", "How long calculations will be allow to exist in the cluster")

	fs.Parse(os.Args[1:])
	return o
}

func (o *options) validate() error {
	if o.retentionString != "" {
		var err error
		o.retention, err = time.ParseDuration(o.retentionString)
		if err != nil {
			return fmt.Errorf("couldn't parse duration: %v", err)
		}
	}
	return nil
}

type controller struct {
	ctx           context.Context
	calcInterface calculationsv1.VegaV1Interface
	retention     time.Duration
	logger        *logrus.Entry
}

func (c *controller) Start(stopChan <-chan struct{}, wg *sync.WaitGroup) error {
	defer wg.Done()

	c.logger.Info("Starting controller")
	runChan := make(chan struct{})

	go func() {
		for {
			runChan <- struct{}{}
			time.Sleep(30 * time.Second)
		}
	}()

	for {
		select {
		case <-stopChan:
			c.logger.Info("Stopping controller")
			return nil
		case <-runChan:
			start := time.Now()
			c.clean()
			c.logger.Infof("Sync time: %v", time.Since(start))
		}
	}
}

func (c *controller) clean() {
	calculations, err := c.calcInterface.Calculations().List(c.ctx, metav1.ListOptions{})
	if err != nil {
		c.logger.WithError(err).Error("Error listing calculations.")
		return
	}

	for _, calc := range calculations.Items {
		calcLogger := c.logger.WithField("calculation", calc.Name)
		if calc.Phase != v1.CompletedPhase {
			continue
		}

		if time.Since(calc.Status.StartTime.Time) <= c.retention {
			continue
		}

		if _, ok := calc.Labels[util.ResultsCollected]; !ok {
			calcLogger.Warn("calculation passed retention but results are not collected. Skipping...")
			continue
		}

		if err := c.calcInterface.Calculations().Delete(c.ctx, calc.Name, metav1.DeleteOptions{}); err != nil {
			calcLogger.WithError(err).Error("Error deleting calculation")
			continue
		}

		calcLogger.Info("Deleted calculation")
	}
}

func main() {
	logger := logrus.New()
	o := gatherOptions()
	if err := o.validate(); err != nil {
		logger.WithError(err).Fatal("validation error")
	}

	clusterConfig, err := util.LoadClusterConfig()
	if err != nil {
		logger.WithError(err).Fatal("could not load cluster clusterConfig")
	}

	calcClient, err := client.NewForConfig(clusterConfig)
	if err != nil {
		logger.WithError(err).Fatal("could not create calculation client")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := controller{
		ctx:           ctx,
		logger:        logrus.NewEntry(logrus.StandardLogger()),
		retention:     o.retention,
		calcInterface: calcClient.VegaV1(),
	}

	stopCh := make(chan struct{})
	defer close(stopCh)

	var wg sync.WaitGroup
	wg.Add(1)
	go c.Start(stopCh, &wg)

	sigTerm := make(chan os.Signal, 1)
	signal.Notify(sigTerm, syscall.SIGTERM)
	signal.Notify(sigTerm, syscall.SIGINT)
	for {
		select {
		case <-sigTerm:
			logger.Infof("Shutdown signal received, exiting...")
			close(stopCh)
			wg.Wait()
			os.Exit(0)
		}
	}
}
