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

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +resource:path=calculation
// +genclient:noStatus
// +genclient:nonNamespaced

type Calculation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CalculationSpec   `json:"spec"`
	DBKey  string            `json:"dbkey"`
	Assign string            `json:"assign"`
	Status CalculationStatus `json:"status"`
	Phase  CalculationPhase  `json:"phase"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +resource:path=calculations

type CalculationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []Calculation `json:"items"`
}

type CalculationSpec struct {
	Steps []Step  `json: "steps, omitempty"`
	Teff  float64 `json: "teff"`
	LogG  float64 `json: "logG"`
}

type Step struct {
	Command string           `json: "command"`
	Args    []string         `json: "args"`
	Status  CalculationPhase `json: "status, omitempty"`
}

type CalculationStatus struct {
	//Conditions represent the latest available observations of an object's current state:
	// StartTime is equal to the creation time of the ProwJob
	StartTime metav1.Time `json:"startTime,omitempty"`
	// PendingTime is the timestamp for when the job moved from triggered to pending
	PendingTime *metav1.Time `json:"pendingTime,omitempty"`
	// CompletionTime is the timestamp for when the job goes to a final state
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`
}
