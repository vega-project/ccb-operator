package db

import (
	"gorm.io/gorm"
)

type CalculationResults struct {
	gorm.Model

	Parameters     map[string]string `gorm:"-"`
	ParametersJSON string            `gorm:"column:parameters_json;type:jsonb"`
	Results        string
}
