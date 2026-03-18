package v1beta1

import (
	"fmt"
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
