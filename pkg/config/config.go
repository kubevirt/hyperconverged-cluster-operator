package config

import (
	networkaddons "github.com/kubevirt/cluster-network-addons-operator/pkg/apis/networkaddonsoperator/v1alpha1"
	hcov1alpha1 "github.com/kubevirt/hyperconverged-cluster-operator/pkg/apis/hco/v1alpha1"
	cdi "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
	kubevirt "kubevirt.io/kubevirt/pkg/api/v1"

	v1 "k8s.io/api/core/v1"
)

var (
	DefaultVersion           = "2.0"
	DefaultContainerRegistry = "quay.io/kubevirt"
	DefaultImagePullPolicy   = v1.PullIfNotPresent
)

// NewHCOSpec - returns an HCOSpec object populated by default variables
func NewHCOSpec(HcoVersion string, HcoContainerRegistry string, HcoImagePullPolicy v1.PullPolicy) *hcov1alpha1.HyperConvergedSpec {
	if HcoVersion == "" {
		HcoVersion = DefaultVersion
	}
	if HcoContainerRegistry == "" {
		HcoContainerRegistry = DefaultContainerRegistry
	}
	if string(HcoImagePullPolicy) == "" {
		HcoImagePullPolicy = DefaultImagePullPolicy
	}

	return &hcov1alpha1.HyperConvergedSpec{
		Version:           HcoVersion,
		ContainerRegistry: HcoContainerRegistry,
		ImagePullPolicy:   HcoImagePullPolicy,
	}
}

// NewKubeVirtSpec - returns a KubeVirtSpec object populated by HCO variables
func NewKubeVirtSpec(hcoSpec *hcov1alpha1.HyperConvergedSpec) kubevirt.KubeVirtSpec {
	return kubevirt.KubeVirtSpec{
		ImageTag:        hcoSpec.Version,
		ImageRegistry:   hcoSpec.ContainerRegistry,
		ImagePullPolicy: hcoSpec.ImagePullPolicy,
	}
}

// NewCDISpec - returns a CDISpec object populated by HCO variables
func NewCdiSpec(hcoSpec *hcov1alpha1.HyperConvergedSpec) cdi.CDISpec {
	return cdi.CDISpec{
		ImagePullPolicy: hcoSpec.ImagePullPolicy,
	}
}

// NewNetworkAddonsSpec - returns a NetworkAddonsSpec object populated by HCO variables
func NewNetworkAddonsSpec(hcoSpec *hcov1alpha1.HyperConvergedSpec) networkaddons.NetworkAddonsConfigSpec {
	return networkaddons.NetworkAddonsConfigSpec{
		Multus:          &networkaddons.Multus{},
		LinuxBridge:     &networkaddons.LinuxBridge{},
		ImagePullPolicy: string(hcoSpec.ImagePullPolicy),
	}
}
