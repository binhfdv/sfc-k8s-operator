/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ServiceFunctionChainSpec defines the desired state of ServiceFunctionChain.
type ServiceFunctionChainSpec struct {
	Functions        []string       `json:"functions"` // list of ServiceFunction names
	IngressInterface string         `json:"ingressInterface,omitempty"`
	EgressInterface  string         `json:"egressInterface,omitempty"`
	Forwarder        TrafficForward `json:"forwarder,omitempty"`
}

type TrafficForward struct {
	Image        string                      `json:"image"`
	Resources    corev1.ResourceRequirements `json:"resources,omitempty"`
	Ports        []corev1.ContainerPort      `json:"ports,omitempty"`
	NodeSelector map[string]string           `json:"nodeSelector,omitempty"`
}

// ServiceFunctionChainStatus defines the observed state of ServiceFunctionChain.
type ServiceFunctionChainStatus struct {
	Ready        bool        `json:"ready"`
	PodName      string      `json:"podName,omitempty"`
	ServiceIP    string      `json:"serviceIP,omitempty"`
	DeployedPods []string    `json:"deployedPods,omitempty"`
	LastUpdated  metav1.Time `json:"lastUpdated,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// ServiceFunctionChain is the Schema for the servicefunctionchains API.
type ServiceFunctionChain struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ServiceFunctionChainSpec   `json:"spec,omitempty"`
	Status ServiceFunctionChainStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ServiceFunctionChainList contains a list of ServiceFunctionChain.
type ServiceFunctionChainList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ServiceFunctionChain `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ServiceFunctionChain{}, &ServiceFunctionChainList{})
}
