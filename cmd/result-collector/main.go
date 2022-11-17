package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/sirupsen/logrus"

	controllerruntime "sigs.k8s.io/controller-runtime"

	v1 "github.com/vega-project/ccb-operator/pkg/apis/calculations/v1"
	resultcollector "github.com/vega-project/ccb-operator/pkg/result-collector"
	"github.com/vega-project/ccb-operator/pkg/util"
)

type options struct {
	calculationsDir string
	resultsDir      string
	namespace       string
	dryRun          bool
}

func gatherOptions() options {
	o := options{}
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	fs.StringVar(&o.calculationsDir, "calculations-dir", "", "The directory that contains the calculations.")
	fs.StringVar(&o.resultsDir, "results-dir", "", "Path were the results will be exported.")
	fs.StringVar(&o.namespace, "namespace", "vega", "Namespace where the calculations exists")
	fs.BoolVar(&o.dryRun, "dry-run", true, "Whether to mutate objects in the cluster")

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
	mgr, err := controllerruntime.NewManager(clusterConfig, controllerruntime.Options{DryRunClient: o.dryRun})
	if err != nil {
		logrus.WithError(err).Fatal("failed to construct manager")
	}

	if err := v1.AddToScheme(mgr.GetScheme()); err != nil {
		logrus.WithError(err).Fatal("Failed to add calculationv1 to scheme")
	}

	ctx := controllerruntime.SetupSignalHandler()
	if err := resultcollector.AddToManager(ctx, mgr, o.namespace, o.calculationsDir, o.resultsDir); err != nil {
		logrus.WithError(err).Fatal("Failed to add calculations controller to manager")
	}

	if err := mgr.Start(ctx); err != nil {
		logrus.WithError(err).Fatal("Manager ended with error")
	}
}
