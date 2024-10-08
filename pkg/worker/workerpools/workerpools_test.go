package workerpools

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	fakectrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/sirupsen/logrus"
	workersv1 "github.com/vega-project/ccb-operator/pkg/apis/workers/v1"
)

func TestReconcile(t *testing.T) {
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
								Node:                  "test-node-1",
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
			nodename:   "test-node-3",
			workerPool: []ctrlruntimeclient.Object{
				&workersv1.WorkerPool{
					ObjectMeta: metav1.ObjectMeta{Name: "vega-workers", Namespace: "vega"},
					Spec: workersv1.WorkerPoolSpec{
						Workers: map[string]workersv1.Worker{
							"test-node-1": {
								Name:                  "test-another-worker",
								Node:                  "test-node-1",
								RegisteredTime:        &metav1.Time{Time: time.Date(1970, time.January, 1, 1, 0, 0, 0, time.Local)},
								LastUpdateTime:        &metav1.Time{Time: time.Date(1970, time.January, 1, 1, 0, 0, 0, time.Local)},
								CalculationsProcessed: 0,
								State:                 workersv1.WorkerAvailableState,
							},
							"test-node-2": {
								Name:                  "test-another-worker-2",
								Node:                  "test-node-2",
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
							"test-node-1": {
								Name:                  "test-another-worker",
								Node:                  "test-node-1",
								RegisteredTime:        &metav1.Time{Time: time.Date(1970, time.January, 1, 1, 0, 0, 0, time.Local)},
								LastUpdateTime:        &metav1.Time{Time: time.Date(1970, time.January, 1, 1, 0, 0, 0, time.Local)},
								CalculationsProcessed: 0,
								State:                 workersv1.WorkerAvailableState,
							},
							"test-node-2": {
								Name:                  "test-another-worker-2",
								Node:                  "test-node-2",
								RegisteredTime:        &metav1.Time{Time: time.Date(1970, time.January, 1, 1, 0, 0, 0, time.Local)},
								LastUpdateTime:        &metav1.Time{Time: time.Date(1970, time.January, 1, 1, 0, 0, 0, time.Local)},
								CalculationsProcessed: 0,
								State:                 workersv1.WorkerAvailableState,
							},
							"test-node-3": {
								Name:                  "test-worker",
								Node:                  "test-node-3",
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
			r := &reconciler{
				logger:     logrus.WithField("test-name", tc.name),
				client:     fakectrlruntimeclient.NewClientBuilder().WithObjects(tc.workerPool...).Build(),
				hostname:   tc.workerName,
				nodename:   tc.nodename,
				namespace:  "vega",
				workerPool: "vega-workers",
			}
			req := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: "vega", Name: "vega-workers"}}
			if err := r.reconcile(context.Background(), req, r.logger); err != nil {
				t.Fatal(err)
			}

			var actualWorkerPoolList workersv1.WorkerPoolList
			if err := r.client.List(context.Background(), &actualWorkerPoolList); err != nil {
				t.Fatal(err)
			}

			reconcileWorkerPoolsForTests(actualWorkerPoolList.Items)
			if diff := cmp.Diff(actualWorkerPoolList.Items, tc.expected, cmpopts.IgnoreFields(metav1.ObjectMeta{}, "ResourceVersion"), cmpopts.IgnoreFields(metav1.TypeMeta{}, "APIVersion", "Kind")); diff != "" {
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
