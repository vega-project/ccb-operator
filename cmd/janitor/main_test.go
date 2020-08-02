package main

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"

	clientgoTesting "k8s.io/client-go/testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"

	v1 "github.com/vega-project/ccb-operator/pkg/apis/calculations/v1"
	"github.com/vega-project/ccb-operator/pkg/client/clientset/versioned/fake"
	"github.com/vega-project/ccb-operator/pkg/util"
)

func TestClean(t *testing.T) {
	var ctx context.Context

	testCases := []struct {
		id              string
		calculations    []*v1.Calculation
		deletedExpected sets.String
	}{
		{
			id: "no calculation expired, no delete expected",
			calculations: []*v1.Calculation{
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
			calculations: []*v1.Calculation{
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
					ObjectMeta: metav1.ObjectMeta{Name: "calc-3", Labels: map[string]string{util.ResultsCollected: "true"}},
					Phase:      v1.CompletedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)}},
				},
			},
			deletedExpected: sets.NewString("calc-3"),
		},
		{
			id: "all calculation expired, delete expected",
			calculations: []*v1.Calculation{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "calc-1", Labels: map[string]string{util.ResultsCollected: "true"}},
					Phase:      v1.CompletedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)}},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "calc-2", Labels: map[string]string{util.ResultsCollected: "true"}},
					Phase:      v1.CompletedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)}},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "calc-3", Labels: map[string]string{util.ResultsCollected: "true"}},
					Phase:      v1.CompletedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)}},
				},
			},
			deletedExpected: sets.NewString("calc-1", "calc-2", "calc-3"),
		},
		{
			id: "calculations expired but there is one with no results collected, expected to skip the one",
			calculations: []*v1.Calculation{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "calc-1", Labels: map[string]string{util.ResultsCollected: "true"}},
					Phase:      v1.CompletedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)}},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "calc-2", Labels: map[string]string{util.ResultsCollected: "true"}},
					Phase:      v1.CompletedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)}},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "calc-3"},
					Phase:      v1.CompletedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)}},
				},
			},
			deletedExpected: sets.NewString("calc-1", "calc-2"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.id, func(t *testing.T) {
			fakecs := fake.NewSimpleClientset()

			var actualDeleted []string

			fakecs.Fake.PrependReactor("delete", "calculations", func(action clientgoTesting.Action) (bool, runtime.Object, error) {
				deleteAction := action.(clientgoTesting.DeleteAction)
				calcName := deleteAction.GetName()

				actualDeleted = append(actualDeleted, calcName)

				if !tc.deletedExpected.Has(calcName) {
					t.Fatalf("delete not expected: %s", calcName)
				}
				return false, nil, nil
			})
			client := fakecs.VegaV1()
			retention, _ := time.ParseDuration("10m")

			for _, calc := range tc.calculations {
				if _, err := client.Calculations().Create(ctx, calc, metav1.CreateOptions{}); err != nil {
					t.Fatalf("couldn't create calculation: %v", calc.Name)
				}
			}

			c := controller{
				logger:        logrus.NewEntry(logrus.StandardLogger()),
				retention:     retention,
				calcInterface: client,
			}

			// Running clean
			c.clean()

			if !reflect.DeepEqual(actualDeleted, tc.deletedExpected.List()) && len(tc.deletedExpected.List()) > 0 {
				t.Fatalf("Expected to delete %s but %s has been deleted", strings.Join(tc.deletedExpected.List(), ","), strings.Join(actualDeleted, ","))
			}
		})
	}

}
