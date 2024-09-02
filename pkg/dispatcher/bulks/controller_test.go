package bulks

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	bulkv1 "github.com/vega-project/ccb-operator/pkg/apis/calculationbulk/v1"
	v1 "github.com/vega-project/ccb-operator/pkg/apis/calculations/v1"
	workersv1 "github.com/vega-project/ccb-operator/pkg/apis/workers/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
