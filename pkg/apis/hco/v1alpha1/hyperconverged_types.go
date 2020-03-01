package v1alpha1

import (
	ownVersion "github.com/kubevirt/hyperconverged-cluster-operator/version"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	conditionsv1 "github.com/openshift/custom-resource-status/conditions/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.
// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file

// HyperConvergedName is the name of the HyperConverged resource that will be reconciled
const HyperConvergedName = "kubevirt-hyperconverged"

// HyperConvergedSpec defines the desired state of HyperConverged
// +k8s:openapi-gen=true
type HyperConvergedSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html

	// BareMetalPlatform indicates whether the infrastructure is baremetal.
	BareMetalPlatform bool `json:"BareMetalPlatform,omitempty"`

	// LocalStorageClassName the name of the local storage class.
	LocalStorageClassName string `json:"LocalStorageClassName,omitempty"`

	// Deprecated: Version is the HCO version
	// +optional
	Version string `json:"version,omitempty"`
}

// HyperConvergedDeployStatus defines the HCO deployment phases
type HyperConvergedDeployStatus string

const (
	HyperConvergedDeployedStatus  HyperConvergedDeployStatus = "Deployed"
	HyperConvergedDeployingStatus HyperConvergedDeployStatus = "Deploying"
	HyperConvergedDeletingStatus  HyperConvergedDeployStatus = "Deleting"
	HyperConvergedDegradedStatus  HyperConvergedDeployStatus = "Degraded"
)

const operatorVersionName = "operator"

// operatorVersion implements openshift operator versioning format, e.g.
//   status:
//     versions:
//     - name: operator
//       version: 1.2.3
type operatorVersion struct {
	Name    string `json:"name,omitempty"`
	Version string `json:"version,omitempty"`
}

// HCO version
var operatorVersions = []operatorVersion{
	{Name: operatorVersionName, Version: ownVersion.Version},
}

// get HCO version in openshift operator format
func GetOperatorVersions() []operatorVersion {
	return operatorVersions
}

// HyperConvergedStatus defines the observed state of HyperConverged
// +k8s:openapi-gen=true
type HyperConvergedStatus struct {
	// Conditions describes the state of the HyperConverged resource.
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +optional
	Conditions []conditionsv1.Condition `json:"conditions,omitempty"  patchStrategy:"merge" patchMergeKey:"type"`

	// RelatedObjects is a list of objects created and maintained by this
	// operator. Object references will be added to this list after they have
	// been created AND found in the cluster.
	// +optional
	RelatedObjects []corev1.ObjectReference `json:"relatedObjects,omitempty"`

	// Deprecated: Phase is the deployment phase of the HCO operator, to be displayed in the UI. Phase is deprecated and
	// should be replaced with `Status` in openshift 4.4
	// +optional
	Phase HyperConvergedDeployStatus `json:"phase,omitempty"`

	// Status is the deployment phase of the HCO operator, to be displayed in the UI
	// +optional
	Status HyperConvergedDeployStatus `json:"status,omitempty"`

	// Versions are set of name/version of the instance
	// +optional
	Versions []operatorVersion `json:"versions,omitempty"`
}

func (hcs *HyperConvergedStatus) SetStatus(status HyperConvergedDeployStatus) {
	hcs.Status = status
	hcs.Phase = status
}

// ConditionReconcileComplete communicates the status of the HyperConverged resource's
// reconcile functionality. Basically, is the Reconcile function running to completion.
const ConditionReconcileComplete conditionsv1.ConditionType = "ReconcileComplete"

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// HyperConverged is the Schema for the hyperconvergeds API
// +k8s:openapi-gen=true
type HyperConverged struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HyperConvergedSpec   `json:"spec,omitempty"`
	Status HyperConvergedStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// HyperConvergedList contains a list of HyperConverged
type HyperConvergedList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HyperConverged `json:"items"`
}

const (
	AppLabel      = "app"
	AppLableValue = "kubevirt-hyperconverged"
)

func init() {
	SchemeBuilder.Register(&HyperConverged{}, &HyperConvergedList{})
}
