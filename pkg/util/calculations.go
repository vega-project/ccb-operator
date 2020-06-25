package util

import (
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/vega-project/ccb-operator/pkg/apis/calculations/v1"
)

// NewCalculation gets the values of teff and logG and creates a calculation
// with its minumum values
func NewCalculation(teff, logG float64) *v1.Calculation {
	calcSpec := v1.CalculationSpec{
		Teff: teff,
		LogG: logG,
		Steps: []v1.Step{
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
		},
	}

	calcName := fmt.Sprintf("calc-%s", InputHash([]byte(fmt.Sprintf("%f", teff)), []byte(fmt.Sprintf("%f", logG))))
	calculation := &v1.Calculation{
		TypeMeta:   metav1.TypeMeta{Kind: "Calculation", APIVersion: "vega.io/v1"},
		ObjectMeta: metav1.ObjectMeta{Name: calcName},
		Phase:      v1.CreatedPhase,
		Status:     v1.CalculationStatus{StartTime: metav1.Time{Time: time.Now()}},
		Spec:       calcSpec,
	}

	return calculation
}
