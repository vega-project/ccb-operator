package resultcollector

import (
	"context"
	"reflect"
	"testing"

	clientgo_testing "k8s.io/client-go/testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/diff"

	calculationsv1 "github.com/vega-project/ccb-operator/pkg/apis/calculations/v1"
	v1 "github.com/vega-project/ccb-operator/pkg/apis/calculations/v1"
	"github.com/vega-project/ccb-operator/pkg/client/clientset/versioned/fake"
	"github.com/vega-project/ccb-operator/pkg/util"
)

func TestUpdateCalculationLabels(t *testing.T) {
	testCases := []struct {
		id             string
		calc           *v1.Calculation
		labelsToUpdate map[string]string
		expectedLabels map[string]string
	}{
		{
			id: "no new labels to update",
			calc: func() *v1.Calculation {
				c := util.NewCalculation(1000, 4.0)
				return c
			}(),
		},
		{
			id: "new label to update",
			calc: func() *v1.Calculation {
				c := util.NewCalculation(1000, 4.0)
				return c
			}(),
			labelsToUpdate: map[string]string{"test-label": "true"},
			expectedLabels: map[string]string{"test-label": "true"},
		},
		{
			id: "new labels to update",
			calc: func() *v1.Calculation {
				c := util.NewCalculation(1000, 4.0)
				return c
			}(),
			labelsToUpdate: map[string]string{"test-label": "true", "test-label2": "true"},
			expectedLabels: map[string]string{"test-label": "true", "test-label2": "true"},
		},
		{
			id: "new labels to update, calc has existing labels",
			calc: func() *v1.Calculation {
				c := util.NewCalculation(1000, 4.0)
				c.Labels = map[string]string{"existing-label": "true", "existing-label2": "true"}
				return c
			}(),
			labelsToUpdate: map[string]string{"test-label": "true", "test-label2": "true"},
			expectedLabels: map[string]string{
				"existing-label":  "true",
				"existing-label2": "true",
				"test-label":      "true",
				"test-label2":     "true"},
		},

		{
			id: "new labels to update and overwrite, calc has existing labels",
			calc: func() *v1.Calculation {
				c := util.NewCalculation(1000, 4.0)
				c.Labels = map[string]string{"existing-label": "true", "existing-label2": "true"}
				return c
			}(),
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
			fakecs := fake.NewSimpleClientset()

			fakecs.Fake.PrependReactor("update", "calculations", func(action clientgo_testing.Action) (bool, runtime.Object, error) {
				createAction := action.(clientgo_testing.CreateAction)
				calc := createAction.GetObject().(*calculationsv1.Calculation)

				if !reflect.DeepEqual(calc.Labels, tc.expectedLabels) {
					t.Fatalf(diff.ObjectReflectDiff(tc.expectedLabels, calc.Labels))
				}

				return false, nil, nil
			})

			controller := &Controller{
				calculationClientSet: fakecs,
			}
			if _, err := fakecs.VegaV1().Calculations().Create(context.Background(), tc.calc, metav1.CreateOptions{}); err != nil {
				t.Fatalf("couldn't create calculation: %v", err)
			}

			if err := controller.updateCalculationLabels(tc.calc.Name, tc.labelsToUpdate); err != nil {
				t.Fatalf("error while updating calculation: %v", err)
			}
		})
	}
}
