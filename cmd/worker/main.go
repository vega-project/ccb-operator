package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/sirupsen/logrus"

	controllerruntime "sigs.k8s.io/controller-runtime"

	"github.com/vega-project/ccb-operator/pkg/util"
	"github.com/vega-project/ccb-operator/pkg/worker"
)

type options struct {
	nfsPath                  string
	atlasControlFiles        string
	atlasDataFiles           string
	kuruzModelTemplateFile   string
	synspecInputTemplateFile string
	namespace                string
	workerPool               string
	nodename                 string
	dryRun                   bool
}

func gatherOptions() options {
	o := options{}
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	fs.StringVar(&o.nfsPath, "nfs-path", "/var/tmp/nfs", "Path of the mounted nfs storage.")
	fs.StringVar(&o.namespace, "namespace", "vega", "Namespace where the calculations exists")
	fs.StringVar(&o.nodename, "nodename", "", "The name of the node in which the worker is running")
	fs.StringVar(&o.workerPool, "worker-pool", "vega-workers", "The pool where the worker will post the status updates")
	fs.BoolVar(&o.dryRun, "dry-run", true, "")

	if err := fs.Parse(os.Args[1:]); err != nil {
		logrus.WithError(err).Fatal("couldn't parse options")
	}
	return o
}

func validateOptions(o options) error {
	if len(o.nfsPath) == 0 {
		return fmt.Errorf("--nfs-path was not provided")
	}

	if len(o.nodename) == 0 {
		return fmt.Errorf("--nodename was not provided")
	}

	return nil
}

func main() {
	logrus.SetFormatter(&logrus.TextFormatter{
		DisableQuote: true,
	})

	logger := logrus.New()

	o := gatherOptions()
	err := validateOptions(o)
	if err != nil {
		logger.WithError(err).Fatal("invalid options")
		os.Exit(1)
	}

	clusterConfig, err := util.LoadClusterConfig()
	if err != nil {
		logger.WithError(err).Fatal("could not load cluster clusterConfig")
	}

	stopCh := make(chan struct{})
	defer close(stopCh)

	// Hostname is the same with the pod's name.
	hostname, err := os.Hostname()
	if err != nil {
		logger.WithError(err).Fatal("couldn't get hostname")
	}

	ctx := controllerruntime.SetupSignalHandler()

	op := worker.NewMainOperator(ctx, hostname, o.nodename, o.namespace, o.workerPool, o.nfsPath, clusterConfig, o.dryRun)
	if err := op.Initialize(); err != nil {
		logger.WithError(err).Fatal("couldn't initialize operator")
	}

	if err := op.Run(stopCh); err != nil {
		logger.WithError(err).Fatal("Error starting operator")
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
