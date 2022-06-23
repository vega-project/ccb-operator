package bulks

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/sirupsen/logrus"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	fakectrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	bulkv1 "github.com/vega-project/ccb-operator/pkg/apis/calculationbulk/v1"
	v1 "github.com/vega-project/ccb-operator/pkg/apis/calculations/v1"
	workersv1 "github.com/vega-project/ccb-operator/pkg/apis/workers/v1"
)

func TestReconcile(t *testing.T) {
	testCases := []struct {
		name             string
		initialResources []ctrlruntimeclient.Object
		expected         []ctrlruntimeclient.Object
	}{
		{
			name: "basic case",
			initialResources: []ctrlruntimeclient.Object{
				&bulkv1.CalculationBulk{
					ObjectMeta:   metav1.ObjectMeta{Name: "test-bulk", Namespace: "vega"},
					WorkerPool:   "workerpool-test",
					Calculations: map[string]bulkv1.Calculation{"test-calc": {Params: v1.Params{Teff: 10000.0, LogG: 4.0}}},
				},
				&workersv1.WorkerPool{
					ObjectMeta: metav1.ObjectMeta{Namespace: "vega", Name: "workerpool-test"},
					Spec: workersv1.WorkerPoolSpec{
						Workers: map[string]workersv1.Worker{
							"node-1": {Name: "worker-1", Node: "node-1", State: workersv1.WorkerAvailableState},
							"node-2": {Name: "worker-2", Node: "node-2", State: workersv1.WorkerAvailableState},
							"node-3": {Name: "worker-3", Node: "node-3", State: workersv1.WorkerAvailableState},
						},
					},
				},
			},
			expected: []ctrlruntimeclient.Object{
				&workersv1.WorkerPool{
					ObjectMeta: metav1.ObjectMeta{Namespace: "vega", Name: "workerpool-test"},
					Spec: workersv1.WorkerPoolSpec{
						Workers: map[string]workersv1.Worker{
							"node-1": {Name: "worker-1", Node: "node-1", State: workersv1.WorkerReservedState},
							"node-2": {Name: "worker-2", Node: "node-2", State: workersv1.WorkerAvailableState},
							"node-3": {Name: "worker-3", Node: "node-3", State: workersv1.WorkerAvailableState},
						},
					},
				},
			},
		},
		{
			name: "multiple case - less calculations",
			initialResources: []ctrlruntimeclient.Object{
				&bulkv1.CalculationBulk{
					ObjectMeta: metav1.ObjectMeta{Name: "test-bulk", Namespace: "vega"},
					WorkerPool: "workerpool-test",
					Calculations: map[string]bulkv1.Calculation{
						"test-calc":   {Params: v1.Params{Teff: 10000.0, LogG: 4.0}},
						"test-calc-2": {Params: v1.Params{Teff: 11000.0, LogG: 4.0}},
					},
				},
				&workersv1.WorkerPool{
					ObjectMeta: metav1.ObjectMeta{Namespace: "vega", Name: "workerpool-test"},
					Spec: workersv1.WorkerPoolSpec{
						Workers: map[string]workersv1.Worker{
							"node-1": {Name: "worker-1", Node: "node-1", State: workersv1.WorkerAvailableState},
							"node-2": {Name: "worker-2", Node: "node-2", State: workersv1.WorkerAvailableState},
							"node-3": {Name: "worker-3", Node: "node-3", State: workersv1.WorkerAvailableState},
						},
					},
				},
			},
			expected: []ctrlruntimeclient.Object{
				&workersv1.WorkerPool{
					ObjectMeta: metav1.ObjectMeta{Namespace: "vega", Name: "workerpool-test"},
					Spec: workersv1.WorkerPoolSpec{
						Workers: map[string]workersv1.Worker{
							"node-1": {Name: "worker-1", Node: "node-1", State: workersv1.WorkerReservedState},
							"node-2": {Name: "worker-2", Node: "node-2", State: workersv1.WorkerReservedState},
							"node-3": {Name: "worker-3", Node: "node-3", State: workersv1.WorkerAvailableState},
						},
					},
				},
			},
		},
		{
			name: "multiple case",
			initialResources: []ctrlruntimeclient.Object{
				&bulkv1.CalculationBulk{
					ObjectMeta: metav1.ObjectMeta{Name: "test-bulk", Namespace: "vega"},
					WorkerPool: "workerpool-test",
					Calculations: map[string]bulkv1.Calculation{
						"test-calc":   {Params: v1.Params{Teff: 10000.0, LogG: 4.0}},
						"test-calc-2": {Params: v1.Params{Teff: 11000.0, LogG: 4.0}},
						"test-calc-3": {Params: v1.Params{Teff: 12000.0, LogG: 4.0}},
						"test-calc-4": {Params: v1.Params{Teff: 13000.0, LogG: 4.0}},
						"test-calc-5": {Params: v1.Params{Teff: 14000.0, LogG: 4.0}},
					},
				},
				&workersv1.WorkerPool{
					ObjectMeta: metav1.ObjectMeta{Namespace: "vega", Name: "workerpool-test"},
					Spec: workersv1.WorkerPoolSpec{
						Workers: map[string]workersv1.Worker{
							"node-1": {Name: "worker-1", Node: "node-1", State: workersv1.WorkerAvailableState},
							"node-2": {Name: "worker-2", Node: "node-2", State: workersv1.WorkerAvailableState},
							"node-3": {Name: "worker-3", Node: "node-3", State: workersv1.WorkerAvailableState},
						},
					},
				},
			},
			expected: []ctrlruntimeclient.Object{
				&workersv1.WorkerPool{
					ObjectMeta: metav1.ObjectMeta{Namespace: "vega", Name: "workerpool-test"},
					Spec: workersv1.WorkerPoolSpec{
						Workers: map[string]workersv1.Worker{
							"node-1": {Name: "worker-1", Node: "node-1", State: workersv1.WorkerReservedState},
							"node-2": {Name: "worker-2", Node: "node-2", State: workersv1.WorkerReservedState},
							"node-3": {Name: "worker-3", Node: "node-3", State: workersv1.WorkerReservedState},
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := &reconciler{
				logger: logrus.WithField("test-name", tc.name),
				client: fakectrlruntimeclient.NewClientBuilder().WithObjects(tc.initialResources...).Build(),
			}

			req := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: "vega", Name: "test-bulk"}}
			if err := r.reconcile(context.Background(), req, r.logger); err != nil {
				t.Fatal(err)
			}

			var actualBulk bulkv1.CalculationBulk
			if err := r.client.Get(context.Background(), ctrlruntimeclient.ObjectKey{Namespace: req.Namespace, Name: req.Name}, &actualBulk); err != nil {
				t.Fatal(err)
			}

			var actualWorkerPool workersv1.WorkerPool
			if err := r.client.Get(context.Background(), ctrlruntimeclient.ObjectKey{Namespace: req.Namespace, Name: "workerpool-test"}, &actualWorkerPool); err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff([]ctrlruntimeclient.Object{&actualWorkerPool}, tc.expected,
				cmpopts.IgnoreFields(metav1.TypeMeta{}, "Kind", "APIVersion"),
				cmpopts.IgnoreFields(metav1.ObjectMeta{}, "ResourceVersion"),
				cmpopts.IgnoreFields(workersv1.Worker{}, "LastUpdateTime")); diff != "" {
				t.Fatal(diff)
			}

		})
	}
}
