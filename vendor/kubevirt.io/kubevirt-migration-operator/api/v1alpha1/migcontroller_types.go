/*
Copyright 2025 The KubeVirt Authors.

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

	sdkapi "kubevirt.io/controller-lifecycle-operator-sdk/api"
)

// MigControllerSpec defines the desired state of MigController.
type MigControllerSpec struct {
	// PriorityClass of the control plane
	PriorityClass *MigControllerPriorityClass `json:"priorityClass,omitempty"`
	// +kubebuilder:validation:Enum=Always;IfNotPresent;Never
	// PullPolicy describes a policy for if/when to pull a container image
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty" valid:"required"`
	// Rules on which nodes infrastructure pods will be scheduled
	Infra sdkapi.NodePlacement `json:"infra,omitempty"`
}

// MigControllerStatus defines the observed state of MigController.
type MigControllerStatus struct {
	sdkapi.Status `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced

// MigController is the Schema for the migcontrollers API.
type MigController struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MigControllerSpec   `json:"spec,omitempty"`
	Status MigControllerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MigControllerList contains a list of MigController.
type MigControllerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MigController `json:"items"`
}

// MigControllerPriorityClass defines the priority class of the control plane.
type MigControllerPriorityClass string

func init() {
	SchemeBuilder.Register(&MigController{}, &MigControllerList{})
}
