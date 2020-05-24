package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	calculationsv1 "github.com/vega-project/ccb-operator/pkg/apis/calculations/v1"
	"github.com/vega-project/ccb-operator/pkg/client/clientset/versioned/fake"
	v1 "github.com/vega-project/ccb-operator/pkg/client/clientset/versioned/typed/calculations/v1"
	"github.com/vega-project/ccb-operator/pkg/util"
)

type options struct {
	dryRun bool
	port   int

	client v1.CalculationsV1Interface
}

const (
	// TODO: flag them
	dryCalculationsTotal = 100
	dryRunFailureRate    = 20
	dryWorkers           = 10
	dryTickerMinutes     = 1
)

func gatherOptions() options {
	o := options{}
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	fs.IntVar(&o.port, "port", 8080, "Port number where the server will listen to")
	fs.BoolVar(&o.dryRun, "dry-run", true, "Dry run mode with a fake calculation agent")

	fs.Parse(os.Args[1:])
	return o
}

func (o *options) getCalculations(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	calcList, err := o.client.Calculations().List(metav1.ListOptions{})
	if err != nil {
		fmt.Fprintf(w, "couldn't get calculations list: %v", err)
		return
	}

	json.NewEncoder(w).Encode(calcList)
}

func (o *options) getCalculation(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	calcID := mux.Vars(r)["id"]

	calc, err := o.client.Calculations().Get(calcID, metav1.GetOptions{})
	if err != nil {
		fmt.Fprintf(w, "couldn't get calculation %s: %v", calcID, err)
		return
	}
	json.NewEncoder(w).Encode(calc)
}

func main() {
	o := gatherOptions()

	if o.dryRun {
		logrus.Info("Running on dry mode...")
		fakecs := fake.NewSimpleClientset()
		o.client = fakecs.CalculationsV1()
		if err := dryRun(o.client); err != nil {
			logrus.WithError(err).Fatal("error while running in dry mode")
		}
	}

	router := mux.NewRouter().StrictSlash(true)
	router.HandleFunc("/calculations", o.getCalculations)
	router.HandleFunc("/calculation/{id}", o.getCalculation)

	logrus.Infof("Listening on %d port", o.port)
	logrus.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", o.port), router))

}

func dryRun(fakeClient v1.CalculationsV1Interface) error {
	var dryCalcList []*calculationsv1.Calculation

	// Generate fake calculations
	teff := 10000
	for teff != 10000+dryCalculationsTotal {
		teff++

		calcName := fmt.Sprintf("calc-%s", util.InputHash([]byte(strconv.Itoa(teff)), []byte("4.00")))
		calcSpec := calculationsv1.CalculationSpec{
			Teff: float64(teff),
			LogG: 4.00,
			Steps: []calculationsv1.Step{
				{
					Command: "atlas12_ada",
					Args:    []string{"s"},
					Status:  calculationsv1.CreatedPhase,
				},
				{
					Command: "atlas12_ada",
					Args:    []string{"r"},
					Status:  calculationsv1.CreatedPhase,
				},
				{
					Command: "synspec49",
					Args:    []string{"<", "input_tlusty_fortfive"},
					Status:  calculationsv1.CreatedPhase,
				},
			},
		}

		calc := &calculationsv1.Calculation{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Calculation",
				APIVersion: "vega.io/v1",
			},
			ObjectMeta: metav1.ObjectMeta{Name: calcName},
			DBKey:      fmt.Sprintf("vz.star:teff_%d", teff),
			Phase:      calculationsv1.CreatedPhase,

			Spec: calcSpec,
		}

		dryCalcList = append(dryCalcList, calc)
		logrus.WithField("calculation", calcName).Info("Creating calculation")
		if _, err := fakeClient.Calculations().Create(calc); err != nil {
			return fmt.Errorf("couldn't create calculation: %v", calc)
		}
	}

	var divided [][]*calculationsv1.Calculation
	chunkSize := dryCalculationsTotal / dryWorkers
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
		go calcsSimulator(fakeClient, calcNameList, worker)
	}
	return nil
}

func calcsSimulator(fakeClient v1.CalculationsV1Interface, calcNameList []string, workerName string) {
	for _, calcName := range calcNameList {
		logger := logrus.WithFields(logrus.Fields{"calculation": calcName, "worker": workerName})
		simulateRun(fakeClient, calcName, logger)
	}
}

func simulateRun(fakeClient v1.CalculationsV1Interface, calcName string, logger *logrus.Entry) {
	logger.Info("Starting simulation")

	ticker := time.NewTicker(dryTickerMinutes * time.Minute)
	defer ticker.Stop()
	done := make(chan bool)

	go func() {
		opts := metav1.SingleObject(metav1.ObjectMeta{Name: calcName})
		watcher, _ := fakeClient.Calculations().Watch(opts)
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
			newCalc, err := fakeClient.Calculations().Get(calcName, metav1.GetOptions{})

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
						newCalc.Spec.Steps[index].Status = getPhaseWithFailureChance(dryRunFailureRate)
						goto End

					case calculationsv1.CreatedPhase:
						newCalc.Spec.Steps[index].Status = calculationsv1.ProcessingPhase
						goto End
					}
				}
			}

		End:
			logger.Info("Updating calculation")
			fakeClient.Calculations().Update(newCalc)
		}
	}
}