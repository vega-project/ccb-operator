package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	cmpopts "github.com/google/go-cmp/cmp/cmpopts"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	fakectrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "github.com/vega-project/ccb-operator/pkg/apis/calculations/v1"
	"github.com/vega-project/ccb-operator/pkg/util"
)

func TestCreateCalculation(t *testing.T) {
	testCases := []struct {
		id                  string
		teff                float64
		logG                float64
		initialCalculations []ctrlruntimeclient.Object
		expected            []v1.Calculation
	}{
		{
			id:   "no initial calculations in cluster, one calculation gets created",
			teff: 12100.0,
			logG: 4.0,
			expected: []v1.Calculation{
				{
					ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"created_by_human": "true"}, Name: fmt.Sprintf("calc-%s", util.InputHash([]byte(fmt.Sprintf("%f", 12100.0)), []byte(fmt.Sprintf("%f", 4.0))))},
					Phase:      v1.CreatedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
					Spec: v1.CalculationSpec{Teff: 12100.0, LogG: 4.0, Steps: []v1.Step{
						{
							Command: "atlas12_ada",
							Args:    []string{"s"},
						},
						{
							Command: "atlas12_ada",
							Args:    []string{"r"},
						},
						{
							Command: "synspec49",
							Args:    []string{"<", "input_tlusty_fortfive"},
						},
					}},
				},
			},
		},
		{
			id:   "one calculation gets created, one already exists in cluster",
			teff: 14100.0,
			logG: 4.0,
			initialCalculations: []ctrlruntimeclient.Object{
				&v1.Calculation{
					ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"created_by_human": "true"}, Name: fmt.Sprintf("calc-%s", util.InputHash([]byte(fmt.Sprintf("%f", 15100.0)), []byte(fmt.Sprintf("%f", 4.0))))},
					Phase:      v1.CreatedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
					Spec: v1.CalculationSpec{Teff: 15100.0, LogG: 4.0, Steps: []v1.Step{
						{
							Command: "atlas12_ada",
							Args:    []string{"s"},
						},
						{
							Command: "atlas12_ada",
							Args:    []string{"r"},
						},
						{
							Command: "synspec49",
							Args:    []string{"<", "input_tlusty_fortfive"},
						},
					}},
				},
			},
			expected: []v1.Calculation{
				{
					ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"created_by_human": "true"}, Name: fmt.Sprintf("calc-%s", util.InputHash([]byte(fmt.Sprintf("%f", 15100.0)), []byte(fmt.Sprintf("%f", 4.0))))},
					Phase:      v1.CreatedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
					Spec: v1.CalculationSpec{Teff: 15100.0, LogG: 4.0, Steps: []v1.Step{
						{
							Command: "atlas12_ada",
							Args:    []string{"s"},
						},
						{
							Command: "atlas12_ada",
							Args:    []string{"r"},
						},
						{
							Command: "synspec49",
							Args:    []string{"<", "input_tlusty_fortfive"},
						},
					}},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"created_by_human": "true"}, Name: fmt.Sprintf("calc-%s", util.InputHash([]byte(fmt.Sprintf("%f", 14100.0)), []byte(fmt.Sprintf("%f", 4.0))))},
					Phase:      v1.CreatedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
					Spec: v1.CalculationSpec{Teff: 14100.0, LogG: 4.0, Steps: []v1.Step{
						{
							Command: "atlas12_ada",
							Args:    []string{"s"},
						},
						{
							Command: "atlas12_ada",
							Args:    []string{"r"},
						},
						{
							Command: "synspec49",
							Args:    []string{"<", "input_tlusty_fortfive"},
						},
					}},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.id, func(t *testing.T) {
			var calc struct {
				Teff string
				LogG string
			}

			fakeClient := fakectrlruntimeclient.NewClientBuilder().WithObjects(tc.initialCalculations...).Build()
			s := server{
				logger: logrus.WithField("test-name", tc.id),
				ctx:    context.Background(),
				client: fakeClient,
			}

			calc.Teff = fmt.Sprintf("%v", tc.teff)
			calc.LogG = fmt.Sprintf("%v", tc.logG)
			data, err := json.Marshal(calc)
			if err != nil {
				t.Fatal(err)
			}

			req, err := http.NewRequest("POST", "/calculations/create", bytes.NewBuffer(data))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(s.createCalculation)
			handler.ServeHTTP(rr, req)

			var actualData *v1.Calculation
			err = json.Unmarshal(rr.Body.Bytes(), &actualData)
			if err != nil {
				t.Fatal(err)
			}

			var calculationList v1.CalculationList
			if err := fakeClient.List(s.ctx, &calculationList); err != nil {
				t.Fatal(err)
			}
			if rr.Code == http.StatusOK {
				logrus.WithFields(logrus.Fields{"calculation": actualData.Name, "teff": actualData.Spec.Teff, "logG": actualData.Spec.LogG}).Info("Created calculation using the api server...")
			} else {
				logrus.Info("No calculation was created...")
			}

			if diff := cmp.Diff(tc.expected, calculationList.Items,
				cmpopts.IgnoreFields(metav1.Time{}, "Time"),
				cmpopts.IgnoreFields(metav1.ObjectMeta{}, "ResourceVersion"),
				cmpopts.IgnoreFields(metav1.TypeMeta{}, "Kind", "APIVersion")); diff != "" {
				t.Fatal(diff)
			}
		})
	}
}

