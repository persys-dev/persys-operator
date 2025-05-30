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

// +kubebuilder:resource:path=persysworkloads,scope=Namespaced
// +kubebuilder:object:generate=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="WorkloadType",type="string",JSONPath=".spec.workloadType"
// +kubebuilder:printcolumn:name="State",type="string",JSONPath=".status.state"
// +kubebuilder:printcolumn:name="Replicas",type="integer",JSONPath=".spec.replicas"
// +kubebuilder:printcolumn:name="Current",type="integer",JSONPath=".status.currentReplicas"
// +groupName=prow.persys.io

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PersysWorkloadSpec defines the desired state of PersysWorkload.
type PersysWorkloadSpec struct {
	// WorkloadType specifies the Kubernetes resource type (Deployment, StatefulSet, Job, CronJob)
	WorkloadType string `json:"workloadType"`
	// Containers defines the pod containers
	Containers []Container `json:"containers"`
	// NodeSelector for node affinity
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	// Affinity rules for advanced scheduling
	Affinity *corev1.Affinity `json:"affinity,omitempty"`
	// Tolerations for taints
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
	// Replicas for scalable workloads
	Replicas int32 `json:"replicas,omitempty"`
	// RestartPolicy for pod (Always, OnFailure, Never)
	RestartPolicy corev1.RestartPolicy `json:"restartPolicy,omitempty"`
	// CronSchedule for CronJob (e.g., "0 0 * * *")
	CronSchedule string `json:"cronSchedule,omitempty"`
}

// Container represents a container in the workload
type Container struct {
	Name         string                      `json:"name"`
	Image        string                      `json:"image"`
	Command      []string                    `json:"command,omitempty"`
	Env          []corev1.EnvVar             `json:"env,omitempty"`
	Ports        []corev1.ContainerPort      `json:"ports,omitempty"`
	VolumeMounts []corev1.VolumeMount        `json:"volumeMounts,omitempty"`
	Resources    corev1.ResourceRequirements `json:"resources,omitempty"`
}

// PersysWorkloadStatus defines the observed state of PersysWorkload.
type PersysWorkloadStatus struct {
	// Current state (Running, Pending, Failed, Completed)
	State string `json:"state"`
	// Current replicas
	CurrentReplicas int32 `json:"currentReplicas"`
	// Pod names
	PodNames []string `json:"podNames,omitempty"`
	// Last update time
	LastUpdateTime *metav1.Time `json:"lastUpdateTime,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=persysworkloads,scope=Namespaced
// +kubebuilder:printcolumn:name="WorkloadType",type="string",JSONPath=".spec.workloadType"
// +kubebuilder:printcolumn:name="State",type="string",JSONPath=".status.state"
// +kubebuilder:printcolumn:name="Replicas",type="integer",JSONPath=".spec.replicas"
// +kubebuilder:printcolumn:name="Current",type="integer",JSONPath=".status.currentReplicas"

// PersysWorkload is the Schema for the persysworkloads API.
type PersysWorkload struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PersysWorkloadSpec   `json:"spec,omitempty"`
	Status PersysWorkloadStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PersysWorkloadList contains a list of PersysWorkload.
type PersysWorkloadList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PersysWorkload `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PersysWorkload{}, &PersysWorkloadList{})
}
