package workers

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes/scheme"

	ctrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	fakectrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "github.com/vega-project/ccb-operator/pkg/apis/calculations/v1"
)

func init() {
	if err := v1.AddToScheme(scheme.Scheme); err != nil {
		panic(fmt.Sprintf("failed to register imagev1 scheme: %v", err))
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
					ObjectMeta: metav1.ObjectMeta{Name: "test-calc", Labels: map[string]string{"assign": "test-pod"}},
					Phase:      v1.ProcessingPhase,
				},
			},
			expected: []v1.Calculation{},
		},
		{
			id:      "more than one calculation to delete, error expected",
			podName: "test-pod",
			calculations: []ctrlruntimeclient.Object{
				&v1.Calculation{
					Assign:     "test-pod",
					ObjectMeta: metav1.ObjectMeta{Name: "test-calc-more", Labels: map[string]string{"assign": "test-pod"}},
					Phase:      v1.ProcessingPhase,
				},
				&v1.Calculation{
					Assign:     "test-pod",
					ObjectMeta: metav1.ObjectMeta{Name: "test-calc-more-1", Labels: map[string]string{"assign": "test-pod"}},
					Phase:      v1.ProcessingPhase,
				},
			},
			expected: []v1.Calculation{
				{
					Assign:     "test-pod",
					ObjectMeta: metav1.ObjectMeta{Name: "test-calc-more", Labels: map[string]string{"assign": "test-pod"}},
					Phase:      v1.ProcessingPhase,
				},
				{
					Assign:     "test-pod",
					ObjectMeta: metav1.ObjectMeta{Name: "test-calc-more-1", Labels: map[string]string{"assign": "test-pod"}},
					Phase:      v1.ProcessingPhase,
				},
			},
			errorMsg: "more than one calculations found assigned to pod test-pod",
		},
		{
			id:      "more than one calculation, but only one to delete",
			podName: "test-pod",
			calculations: []ctrlruntimeclient.Object{
				&v1.Calculation{
					Assign:     "test-pod",
					ObjectMeta: metav1.ObjectMeta{Name: "test-calc", Labels: map[string]string{"assign": "test-pod"}},
					Phase:      v1.ProcessingPhase,
				},
				&v1.Calculation{
					Assign:     "another-pod",
					ObjectMeta: metav1.ObjectMeta{Name: "test-calc-2", Labels: map[string]string{"assign": "another-pod"}},
					Phase:      v1.ProcessingPhase,
				},
			},
			expected: []v1.Calculation{
				{
					Assign:     "another-pod",
					ObjectMeta: metav1.ObjectMeta{Name: "test-calc-2", Labels: map[string]string{"assign": "another-pod"}},
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

func TestAssignCalculationToPod(t *testing.T) {
	testCases := []struct {
		id                   string
		podName              string
		calculations         []ctrlruntimeclient.Object
		expectedCalculations []v1.Calculation
	}{
		{
			id:      "created by human, single happy case",
			podName: "test-pod",
			calculations: []ctrlruntimeclient.Object{
				&v1.Calculation{
					Spec: v1.CalculationSpec{
						Teff: 12000.0,
						LogG: 4.0,
					},
					ObjectMeta: metav1.ObjectMeta{Name: "test-calc", Labels: map[string]string{"created_by_human": "true"}},
					Phase:      v1.CreatedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 15, 20, 0, 0, time.UTC)}},
				},
			},
			expectedCalculations: []v1.Calculation{
				{
					Spec: v1.CalculationSpec{
						Teff: 12000.0,
						LogG: 4.0,
					},
					Assign:     "test-pod",
					ObjectMeta: metav1.ObjectMeta{Name: "test-calc", Labels: map[string]string{"created_by_human": "true"}},
					Phase:      v1.CreatedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 15, 20, 0, 0, time.UTC)}},
				},
			},
		},
		{
			id:      "created by human, multiple happy case",
			podName: "test-pod",
			calculations: []ctrlruntimeclient.Object{
				&v1.Calculation{
					Spec: v1.CalculationSpec{
						Teff: 12000.0,
						LogG: 4.0,
					},
					ObjectMeta: metav1.ObjectMeta{Name: "test-calc", Labels: map[string]string{"created_by_human": "true"}},
					Phase:      v1.CreatedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 15, 20, 0, 0, time.UTC)}},
				},
				&v1.Calculation{
					Spec: v1.CalculationSpec{
						Teff: 13000.0,
						LogG: 4.0,
					},
					ObjectMeta: metav1.ObjectMeta{Name: "test-calc-2", Labels: map[string]string{"created_by_human": "true"}},
					Phase:      v1.CreatedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 15, 30, 0, 0, time.UTC)}},
				},
			},
			expectedCalculations: []v1.Calculation{
				{
					Spec: v1.CalculationSpec{
						Teff: 12000.0,
						LogG: 4.0,
					},
					Assign:     "test-pod",
					ObjectMeta: metav1.ObjectMeta{Name: "test-calc", Labels: map[string]string{"created_by_human": "true"}},
					Phase:      v1.CreatedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 15, 20, 0, 0, time.UTC)}},
				},
				{
					Spec: v1.CalculationSpec{
						Teff: 13000.0,
						LogG: 4.0,
					},
					ObjectMeta: metav1.ObjectMeta{Name: "test-calc-2", Labels: map[string]string{"created_by_human": "true"}},
					Phase:      v1.CreatedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 15, 30, 0, 0, time.UTC)}},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.id, func(t *testing.T) {
			r := &reconciler{
				logger: logrus.WithField("test-name", tc.id),
				client: fakectrlruntimeclient.NewClientBuilder().WithObjects(tc.calculations...).Build(),
			}

			if err := r.assignCalculationToPod(context.Background(), tc.podName); err != nil {
				t.Fatal(err)
			}

			actualCalculations := &v1.CalculationList{}
			if err := r.client.List(context.Background(), actualCalculations); err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tc.expectedCalculations, actualCalculations.Items,
				cmpopts.IgnoreFields(metav1.TypeMeta{}, "Kind", "APIVersion"),
				cmpopts.IgnoreFields(metav1.ObjectMeta{}, "ResourceVersion")); diff != "" {
				t.Fatal(diff)
			}
		})
	}
}
