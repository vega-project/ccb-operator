package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"

	corev1 "k8s.io/api/core/v1"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	controllerruntime "sigs.k8s.io/controller-runtime"
	ctrlruntimelog "sigs.k8s.io/controller-runtime/pkg/log"

	v1 "github.com/vega-project/ccb-operator/pkg/apis/calculations/v1"
	"github.com/vega-project/ccb-operator/pkg/dispatcher/bulks"
	"github.com/vega-project/ccb-operator/pkg/dispatcher/calculations"
	"github.com/vega-project/ccb-operator/pkg/dispatcher/workerpools"
	"github.com/vega-project/ccb-operator/pkg/dispatcher/workers"
	"github.com/vega-project/ccb-operator/pkg/util"
)

type options struct {
	namespace string
	dryRun    bool
}

func gatherOptions() options {
	o := options{}
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	fs.StringVar(&o.namespace, "namespace", "vega", "Namespace where the calculations exists")
	fs.BoolVar(&o.dryRun, "dry-run", true, "")

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

	go func() {
		http.Handle("/metrics", promhttp.Handler())
		http.ListenAndServe(":3001", nil)
	}()

	mgr, err := controllerruntime.NewManager(clusterConfig, controllerruntime.Options{
		DryRunClient: o.dryRun,
		Logger:       ctrlruntimelog.NullLogger{},
	})
	if err != nil {
		logrus.WithError(err).Fatal("failed to construct manager")
	}

	if err := v1.AddToScheme(mgr.GetScheme()); err != nil {
		logrus.WithError(err).Fatal("Failed to add calculationv1 to scheme")
	}

	if err := corev1.AddToScheme(mgr.GetScheme()); err != nil {
		logrus.WithError(err).Fatal("Failed to add corev1 to scheme")
	}

	ctx := controllerruntime.SetupSignalHandler()
	if err := calculations.AddToManager(ctx, mgr, o.namespace); err != nil {
		logrus.WithError(err).Fatal("Failed to add calculations controller to manager")
	}

	if err := workers.AddToManager(mgr, o.namespace); err != nil {
		logrus.WithError(err).Fatal("Failed to add workers controller to manager")
	}

	if err := workerpools.AddToManager(mgr, o.namespace); err != nil {
		logrus.WithError(err).Fatal("Failed to add workerpools controller to manager")
	}

	if err := bulks.AddToManager(ctx, mgr, o.namespace); err != nil {
		logrus.WithError(err).Fatal("Failed to add workerpools controller to manager")
	}

	if err := mgr.Start(ctx); err != nil {
		logrus.WithError(err).Fatal("Manager ended with error")
	}
}
