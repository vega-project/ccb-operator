package resultcollector

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"

	"k8s.io/apimachinery/pkg/types"

	ctrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	fakectrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/sirupsen/logrus"
	v1 "github.com/vega-project/ccb-operator/pkg/apis/calculations/v1"
	"github.com/vega-project/ccb-operator/pkg/util"
)

func TestUpdateCalculationLabels(t *testing.T) {
	testCases := []struct {
		id             string
		calculation    []ctrlruntimeclient.Object
		labelsToUpdate map[string]string
		expectedLabels map[string]string
	}{
		{
			id: "no new labels to update",
			calculation: []ctrlruntimeclient.Object{
				func() *v1.Calculation {
					c := util.NewCalculation(1000, 4.0)
					return c
				}(),
			},
		},
		{
			id: "new label to update",
			calculation: []ctrlruntimeclient.Object{
				func() *v1.Calculation {
					c := util.NewCalculation(1000, 4.0)
					return c
				}(),
			},
			labelsToUpdate: map[string]string{"test-label": "true"},
			expectedLabels: map[string]string{"test-label": "true"},
		},
		{
			id: "new labels to update",
			calculation: []ctrlruntimeclient.Object{
				func() *v1.Calculation {
					c := util.NewCalculation(1000, 4.0)
					return c
				}(),
			},
			labelsToUpdate: map[string]string{"test-label": "true", "test-label2": "true"},
			expectedLabels: map[string]string{"test-label": "true", "test-label2": "true"},
		},
		{
			id: "new labels to update, calc has existing labels",
			calculation: []ctrlruntimeclient.Object{
				func() *v1.Calculation {
					c := util.NewCalculation(1000, 4.0)
					c.Labels = map[string]string{"existing-label": "true", "existing-label2": "true"}
					return c
				}(),
			},
			labelsToUpdate: map[string]string{"test-label": "true", "test-label2": "true"},
			expectedLabels: map[string]string{
				"existing-label":  "true",
				"existing-label2": "true",
				"test-label":      "true",
				"test-label2":     "true"},
		},
		{
			id: "new labels to update and overwrite, calc has existing labels",
			calculation: []ctrlruntimeclient.Object{
				func() *v1.Calculation {
					c := util.NewCalculation(1000, 4.0)
					c.Labels = map[string]string{"existing-label": "true", "existing-label2": "true"}
					return c
				}(),
			},
			labelsToUpdate: map[string]string{"test-label": "true", "test-label2": "true", "existing-label": "overwritten"},
			expectedLabels: map[string]string{
				"existing-label":  "overwritten",
				"existing-label2": "true",
				"test-label":      "true",
				"test-label2":     "true"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.id, func(t *testing.T) {
			r := &reconciler{
				logger: logrus.WithField("test-name", tc.id),
				client: fakectrlruntimeclient.NewClientBuilder().WithObjects(tc.calculation...).Build(),
			}

			for _, calc := range tc.calculation {
				req := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: calc.GetNamespace(), Name: calc.GetName()}}
				if err := r.reconcile(context.Background(), req, r.logger); err != nil {
					t.Fatal(err)
				}
				if err := r.updateCalculationLabels(context.Background(), calc.GetName(), tc.labelsToUpdate); err != nil {
					t.Fatalf("error while updating calculation: %v", err)
				}

				actualCalculation := v1.Calculation{}
				if err := r.client.Get(context.Background(), ctrlruntimeclient.ObjectKey{Namespace: calc.GetNamespace(), Name: calc.GetName()}, &actualCalculation); err != nil {
					t.Fatal(err)
				}

				if diff := cmp.Diff(actualCalculation.Labels, tc.expectedLabels); diff != "" {
					t.Fatal(diff)
				}
			}
		})
	}
}
