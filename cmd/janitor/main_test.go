package main

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	cmpopts "github.com/google/go-cmp/cmp/cmpopts"
	"github.com/sirupsen/logrus"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	fakectrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "github.com/vega-project/ccb-operator/pkg/apis/calculations/v1"
	"github.com/vega-project/ccb-operator/pkg/util"
)

func TestClean(t *testing.T) {
	testCases := []struct {
		id           string
		calculations []ctrlruntimeclient.Object
		expected     []v1.Calculation
	}{
		{
			id: "no calculation expired, no delete expected",
			calculations: []ctrlruntimeclient.Object{
				&v1.Calculation{
					ObjectMeta: metav1.ObjectMeta{Name: "calc-1"},
					Phase:      v1.CompletedPhase,
				},
				&v1.Calculation{
					ObjectMeta: metav1.ObjectMeta{Name: "calc-2"},
					Phase:      v1.CompletedPhase,
				},
				&v1.Calculation{
					ObjectMeta: metav1.ObjectMeta{Name: "calc-3"},
					Phase:      v1.CompletedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Now()}},
				},
			},
			expected: []v1.Calculation{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "calc-1"},
					Phase:      v1.CompletedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Now()}},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "calc-2"},
					Phase:      v1.CompletedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Now()}},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "calc-3"},
					Phase:      v1.CompletedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Now()}},
				},
			},
		},
		{
			id: "a calculation expired, delete expected",
			calculations: []ctrlruntimeclient.Object{
				&v1.Calculation{
					ObjectMeta: metav1.ObjectMeta{Name: "calc-1"},
					Phase:      v1.CompletedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Now()}},
				},
				&v1.Calculation{
					ObjectMeta: metav1.ObjectMeta{Name: "calc-2"},
					Phase:      v1.CompletedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Now()}},
				},
				&v1.Calculation{
					ObjectMeta: metav1.ObjectMeta{Name: "calc-3", Labels: map[string]string{util.ResultsCollected: "true"}},
					Phase:      v1.CompletedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)}},
				},
			},
			expected: []v1.Calculation{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "calc-1"},
					Phase:      v1.CompletedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Now()}},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "calc-2"},
					Phase:      v1.CompletedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Now()}},
				},
			},
		},
		{
			id: "all calculation expired, delete expected",
			calculations: []ctrlruntimeclient.Object{
				&v1.Calculation{
					ObjectMeta: metav1.ObjectMeta{Name: "calc-1", Labels: map[string]string{util.ResultsCollected: "true"}},
					Phase:      v1.CompletedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)}},
				},
				&v1.Calculation{
					ObjectMeta: metav1.ObjectMeta{Name: "calc-2", Labels: map[string]string{util.ResultsCollected: "true"}},
					Phase:      v1.CompletedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)}},
				},
				&v1.Calculation{
					ObjectMeta: metav1.ObjectMeta{Name: "calc-3", Labels: map[string]string{util.ResultsCollected: "true"}},
					Phase:      v1.CompletedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)}},
				},
			},
			expected: []v1.Calculation{},
		},
		{
			id: "calculations expired but there is one with no results collected, expected to skip the one",
			calculations: []ctrlruntimeclient.Object{
				&v1.Calculation{
					ObjectMeta: metav1.ObjectMeta{Name: "calc-1", Labels: map[string]string{util.ResultsCollected: "true"}},
					Phase:      v1.CompletedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)}},
				},
				&v1.Calculation{
					ObjectMeta: metav1.ObjectMeta{Name: "calc-2", Labels: map[string]string{util.ResultsCollected: "true"}},
					Phase:      v1.CompletedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)}},
				},
				&v1.Calculation{
					ObjectMeta: metav1.ObjectMeta{Name: "calc-3"},
					Phase:      v1.CompletedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)}},
				},
			},
			expected: []v1.Calculation{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "calc-3"},
					Phase:      v1.CompletedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)}},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.id, func(t *testing.T) {
			fakeClient := fakectrlruntimeclient.NewClientBuilder().WithObjects(tc.calculations...).Build()

			retention, _ := time.ParseDuration("10m")
			c := controller{
				logger:    logrus.NewEntry(logrus.StandardLogger()),
				retention: retention,
				client:    fakeClient,
			}

			c.clean()

			var calculationList v1.CalculationList
			if err := fakeClient.List(context.Background(), &calculationList); err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tc.expected, calculationList.Items,
				cmpopts.IgnoreFields(metav1.ObjectMeta{}, "ResourceVersion"),
				cmpopts.IgnoreFields(v1.CalculationStatus{}, "StartTime")); diff != "" {
				t.Fatal(diff)
			}

		})
	}

}
