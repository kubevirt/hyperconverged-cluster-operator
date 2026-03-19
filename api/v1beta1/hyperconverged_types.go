package v1beta1

import (
	openshiftconfigv1 "github.com/openshift/api/config/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "kubevirt.io/api/core/v1"
	aaqv1alpha1 "kubevirt.io/application-aware-quota/staging/src/kubevirt.io/application-aware-quota-api/pkg/apis/core/v1alpha1"
	cdiv1beta1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	sdkapi "kubevirt.io/controller-lifecycle-operator-sdk/api"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.
// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file

// HyperConvergedName is the name of the HyperConverged resource that will be reconciled
const HyperConvergedName = "kubevirt-hyperconverged"

type HyperConvergedTuningPolicy string

// HyperConvergedAnnotationTuningPolicy defines a static configuration of the kubevirt query per seconds (qps) and burst values
// through annotation values.
const (
	HyperConvergedAnnotationTuningPolicy HyperConvergedTuningPolicy = "annotation"
	// Deprecated: The highBurst profile is deprecated as of v1.16.0 ahead of removal in a future release
	HyperConvergedHighBurstProfile HyperConvergedTuningPolicy = "highBurst"
)

// HyperConvergedSpec defines the desired state of HyperConverged
// +k8s:openapi-gen=true
type HyperConvergedSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html

	// Deprecated: LocalStorageClassName the name of the local storage class.
	// +k8s:conversion-gen=false
	LocalStorageClassName string `json:"localStorageClassName,omitempty"`

	// TuningPolicy allows to configure the mode in which the RateLimits of kubevirt are set.
	// If TuningPolicy is not present the default kubevirt values are used.
	// It can be set to `annotation` for fine-tuning the kubevirt queryPerSeconds (qps) and burst values.
	// Qps and burst values are taken from the annotation hco.kubevirt.io/tuningPolicy
	// +kubebuilder:validation:Enum=annotation;highBurst
	// +optional
	TuningPolicy HyperConvergedTuningPolicy `json:"tuningPolicy,omitempty"`

	// infra HyperConvergedConfig influences the pod configuration (currently only placement)
	// for all the infra components needed on the virtualization enabled cluster
	// but not necessarily directly on each node running VMs/VMIs.
	// +optional
	// +k8s:conversion-gen=false
	Infra HyperConvergedConfig `json:"infra,omitempty"`

	// workloads HyperConvergedConfig influences the pod configuration (currently only placement) of components
	// which need to be running on a node where virtualization workloads should be able to run.
	// Changes to Workloads HyperConvergedConfig can be applied only without existing workload.
	// +optional
	// +k8s:conversion-gen=false
	Workloads HyperConvergedConfig `json:"workloads,omitempty"`

	// featureGates is a map of feature gate flags. Setting a flag to `true` will enable
	// the feature. Setting `false` or removing the feature gate, disables the feature.
	// +kubebuilder:default={"downwardMetrics": false, "deployKubeSecondaryDNS": false, "disableMDevConfiguration": false, "persistentReservation": false, "enableMultiArchBootImageImport": false, "decentralizedLiveMigration": true, "declarativeHotplugVolumes": false, "videoConfig": true, "objectGraph": false, "incrementalBackup": false, "containerPathVolumes": false}
	// +optional
	// +k8s:conversion-gen=false
	FeatureGates HyperConvergedFeatureGates `json:"featureGates,omitempty"`

	// Live migration limits and timeouts are applied so that migration processes do not
	// overwhelm the cluster.
	// +kubebuilder:default={"completionTimeoutPerGiB": 150, "parallelMigrationsPerCluster": 5, "parallelOutboundMigrationsPerNode": 2, "progressTimeout": 150, "allowAutoConverge": false, "allowPostCopy": false}
	// +optional
	// +k8s:conversion-gen=false
	LiveMigrationConfig hcov1.LiveMigrationConfigurations `json:"liveMigrationConfig,omitempty"`

	// PermittedHostDevices holds information about devices allowed for passthrough
	// +optional
	// +k8s:conversion-gen=false
	PermittedHostDevices *hcov1.PermittedHostDevices `json:"permittedHostDevices,omitempty"`

	// MediatedDevicesConfiguration holds information about MDEV types to be defined on nodes, if available
	// +optional
	// +k8s:conversion-gen=false
	MediatedDevicesConfiguration *MediatedDevicesConfiguration `json:"mediatedDevicesConfiguration,omitempty"`

	// certConfig holds the rotation policy for internal, self-signed certificates
	// +kubebuilder:default={"ca": {"duration": "48h0m0s", "renewBefore": "24h0m0s"}, "server": {"duration": "24h0m0s", "renewBefore": "12h0m0s"}}
	// +optional
	// +k8s:conversion-gen=false
	CertConfig hcov1.HyperConvergedCertConfig `json:"certConfig,omitempty"`

	// ResourceRequirements describes the resource requirements for the operand workloads.
	// +kubebuilder:default={"vmiCPUAllocationRatio": 10}
	// +kubebuilder:validation:XValidation:rule="!has(self.vmiCPUAllocationRatio) || self.vmiCPUAllocationRatio > 0",message="vmiCPUAllocationRatio must be greater than 0"
	// +optional
	// +k8s:conversion-gen=false
	ResourceRequirements *OperandResourceRequirements `json:"resourceRequirements,omitempty"`

	// Override the storage class used for scratch space during transfer operations. The scratch space storage class
	// is determined in the following order:
	// value of scratchSpaceStorageClass, if that doesn't exist, use the default storage class, if there is no default
	// storage class, use the storage class of the DataVolume, if no storage class specified, use no storage class for
	// scratch space
	// +optional
	// +k8s:conversion-gen=false
	ScratchSpaceStorageClass *string `json:"scratchSpaceStorageClass,omitempty"`

	// VDDK Init Image eventually used to import VMs from external providers
	//
	// Deprecated: please use the Migration Toolkit for Virtualization
	// +optional
	// +k8s:conversion-gen=false
	VddkInitImage *string `json:"vddkInitImage,omitempty"`

	// DefaultCPUModel defines a cluster default for CPU model: default CPU model is set when VMI doesn't have any CPU model.
	// When VMI has CPU model set, then VMI's CPU model is preferred.
	// When default CPU model is not set and VMI's CPU model is not set too, host-model will be set.
	// Default CPU model can be changed when kubevirt is running.
	// +optional
	// +k8s:conversion-gen=false
	DefaultCPUModel *string `json:"defaultCPUModel,omitempty"`

	// DefaultRuntimeClass defines a cluster default for the RuntimeClass to be used for VMIs pods if not set there.
	// Default RuntimeClass can be changed when kubevirt is running, existing VMIs are not impacted till
	// the next restart/live-migration when they are eventually going to consume the new default RuntimeClass.
	// +optional
	// +k8s:conversion-gen=false
	DefaultRuntimeClass *string `json:"defaultRuntimeClass,omitempty"`

	// ObsoleteCPUs allows avoiding scheduling of VMs for obsolete CPU models
	// +optional
	// +k8s:conversion-gen=false
	ObsoleteCPUs *HyperConvergedObsoleteCPUs `json:"obsoleteCPUs,omitempty"`

	// CommonTemplatesNamespace defines namespace in which common templates will
	// be deployed. It overrides the default openshift namespace.
	// +optional
	// +k8s:conversion-gen=false
	CommonTemplatesNamespace *string `json:"commonTemplatesNamespace,omitempty"`

	// StorageImport contains configuration for importing containerized data
	// +optional
	// +k8s:conversion-gen=false
	StorageImport *hcov1.StorageImportConfig `json:"storageImport,omitempty"`

	// WorkloadUpdateStrategy defines at the cluster level how to handle automated workload updates
	// +kubebuilder:default={"workloadUpdateMethods": {"LiveMigrate"}, "batchEvictionSize": 10, "batchEvictionInterval": "1m0s"}
	// +k8s:conversion-gen=false
	WorkloadUpdateStrategy hcov1.HyperConvergedWorkloadUpdateStrategy `json:"workloadUpdateStrategy,omitempty"`

	// DataImportCronTemplates holds list of data import cron templates (golden images)
	// +optional
	// +listType=atomic
	// +k8s:conversion-gen=false
	DataImportCronTemplates []hcov1.DataImportCronTemplate `json:"dataImportCronTemplates,omitempty"`

	// FilesystemOverhead describes the space reserved for overhead when using Filesystem volumes.
	// A value is between 0 and 1, if not defined it is 0.055 (5.5 percent overhead)
	// +optional
	// +k8s:conversion-gen=false
	FilesystemOverhead *cdiv1beta1.FilesystemOverhead `json:"filesystemOverhead,omitempty"`

	// UninstallStrategy defines how to proceed on uninstall when workloads (VirtualMachines, DataVolumes) still exist.
	// BlockUninstallIfWorkloadsExist will prevent the CR from being removed when workloads still exist.
	// BlockUninstallIfWorkloadsExist is the safest choice to protect your workloads from accidental data loss, so it's strongly advised.
	// RemoveWorkloads will cause all the workloads to be cascading deleted on uninstallation.
	// WARNING: please notice that RemoveWorkloads will cause your workloads to be deleted as soon as this CR will be, even accidentally, deleted.
	// Please correctly consider the implications of this option before setting it.
	// BlockUninstallIfWorkloadsExist is the default behaviour.
	// +kubebuilder:default=BlockUninstallIfWorkloadsExist
	// +default="BlockUninstallIfWorkloadsExist"
	// +kubebuilder:validation:Enum=RemoveWorkloads;BlockUninstallIfWorkloadsExist
	// +optional
	// +k8s:conversion-gen=false
	UninstallStrategy hcov1.HyperConvergedUninstallStrategy `json:"uninstallStrategy,omitempty"`

	// LogVerbosityConfig configures the verbosity level of Kubevirt's different components. The higher
	// the value - the higher the log verbosity.
	// +optional
	// +k8s:conversion-gen=false
	LogVerbosityConfig *hcov1.LogVerbosityConfiguration `json:"logVerbosityConfig,omitempty"`

	// TLSSecurityProfile specifies the settings for TLS connections to be propagated to all kubevirt-hyperconverged components.
	// If unset, the hyperconverged cluster operator will consume the value set on the APIServer CR on OCP/OKD or Intermediate if on vanilla k8s.
	// Note that only Old, Intermediate and Custom profiles are currently supported, and the maximum available
	// MinTLSVersions is VersionTLS12.
	// +optional
	// +k8s:conversion-gen=false
	TLSSecurityProfile *openshiftconfigv1.TLSSecurityProfile `json:"tlsSecurityProfile,omitempty"`

	// TektonPipelinesNamespace defines namespace in which example pipelines will be deployed.
	// If unset, then the default value is the operator namespace.
	// +optional
	// +kubebuilder:deprecatedversion:warning="tektonPipelinesNamespace field is ignored"
	// Deprecated: This field is ignored.
	// +k8s:conversion-gen=false
	TektonPipelinesNamespace *string `json:"tektonPipelinesNamespace,omitempty"`

	// TektonTasksNamespace defines namespace in which tekton tasks will be deployed.
	// If unset, then the default value is the operator namespace.
	// +optional
	// +kubebuilder:deprecatedversion:warning="tektonTasksNamespace field is ignored"
	// Deprecated: This field is ignored.
	// +k8s:conversion-gen=false
	TektonTasksNamespace *string `json:"tektonTasksNamespace,omitempty"`

	// KubeSecondaryDNSNameServerIP defines name server IP used by KubeSecondaryDNS
	// +optional
	// +k8s:conversion-gen=false
	KubeSecondaryDNSNameServerIP *string `json:"kubeSecondaryDNSNameServerIP,omitempty"`

	// KubeMacPoolConfiguration holds kubemacpool MAC address range configuration.
	// +optional
	// +k8s:conversion-gen=false
	KubeMacPoolConfiguration *hcov1.KubeMacPoolConfig `json:"kubeMacPoolConfiguration,omitempty"`

	// EvictionStrategy defines at the cluster level if the VirtualMachineInstance should be
	// migrated instead of shut-off in case of a node drain. If the VirtualMachineInstance specific
	// field is set it overrides the cluster level one.
	// Allowed values:
	// - `None` no eviction strategy at cluster level.
	// - `LiveMigrate` migrate the VM on eviction; a not live migratable VM with no specific strategy will block the drain of the node util manually evicted.
	// - `LiveMigrateIfPossible` migrate the VM on eviction if live migration is possible, otherwise directly evict.
	// - `External` block the drain, track eviction and notify an external controller.
	// Defaults to LiveMigrate with multiple worker nodes, None on single worker clusters.
	// +kubebuilder:validation:Enum=None;LiveMigrate;LiveMigrateIfPossible;External
	// +optional
	// +k8s:conversion-gen=false
	EvictionStrategy *v1.EvictionStrategy `json:"evictionStrategy,omitempty"`

	// VMStateStorageClass is the name of the storage class to use for the PVCs created to preserve VM state, like TPM.
	// +optional
	// +k8s:conversion-gen=false
	VMStateStorageClass *string `json:"vmStateStorageClass,omitempty"`

	// VirtualMachineOptions holds the cluster level information regarding the virtual machine.
	// +kubebuilder:default={"disableFreePageReporting": false, "disableSerialConsoleLog": false}
	// +default={"disableFreePageReporting": false, "disableSerialConsoleLog": false}
	// +optional
	// +k8s:conversion-gen=false
	VirtualMachineOptions *VirtualMachineOptions `json:"virtualMachineOptions,omitempty"`

	// CommonBootImageNamespace override the default namespace of the common boot images, in order to hide them.
	//
	// If not set, HCO won't set any namespace, letting SSP to use the default. If set, use the namespace to create the
	// DataImportCronTemplates and the common image streams, with this namespace. This field is not set by default.
	//
	// +optional
	// +k8s:conversion-gen=false
	CommonBootImageNamespace *string `json:"commonBootImageNamespace,omitempty"`

	// KSMConfiguration holds the information regarding
	// the enabling the KSM in the nodes (if available).
	// +optional
	// +k8s:conversion-gen=false
	KSMConfiguration *v1.KSMConfiguration `json:"ksmConfiguration,omitempty"`

	// NetworkBinding defines the network binding plugins.
	// Those bindings can be used when defining virtual machine interfaces.
	// +optional
	// +k8s:conversion-gen=false
	NetworkBinding map[string]v1.InterfaceBindingPlugin `json:"networkBinding,omitempty"`

	// ApplicationAwareConfig set the AAQ configurations
	// +optional
	// +k8s:conversion-gen=false
	ApplicationAwareConfig *ApplicationAwareConfigurations `json:"applicationAwareConfig,omitempty"`

	// HigherWorkloadDensity holds configuration aimed to increase virtual machine density
	// +kubebuilder:default={"memoryOvercommitPercentage": 100}
	// +default={"memoryOvercommitPercentage": 100}
	// +optional
	// +k8s:conversion-gen=false
	HigherWorkloadDensity *hcov1.HigherWorkloadDensityConfiguration `json:"higherWorkloadDensity,omitempty"`

	// Opt-in to automatic delivery/updates of the common data import cron templates.
	// There are two sources for the data import cron templates: hard coded list of common templates, and custom (user
	// defined) templates that can be added to the dataImportCronTemplates field. This field only controls the common
	// templates. It is possible to use custom templates by adding them to the dataImportCronTemplates field.
	// +optional
	// +kubebuilder:default=true
	// +default=true
	// +k8s:conversion-gen=false
	EnableCommonBootImageImport *bool `json:"enableCommonBootImageImport,omitempty"`

	// InstancetypeConfig holds the configuration of instance type related functionality within KubeVirt.
	// +optional
	// +k8s:conversion-gen=false
	InstancetypeConfig *v1.InstancetypeConfiguration `json:"instancetypeConfig,omitempty"`

	// CommonInstancetypesDeployment holds the configuration of common-instancetypes deployment within KubeVirt.
	// +optional
	// +k8s:conversion-gen=false
	CommonInstancetypesDeployment *v1.CommonInstancetypesDeployment `json:"CommonInstancetypesDeployment,omitempty"`

	// deploy VM console proxy resources in SSP operator
	// +optional
	// +kubebuilder:default=false
	// +default=false
	// +k8s:conversion-gen=false
	DeployVMConsoleProxy *bool `json:"deployVmConsoleProxy,omitempty"`

	// EnableApplicationAwareQuota if true, enables the Application Aware Quota feature
	// +optional
	// +kubebuilder:default=false
	// +default=false
	// +k8s:conversion-gen=false
	EnableApplicationAwareQuota *bool `json:"enableApplicationAwareQuota,omitempty"`

	// LiveUpdateConfiguration holds the cluster configuration for live update of virtual machines - max cpu sockets,
	// max guest memory and max hotplug ratio. This setting can affect VM CPU and memory settings.
	// +optional
	// +k8s:conversion-gen=false
	LiveUpdateConfiguration *v1.LiveUpdateConfiguration `json:"liveUpdateConfiguration,omitempty"`

	// Hypervisors specifies which hypervisor the cluster uses to run virtual machines.
	// If empty or not set, KubeVirt defaults to KVM. Currently, only a single entry is supported.
	// Allowed values for the hypervisor name are "kvm" and "hyperv-direct".
	// +listType=atomic
	// +kubebuilder:validation:MaxItems:=1
	// +optional
	// +k8s:conversion-gen=false
	Hypervisors []v1.HypervisorConfiguration `json:"hypervisors,omitempty"`

	// RoleAggregationStrategy controls whether KubeVirt RBAC cluster roles should be aggregated
	// to the default Kubernetes roles (admin, edit, view).
	// When set to "AggregateToDefault" or not specified, the aggregate-to-* labels are added to the cluster roles.
	// When set to "Manual", the labels are not added, and roles will not be aggregated to the default roles.
	// +optional
	// +kubebuilder:validation:Enum=AggregateToDefault;Manual
	// +k8s:conversion-gen=false
	RoleAggregationStrategy *v1.RoleAggregationStrategy `json:"roleAggregationStrategy,omitempty"`
}

