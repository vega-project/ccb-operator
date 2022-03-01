package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	ctrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vega-project/ccb-operator/pkg/util"
)

type options struct {
	dryRun     bool
	port       int
	resultsDir string
	namespace  string

	simulator simulator
}

func gatherOptions() options {
	o := options{}
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	fs.IntVar(&o.port, "port", 8080, "Port number where the server will listen to")
	fs.BoolVar(&o.dryRun, "dry-run", true, "Dry run mode with a fake calculation agent")
	fs.StringVar(&o.resultsDir, "calculation-results-dir", "", "Path were the results of the calculations exist.")
	fs.StringVar(&o.namespace, "namespace", "vega", "The namespace where the calculations exist.")
	o.simulator.bind(fs)

	fs.Parse(os.Args[1:])
	return o
}

func main() {
	o := gatherOptions()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	clusterConfig, err := util.LoadClusterConfig()
	if err != nil {
		logrus.WithError(err).Fatal("could not load cluster clusterConfig")
	}

	var c ctrlruntimeclient.Client
	if o.dryRun {
		logrus.Info("Running on dry mode...")
		o.simulator.initialize(ctx)
		if err := o.simulator.startDryRun(); err != nil {
			logrus.WithError(err).Fatal("error while running in dry mode")
		}
		c = o.simulator.fakeClient
	} else {
		c, err = ctrlruntimeclient.New(clusterConfig, ctrlruntimeclient.Options{})
		if err != nil {
			logrus.WithError(err).Fatal("failed to create client")
		}
	}

	s := server{
		logger:      logrus.WithField("component", "apiserver"),
		ctx:         ctx,
		client:      c,
		resultsPath: o.resultsDir,
		namespace:   o.namespace,
	}

	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	r.GET("/calculations", s.getCalculations)
	r.POST("/calculations/create", s.createCalculation)
	r.DELETE("/calculations/delete/:id", s.deleteCalculation)

	r.GET("/calculation", s.getCalculation)
	r.GET("/calculation/:id", s.getCalculationByName)

	r.GET("/bulks", s.getCalculationBulks)
	r.GET("/bulk/:id", s.getCalculationBulkByName)
	r.POST("/bulk/create", s.createCalculationBulk)

	r.GET("/workerpools", s.getWorkerPools)

	r.GET("/calculations/results", s.getCalculationResults)
	r.GET("/calculations/results/:id", s.getCalculationResultsByID)

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "OK"})
	})

	logrus.Infof("Listening on %d port", o.port)
	logrus.Fatal(r.Run(fmt.Sprintf(":%d", o.port)))
}
