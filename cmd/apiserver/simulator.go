package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"time"

	"github.com/sirupsen/logrus"
	ctrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	fakectrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	bulkv1 "github.com/vega-project/ccb-operator/pkg/apis/calculationbulk/v1"
	v1 "github.com/vega-project/ccb-operator/pkg/apis/calculations/v1"
	"github.com/vega-project/ccb-operator/pkg/util"
)

type simulator struct {
	ctx        context.Context
	fakeClient ctrlruntimeclient.WithWatch

	dryCalculationsTotal int
	dryRunFailureRate    int
	dryWorkers           int
	dryTickerMinutes     int

	dryCalcList []ctrlruntimeclient.Object
}

func (s *simulator) bind(fs *flag.FlagSet) {
	fs.IntVar(&s.dryCalculationsTotal, "dry-total-calculations", 100, "Number of total calculations (dry-run)")
	fs.IntVar(&s.dryRunFailureRate, "dry-failure-rate", 20, "Calculations failure rate in percentage (dry-run)")
	fs.IntVar(&s.dryWorkers, "dry-workers", 10, "Number of workers (dry-run)")
	fs.IntVar(&s.dryTickerMinutes, "dry-ticker", 1, "Minutes per calculation update (dry-run)")
}

func (s *simulator) initialize(ctx context.Context) {
	s.ctx = ctx
	// Generate fake calculations
	teff := 10000
	for teff != 10000+s.dryCalculationsTotal {
		teff++
		c := &bulkv1.Calculation{
			Params: v1.Params{
				Teff: float64(teff),
				LogG: 4.0,
			},
		}
		newCalc := util.NewCalculation(c)
		s.dryCalcList = append(s.dryCalcList, newCalc)
	}

	s.fakeClient = fakectrlruntimeclient.NewClientBuilder().WithObjects(s.dryCalcList...).Build()
}

func (s *simulator) startDryRun() error {
	var divided [][]ctrlruntimeclient.Object
	chunkSize := s.dryCalculationsTotal / s.dryWorkers
	for i := 0; i < len(s.dryCalcList); i += chunkSize {
		end := i + chunkSize
		if end > len(s.dryCalcList) {
			end = len(s.dryCalcList)
		}
		divided = append(divided, s.dryCalcList[i:end])
	}

	calcsByWorker := make(map[string]*v1.CalculationList)
	// Assign calculations to workers
	for i, calcList := range divided {
		workerName := fmt.Sprintf("vega-worker-%d", i)
		calculations := &v1.CalculationList{}
		for _, calc := range calcList {
			c, ok := calc.(*v1.Calculation)
			if !ok {
				fmt.Printf("error: %#v\n", c)
				break
			}
			c.Assign = workerName
			calculations.Items = append(calculations.Items, *c)
		}
		calcsByWorker[workerName] = calculations
	}

	for worker, calculations := range calcsByWorker {
		go s.calcsSimulator(calculations, worker)
	}

	return nil
}

func (s *simulator) calcsSimulator(calculations *v1.CalculationList, workerName string) {
	ticker := time.NewTicker(time.Duration(s.dryTickerMinutes) * time.Minute)
	defer ticker.Stop()
	done := make(chan bool)

	go func() {
		watcher, _ := s.fakeClient.Watch(s.ctx, calculations)
		defer watcher.Stop()

		// Watch calculation until Completed or Failed status
		for {
			select {
			case event := <-watcher.ResultChan():
				obj := event.Object.(*v1.Calculation)
				if obj.Phase == v1.CompletedPhase || obj.Phase == v1.FailedPhase {
					done <- true
				}
			}
		}
	}()

	for _, calc := range calculations.Items {
		logger := logrus.WithFields(logrus.Fields{"calculation": calc.Name, "worker": workerName})
		s.simulateRun(calc, done, ticker, logger)
	}
}

func (s *simulator) simulateRun(calc v1.Calculation, done chan bool, ticker *time.Ticker, logger *logrus.Entry) {
	logger.Info("Starting simulation")

	getPhaseWithFailureChance := func(chance int) v1.CalculationPhase {
		r := rand.Intn(100)
		if r < chance {
			return v1.FailedPhase
		}
		return v1.CompletedPhase
	}

	isFinished := func(spec v1.CalculationSpec) bool {
		for _, step := range spec.Steps {
			if step.Status == v1.ProcessingPhase || step.Status == v1.CreatedPhase {
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

			newCalc := &v1.Calculation{}
			err := s.fakeClient.Get(s.ctx, ctrlruntimeclient.ObjectKey{Namespace: calc.Namespace, Name: calc.Name}, newCalc)
			if err != nil {
				logger.WithError(err).Error("couldn't get calculation")
				goto End
			}

			switch newCalc.Phase {
			case v1.CreatedPhase:
				newCalc.Phase = v1.ProcessingPhase
				newCalc.Spec.Steps[0].Status = v1.ProcessingPhase
				break

			case v1.ProcessingPhase:
				if isFinished(newCalc.Spec) {
					newCalc.Phase = v1.CompletedPhase
				}

				for index, step := range newCalc.Spec.Steps {
					switch step.Status {

					case v1.CompletedPhase:
						continue

					case v1.FailedPhase:
						newCalc.Phase = v1.FailedPhase
						goto End

					case v1.ProcessingPhase:
						newCalc.Spec.Steps[index].Status = getPhaseWithFailureChance(s.dryRunFailureRate)
						goto End

					case v1.CreatedPhase:
						newCalc.Spec.Steps[index].Status = v1.ProcessingPhase
						goto End
					}
				}
			}

		End:

			if err := s.fakeClient.Update(s.ctx, newCalc); err != nil {
				logger.WithError(err).Errorf("failed to update calculation %s", newCalc.Name)
			} else {
				logger.Info("Updating calculation")
			}
		}
	}
}
