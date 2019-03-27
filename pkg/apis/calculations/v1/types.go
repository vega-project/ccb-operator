package v1

import (
	v1 "k8s.io/api/core/v1"
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
	//More info: https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#typical-status-properties
	Conditions []CalculationCondition `json:"conditions,omitempty"`
}

type CalculationCondition struct {
	// Type of calculation condition.
	Type CalculationConditionType `json:"type"`
	// Status of the condition, one of True, False, Unknown.
	Status v1.ConditionStatus `json:"status"`
	// The last time this condition was updated.
	LastUpdateTime string `json:"lastUpdateTime,omitempty"`
	// Last time the condition transitioned from one status to another.
	LastTransitionTime string `json:"lastTransitionTime,omitempty"`
	// The reason for the condition's last transition.
	Reason string `json:"reason,omitempty"`
	// Human-readable message indicating details about last transition
	Message string `json:"message,omitempty"`
}
