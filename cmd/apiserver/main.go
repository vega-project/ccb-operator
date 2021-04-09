package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"

	client "github.com/vega-project/ccb-operator/pkg/client/clientset/versioned"
	"github.com/vega-project/ccb-operator/pkg/client/clientset/versioned/fake"
	v1 "github.com/vega-project/ccb-operator/pkg/client/clientset/versioned/typed/calculations/v1"
	"github.com/vega-project/ccb-operator/pkg/util"
)

type options struct {
	dryCalculationsTotal int
	dryRunFailureRate    int
	dryWorkers           int
	dryTickerMinutes     int

	dryRun bool
	port   int

	simulator simulator
}

func gatherOptions() options {
	o := options{}
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	fs.IntVar(&o.port, "port", 8080, "Port number where the server will listen to")
	fs.BoolVar(&o.dryRun, "dry-run", true, "Dry run mode with a fake calculation agent")

	fs.IntVar(&o.dryCalculationsTotal, "dry-total-calculations", 100, "Number of total calculations (dry-run)")
	fs.IntVar(&o.dryRunFailureRate, "dry-failure-rate", 20, "Calculations failure rate in percentage (dry-run)")
	fs.IntVar(&o.dryWorkers, "dry-workers", 10, "Number of workers (dry-run)")
	fs.IntVar(&o.dryTickerMinutes, "dry-ticker", 1, "Minutes per calculation update (dry-run)")

	fs.Parse(os.Args[1:])
	o.simulator.Bind(fs)
	return o
}

func main() {
	o := gatherOptions()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	clusterConfig, err := util.LoadClusterConfig()
	if err != nil {
		logrus.WithError(err).Error("could not load cluster clusterConfig")
	}

	var c v1.VegaV1Interface

	if o.dryRun {
		o.simulator.ctx = ctx
		logrus.Info("Running on dry mode...")
		fakecs := fake.NewSimpleClientset()
		o.simulator.fakeClient = fakecs.VegaV1()
		if err := o.simulator.startDryRun(); err != nil {
			logrus.WithError(err).Fatal("error while running in dry mode")
		}
		c = fakecs.VegaV1()
	} else {

		vegaClient, err := client.NewForConfig(clusterConfig)
		if err != nil {
			logrus.WithError(err).Error("could not create client")
		}
		c = vegaClient.VegaV1()

	}

	s := server{
		logger: logrus.WithField("component", "apiserver"),
		ctx:    ctx,
		client: c,
	}

	router := mux.NewRouter().StrictSlash(true)
	router.HandleFunc("/calculations", s.getCalculations)
	router.HandleFunc("/calculation/{id}", s.getCalculationByName)
	router.HandleFunc("/calculation", s.getCalculation)
	router.HandleFunc("/calculations/create", s.createCalculation)
	router.HandleFunc("/calculations/delete/{id}", s.deleteCalculation)
	router.HandleFunc("/healthz", func(rw http.ResponseWriter, r *http.Request) {
		rw.WriteHeader(200)
		rw.Write([]byte("OK"))
	})

	logrus.Infof("Listening on %d port", o.port)
	logrus.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", o.port), router))

}
