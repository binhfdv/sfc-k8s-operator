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

// ServiceFunctionSpec defines the desired state of ServiceFunction.
type ServiceFunctionSpec struct {
	Image        string                      `json:"image"`
	Resources    corev1.ResourceRequirements `json:"resources,omitempty"`
	Ports        []corev1.ContainerPort      `json:"ports,omitempty"`
	HostSF       HostFunction                `json:"hostsf,omitempty"`
	NextSF       Function                    `json:"nextsf,omitempty"`
	NodeSelector map[string]string           `json:"nodeSelector,omitempty"`
}

type Function struct {
	Name  string                 `json:"name"`
	Ports []corev1.ContainerPort `json:"ports,omitempty"`
}

type HostFunction struct {
	Name   string                 `json:"name"`
	Image  string                 `json:"image"`
	Ports  []corev1.ContainerPort `json:"ports,omitempty"`
	Labels map[string]string      `json:"labels,omitempty"`
}

// type Label struct {
// 	Key   string `json:"key"`
// 	Value string `json:"value"`
// }

// ServiceFunctionStatus defines the observed state of ServiceFunction.
type ServiceFunctionStatus struct {
	Ready       bool        `json:"ready"`
	PodName     string      `json:"podName,omitempty"`
	ServiceIP   string      `json:"serviceIP,omitempty"`
	LastUpdated metav1.Time `json:"lastUpdated,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// ServiceFunction is the Schema for the servicefunctions API.
type ServiceFunction struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ServiceFunctionSpec   `json:"spec,omitempty"`
	Status ServiceFunctionStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ServiceFunctionList contains a list of ServiceFunction.
type ServiceFunctionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ServiceFunction `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ServiceFunction{}, &ServiceFunctionList{})
}
