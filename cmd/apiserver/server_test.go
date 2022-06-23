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

	"github.com/gin-gonic/gin"
	"github.com/google/go-cmp/cmp"
	cmpopts "github.com/google/go-cmp/cmp/cmpopts"

	"github.com/sirupsen/logrus"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	fakectrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	bulkv1 "github.com/vega-project/ccb-operator/pkg/apis/calculationbulk/v1"
	v1 "github.com/vega-project/ccb-operator/pkg/apis/calculations/v1"
	workersv1 "github.com/vega-project/ccb-operator/pkg/apis/workers/v1"
	"github.com/vega-project/ccb-operator/pkg/util"
)

func init() {
	gin.SetMode(gin.ReleaseMode)
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
					Spec:       v1.CalculationSpec{Params: v1.Params{Teff: 16000.0, LogG: 4.0}},
				},
			},
			expected: []v1.Calculation{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "calc-1"},
					Phase:      v1.CreatedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
					Spec:       v1.CalculationSpec{Params: v1.Params{Teff: 16000.0, LogG: 4.0}},
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
					Spec:       v1.CalculationSpec{Params: v1.Params{Teff: 10000.0, LogG: 4.0}},
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
					Spec:       v1.CalculationSpec{Params: v1.Params{Teff: 11000.0, LogG: 4.0}},
				},
				&v1.Calculation{
					ObjectMeta: metav1.ObjectMeta{Name: "calc-1"},
					Phase:      v1.CreatedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
					Spec:       v1.CalculationSpec{Params: v1.Params{Teff: 12000.0, LogG: 4.0}},
				},
			},
			expected: []v1.Calculation{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "calc-1"},
					Phase:      v1.CreatedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
					Spec:       v1.CalculationSpec{Params: v1.Params{Teff: 12000.0, LogG: 4.0}},
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
					Spec:       v1.CalculationSpec{Params: v1.Params{Teff: 11000.0, LogG: 4.0}},
				},
				&v1.Calculation{
					ObjectMeta: metav1.ObjectMeta{Name: "calc-2"},
					Phase:      v1.CreatedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
					Spec:       v1.CalculationSpec{Params: v1.Params{Teff: 12000.0, LogG: 4.0}},
				},
			},
			expected: []v1.Calculation{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "calc-1"},
					Phase:      v1.CreatedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
					Spec:       v1.CalculationSpec{Params: v1.Params{Teff: 11000.0, LogG: 4.0}},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "calc-2"},
					Phase:      v1.CreatedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
					Spec:       v1.CalculationSpec{Params: v1.Params{Teff: 12000.0, LogG: 4.0}},
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

			req, err := http.NewRequest("DELETE", fmt.Sprintf("/calculations/delete/%s", tc.calculationToDelete), nil)
			if err != nil {
				t.Fatal(err)
			}

			r := gin.Default()
			r.DELETE("/calculations/delete/:id", s.deleteCalculation)

			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, req)

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
					Spec:       v1.CalculationSpec{Params: v1.Params{Teff: 10000.0, LogG: 4.0}},
				},
			},
			expected: []v1.Calculation{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "calc-1"},
					Phase:      v1.CreatedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
					Spec:       v1.CalculationSpec{Params: v1.Params{Teff: 10000.0, LogG: 4.0}},
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
					Spec:       v1.CalculationSpec{Params: v1.Params{Teff: 11000.0, LogG: 4.0}},
				},
				&v1.Calculation{
					ObjectMeta: metav1.ObjectMeta{Name: "calc-3"},
					Phase:      v1.CreatedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
					Spec:       v1.CalculationSpec{Params: v1.Params{Teff: 12000.0, LogG: 4.0}},
				},
			},
			expected: []v1.Calculation{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "calc-2"},
					Phase:      v1.CreatedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
					Spec:       v1.CalculationSpec{Params: v1.Params{Teff: 11000.0, LogG: 4.0}},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "calc-3"},
					Phase:      v1.CreatedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
					Spec:       v1.CalculationSpec{Params: v1.Params{Teff: 12000.0, LogG: 4.0}},
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

		r := gin.Default()
		r.GET("/calculations", s.getCalculations)

		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusOK {
			t.Errorf("Handler returned wrong status code: got %v want %v",
				status, http.StatusOK)
		}

		var actualData struct {
			Data *v1.CalculationList `json:"data,omitempty"`
		}

		err = json.Unmarshal(rr.Body.Bytes(), &actualData)
		if err != nil {
			t.Fatal(err)
		}

		if diff := cmp.Diff(tc.expected, actualData.Data.Items, cmpopts.IgnoreFields(metav1.ObjectMeta{}, "ResourceVersion")); diff != "" {
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
			id: "one calculation returns",
			initialCalculation: []ctrlruntimeclient.Object{
				&v1.Calculation{
					ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("calc-%s", util.InputHash([]byte(fmt.Sprintf("%f", 12100.0)), []byte(fmt.Sprintf("%f", 4.0))))},
					Phase:      v1.CreatedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
					Spec:       v1.CalculationSpec{Params: v1.Params{Teff: 12100.0, LogG: 4.0}},
				},
			},
			expected: v1.Calculation{
				ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("calc-%s", util.InputHash([]byte(fmt.Sprintf("%f", 12100.0)), []byte(fmt.Sprintf("%f", 4.0))))},
				Phase:      v1.CreatedPhase,
				Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
				Spec:       v1.CalculationSpec{Params: v1.Params{Teff: 12100.0, LogG: 4.0}},
			},
		},
		{
			id: "get calculation with wrong name",
			initialCalculation: []ctrlruntimeclient.Object{
				&v1.Calculation{
					ObjectMeta: metav1.ObjectMeta{Name: "calc-wrong-name"},
					Phase:      v1.CreatedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
					Spec:       v1.CalculationSpec{Params: v1.Params{Teff: 12100.0, LogG: 4.0}},
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

		req, err := http.NewRequest("GET", fmt.Sprintf("/calculation/%s", tc.initialCalculation[0].GetName()), nil)
		if err != nil {
			t.Fatal(err)
		}

		r := gin.Default()
		r.GET("/calculation/:id", s.getCalculationByName)

		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		var actualData struct {
			Data *v1.Calculation `json:"data,omitempty"`
		}

		err = json.Unmarshal(rr.Body.Bytes(), &actualData)
		if err != nil {
			t.Fatal(err)
		}

		if diff := cmp.Diff(tc.expected, *actualData.Data,
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
					Spec:       v1.CalculationSpec{Params: v1.Params{Teff: 12100.0, LogG: 4.0}},
				},
				&v1.Calculation{
					ObjectMeta: metav1.ObjectMeta{Name: "calc-2"},
					Phase:      v1.CreatedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
					Spec:       v1.CalculationSpec{Params: v1.Params{Teff: 13100.0, LogG: 4.0}},
				},
			},
			expected: []v1.Calculation{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "calc-1"},
					Phase:      v1.CreatedPhase,
					Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)}},
					Spec:       v1.CalculationSpec{Params: v1.Params{Teff: 12100.0, LogG: 4.0}},
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

		r := gin.Default()
		r.GET("/calculation", s.getCalculation)

		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		var actualData struct {
			Data []v1.Calculation `json:"data,omitempty"`
		}

		if err := json.Unmarshal(rr.Body.Bytes(), &actualData); err != nil {
			t.Fatal(err)
		}

		if diff := cmp.Diff(tc.expected, actualData.Data, cmpopts.IgnoreFields(metav1.ObjectMeta{}, "ResourceVersion")); diff != "" {
			t.Fatal(diff)
		}
	}
}

