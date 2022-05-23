package util

import v1 "github.com/vega-project/ccb-operator/pkg/apis/calculations/v1"

type Result struct {
	CalcName     string
	Step         int
	Status       v1.CalculationPhase
	StdoutStderr string
	CommandError error
}
