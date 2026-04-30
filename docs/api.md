# API Docs

This Document documents the types introduced by the hyperconverged-cluster-operator to be consumed by users.

> Note this document is generated from code comments. When contributing a change to this document please do so by changing the code comments.

## Table of Contents
* [ApplicationAwareConfigurations](#applicationawareconfigurations)
* [HyperConverged](#hyperconverged)
* [HyperConvergedConfig](#hyperconvergedconfig)
* [HyperConvergedFeatureGates](#hyperconvergedfeaturegates)
* [HyperConvergedList](#hyperconvergedlist)
* [HyperConvergedObsoleteCPUs](#hyperconvergedobsoletecpus)
* [HyperConvergedSpec](#hyperconvergedspec)
* [MediatedDevicesConfiguration](#mediateddevicesconfiguration)
* [NodeMediatedDeviceTypesConfig](#nodemediateddevicetypesconfig)
* [OperandResourceRequirements](#operandresourcerequirements)
* [VirtualMachineOptions](#virtualmachineoptions)


## ApplicationAwareConfigurations

ApplicationAwareConfigurations holds the AAQ configurations

| Field | Description | Scheme | Default | Required |
| ----- | ----------- | ------ | ------- | -------- |
| vmiCalcConfigName | VmiCalcConfigName determine how resource allocation will be done with ApplicationsResourceQuota. allowed values are: VmiPodUsage, VirtualResources, DedicatedVirtualResources, IgnoreVmiCalculator or GuestEffectiveResources | *aaqv1alpha1.VmiCalcConfigName | DedicatedVirtualResources | false |
| namespaceSelector | NamespaceSelector determines in which namespaces scheduling gate will be added to pods.. | *[metav1.LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#labelselector-v1-meta) |  | false |
| allowApplicationAwareClusterResourceQuota | AllowApplicationAwareClusterResourceQuota if set to true, allows creation and management of ClusterAppsResourceQuota | bool | false | false |

[Back to TOC](#table-of-contents)

## HyperConverged

HyperConverged is the Schema for the hyperconvergeds API

| Field | Description | Scheme | Default | Required |
| ----- | ----------- | ------ | ------- | -------- |
| metadata |  | [metav1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#objectmeta-v1-meta) |  | false |
| spec |  | [HyperConvergedSpec](#hyperconvergedspec) | {"certConfig": {"ca": {"duration": "48h0m0s", "renewBefore": "24h0m0s"}, "server": {"duration": "24h0m0s", "renewBefore": "12h0m0s"}},"featureGates": {"downwardMetrics": false, "deployKubeSecondaryDNS": false, "disableMDevConfiguration": false, "persistentReservation": false, "enableMultiArchBootImageImport": false, "decentralizedLiveMigration": true, "declarativeHotplugVolumes": false, "videoConfig": true, "objectGraph": false, "incrementalBackup": false, "containerPathVolumes": false}, "liveMigrationConfig": {"completionTimeoutPerGiB": 150, "parallelMigrationsPerCluster": 5, "parallelOutboundMigrationsPerNode": 2, "progressTimeout": 150, "allowAutoConverge": false, "allowPostCopy": false}, "resourceRequirements": {"vmiCPUAllocationRatio": 10}, "uninstallStrategy": "BlockUninstallIfWorkloadsExist", "virtualMachineOptions": {"disableFreePageReporting": false, "disableSerialConsoleLog": false}, "enableApplicationAwareQuota": false, "enableCommonBootImageImport": true, "deployVmConsoleProxy": false} | false |
| status |  | hcov1.HyperConvergedStatus |  | false |

[Back to TOC](#table-of-contents)

## HyperConvergedConfig

HyperConvergedConfig defines a set of configurations to pass to components

| Field | Description | Scheme | Default | Required |
| ----- | ----------- | ------ | ------- | -------- |
| nodePlacement | NodePlacement describes node scheduling configuration. | *[sdkapi.NodePlacement](https://github.com/kubevirt/controller-lifecycle-operator-sdk/blob/bbf16167410b7a781c7b08a3f088fc39551c7a00/pkg/sdk/api/types.go#L49) |  | false |

[Back to TOC](#table-of-contents)

## HyperConvergedFeatureGates

HyperConvergedFeatureGates is a set of optional feature gates to enable or disable new features that are not enabled by default yet.

| Field | Description | Scheme | Default | Required |
| ----- | ----------- | ------ | ------- | -------- |
| downwardMetrics | Allow to expose a limited set of host metrics to guests. | *bool | false | false |
| withHostPassthroughCPU | Deprecated: This feature gate is ignored | *bool |  | false |
| enableCommonBootImageImport | Deprecated: This feature gate is ignored. Use spec.enableCommonBootImageImport instead | *bool |  | false |
| deployTektonTaskResources | Deprecated: This feature gate is ignored. | *bool |  | false |
| deployVmConsoleProxy | Deprecated: This feature gate is ignored. Use spec.deployVmConsoleProxy instead | *bool |  | false |
| deployKubeSecondaryDNS | Deploy KubeSecondaryDNS by CNAO | *bool | false | false |
| deployKubevirtIpamController | Deprecated: this feature gate is ignored. | *bool |  | false |
| nonRoot | Deprecated: This feature gate is ignored. | *bool |  | false |
| disableMDevConfiguration | Deprecated: This feature gate will be removed in a future release. Use spec.mediatedDevicesConfiguration.enabled instead. | *bool | false | false |
| persistentReservation | Enable persistent reservation of a LUN through the SCSI Persistent Reserve commands on Kubevirt. In order to issue privileged SCSI ioctls, the VM requires activation of the persistent reservation flag. Once this feature gate is enabled, then the additional container with the qemu-pr-helper is deployed inside the virt-handler pod. Enabling (or removing) the feature gate causes the redeployment of the virt-handler pod. | *bool | false | false |
| enableManagedTenantQuota | Deprecated: This feature gate is ignored. | *bool |  | false |
| autoResourceLimits | Deprecated: this feature gate is ignored. | *bool |  | false |
| alignCPUs | Enable KubeVirt to request up to two additional dedicated CPUs in order to complete the total CPU count to an even parity when using emulator thread isolation. Note: this feature is in Developer Preview. | *bool | false | false |
| enableApplicationAwareQuota | Deprecated: This field is ignored and will be removed on the next version of the API. Use spec.enableApplicationAwareQuota instead | *bool |  | false |
| primaryUserDefinedNetworkBinding | Deprecated: this feature gate is ignored. | *bool |  | false |
| enableMultiArchBootImageImport | EnableMultiArchBootImageImport allows the HCO to run on heterogeneous clusters with different CPU architectures. Setting this field to true will allow the HCO to create Golden Images for different CPU architectures.\n\nThis feature is in Developer Preview. | *bool | false | false |
| decentralizedLiveMigration | DecentralizedLiveMigration enables the decentralized live migration (cross-cluster migration) feature. This feature allows live migration of VirtualMachineInstances between different clusters. This feature is in Developer Preview. | *bool | true | false |
| declarativeHotplugVolumes | DeclarativeHotplugVolumes enables the use of the declarative volume hotplug feature in KubeVirt. When set to true, the \"DeclarativeHotplugVolumes\" feature gate is enabled instead of \"HotplugVolumes\". When set to false or nil, the \"HotplugVolumes\" feature gate is enabled (default behavior). This feature is in Developer Preview. | *bool | false | false |
| videoConfig | VideoConfig allows users to configure video device types for their virtual machines. This can be useful for workloads that require specific video capabilities or architectures. Note: This feature is in Tech Preview. | *bool | true | false |
| objectGraph | ObjectGraph enables the ObjectGraph VM and VMI subresource in KubeVirt. This subresource returns a structured list of k8s objects that are related to the specified VM or VMI, enabling better dependency tracking. Note: This feature is in Developer Preview. | *bool | false | false |
| incrementalBackup | IncrementalBackup enables changed block tracking backups and incremental backups using QEMU capabilities in KubeVirt. When enabled, this also enables the UtilityVolumes feature gate in the KubeVirt CR. Note: This feature is in Tech Preview. | *bool | false | false |
| containerPathVolumes | ContainerPathVolumes enables the use of container paths as volumes in KubeVirt. This allows VMs to access files and directories from the virt-launcher pod's filesystem via virtiofs. | *bool | false | false |

[Back to TOC](#table-of-contents)

## HyperConvergedList

HyperConvergedList contains a list of HyperConverged

| Field | Description | Scheme | Default | Required |
| ----- | ----------- | ------ | ------- | -------- |
| metadata |  | [metav1.ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#listmeta-v1-meta) |  | false |
| items |  | [][HyperConverged](#hyperconverged) |  | true |

[Back to TOC](#table-of-contents)

## HyperConvergedObsoleteCPUs

HyperConvergedObsoleteCPUs allows avoiding scheduling of VMs for obsolete CPU models

| Field | Description | Scheme | Default | Required |
| ----- | ----------- | ------ | ------- | -------- |
| minCPUModel | MinCPUModel is not in use Deprecated: This field is not in use and is ignored. | string |  | false |
| cpuModels | CPUModels is a list of obsolete CPU models. When the node-labeller obtains the list of obsolete CPU models, it eliminates those CPU models and creates labels for valid CPU models. The default values for this field is nil, however, HCO uses opinionated values, and adding values to this list will add them to the opinionated values. | []string |  | false |

[Back to TOC](#table-of-contents)

## HyperConvergedSpec

HyperConvergedSpec defines the desired state of HyperConverged

| Field | Description | Scheme | Default | Required |
| ----- | ----------- | ------ | ------- | -------- |
| localStorageClassName | Deprecated: LocalStorageClassName the name of the local storage class. | string |  | false |
| tuningPolicy | TuningPolicy allows to configure the mode in which the RateLimits of kubevirt are set. If TuningPolicy is not present the default kubevirt values are used. It can be set to `annotation` for fine-tuning the kubevirt queryPerSeconds (qps) and burst values. Qps and burst values are taken from the annotation hco.kubevirt.io/tuningPolicy | hcov1.HyperConvergedTuningPolicy |  | false |
| infra | infra HyperConvergedConfig influences the pod configuration (currently only placement) for all the infra components needed on the virtualization enabled cluster but not necessarily directly on each node running VMs/VMIs. | [HyperConvergedConfig](#hyperconvergedconfig) |  | false |
| workloads | workloads HyperConvergedConfig influences the pod configuration (currently only placement) of components which need to be running on a node where virtualization workloads should be able to run. Changes to Workloads HyperConvergedConfig can be applied only without existing workload. | [HyperConvergedConfig](#hyperconvergedconfig) |  | false |
| featureGates | featureGates is a map of feature gate flags. Setting a flag to `true` will enable the feature. Setting `false` or removing the feature gate, disables the feature. | [HyperConvergedFeatureGates](#hyperconvergedfeaturegates) | {"downwardMetrics": false, "deployKubeSecondaryDNS": false, "disableMDevConfiguration": false, "persistentReservation": false, "enableMultiArchBootImageImport": false, "decentralizedLiveMigration": true, "declarativeHotplugVolumes": false, "videoConfig": true, "objectGraph": false, "incrementalBackup": false, "containerPathVolumes": false} | false |
| liveMigrationConfig | Live migration limits and timeouts are applied so that migration processes do not overwhelm the cluster. | hcov1.LiveMigrationConfigurations | {"completionTimeoutPerGiB": 150, "parallelMigrationsPerCluster": 5, "parallelOutboundMigrationsPerNode": 2, "progressTimeout": 150, "allowAutoConverge": false, "allowPostCopy": false} | false |
| permittedHostDevices | PermittedHostDevices holds information about devices allowed for passthrough | *hcov1.PermittedHostDevices |  | false |
| mediatedDevicesConfiguration | MediatedDevicesConfiguration holds information about MDEV types to be defined on nodes, if available | *[MediatedDevicesConfiguration](#mediateddevicesconfiguration) |  | false |
| certConfig | certConfig holds the rotation policy for internal, self-signed certificates | hcov1.HyperConvergedCertConfig | {"ca": {"duration": "48h0m0s", "renewBefore": "24h0m0s"}, "server": {"duration": "24h0m0s", "renewBefore": "12h0m0s"}} | false |
| resourceRequirements | ResourceRequirements describes the resource requirements for the operand workloads. | *[OperandResourceRequirements](#operandresourcerequirements) | {"vmiCPUAllocationRatio": 10} | false |
| scratchSpaceStorageClass | Override the storage class used for scratch space during transfer operations. The scratch space storage class is determined in the following order: value of scratchSpaceStorageClass, if that doesn't exist, use the default storage class, if there is no default storage class, use the storage class of the DataVolume, if no storage class specified, use no storage class for scratch space | *string |  | false |
| vddkInitImage | VDDK Init Image eventually used to import VMs from external providers\n\nDeprecated: please use the Migration Toolkit for Virtualization | *string |  | false |
| defaultCPUModel | DefaultCPUModel defines a cluster default for CPU model: default CPU model is set when VMI doesn't have any CPU model. When VMI has CPU model set, then VMI's CPU model is preferred. When default CPU model is not set and VMI's CPU model is not set too, host-model will be set. Default CPU model can be changed when kubevirt is running. | *string |  | false |
| defaultRuntimeClass | DefaultRuntimeClass defines a cluster default for the RuntimeClass to be used for VMIs pods if not set there. Default RuntimeClass can be changed when kubevirt is running, existing VMIs are not impacted till the next restart/live-migration when they are eventually going to consume the new default RuntimeClass. | *string |  | false |
| obsoleteCPUs | ObsoleteCPUs allows avoiding scheduling of VMs for obsolete CPU models | *[HyperConvergedObsoleteCPUs](#hyperconvergedobsoletecpus) |  | false |
| commonTemplatesNamespace | CommonTemplatesNamespace defines namespace in which common templates will be deployed. It overrides the default openshift namespace. | *string |  | false |
| storageImport | StorageImport contains configuration for importing containerized data | *hcov1.StorageImportConfig |  | false |
| workloadUpdateStrategy | WorkloadUpdateStrategy defines at the cluster level how to handle automated workload updates | hcov1.HyperConvergedWorkloadUpdateStrategy | {"workloadUpdateMethods": {"LiveMigrate"}, "batchEvictionSize": 10, "batchEvictionInterval": "1m0s"} | false |
| dataImportCronTemplates | DataImportCronTemplates holds list of data import cron templates (golden images) | []hcov1.DataImportCronTemplate |  | false |
| filesystemOverhead | FilesystemOverhead describes the space reserved for overhead when using Filesystem volumes. A value is between 0 and 1, if not defined it is 0.055 (5.5 percent overhead) | *cdiv1beta1.FilesystemOverhead |  | false |
| uninstallStrategy | UninstallStrategy defines how to proceed on uninstall when workloads (VirtualMachines, DataVolumes) still exist. BlockUninstallIfWorkloadsExist will prevent the CR from being removed when workloads still exist. BlockUninstallIfWorkloadsExist is the safest choice to protect your workloads from accidental data loss, so it's strongly advised. RemoveWorkloads will cause all the workloads to be cascading deleted on uninstallation. WARNING: please notice that RemoveWorkloads will cause your workloads to be deleted as soon as this CR will be, even accidentally, deleted. Please correctly consider the implications of this option before setting it. BlockUninstallIfWorkloadsExist is the default behaviour. | hcov1.HyperConvergedUninstallStrategy | BlockUninstallIfWorkloadsExist | false |
| logVerbosityConfig | LogVerbosityConfig configures the verbosity level of Kubevirt's different components. The higher the value - the higher the log verbosity. | *hcov1.LogVerbosityConfiguration |  | false |
| tlsSecurityProfile | TLSSecurityProfile specifies the settings for TLS connections to be propagated to all kubevirt-hyperconverged components. If unset, the hyperconverged cluster operator will consume the value set on the APIServer CR on OCP/OKD or Intermediate if on vanilla k8s. Note that only Old, Intermediate and Custom profiles are currently supported, and the maximum available MinTLSVersions is VersionTLS12. | *openshiftconfigv1.TLSSecurityProfile |  | false |
| tektonPipelinesNamespace | TektonPipelinesNamespace defines namespace in which example pipelines will be deployed. If unset, then the default value is the operator namespace. Deprecated: This field is ignored. | *string |  | false |
| tektonTasksNamespace | TektonTasksNamespace defines namespace in which tekton tasks will be deployed. If unset, then the default value is the operator namespace. Deprecated: This field is ignored. | *string |  | false |
| kubeSecondaryDNSNameServerIP | KubeSecondaryDNSNameServerIP defines name server IP used by KubeSecondaryDNS | *string |  | false |
| kubeMacPoolConfiguration | KubeMacPoolConfiguration holds kubemacpool MAC address range configuration. | *hcov1.KubeMacPoolConfig |  | false |
| evictionStrategy | EvictionStrategy defines at the cluster level if the VirtualMachineInstance should be migrated instead of shut-off in case of a node drain. If the VirtualMachineInstance specific field is set it overrides the cluster level one. Allowed values: - `None` no eviction strategy at cluster level. - `LiveMigrate` migrate the VM on eviction; a not live migratable VM with no specific strategy will block the drain of the node util manually evicted. - `LiveMigrateIfPossible` migrate the VM on eviction if live migration is possible, otherwise directly evict. - `External` block the drain, track eviction and notify an external controller. Defaults to LiveMigrate with multiple worker nodes, None on single worker clusters. | *v1.EvictionStrategy |  | false |
| vmStateStorageClass | VMStateStorageClass is the name of the storage class to use for the PVCs created to preserve VM state, like TPM. | *string |  | false |
| virtualMachineOptions | VirtualMachineOptions holds the cluster level information regarding the virtual machine. | *[VirtualMachineOptions](#virtualmachineoptions) | {"disableFreePageReporting": false, "disableSerialConsoleLog": false} | false |
| commonBootImageNamespace | CommonBootImageNamespace override the default namespace of the common boot images, in order to hide them.\n\nIf not set, HCO won't set any namespace, letting SSP to use the default. If set, use the namespace to create the DataImportCronTemplates and the common image streams, with this namespace. This field is not set by default. | *string |  | false |
| ksmConfiguration | KSMConfiguration holds the information regarding the enabling the KSM in the nodes (if available). | *v1.KSMConfiguration |  | false |
| networkBinding | NetworkBinding defines the network binding plugins. Those bindings can be used when defining virtual machine interfaces. | map[string]v1.InterfaceBindingPlugin |  | false |
| applicationAwareConfig | ApplicationAwareConfig set the AAQ configurations | *[ApplicationAwareConfigurations](#applicationawareconfigurations) |  | false |
| higherWorkloadDensity | HigherWorkloadDensity holds configuration aimed to increase virtual machine density | *hcov1.HigherWorkloadDensityConfiguration | {"memoryOvercommitPercentage": 100} | false |
| enableCommonBootImageImport | Opt-in to automatic delivery/updates of the common data import cron templates. There are two sources for the data import cron templates: hard coded list of common templates, and custom (user defined) templates that can be added to the dataImportCronTemplates field. This field only controls the common templates. It is possible to use custom templates by adding them to the dataImportCronTemplates field. | *bool | true | false |
| instancetypeConfig | InstancetypeConfig holds the configuration of instance type related functionality within KubeVirt. | *v1.InstancetypeConfiguration |  | false |
| CommonInstancetypesDeployment | CommonInstancetypesDeployment holds the configuration of common-instancetypes deployment within KubeVirt. | *v1.CommonInstancetypesDeployment |  | false |
| deployVmConsoleProxy | deploy VM console proxy resources in SSP operator | *bool | false | false |
| enableApplicationAwareQuota | EnableApplicationAwareQuota if true, enables the Application Aware Quota feature | *bool | false | false |
| liveUpdateConfiguration | LiveUpdateConfiguration holds the cluster configuration for live update of virtual machines - max cpu sockets, max guest memory and max hotplug ratio. This setting can affect VM CPU and memory settings. | *v1.LiveUpdateConfiguration |  | false |
| hypervisors | Hypervisors specifies which hypervisor the cluster uses to run virtual machines. If empty or not set, KubeVirt defaults to KVM. Currently, only a single entry is supported. Allowed values for the hypervisor name are \"kvm\" and \"hyperv-direct\". | []v1.HypervisorConfiguration |  | false |
| roleAggregationStrategy | RoleAggregationStrategy controls whether KubeVirt RBAC cluster roles should be aggregated to the default Kubernetes roles (admin, edit, view). When set to \"AggregateToDefault\" or not specified, the aggregate-to-* labels are added to the cluster roles. When set to \"Manual\", the labels are not added, and roles will not be aggregated to the default roles. | *v1.RoleAggregationStrategy |  | false |

[Back to TOC](#table-of-contents)

## MediatedDevicesConfiguration

MediatedDevicesConfiguration holds information about MDEV types to be defined, if available

| Field | Description | Scheme | Default | Required |
| ----- | ----------- | ------ | ------- | -------- |
| mediatedDeviceTypes |  | []string |  | true |
| mediatedDevicesTypes | Deprecated: please use mediatedDeviceTypes instead. | []string |  | false |
| nodeMediatedDeviceTypes |  | [][NodeMediatedDeviceTypesConfig](#nodemediateddevicetypesconfig) |  | false |

[Back to TOC](#table-of-contents)

## NodeMediatedDeviceTypesConfig

NodeMediatedDeviceTypesConfig holds information about MDEV types to be defined in a specific node that matches the NodeSelector field.

| Field | Description | Scheme | Default | Required |
| ----- | ----------- | ------ | ------- | -------- |
| nodeSelector | NodeSelector is a selector which must be true for the vmi to fit on a node. Selector which must match a node's labels for the vmi to be scheduled on that node. More info: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/ | map[string]string |  | true |
| mediatedDeviceTypes |  | []string |  | true |
| mediatedDevicesTypes | Deprecated: please use mediatedDeviceTypes instead. | []string |  | true |

[Back to TOC](#table-of-contents)

## OperandResourceRequirements

OperandResourceRequirements is a list of resource requirements for the operand workloads pods

| Field | Description | Scheme | Default | Required |
| ----- | ----------- | ------ | ------- | -------- |
| storageWorkloads | StorageWorkloads defines the resources requirements for storage workloads. It will propagate to the CDI custom resource | *corev1.ResourceRequirements |  | false |
| vmiCPUAllocationRatio | VmiCPUAllocationRatio defines, for each requested virtual CPU, how much physical CPU to request per VMI from the hosting node. The value is in fraction of a CPU thread (or core on non-hyperthreaded nodes). VMI POD CPU request = number of vCPUs * 1/vmiCPUAllocationRatio For example, a value of 1 means 1 physical CPU thread per VMI CPU thread. A value of 100 would be 1% of a physical thread allocated for each requested VMI thread. This option has no effect on VMIs that request dedicated CPUs. Defaults to 10 | *int | 10 | false |
| autoCPULimitNamespaceLabelSelector | When set, AutoCPULimitNamespaceLabelSelector will set a CPU limit on virt-launcher for VMIs running inside namespaces that match the label selector. The CPU limit will equal the number of requested vCPUs. This setting does not apply to VMIs with dedicated CPUs. | *[metav1.LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#labelselector-v1-meta) |  | false |

[Back to TOC](#table-of-contents)

## VirtualMachineOptions

VirtualMachineOptions holds the cluster level information regarding the virtual machine.

| Field | Description | Scheme | Default | Required |
| ----- | ----------- | ------ | ------- | -------- |
| disableFreePageReporting | DisableFreePageReporting disable the free page reporting of memory balloon device https://libvirt.org/formatdomain.html#memory-balloon-device. This will have effect only if AutoattachMemBalloon is not false and the vmi is not requesting any high performance feature (dedicatedCPU/realtime/hugePages), in which free page reporting is always disabled. | *bool | false | false |
| disableSerialConsoleLog | DisableSerialConsoleLog disables logging the auto-attached default serial console. If not set, serial console logs will be written to a file and then streamed from a container named `guest-console-log`. The value can be individually overridden for each VM, not relevant if AutoattachSerialConsole is disabled for the VM. | *bool | false | false |

[Back to TOC](#table-of-contents)