func TestGetCalculationBulkByName(t *testing.T) {
	testCases := []struct {
		id                      string
		name                    string
		initialCalculationBulks []ctrlruntimeclient.Object
		expected                *bulkv1.CalculationBulk
		errorExpected           bool
	}{
		{
			id:   "one calculation returns",
			name: "test-bulk",
			initialCalculationBulks: []ctrlruntimeclient.Object{
				&bulkv1.CalculationBulk{
					ObjectMeta: metav1.ObjectMeta{Name: "test-bulk", Namespace: "vega"},
					WorkerPool: "test-worker-pool",
				},
			},
			expected: &bulkv1.CalculationBulk{
				ObjectMeta: metav1.ObjectMeta{Name: "test-bulk", Namespace: "vega"},
				WorkerPool: "test-worker-pool",
			},
		},
		{
			id:   "get calculation with wrong name",
			name: "test-bulk",
			initialCalculationBulks: []ctrlruntimeclient.Object{
				&bulkv1.CalculationBulk{
					ObjectMeta: metav1.ObjectMeta{Name: "test-bulk-another", Namespace: "vega"},
					WorkerPool: "test-worker-pool",
				},
			},
			errorExpected: true,
		},
	}

	for _, tc := range testCases {
		fakeClient := fakectrlruntimeclient.NewClientBuilder().WithObjects(tc.initialCalculationBulks...).Build()

		s := server{
			logger:    logrus.WithField("test-name", tc.id),
			ctx:       context.Background(),
			client:    fakeClient,
			namespace: "vega",
		}

		req, err := http.NewRequest("GET", fmt.Sprintf("/bulk/%s", tc.name), nil)
		if err != nil {
			t.Fatal(err)
		}

		r := gin.Default()
		r.GET("/bulk/:id", s.getCalculationBulkByName)

		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		if rr.Result().StatusCode == http.StatusOK && tc.errorExpected {
			t.Fatal("expected error, got 200")
		}

		if rr.Result().StatusCode != http.StatusOK && !tc.errorExpected {
			t.Fatalf("didn't expected error, got %s", rr.Body.Bytes())
		}

		var actualData struct {
			Data *bulkv1.CalculationBulk `json:"data,omitempty"`
		}

		if err := json.Unmarshal(rr.Body.Bytes(), &actualData); err != nil {
			t.Fatal(err)
		}

		if diff := cmp.Diff(tc.expected, actualData.Data,
			cmpopts.IgnoreFields(metav1.TypeMeta{}, "Kind", "APIVersion"),
			cmpopts.IgnoreFields(metav1.ObjectMeta{}, "ResourceVersion")); diff != "" {
			t.Fatal(diff)
		}
	}
}

