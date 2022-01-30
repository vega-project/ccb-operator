package util

import (
	"sort"

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

func GetFirstCalculationBulk(bulks map[string]workersv1.CalculationBulk) *workersv1.CalculationBulk {
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

	return &ret[0]
}