// HyperConvergedConfig defines a set of configurations to pass to components
// +k8s:conversion-gen=false
type HyperConvergedConfig struct {
	// NodePlacement describes node scheduling configuration.
	// +optional
	NodePlacement *sdkapi.NodePlacement `json:"nodePlacement,omitempty"`
}

// VirtualMachineOptions holds the cluster level information regarding the virtual machine.
// +k8s:conversion-gen=false
type VirtualMachineOptions struct {
	// DisableFreePageReporting disable the free page reporting of
	// memory balloon device https://libvirt.org/formatdomain.html#memory-balloon-device.
	// This will have effect only if AutoattachMemBalloon is not false and the vmi is not
	// requesting any high performance feature (dedicatedCPU/realtime/hugePages), in which free page reporting is always disabled.
	// +optional
	// +kubebuilder:default=false
	// +default=false
	DisableFreePageReporting *bool `json:"disableFreePageReporting,omitempty"`

	// DisableSerialConsoleLog disables logging the auto-attached default serial console.
	// If not set, serial console logs will be written to a file and then streamed from a container named `guest-console-log`.
	// The value can be individually overridden for each VM, not relevant if AutoattachSerialConsole is disabled for the VM.
	// +optional
	// +kubebuilder:default=false
	// +default=false
	DisableSerialConsoleLog *bool `json:"disableSerialConsoleLog,omitempty"`
}

