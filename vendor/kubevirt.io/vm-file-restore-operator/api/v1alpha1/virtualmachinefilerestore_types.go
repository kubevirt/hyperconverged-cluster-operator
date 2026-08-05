/*
Copyright 2026.

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

// VirtualMachineFileRestoreSpec defines the desired state of VirtualMachineFileRestore.
type VirtualMachineFileRestoreSpec struct {
	// Target is a reference to the target VirtualMachine to restore files into.
	// +kubebuilder:validation:Required
	Target corev1.TypedLocalObjectReference `json:"target"`

	// Source specifies where to restore files from (PVC, VolumeSnapshot, or remote storage).
	// +kubebuilder:validation:Required
	Source RestoreSource `json:"source"`

	// SourcePath specifies the file or directory path to restore from the backup.
	// If not specified, manual restore mode is enabled (volume is hotplugged but no automatic restore).
	// +optional
	SourcePath string `json:"sourcePath,omitempty"`

	// TargetPath specifies where to restore files in the target VM filesystem.
	// If not specified, files are restored to their original locations.
	// +optional
	TargetPath string `json:"targetPath,omitempty"`

	// SourcePartition specifies the partition number on the backup volume to restore from.
	// +optional
	SourcePartition *int32 `json:"sourcePartition,omitempty"`
}

// RestoreSource defines the source for file restoration.
// Exactly one of PVC, Snapshot, or Remote must be specified.
type RestoreSource struct {
	// PVC specifies a PersistentVolumeClaim to restore from.
	// +optional
	PVC *PVCSource `json:"pvc,omitempty"`

	// Snapshot specifies a VolumeSnapshot to restore from.
	// +optional
	Snapshot *VolumeSnapshotSource `json:"snapshot,omitempty"`

	// Remote specifies a remote storage location (e.g., S3) to restore from.
	// +optional
	Remote *RemoteSource `json:"remote,omitempty"`
}

// PVCSource specifies a PersistentVolumeClaim source.
type PVCSource struct {
	// Name is the name of the PVC.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Namespace is the namespace of the PVC. If empty, defaults to the same namespace as the VirtualMachineFileRestore.
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// VolumeSnapshotSource specifies a VolumeSnapshot source.
type VolumeSnapshotSource struct {
	// Name is the name of the VolumeSnapshot.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Namespace is the namespace of the VolumeSnapshot. If empty, defaults to the same namespace as the VirtualMachineFileRestore.
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// RemoteSource specifies a remote storage location (e.g., S3).
// The guest helper is expected to use rclone or similar tool with pre-configured remotes.
type RemoteSource struct {
	// Name is the name of the rclone remote (as configured in the guest).
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Bucket is the bucket name in the remote storage.
	// +kubebuilder:validation:Required
	Bucket string `json:"bucket"`
}

// VirtualMachineFileRestoreStatus defines the observed state of VirtualMachineFileRestore.
type VirtualMachineFileRestoreStatus struct {
	// Phase represents the current phase of the file restore operation.
	// +optional
	Phase RestorePhase `json:"phase,omitempty"`

	// Conditions represent the latest available observations of the restore's state.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// StartTime is the time when the restore operation started.
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// CompletionTime is the time when the restore operation completed.
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// RestoredFilesCount is the number of files successfully restored.
	// +optional
	RestoredFilesCount int32 `json:"restoredFilesCount,omitempty"`

	// ErrorMessage provides details about any error that occurred during restoration.
	// +optional
	ErrorMessage string `json:"errorMessage,omitempty"`

	// MountPath is where the restore volume is mounted in the guest OS.
	// For Linux: /backup, for Windows: C:\backup
	// +optional
	MountPath string `json:"mountPath,omitempty"`

	// AttachmentRetries tracks the number of times we've waited for volume attachment.
	// Used to implement timeout for stuck attachments.
	// +optional
	AttachmentRetries int32 `json:"attachmentRetries,omitempty"`

	// SSHRetries tracks the number of SSH connection attempts.
	// Used to implement retry logic for transient SSH failures.
	// +optional
	SSHRetries int32 `json:"sshRetries,omitempty"`

	// LastAttachmentCheckTime is the timestamp of the last attachment status check.
	// Used for rate limiting to prevent rapid reconciliation loops.
	// +optional
	LastAttachmentCheckTime *metav1.Time `json:"lastAttachmentCheckTime,omitempty"`

	// LastSSHCheckTime is the timestamp of the last SSH connection attempt.
	// Used for rate limiting to prevent rapid reconciliation loops.
	// +optional
	LastSSHCheckTime *metav1.Time `json:"lastSSHCheckTime,omitempty"`
}

// RestorePhase is a label for the phase of a VirtualMachineFileRestore operation.
// +kubebuilder:validation:Enum=New;Init;Hotplugging;WaitingForAttachment;SSHConnecting;Restoring;VolumeReady;Cleanup;Succeeded;Failed
type RestorePhase string

const (
	// RestorePhaseNew means the restore has been accepted but not yet started.
	RestorePhaseNew RestorePhase = "New"
	// RestorePhaseInit means initialization and validation is in progress.
	RestorePhaseInit RestorePhase = "Init"
	// RestorePhaseHotplugging means the volume is being attached to the VM.
	RestorePhaseHotplugging RestorePhase = "Hotplugging"
	// RestorePhaseWaitingForAttachment means waiting for volume to be ready.
	RestorePhaseWaitingForAttachment RestorePhase = "WaitingForAttachment"
	// RestorePhaseSSHConnecting means establishing SSH connection to guest.
	RestorePhaseSSHConnecting RestorePhase = "SSHConnecting"
	// RestorePhaseRestoring means restore operation is in progress.
	RestorePhaseRestoring RestorePhase = "Restoring"
	// RestorePhaseVolumeReady means volume is mounted for manual restore.
	RestorePhaseVolumeReady RestorePhase = "VolumeReady"
	// RestorePhaseCleanup means removing volume and cleaning up.
	RestorePhaseCleanup RestorePhase = "Cleanup"
	// RestorePhaseSucceeded means the restore completed successfully.
	RestorePhaseSucceeded RestorePhase = "Succeeded"
	// RestorePhaseFailed means the restore failed.
	RestorePhaseFailed RestorePhase = "Failed"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=vmfr;vmfrestore,scope=Namespaced
// +kubebuilder:printcolumn:name="Target",type=string,JSONPath=`.spec.target.name`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Files",type=integer,JSONPath=`.status.restoredFilesCount`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// VirtualMachineFileRestore is the Schema for the virtualmachinefilerestores API.
type VirtualMachineFileRestore struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VirtualMachineFileRestoreSpec   `json:"spec,omitempty"`
	Status VirtualMachineFileRestoreStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// VirtualMachineFileRestoreList contains a list of VirtualMachineFileRestore.
type VirtualMachineFileRestoreList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VirtualMachineFileRestore `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VirtualMachineFileRestore{}, &VirtualMachineFileRestoreList{})
}
