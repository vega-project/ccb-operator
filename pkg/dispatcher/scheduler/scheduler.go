package scheduler

import (
	"context"
	"sync"

	"github.com/sirupsen/logrus"
	ctrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/vega-project/ccb-operator/pkg/apis/calculations/v1"
	workersv1 "github.com/vega-project/ccb-operator/pkg/apis/workers/v1"
	"github.com/vega-project/ccb-operator/pkg/util"
)

type Scheduler struct {
	logger        *logrus.Entry
	calculationCh chan v1.Calculation
	client        ctrlruntimeclient.Client
}

func NewScheduler(calculationCh chan v1.Calculation, client ctrlruntimeclient.Client) *Scheduler {
	return &Scheduler{
		logger:        logrus.WithField("component", "scheduler"),
		calculationCh: calculationCh,
		client:        client,
	}
}

func (s *Scheduler) Run(ctx context.Context, stopCh chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		select {
		case calc := <-s.calculationCh:
			workerpool := &workersv1.WorkerPool{}
			if err := s.client.Get(ctx, ctrlruntimeclient.ObjectKey{Namespace: calc.Namespace, Name: calc.WorkerPool}, workerpool); err != nil {
				s.logger.WithError(err).Errorf("failed to get workerpool: %s in namespace %s", calc.WorkerPool, calc.Namespace)
				break
			}

			workerToReserve := util.GetFirstAvailableWorker(workerpool.Spec.Workers)
			if workerToReserve == nil {
				break
			}

			calc.Assign = workerToReserve.Name
			if calc.Labels == nil {
				calc.Labels = make(map[string]string)
			}
			calc.Labels["vegaproject.io/assign"] = workerToReserve.Name

			s.logger.WithField("calc-name", calc.Name).Info("Creating calculation.")
			if err := s.client.Create(ctx, &calc); err != nil {
				s.logger.WithError(err).Error("couldn't create calculation")
				break
			}

			if err := util.UpdateWorkerStatusInPool(ctx, s.client, calc.WorkerPool, workerToReserve.Node, calc.Namespace, workersv1.WorkerReservedState); err != nil {
				s.logger.WithError(err).Error("failed to update worker's state in worker pool")
			}
		case <-stopCh:
			close(s.calculationCh)
			close(stopCh)
			s.logger.Info("terminating scheduler...")
			return
		}
	}
}