// HyperConvergedFeatureGates is a set of optional feature gates to enable or disable new features that are not enabled
// by default yet.
// +k8s:openapi-gen=true
type HyperConvergedFeatureGates struct {
	// Allow to expose a limited set of host metrics to guests.
	// +optional
	// +kubebuilder:default=false
	// +default=false
	DownwardMetrics *bool `json:"downwardMetrics,omitempty"`

	// Deprecated: This feature gate is ignored
	WithHostPassthroughCPU *bool `json:"withHostPassthroughCPU,omitempty"`

	// Deprecated: This feature gate is ignored. Use spec.enableCommonBootImageImport instead
	EnableCommonBootImageImport *bool `json:"enableCommonBootImageImport,omitempty"`

	// Deprecated: This feature gate is ignored.
	DeployTektonTaskResources *bool `json:"deployTektonTaskResources,omitempty"`

	// Deprecated: This feature gate is ignored.
	// Use spec.deployVmConsoleProxy instead
	DeployVMConsoleProxy *bool `json:"deployVmConsoleProxy,omitempty"`

	// Deploy KubeSecondaryDNS by CNAO
	// +optional
	// +kubebuilder:default=false
	// +default=false
	DeployKubeSecondaryDNS *bool `json:"deployKubeSecondaryDNS,omitempty"`

	// Deprecated: this feature gate is ignored.
	DeployKubevirtIpamController *bool `json:"deployKubevirtIpamController,omitempty"`

	// Deprecated: This feature gate is ignored.
	NonRoot *bool `json:"nonRoot,omitempty"`

	// Disable mediated devices handling on KubeVirt
	// +optional
	// +kubebuilder:default=false
	// +default=false
	DisableMDevConfiguration *bool `json:"disableMDevConfiguration,omitempty"`

	// Enable persistent reservation of a LUN through the SCSI Persistent Reserve commands on Kubevirt.
	// In order to issue privileged SCSI ioctls, the VM requires activation of the persistent reservation flag.
	// Once this feature gate is enabled, then the additional container with the qemu-pr-helper is deployed inside the virt-handler pod.
	// Enabling (or removing) the feature gate causes the redeployment of the virt-handler pod.
	// +optional
	// +kubebuilder:default=false
	// +default=false
	PersistentReservation *bool `json:"persistentReservation,omitempty"`

	// Deprecated: This feature gate is ignored.
	EnableManagedTenantQuota *bool `json:"enableManagedTenantQuota,omitempty"`

	// TODO update description to also include cpu limits as well, after 4.14

	// Deprecated: this feature gate is ignored.
	AutoResourceLimits *bool `json:"autoResourceLimits,omitempty"`

	// Enable KubeVirt to request up to two additional dedicated CPUs
	// in order to complete the total CPU count to an even parity when using emulator thread isolation.
	// Note: this feature is in Developer Preview.
	// +optional
	// +kubebuilder:default=false
	// +default=false
	AlignCPUs *bool `json:"alignCPUs,omitempty"`

	// Deprecated: This field is ignored and will be removed on the next version of the API.
	// Use spec.enableApplicationAwareQuota instead
	EnableApplicationAwareQuota *bool `json:"enableApplicationAwareQuota,omitempty"`

	// Deprecated: this feature gate is ignored.
	PrimaryUserDefinedNetworkBinding *bool `json:"primaryUserDefinedNetworkBinding,omitempty"`

	// EnableMultiArchBootImageImport allows the HCO to run on heterogeneous clusters with different CPU architectures.
	// Setting this field to true will allow the HCO to create Golden Images for different CPU architectures.
	//
	// This feature is in Developer Preview.
	//
	// +optional
	// +kubebuilder:default=false
	// +default=false
	EnableMultiArchBootImageImport *bool `json:"enableMultiArchBootImageImport,omitempty"`

	// DecentralizedLiveMigration enables the decentralized live migration (cross-cluster migration) feature.
	// This feature allows live migration of VirtualMachineInstances between different clusters.
	// This feature is in Developer Preview.
	//
	// +optional
	// +kubebuilder:default=true
	// +default=true
	DecentralizedLiveMigration *bool `json:"decentralizedLiveMigration,omitempty"`

	// DeclarativeHotplugVolumes enables the use of the declarative volume hotplug feature in KubeVirt.
	// When set to true, the "DeclarativeHotplugVolumes" feature gate is enabled instead of "HotplugVolumes".
	// When set to false or nil, the "HotplugVolumes" feature gate is enabled (default behavior).
	// This feature is in Developer Preview.
	//
	// +optional
	// +kubebuilder:default=false
	// +default=false
	DeclarativeHotplugVolumes *bool `json:"declarativeHotplugVolumes,omitempty"`

	// VideoConfig allows users to configure video device types for their virtual machines.
	// This can be useful for workloads that require specific video capabilities or architectures.
	// Note: This feature is in Tech Preview.
	// +optional
	// +kubebuilder:default=true
	// +default=true
	VideoConfig *bool `json:"videoConfig,omitempty"`

	// ObjectGraph enables the ObjectGraph VM and VMI subresource in KubeVirt.
	// This subresource returns a structured list of k8s objects that are related to the specified VM or VMI, enabling better dependency tracking.
	// Note: This feature is in Developer Preview.
	// +optional
	// +kubebuilder:default=false
	// +default=false
	ObjectGraph *bool `json:"objectGraph,omitempty"`

	// IncrementalBackup enables changed block tracking backups and incremental backups using QEMU capabilities in KubeVirt.
	// When enabled, this also enables the UtilityVolumes feature gate in the KubeVirt CR.
	// Note: This feature is in Tech Preview.
	// +optional
	// +kubebuilder:default=false
	// +default=false
	IncrementalBackup *bool `json:"incrementalBackup,omitempty"`

	// ContainerPathVolumes enables the use of container paths as volumes in KubeVirt.
	// This allows VMs to access files and directories from the virt-launcher pod's filesystem via virtiofs.
	// +optional
	// +kubebuilder:default=false
	// +default=false
	ContainerPathVolumes *bool `json:"containerPathVolumes,omitempty"`
}