func TestDeleteCalculation(t *testing.T) {
	testCases := []struct {
		id                  string
		calculationToDelete string
		initialCalculations []ctrlruntimeclient.Object
		expected            []v1.Calculation
		errorExpected       bool
	}{
		{
			id:                  "one calculation nothing gets deleted",
			calculationToDelete: "calc-wrong-name",
			initialCalculations: []ctrlruntimeclient.Object{
				&v1.Calculation{
					ObjectMeta: metav1.ObjectMeta{Name: "calc-1"},
					Phase:      v1.CreatedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
					Spec:       v1.CalculationSpec{Teff: 16000.0, LogG: 4.0},
				},
			},
			expected: []v1.Calculation{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "calc-1"},
					Phase:      v1.CreatedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
					Spec:       v1.CalculationSpec{Teff: 16000.0, LogG: 4.0},
				},
			},
			errorExpected: true,
		},
		{
			id:                  "one calculation one gets deleted",
			calculationToDelete: "calc-delete",
			initialCalculations: []ctrlruntimeclient.Object{
				&v1.Calculation{
					ObjectMeta: metav1.ObjectMeta{Name: "calc-delete"},
					Phase:      v1.CreatedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
					Spec:       v1.CalculationSpec{Teff: 10000.0, LogG: 4.0},
				},
			},
			expected: []v1.Calculation{},
		},
		{
			id:                  "one out of X calculations get deleted",
			calculationToDelete: "calc-delete",
			initialCalculations: []ctrlruntimeclient.Object{
				&v1.Calculation{
					ObjectMeta: metav1.ObjectMeta{Name: "calc-delete"},
					Phase:      v1.CreatedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
					Spec:       v1.CalculationSpec{Teff: 11000.0, LogG: 4.0},
				},
				&v1.Calculation{
					ObjectMeta: metav1.ObjectMeta{Name: "calc-1"},
					Phase:      v1.CreatedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
					Spec:       v1.CalculationSpec{Teff: 12000.0, LogG: 4.0},
				},
			},
			expected: []v1.Calculation{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "calc-1"},
					Phase:      v1.CreatedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
					Spec:       v1.CalculationSpec{Teff: 12000.0, LogG: 4.0},
				},
			},
		},
		{
			id:                  "none out of X calculations get deleted",
			calculationToDelete: "calc-wrong-name",
			initialCalculations: []ctrlruntimeclient.Object{
				&v1.Calculation{
					ObjectMeta: metav1.ObjectMeta{Name: "calc-1"},
					Phase:      v1.CreatedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
					Spec:       v1.CalculationSpec{Teff: 11000.0, LogG: 4.0},
				},
				&v1.Calculation{
					ObjectMeta: metav1.ObjectMeta{Name: "calc-2"},
					Phase:      v1.CreatedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
					Spec:       v1.CalculationSpec{Teff: 12000.0, LogG: 4.0},
				},
			},
			expected: []v1.Calculation{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "calc-1"},
					Phase:      v1.CreatedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
					Spec:       v1.CalculationSpec{Teff: 11000.0, LogG: 4.0},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "calc-2"},
					Phase:      v1.CreatedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
					Spec:       v1.CalculationSpec{Teff: 12000.0, LogG: 4.0},
				},
			},
			errorExpected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.id, func(t *testing.T) {
			fakeClient := fakectrlruntimeclient.NewClientBuilder().WithObjects(tc.initialCalculations...).Build()

			s := server{
				logger: logrus.WithField("test-name", tc.id),
				ctx:    context.Background(),
				client: fakeClient,
			}

			req, err := http.NewRequest("DELETE", "/calculations/delete/", nil)
			if err != nil {
				t.Fatal(err)
			}
			vars := map[string]string{
				"id": tc.calculationToDelete,
			}

			req = mux.SetURLVars(req, vars)

			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(s.deleteCalculation)
			handler.ServeHTTP(rr, req)

			var calculationList v1.CalculationList
			if err := fakeClient.List(s.ctx, &calculationList); err != nil {
				t.Fatal(err)
			}

			if status := rr.Code; status == http.StatusOK {
				logrus.Info(rr.Body)
			} else if status != http.StatusOK {
				logrus.Info(rr.Body)
			}

			if diff := cmp.Diff(tc.expected, calculationList.Items,
				cmpopts.IgnoreFields(metav1.ObjectMeta{}, "ResourceVersion"),
				cmpopts.IgnoreFields(metav1.TypeMeta{}, "Kind", "APIVersion")); diff != "" && !tc.errorExpected {
				t.Fatal(diff)
			}
		})
	}
}

