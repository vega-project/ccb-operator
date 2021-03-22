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
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cmpopts "github.com/google/go-cmp/cmp/cmpopts"
	calculationsv1 "github.com/vega-project/ccb-operator/pkg/apis/calculations/v1"
	v1 "github.com/vega-project/ccb-operator/pkg/apis/calculations/v1"
	"github.com/vega-project/ccb-operator/pkg/client/clientset/versioned/fake"
	"github.com/vega-project/ccb-operator/pkg/util"
)

func TestCreateCalculation(t *testing.T) {
	testCases := []struct {
		id                  string
		teff                float64
		logG                float64
		initialCalculations []calculationsv1.Calculation
		expected            []calculationsv1.Calculation
	}{
		{
			id:   "no initial calculatins in cluster, one calculation gets created",
			teff: 12100.0,
			logG: 4.0,
			expected: []calculationsv1.Calculation{
				{
					TypeMeta:   metav1.TypeMeta{Kind: "Calculation", APIVersion: "vega.io/v1"},
					ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"created_by_human": "true"}, Name: fmt.Sprintf("calc-%s", util.InputHash([]byte(fmt.Sprintf("%f", 12100.0)), []byte(fmt.Sprintf("%f", 4.0))))},
					Phase:      calculationsv1.CreatedPhase,
					Status:     calculationsv1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
					Spec: calculationsv1.CalculationSpec{Teff: 12100.0, LogG: 4.0, Steps: []v1.Step{
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
			initialCalculations: []calculationsv1.Calculation{
				{
					TypeMeta:   metav1.TypeMeta{Kind: "Calculation", APIVersion: "vega.io/v1"},
					ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"created_by_human": "true"}, Name: fmt.Sprintf("calc-%s", util.InputHash([]byte(fmt.Sprintf("%f", 15100.0)), []byte(fmt.Sprintf("%f", 4.0))))},
					Phase:      calculationsv1.CreatedPhase,
					Status:     calculationsv1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
					Spec: calculationsv1.CalculationSpec{Teff: 15100.0, LogG: 4.0, Steps: []v1.Step{
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
			expected: []calculationsv1.Calculation{
				{
					TypeMeta:   metav1.TypeMeta{Kind: "Calculation", APIVersion: "vega.io/v1"},
					ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"created_by_human": "true"}, Name: fmt.Sprintf("calc-%s", util.InputHash([]byte(fmt.Sprintf("%f", 15100.0)), []byte(fmt.Sprintf("%f", 4.0))))},
					Phase:      calculationsv1.CreatedPhase,
					Status:     calculationsv1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
					Spec: calculationsv1.CalculationSpec{Teff: 15100.0, LogG: 4.0, Steps: []v1.Step{
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
					TypeMeta:   metav1.TypeMeta{Kind: "Calculation", APIVersion: "vega.io/v1"},
					ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"created_by_human": "true"}, Name: fmt.Sprintf("calc-%s", util.InputHash([]byte(fmt.Sprintf("%f", 14100.0)), []byte(fmt.Sprintf("%f", 4.0))))},
					Phase:      calculationsv1.CreatedPhase,
					Status:     calculationsv1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
					Spec: calculationsv1.CalculationSpec{Teff: 14100.0, LogG: 4.0, Steps: []v1.Step{
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
		fakecs := fake.NewSimpleClientset()
		fakeClient := fakecs.VegaV1()

		var calc struct {
			Teff string
			LogG string
		}

		s := server{
			logger: logrus.WithField("test-name", "create calculations test"),
			ctx:    context.Background(),
			client: fakeClient,
		}

		for _, calc := range tc.initialCalculations {
			_, err := fakeClient.Calculations().Create(s.ctx, &calc, metav1.CreateOptions{})
			if err != nil {
				t.Errorf("Coudn't create the calculation: %v", calc)
			}
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

		var actualData *calculationsv1.Calculation

		err = json.Unmarshal(rr.Body.Bytes(), &actualData)
		if err != nil {
			t.Fatal(err)
		}

		calculationList, err := fakeClient.Calculations().List(s.ctx, metav1.ListOptions{})
		if err != nil {
			t.Fatal(err)
		}

		if rr.Code == http.StatusOK {
			logrus.WithFields(logrus.Fields{"calculation": actualData.Name, "teff": actualData.Spec.Teff, "logG": actualData.Spec.LogG}).Info("Created calculation using the api server...")
		} else {
			logrus.Info("No calculation was created...")
		}

		if diff := cmp.Diff(tc.expected, calculationList.Items, cmpopts.IgnoreFields(metav1.Time{}, "Time")); diff != "" {
			t.Fatal(diff)
		}
	}
}

func TestDeleteCalculation(t *testing.T) {
	testCases := []struct {
		id                  string
		calculationToDelete string
		initialCalculations []calculationsv1.Calculation
		expected            []calculationsv1.Calculation
		errorExpected       bool
	}{
		{
			id:                  "one calculation nothing gets deleted",
			calculationToDelete: "calc-wrong-name",
			initialCalculations: []calculationsv1.Calculation{
				{
					TypeMeta:   metav1.TypeMeta{Kind: "Calculation", APIVersion: "vega.io/v1"},
					ObjectMeta: metav1.ObjectMeta{Name: "calc-1"},
					Phase:      calculationsv1.CreatedPhase,
					Status:     calculationsv1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
					Spec:       calculationsv1.CalculationSpec{Teff: 16000.0, LogG: 4.0},
				},
			},
			expected: []calculationsv1.Calculation{
				{
					TypeMeta:   metav1.TypeMeta{Kind: "Calculation", APIVersion: "vega.io/v1"},
					ObjectMeta: metav1.ObjectMeta{Name: "calc-1"},
					Phase:      calculationsv1.CreatedPhase,
					Status:     calculationsv1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
					Spec:       calculationsv1.CalculationSpec{Teff: 16000.0, LogG: 4.0},
				},
			},
			errorExpected: true,
		},
		{
			id:                  "one calculation one gets deleted",
			calculationToDelete: "calc-delete",
			initialCalculations: []calculationsv1.Calculation{
				{
					TypeMeta:   metav1.TypeMeta{Kind: "Calculation", APIVersion: "vega.io/v1"},
					ObjectMeta: metav1.ObjectMeta{Name: "calc-delete"},
					Phase:      calculationsv1.CreatedPhase,
					Status:     calculationsv1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
					Spec:       calculationsv1.CalculationSpec{Teff: 10000.0, LogG: 4.0},
				},
			},
		},
		{
			id:                  "one out of X calculations get deleted",
			calculationToDelete: "calc-delete",
			initialCalculations: []calculationsv1.Calculation{
				{
					TypeMeta:   metav1.TypeMeta{Kind: "Calculation", APIVersion: "vega.io/v1"},
					ObjectMeta: metav1.ObjectMeta{Name: "calc-delete"},
					Phase:      calculationsv1.CreatedPhase,
					Status:     calculationsv1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
					Spec:       calculationsv1.CalculationSpec{Teff: 11000.0, LogG: 4.0},
				},
				{
					TypeMeta:   metav1.TypeMeta{Kind: "Calculation", APIVersion: "vega.io/v1"},
					ObjectMeta: metav1.ObjectMeta{Name: "calc-1"},
					Phase:      calculationsv1.CreatedPhase,
					Status:     calculationsv1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
					Spec:       calculationsv1.CalculationSpec{Teff: 12000.0, LogG: 4.0},
				},
			},
			expected: []calculationsv1.Calculation{
				{
					TypeMeta:   metav1.TypeMeta{Kind: "Calculation", APIVersion: "vega.io/v1"},
					ObjectMeta: metav1.ObjectMeta{Name: "calc-1"},
					Phase:      calculationsv1.CreatedPhase,
					Status:     calculationsv1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
					Spec:       calculationsv1.CalculationSpec{Teff: 12000.0, LogG: 4.0},
				},
			},
		},
		{
			id:                  "none out of X calculations get deleted",
			calculationToDelete: "calc-wrong-name",
			initialCalculations: []calculationsv1.Calculation{
				{
					TypeMeta:   metav1.TypeMeta{Kind: "Calculation", APIVersion: "vega.io/v1"},
					ObjectMeta: metav1.ObjectMeta{Name: "calc-1"},
					Phase:      calculationsv1.CreatedPhase,
					Status:     calculationsv1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
					Spec:       calculationsv1.CalculationSpec{Teff: 11000.0, LogG: 4.0},
				},
				{
					TypeMeta:   metav1.TypeMeta{Kind: "Calculation", APIVersion: "vega.io/v1"},
					ObjectMeta: metav1.ObjectMeta{Name: "calc-2"},
					Phase:      calculationsv1.CreatedPhase,
					Status:     calculationsv1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
					Spec:       calculationsv1.CalculationSpec{Teff: 12000.0, LogG: 4.0},
				},
			},
			expected: []calculationsv1.Calculation{
				{
					TypeMeta:   metav1.TypeMeta{Kind: "Calculation", APIVersion: "vega.io/v1"},
					ObjectMeta: metav1.ObjectMeta{Name: "calc-1"},
					Phase:      calculationsv1.CreatedPhase,
					Status:     calculationsv1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
					Spec:       calculationsv1.CalculationSpec{Teff: 11000.0, LogG: 4.0},
				},
				{
					TypeMeta:   metav1.TypeMeta{Kind: "Calculation", APIVersion: "vega.io/v1"},
					ObjectMeta: metav1.ObjectMeta{Name: "calc-2"},
					Phase:      calculationsv1.CreatedPhase,
					Status:     calculationsv1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
					Spec:       calculationsv1.CalculationSpec{Teff: 12000.0, LogG: 4.0},
				},
			},
			errorExpected: true,
		},
	}

	for _, tc := range testCases {
		fakecs := fake.NewSimpleClientset()
		fakeClient := fakecs.VegaV1()

		s := server{
			logger: logrus.WithField("test-name", tc.id),
			ctx:    context.Background(),
			client: fakeClient,
		}

		for _, calc := range tc.initialCalculations {
			_, err := fakeClient.Calculations().Create(s.ctx, &calc, metav1.CreateOptions{})
			if err != nil {
				t.Errorf("Coudn't create the calculation: %v", calc)
			}
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

		calculationList, err := fakeClient.Calculations().List(s.ctx, metav1.ListOptions{})
		if err != nil {
			t.Fatal(err)
		}

		if status := rr.Code; status == http.StatusOK {
			logrus.Info(rr.Body)
		} else if status != http.StatusOK {
			logrus.Info(rr.Body)
		}

		if diff := cmp.Diff(tc.expected, calculationList.Items); diff != "" && !tc.errorExpected {
			t.Fatal(diff)
		}
	}
}

func TestGetCalculations(t *testing.T) {
	testCases := []struct {
		id                  string
		initialCalculations []calculationsv1.Calculation
		expected            []calculationsv1.Calculation
	}{
		{
			id: "get no calculation",
		},
		{
			id: "get one calculation",
			initialCalculations: []calculationsv1.Calculation{
				{
					TypeMeta:   metav1.TypeMeta{Kind: "Calculation", APIVersion: "vega.io/v1"},
					ObjectMeta: metav1.ObjectMeta{Name: "calc-1"},
					Phase:      calculationsv1.CreatedPhase,
					Status:     calculationsv1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
					Spec:       calculationsv1.CalculationSpec{Teff: 10000.0, LogG: 4.0},
				},
			},
			expected: []calculationsv1.Calculation{
				{
					TypeMeta:   metav1.TypeMeta{Kind: "Calculation", APIVersion: "vega.io/v1"},
					ObjectMeta: metav1.ObjectMeta{Name: "calc-1"},
					Phase:      calculationsv1.CreatedPhase,
					Status:     calculationsv1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
					Spec:       calculationsv1.CalculationSpec{Teff: 10000.0, LogG: 4.0},
				},
			},
		},
		{
			id: "get two calculations",
			initialCalculations: []calculationsv1.Calculation{
				{
					TypeMeta:   metav1.TypeMeta{Kind: "Calculation", APIVersion: "vega.io/v1"},
					ObjectMeta: metav1.ObjectMeta{Name: "calc-2"},
					Phase:      calculationsv1.CreatedPhase,
					Status:     calculationsv1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
					Spec:       calculationsv1.CalculationSpec{Teff: 11000.0, LogG: 4.0},
				},
				{
					TypeMeta:   metav1.TypeMeta{Kind: "Calculation", APIVersion: "vega.io/v1"},
					ObjectMeta: metav1.ObjectMeta{Name: "calc-3"},
					Phase:      calculationsv1.CreatedPhase,
					Status:     calculationsv1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
					Spec:       calculationsv1.CalculationSpec{Teff: 12000.0, LogG: 4.0},
				},
			},
			expected: []calculationsv1.Calculation{
				{
					TypeMeta:   metav1.TypeMeta{Kind: "Calculation", APIVersion: "vega.io/v1"},
					ObjectMeta: metav1.ObjectMeta{Name: "calc-2"},
					Phase:      calculationsv1.CreatedPhase,
					Status:     calculationsv1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
					Spec:       calculationsv1.CalculationSpec{Teff: 11000.0, LogG: 4.0},
				},
				{
					TypeMeta:   metav1.TypeMeta{Kind: "Calculation", APIVersion: "vega.io/v1"},
					ObjectMeta: metav1.ObjectMeta{Name: "calc-3"},
					Phase:      calculationsv1.CreatedPhase,
					Status:     calculationsv1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
					Spec:       calculationsv1.CalculationSpec{Teff: 12000.0, LogG: 4.0},
				},
			},
		},
	}

	for _, tc := range testCases {
		fakecs := fake.NewSimpleClientset()
		fakeClient := fakecs.VegaV1()

		s := server{
			logger: logrus.WithField("test-name", tc.id),
			ctx:    context.Background(),
			client: fakeClient,
		}

		for _, calc := range tc.initialCalculations {
			_, err := fakeClient.Calculations().Create(s.ctx, &calc, metav1.CreateOptions{})
			if err != nil {
				t.Errorf("Coudn't create the calculation: %v", tc.initialCalculations)
			}
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

		var actualData *calculationsv1.CalculationList

		err = json.Unmarshal(rr.Body.Bytes(), &actualData)
		if err != nil {
			t.Fatal(err)
		}

		if diff := cmp.Diff(tc.expected, actualData.Items); diff != "" {
			t.Fatal(diff)
		}
	}
}

func TestGetCalculationByName(t *testing.T) {
	testCases := []struct {
		id                 string
		initialCalculation calculationsv1.Calculation
		expected           calculationsv1.Calculation
		errorExpected      bool
	}{
		{
			id: "no calculations",
		},
		{
			id: "one calculation returns",
			initialCalculation: calculationsv1.Calculation{
				TypeMeta:   metav1.TypeMeta{Kind: "Calculation", APIVersion: "vega.io/v1"},
				ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("calc-%s", util.InputHash([]byte(fmt.Sprintf("%f", 12100.0)), []byte(fmt.Sprintf("%f", 4.0))))},
				Phase:      calculationsv1.CreatedPhase,
				Status:     calculationsv1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
				Spec:       calculationsv1.CalculationSpec{Teff: 12100.0, LogG: 4.0},
			},
			expected: calculationsv1.Calculation{
				TypeMeta:   metav1.TypeMeta{Kind: "Calculation", APIVersion: "vega.io/v1"},
				ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("calc-%s", util.InputHash([]byte(fmt.Sprintf("%f", 12100.0)), []byte(fmt.Sprintf("%f", 4.0))))},
				Phase:      calculationsv1.CreatedPhase,
				Status:     calculationsv1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
				Spec:       calculationsv1.CalculationSpec{Teff: 12100.0, LogG: 4.0},
			},
		},
		{
			id: "get calculation with wrong name",
			initialCalculation: calculationsv1.Calculation{
				TypeMeta:   metav1.TypeMeta{Kind: "Calculation", APIVersion: "vega.io/v1"},
				ObjectMeta: metav1.ObjectMeta{Name: "calc-wrong-name"},
				Phase:      calculationsv1.CreatedPhase,
				Status:     calculationsv1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
				Spec:       calculationsv1.CalculationSpec{Teff: 12100.0, LogG: 4.0},
			},
			errorExpected: true,
		},
	}

	for _, tc := range testCases {
		fakecs := fake.NewSimpleClientset()
		fakeClient := fakecs.VegaV1()

		s := server{
			logger: logrus.WithField("test-name", tc.id),
			ctx:    context.Background(),
			client: fakeClient,
		}

		_, err := fakeClient.Calculations().Create(s.ctx, &tc.initialCalculation, metav1.CreateOptions{})
		if err != nil {
			t.Errorf("Coudn't create the calculation: %v", tc.initialCalculation)
		}

		req, err := http.NewRequest("GET", "/calculation/", nil)
		if err != nil {
			t.Fatal(err)
		}

		vars := map[string]string{
			"id": tc.initialCalculation.Name,
		}
		req = mux.SetURLVars(req, vars)

		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(s.getCalculationByName)
		handler.ServeHTTP(rr, req)

		var actualData *calculationsv1.Calculation

		err = json.Unmarshal(rr.Body.Bytes(), &actualData)
		if err != nil {
			t.Fatal(err)
		}

		if diff := cmp.Diff(tc.expected, *actualData); diff != "" && !tc.errorExpected {
			t.Fatal(diff)
		}
	}
}

func TestGetCalculation(t *testing.T) {
	testCases := []struct {
		id                  string
		teff                float64
		logG                float64
		initialCalculations []calculationsv1.Calculation
		expected            calculationsv1.Calculation
	}{
		{
			id: "no calculations",
		},
		{
			id:   "one calculation returns",
			teff: 12100.0,
			logG: 4.0,
			initialCalculations: []calculationsv1.Calculation{
				{
					TypeMeta:   metav1.TypeMeta{Kind: "Calculation", APIVersion: "vega.io/v1"},
					ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("calc-%s", util.InputHash([]byte(fmt.Sprintf("%f", 12100.0)), []byte(fmt.Sprintf("%f", 4.0))))},
					Phase:      calculationsv1.CreatedPhase,
					Status:     calculationsv1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
					Spec:       calculationsv1.CalculationSpec{Teff: 12100.0, LogG: 4.0},
				},
				{
					TypeMeta:   metav1.TypeMeta{Kind: "Calculation", APIVersion: "vega.io/v1"},
					ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("calc-%s", util.InputHash([]byte(fmt.Sprintf("%f", 13100.0)), []byte(fmt.Sprintf("%f", 4.0))))},
					Phase:      calculationsv1.CreatedPhase,
					Status:     calculationsv1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
					Spec:       calculationsv1.CalculationSpec{Teff: 13100.0, LogG: 4.0},
				},
			},
			expected: calculationsv1.Calculation{
				TypeMeta:   metav1.TypeMeta{Kind: "Calculation", APIVersion: "vega.io/v1"},
				ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("calc-%s", util.InputHash([]byte(fmt.Sprintf("%f", 12100.0)), []byte(fmt.Sprintf("%f", 4.0))))},
				Phase:      calculationsv1.CreatedPhase,
				Status:     calculationsv1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
				Spec:       calculationsv1.CalculationSpec{Teff: 12100.0, LogG: 4.0},
			},
		},
	}

	for _, tc := range testCases {
		fakecs := fake.NewSimpleClientset()
		fakeClient := fakecs.VegaV1()

		s := server{
			logger: logrus.WithField("test-name", tc.id),
			ctx:    context.Background(),
			client: fakeClient,
		}

		for _, calc := range tc.initialCalculations {
			_, err := fakeClient.Calculations().Create(s.ctx, &calc, metav1.CreateOptions{})
			if err != nil {
				t.Errorf("Coudn't create the calculation: %v", calc)
			}
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

		var actualData calculationsv1.Calculation

		err = json.Unmarshal(rr.Body.Bytes(), &actualData)
		if err != nil {
			t.Fatal(err)
		}

		if diff := cmp.Diff(tc.expected, actualData); diff != "" {
			t.Fatal(diff)
		}
	}
}
