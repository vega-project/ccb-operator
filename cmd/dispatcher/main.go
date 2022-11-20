package main

import (
	"flag"
	"net/http"
	"os"
	"sync"

	corev1 "k8s.io/api/core/v1"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	controllerruntime "sigs.k8s.io/controller-runtime"

	v1 "github.com/vega-project/ccb-operator/pkg/apis/calculations/v1"
	"github.com/vega-project/ccb-operator/pkg/dispatcher/bulks"
	"github.com/vega-project/ccb-operator/pkg/dispatcher/calculations"
	"github.com/vega-project/ccb-operator/pkg/dispatcher/factory"
	"github.com/vega-project/ccb-operator/pkg/dispatcher/scheduler"
	"github.com/vega-project/ccb-operator/pkg/dispatcher/workers"
	"github.com/vega-project/ccb-operator/pkg/util"
)

type options struct {
	namespace string
	dryRun    bool
	nfsPath   string
}

func gatherOptions() (options, error) {
	o := options{}
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	fs.StringVar(&o.namespace, "namespace", "vega", "Namespace where the calculations exists.")
	fs.BoolVar(&o.dryRun, "dry-run", true, "Whether to mutate the objects.")
	fs.StringVar(&o.nfsPath, "nfs-path", "/var/tmp/nfs", "Path of the mounted nfs storage.")

	if err := fs.Parse(os.Args[1:]); err != nil {
		return o, err
	}
	return o, nil
}

func main() {
	logger := logrus.New()
	o, err := gatherOptions()
	if err != nil {
		logger.WithError(err).Fatal("invalid options")
	}

	clusterConfig, err := util.LoadClusterConfig()
	if err != nil {
		logger.WithError(err).Fatal("could not load cluster clusterConfig")
	}

	go func() {
		http.Handle("/metrics", promhttp.Handler())
		http.ListenAndServe(":3001", nil)
	}()

	calculationCh := make(chan v1.Calculation)
	stop := make(chan struct{})
	wg := &sync.WaitGroup{}

	mgr, err := controllerruntime.NewManager(clusterConfig, controllerruntime.Options{DryRunClient: o.dryRun})
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

	if err := bulks.AddToManager(ctx, mgr, o.namespace, calculationCh); err != nil {
		logrus.WithError(err).Fatal("Failed to add workerpools controller to manager")
	}

	if err := factory.AddToManager(ctx, mgr, o.namespace, calculationCh, o.nfsPath); err != nil {
		logrus.WithError(err).Fatal("Failed to add workerpools controller to manager")
	}

	sched := scheduler.NewScheduler(calculationCh, mgr.GetClient())

	wg.Add(1)
	go sched.Run(ctx, stop, wg)

	if err := mgr.Start(ctx); err != nil {
		logrus.WithError(err).Error("Manager ended with error")
		stop <- struct{}{}
		wg.Wait()
	}
}
