package v1beta1

import (
	"fmt"
	"maps"
	"reflect"
	"slices"

	conversion2 "k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	kubevirtv1 "kubevirt.io/api/core/v1"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
)

// Implement the conversion.Convertible interface, to be used in the conversion webhook.

func (src *HyperConverged) ConvertTo(dstRaw conversion.Hub) error { //revive:disable:receiver-naming
	dst := dstRaw.(*hcov1.HyperConverged)

	if err := Convert_v1beta1_HyperConverged_To_v1_HyperConverged(src, dst, nil); err != nil {
		return fmt.Errorf("failed to convert HyperConverged from v1beta1 to v1; %w", err)
	}

	convert_v1beta1_FeatureGates_To_v1(&src.Spec.FeatureGates, &dst.Spec.FeatureGates)

	convertNodePlacementsV1beta1ToV1(src.Spec, &dst.Spec)

	if err := convertVirtualizationV1beta1ToV1(src.Spec, &dst.Spec.Virtualization); err != nil {
		return fmt.Errorf("failed to convert HyperConverged's spec.virtualization from v1beta1 to v1; %w", err)
	}

	dst.Spec.Storage = convertStorageV1beta1ToV1(src.Spec)

	convertSecurityV1beta1ToV1(src.Spec, &dst.Spec.Security)

	dst.Spec.Networking = convertNetworkingV1beta1ToV1(src.Spec)

	convertWorkloadSourcesV1beta1ToV1(src.Spec, &dst.Spec.WorkloadSources)

	if err := convertDeploymentV1beta1ToV1(src.Spec, &dst.Spec.Deployment); err != nil {
		return fmt.Errorf("failed to convert HyperConverged's spec.deployment from v1beta1 to v1; %w", err)
	}

	return nil
}

func (dst *HyperConverged) ConvertFrom(srcRaw conversion.Hub) error { //revive:disable:receiver-naming
	src := srcRaw.(*hcov1.HyperConverged)

	if err := Convert_v1_HyperConverged_To_v1beta1_HyperConverged(src, dst, nil); err != nil {
		return fmt.Errorf("failed to convert HyperConverged from v1 to v1beta1; %w", err)
	}

	convert_v1_FeatureGates_To_v1beta1(src.Spec.FeatureGates, &dst.Spec.FeatureGates)

	convertNodePlacementsV1ToV1beta1(src.Spec, &dst.Spec)

	if err := convertVirtualizationV1ToV1beta1(src.Spec.Virtualization, &dst.Spec); err != nil {
		return fmt.Errorf("failed to convert HyperConverged's spec.virtualization from v1 to v1beta1; %w", err)
	}

	convertStorageV1ToV1beta1(src.Spec.Storage, &dst.Spec)

	convertSecurityV1ToV1beta1(src.Spec.Security, &dst.Spec)

	convertNetworkingV1ToV1beta1(src.Spec.Networking, &dst.Spec)

	convertWorkloadSourcesV1ToV1beta1(src.Spec.WorkloadSources, &dst.Spec)

	if err := convertDeploymentV1ToV1beta1(src.Spec.Deployment, &dst.Spec); err != nil {
		return fmt.Errorf("failed to convert HyperConverged's spec.deployment from v1 to v1beta1; %w", err)
	}

	return nil
}

func convertNodePlacementsV1ToV1beta1(v1Spec hcov1.HyperConvergedSpec, v1beta1Spec *HyperConvergedSpec) {
	if v1Spec.NodePlacements == nil {
		return
	}

	if v1Spec.NodePlacements.Infra != nil {
		v1beta1Spec.Infra.NodePlacement = v1Spec.NodePlacements.Infra.DeepCopy()
	}

	if v1Spec.NodePlacements.Workload != nil {
		v1beta1Spec.Workloads.NodePlacement = v1Spec.NodePlacements.Workload.DeepCopy()
	}
}

func convertNodePlacementsV1beta1ToV1(v1beta1Spec HyperConvergedSpec, v1Spec *hcov1.HyperConvergedSpec) {
	if v1beta1Spec.Infra.NodePlacement != nil {
		v1Spec.NodePlacements = &hcov1.NodePlacements{
			Infra: v1beta1Spec.Infra.NodePlacement.DeepCopy(),
		}
	}

	if v1beta1Spec.Workloads.NodePlacement != nil {
		if v1Spec.NodePlacements == nil {
			v1Spec.NodePlacements = &hcov1.NodePlacements{}
		}

		v1Spec.NodePlacements.Workload = v1beta1Spec.Workloads.NodePlacement.DeepCopy()
	}
}

