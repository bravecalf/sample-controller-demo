package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Inference is a specification for a Inference resource
type Inference struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InferenceSpec   `json:"spec"`
	Status InferenceStatus `json:"status"`
}

type DeploymentSpec struct {
	Name          string `json:"name"`
	Image         string `json:"image"`
	Replicas      int32  `json:"replicas"`
	ContainerPort int32  `json:"containerPort"`
}

type ServiceSpec struct {
	Name string `json:"name"`
}

type IngressSpec struct {
	Name      string `json:"name"`
	UrlPath   string `json:"urlPath"`
	ClassName string `json:"className"`
}

// InferenceSpec is the spec for a Inference resource
type InferenceSpec struct {
	Deployment DeploymentSpec `json:"deployment"`
	Service    ServiceSpec    `json:"service"`
	Ingress    IngressSpec    `json:"ingress"`
}

// InferenceStatus is the status for a Inference resource
type InferenceStatus struct {
	AvailableReplicas int32 `json:"availableReplicas"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// InferenceList is a list of Inference resources
type InferenceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Inference `json:"items"`
}
