package util

import (
	"context"
	"fmt"
	"sort"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"

	ctrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/vega-project/ccb-operator/pkg/apis/calculationbulk/v1"
	workersv1 "github.com/vega-project/ccb-operator/pkg/apis/workers/v1"
)

func SortWorkers(workers map[string]workersv1.Worker) []workersv1.Worker {
	var ret []workersv1.Worker

	for _, v := range workers {
		ret = append(ret, v)

	}
	sort.Slice(ret, func(i, j int) bool {
		return ret[i].LastUpdateTime.Before(ret[j].LastUpdateTime)
	})
	return ret
}

func GetFirstAvailableWorker(workers map[string]workersv1.Worker) *workersv1.Worker {
	for _, worker := range SortWorkers(workers) {
		if worker.State == workersv1.WorkerAvailableState {
			return &worker
		}
	}
	return nil
}

func GetCalculationBulksByRegisteredTime(bulks map[string]workersv1.CalculationBulk) []workersv1.CalculationBulk {
	var ret []workersv1.CalculationBulk

	for _, bulk := range bulks {
		if bulk.State == v1.CalculationBulkAvailableState {
			ret = append(ret, bulk)
		}
	}
	sort.Slice(ret, func(i, j int) bool {
		return ret[i].RegisteredTime.Before(ret[j].RegisteredTime)
	})

	if len(ret) == 0 {
		return nil
	}

	return ret
}

func UpdateWorkerStatusInPool(ctx context.Context, client ctrlruntimeclient.Client, workerPool, nodename, namespace string, state workersv1.WorkerState) error {
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		pool := &workersv1.WorkerPool{}
		err := client.Get(ctx, ctrlruntimeclient.ObjectKey{Namespace: namespace, Name: workerPool}, pool)
		if err != nil {
			return fmt.Errorf("failed to get workerpool %s in namespace %s: %w", workerPool, namespace, err)
		}

		now := time.Now()
		worker, exists := pool.Spec.Workers[nodename]
		if exists {
			if worker.LastUpdateTime != nil {
				worker.LastUpdateTime.Time = now
			} else {
				worker.LastUpdateTime = &metav1.Time{Time: now}
			}
			worker.State = state
		}

		pool.Spec.Workers[nodename] = worker
		if err := client.Update(ctx, pool); err != nil {
			return fmt.Errorf("failed to update WorkerPool %s: %w", pool.Name, err)
		}
		return nil
	}); err != nil {
		return err
	}

	return nil
}