// MediatedDevicesConfiguration holds information about MDEV types to be defined, if available
// +k8s:openapi-gen=true
// +kubebuilder:validation:XValidation:rule="(has(self.mediatedDeviceTypes) && size(self.mediatedDeviceTypes)>0) || (has(self.mediatedDevicesTypes) && size(self.mediatedDevicesTypes)>0)",message="for mediatedDevicesConfiguration a non-empty mediatedDeviceTypes or mediatedDevicesTypes(deprecated) is required"
type MediatedDevicesConfiguration struct {
	// +optional
	// +listType=atomic
	MediatedDeviceTypes []string `json:"mediatedDeviceTypes"`

	// Deprecated: please use mediatedDeviceTypes instead.
	// +optional
	// +listType=atomic
	// +k8s:conversion-gen=false
	MediatedDevicesTypes []string `json:"mediatedDevicesTypes,omitempty"`

	// +optional
	// +listType=atomic
	NodeMediatedDeviceTypes []NodeMediatedDeviceTypesConfig `json:"nodeMediatedDeviceTypes,omitempty"`
}

// NodeMediatedDeviceTypesConfig holds information about MDEV types to be defined in a specific node that matches the NodeSelector field.
// +k8s:openapi-gen=true
// +kubebuilder:validation:XValidation:rule="(has(self.mediatedDeviceTypes) && size(self.mediatedDeviceTypes)>0) || (has(self.mediatedDevicesTypes) && size(self.mediatedDevicesTypes)>0)",message="for nodeMediatedDeviceTypes a non-empty mediatedDeviceTypes or mediatedDevicesTypes(deprecated) is required"
type NodeMediatedDeviceTypesConfig struct {

	// NodeSelector is a selector which must be true for the vmi to fit on a node.
	// Selector which must match a node's labels for the vmi to be scheduled on that node.
	// More info: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/
	NodeSelector map[string]string `json:"nodeSelector"`

	// +listType=atomic
	// +optional
	MediatedDeviceTypes []string `json:"mediatedDeviceTypes"`

	// Deprecated: please use mediatedDeviceTypes instead.
	// +listType=atomic
	// +optional
	// +k8s:conversion-gen=false
	MediatedDevicesTypes []string `json:"mediatedDevicesTypes"`
}

