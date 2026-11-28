/*
 * This file is part of the KubeVirt project
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * Copyright The KubeVirt Authors.
 *
 */

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// VirtualMachineImportSpec defines the desired import.
// The spec is immutable after creation. To change import parameters,
// delete the VirtualMachineImport and create a new one.
// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="spec is immutable"
type VirtualMachineImportSpec struct {
	// Source specifies where to import from.
	Source VirtualMachineImportSource `json:"source"`

	// TargetName is the name for the created VirtualMachine or
	// VirtualMachineTemplate. Defaults to the VirtualMachineImport name.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:XValidation:rule="!format.dns1123Subdomain().validate(self).hasValue()",message="must be a valid DNS-1123 subdomain"
	// +optional
	TargetName string `json:"targetName,omitempty"`

	// StorageClassName overrides the default storage class for all
	// DataVolumes created during the import.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:XValidation:rule="!format.dns1123Subdomain().validate(self).hasValue()",message="must be a valid DNS-1123 subdomain"
	// +optional
	StorageClassName string `json:"storageClassName,omitempty"`
}

// VirtualMachineImportSource specifies where to import from.
// +kubebuilder:validation:XValidation:rule="has(self.registry)",message="a registry source is required"
type VirtualMachineImportSource struct {
	// Registry specifies an OCI artifact in a container registry.
	// +optional
	Registry *VirtualMachineImportRegistrySource `json:"registry,omitempty"`
}

// VirtualMachineImportRegistrySource describes an OCI artifact in a container registry.
type VirtualMachineImportRegistrySource struct {
	// URL is the registry URL
	// (e.g. docker://registry.example.com/vms/fedora:v1).
	// +kubebuilder:validation:XValidation:rule="self.startsWith('docker://')",message="url must start with docker://"
	URL string `json:"url"`

	// SecretRef is the name of a Secret containing registry credentials.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:XValidation:rule="!format.dns1123Subdomain().validate(self).hasValue()",message="must be a valid DNS-1123 subdomain"
	// +optional
	SecretRef string `json:"secretRef,omitempty"`

	// CertConfigMap is the name of a ConfigMap containing registry CA certificates.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:XValidation:rule="!format.dns1123Subdomain().validate(self).hasValue()",message="must be a valid DNS-1123 subdomain"
	// +optional
	CertConfigMap string `json:"certConfigMap,omitempty"`

	// Platform selects the architecture variant from a multi-arch artifact.
	// +optional
	Platform *PlatformOptions `json:"platform,omitempty"`
}

// PlatformOptions selects a platform variant from a multi-arch OCI artifact.
type PlatformOptions struct {
	// Architecture selects the platform variant (e.g. amd64, arm64).
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +optional
	Architecture string `json:"architecture,omitempty"`
}

// VirtualMachineImportStatus reports the observed import state.
type VirtualMachineImportStatus struct {
	// Conditions represent the latest observations of the import.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ArtifactInfo contains metadata read from the OCI artifact.
	// +optional
	ArtifactInfo *ArtifactInfo `json:"artifactInfo,omitempty"`

	// DiskImports tracks the status of each disk being imported.
	// +optional
	// +listType=map
	// +listMapKey=name
	DiskImports []DiskImportStatus `json:"diskImports,omitempty"`

	// TargetRef references the created VirtualMachine or VirtualMachineTemplate
	// in the namespace of the VirtualMachineImport.
	// +optional
	TargetRef *corev1.TypedLocalObjectReference `json:"targetRef,omitempty"`

	// ManifestConfigMapHash is the sha256 hash of the data of the manifest ConfigMap
	// the controller created. The import fails if the data no longer matches it.
	// +optional
	ManifestConfigMapHash string `json:"manifestConfigMapHash,omitempty"`

	// FetchRestarts counts how often fetching the artifact metadata started over
	// because the result of a succeeded fetch was lost.
	// +optional
	FetchRestarts int32 `json:"fetchRestarts,omitempty"`
}

// ArtifactInfo contains metadata read from the OCI artifact.
type ArtifactInfo struct {
	// ArtifactType is the OCI artifactType from the manifest.
	ArtifactType string `json:"artifactType"`

	// Architecture is the resolved platform architecture.
	Architecture string `json:"architecture"`

	// Digest is the digest the disks are imported from: the image index
	// digest for a multi-arch artifact, otherwise the manifest digest.
	Digest string `json:"digest"`
}

// DiskImportStatus tracks the import of a single disk.
type DiskImportStatus struct {
	// Name is the disk name, from the io.kubevirt.disk.name layer annotation.
	Name string `json:"name"`

	// DataVolumeName is the name of the created DataVolume.
	DataVolumeName string `json:"dataVolumeName"`

	// Phase mirrors cdiv1.DataVolumePhase values
	// (e.g. "Succeeded", "ImportInProgress", "Failed").
	Phase string `json:"phase"`

	// Progress mirrors the DataVolume progress (e.g. "45.3%").
	// +optional
	Progress string `json:"progress,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=vmimport;vmimports
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Target",type="string",JSONPath=".status.targetRef.name"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Progressing",type="string",JSONPath=".status.conditions[?(@.type=='Progressing')].status"
// +kubebuilder:printcolumn:name="Reason",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].reason"
// +genclient

// VirtualMachineImport imports a VirtualMachine or VirtualMachineTemplate
// from an OCI artifact in a container registry.
type VirtualMachineImport struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec VirtualMachineImportSpec `json:"spec"`

	// +optional
	Status VirtualMachineImportStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// VirtualMachineImportList contains a list of VirtualMachineImport.
type VirtualMachineImportList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VirtualMachineImport `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VirtualMachineImport{}, &VirtualMachineImportList{})
}