func TestCreateCalculationBulk(t *testing.T) {
	testCases := []struct {
		id           string
		body         string
		initialBulks []ctrlruntimeclient.Object
		expected     []bulkv1.CalculationBulk
	}{
		{
			id: "no initial calculations in cluster",
			body: `{
				"worker_pool": "vega-pool",
				"calculations": {
					"calc-test-1": {
						"params": {
							"log_g": 4,
							"teff": 10100
						}
					},
					"calc-test-2": {
						"params": {
							"log_g": 4,
							"teff": 10200
						}
					},
					"calc-test-3": {
						"params": {
							"log_g": 4,
							"teff": 10300
						}
					}
				}
			}`,
			expected: []bulkv1.CalculationBulk{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "bulk-rcmmg0k8fjkpb29b"},
					WorkerPool: "vega-pool",
					Calculations: map[string]bulkv1.Calculation{
						"calc-test-1": {Params: v1.Params{Teff: 10100, LogG: 4.0}},
						"calc-test-2": {Params: v1.Params{Teff: 10200, LogG: 4.0}},
						"calc-test-3": {Params: v1.Params{Teff: 10300, LogG: 4.0}},
					},
					Status: bulkv1.CalculationBulkStatus{State: bulkv1.CalculationBulkAvailableState},
				},
			},
		},
		{
			id: "initial calculations in cluster",
			initialBulks: []ctrlruntimeclient.Object{
				&bulkv1.CalculationBulk{ObjectMeta: metav1.ObjectMeta{Name: "bulk-12345"}},
			},
			body: `{
				"worker_pool": "vega-pool",
				"calculations": {
					"calc-test-1": {
						"params": {
							"log_g": 4,
							"teff": 10100
						}
					},
					"calc-test-2": {
						"params": {
							"log_g": 4,
							"teff": 10200
						}
					},
					"calc-test-3": {
						"params": {
							"log_g": 4,
							"teff": 10300
						}
					}
				}
			}`,
			expected: []bulkv1.CalculationBulk{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "bulk-12345"},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "bulk-rcmmg0k8fjkpb29b"},
					WorkerPool: "vega-pool",
					Calculations: map[string]bulkv1.Calculation{
						"calc-test-1": {Params: v1.Params{Teff: 10100, LogG: 4.0}},
						"calc-test-2": {Params: v1.Params{Teff: 10200, LogG: 4.0}},
						"calc-test-3": {Params: v1.Params{Teff: 10300, LogG: 4.0}},
					},
					Status: bulkv1.CalculationBulkStatus{State: bulkv1.CalculationBulkAvailableState},
				},
			},
		},
	}

	for _, tc := range testCases {
		fakeClient := fakectrlruntimeclient.NewClientBuilder().WithObjects(tc.initialBulks...).Build()
		s := server{
			logger: logrus.WithField("test-name", tc.id),
			ctx:    context.Background(),
			client: fakeClient,
		}

		req, err := http.NewRequest("POST", "/bulk/create", bytes.NewBuffer([]byte(tc.body)))
		if err != nil {
			t.Fatal(err)
		}

		r := gin.Default()
		r.POST("/bulk/create", s.createCalculationBulk)

		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		var actualData struct {
			Data *bulkv1.CalculationBulk `json:"data,omitempty"`
		}

		if err := json.Unmarshal(rr.Body.Bytes(), &actualData); err != nil {
			t.Fatal(err)
		}

		var bulkList bulkv1.CalculationBulkList
		if err := fakeClient.List(s.ctx, &bulkList); err != nil {
			t.Fatal(err)
		}

		if diff := cmp.Diff(tc.expected, bulkList.Items,
			cmpopts.IgnoreFields(metav1.Time{}, "Time"),
			cmpopts.IgnoreFields(metav1.ObjectMeta{}, "ResourceVersion"),
			cmpopts.IgnoreFields(metav1.TypeMeta{}, "Kind", "APIVersion")); diff != "" {
			t.Fatal(diff)
		}
	}
}