// OperandResourceRequirements is a list of resource requirements for the operand workloads pods
// +k8s:openapi-gen=true
// +k8s:conversion-gen=false
type OperandResourceRequirements struct {
	// StorageWorkloads defines the resources requirements for storage workloads. It will propagate to the CDI custom
	// resource
	// +optional
	StorageWorkloads *corev1.ResourceRequirements `json:"storageWorkloads,omitempty"`

	// VmiCPUAllocationRatio defines, for each requested virtual CPU,
	// how much physical CPU to request per VMI from the
	// hosting node. The value is in fraction of a CPU thread (or
	// core on non-hyperthreaded nodes).
	// VMI POD CPU request = number of vCPUs * 1/vmiCPUAllocationRatio
	// For example, a value of 1 means 1 physical CPU thread per VMI CPU thread.
	// A value of 100 would be 1% of a physical thread allocated for each
	// requested VMI thread.
	// This option has no effect on VMIs that request dedicated CPUs.
	// Defaults to 10
	// +kubebuilder:default=10
	// +kubebuilder:validation:Minimum=1
	// +default=10
	// +optional
	VmiCPUAllocationRatio *int `json:"vmiCPUAllocationRatio,omitempty"`

	// When set, AutoCPULimitNamespaceLabelSelector will set a CPU limit on virt-launcher for VMIs running inside
	// namespaces that match the label selector.
	// The CPU limit will equal the number of requested vCPUs.
	// This setting does not apply to VMIs with dedicated CPUs.
	// +optional
	AutoCPULimitNamespaceLabelSelector *metav1.LabelSelector `json:"autoCPULimitNamespaceLabelSelector,omitempty"`
}

