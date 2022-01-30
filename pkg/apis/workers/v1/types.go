package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	bulkv1 "github.com/vega-project/ccb-operator/pkg/apis/calculationbulk/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +resource:path=workerpool

type WorkerPool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WorkerPoolSpec   `json:"spec,omitempty"`
	Status WorkerPoolStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type WorkerPoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []WorkerPool `json:"items"`
}

type WorkerPoolSpec struct {
	CalculationBulks map[string]CalculationBulk `json:"calculationBulks,omitempty"`
	Workers          map[string]Worker          `json:"workers,omitempty"`
}

type CalculationBulk struct {
	Name           string                      `json:"name,omitempty"`
	RegisteredTime *metav1.Time                `json:"registeredTime,omitempty"`
	State          bulkv1.CalculationBulkState `json:"state,omitempty"`
}

type Worker struct {
	Name                  string       `json:"name"`
	RegisteredTime        *metav1.Time `json:"registeredTime"`
	LastUpdateTime        *metav1.Time `json:"lastUpdateTime,omitempty"`
	CalculationsProcessed int64        `json:"calculationsProcessed"`
	State                 WorkerState  `json:"status"`
}

type WorkerState string

const (
	WorkerAvailableState  WorkerState = "Available"
	WorkerProcessingState WorkerState = "Processing"
	WorkerUnknownState    WorkerState = "Unknown"
)

type WorkerPoolStatus struct {
	CreationTime   *metav1.Time `json:"creationTime,omitempty"`
	PendingTime    *metav1.Time `json:"pendingTime,omitempty"`
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`
}
