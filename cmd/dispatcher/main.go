package main

import (
	"flag"
	"net/http"
	"os"

	corev1 "k8s.io/api/core/v1"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"

	v1 "github.com/vega-project/ccb-operator/pkg/apis/calculations/v1"
	"github.com/vega-project/ccb-operator/pkg/dispatcher/bulks"
	"github.com/vega-project/ccb-operator/pkg/dispatcher/calculations"
	"github.com/vega-project/ccb-operator/pkg/dispatcher/factory"
	"github.com/vega-project/ccb-operator/pkg/dispatcher/workers"
	"github.com/vega-project/ccb-operator/pkg/grpc"
	"github.com/vega-project/ccb-operator/pkg/util"
)

type options struct {
	namespace         string
	nfsPath           string
	grpcClientOptions grpc.Options
}

func gatherOptions() (options, error) {
	o := options{}
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	fs.StringVar(&o.namespace, "namespace", "vega", "Namespace where the calculations exists.")
	fs.StringVar(&o.nfsPath, "nfs-path", "/var/tmp/nfs", "Path of the mounted nfs storage.")
	o.grpcClientOptions.Bind(fs)

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
		if err := http.ListenAndServe(":3001", nil); err != nil {
			logger.WithError(err).Error("couldn't start the metrics http server")
		}
	}()

	calculationCh := make(chan v1.Calculation)
	cacheOpts := cache.Options{DefaultNamespaces: map[string]cache.Config{o.namespace: {}}}
	mgr, err := controllerruntime.NewManager(clusterConfig, controllerruntime.Options{Cache: cacheOpts})
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

	grpcClient, err := grpc.NewClient(o.grpcClientOptions.Address())
	if err != nil {
		logrus.WithError(err).Fatal("failed to construct grpc client")
	}

	if err := bulks.AddToManager(ctx, mgr, o.namespace, calculationCh, grpcClient); err != nil {
		logrus.WithError(err).Fatal("Failed to add workerpools controller to manager")
	}

	if err := factory.AddToManager(ctx, mgr, o.namespace, calculationCh, o.nfsPath); err != nil {
		logrus.WithError(err).Fatal("Failed to add workerpools controller to manager")
	}

	if err := mgr.Start(ctx); err != nil {
		logrus.WithError(err).Error("Manager ended with error")
	}
}