func convertVirtualizationV1ToV1beta1(v1VirtConfig hcov1.VirtualizationConfig, v1beta1Spec *HyperConvergedSpec) error {
	v1VirtConfig.LiveMigrationConfig.DeepCopyInto(&v1beta1Spec.LiveMigrationConfig)

	if v1VirtConfig.PermittedHostDevices != nil {
		v1beta1Spec.PermittedHostDevices = v1VirtConfig.PermittedHostDevices.DeepCopy()
	}

	if v1VirtConfig.MediatedDevicesConfiguration != nil {
		v1beta1Spec.MediatedDevicesConfiguration = &MediatedDevicesConfiguration{}
		if err := converter.Convert(v1VirtConfig.MediatedDevicesConfiguration, v1beta1Spec.MediatedDevicesConfiguration, converter.DefaultMeta(reflect.TypeOf(&MediatedDevicesConfiguration{}))); err != nil {
			return err
		}
	}

	v1VirtConfig.WorkloadUpdateStrategy.DeepCopyInto(&v1beta1Spec.WorkloadUpdateStrategy)

	if len(v1VirtConfig.ObsoleteCPUModels) > 0 {
		v1beta1Spec.ObsoleteCPUs = &HyperConvergedObsoleteCPUs{
			CPUModels: slices.Clone(v1VirtConfig.ObsoleteCPUModels),
		}
	}

	v1beta1Spec.EvictionStrategy = setPtr(v1VirtConfig.EvictionStrategy)

	convertVMOptionsV1ToV1beta1(v1VirtConfig.VirtualMachineOptions, v1beta1Spec)

	if v1VirtConfig.HigherWorkloadDensity != nil {
		v1beta1Spec.HigherWorkloadDensity = v1VirtConfig.HigherWorkloadDensity.DeepCopy()
	}

	if v1VirtConfig.LiveUpdateConfiguration != nil {
		v1beta1Spec.LiveUpdateConfiguration = v1VirtConfig.LiveUpdateConfiguration.DeepCopy()
	}

	if v1VirtConfig.KSMConfiguration != nil {
		v1beta1Spec.KSMConfiguration = v1VirtConfig.KSMConfiguration.DeepCopy()
	}

	if len(v1VirtConfig.Hypervisors) > 0 {
		v1beta1Spec.Hypervisors = make([]kubevirtv1.HypervisorConfiguration, len(v1VirtConfig.Hypervisors))
		for i := range v1VirtConfig.Hypervisors {
			v1VirtConfig.Hypervisors[i].DeepCopyInto(&v1beta1Spec.Hypervisors[i])
		}
	}

	v1beta1Spec.RoleAggregationStrategy = setPtr(v1VirtConfig.RoleAggregationStrategy)

	convertResourceRequirementsV1ToV1beta1(v1VirtConfig, v1beta1Spec)

	return nil
}

func convertResourceRequirementsV1ToV1beta1(v1VirtConfig hcov1.VirtualizationConfig, v1beta1Spec *HyperConvergedSpec) {
	if v1VirtConfig.VmiCPUAllocationRatio == nil &&
		v1VirtConfig.AutoCPULimitNamespaceLabelSelector == nil {
		return
	}

	if v1beta1Spec.ResourceRequirements == nil {
		v1beta1Spec.ResourceRequirements = &OperandResourceRequirements{}
	}

	v1beta1Spec.ResourceRequirements.VmiCPUAllocationRatio = setPtr(v1VirtConfig.VmiCPUAllocationRatio)

	if v1VirtConfig.AutoCPULimitNamespaceLabelSelector != nil {
		v1beta1Spec.ResourceRequirements.AutoCPULimitNamespaceLabelSelector = v1VirtConfig.AutoCPULimitNamespaceLabelSelector.DeepCopy()
	}
}

