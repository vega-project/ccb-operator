package workerpools

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/sirupsen/logrus"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"

	"k8s.io/apimachinery/pkg/types"

	ctrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	fakectrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	bulkv1 "github.com/vega-project/ccb-operator/pkg/apis/calculationbulk/v1"
	v1 "github.com/vega-project/ccb-operator/pkg/apis/calculations/v1"
	workersv1 "github.com/vega-project/ccb-operator/pkg/apis/workers/v1"
)

func init() {
	if err := workersv1.AddToScheme(scheme.Scheme); err != nil {
		panic(fmt.Sprintf("failed to register scheme: %v", err))
	}

	if err := v1.AddToScheme(scheme.Scheme); err != nil {
		panic(fmt.Sprintf("failed to register scheme: %v", err))
	}

	if err := bulkv1.AddToScheme(scheme.Scheme); err != nil {
		panic(fmt.Sprintf("failed to register scheme: %v", err))
	}
}

func TestReconcile(t *testing.T) {
	testCases := []struct {
		name                 string
		workerpools          []ctrlruntimeclient.Object
		calculationBulks     []ctrlruntimeclient.Object
		expectedCalculations []v1.Calculation
	}{
		{
			name: "basic case, no free worker",
			calculationBulks: []ctrlruntimeclient.Object{
				&bulkv1.CalculationBulk{
					ObjectMeta:   metav1.ObjectMeta{Name: "test-bulk"},
					Calculations: map[string]bulkv1.Calculation{"test-calc": {Params: bulkv1.Params{Teff: 10000.0, LogG: 4.0}}},
				},
			},
			workerpools: []ctrlruntimeclient.Object{
				&workersv1.WorkerPool{
					ObjectMeta: metav1.ObjectMeta{Namespace: "test-namespace", Name: "workerpool-test"},
					Spec: workersv1.WorkerPoolSpec{
						Workers: map[string]workersv1.Worker{
							"node-1": {Name: "worker-1", State: workersv1.WorkerProcessingState},
							"node-2": {Name: "worker-2", State: workersv1.WorkerProcessingState},
							"node-3": {Name: "worker-3", State: workersv1.WorkerProcessingState},
						},
					},
				},
			},
		},
		{
			name: "basic case, one free worker",
			calculationBulks: []ctrlruntimeclient.Object{
				&bulkv1.CalculationBulk{
					ObjectMeta:   metav1.ObjectMeta{Name: "test-bulk"},
					Calculations: map[string]bulkv1.Calculation{"test-calc": {Params: bulkv1.Params{Teff: 10000.0, LogG: 4.0}}},
				},
			},
			workerpools: []ctrlruntimeclient.Object{
				&workersv1.WorkerPool{
					ObjectMeta: metav1.ObjectMeta{Namespace: "test-namespace", Name: "workerpool-test"},
					Spec: workersv1.WorkerPoolSpec{
						Workers: map[string]workersv1.Worker{
							"worker-1": {Name: "worker-1", State: workersv1.WorkerAvailableState, LastUpdateTime: &metav1.Time{Time: time.Date(2022, 1, 1, 12, 0, 0, 0, time.UTC)}},
							"node-2":   {Name: "worker-2", State: workersv1.WorkerProcessingState},
							"node-3":   {Name: "worker-3", State: workersv1.WorkerProcessingState},
						},
					},
				},
			},
			expectedCalculations: []v1.Calculation{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "calc-xc864fxvd5xccn6x",
						Namespace: "test-namespace",
						Labels: map[string]string{
							"vegaproject.io/bulk":            "test-bulk",
							"vegaproject.io/calculationName": "test-calc",
							"vegaproject.io/assign":          "worker-1",
						},
					},
					Assign: "worker-1",
					Phase:  "Created",
					Spec: v1.CalculationSpec{
						Teff: 10000,
						LogG: 4,
						Steps: []v1.Step{
							{Command: "atlas12_ada", Args: []string{"s"}},
							{Command: "atlas12_ada", Args: []string{"r"}},
							{Command: "synspec49", Args: []string{"<", "input_tlusty_fortfive"}},
						},
					},
				},
			},
		},

		{
			name: "basic case, multiple free workers",
			calculationBulks: []ctrlruntimeclient.Object{
				&bulkv1.CalculationBulk{
					ObjectMeta:   metav1.ObjectMeta{Name: "test-bulk"},
					Calculations: map[string]bulkv1.Calculation{"test-calc": {Params: bulkv1.Params{Teff: 10000.0, LogG: 4.0}}},
				},
			},
			workerpools: []ctrlruntimeclient.Object{
				&workersv1.WorkerPool{
					ObjectMeta: metav1.ObjectMeta{Namespace: "test-namespace", Name: "workerpool-test"},
					Spec: workersv1.WorkerPoolSpec{
						Workers: map[string]workersv1.Worker{
							"node-1": {Name: "worker-1", State: workersv1.WorkerAvailableState, LastUpdateTime: &metav1.Time{Time: time.Date(2022, 1, 1, 12, 0, 0, 0, time.UTC)}},
							"node-2": {Name: "worker-2", State: workersv1.WorkerAvailableState, LastUpdateTime: &metav1.Time{Time: time.Date(2022, 1, 1, 11, 0, 0, 0, time.UTC)}},
							"node-3": {Name: "worker-3", State: workersv1.WorkerAvailableState, LastUpdateTime: &metav1.Time{Time: time.Date(2022, 1, 1, 10, 0, 0, 0, time.UTC)}},
						},
					},
				},
			},
			expectedCalculations: []v1.Calculation{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "calc-xc864fxvd5xccn6x",
						Namespace: "test-namespace",
						Labels: map[string]string{
							"vegaproject.io/bulk":            "test-bulk",
							"vegaproject.io/calculationName": "test-calc",
							"vegaproject.io/assign":          "worker-3",
						},
					},
					Assign: "worker-3",
					Phase:  "Created",
					Spec: v1.CalculationSpec{
						Teff: 10000,
						LogG: 4,
						Steps: []v1.Step{
							{Command: "atlas12_ada", Args: []string{"s"}},
							{Command: "atlas12_ada", Args: []string{"r"}},
							{Command: "synspec49", Args: []string{"<", "input_tlusty_fortfive"}},
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
				client: fakectrlruntimeclient.NewClientBuilder().WithObjects(append(tc.calculationBulks, tc.workerpools...)...).Build(),
			}
			req := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: "test-namespace", Name: "workerpool-test"}}
			if err := r.reconcile(context.Background(), req, r.logger); err != nil {
				t.Fatal(err)
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