// HyperConvergedObsoleteCPUs allows avoiding scheduling of VMs for obsolete CPU models
// +k8s:openapi-gen=true
type HyperConvergedObsoleteCPUs struct {
	// MinCPUModel is not in use
	// Deprecated: This field is not in use and is ignored.
	// +k8s:conversion-gen=false
	MinCPUModel string `json:"minCPUModel,omitempty"`
	// CPUModels is a list of obsolete CPU models. When the node-labeller obtains the list of obsolete CPU models, it
	// eliminates those CPU models and creates labels for valid CPU models.
	// The default values for this field is nil, however, HCO uses opinionated values, and adding values to this list
	// will add them to the opinionated values.
	// +listType=set
	// +optional
	CPUModels []string `json:"cpuModels,omitempty"`
}

// HyperConvergedStatus defines the observed state of HyperConverged
// +k8s:openapi-gen=true
type HyperConvergedStatus struct {
	// Conditions describes the state of the HyperConverged resource.
	// +listType=atomic
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"  patchStrategy:"merge" patchMergeKey:"type"`

	// RelatedObjects is a list of objects created and maintained by this
	// operator. Object references will be added to this list after they have
	// been created AND found in the cluster.
	// +listType=atomic
	// +optional
	RelatedObjects []corev1.ObjectReference `json:"relatedObjects,omitempty"`

	// Versions is a list of HCO component versions, as name/version pairs. The version with a name of "operator"
	// is the HCO version itself, as described here:
	// https://github.com/openshift/cluster-version-operator/blob/master/docs/dev/clusteroperator.md#version
	// +listType=atomic
	// +optional
	Versions []Version `json:"versions,omitempty"`

	// ObservedGeneration reflects the HyperConverged resource generation. If the ObservedGeneration is less than the
	// resource generation in metadata, the status is out of date
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// DataImportSchedule is the cron expression that is used in for the hard-coded data import cron templates. HCO
	// generates the value of this field once and stored in the status field, so will survive restart.
	// +optional
	DataImportSchedule string `json:"dataImportSchedule,omitempty"`

	// DataImportCronTemplates is a list of the actual DataImportCronTemplates as HCO update in the SSP CR. The list
	// contains both the common and the custom templates, including any modification done by HCO.
	DataImportCronTemplates []DataImportCronTemplateStatus `json:"dataImportCronTemplates,omitempty"`

	// SystemHealthStatus reflects the health of HCO and its secondary resources, based on the aggregated conditions.
	// +optional
	SystemHealthStatus string `json:"systemHealthStatus,omitempty"`

	// InfrastructureHighlyAvailable describes whether the cluster has only one worker node
	// (false) or more (true).
	// +optional
	InfrastructureHighlyAvailable *bool `json:"infrastructureHighlyAvailable,omitempty"`

	// NodeInfo holds information about the cluster nodes
	NodeInfo NodeInfoStatus `json:"nodeInfo,omitempty"`
}

