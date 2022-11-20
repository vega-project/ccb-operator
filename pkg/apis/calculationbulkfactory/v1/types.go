package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:shortName=cbf
// +resource:path=calculationbulkfactory

type CalculationBulkFactory struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	WorkerPool string   `json:"worker_pool,omitempty"`
	BulkOutput string   `json:"bulk_output,omitempty"`
	Command    string   `json:"command,omitempty"`
	Args       []string `json:"args,omitempty"`

	Status CalculationBulkFactoryStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +resource:path=calculationbulkfactories

type CalculationBulkFactoryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []CalculationBulkFactory `json:"items"`
}

type CalculationBulkFactoryStatus struct {
	CreatedTime    metav1.Time        `json:"startTime,omitempty"`
	CompletionTime *metav1.Time       `json:"completionTime,omitempty"`
	Conditions     []metav1.Condition `json:"conditions,omitempty"`
	BulkCreated    bool               `json:"bulk_created,omitempty"`
}
