# API Docs

This Document documents the types introduced by the hyperconverged-cluster-operator to be consumed by users.

> Note this document is generated from code comments. When contributing a change to this document please do so by changing the code comments.

## Table of Contents
* [ApplicationAwareConfigurations](#applicationawareconfigurations)
* [CertRotateConfigCA](#certrotateconfigca)
* [CertRotateConfigServer](#certrotateconfigserver)
* [DataImportCronStatus](#dataimportcronstatus)
* [DataImportCronTemplate](#dataimportcrontemplate)
* [DataImportCronTemplateStatus](#dataimportcrontemplatestatus)
* [DeploymentConfig](#deploymentconfig)
* [HigherWorkloadDensityConfiguration](#higherworkloaddensityconfiguration)
* [HyperConverged](#hyperconverged)
* [HyperConvergedCertConfig](#hyperconvergedcertconfig)
* [HyperConvergedList](#hyperconvergedlist)
* [HyperConvergedSpec](#hyperconvergedspec)
* [HyperConvergedStatus](#hyperconvergedstatus)
* [HyperConvergedWorkloadUpdateStrategy](#hyperconvergedworkloadupdatestrategy)
* [KubeMacPoolConfig](#kubemacpoolconfig)
* [LiveMigrationConfigurations](#livemigrationconfigurations)
* [LogVerbosityConfiguration](#logverbosityconfiguration)
* [MediatedDevicesConfiguration](#mediateddevicesconfiguration)
* [MediatedHostDevice](#mediatedhostdevice)
* [NetworkingConfig](#networkingconfig)
* [NodeInfoStatus](#nodeinfostatus)
* [NodeMediatedDeviceTypesConfig](#nodemediateddevicetypesconfig)
* [NodePlacements](#nodeplacements)
* [PciHostDevice](#pcihostdevice)
* [PermittedHostDevices](#permittedhostdevices)
* [SecurityConfig](#securityconfig)
* [StorageConfig](#storageconfig)
* [StorageImportConfig](#storageimportconfig)
* [USBHostDevice](#usbhostdevice)
* [USBSelector](#usbselector)
* [Version](#version)
* [VirtualMachineOptions](#virtualmachineoptions)
* [VirtualizationConfig](#virtualizationconfig)
* [WorkloadSourcesConfig](#workloadsourcesconfig)
* [HCO Feature Gates](#hco-feature-gates)


## ApplicationAwareConfigurations

ApplicationAwareConfigurations holds the AAQ configurations

| Field | Description | Scheme | Default | Required |
| ----- | ----------- | ------ | ------- | -------- |
| enable | Enable if true, enables the Application Aware Quota feature | *bool | false | false |
| vmiCalcConfigName | VmiCalcConfigName determine how resource allocation will be done with ApplicationsResourceQuota. allowed values are: VmiPodUsage, VirtualResources, DedicatedVirtualResources, IgnoreVmiCalculator or GuestEffectiveResources | *aaqv1alpha1.VmiCalcConfigName | DedicatedVirtualResources | false |
| namespaceSelector | NamespaceSelector determines in which namespaces scheduling gate will be added to pods.. | *[metav1.LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#labelselector-v1-meta) |  | false |
| allowApplicationAwareClusterResourceQuota | AllowApplicationAwareClusterResourceQuota if set to true, allows creation and management of ClusterAppsResourceQuota | bool | false | false |

[Back to TOC](#table-of-contents)

## CertRotateConfigCA

CertRotateConfigCA contains the tunables for TLS certificates.

| Field | Description | Scheme | Default | Required |
| ----- | ----------- | ------ | ------- | -------- |
| duration | The requested 'duration' (i.e. lifetime) of the Certificate. This should comply with golang's ParseDuration format (https://golang.org/pkg/time/#ParseDuration) | *metav1.Duration | "48h0m0s" | false |
| renewBefore | The amount of time before the currently issued certificate's `notAfter` time that we will begin to attempt to renew the certificate. This should comply with golang's ParseDuration format (https://golang.org/pkg/time/#ParseDuration) | *metav1.Duration | "24h0m0s" | false |

[Back to TOC](#table-of-contents)

## CertRotateConfigServer

CertRotateConfigServer contains the tunables for TLS certificates.

| Field | Description | Scheme | Default | Required |
| ----- | ----------- | ------ | ------- | -------- |
| duration | The requested 'duration' (i.e. lifetime) of the Certificate. This should comply with golang's ParseDuration format (https://golang.org/pkg/time/#ParseDuration) | *metav1.Duration | "24h0m0s" | false |
| renewBefore | The amount of time before the currently issued certificate's `notAfter` time that we will begin to attempt to renew the certificate. This should comply with golang's ParseDuration format (https://golang.org/pkg/time/#ParseDuration) | *metav1.Duration | "12h0m0s" | false |

[Back to TOC](#table-of-contents)

## DataImportCronStatus

DataImportCronStatus is the status field of the DIC template

| Field | Description | Scheme | Default | Required |
| ----- | ----------- | ------ | ------- | -------- |
| conditions | Conditions is a list of conditions that describe the state of the DataImportCronTemplate. | []metav1.Condition |  | false |
| commonTemplate | CommonTemplate indicates whether this is a common template (true), or a custom one (false) | bool |  | false |
| modified | Modified indicates if a common template was customized. Always false for custom templates. | bool |  | false |
| originalSupportedArchitectures | OriginalSupportedArchitectures is a comma-separated list of CPU architectures that the original template supports. | string |  | false |

[Back to TOC](#table-of-contents)

## DataImportCronTemplate

DataImportCronTemplate defines the template type for DataImportCrons. It requires metadata.name to be specified while leaving namespace as optional.

| Field | Description | Scheme | Default | Required |
| ----- | ----------- | ------ | ------- | -------- |
| metadata |  | [metav1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#objectmeta-v1-meta) |  | false |
| spec |  | *cdiv1beta1.DataImportCronSpec |  | false |

[Back to TOC](#table-of-contents)

## DataImportCronTemplateStatus

DataImportCronTemplateStatus is a copy of a dataImportCronTemplate as defined in the spec, or in the HCO image.

| Field | Description | Scheme | Default | Required |
| ----- | ----------- | ------ | ------- | -------- |
| metadata |  | [metav1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#objectmeta-v1-meta) |  | false |
| spec |  | *cdiv1beta1.DataImportCronSpec |  | false |
| status |  | [DataImportCronStatus](#dataimportcronstatus) |  | false |

[Back to TOC](#table-of-contents)

## DeploymentConfig



| Field | Description | Scheme | Default | Required |
| ----- | ----------- | ------ | ------- | -------- |
| nodePlacements | NodePlacements defines the node scheduling configuration for infrastructure or workload entities | *[NodePlacements](#nodeplacements) |  | false |
| uninstallStrategy | UninstallStrategy defines how to proceed on uninstall when workloads (VirtualMachines, DataVolumes) still exist. BlockUninstallIfWorkloadsExist will prevent the CR from being removed when workloads still exist. BlockUninstallIfWorkloadsExist is the safest choice to protect your workloads from accidental data loss, so it's strongly advised. RemoveWorkloads will cause all the workloads to be cascading deleted on uninstallation. WARNING: please notice that RemoveWorkloads will cause your workloads to be deleted as soon as this CR will be, even accidentally, deleted. Please correctly consider the implications of this option before setting it. BlockUninstallIfWorkloadsExist is the default behavior. | HyperConvergedUninstallStrategy | BlockUninstallIfWorkloadsExist | false |
| logVerbosityConfig | LogVerbosityConfig configures the verbosity level of Kubevirt's different components. The higher the value - the higher the log verbosity. | *[LogVerbosityConfiguration](#logverbosityconfiguration) |  | false |
| applicationAwareConfig | ApplicationAwareConfig set the AAQ configurations | *[ApplicationAwareConfigurations](#applicationawareconfigurations) |  | false |
| deployVmConsoleProxy | deploy VM console proxy resources in SSP operator | *bool | false | false |

[Back to TOC](#table-of-contents)

## HigherWorkloadDensityConfiguration

HigherWorkloadDensityConfiguration holds configuration aimed to increase virtual machine density

| Field | Description | Scheme | Default | Required |
| ----- | ----------- | ------ | ------- | -------- |
| memoryOvercommitPercentage | MemoryOvercommitPercentage is the percentage of memory we want to give VMIs compared to the amount given to its parent pod (virt-launcher). For example, a value of 102 means the VMI will \"see\" 2% more memory than its parent pod. Values under 100 are effectively \"undercommits\". Overcommits can lead to memory exhaustion, which in turn can lead to crashes. Use carefully. | int | 100 | false |

[Back to TOC](#table-of-contents)

## HyperConverged

HyperConverged is the Schema for the hyperconvergeds API

| Field | Description | Scheme | Default | Required |
| ----- | ----------- | ------ | ------- | -------- |
| metadata |  | [metav1.ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#objectmeta-v1-meta) |  | false |
| spec |  | [HyperConvergedSpec](#hyperconvergedspec) | {"security": {"certConfig": {"ca": {"duration": "48h0m0s", "renewBefore": "24h0m0s"}, "server": {"duration": "24h0m0s", "renewBefore": "12h0m0s"}}}, "virtualization": {"liveMigrationConfig": {"completionTimeoutPerGiB": 150, "parallelMigrationsPerCluster": 5, "parallelOutboundMigrationsPerNode": 2, "progressTimeout": 150, "allowAutoConverge": false, "allowPostCopy": false}, "virtualMachineOptions": {"disableFreePageReporting": false, "disableSerialConsoleLog": false}, "vmiCPUAllocationRatio": 10},"workloadSources":{"enableCommonBootImageImport":true}, "deployment": {"uninstallStrategy": "BlockUninstallIfWorkloadsExist", "deployVmConsoleProxy": false, "applicationAwareConfig": {"enable": false}}} | false |
| status |  | [HyperConvergedStatus](#hyperconvergedstatus) |  | false |

[Back to TOC](#table-of-contents)

## HyperConvergedCertConfig

HyperConvergedCertConfig holds the CertConfig entries for the HCO operands

| Field | Description | Scheme | Default | Required |
| ----- | ----------- | ------ | ------- | -------- |
| ca | CA configuration - CA certs are kept in the CA bundle as long as they are valid | [CertRotateConfigCA](#certrotateconfigca) | {"duration": "48h0m0s", "renewBefore": "24h0m0s"} | false |
| server | Server configuration - Certs are rotated and discarded | [CertRotateConfigServer](#certrotateconfigserver) | {"duration": "24h0m0s", "renewBefore": "12h0m0s"} | false |

[Back to TOC](#table-of-contents)

## HyperConvergedList

HyperConvergedList contains a list of HyperConverged

| Field | Description | Scheme | Default | Required |
| ----- | ----------- | ------ | ------- | -------- |
| metadata |  | [metav1.ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#listmeta-v1-meta) |  | false |
| items |  | [][HyperConverged](#hyperconverged) |  | true |

[Back to TOC](#table-of-contents)

## HyperConvergedSpec

HyperConvergedSpec defines the desired state of HyperConverged

| Field | Description | Scheme | Default | Required |
| ----- | ----------- | ------ | ------- | -------- |
| featureGates | For feature gate details, see [here](#hco-feature-gates) | featuregates.HyperConvergedFeatureGates |  | false |
| virtualization | Virtualization contains all the configurations for virtualization | [VirtualizationConfig](#virtualizationconfig) | {"liveMigrationConfig": {"completionTimeoutPerGiB": 150, "parallelMigrationsPerCluster": 5, "parallelOutboundMigrationsPerNode": 2, "progressTimeout": 150, "allowAutoConverge": false, "allowPostCopy": false}, "virtualMachineOptions": {"disableFreePageReporting": false, "disableSerialConsoleLog": false}, "vmiCPUAllocationRatio": 10} | false |
| storage | Storage contains all the configurations for storage | *[StorageConfig](#storageconfig) |  | false |
| networking | Networking contains all the configurations for networking | *[NetworkingConfig](#networkingconfig) |  | false |
| workloadSources | WorkloadSources contains all the configurations for workload sources | [WorkloadSourcesConfig](#workloadsourcesconfig) |  | false |
| security | Security contains all the security configurations | [SecurityConfig](#securityconfig) | {"certConfig": {"ca": {"duration": "48h0m0s", "renewBefore": "24h0m0s"}, "server": {"duration": "24h0m0s", "renewBefore": "12h0m0s"}}} | false |
| deployment | Deployment contains all the configurations related to deployment of KubeVirt components | [DeploymentConfig](#deploymentconfig) | {"uninstallStrategy": "BlockUninstallIfWorkloadsExist", "deployVmConsoleProxy": false, "applicationAwareConfig": {"enable": false}} | false |

[Back to TOC](#table-of-contents)

## HyperConvergedStatus

HyperConvergedStatus defines the observed state of HyperConverged

| Field | Description | Scheme | Default | Required |
| ----- | ----------- | ------ | ------- | -------- |
| conditions | Conditions describes the state of the HyperConverged resource. | []metav1.Condition |  | false |
| relatedObjects | RelatedObjects is a list of objects created and maintained by this operator. Object references will be added to this list after they have been created AND found in the cluster. | []corev1.ObjectReference |  | false |
| versions | Versions is a list of HCO component versions, as name/version pairs. The version with a name of \"operator\" is the HCO version itself, as described here: https://github.com/openshift/cluster-version-operator/blob/master/docs/dev/clusteroperator.md#version | [][Version](#version) |  | false |
| observedGeneration | ObservedGeneration reflects the HyperConverged resource generation. If the ObservedGeneration is less than the resource generation in metadata, the status is out of date | int64 |  | false |
| dataImportSchedule | DataImportSchedule is the cron expression that is used in for the hard-coded data import cron templates. HCO generates the value of this field once and stored in the status field, so will survive restart. | string |  | false |
| dataImportCronTemplates | DataImportCronTemplates is a list of the actual DataImportCronTemplates as HCO update in the SSP CR. The list contains both the common and the custom templates, including any modification done by HCO. | [][DataImportCronTemplateStatus](#dataimportcrontemplatestatus) |  | false |
| systemHealthStatus | SystemHealthStatus reflects the health of HCO and its secondary resources, based on the aggregated conditions. | string |  | false |
| infrastructureHighlyAvailable | InfrastructureHighlyAvailable describes whether the cluster has only one worker node (false) or more (true). | *bool |  | false |
| nodeInfo | NodeInfo holds information about the cluster nodes | [NodeInfoStatus](#nodeinfostatus) |  | false |

[Back to TOC](#table-of-contents)

## HyperConvergedWorkloadUpdateStrategy

HyperConvergedWorkloadUpdateStrategy defines options related to updating a KubeVirt install

| Field | Description | Scheme | Default | Required |
| ----- | ----------- | ------ | ------- | -------- |
| workloadUpdateMethods | WorkloadUpdateMethods defines the methods that can be used to disrupt workloads during automated workload updates. When multiple methods are present, the least disruptive method takes precedence over more disruptive methods. For example if both LiveMigrate and Evict methods are listed, only VMs which are not live migratable will be restarted/shutdown. An empty list defaults to no automated workload updating. | []string | {"LiveMigrate"} | true |
| batchEvictionSize | BatchEvictionSize Represents the number of VMIs that can be forced updated per the BatchShutdownInterval interval | *int | 10 | false |
| batchEvictionInterval | BatchEvictionInterval Represents the interval to wait before issuing the next batch of shutdowns | *metav1.Duration | "1m0s" | false |

[Back to TOC](#table-of-contents)

## KubeMacPoolConfig

KubeMacPoolConfig defines kubemacpool MAC address range configuration

| Field | Description | Scheme | Default | Required |
| ----- | ----------- | ------ | ------- | -------- |
| rangeStart | RangeStart defines the first MAC address in the kubemacpool range. The MAC address format should be AA:BB:CC:DD:EE:FF. | *string |  | false |
| rangeEnd | RangeEnd defines the last MAC address in the kubemacpool range. The MAC address format should be AA:BB:CC:DD:EE:FF. | *string |  | false |

[Back to TOC](#table-of-contents)

## LiveMigrationConfigurations

LiveMigrationConfigurations - Live migration limits and timeouts are applied so that migration processes do not overwhelm the cluster.

| Field | Description | Scheme | Default | Required |
| ----- | ----------- | ------ | ------- | -------- |
| parallelMigrationsPerCluster | Number of migrations running in parallel in the cluster. | *uint32 | 5 | false |
| parallelOutboundMigrationsPerNode | Maximum number of outbound migrations per node. | *uint32 | 2 | false |
| bandwidthPerMigration | Bandwidth limit of each migration, the value is quantity of bytes per second (e.g. 2048Mi = 2048MiB/sec) | *string |  | false |
| completionTimeoutPerGiB | If a migrating VM is big and busy, while the connection to the destination node is slow, migration may never converge. The completion timeout is calculated based on completionTimeoutPerGiB times the size of the guest (both RAM and migrated disks, if any). For example, with completionTimeoutPerGiB set to 800, a virtual machine instance with 6GiB memory will timeout if it has not completed migration in 1h20m. Use a lower completionTimeoutPerGiB to induce quicker failure, so that another destination or post-copy is attempted. Use a higher completionTimeoutPerGiB to let workload with spikes in its memory dirty rate to converge. The format is a number. | *int64 | 150 | false |
| progressTimeout | The migration will be canceled if memory copy fails to make progress in this time, in seconds. | *int64 | 150 | false |
| network | The migrations will be performed over a dedicated multus network to minimize disruption to tenant workloads due to network saturation when VM live migrations are triggered. | *string |  | false |
| allowAutoConverge | AllowAutoConverge allows the platform to compromise performance/availability of VMIs to guarantee successful VMI live migrations. Defaults to false | *bool | false | false |
| allowPostCopy | When enabled, KubeVirt attempts to use post-copy live-migration in case it reaches its completion timeout while attempting pre-copy live-migration. Post-copy migrations allow even the busiest VMs to successfully live-migrate. However, events like a network failure or a failure in any of the source or destination nodes can cause the migrated VM to crash or reach inconsistency. Enable this option when evicting nodes is more important than keeping VMs alive. Defaults to false. | *bool | false | false |

[Back to TOC](#table-of-contents)

## LogVerbosityConfiguration

LogVerbosityConfiguration configures log verbosity for different components

| Field | Description | Scheme | Default | Required |
| ----- | ----------- | ------ | ------- | -------- |
| kubevirt | Kubevirt is a struct that allows specifying the log verbosity level that controls the amount of information logged for each Kubevirt component. | *v1.LogVerbosity |  | false |
| cdi | CDI indicates the log verbosity level that controls the amount of information logged for CDI components. | *int32 |  | false |

[Back to TOC](#table-of-contents)

## MediatedDevicesConfiguration

MediatedDevicesConfiguration holds information about MDEV types to be defined, if available

| Field | Description | Scheme | Default | Required |
| ----- | ----------- | ------ | ------- | -------- |
| mediatedDeviceTypes |  | []string |  | true |
| nodeMediatedDeviceTypes |  | [][NodeMediatedDeviceTypesConfig](#nodemediateddevicetypesconfig) |  | false |

[Back to TOC](#table-of-contents)

## MediatedHostDevice

MediatedHostDevice represents a host mediated device allowed for passthrough

| Field | Description | Scheme | Default | Required |
| ----- | ----------- | ------ | ------- | -------- |
| mdevNameSelector | name of a mediated device type required to identify a mediated device on a host | string |  | true |
| resourceName | name by which a device is advertised and being requested | string |  | true |
| externalResourceProvider | indicates that this resource is being provided by an external device plugin | bool |  | false |
| disabled | HCO enforces the existence of several MediatedHostDevice objects. Set disabled field to true instead of remove these objects. | bool |  | false |

[Back to TOC](#table-of-contents)

## NetworkingConfig

NetworkingConfig contains all the networking configurations

| Field | Description | Scheme | Default | Required |
| ----- | ----------- | ------ | ------- | -------- |
| kubeSecondaryDNSNameServerIP | KubeSecondaryDNSNameServerIP defines name server IP used by KubeSecondaryDNS | *string |  | false |
| kubeMacPoolConfiguration | KubeMacPoolConfiguration holds kubemacpool MAC address range configuration. | *[KubeMacPoolConfig](#kubemacpoolconfig) |  | false |
| networkBinding | NetworkBinding defines the network binding plugins. Those bindings can be used when defining virtual machine interfaces. | map[string]v1.InterfaceBindingPlugin |  | false |

[Back to TOC](#table-of-contents)

## NodeInfoStatus

NodeInfoStatus holds information about the cluster nodes

| Field | Description | Scheme | Default | Required |
| ----- | ----------- | ------ | ------- | -------- |
| workloadsArchitectures | WorkloadsArchitectures is a distinct list of the CPU architectures of the workloads nodes in the cluster. | []string |  | false |
| controlPlaneArchitectures | ControlPlaneArchitectures is a distinct list of the CPU architecture of the control-plane nodes. | []string |  | false |

[Back to TOC](#table-of-contents)

## NodeMediatedDeviceTypesConfig

NodeMediatedDeviceTypesConfig holds information about MDEV types to be defined in a specific node that matches the NodeSelector field.

| Field | Description | Scheme | Default | Required |
| ----- | ----------- | ------ | ------- | -------- |
| nodeSelector | NodeSelector is a selector which must be true for the vmi to fit on a node. Selector which must match a node's labels for the vmi to be scheduled on that node. More info: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/ | map[string]string |  | true |
| mediatedDeviceTypes |  | []string |  | true |

[Back to TOC](#table-of-contents)

## NodePlacements

NodePlacements defines the node scheduling configuration for infrastructure or workload entities

| Field | Description | Scheme | Default | Required |
| ----- | ----------- | ------ | ------- | -------- |
| infra | Infra describes node scheduling configuration for infrastructure entities | *[sdkapi.NodePlacement](https://github.com/kubevirt/controller-lifecycle-operator-sdk/blob/bbf16167410b7a781c7b08a3f088fc39551c7a00/pkg/sdk/api/types.go#L49) |  | false |
| workload | Workload describes node scheduling configuration for workload entities | *[sdkapi.NodePlacement](https://github.com/kubevirt/controller-lifecycle-operator-sdk/blob/bbf16167410b7a781c7b08a3f088fc39551c7a00/pkg/sdk/api/types.go#L49) |  | false |

[Back to TOC](#table-of-contents)

## PciHostDevice

PciHostDevice represents a host PCI device allowed for passthrough

| Field | Description | Scheme | Default | Required |
| ----- | ----------- | ------ | ------- | -------- |
| pciDeviceSelector | a combination of a vendor_id:product_id required to identify a PCI device on a host. | string |  | true |
| resourceName | name by which a device is advertised and being requested | string |  | true |
| externalResourceProvider | indicates that this resource is being provided by an external device plugin | bool |  | false |
| disabled | HCO enforces the existence of several PciHostDevice objects. Set disabled field to true instead of remove these objects. | bool |  | false |

[Back to TOC](#table-of-contents)

## PermittedHostDevices

PermittedHostDevices holds information about devices allowed for passthrough

| Field | Description | Scheme | Default | Required |
| ----- | ----------- | ------ | ------- | -------- |
| pciHostDevices |  | [][PciHostDevice](#pcihostdevice) |  | false |
| usbHostDevices |  | [][USBHostDevice](#usbhostdevice) |  | false |
| mediatedDevices |  | [][MediatedHostDevice](#mediatedhostdevice) |  | false |

[Back to TOC](#table-of-contents)

## SecurityConfig

SecurityConfig contains all the security configurations

| Field | Description | Scheme | Default | Required |
| ----- | ----------- | ------ | ------- | -------- |
| certConfig | certConfig holds the rotation policy for internal, self-signed certificates | [HyperConvergedCertConfig](#hyperconvergedcertconfig) | {"ca": {"duration": "48h0m0s", "renewBefore": "24h0m0s"}, "server": {"duration": "24h0m0s", "renewBefore": "12h0m0s"}} | false |
| tlsSecurityProfile | TLSSecurityProfile specifies the settings for TLS connections to be propagated to all kubevirt-hyperconverged components. If unset, the hyperconverged cluster operator will consume the value set on the APIServer CR on OCP/OKD or Intermediate if on vanilla k8s. Note that only Old, Intermediate and Custom profiles are currently supported, and the maximum available MinTLSVersions is VersionTLS12. | *openshiftconfigv1.TLSSecurityProfile |  | false |

[Back to TOC](#table-of-contents)

## StorageConfig

StorageConfig contains all the storage configurations

| Field | Description | Scheme | Default | Required |
| ----- | ----------- | ------ | ------- | -------- |
| vmStateStorageClass | VMStateStorageClass is the name of the storage class to use for the PVCs created to preserve VM state, like TPM. | *string |  | false |
| scratchSpaceStorageClass | Override the storage class used for scratch space during transfer operations. The scratch space storage class is determined in the following order: value of scratchSpaceStorageClass, if that doesn't exist, use the default storage class, if there is no default storage class, use the storage class of the DataVolume, if no storage class specified, use no storage class for scratch space | *string |  | false |
| storageImport | StorageImport contains configuration for importing containerized data | *[StorageImportConfig](#storageimportconfig) |  | false |
| filesystemOverhead | FilesystemOverhead describes the space reserved for overhead when using Filesystem volumes. A value is between 0 and 1, if not defined it is 0.055 (5.5 percent overhead) | *cdiv1beta1.FilesystemOverhead |  | false |
| workloadResourceRequirements | WorkloadResourceRequirements defines the resource requirements for storage workloads. It will propagate to the CDI custom resource | *corev1.ResourceRequirements |  | false |

[Back to TOC](#table-of-contents)

## StorageImportConfig

StorageImportConfig contains configuration for importing containerized data

| Field | Description | Scheme | Default | Required |
| ----- | ----------- | ------ | ------- | -------- |
| insecureRegistries | InsecureRegistries is a list of image registries URLs that are not secured. Setting an insecure registry URL in this list allows pulling images from this registry. | []string |  | false |

[Back to TOC](#table-of-contents)

## USBHostDevice

USBHostDevice represents a host USB device allowed for passthrough

| Field | Description | Scheme | Default | Required |
| ----- | ----------- | ------ | ------- | -------- |
| resourceName | Identifies the list of USB host devices. e.g: kubevirt.io/storage, kubevirt.io/bootable-usb, etc | string |  | true |
| selectors |  | [][USBSelector](#usbselector) |  | false |
| externalResourceProvider | If true, KubeVirt will leave the allocation and monitoring to an external device plugin | bool |  | false |
| disabled | HCO enforces the existence of several USBHostDevice objects. Set disabled field to true instead of remove these objects. | bool |  | false |

[Back to TOC](#table-of-contents)

## USBSelector

USBSelector represents a selector for a USB device allowed for passthrough

| Field | Description | Scheme | Default | Required |
| ----- | ----------- | ------ | ------- | -------- |
| vendor |  | string |  | true |
| product |  | string |  | true |

[Back to TOC](#table-of-contents)

## Version



| Field | Description | Scheme | Default | Required |
| ----- | ----------- | ------ | ------- | -------- |
| name |  | string |  | false |
| version |  | string |  | false |

[Back to TOC](#table-of-contents)

## VirtualMachineOptions

VirtualMachineOptions holds the cluster level information regarding the virtual machine.

| Field | Description | Scheme | Default | Required |
| ----- | ----------- | ------ | ------- | -------- |
| disableFreePageReporting | DisableFreePageReporting disable the free page reporting of memory balloon device https://libvirt.org/formatdomain.html#memory-balloon-device. This will have effect only if AutoattachMemBalloon is not false and the vmi is not requesting any high performance feature (dedicatedCPU/realtime/hugePages), in which free page reporting is always disabled. | *bool | false | false |
| disableSerialConsoleLog | DisableSerialConsoleLog disables logging the auto-attached default serial console. If not set, serial console logs will be written to a file and then streamed from a container named `guest-console-log`. The value can be individually overridden for each VM, not relevant if AutoattachSerialConsole is disabled for the VM. | *bool | false | false |
| defaultCPUModel | DefaultCPUModel defines a cluster default for CPU model: default CPU model is set when VMI doesn't have any CPU model. When VMI has CPU model set, then VMI's CPU model is preferred. When default CPU model is not set and VMI's CPU model is not set too, host-model will be set. Default CPU model can be changed when kubevirt is running. | *string |  | false |
| defaultRuntimeClass | DefaultRuntimeClass defines a cluster default for the RuntimeClass to be used for VMIs pods if not set there. Default RuntimeClass can be changed when kubevirt is running, existing VMIs are not impacted till the next restart/live-migration when they are eventually going to consume the new default RuntimeClass. | *string |  | false |

[Back to TOC](#table-of-contents)

## VirtualizationConfig

VirtualizationConfig contains all the virtualization configurations

| Field | Description | Scheme | Default | Required |
| ----- | ----------- | ------ | ------- | -------- |
| tuningPolicy | TuningPolicy allows configuring the mode in which the RateLimits of kubevirt are set. If TuningPolicy is not present the default kubevirt values are used. It can be set to `annotation` for fine-tuning the kubevirt queryPerSeconds (QPS) and burst values. QPS and burst values are taken from the annotation hco.kubevirt.io/tuningPolicy | HyperConvergedTuningPolicy |  | false |
| liveMigrationConfig | Live migration limits and timeouts are applied so that migration processes do not overwhelm the cluster. | [LiveMigrationConfigurations](#livemigrationconfigurations) | {"completionTimeoutPerGiB": 150, "parallelMigrationsPerCluster": 5, "parallelOutboundMigrationsPerNode": 2, "progressTimeout": 150, "allowAutoConverge": false, "allowPostCopy": false} | false |
| permittedHostDevices | PermittedHostDevices holds information about devices allowed for passthrough | *[PermittedHostDevices](#permittedhostdevices) |  | false |
| mediatedDevicesConfiguration | MediatedDevicesConfiguration holds information about MDEV types to be defined on nodes, if available | *[MediatedDevicesConfiguration](#mediateddevicesconfiguration) |  | false |
| workloadUpdateStrategy | WorkloadUpdateStrategy defines at the cluster level how to handle automated workload updates | [HyperConvergedWorkloadUpdateStrategy](#hyperconvergedworkloadupdatestrategy) | {"workloadUpdateMethods": {"LiveMigrate"}, "batchEvictionSize": 10, "batchEvictionInterval": "1m0s"} | false |
| obsoleteCPUModels | ObsoleteCPUModels is a list of obsolete CPU models. When the node-labeller obtains the list of obsolete CPU models, it eliminates those CPU models and creates labels for valid CPU models. The default values for this field is nil, however, HCO uses opinionated values, and adding values to this list will add them to the opinionated values. | []string |  | false |
| evictionStrategy | EvictionStrategy defines at the cluster level if the VirtualMachineInstance should be migrated instead of shut-off in case of a node drain. If the VirtualMachineInstance specific field is set it overrides the cluster level one. Allowed values: - `None` no eviction strategy at cluster level. - `LiveMigrate` migrate the VM on eviction; a not live migratable VM with no specific strategy will block the drain of the node util manually evicted. - `LiveMigrateIfPossible` migrate the VM on eviction if live migration is possible, otherwise directly evict. - `External` block the drain, track eviction and notify an external controller. Defaults to LiveMigrate with multiple worker nodes, None on single worker clusters. | *v1.EvictionStrategy |  | false |
| virtualMachineOptions | VirtualMachineOptions holds the cluster level information regarding the virtual machine. | *[VirtualMachineOptions](#virtualmachineoptions) | {"disableFreePageReporting": false, "disableSerialConsoleLog": false} | false |
| higherWorkloadDensity | HigherWorkloadDensity holds configuration aimed to increase virtual machine density | *[HigherWorkloadDensityConfiguration](#higherworkloaddensityconfiguration) | {"memoryOvercommitPercentage": 100} | false |
| liveUpdateConfiguration | LiveUpdateConfiguration holds the cluster configuration for live update of virtual machines - max cpu sockets, max guest memory and max hotplug ratio. This setting can affect VM CPU and memory settings. | *v1.LiveUpdateConfiguration |  | false |
| ksmConfiguration | KSMConfiguration holds the information regarding the enabling the KSM in the nodes (if available). | *v1.KSMConfiguration |  | false |
| hypervisors | Hypervisors specifies which hypervisor the cluster uses to run virtual machines. If empty or not set, KubeVirt defaults to KVM. Currently, only a single entry is supported. Allowed values for the hypervisor name are \"kvm\" and \"hyperv-direct\". | []v1.HypervisorConfiguration |  | false |
| roleAggregationStrategy | RoleAggregationStrategy controls whether KubeVirt RBAC cluster roles should be aggregated to the default Kubernetes roles (admin, edit, view). When set to \"AggregateToDefault\" or not specified, the aggregate-to-* labels are added to the cluster roles. When set to \"Manual\", the labels are not added, and roles will not be aggregated to the default roles. | *v1.RoleAggregationStrategy |  | false |
| vmiCPUAllocationRatio | VmiCPUAllocationRatio defines, for each requested virtual CPU, how much physical CPU to request per VMI from the hosting node. The value is in fraction of a CPU thread (or core on non-hyperthreaded nodes). VMI POD CPU request = number of vCPUs * 1/vmiCPUAllocationRatio For example, a value of 1 means 1 physical CPU thread per VMI CPU thread. A value of 100 would be 1% of a physical thread allocated for each requested VMI thread. This option has no effect on VMIs that request dedicated CPUs. Defaults to 10 | *int | 10 | false |
| autoCPULimitNamespaceLabelSelector | When set, AutoCPULimitNamespaceLabelSelector will set a CPU limit on virt-launcher for VMIs running inside namespaces that match the label selector. The CPU limit will equal the number of requested vCPUs. This setting does not apply to VMIs with dedicated CPUs. | *[metav1.LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#labelselector-v1-meta) |  | false |

[Back to TOC](#table-of-contents)

## WorkloadSourcesConfig

WorkloadSourcesConfig contains all the configurations related to workloads resources

| Field | Description | Scheme | Default | Required |
| ----- | ----------- | ------ | ------- | -------- |
| commonTemplatesNamespace | CommonTemplatesNamespace defines namespace in which common templates will be deployed. It overrides the default openshift namespace. | *string |  | false |
| commonBootImageNamespace | CommonBootImageNamespace override the default namespace of the common boot images, in order to hide them.\n\nIf not set, HCO won't set any namespace, letting SSP to use the default. If set, use the namespace to create the DataImportCronTemplates and the common image streams, with this namespace. This field is not set by default. | *string |  | false |
| enableCommonBootImageImport | Opt-in to automatic delivery/updates of the common data import cron templates. There are two sources for the data import cron templates: hard coded list of common templates, and custom (user defined) templates that can be added to the dataImportCronTemplates field. This field only controls the common templates. It is possible to use custom templates by adding them to the dataImportCronTemplates field. | *bool | true | false |
| dataImportCronTemplates | DataImportCronTemplates holds list of data import cron templates (golden images) | [][DataImportCronTemplate](#dataimportcrontemplate) |  | false |
| instancetypeConfig | InstancetypeConfig holds the configuration of instance type related functionality within KubeVirt. | *v1.InstancetypeConfiguration |  | false |
| commonInstancetypesDeployment | CommonInstancetypesDeployment holds the configuration of common-instancetypes deployment within KubeVirt. | *v1.CommonInstancetypesDeployment |  | false |

[Back to TOC](#table-of-contents)
## HCO Feature Gates
FeatureGates is a set of optional feature gates to enable or disable new
features that are not generally available yet. Add a new FeatureGate Object
to this set, to enable a feature that is disabled by default, or to disable a
feature that is enabled by default.

A feature gate may be in the following phases:
* `alpha`: the feature is in dev-preview. It is disabled by default, but can be
  enabled
* `beta`: the feature gate is in tech-preview. It is enabled by default, but
  can be disabled.
* `GA`: the feature is graduated and is always enabled. There is no way to
  disable it.
* `deprecated`: the feature is no longer supported. There is no way to enable it

| Name | Description | Phase |
| ---- | ----------- | ----- |
| decentralizedLiveMigration | DecentralizedLiveMigration enables the decentralized live migration (cross-cluster migration) feature. This feature allows live migration of VirtualMachineInstances between different clusters. This feature is in Developer Preview. | beta |
| videoConfig | VideoConfig allows users to configure video device types for their virtual machines. This can be useful for workloads that require specific video capabilities or architectures. Note: This feature is in Tech Preview. | beta |
| alignCPUs | Enable KubeVirt to request up to two additional dedicated CPUs in order to complete the total CPU count to an even parity when using emulator thread isolation. Note: this feature is in Developer Preview. | alpha |
| containerPathVolumes | ContainerPathVolumes enables the use of container paths as volumes in KubeVirt. This allows VMs to access files and directories from the virt-launcher pod's filesystem via virtiofs. | alpha |
| declarativeHotplugVolumes | DeclarativeHotplugVolumes enables the use of the declarative volume hotplug feature in KubeVirt. When set to true, the "DeclarativeHotplugVolumes" feature gate is enabled instead of "HotplugVolumes". When set to false or nil, the "HotplugVolumes" feature gate is enabled (default behavior). This feature is in Developer Preview. | alpha |
| deployKubeSecondaryDNS | Deploy KubeSecondaryDNS by CNAO | alpha |
| downwardMetrics | Allow to expose a limited set of host metrics to guests. | alpha |
| enableMultiArchBootImageImport | EnableMultiArchBootImageImport allows the HCO to run on heterogeneous clusters with different CPU architectures. Setting this field to true will allow the HCO to create Golden Images for different CPU architectures. This feature is in Developer Preview. | alpha |
| incrementalBackup | IncrementalBackup enables changed block tracking backups and incremental backups using QEMU capabilities in KubeVirt. When enabled, this also enables the UtilityVolumes feature gate in the KubeVirt CR. Note: This feature is in Tech Preview. | alpha |
| objectGraph | ObjectGraph enables the ObjectGraph VM and VMI subresource in KubeVirt. This subresource returns a structured list of k8s objects that are related to the specified VM or VMI, enabling better dependency tracking. Note: This feature is in Developer Preview. | alpha |
| persistentReservation | Enable persistent reservation of a LUN through the SCSI Persistent Reserve commands on Kubevirt. In order to issue privileged SCSI ioctls, the VM requires activation of the persistent reservation flag. Once this feature gate is enabled, then the additional container with the qemu-pr-helper is deployed inside the virt-handler pod. Enabling (or removing) the feature gate causes the redeployment of the virt-handler pod. | alpha |
| disableMDevConfiguration | Deprecated: This feature gate will be removed in a future release. Use spec.mediatedDevicesConfiguration.enabled instead. | deprecated |

[Back to TOC](#table-of-contents)

