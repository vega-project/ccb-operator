package workers

import (
	"context"
	"testing"

	"github.com/sirupsen/logrus"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	corev1 "k8s.io/api/core/v1"
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
		name           string
		clusterObjects []ctrlruntimeclient.Object

		expectedCalculations     []v1.Calculation
		expectedWorkerPools      []workersv1.WorkerPool
		expectedCalculationBulks []bulkv1.CalculationBulk
	}{
		{
			name: "basic case, nothing to delete",
			clusterObjects: []ctrlruntimeclient.Object{
				&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "worker-1", Namespace: "vega"}, Status: corev1.PodStatus{Phase: corev1.PodRunning}},
				&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "worker-2", Namespace: "vega"}, Status: corev1.PodStatus{Phase: corev1.PodRunning}},

				&bulkv1.CalculationBulk{
					ObjectMeta:   metav1.ObjectMeta{Name: "test-bulk", Namespace: "vega"},
					Calculations: map[string]bulkv1.Calculation{"calc-test": {Phase: v1.ProcessingPhase}},
				},

				&workersv1.WorkerPool{
					ObjectMeta: metav1.ObjectMeta{Namespace: "vega", Name: "workerpool-test"},
					Spec: workersv1.WorkerPoolSpec{
						Workers: map[string]workersv1.Worker{
							"node-1": {Name: "worker-1", State: workersv1.WorkerProcessingState},
							"node-2": {Name: "worker-2", State: workersv1.WorkerProcessingState},
						},
					},
				},

				&v1.Calculation{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "vega",
						Name:      "calc-1",
						Labels: map[string]string{
							"vegaproject.io/bulk":            "test-bulk",
							"vegaproject.io/calculationName": "calc-test",
						},
					},
					Assign: "worker-1",
				},
			},
			expectedCalculations: []v1.Calculation{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "vega",
						Name:      "calc-1",
						Labels: map[string]string{
							"vegaproject.io/bulk":            "test-bulk",
							"vegaproject.io/calculationName": "calc-test",
						},
					},
					Assign: "worker-1",
				},
			},
			expectedWorkerPools: []workersv1.WorkerPool{
				{
					ObjectMeta: metav1.ObjectMeta{Namespace: "vega", Name: "workerpool-test"},
					Spec: workersv1.WorkerPoolSpec{
						Workers: map[string]workersv1.Worker{
							"node-1": {Name: "worker-1", State: workersv1.WorkerProcessingState},
							"node-2": {Name: "worker-2", State: workersv1.WorkerProcessingState},
						},
					},
				},
			},
			expectedCalculationBulks: []bulkv1.CalculationBulk{
				{
					ObjectMeta:   metav1.ObjectMeta{Name: "test-bulk", Namespace: "vega"},
					Calculations: map[string]bulkv1.Calculation{"calc-test": {Phase: v1.ProcessingPhase}},
				},
			},
		},
		{
			name: "basic case, pod deleted",
			clusterObjects: []ctrlruntimeclient.Object{
				&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "worker-2", Namespace: "vega"}, Status: corev1.PodStatus{Phase: corev1.PodRunning}},

				&bulkv1.CalculationBulk{
					ObjectMeta:   metav1.ObjectMeta{Name: "test-bulk", Namespace: "vega"},
					Calculations: map[string]bulkv1.Calculation{"calc-test": {Phase: v1.ProcessingPhase}},
				},

				&workersv1.WorkerPool{
					ObjectMeta: metav1.ObjectMeta{Namespace: "vega", Name: "workerpool-test"},
					Spec: workersv1.WorkerPoolSpec{
						Workers: map[string]workersv1.Worker{
							"node-1": {Name: "worker-1", State: workersv1.WorkerProcessingState},
							"node-2": {Name: "worker-2", State: workersv1.WorkerProcessingState},
						},
					},
				},

				&v1.Calculation{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "vega",
						Name:      "calc-1",
						Labels: map[string]string{
							"vegaproject.io/bulk":            "test-bulk",
							"vegaproject.io/calculationName": "calc-test",
							"vegaproject.io/assign":          "worker-1",
						},
					},
					Phase:  v1.ProcessingPhase,
					Assign: "worker-1",
				},
			},
			expectedCalculations: []v1.Calculation{},
			expectedWorkerPools: []workersv1.WorkerPool{
				{
					ObjectMeta: metav1.ObjectMeta{Namespace: "vega", Name: "workerpool-test"},
					Spec: workersv1.WorkerPoolSpec{
						Workers: map[string]workersv1.Worker{
							"node-1": {Name: "worker-1", State: workersv1.WorkerUnknownState},
							"node-2": {Name: "worker-2", State: workersv1.WorkerProcessingState},
						},
					},
				},
			},
			expectedCalculationBulks: []bulkv1.CalculationBulk{
				{
					ObjectMeta:   metav1.ObjectMeta{Name: "test-bulk", Namespace: "vega"},
					Calculations: map[string]bulkv1.Calculation{"calc-test": {}},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := &reconciler{
				logger: logrus.WithField("test-name", tc.name),
				client: fakectrlruntimeclient.NewClientBuilder().WithObjects(tc.clusterObjects...).Build(),
			}
			req := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: "vega", Name: "worker-1"}}
			if err := r.reconcile(context.Background(), req, r.logger); err != nil {
				t.Fatal(err)
			}

			var actualWorkerPools workersv1.WorkerPoolList
			if err := r.client.List(context.Background(), &actualWorkerPools); err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(actualWorkerPools.Items, tc.expectedWorkerPools,
				cmpopts.IgnoreFields(metav1.TypeMeta{}, "Kind", "APIVersion"),
				cmpopts.IgnoreFields(metav1.ObjectMeta{}, "ResourceVersion")); diff != "" {
				t.Fatal(diff)
			}

			var actualCalculationBulks bulkv1.CalculationBulkList
			if err := r.client.List(context.Background(), &actualCalculationBulks); err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(actualCalculationBulks.Items, tc.expectedCalculationBulks,
				cmpopts.IgnoreFields(metav1.TypeMeta{}, "Kind", "APIVersion"),
				cmpopts.IgnoreFields(metav1.ObjectMeta{}, "ResourceVersion")); diff != "" {
				t.Fatal(diff)
			}

			var actualCalculations v1.CalculationList
			if err := r.client.List(context.Background(), &actualCalculations); err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(actualCalculations.Items, tc.expectedCalculations,
				cmpopts.IgnoreFields(v1.CalculationStatus{}, "StartTime"),
				cmpopts.IgnoreFields(metav1.ObjectMeta{}, "ResourceVersion")); diff != "" {
				t.Fatal(diff)
			}
		})
	}

}

