package util

import (
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	bulkv1 "github.com/vega-project/ccb-operator/pkg/apis/calculationbulk/v1"
	v1 "github.com/vega-project/ccb-operator/pkg/apis/calculations/v1"
)

// NewCalculation gets the values of teff and logG and creates a calculation
// with its minumum values
func NewCalculation(calc *bulkv1.Calculation) *v1.Calculation {
	if len(calc.Steps) == 0 {
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
				Command: "synspec49",
				Args:    []string{"<", "input_tlusty_fortfive"},
			},
		}
	}

	calcSpec := v1.CalculationSpec{
		Teff:  calc.Params.Teff,
		LogG:  calc.Params.LogG,
		Steps: calc.Steps,
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
