package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CalculationConditionType string
type CalculationPhase string

const (
	CreatedPhase    CalculationPhase = "Created"
	ProcessingPhase CalculationPhase = "Processing"
	CompletedPhase  CalculationPhase = "Completed"
	FailedPhase     CalculationPhase = "Failed"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:shortName=calc
// +resource:path=calculation

type Calculation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec       CalculationSpec   `json:"spec,omitempty"`
	Pipeline   Pipeline          `json:"pipeline,omitempty"`
	Assign     string            `json:"assign,omitempty"`
	InputFiles *InputFiles       `json:"input_files,omitempty"`
	Status     CalculationStatus `json:"status,omitempty"`
	Phase      CalculationPhase  `json:"phase,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +resource:path=calculations

type CalculationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []Calculation `json:"items"`
}

type CalculationSpec struct {
	Steps  []Step `json:"steps,omitempty"`
	Params Params `json:"params,omitempty"`
}

type Params struct {
	LogG float64 `json:"log_g,omitempty"`
	Teff float64 `json:"teff,omitempty"`
}

type Step struct {
	Command string           `json:"command"`
	Args    []string         `json:"args"`
	Status  CalculationPhase `json:"status,omitempty"`
}

type InputFiles struct {
	Files   []string `json:"files,omitempty"`
	Symlink bool     `json:"symlink,omitempty"`
}

type Pipeline string

const VegaPipeline Pipeline = "vega"

type CalculationStatus struct {
	//Conditions represent the latest available observations of an object's current state:
	// StartTime is equal to the creation time of the Calculation
	StartTime metav1.Time `json:"startTime,omitempty"`
	// PendingTime is the timestamp for when the job moved from triggered to pending
	PendingTime *metav1.Time `json:"pendingTime,omitempty"`
	// CompletionTime is the timestamp for when the job goes to a final state
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`
}
