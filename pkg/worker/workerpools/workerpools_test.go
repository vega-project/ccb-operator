package workerpools

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	fakectrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/sirupsen/logrus"
	workersv1 "github.com/vega-project/ccb-operator/pkg/apis/workers/v1"
)

func TestRegisterWorkerInPool(t *testing.T) {
	testCases := []struct {
		name       string
		workerName string
		nodename   string
		workerPool []ctrlruntimeclient.Object
		expected   []workersv1.WorkerPool
	}{
		{
			name:       "basic case, no worker was previously registered",
			workerName: "test-worker",
			nodename:   "test-node-1",
			workerPool: []ctrlruntimeclient.Object{
				&workersv1.WorkerPool{
					ObjectMeta: metav1.ObjectMeta{Name: "vega-workers", Namespace: "vega"},
				},
			},
			expected: []workersv1.WorkerPool{
				{
					TypeMeta:   metav1.TypeMeta{Kind: "WorkerPool", APIVersion: "vegaproject.io/v1"},
					ObjectMeta: metav1.ObjectMeta{Name: "vega-workers", Namespace: "vega"},
					Spec: workersv1.WorkerPoolSpec{
						Workers: map[string]workersv1.Worker{
							"test-node-1": {
								Name:                  "test-worker",
								RegisteredTime:        &metav1.Time{Time: time.Date(1970, time.January, 1, 1, 0, 0, 0, time.Local)},
								LastUpdateTime:        &metav1.Time{Time: time.Date(1970, time.January, 1, 1, 0, 0, 0, time.Local)},
								CalculationsProcessed: 0,
								State:                 workersv1.WorkerAvailableState,
							},
						},
					},
				},
			},
		},
		{
			name:       "basic case, new worker to register",
			workerName: "test-worker",
			nodename:   "test-node-1",
			workerPool: []ctrlruntimeclient.Object{
				&workersv1.WorkerPool{
					ObjectMeta: metav1.ObjectMeta{Name: "vega-workers", Namespace: "vega"},
					Spec: workersv1.WorkerPoolSpec{
						Workers: map[string]workersv1.Worker{
							"test-another-worker": {
								Name:                  "test-another-worker",
								RegisteredTime:        &metav1.Time{Time: time.Date(1970, time.January, 1, 1, 0, 0, 0, time.Local)},
								LastUpdateTime:        &metav1.Time{Time: time.Date(1970, time.January, 1, 1, 0, 0, 0, time.Local)},
								CalculationsProcessed: 0,
								State:                 workersv1.WorkerAvailableState,
							},
							"test-another-worker-2": {
								Name:                  "test-another-worker-2",
								RegisteredTime:        &metav1.Time{Time: time.Date(1970, time.January, 1, 1, 0, 0, 0, time.Local)},
								LastUpdateTime:        &metav1.Time{Time: time.Date(1970, time.January, 1, 1, 0, 0, 0, time.Local)},
								CalculationsProcessed: 0,
								State:                 workersv1.WorkerAvailableState,
							},
						},
					},
				},
			},
			expected: []workersv1.WorkerPool{
				{
					TypeMeta:   metav1.TypeMeta{Kind: "WorkerPool", APIVersion: "vegaproject.io/v1"},
					ObjectMeta: metav1.ObjectMeta{Name: "vega-workers", Namespace: "vega"},
					Spec: workersv1.WorkerPoolSpec{
						Workers: map[string]workersv1.Worker{
							"test-another-worker": {
								Name:                  "test-another-worker",
								RegisteredTime:        &metav1.Time{Time: time.Date(1970, time.January, 1, 1, 0, 0, 0, time.Local)},
								LastUpdateTime:        &metav1.Time{Time: time.Date(1970, time.January, 1, 1, 0, 0, 0, time.Local)},
								CalculationsProcessed: 0,
								State:                 workersv1.WorkerAvailableState,
							},
							"test-another-worker-2": {
								Name:                  "test-another-worker-2",
								RegisteredTime:        &metav1.Time{Time: time.Date(1970, time.January, 1, 1, 0, 0, 0, time.Local)},
								LastUpdateTime:        &metav1.Time{Time: time.Date(1970, time.January, 1, 1, 0, 0, 0, time.Local)},
								CalculationsProcessed: 0,
								State:                 workersv1.WorkerAvailableState,
							},
							"test-node-1": {
								Name:                  "test-worker",
								RegisteredTime:        &metav1.Time{Time: time.Date(1970, time.January, 1, 1, 0, 0, 0, time.Local)},
								LastUpdateTime:        &metav1.Time{Time: time.Date(1970, time.January, 1, 1, 0, 0, 0, time.Local)},
								CalculationsProcessed: 0,
								State:                 workersv1.WorkerAvailableState,
							},
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := fakectrlruntimeclient.NewClientBuilder().WithObjects(tc.workerPool...).Build()
			if err := registerWorkerInPool(context.Background(), logrus.WithField("test-name", tc.name), client, "vega-workers", tc.nodename, tc.workerName, "vega"); err != nil {
				t.Fatal(err)
			}

			var actualWorkerPoolList workersv1.WorkerPoolList
			if err := client.List(context.Background(), &actualWorkerPoolList); err != nil {
				t.Fatal(err)
			}

			reconcileWorkerPoolsForTests(actualWorkerPoolList.Items)
			if diff := cmp.Diff(actualWorkerPoolList.Items, tc.expected, cmpopts.IgnoreFields(metav1.ObjectMeta{}, "ResourceVersion")); diff != "" {
				t.Fatal(diff)
			}

		})
	}
}

func reconcileWorkerPoolsForTests(pools []workersv1.WorkerPool) {
	zeroTime := &metav1.Time{Time: time.Date(1970, time.January, 1, 1, 0, 0, 0, time.Local)}
	for i, pool := range pools {
		for name, worker := range pool.Spec.Workers {
			worker.RegisteredTime = zeroTime
			worker.LastUpdateTime = zeroTime
			pools[i].Spec.Workers[name] = worker
		}
	}
}
