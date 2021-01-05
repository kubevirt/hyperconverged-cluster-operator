package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	sdkapi "kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk/api"

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
	BareMetalPlatform bool `json:"bareMetalPlatform,omitempty"`

	// LocalStorageClassName the name of the local storage class.
	LocalStorageClassName string `json:"localStorageClassName,omitempty"`

	// infra HyperConvergedConfig influences the pod configuration (currently only placement)
	// for all the infra components needed on the virtualization enabled cluster
	// but not necessarely directly on each node running VMs/VMIs.
	// +optional
	Infra HyperConvergedConfig `json:"infra,omitempty"`

	// workloads HyperConvergedConfig influences the pod configuration (currently only placement) of components
	// which need to be running on a node where virtualization workloads should be able to run.
	// Changes to Workloads HyperConvergedConfig can be applied only without existing workload.
	// +optional
	Workloads HyperConvergedConfig `json:"workloads,omitempty"`

	// featureGates HyperConvergedFeatureGates contain a list of feature enabler flags. Setting a flag to `true` will enable
	// the feature. Setting `false` or removing the feature gate, disables the feature.
	// optional+
	FeatureGates HyperConvergedFeatureGates `json:"featureGates,omitempty"`

	// operator version
	Version string `json:"version,omitempty"`
}

// HyperConvergedConfig defines a set of configurations to pass to components
type HyperConvergedConfig struct {
	// NodePlacement describes node scheduling configuration.
	// +optional
	NodePlacement *sdkapi.NodePlacement `json:"nodePlacement,omitempty"`
}

type HyperConvergedFeatureGates map[string]bool

func (fgs HyperConvergedFeatureGates) IsEnabled(fgName string) bool {
	fg, found := fgs[fgName]
	return fg && found
}

// get list of feature gates from a specific operand list
func (fgs HyperConvergedFeatureGates) GetFeatureGateList(candidates []string) []string {
	res := make([]string, 0, len(fgs))
	for _, fg := range candidates {
		if fgs.IsEnabled(fg) {
			res = append(res, fg)
		}
	}

	return res
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

	// Versions is a list of HCO component versions, as name/version pairs. The version with a name of "operator"
	// is the HCO version itself, as described here:
	// https://github.com/openshift/cluster-version-operator/blob/master/docs/dev/clusteroperator.md#version
	// +optional
	Versions Versions `json:"versions,omitempty"`
}

func (hcs *HyperConvergedStatus) UpdateVersion(name, version string) {
	if hcs.Versions == nil {
		hcs.Versions = Versions{}
	}
	hcs.Versions.updateVersion(name, version)
}

func (hcs *HyperConvergedStatus) GetVersion(name string) (string, bool) {
	return hcs.Versions.getVersion(name)
}

type Version struct {
	Name    string `json:"name,omitempty"`
	Version string `json:"version,omitempty"`
}

func newVersion(name, version string) Version {
	return Version{Name: name, Version: version}
}

type Versions []Version

func (vs *Versions) updateVersion(name, version string) {
	for i, v := range *vs {
		if v.Name == name {
			(*vs)[i].Version = version
			return
		}
	}
	*vs = append(*vs, newVersion(name, version))
}

func (vs *Versions) getVersion(name string) (string, bool) {
	for _, v := range *vs {
		if v.Name == name {
			return v.Version, true
		}
	}
	return "", false
}

const (

	// ConditionReconcileComplete communicates the status of the HyperConverged resource's
	// reconcile functionality. Basically, is the Reconcile function running to completion.
	ConditionReconcileComplete conditionsv1.ConditionType = "ReconcileComplete"

	// ConditionTaintedConfiguration indicates that a hidden/debug configuration
	// has been applied to the HyperConverged resource via a specialized annotation.
	// This condition is exposed only when its value is True, and is otherwise hidden.
	ConditionTaintedConfiguration conditionsv1.ConditionType = "TaintedConfiguration"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// HyperConverged is the Schema for the hyperconvergeds API
// +k8s:openapi-gen=true
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:resource:scope=Namespaced,categories={all},shortName={hco,hcos}
// +kubebuilder:subresource:status
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

func init() {
	SchemeBuilder.Register(&HyperConverged{}, &HyperConvergedList{})
}
