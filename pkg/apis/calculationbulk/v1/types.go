package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:shortName=calcbulk
// +resource:path=calculationbulk

type CalculationBulk struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Calculations map[string]Calculation `json:"calculations"`
}

type Calculation struct {
	LogG float64 `json:"log_g,omitempty"`
	Teff float64 `json:"teff,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +resource:path=calculationbulks

type CalculationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []CalculationBulk `json:"items"`
}
