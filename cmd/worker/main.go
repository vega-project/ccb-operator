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
	dryRun                   bool
}

func gatherOptions() options {
	o := options{}
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	fs.StringVar(&o.nfsPath, "nfs-path", "/var/tmp/nfs", "Path of the mounted nfs storage.")
	fs.StringVar(&o.atlasControlFiles, "atlas-control-files-path", "/var/tmp/nfs/atlas-control-files", "Path of the atlas12 control files.")
	fs.StringVar(&o.atlasDataFiles, "atlas-data-files-path", "/var/tmp/nfs/atlas-data-files", "Path of the atlas12 data files.")

	fs.StringVar(&o.kuruzModelTemplateFile, "kuruz-model-template-file", "", "Kuruz model template file.")
	fs.StringVar(&o.synspecInputTemplateFile, "synspec-input-template-file", "", "Synspec input template file.")
	fs.StringVar(&o.namespace, "namespace", "vega", "Namespace where the calculations exists")
	fs.BoolVar(&o.dryRun, "dry-run", true, "")

	fs.Parse(os.Args[1:])
	return o
}

func validateOptions(o options) error {
	if len(o.nfsPath) == 0 {
		return fmt.Errorf("--nfs-path was not provided")
	}

	if len(o.atlasControlFiles) == 0 {
		return fmt.Errorf("--atlas-control-files-path was not provided")
	}

	if len(o.atlasDataFiles) == 0 {
		return fmt.Errorf("--atlas-data-files-path was not provided")
	}

	if len(o.kuruzModelTemplateFile) == 0 {
		return fmt.Errorf("--kuruz-model-template-file was not provided")
	}

	if len(o.synspecInputTemplateFile) == 0 {
		return fmt.Errorf("--synspec-input-template-file was not provided")
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
		logger.WithError(err).Error("could not load cluster clusterConfig")
	}

	stopCh := make(chan struct{})
	defer close(stopCh)

	// Hostname is the same with the pod's name.
	hostname, err := os.Hostname()
	if err != nil {
		logger.WithError(err).Error("couldn't get hostname")
		os.Exit(1)
	}

	ctx := controllerruntime.SetupSignalHandler()

	op := worker.NewMainOperator(ctx, hostname, o.namespace, o.nfsPath, o.atlasControlFiles, o.atlasDataFiles, o.kuruzModelTemplateFile, o.synspecInputTemplateFile, clusterConfig, o.dryRun)

	// Initialize operator
	op.Initialize()

	if err := op.Run(stopCh); err != nil {
		logger.Fatalf("Error starting operator: %s", err.Error())
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
