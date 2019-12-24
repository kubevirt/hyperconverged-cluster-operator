package v1alpha1

import (
	conditionsv1 "github.com/openshift/custom-resource-status/conditions/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cmdv1 "kubevirt.io/kubevirt/pkg/handler-launcher-com/cmd/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.
// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file

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

	// KubevirtConfigurations is the configurations to be passed on to virt-configMap.
	KubevirtConfigurations KubevirtConfigurations `json:"KubevirtConfigurations,omitempty"`
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

func init() {
	SchemeBuilder.Register(&HyperConverged{}, &HyperConvergedList{})
}

type KubevirtConfigurations struct {
	ResourceVersion                   string            `json:"ResourceVersion,omitempty"`
	UseEmulation                      bool              `json:"UseEmulation,omitempty"`
	MigrationConfig                   *MigrationConfig  `json:"MigrationConfig,omitempty"`
	ImagePullPolicy                   corev1.PullPolicy `json:"ImagePullPolicy,omitempty"`
	MachineType                       string            `json:"MachineType,omitempty"`
	CPUModel                          string            `json:"CPUModel,omitempty"`
	CPURequest                        resource.Quantity `json:"CPURequest,omitempty"`
	MemoryOvercommit                  int               `json:"MemoryOvercommit,omitempty"`
	EmulatedMachines                  []string          `json:"EmulatedMachines,omitempty"`
	FeatureGates                      *FeatureGates     `json:"FeatureGates,omitempty"`
	LessPVCSpaceToleration            int               `json:"LessPVCSpaceToleration,omitempty"`
	NodeSelectors                     map[string]string `json:"NodeSelectors,omitempty"`
	NetworkInterface                  string            `json:"NetworkInterface,omitempty"`
	PermitSlirpInterface              bool              `json:"PermitSlirpInterface,omitempty"`
	PermitBridgeInterfaceOnPodNetwork bool              `json:"PermitBridgeInterfaceOnPodNetwork,omitempty"`
	SmbiosConfig                      *cmdv1.SMBios     `json:"SmbiosConfig,omitempty"`
}

type MigrationConfig struct {
	ParallelOutboundMigrationsPerNode *uint32            `json:"parallelOutboundMigrationsPerNode,omitempty"`
	ParallelMigrationsPerCluster      *uint32            `json:"parallelMigrationsPerCluster,omitempty"`
	BandwidthPerMigration             *resource.Quantity `json:"bandwidthPerMigration,omitempty"`
	NodeDrainTaintKey                 *string            `json:"nodeDrainTaintKey,omitempty"`
	ProgressTimeout                   *int64             `json:"progressTimeout,omitempty"`
	CompletionTimeoutPerGiB           *int64             `json:"completionTimeoutPerGiB,omitempty"`
	UnsafeMigrationOverride           bool               `json:"unsafeMigrationOverride"`
	AllowAutoConverge                 bool               `json:"allowAutoConverge"`
}

// FeatureGates represents feature-gates to be enabled or disabled in kubevirt configMap
type FeatureGates struct {
	DataVolumes      string `json:"DataVolumes,omitempty"`
	SRIOV            string `json:"SRIOV,omitempty"`
	LiveMigration    string `json:"LiveMigration,omitempty"`
	CPUManager       string `json:"CPUManager,omitempty"`
	CPUNodeDiscovery string `json:"CPUNodeDiscovery,omitempty"`
	Sidecar          string `json:"Sidecar,omitempty"`
}
