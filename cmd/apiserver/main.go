package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	calculationsv1 "github.com/vega-project/ccb-operator/pkg/apis/calculations/v1"
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
	return o
}

func (o *options) startDryRun(ctx context.Context, fakeClient v1.VegaV1Interface) error {
	var dryCalcList []*calculationsv1.Calculation

	// Generate fake calculations
	teff := 10000
	for teff != 10000+o.dryCalculationsTotal {
		teff++

		newCalc := util.NewCalculation(float64(teff), 4.0)
		newCalc.DBKey = fmt.Sprintf("vz.star:teff_%d", teff)

		dryCalcList = append(dryCalcList, newCalc)

		logrus.WithFields(logrus.Fields{"calculation": newCalc.Name, "teff": teff, "logG": "4.0"}).Info("Creating calculation")
		if _, err := fakeClient.Calculations().Create(ctx, newCalc, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("couldn't create calculation: %v", newCalc)
		}
	}

	var divided [][]*calculationsv1.Calculation
	chunkSize := o.dryCalculationsTotal / o.dryWorkers
	for i := 0; i < len(dryCalcList); i += chunkSize {
		end := i + chunkSize
		if end > len(dryCalcList) {
			end = len(dryCalcList)
		}
		divided = append(divided, dryCalcList[i:end])
	}

	calcsByWorker := make(map[string][]string)
	// Assign calculations to workers
	for i, calcList := range divided {
		workerName := fmt.Sprintf("vega-worker-%d", i)
		var calcNameList []string
		for z, calc := range calcList {
			calcList[z].Assign = workerName
			calcNameList = append(calcNameList, calc.Name)
		}
		calcsByWorker[workerName] = calcNameList
	}

	for worker, calcNameList := range calcsByWorker {
		go o.calcsSimulator(ctx, fakeClient, calcNameList, worker)
	}
	return nil
}

func (o *options) calcsSimulator(ctx context.Context, fakeClient v1.VegaV1Interface, calcNameList []string, workerName string) {
	for _, calcName := range calcNameList {
		logger := logrus.WithFields(logrus.Fields{"calculation": calcName, "worker": workerName})
		o.simulateRun(ctx, fakeClient, calcName, logger)
	}
}

func (o *options) simulateRun(ctx context.Context, fakeClient v1.VegaV1Interface, calcName string, logger *logrus.Entry) {
	logger.Info("Starting simulation")

	ticker := time.NewTicker(time.Duration(o.dryTickerMinutes) * time.Minute)
	defer ticker.Stop()
	done := make(chan bool)

	go func() {
		opts := metav1.SingleObject(metav1.ObjectMeta{Name: calcName})
		watcher, _ := fakeClient.Calculations().Watch(ctx, opts)
		defer watcher.Stop()

		// Watch calculation until Completed or Failed status
		for {
			select {
			case event, _ := <-watcher.ResultChan():
				obj := event.Object.(*calculationsv1.Calculation)
				if obj.Phase == calculationsv1.CompletedPhase || obj.Phase == calculationsv1.FailedPhase {
					done <- true
				}
			}
		}
	}()

	getPhaseWithFailureChance := func(chance int) calculationsv1.CalculationPhase {
		r := rand.Intn(100)
		if r < chance {
			return calculationsv1.FailedPhase
		}
		return calculationsv1.CompletedPhase
	}

	isFinished := func(spec calculationsv1.CalculationSpec) bool {
		for _, step := range spec.Steps {
			if step.Status == calculationsv1.ProcessingPhase || step.Status == calculationsv1.CreatedPhase {
				return false
			}
		}
		return true
	}

	for {
		select {
		case <-done:
			logger.Warn("Simulation finished")
			return
		case <-ticker.C:
			newCalc, err := fakeClient.Calculations().Get(ctx, calcName, metav1.GetOptions{})

			if err != nil {
				logger.WithError(err).Error("couldn't get calculation")
				goto End
			}

			switch newCalc.Phase {
			case calculationsv1.CreatedPhase:
				newCalc.Phase = calculationsv1.ProcessingPhase
				newCalc.Spec.Steps[0].Status = calculationsv1.ProcessingPhase
				break

			case calculationsv1.ProcessingPhase:
				if isFinished(newCalc.Spec) {
					newCalc.Phase = calculationsv1.CompletedPhase
				}

				for index, step := range newCalc.Spec.Steps {
					switch step.Status {

					case calculationsv1.CompletedPhase:
						continue

					case calculationsv1.FailedPhase:
						newCalc.Phase = calculationsv1.FailedPhase
						goto End

					case calculationsv1.ProcessingPhase:
						newCalc.Spec.Steps[index].Status = getPhaseWithFailureChance(o.dryRunFailureRate)
						goto End

					case calculationsv1.CreatedPhase:
						newCalc.Spec.Steps[index].Status = calculationsv1.ProcessingPhase
						goto End
					}
				}
			}

		End:
			logger.Info("Updating calculation")
			fakeClient.Calculations().Update(ctx, newCalc, metav1.UpdateOptions{})
		}
	}
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
		logrus.Info("Running on dry mode...")
		fakecs := fake.NewSimpleClientset()
		if err := o.startDryRun(ctx, fakecs.VegaV1()); err != nil {
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