func convertVMOptionsV1ToV1beta1(v1VMOptions *hcov1.VirtualMachineOptions, v1beta1Spec *HyperConvergedSpec) {
	if v1VMOptions == nil {
		return
	}
	// fields under v1beta1.VirtualMachineOptions
	v1beta1Spec.VirtualMachineOptions = &VirtualMachineOptions{
		DisableFreePageReporting: setPtr(v1VMOptions.DisableFreePageReporting),
		DisableSerialConsoleLog:  setPtr(v1VMOptions.DisableSerialConsoleLog),
	}
	// fields under v1beta1.Spec
	v1beta1Spec.DefaultRuntimeClass = setPtr(v1VMOptions.DefaultRuntimeClass)
	v1beta1Spec.DefaultCPUModel = setPtr(v1VMOptions.DefaultCPUModel)
}

func convertVirtualizationV1beta1ToV1(v1beta1Spec HyperConvergedSpec, v1VirtConfig *hcov1.VirtualizationConfig) error {
	v1beta1Spec.LiveMigrationConfig.DeepCopyInto(&v1VirtConfig.LiveMigrationConfig)

	if v1beta1Spec.PermittedHostDevices != nil {
		v1VirtConfig.PermittedHostDevices = v1beta1Spec.PermittedHostDevices.DeepCopy()
	}

	if v1beta1Spec.MediatedDevicesConfiguration != nil {
		v1VirtConfig.MediatedDevicesConfiguration = &hcov1.MediatedDevicesConfiguration{}
		if err := converter.Convert(v1beta1Spec.MediatedDevicesConfiguration, v1VirtConfig.MediatedDevicesConfiguration, converter.DefaultMeta(reflect.TypeOf(&hcov1.MediatedDevicesConfiguration{}))); err != nil {
			return err
		}
	}

	v1beta1Spec.WorkloadUpdateStrategy.DeepCopyInto(&v1VirtConfig.WorkloadUpdateStrategy)

	if v1beta1Spec.ObsoleteCPUs != nil && len(v1beta1Spec.ObsoleteCPUs.CPUModels) > 0 {
		v1VirtConfig.ObsoleteCPUModels = slices.Clone(v1beta1Spec.ObsoleteCPUs.CPUModels)
	}

	v1VirtConfig.EvictionStrategy = setPtr(v1beta1Spec.EvictionStrategy)

	convertVMOptionsV1beta1ToV1(v1beta1Spec, v1VirtConfig)

	if v1beta1Spec.HigherWorkloadDensity != nil {
		v1VirtConfig.HigherWorkloadDensity = v1beta1Spec.HigherWorkloadDensity.DeepCopy()
	}

	if v1beta1Spec.LiveUpdateConfiguration != nil {
		v1VirtConfig.LiveUpdateConfiguration = v1beta1Spec.LiveUpdateConfiguration.DeepCopy()
	}

	if v1beta1Spec.KSMConfiguration != nil {
		v1VirtConfig.KSMConfiguration = v1beta1Spec.KSMConfiguration.DeepCopy()
	}

	if len(v1beta1Spec.Hypervisors) > 0 {
		v1VirtConfig.Hypervisors = make([]kubevirtv1.HypervisorConfiguration, len(v1beta1Spec.Hypervisors))
		for i := range v1beta1Spec.Hypervisors {
			v1beta1Spec.Hypervisors[i].DeepCopyInto(&v1VirtConfig.Hypervisors[i])
		}
	}

	v1VirtConfig.RoleAggregationStrategy = setPtr(v1beta1Spec.RoleAggregationStrategy)

	convertResourceRequirementsV1beta1ToV1(v1beta1Spec.ResourceRequirements, v1VirtConfig)

	return nil
}

func convertResourceRequirementsV1beta1ToV1(v1beta1Reqs *OperandResourceRequirements, v1VirtConfig *hcov1.VirtualizationConfig) {
	if v1beta1Reqs == nil {
		return
	}

	v1VirtConfig.VmiCPUAllocationRatio = setPtr(v1beta1Reqs.VmiCPUAllocationRatio)

	if v1beta1Reqs.AutoCPULimitNamespaceLabelSelector != nil {
		v1VirtConfig.AutoCPULimitNamespaceLabelSelector = v1beta1Reqs.AutoCPULimitNamespaceLabelSelector.DeepCopy()
	}
}