type Version struct {
	Name    string `json:"name,omitempty"`
	Version string `json:"version,omitempty"`
}

// DataImportCronStatus is the status field of the DIC template
type DataImportCronStatus struct {
	// Conditions is a list of conditions that describe the state of the DataImportCronTemplate.
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// CommonTemplate indicates whether this is a common template (true), or a custom one (false)
	CommonTemplate bool `json:"commonTemplate,omitempty"`

	// Modified indicates if a common template was customized. Always false for custom templates.
	Modified bool `json:"modified,omitempty"`

	// OriginalSupportedArchitectures is a comma-separated list of CPU architectures that the original
	// template supports.
	OriginalSupportedArchitectures string `json:"originalSupportedArchitectures,omitempty"`
}

// DataImportCronTemplateStatus is a copy of a dataImportCronTemplate as defined in the spec, or in the HCO image.
type DataImportCronTemplateStatus struct {
	hcov1.DataImportCronTemplate `json:",inline"`

	Status DataImportCronStatus `json:"status,omitempty"`
}

// NodeInfoStatus holds information about the cluster nodes
type NodeInfoStatus struct {
	// WorkloadsArchitectures is a distinct list of the CPU architectures of the workloads nodes in the cluster.
	WorkloadsArchitectures []string `json:"workloadsArchitectures,omitempty"`
	// ControlPlaneArchitectures is a distinct list of the CPU architecture of the control-plane nodes.
	ControlPlaneArchitectures []string `json:"controlPlaneArchitectures,omitempty"`
}

