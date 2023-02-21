package calculations

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
	calcv1 "github.com/vega-project/ccb-operator/pkg/apis/calculations/v1"
)

func TestReconcile(t *testing.T) {
	testCases := []struct {
		name          string
		bulks         []ctrlruntimeclient.Object
		calculations  []ctrlruntimeclient.Object
		expectedBulks []bulkv1.CalculationBulk
	}{
		{
			name: "basic case",
			bulks: []ctrlruntimeclient.Object{
				&bulkv1.CalculationBulk{
					ObjectMeta: metav1.ObjectMeta{Name: "test-bulk", Namespace: "vega"},
					Calculations: map[string]bulkv1.Calculation{
						"test-calc": {},
					},
				},
			},
			calculations: []ctrlruntimeclient.Object{
				&calcv1.Calculation{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-calc",
						Namespace: "vega",
						Labels:    map[string]string{"vegaproject.io/bulk": "test-bulk", "vegaproject.io/calculationName": "test-calc"},
					},
					Phase: calcv1.CreatedPhase,
				},
			},
			expectedBulks: []bulkv1.CalculationBulk{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "test-bulk", Namespace: "vega"},
					Calculations: map[string]bulkv1.Calculation{
						"test-calc": {Phase: calcv1.CreatedPhase},
					},
				},
			},
		},
		{
			name: "basic case, multiple calculations",
			bulks: []ctrlruntimeclient.Object{
				&bulkv1.CalculationBulk{
					ObjectMeta: metav1.ObjectMeta{Name: "test-bulk", Namespace: "vega"},
					Calculations: map[string]bulkv1.Calculation{
						"test-calc": {},
					},
				},
			},
			calculations: []ctrlruntimeclient.Object{
				&calcv1.Calculation{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-calc",
						Namespace: "vega",
						Labels:    map[string]string{"vegaproject.io/bulk": "test-bulk", "vegaproject.io/calculationName": "test-calc"},
					},
					Phase: calcv1.ProcessingPhase,
				},
				&calcv1.Calculation{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-calc-2",
						Namespace: "vega",
						Labels:    map[string]string{"vegaproject.io/bulk": "test-bulk", "vegaproject.io/calculationName": "test-calc-2"},
					},
					Phase: calcv1.CompletedPhase,
				},
				&calcv1.Calculation{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-calc-3",
						Namespace: "vega",
						Labels:    map[string]string{"vegaproject.io/bulk": "test-bulk", "vegaproject.io/calculationName": "test-calc-3"},
					},
					Phase: calcv1.ProcessingPhase,
				},
			},
			expectedBulks: []bulkv1.CalculationBulk{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "test-bulk", Namespace: "vega"},
					Calculations: map[string]bulkv1.Calculation{
						"test-calc": {Phase: calcv1.ProcessingPhase},
					},
				},
			},
		},
		{
			name: "basic case for the post calculation",
			bulks: []ctrlruntimeclient.Object{
				&bulkv1.CalculationBulk{
					ObjectMeta: metav1.ObjectMeta{Name: "test-bulk", Namespace: "vega"},
					PostCalculation: &bulkv1.Calculation{
						Steps: []calcv1.Step{{
							Command: "python",
							Args:    []string{"post-calc.py"},
						}},
					},
				},
			},
			calculations: []ctrlruntimeclient.Object{
				&calcv1.Calculation{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-calc",
						Namespace: "vega",
						Labels:    map[string]string{"vegaproject.io/bulk": "test-bulk", "vegaproject.io/postCalculation": ""},
					},
					Phase: calcv1.CreatedPhase,
				},
			},
			expectedBulks: []bulkv1.CalculationBulk{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "test-bulk", Namespace: "vega"},
					PostCalculation: &bulkv1.Calculation{
						Steps: []calcv1.Step{{
							Command: "python",
							Args:    []string{"post-calc.py"},
						}},
						Phase: calcv1.CreatedPhase,
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := &reconciler{
				logger: logrus.WithField("test-name", tc.name),
				client: fakectrlruntimeclient.NewClientBuilder().WithObjects(append(tc.bulks, tc.calculations...)...).Build(),
			}

			req := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: "vega", Name: "test-calc"}}
			if err := r.reconcile(context.Background(), req, r.logger); err != nil {
				t.Fatal(err)
			}

			var actualBulks bulkv1.CalculationBulkList
			if err := r.client.List(context.Background(), &actualBulks); err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(actualBulks.Items, tc.expectedBulks,
				cmpopts.IgnoreFields(metav1.TypeMeta{}, "Kind", "APIVersion"),
				cmpopts.IgnoreFields(metav1.ObjectMeta{}, "ResourceVersion")); diff != "" {
				t.Fatal(diff)
			}

		})
	}
}