func TestGetCalculations(t *testing.T) {
	testCases := []struct {
		id                  string
		initialCalculations []ctrlruntimeclient.Object
		expected            []v1.Calculation
	}{
		{
			id: "get no calculation",
		},
		{
			id: "get one calculation",
			initialCalculations: []ctrlruntimeclient.Object{
				&v1.Calculation{
					ObjectMeta: metav1.ObjectMeta{Name: "calc-1"},
					Phase:      v1.CreatedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
					Spec:       v1.CalculationSpec{Teff: 10000.0, LogG: 4.0},
				},
			},
			expected: []v1.Calculation{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "calc-1"},
					Phase:      v1.CreatedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
					Spec:       v1.CalculationSpec{Teff: 10000.0, LogG: 4.0},
				},
			},
		},
		{
			id: "get two calculations",
			initialCalculations: []ctrlruntimeclient.Object{
				&v1.Calculation{
					ObjectMeta: metav1.ObjectMeta{Name: "calc-2"},
					Phase:      v1.CreatedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
					Spec:       v1.CalculationSpec{Teff: 11000.0, LogG: 4.0},
				},
				&v1.Calculation{
					ObjectMeta: metav1.ObjectMeta{Name: "calc-3"},
					Phase:      v1.CreatedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
					Spec:       v1.CalculationSpec{Teff: 12000.0, LogG: 4.0},
				},
			},
			expected: []v1.Calculation{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "calc-2"},
					Phase:      v1.CreatedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
					Spec:       v1.CalculationSpec{Teff: 11000.0, LogG: 4.0},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "calc-3"},
					Phase:      v1.CreatedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
					Spec:       v1.CalculationSpec{Teff: 12000.0, LogG: 4.0},
				},
			},
		},
	}

	for _, tc := range testCases {
		fakeClient := fakectrlruntimeclient.NewClientBuilder().WithObjects(tc.initialCalculations...).Build()

		s := server{
			logger: logrus.WithField("test-name", tc.id),
			ctx:    context.Background(),
			client: fakeClient,
		}

		req, err := http.NewRequest("GET", "/calculations", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(s.getCalculations)
		handler.ServeHTTP(rr, req)
		if status := rr.Code; status != http.StatusOK {
			t.Errorf("Handler returned wrong status code: got %v want %v",
				status, http.StatusOK)
		}

		var actualData *v1.CalculationList
		err = json.Unmarshal(rr.Body.Bytes(), &actualData)
		if err != nil {
			t.Fatal(err)
		}

		if diff := cmp.Diff(tc.expected, actualData.Items, cmpopts.IgnoreFields(metav1.ObjectMeta{}, "ResourceVersion")); diff != "" {
			t.Fatal(diff)
		}
	}
}