func TestGetWorkerPools(t *testing.T) {
	testCases := []struct {
		id                 string
		initialWorkerPools []ctrlruntimeclient.Object
		expected           []workersv1.WorkerPool
	}{
		{
			id: "get no workerpool",
		},
		{
			id: "get one workerpool",
			initialWorkerPools: []ctrlruntimeclient.Object{
				&workersv1.WorkerPool{
					ObjectMeta: metav1.ObjectMeta{Name: "workerpool-1"},
					Spec: workersv1.WorkerPoolSpec{
						Workers: map[string]workersv1.Worker{
							"worker1": {Name: "worker-name", RegisteredTime: &metav1.Time{}, State: workersv1.WorkerUnknownState, LastUpdateTime: &metav1.Time{}, CalculationsProcessed: 0},
						},
					},
					TypeMeta: metav1.TypeMeta{Kind: "WorkerPool", APIVersion: "1.0"},
					Status: workersv1.WorkerPoolStatus{
						CreationTime:   &metav1.Time{},
						PendingTime:    &metav1.Time{},
						CompletionTime: &metav1.Time{},
					},
				},
			},
			expected: []workersv1.WorkerPool{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "workerpool-1"},
					Spec: workersv1.WorkerPoolSpec{
						Workers: map[string]workersv1.Worker{
							"worker1": {Name: "worker-name", RegisteredTime: nil, State: workersv1.WorkerUnknownState, LastUpdateTime: nil, CalculationsProcessed: 0},
						},
					},
					TypeMeta: metav1.TypeMeta{Kind: "WorkerPool", APIVersion: "1.0"},
					Status: workersv1.WorkerPoolStatus{
						CreationTime:   nil,
						PendingTime:    nil,
						CompletionTime: nil,
					},
				},
			},
		},
		{
			id: "get two workerpools",
			initialWorkerPools: []ctrlruntimeclient.Object{
				&workersv1.WorkerPool{
					ObjectMeta: metav1.ObjectMeta{Name: "workerpool-2"},
					Spec: workersv1.WorkerPoolSpec{
						Workers: map[string]workersv1.Worker{
							"worker1": {Name: "worker-name", RegisteredTime: &metav1.Time{}, State: workersv1.WorkerUnknownState, LastUpdateTime: &metav1.Time{}, CalculationsProcessed: 0},
						},
					},
					TypeMeta: metav1.TypeMeta{Kind: "WorkerPool", APIVersion: "1.0"},
					Status: workersv1.WorkerPoolStatus{
						CreationTime:   &metav1.Time{},
						PendingTime:    &metav1.Time{},
						CompletionTime: &metav1.Time{},
					},
				},
				&workersv1.WorkerPool{
					ObjectMeta: metav1.ObjectMeta{Name: "workerpool-3"},
					Spec: workersv1.WorkerPoolSpec{
						Workers: map[string]workersv1.Worker{
							"worker1": {Name: "worker-name", RegisteredTime: &metav1.Time{}, State: workersv1.WorkerUnknownState, LastUpdateTime: &metav1.Time{}, CalculationsProcessed: 0},
						},
					},
					TypeMeta: metav1.TypeMeta{Kind: "WorkerPool", APIVersion: "1.0"},
					Status: workersv1.WorkerPoolStatus{
						CreationTime:   &metav1.Time{},
						PendingTime:    &metav1.Time{},
						CompletionTime: &metav1.Time{},
					},
				},
			},
			expected: []workersv1.WorkerPool{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "workerpool-2"},
					Spec: workersv1.WorkerPoolSpec{
						Workers: map[string]workersv1.Worker{
							"worker1": {Name: "worker-name", RegisteredTime: nil, State: workersv1.WorkerUnknownState, LastUpdateTime: nil, CalculationsProcessed: 0},
						},
					},
					TypeMeta: metav1.TypeMeta{Kind: "WorkerPool", APIVersion: "1.0"},
					Status: workersv1.WorkerPoolStatus{
						CreationTime:   nil,
						PendingTime:    nil,
						CompletionTime: nil,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "workerpool-3"},
					Spec: workersv1.WorkerPoolSpec{
						Workers: map[string]workersv1.Worker{
							"worker1": {Name: "worker-name", RegisteredTime: nil, State: workersv1.WorkerUnknownState, LastUpdateTime: nil, CalculationsProcessed: 0},
						},
					},
					TypeMeta: metav1.TypeMeta{Kind: "WorkerPool", APIVersion: "1.0"},
					Status: workersv1.WorkerPoolStatus{
						CreationTime:   nil,
						PendingTime:    nil,
						CompletionTime: nil,
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		fakeClient := fakectrlruntimeclient.NewClientBuilder().WithObjects(tc.initialWorkerPools...).Build()

		s := server{
			logger: logrus.WithField("test-name", tc.id),
			ctx:    context.Background(),
			client: fakeClient,
		}

		req, err := http.NewRequest("GET", "/workerpools", nil)
		if err != nil {
			t.Fatal(err)
		}

		r := gin.Default()
		r.GET("/workerpools", s.getWorkerPools)

		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusOK {
			t.Errorf("Handler returned wrong status code: got %v want %v",
				status, http.StatusOK)
		}

		var actualData struct {
			Data *workersv1.WorkerPoolList `json:"data,omitempty"`
		}

		err = json.Unmarshal(rr.Body.Bytes(), &actualData)
		if err != nil {
			t.Fatal(err)
		}

		if diff := cmp.Diff(tc.expected, actualData.Data.Items, cmpopts.IgnoreFields(metav1.ObjectMeta{}, "ResourceVersion")); diff != "" {
			t.Fatal(diff)
		}
	}
}