func convertVMOptionsV1beta1ToV1(v1beta1Spec HyperConvergedSpec, v1VirtConfig *hcov1.VirtualizationConfig) {
	if v1beta1Spec.VirtualMachineOptions == nil && v1beta1Spec.DefaultCPUModel == nil && v1beta1Spec.DefaultRuntimeClass == nil {
		return
	}

	v1VirtConfig.VirtualMachineOptions = &hcov1.VirtualMachineOptions{
		DefaultCPUModel:     setPtr(v1beta1Spec.DefaultCPUModel),
		DefaultRuntimeClass: setPtr(v1beta1Spec.DefaultRuntimeClass),
	}

	if v1beta1Spec.VirtualMachineOptions != nil {
		v1VirtConfig.VirtualMachineOptions.DisableFreePageReporting = setPtr(v1beta1Spec.VirtualMachineOptions.DisableFreePageReporting)
		v1VirtConfig.VirtualMachineOptions.DisableSerialConsoleLog = setPtr(v1beta1Spec.VirtualMachineOptions.DisableSerialConsoleLog)
	}
}

func convertStorageV1ToV1beta1(v1StorageConfig *hcov1.StorageConfig, v1beta1Spec *HyperConvergedSpec) {
	if v1StorageConfig == nil {
		return
	}

	v1beta1Spec.VMStateStorageClass = setPtr(v1StorageConfig.VMStateStorageClass)
	v1beta1Spec.ScratchSpaceStorageClass = setPtr(v1StorageConfig.ScratchSpaceStorageClass)

	if v1StorageConfig.StorageImport != nil {
		v1beta1Spec.StorageImport = v1StorageConfig.StorageImport.DeepCopy()
	}
	if v1StorageConfig.FilesystemOverhead != nil {
		v1beta1Spec.FilesystemOverhead = v1StorageConfig.FilesystemOverhead.DeepCopy()
	}

	if v1StorageConfig.StorageWorkloads != nil {
		if v1beta1Spec.ResourceRequirements == nil {
			v1beta1Spec.ResourceRequirements = &OperandResourceRequirements{}
		}

		if v1StorageConfig.StorageWorkloads != nil {
			v1beta1Spec.ResourceRequirements.StorageWorkloads = v1StorageConfig.StorageWorkloads.DeepCopy()
		}
	}
}

func convertStorageV1beta1ToV1(v1beta1Spec HyperConvergedSpec) *hcov1.StorageConfig {

	if areV1beta1StorageFieldsEmpty(v1beta1Spec) {
		return nil
	}

	v1StorageConfig := &hcov1.StorageConfig{}

	v1StorageConfig.VMStateStorageClass = setPtr(v1beta1Spec.VMStateStorageClass)
	v1StorageConfig.ScratchSpaceStorageClass = setPtr(v1beta1Spec.ScratchSpaceStorageClass)

	if v1beta1Spec.StorageImport != nil {
		v1StorageConfig.StorageImport = v1beta1Spec.StorageImport.DeepCopy()
	}

	if v1beta1Spec.FilesystemOverhead != nil {
		v1StorageConfig.FilesystemOverhead = v1beta1Spec.FilesystemOverhead.DeepCopy()
	}

	if v1beta1Spec.ResourceRequirements != nil && v1beta1Spec.ResourceRequirements.StorageWorkloads != nil {
		v1StorageConfig.StorageWorkloads = v1beta1Spec.ResourceRequirements.StorageWorkloads.DeepCopy()
	}

	return v1StorageConfig
}

func areV1beta1StorageFieldsEmpty(v1beta1Spec HyperConvergedSpec) bool {
	return v1beta1Spec.VMStateStorageClass == nil &&
		v1beta1Spec.ScratchSpaceStorageClass == nil &&
		(v1beta1Spec.StorageImport == nil || len(v1beta1Spec.StorageImport.InsecureRegistries) == 0) &&
		v1beta1Spec.FilesystemOverhead == nil &&
		(v1beta1Spec.ResourceRequirements == nil || v1beta1Spec.ResourceRequirements.StorageWorkloads == nil)
}

func convertSecurityV1ToV1beta1(v1SecurityConfig hcov1.SecurityConfig, v1beta1Spec *HyperConvergedSpec) {
	v1SecurityConfig.CertConfig.DeepCopyInto(&v1beta1Spec.CertConfig)

	if v1SecurityConfig.TLSSecurityProfile != nil {
		v1beta1Spec.TLSSecurityProfile = v1SecurityConfig.TLSSecurityProfile.DeepCopy()
	}
}

