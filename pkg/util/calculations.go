package util

import (
	"fmt"
	"sort"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	bulkv1 "github.com/vega-project/ccb-operator/pkg/apis/calculationbulk/v1"
	v1 "github.com/vega-project/ccb-operator/pkg/apis/calculations/v1"
)

// NewCalculation gets the values of teff and logG and creates a calculation
// with its minumum values
func NewCalculation(calc *bulkv1.Calculation) *v1.Calculation {
	if calc.Pipeline == v1.VegaPipeline {
		calc.Steps = []v1.Step{
			{
				Command: "atlas12_ada",
				Args:    []string{"s"},
			},
			{
				Command: "atlas12_ada",
				Args:    []string{"r"},
			},
			{
				Command: "/bin/bash",
				Args:    []string{"-c", "synspec49 < input_tlusty_fortfive"},
			},
		}
	}

	calcSpec := v1.CalculationSpec{
		Params: calc.Params,
		Steps:  calc.Steps,
	}

	calcName := GetCalculationName(*calc)
	calculation := &v1.Calculation{
		ObjectMeta: metav1.ObjectMeta{Name: calcName},
		Phase:      v1.CreatedPhase,
		Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Now()}},
		Spec:       calcSpec,
	}

	return calculation
}

func GetCalculationName(calc bulkv1.Calculation) string {
	return fmt.Sprintf("calc-%s", InputHash([]byte(fmt.Sprintf("%v", calc))))
}

func IsFinishedCalculation(steps []v1.Step) bool {
	for _, step := range steps {
		if step.Status == "" {
			return false
		}
	}
	return true
}

func GetCalculationFinalPhase(steps []v1.Step) v1.CalculationPhase {
	if hasFailedStep(steps) {
		return v1.FailedPhase
	}
	return v1.CompletedPhase
}

func hasFailedStep(steps []v1.Step) bool {
	for _, step := range steps {
		if step.Status == "Failed" {
			return true
		}
	}
	return false
}

type sortedCalculations struct {
	Items []item
}

type item struct {
	Name        string
	Calculation bulkv1.Calculation
}

func GetSortedCreatedCalculations(calcs map[string]bulkv1.Calculation) sortedCalculations {
	keys := make([]string, 0, len(calcs))
	for k, v := range calcs {
		if v.Phase == "" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	var sorted sortedCalculations
	for _, key := range keys {
		sorted.Items = append(sorted.Items, item{
			Name:        key,
			Calculation: calcs[key],
		})

	}
	return sorted
}
