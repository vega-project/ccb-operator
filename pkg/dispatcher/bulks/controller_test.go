package bulks

import (
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/sirupsen/logrus"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	bulkv1 "github.com/vega-project/ccb-operator/pkg/apis/calculationbulk/v1"
	v1 "github.com/vega-project/ccb-operator/pkg/apis/calculations/v1"
	workersv1 "github.com/vega-project/ccb-operator/pkg/apis/workers/v1"
	proto "github.com/vega-project/ccb-operator/proto"
)

func Test_assignCalculationsToWorkers(t *testing.T) {
	tests := []struct {
		name       string
		bulk       *bulkv1.CalculationBulk
		workerpool *workersv1.WorkerPool
		namespace  string
		want       []v1.Calculation
	}{
		{
			name: "2 calculations, 3 workers available - expect 2 calculations assigned to 2 workers",
			bulk: &bulkv1.CalculationBulk{
				Calculations: map[string]bulkv1.Calculation{
					"calc1": {Pipeline: v1.VegaPipeline, Params: v1.Params{LogG: 4.0, Teff: 10000.0}},
					"calc2": {Pipeline: v1.VegaPipeline, Params: v1.Params{LogG: 4.0, Teff: 11000.0}},
					"calc3": {Pipeline: v1.VegaPipeline, Params: v1.Params{LogG: 4.0, Teff: 12000.0}},
				},
			},
			workerpool: &workersv1.WorkerPool{
				Spec: workersv1.WorkerPoolSpec{
					Workers: map[string]workersv1.Worker{
						"worker1-node": {
							Name:  "worker1",
							State: workersv1.WorkerAvailableState,
						},
						"worker2-node": {
							Name:  "worker2",
							State: workersv1.WorkerAvailableState,
						},
					},
				},
			},
			want: []v1.Calculation{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "calc-1wij91455czrwswi",
						Labels: map[string]string{
							"vegaproject.io/assign":          "worker1",
							"vegaproject.io/bulk":            "",
							"vegaproject.io/calculationName": "calc1",
							"vegaproject.io/rootFolder":      "",
						},
					},
					Spec: v1.CalculationSpec{
						Steps: []v1.Step{
							{Command: "atlas12_ada", Args: []string{"s"}},
							{Command: "atlas12_ada", Args: []string{"r"}},
							{Command: "/bin/bash", Args: []string{"-c", "synspec49 < input_tlusty_fortfive"}},
						},
						Params: v1.Params{LogG: 4.0, Teff: 10000.0},
					},

					Pipeline: "vega",
					Assign:   "worker1",
					Phase:    "Created",
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "calc-xmlv9xmmc2mcgjq0",
						Labels: map[string]string{
							"vegaproject.io/assign":          "worker2",
							"vegaproject.io/bulk":            "",
							"vegaproject.io/calculationName": "calc2",
							"vegaproject.io/rootFolder":      "",
						},
					},
					Spec: v1.CalculationSpec{
						Steps: []v1.Step{
							{Command: "atlas12_ada", Args: []string{"s"}},
							{Command: "atlas12_ada", Args: []string{"r"}},
							{Command: "/bin/bash", Args: []string{"-c", "synspec49 < input_tlusty_fortfive"}},
						},
						Params: v1.Params{LogG: 4.0, Teff: 11000.0},
					},
					Pipeline: "vega",
					Assign:   "worker2",
					Phase:    "Created",
				},
			},
		},

		{
			name: "more calculations than workers available - expect 2 calculations assigned to 2 workers",
			bulk: &bulkv1.CalculationBulk{
				Calculations: map[string]bulkv1.Calculation{
					"calc1": {Pipeline: v1.VegaPipeline, Params: v1.Params{LogG: 4.0, Teff: 10000.0}},
					"calc2": {Pipeline: v1.VegaPipeline, Params: v1.Params{LogG: 4.0, Teff: 11000.0}},
					"calc3": {Pipeline: v1.VegaPipeline, Params: v1.Params{LogG: 4.0, Teff: 12000.0}},
					"calc4": {Pipeline: v1.VegaPipeline, Params: v1.Params{LogG: 4.0, Teff: 13000.0}},
					"calc5": {Pipeline: v1.VegaPipeline, Params: v1.Params{LogG: 4.0, Teff: 14000.0}},
				},
			},
			workerpool: &workersv1.WorkerPool{
				Spec: workersv1.WorkerPoolSpec{
					Workers: map[string]workersv1.Worker{
						"worker1-node": {
							Name:  "worker1",
							State: workersv1.WorkerAvailableState,
						},
						"worker2-node": {
							Name:  "worker2",
							State: workersv1.WorkerAvailableState,
						},
					},
				},
			},
			want: []v1.Calculation{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "calc-1wij91455czrwswi",
						Labels: map[string]string{
							"vegaproject.io/assign":          "worker1",
							"vegaproject.io/bulk":            "",
							"vegaproject.io/calculationName": "calc1",
							"vegaproject.io/rootFolder":      "",
						},
					},
					Spec: v1.CalculationSpec{
						Steps: []v1.Step{
							{Command: "atlas12_ada", Args: []string{"s"}},
							{Command: "atlas12_ada", Args: []string{"r"}},
							{Command: "/bin/bash", Args: []string{"-c", "synspec49 < input_tlusty_fortfive"}},
						},
						Params: v1.Params{LogG: 4.0, Teff: 10000.0},
					},

					Pipeline: "vega",
					Assign:   "worker1",
					Phase:    "Created",
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "calc-xmlv9xmmc2mcgjq0",
						Labels: map[string]string{
							"vegaproject.io/assign":          "worker2",
							"vegaproject.io/bulk":            "",
							"vegaproject.io/calculationName": "calc2",
							"vegaproject.io/rootFolder":      "",
						},
					},
					Spec: v1.CalculationSpec{
						Steps: []v1.Step{
							{Command: "atlas12_ada", Args: []string{"s"}},
							{Command: "atlas12_ada", Args: []string{"r"}},
							{Command: "/bin/bash", Args: []string{"-c", "synspec49 < input_tlusty_fortfive"}},
						},
						Params: v1.Params{LogG: 4.0, Teff: 11000.0},
					},
					Pipeline: "vega",
					Assign:   "worker2",
					Phase:    "Created",
				},
			},
		},
		{
			name: "1 calculation - 2 processed calculations, 2 workers available - expect 1 calculations assigned to 1 worker",
			bulk: &bulkv1.CalculationBulk{
				Calculations: map[string]bulkv1.Calculation{
					"calc1": {Pipeline: v1.VegaPipeline, Params: v1.Params{LogG: 4.0, Teff: 10000.0}},
					"calc2": {Pipeline: v1.VegaPipeline, Params: v1.Params{LogG: 4.0, Teff: 11000.0}, Phase: v1.ProcessingPhase},
					"calc3": {Pipeline: v1.VegaPipeline, Params: v1.Params{LogG: 4.0, Teff: 12000.0}, Phase: v1.CompletedPhase},
				},
			},
			workerpool: &workersv1.WorkerPool{
				Spec: workersv1.WorkerPoolSpec{
					Workers: map[string]workersv1.Worker{
						"worker1-node": {
							Name:  "worker1",
							State: workersv1.WorkerAvailableState,
						},
						"worker2-node": {
							Name:  "worker2",
							State: workersv1.WorkerAvailableState,
						},
					},
				},
			},
			want: []v1.Calculation{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "calc-1wij91455czrwswi",
						Labels: map[string]string{
							"vegaproject.io/assign":          "worker1",
							"vegaproject.io/bulk":            "",
							"vegaproject.io/calculationName": "calc1",
							"vegaproject.io/rootFolder":      "",
						},
					},
					Spec: v1.CalculationSpec{
						Steps: []v1.Step{
							{Command: "atlas12_ada", Args: []string{"s"}},
							{Command: "atlas12_ada", Args: []string{"r"}},
							{Command: "/bin/bash", Args: []string{"-c", "synspec49 < input_tlusty_fortfive"}},
						},
						Params: v1.Params{LogG: 4.0, Teff: 10000.0},
					},

					Pipeline: "vega",
					Assign:   "worker1",
					Phase:    "Created",
				},
			},
		},
		{
			name: "3 calculation, no workers available - expect no calculations assigned to workers",
			bulk: &bulkv1.CalculationBulk{
				Calculations: map[string]bulkv1.Calculation{
					"calc1": {Pipeline: v1.VegaPipeline, Params: v1.Params{LogG: 4.0, Teff: 10000.0}},
					"calc2": {Pipeline: v1.VegaPipeline, Params: v1.Params{LogG: 4.0, Teff: 11000.0}},
					"calc3": {Pipeline: v1.VegaPipeline, Params: v1.Params{LogG: 4.0, Teff: 12000.0}},
				},
			},
			workerpool: &workersv1.WorkerPool{
				Spec: workersv1.WorkerPoolSpec{
					Workers: map[string]workersv1.Worker{
						"worker1-node": {
							Name:  "worker1",
							State: workersv1.WorkerProcessingState,
						},
						"worker2-node": {
							Name:  "worker2",
							State: workersv1.WorkerProcessingState,
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diff := cmp.Diff(assignCalculationsToWorkers(tt.bulk, tt.workerpool, tt.namespace), tt.want, cmpopts.IgnoreFields(metav1.Time{}, "Time")); diff != "" {
				t.Fatal(diff)
			}
		})
	}
}

type fakeResults struct {
	parameters map[string]string
	createdAt  time.Time
	results    string
}

type fakeGRPCClient struct {
	results []fakeResults
}

func (f *fakeGRPCClient) StoreData(parameters map[string]string, results string) (*proto.StoreResponse, error) {
	return nil, nil
}

func (f *fakeGRPCClient) GetData(parameters map[string]string) (*proto.GetDataResponse, error) {
	for _, result := range f.results {
		if reflect.DeepEqual(result.parameters, parameters) {
			return &proto.GetDataResponse{
				Results:   result.results,
				CreatedAt: result.createdAt.Format(time.RFC3339),
			}, nil
		}
	}
	return nil, nil
}

func (f *fakeGRPCClient) Close() error {
	return nil
}

func Test_reconciler_reconcileCalculations(t *testing.T) {
	tests := []struct {
		name             string
		results          []fakeResults
		calcs            map[string]bulkv1.Calculation
		bulkCreationTime time.Time
		wantErr          bool
		expectedCalcs    map[string]bulkv1.Calculation
	}{
		{
			name: "no results",
			calcs: map[string]bulkv1.Calculation{
				"calc1": {Pipeline: v1.VegaPipeline, Params: v1.Params{LogG: 4.0, Teff: 10000.0}},
				"calc2": {Pipeline: v1.VegaPipeline, Params: v1.Params{LogG: 4.0, Teff: 11000.0}},
				"calc3": {Pipeline: v1.VegaPipeline, Params: v1.Params{LogG: 4.0, Teff: 12000.0}},
			},
			results:          []fakeResults{},
			bulkCreationTime: time.Now(),
			expectedCalcs: map[string]bulkv1.Calculation{
				"calc1": {Pipeline: v1.VegaPipeline, Params: v1.Params{LogG: 4.0, Teff: 10000.0}},
				"calc2": {Pipeline: v1.VegaPipeline, Params: v1.Params{LogG: 4.0, Teff: 11000.0}},
				"calc3": {Pipeline: v1.VegaPipeline, Params: v1.Params{LogG: 4.0, Teff: 12000.0}},
			},
		},

		{
			name: "results for some calculations",
			calcs: map[string]bulkv1.Calculation{
				"calc1": {Pipeline: v1.VegaPipeline, Params: v1.Params{LogG: 4.000000, Teff: 10000.000000}},
				"calc2": {Pipeline: v1.VegaPipeline, Params: v1.Params{LogG: 4.000000, Teff: 11000.000000}},
				"calc3": {Pipeline: v1.VegaPipeline, Params: v1.Params{LogG: 4.000000, Teff: 12000.000000}},
			},
			results: []fakeResults{
				{
					parameters: map[string]string{"log_g": "4.000000", "teff": "10000.000000"},
					results:    "results1",
					createdAt:  time.Now().Add(-24 * time.Hour),
				},
			},
			bulkCreationTime: time.Now(),
			expectedCalcs: map[string]bulkv1.Calculation{
				"calc1": {Pipeline: v1.VegaPipeline, Params: v1.Params{LogG: 4.000000, Teff: 10000.000000}, Phase: v1.CachedPhase},
				"calc2": {Pipeline: v1.VegaPipeline, Params: v1.Params{LogG: 4.000000, Teff: 11000.000000}},
				"calc3": {Pipeline: v1.VegaPipeline, Params: v1.Params{LogG: 4.000000, Teff: 12000.000000}},
			},
		},

		{
			name: "results for some calculations, some results are newer than the bulk creation time",
			calcs: map[string]bulkv1.Calculation{
				"calc1": {Pipeline: v1.VegaPipeline, Params: v1.Params{LogG: 4.000000, Teff: 10000.000000}},
				"calc2": {Pipeline: v1.VegaPipeline, Params: v1.Params{LogG: 4.000000, Teff: 11000.000000}},
				"calc3": {Pipeline: v1.VegaPipeline, Params: v1.Params{LogG: 4.000000, Teff: 12000.000000}},
			},
			results: []fakeResults{
				{
					parameters: map[string]string{"log_g": "4.000000", "teff": "10000.000000"},
					results:    "results1",
					createdAt:  time.Now().Add(-24 * time.Hour),
				},
				{
					parameters: map[string]string{"log_g": "4.000000", "teff": "11000.000000"},
					results:    "results1",
					createdAt:  time.Now().Add(+1 * time.Hour),
				},
			},
			bulkCreationTime: time.Now(),
			expectedCalcs: map[string]bulkv1.Calculation{
				"calc1": {Pipeline: v1.VegaPipeline, Params: v1.Params{LogG: 4.000000, Teff: 10000.000000}, Phase: v1.CachedPhase},
				"calc2": {Pipeline: v1.VegaPipeline, Params: v1.Params{LogG: 4.000000, Teff: 11000.000000}},
				"calc3": {Pipeline: v1.VegaPipeline, Params: v1.Params{LogG: 4.000000, Teff: 12000.000000}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &reconciler{
				logger:     logrus.WithField("name", tt.name),
				gRPCClient: &fakeGRPCClient{results: tt.results},
			}
			if err := r.reconcileCalculations(tt.calcs, tt.bulkCreationTime); (err != nil) != tt.wantErr {
				t.Errorf("reconciler.reconcileCalculations() error = %v, wantErr %v", err, tt.wantErr)
			}

			if diff := cmp.Diff(tt.calcs, tt.expectedCalcs); diff != "" {
				t.Fatal(diff)
			}
		})
	}
}
