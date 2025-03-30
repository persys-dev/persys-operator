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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// PersysOperatorSpec defines the desired state of PersysOperator.
type PersysOperatorSpec struct {
	Image    string `json:"image"`
	CPU      string `json:"cpu"`
	Memory   string `json:"memory"`
	Replicas int32  `json:"replicas"`
}

// PersysOperatorStatus defines the observed state of PersysOperator.
type PersysOperatorStatus struct {
	CurrentReplicas int32    `json:"currentReplicas"`
	PodNames        []string `json:"podNames,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Replicas", type="integer",JSONPath=".spec.replicas"
// +kubebuilder:printcolumn:name="Current",type="integer",JSONPath=".status.currentReplicas"

// PersysOperator is the Schema for the persysoperators API.
type PersysOperator struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PersysOperatorSpec   `json:"spec,omitempty"`
	Status PersysOperatorStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PersysOperatorList contains a list of PersysOperator.
type PersysOperatorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PersysOperator `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PersysOperator{}, &PersysOperatorList{})
}