func TestGetCalculationByName(t *testing.T) {
	testCases := []struct {
		id                 string
		initialCalculation []ctrlruntimeclient.Object
		expected           v1.Calculation
		errorExpected      bool
	}{
		{
			id: "no calculations",
		},
		{
			id: "one calculation returns",
			initialCalculation: []ctrlruntimeclient.Object{
				&v1.Calculation{
					ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("calc-%s", util.InputHash([]byte(fmt.Sprintf("%f", 12100.0)), []byte(fmt.Sprintf("%f", 4.0))))},
					Phase:      v1.CreatedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
					Spec:       v1.CalculationSpec{Teff: 12100.0, LogG: 4.0},
				},
			},
			expected: v1.Calculation{
				ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("calc-%s", util.InputHash([]byte(fmt.Sprintf("%f", 12100.0)), []byte(fmt.Sprintf("%f", 4.0))))},
				Phase:      v1.CreatedPhase,
				Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
				Spec:       v1.CalculationSpec{Teff: 12100.0, LogG: 4.0},
			},
		},
		{
			id: "get calculation with wrong name",
			initialCalculation: []ctrlruntimeclient.Object{
				&v1.Calculation{
					ObjectMeta: metav1.ObjectMeta{Name: "calc-wrong-name"},
					Phase:      v1.CreatedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
					Spec:       v1.CalculationSpec{Teff: 12100.0, LogG: 4.0},
				},
			},
			errorExpected: true,
		},
	}

	for _, tc := range testCases {
		fakeClient := fakectrlruntimeclient.NewClientBuilder().WithObjects(tc.initialCalculation...).Build()

		s := server{
			logger: logrus.WithField("test-name", tc.id),
			ctx:    context.Background(),
			client: fakeClient,
		}

		req, err := http.NewRequest("GET", "/calculation/", nil)
		if err != nil {
			t.Fatal(err)
		}

		vars := make(map[string]string)
		if len(tc.initialCalculation) > 0 {
			vars["id"] = tc.initialCalculation[0].GetName()
		}
		req = mux.SetURLVars(req, vars)

		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(s.getCalculationByName)
		handler.ServeHTTP(rr, req)

		var actualData *v1.Calculation

		err = json.Unmarshal(rr.Body.Bytes(), &actualData)
		if err != nil {
			t.Fatal(err)
		}

		if diff := cmp.Diff(tc.expected, *actualData,
			cmpopts.IgnoreFields(metav1.TypeMeta{}, "Kind", "APIVersion"),
			cmpopts.IgnoreFields(metav1.ObjectMeta{}, "ResourceVersion")); diff != "" && !tc.errorExpected {
			t.Fatal(diff)
		}
	}
}

func TestGetCalculation(t *testing.T) {
	testCases := []struct {
		id                  string
		teff                float64
		logG                float64
		initialCalculations []ctrlruntimeclient.Object
		expected            []v1.Calculation
	}{
		{
			id:   "one calculation returns",
			teff: 12100.0,
			logG: 4.0,
			initialCalculations: []ctrlruntimeclient.Object{
				&v1.Calculation{
					ObjectMeta: metav1.ObjectMeta{Name: "calc-1"},
					Phase:      v1.CreatedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
					Spec:       v1.CalculationSpec{Teff: 12100.0, LogG: 4.0},
				},
				&v1.Calculation{
					ObjectMeta: metav1.ObjectMeta{Name: "calc-2"},
					Phase:      v1.CreatedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
					Spec:       v1.CalculationSpec{Teff: 13100.0, LogG: 4.0},
				},
			},
			expected: []v1.Calculation{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "calc-1"},
					Phase:      v1.CreatedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
					Spec:       v1.CalculationSpec{Teff: 12100.0, LogG: 4.0},
				},
			},
		},
	}

	for _, tc := range testCases {
		fakeClient := fakectrlruntimeclient.NewClientBuilder().WithObjects(tc.initialCalculations...).Build()

		s := server{
			logger: logrus.WithField("test-name", tc.id),
			ctx:    context.Background(),
			client: fakeClient,
		}

		req, err := http.NewRequest("GET", "/calculation", nil)
		if err != nil {
			t.Fatal(err)
		}

		q := req.URL.Query()
		q.Add("teff", fmt.Sprintf("%v", tc.teff))
		q.Add("logG", fmt.Sprintf("%v", tc.logG))
		req.URL.RawQuery = q.Encode()

		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(s.getCalculation)
		handler.ServeHTTP(rr, req)

		var actualData []v1.Calculation
		err = json.Unmarshal(rr.Body.Bytes(), &actualData)
		if err != nil {
			t.Fatal(err)
		}

		if diff := cmp.Diff(tc.expected, actualData, cmpopts.IgnoreFields(metav1.ObjectMeta{}, "ResourceVersion")); diff != "" {
			t.Fatal(diff)
		}
	}
}
