package v1beta1

import (
	conditionsv1 "github.com/openshift/custom-resource-status/conditions/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	sdkapi "kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk/api"
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

	// featureGates is a map of feature gate flags. Setting a flag to `true` will enable
	// the feature. Setting `false` or removing the feature gate, disables the feature.
	// +optional
	FeatureGates HyperConvergedFeatureGates `json:"featureGates,omitempty"`

	// Live migration limits and timeouts are applied so that migration processes do not
	// overwhelm the cluster.
	// +optional
	LiveMigrationConfig LiveMigrationConfigurations `json:"liveMigrationConfig,omitempty"`

	// PermittedHostDevices holds inforamtion about devices allowed for passthrough
	// +optional
	PermittedHostDevices *PermittedHostDevices `json:"permittedHostDevices,omitempty"`

	// operator version
	Version string `json:"version,omitempty"`
}

// HyperConvergedConfig defines a set of configurations to pass to components
type HyperConvergedConfig struct {
	// NodePlacement describes node scheduling configuration.
	// +optional
	NodePlacement *sdkapi.NodePlacement `json:"nodePlacement,omitempty"`
}

// LiveMigrationConfigurations - Live migration limits and timeouts are applied so that migration processes do not
// overwhelm the cluster.
// +optional
// +k8s:openapi-gen=true
type LiveMigrationConfigurations struct {
	// Number of migrations running in parallel in the cluster.
	// +optional
	// +kubebuilder:default=5
	ParallelMigrationsPerCluster *uint32 `json:"parallelMigrationsPerCluster,omitempty"`

	// Maximum number of outbound migrations per node.
	// +optional
	// +kubebuilder:default=2
	ParallelOutboundMigrationsPerNode *uint32 `json:"parallelOutboundMigrationsPerNode,omitempty"`

	// Bandwidth limit of each migration, in MiB/s.
	// +optional
	// +kubebuilder:default="64Mi"
	// +kubebuilder:validation:Pattern=^(\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))(([KMGTPE]i)|[numkMGTPE]|([eE](\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))))?$
	BandwidthPerMigration *string `json:"bandwidthPerMigration,omitempty"`

	// The migration will be canceled if it has not completed in this time, in seconds per GiB
	// of memory. For example, a virtual machine instance with 6GiB memory will timeout if it has not completed
	// migration in 4800 seconds. If the Migration Method is BlockMigration, the size of the migrating disks is included
	// in the calculation.
	// +kubebuilder:default=800
	CompletionTimeoutPerGiB *int64 `json:"completionTimeoutPerGiB,omitempty"`

	// The migration will be canceled if memory copy fails to make progress in this time, in seconds.
	// +kubebuilder:default=150
	ProgressTimeout *int64 `json:"progressTimeout,omitempty"`
}

type FeatureGate *bool

// HyperConvergedFeatureGates is a set of optional feature gates to enable or disable new features that are not enabled
// by default yet.
// +optional
// +k8s:openapi-gen=true
type HyperConvergedFeatureGates struct {
	// Allow migrating a virtual machine with CPU host-passthrough mode. This should be
	// enabled only when the Cluster is homogeneous from CPU HW perspective doc here
	// +optional
	// +kubebuilder:default=false
	WithHostPassthroughCPU FeatureGate `json:"withHostPassthroughCPU,omitempty"`
}

func (fgs *HyperConvergedFeatureGates) IsWithHostPassthroughCPUEnabled() bool {
	return (fgs != nil) && (fgs.WithHostPassthroughCPU != nil) && (*fgs.WithHostPassthroughCPU)
}

// PermittedHostDevices holds inforamtion about devices allowed for passthrough
// +k8s:openapi-gen=true
type PermittedHostDevices struct {
	// +listType=atomic
	PciHostDevices []PciHostDevice `json:"pciHostDevices,omitempty"`
	// +listType=atomic
	MediatedDevices []MediatedHostDevice `json:"mediatedDevices,omitempty"`
}

// PciHostDevice represents a host PCI device allowed for passthrough
// +k8s:openapi-gen=true
type PciHostDevice struct {
	PCIVendorSelector        string `json:"pciVendorSelector"`
	ResourceName             string `json:"resourceName"`
	ExternalResourceProvider bool   `json:"externalResourceProvider,omitempty"`
}

// MediatedHostDevice represents a host mediated device allowed for passthrough
// +k8s:openapi-gen=true
type MediatedHostDevice struct {
	MDEVNameSelector         string `json:"mdevNameSelector"`
	ResourceName             string `json:"resourceName"`
	ExternalResourceProvider bool   `json:"externalResourceProvider,omitempty"`
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