func TestDeleteAssignedCalculations(t *testing.T) {
	testCases := []struct {
		id           string
		podName      string
		calculations []ctrlruntimeclient.Object
		expected     []v1.Calculation
		errorMsg     string
	}{
		{
			id:      "no calculation to delete",
			podName: "test-pod",
		},
		{
			id:      "one calculation to delete",
			podName: "test-pod",
			calculations: []ctrlruntimeclient.Object{
				&v1.Calculation{
					Assign:     "test-pod",
					ObjectMeta: metav1.ObjectMeta{Name: "test-calc", Labels: map[string]string{"vegaproject.io/assign": "test-pod"}},
					Phase:      v1.ProcessingPhase,
				},
			},
			expected: []v1.Calculation{},
		},
		{
			id:      "more than one calculation, but only one to delete",
			podName: "test-pod",
			calculations: []ctrlruntimeclient.Object{
				&v1.Calculation{
					Assign:     "test-pod",
					ObjectMeta: metav1.ObjectMeta{Name: "test-calc", Labels: map[string]string{"vegaproject.io/assign": "test-pod"}},
					Phase:      v1.ProcessingPhase,
				},
				&v1.Calculation{
					Assign:     "another-pod",
					ObjectMeta: metav1.ObjectMeta{Name: "test-calc-2", Labels: map[string]string{"vegaproject.io/assign": "another-pod"}},
					Phase:      v1.ProcessingPhase,
				},
			},
			expected: []v1.Calculation{
				{
					Assign:     "another-pod",
					ObjectMeta: metav1.ObjectMeta{Name: "test-calc-2", Labels: map[string]string{"vegaproject.io/assign": "another-pod"}},
					Phase:      v1.ProcessingPhase,
				},
			},
		},
	}

	for _, tc := range testCases {
		r := &reconciler{
			logger: logrus.WithField("test-name", tc.id),
			client: fakectrlruntimeclient.NewClientBuilder().WithObjects(tc.calculations...).Build(),
		}

		if err := r.deleteAssignedCalculations(context.Background(), tc.podName); err != nil && len(tc.errorMsg) == 0 {
			t.Fatalf("error wasn't expected: %v", err)
		} else if err == nil && len(tc.errorMsg) > 0 {
			t.Fatal("error was expected, but got nil")
		}

		actualCalculations := &v1.CalculationList{}
		if err := r.client.List(context.Background(), actualCalculations); err != nil {
			t.Fatal(err)
		}

		if diff := cmp.Diff(actualCalculations.Items, tc.expected, cmpopts.IgnoreFields(metav1.ObjectMeta{}, "ResourceVersion")); diff != "" {
			t.Fatal(diff)
		}
	}
}
