/*
Copyright 2024.

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

// PersesDatasourceStatus defines the observed state of PersesDatasource
type PersesDatasourceStatus struct {
	// +operator-sdk:csv:customresourcedefinitions:type=status
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

type DatasourceSpec struct {
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Config Datasource `json:"config,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// +optional
	Client *Client `json:"client,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=perds

// PersesDatasource is the Schema for the PersesDatasources API
type PersesDatasource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DatasourceSpec         `json:"spec,omitempty"`
	Status PersesDatasourceStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// PersesDatasourceList contains a list of PersesDatasource
type PersesDatasourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PersesDatasource `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PersesDatasource{}, &PersesDatasourceList{})
}