func convertSecurityV1beta1ToV1(v1beta1Spec HyperConvergedSpec, v1SecurityConfig *hcov1.SecurityConfig) {
	v1beta1Spec.CertConfig.DeepCopyInto(&v1SecurityConfig.CertConfig)

	if v1beta1Spec.TLSSecurityProfile != nil {
		v1SecurityConfig.TLSSecurityProfile = v1beta1Spec.TLSSecurityProfile.DeepCopy()
	}
}

func convertNetworkingV1ToV1beta1(v1Networking *hcov1.NetworkingConfig, v1beta1Spec *HyperConvergedSpec) {
	if v1Networking == nil {
		return
	}

	v1beta1Spec.NetworkBinding = maps.Clone(v1Networking.NetworkBinding)
	if v1Networking.KubeMacPoolConfiguration != nil {
		v1beta1Spec.KubeMacPoolConfiguration = v1Networking.KubeMacPoolConfiguration.DeepCopy()
	}
	v1beta1Spec.KubeSecondaryDNSNameServerIP = setPtr(v1Networking.KubeSecondaryDNSNameServerIP)
}

func convertNetworkingV1beta1ToV1(v1beta1Spec HyperConvergedSpec) *hcov1.NetworkingConfig {
	if areV1beta1NetworkingFieldsEmpty(v1beta1Spec) {
		return nil
	}

	var kubeMacPoolConfig *hcov1.KubeMacPoolConfig
	if v1beta1Spec.KubeMacPoolConfiguration != nil {
		kubeMacPoolConfig = v1beta1Spec.KubeMacPoolConfiguration.DeepCopy()
	}

	return &hcov1.NetworkingConfig{
		NetworkBinding:               maps.Clone(v1beta1Spec.NetworkBinding),
		KubeMacPoolConfiguration:     kubeMacPoolConfig,
		KubeSecondaryDNSNameServerIP: setPtr(v1beta1Spec.KubeSecondaryDNSNameServerIP),
	}
}

func areV1beta1NetworkingFieldsEmpty(v1beta1Spec HyperConvergedSpec) bool {
	return v1beta1Spec.NetworkBinding == nil &&
		v1beta1Spec.KubeMacPoolConfiguration == nil &&
		v1beta1Spec.KubeSecondaryDNSNameServerIP == nil
}

func convertWorkloadSourcesV1ToV1beta1(v1Config hcov1.WorkloadSourcesConfig, v1beta1Spec *HyperConvergedSpec) {
	v1beta1Spec.CommonTemplatesNamespace = setPtr(v1Config.CommonTemplatesNamespace)
	v1beta1Spec.CommonBootImageNamespace = setPtr(v1Config.CommonBootImageNamespace)
	v1beta1Spec.EnableCommonBootImageImport = setPtr(v1Config.EnableCommonBootImageImport)

	if len(v1Config.DataImportCronTemplates) > 0 {
		v1beta1Spec.DataImportCronTemplates = make([]hcov1.DataImportCronTemplate, len(v1Config.DataImportCronTemplates))

		for i := range v1Config.DataImportCronTemplates {
			v1Config.DataImportCronTemplates[i].DeepCopyInto(&v1beta1Spec.DataImportCronTemplates[i])
		}
	}

	if v1Config.InstancetypeConfig != nil {
		v1beta1Spec.InstancetypeConfig = v1Config.InstancetypeConfig.DeepCopy()
	}

	if v1Config.CommonInstancetypesDeployment != nil {
		v1beta1Spec.CommonInstancetypesDeployment = v1Config.CommonInstancetypesDeployment.DeepCopy()
	}
}

