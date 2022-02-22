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
	workersv1 "github.com/vega-project/ccb-operator/pkg/apis/workers/v1"
)

func TestReconcile(t *testing.T) {
	testCases := []struct {
		name     string
		bulks    []ctrlruntimeclient.Object
		pools    []ctrlruntimeclient.Object
		expected []workersv1.WorkerPool
	}{
		{
			name: "basic case",
			bulks: []ctrlruntimeclient.Object{
				&bulkv1.CalculationBulk{
					ObjectMeta: metav1.ObjectMeta{Name: "test-bulk", Namespace: "vega"},
					WorkerPool: "test-pool",
					Status: bulkv1.CalculationBulkStatus{
						State: bulkv1.CalculationBulkAvailableState,
					},
				},
			},
			pools: []ctrlruntimeclient.Object{
				&workersv1.WorkerPool{
					ObjectMeta: metav1.ObjectMeta{Name: "test-pool", Namespace: "vega"},
				},
			},
			expected: []workersv1.WorkerPool{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "test-pool", Namespace: "vega"},
					Spec: workersv1.WorkerPoolSpec{
						CalculationBulks: map[string]workersv1.CalculationBulk{
							"test-bulk": {Name: "test-bulk", State: bulkv1.CalculationBulkAvailableState},
						}},
				},
			},
		},
		{
			name: "multiple case",
			bulks: []ctrlruntimeclient.Object{
				&bulkv1.CalculationBulk{
					ObjectMeta: metav1.ObjectMeta{Name: "test-bulk", Namespace: "vega"},
					WorkerPool: "test-pool",
					Status: bulkv1.CalculationBulkStatus{
						State: bulkv1.CalculationBulkAvailableState,
					},
				},
			},
			pools: []ctrlruntimeclient.Object{
				&workersv1.WorkerPool{
					ObjectMeta: metav1.ObjectMeta{Name: "test-pool", Namespace: "vega"},
					Spec: workersv1.WorkerPoolSpec{
						CalculationBulks: map[string]workersv1.CalculationBulk{
							"test-bulk-0": {Name: "test-bulk-0", State: bulkv1.CalculationBulkAvailableState},
							"test-bulk-1": {Name: "test-bulk-1", State: bulkv1.CalculationBulkAvailableState},
						},
					},
				},
			},
			expected: []workersv1.WorkerPool{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "test-pool", Namespace: "vega"},
					Spec: workersv1.WorkerPoolSpec{
						CalculationBulks: map[string]workersv1.CalculationBulk{
							"test-bulk-0": {Name: "test-bulk-0", State: bulkv1.CalculationBulkAvailableState},
							"test-bulk-1": {Name: "test-bulk-1", State: bulkv1.CalculationBulkAvailableState},
							"test-bulk":   {Name: "test-bulk", State: bulkv1.CalculationBulkAvailableState},
						}},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := &reconciler{
				logger: logrus.WithField("test-name", tc.name),
				client: fakectrlruntimeclient.NewClientBuilder().WithObjects(append(tc.bulks, tc.pools...)...).Build(),
			}

			req := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: "vega", Name: "test-bulk"}}
			if err := r.reconcile(context.Background(), req, r.logger); err != nil {
				t.Fatal(err)
			}

			var actualPools workersv1.WorkerPoolList
			if err := r.client.List(context.Background(), &actualPools); err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(actualPools.Items, tc.expected,
				cmpopts.IgnoreFields(metav1.TypeMeta{}, "Kind", "APIVersion"),
				cmpopts.IgnoreFields(metav1.ObjectMeta{}, "ResourceVersion")); diff != "" {
				t.Fatal(diff)
			}

		})
	}
}