// ApplicationAwareConfigurations holds the AAQ configurations
// +k8s:openapi-gen=true
type ApplicationAwareConfigurations struct {
	// VmiCalcConfigName determine how resource allocation will be done with ApplicationsResourceQuota.
	// allowed values are: VmiPodUsage, VirtualResources, DedicatedVirtualResources, IgnoreVmiCalculator or GuestEffectiveResources
	// +kubebuilder:validation:Enum=VmiPodUsage;VirtualResources;DedicatedVirtualResources;IgnoreVmiCalculator;GuestEffectiveResources
	// +kubebuilder:default=DedicatedVirtualResources
	VmiCalcConfigName *aaqv1alpha1.VmiCalcConfigName `json:"vmiCalcConfigName,omitempty"`

	// NamespaceSelector determines in which namespaces scheduling gate will be added to pods..
	NamespaceSelector *metav1.LabelSelector `json:"namespaceSelector,omitempty"`

	// AllowApplicationAwareClusterResourceQuota if set to true, allows creation and management of ClusterAppsResourceQuota
	// +kubebuilder:default=false
	AllowApplicationAwareClusterResourceQuota bool `json:"allowApplicationAwareClusterResourceQuota,omitempty"`
}

const (
	ConditionAvailable = "Available"

	// ConditionProgressing indicates that the operator is actively making changes to the resources maintained by the
	// operator
	ConditionProgressing = "Progressing"

	// ConditionDegraded indicates that the resources maintained by the operator are not functioning completely.
	// An example of a degraded state would be if not all pods in a deployment were running.
	// It may still be available, but it is degraded
	ConditionDegraded = "Degraded"

	// ConditionUpgradeable indicates whether the resources maintained by the operator are in a state that is safe to upgrade.
	// When `False`, the resources maintained by the operator should not be upgraded and the
	// message field should contain a human-readable description of what the administrator should do to
	// allow the operator to successfully update the resources maintained by the operator.
	ConditionUpgradeable = "Upgradeable"

	// ConditionReconcileComplete communicates the status of the HyperConverged resource's
	// reconcile functionality. Basically, is the Reconcile function running to completion.
	ConditionReconcileComplete = "ReconcileComplete"

	// ConditionTaintedConfiguration indicates that a hidden/debug configuration
	// has been applied to the HyperConverged resource via a specialized annotation.
	// This condition is exposed only when its value is True, and is otherwise hidden.
	ConditionTaintedConfiguration = "TaintedConfiguration"
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

	// +kubebuilder:default={"certConfig": {"ca": {"duration": "48h0m0s", "renewBefore": "24h0m0s"}, "server": {"duration": "24h0m0s", "renewBefore": "12h0m0s"}},"featureGates": {"downwardMetrics": false, "deployKubeSecondaryDNS": false, "disableMDevConfiguration": false, "persistentReservation": false, "enableMultiArchBootImageImport": false, "decentralizedLiveMigration": true, "declarativeHotplugVolumes": false, "videoConfig": true, "objectGraph": false, "incrementalBackup": false, "containerPathVolumes": false}, "liveMigrationConfig": {"completionTimeoutPerGiB": 150, "parallelMigrationsPerCluster": 5, "parallelOutboundMigrationsPerNode": 2, "progressTimeout": 150, "allowAutoConverge": false, "allowPostCopy": false}, "resourceRequirements": {"vmiCPUAllocationRatio": 10}, "uninstallStrategy": "BlockUninstallIfWorkloadsExist", "virtualMachineOptions": {"disableFreePageReporting": false, "disableSerialConsoleLog": false}, "enableApplicationAwareQuota": false, "enableCommonBootImageImport": true, "deployVmConsoleProxy": false}
	// +optional
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