func convertWorkloadSourcesV1beta1ToV1(v1beta1Spec HyperConvergedSpec, v1Config *hcov1.WorkloadSourcesConfig) {
	v1Config.CommonTemplatesNamespace = setPtr(v1beta1Spec.CommonTemplatesNamespace)
	v1Config.CommonBootImageNamespace = setPtr(v1beta1Spec.CommonBootImageNamespace)
	v1Config.EnableCommonBootImageImport = setPtr(v1beta1Spec.EnableCommonBootImageImport)

	if len(v1beta1Spec.DataImportCronTemplates) > 0 {
		v1Config.DataImportCronTemplates = make([]hcov1.DataImportCronTemplate, len(v1beta1Spec.DataImportCronTemplates))
		for i := range v1beta1Spec.DataImportCronTemplates {
			v1beta1Spec.DataImportCronTemplates[i].DeepCopyInto(&v1Config.DataImportCronTemplates[i])
		}
	}

	if v1beta1Spec.InstancetypeConfig != nil {
		v1Config.InstancetypeConfig = v1beta1Spec.InstancetypeConfig.DeepCopy()
	}

	if v1beta1Spec.CommonInstancetypesDeployment != nil {
		v1Config.CommonInstancetypesDeployment = v1beta1Spec.CommonInstancetypesDeployment.DeepCopy()
	}
}

func convertDeploymentV1ToV1beta1(v1Config hcov1.DeploymentConfig, v1beta1Spec *HyperConvergedSpec) error {
	if err := convertAAQConfigV1ToV1beta1(v1Config, v1beta1Spec); err != nil {
		return err
	}

	v1beta1Spec.UninstallStrategy = v1Config.UninstallStrategy

	if v1Config.LogVerbosityConfig != nil {
		v1beta1Spec.LogVerbosityConfig = v1Config.LogVerbosityConfig.DeepCopy()
	}

	v1beta1Spec.DeployVMConsoleProxy = setPtr(v1Config.DeployVMConsoleProxy)

	return nil
}

func convertAAQConfigV1ToV1beta1(v1Config hcov1.DeploymentConfig, v1beta1Spec *HyperConvergedSpec) error {
	if v1Config.ApplicationAwareConfig == nil {
		return nil
	}

	v1beta1Spec.ApplicationAwareConfig = &ApplicationAwareConfigurations{}
	if err := converter.Convert(v1Config.ApplicationAwareConfig, v1beta1Spec.ApplicationAwareConfig, nil); err != nil {
		return err
	}

	v1beta1Spec.EnableApplicationAwareQuota = setPtr(v1Config.ApplicationAwareConfig.Enable)

	return nil
}

func convertDeploymentV1beta1ToV1(v1beta1Spec HyperConvergedSpec, v1Config *hcov1.DeploymentConfig) error {
	if err := convertAAQConfigV1beta1ToV1(v1beta1Spec, v1Config); err != nil {
		return err
	}

	v1Config.UninstallStrategy = v1beta1Spec.UninstallStrategy

	if v1beta1Spec.LogVerbosityConfig != nil {
		v1Config.LogVerbosityConfig = v1beta1Spec.LogVerbosityConfig.DeepCopy()
	}

	v1Config.DeployVMConsoleProxy = setPtr(v1beta1Spec.DeployVMConsoleProxy)

	return nil
}

func convertAAQConfigV1beta1ToV1(v1beta1Spec HyperConvergedSpec, v1Config *hcov1.DeploymentConfig) error {
	if v1beta1Spec.ApplicationAwareConfig == nil && v1beta1Spec.EnableApplicationAwareQuota == nil {
		return nil
	}

	v1Config.ApplicationAwareConfig = &hcov1.ApplicationAwareConfigurations{}
	if v1beta1Spec.ApplicationAwareConfig != nil {
		if err := converter.Convert(v1beta1Spec.ApplicationAwareConfig, v1Config.ApplicationAwareConfig, nil); err != nil {
			return err
		}
	}

	v1Config.ApplicationAwareConfig.Enable = setPtr(v1beta1Spec.EnableApplicationAwareQuota)

	return nil
}

var converter *conversion2.Converter

func init() {
	var conversionScheme = runtime.NewScheme()
	if err := AddToScheme(conversionScheme); err != nil {
		panic(err)
	}

	if err := hcov1.AddToScheme(conversionScheme); err != nil {
		panic(err)
	}

	if err := RegisterConversions(conversionScheme); err != nil {
		panic(err)
	}

	converter = conversionScheme.Converter()
	if converter == nil {
		panic("unable to register HyperConvergedScheme with runtime.Scheme")
	}
}

func setPtr[T comparable](orig *T) *T {
	if orig == nil {
		return nil
	}

	dst := new(T)
	*dst = *orig
	return dst
}
