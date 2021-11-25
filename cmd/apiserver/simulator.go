package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"time"

	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	calculationsv1 "github.com/vega-project/ccb-operator/pkg/apis/calculations/v1"
	v1 "github.com/vega-project/ccb-operator/pkg/client/clientset/versioned/typed/calculations/v1"
	"github.com/vega-project/ccb-operator/pkg/util"
)

type simulator struct {
	ctx        context.Context
	fakeClient v1.VegaV1Interface

	dryCalculationsTotal int
	dryRunFailureRate    int
	dryWorkers           int
	dryTickerMinutes     int
}

func (s *simulator) Bind(fs *flag.FlagSet) {
	fs.IntVar(&s.dryCalculationsTotal, "dry-total-calculations", 100, "Number of total calculations (dry-run)")
	fs.IntVar(&s.dryRunFailureRate, "dry-failure-rate", 20, "Calculations failure rate in percentage (dry-run)")
	fs.IntVar(&s.dryWorkers, "dry-workers", 10, "Number of workers (dry-run)")
	fs.IntVar(&s.dryTickerMinutes, "dry-ticker", 1, "Minutes per calculation update (dry-run)")
}

func (s *simulator) startDryRun() error {
	var dryCalcList []*calculationsv1.Calculation

	// Generate fake calculations
	teff := 10000
	for teff != 10000+s.dryCalculationsTotal {
		teff++

		newCalc := util.NewCalculation(float64(teff), 4.0)

		dryCalcList = append(dryCalcList, newCalc)

		logrus.WithFields(logrus.Fields{"calculation": newCalc.Name, "teff": teff, "logG": "4.0"}).Info("Creating calculation")
		if _, err := s.fakeClient.Calculations().Create(s.ctx, newCalc, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("couldn't create calculation: %v", newCalc)
		}
	}

	var divided [][]*calculationsv1.Calculation
	chunkSize := s.dryCalculationsTotal / s.dryWorkers
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
		go s.calcsSimulator(calcNameList, worker)
	}
	return nil
}

func (s *simulator) calcsSimulator(calcNameList []string, workerName string) {
	for _, calcName := range calcNameList {
		logger := logrus.WithFields(logrus.Fields{"calculation": calcName, "worker": workerName})
		s.simulateRun(calcName, logger)
	}
}

func (s *simulator) simulateRun(calcName string, logger *logrus.Entry) {
	logger.Info("Starting simulation")

	ticker := time.NewTicker(time.Duration(s.dryTickerMinutes) * time.Minute)
	defer ticker.Stop()
	done := make(chan bool)

	go func() {
		opts := metav1.SingleObject(metav1.ObjectMeta{Name: calcName})
		watcher, _ := s.fakeClient.Calculations().Watch(s.ctx, opts)
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
			newCalc, err := s.fakeClient.Calculations().Get(s.ctx, calcName, metav1.GetOptions{})

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
						newCalc.Spec.Steps[index].Status = getPhaseWithFailureChance(s.dryRunFailureRate)
						goto End

					case calculationsv1.CreatedPhase:
						newCalc.Spec.Steps[index].Status = calculationsv1.ProcessingPhase
						goto End
					}
				}
			}

		End:
			logger.Info("Updating calculation")
			s.fakeClient.Calculations().Update(s.ctx, newCalc, metav1.UpdateOptions{})
		}
	}
}
