package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	calculationsv1 "github.com/vega-project/ccb-operator/pkg/apis/calculations/v1"
	v1 "github.com/vega-project/ccb-operator/pkg/apis/calculations/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:shortName=calcbulk
// +resource:path=calculationbulk

type CalculationBulk struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	WorkerPool   string                 `json:"workerPool,omitempty"`
	Calculations map[string]Calculation `json:"calculations,omitempty"`
	Status       CalculationBulkStatus  `json:"status,omitempty"`
}

type Calculation struct {
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Params Params                          `json:"params,omitempty"`
	Steps  []v1.Step                       `json:"steps,omitempty"`
	Phase  calculationsv1.CalculationPhase `json:"phase,omitempty"`
}

type Params struct {
	LogG float64 `json:"log_g,omitempty"`
	Teff float64 `json:"teff,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +resource:path=calculationbulks

type CalculationBulkList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []CalculationBulk `json:"items"`
}

type CalculationBulkStatus struct {
	CreatedTime    metav1.Time          `json:"startTime,omitempty"`
	CompletionTime *metav1.Time         `json:"completionTime,omitempty"`
	State          CalculationBulkState `json:"state,omitempty"`
}

type CalculationBulkState string

const (
	CalculationBulkAvailableState  CalculationBulkState = "Available"
	CalculationBulkProcessingState CalculationBulkState = "Processing"
	CalculationBulkUnknownState    CalculationBulkState = "Unknown"
)
