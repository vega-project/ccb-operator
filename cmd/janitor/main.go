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

	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	ctrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/vega-project/ccb-operator/pkg/apis/calculations/v1"
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

	if err := fs.Parse(os.Args[1:]); err != nil {
		logrus.WithError(err).Fatal("couldn't parse arguments")
	}
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
	ctx       context.Context
	client    ctrlruntimeclient.Client
	retention time.Duration
	logger    *logrus.Entry
}

func (c *controller) Start(stopChan <-chan struct{}, wg *sync.WaitGroup) {
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
			return
		case <-runChan:
			start := time.Now()
			if err := c.clean(); err != nil {
				c.logger.WithError(err).Error("Errors occurred while cleaning the calculations")
			}

			c.logger.Infof("Sync time: %v", time.Since(start))
		}
	}
}

func (c *controller) clean() error {
	var calculations v1.CalculationList
	if err := c.client.List(c.ctx, &calculations); err != nil {
		return fmt.Errorf("couldn't list calculations")
	}

	var errs []error
	for _, calc := range calculations.Items {
		logger := c.logger.WithField("calculation", calc.Name)
		if calc.Phase != v1.CompletedPhase {
			continue
		}

		if time.Since(calc.Status.StartTime.Time) <= c.retention {
			continue
		}

		if _, ok := calc.Labels[util.ResultsCollected]; !ok {
			continue
		}

		if err := c.client.Delete(c.ctx, &calc); err != nil {
			errs = append(errs, fmt.Errorf("couldn't delete calculation %s: %w", calc.Name, err))
			continue
		}

		logger.Info("Calculation deleted...")
	}
	return utilerrors.NewAggregate(errs)
}

func main() {
	logger := logrus.WithField("component", "janitor")
	o := gatherOptions()
	if err := o.validate(); err != nil {
		logger.WithError(err).Fatal("validation error")
	}

	clusterConfig, err := util.LoadClusterConfig()
	if err != nil {
		logger.WithError(err).Fatal("could not load cluster clusterConfig")
	}

	client, err := ctrlruntimeclient.New(clusterConfig, ctrlruntimeclient.Options{})
	if err != nil {
		logrus.WithError(err).Fatal("failed to create client")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := controller{
		ctx:       ctx,
		logger:    logger,
		retention: o.retention,
		client:    client,
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
