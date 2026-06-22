package commontestutils

import "github.com/kubevirt/hyperconverged-cluster-operator/pkg/nodeinfo"

var (
	origIsControlPlaneHighlyAvailable = nodeinfo.IsControlPlaneHighlyAvailable
	origIsControlPlaneNodeExists      = nodeinfo.IsControlPlaneNodeExists
	origIsInfraHighlyAvailable        = nodeinfo.IsInfrastructureHighlyAvailable
	origGetControlPlaneArchitectures  = nodeinfo.GetControlPlaneArchitectures
	origGetWorkloadsArchitectures     = nodeinfo.GetWorkloadsArchitectures
	origGetDefaultArchitecture        = nodeinfo.GetDefaultArchitecture
)

func ResetNodeInfoMocks() {
	nodeinfo.IsControlPlaneHighlyAvailable = origIsControlPlaneHighlyAvailable
	nodeinfo.IsControlPlaneNodeExists = origIsControlPlaneNodeExists
	nodeinfo.IsInfrastructureHighlyAvailable = origIsInfraHighlyAvailable
	nodeinfo.GetControlPlaneArchitectures = origGetControlPlaneArchitectures
	nodeinfo.GetWorkloadsArchitectures = origGetWorkloadsArchitectures
	nodeinfo.GetDefaultArchitecture = origGetDefaultArchitecture
}

// HighlyAvailableNodeInfoMocks mocks highly available cluster
func HighlyAvailableNodeInfoMocks() {
	nodeinfo.IsInfrastructureHighlyAvailable = func() bool {
		return true
	}

	nodeinfo.IsControlPlaneNodeExists = func() bool {
		return true
	}

	nodeinfo.IsControlPlaneHighlyAvailable = func() bool {
		return true
	}
}

// SNONodeInfoMock mocks Openshift SNO
func SNONodeInfoMock() {
	nodeinfo.IsInfrastructureHighlyAvailable = func() bool {
		return false
	}

	nodeinfo.IsControlPlaneNodeExists = func() bool {
		return true
	}

	nodeinfo.IsControlPlaneHighlyAvailable = func() bool {
		return false
	}
}

// SRCPHAINodeInfoMock mocks Openshift with SingleReplica ControlPlane and HighAvailable Infrastructure
func SRCPHAINodeInfoMock() {
	nodeinfo.IsInfrastructureHighlyAvailable = func() bool {
		return true
	}

	nodeinfo.IsControlPlaneNodeExists = func() bool {
		return true
	}

	nodeinfo.IsControlPlaneHighlyAvailable = func() bool {
		return false
	}
}

// ControlPlaneArchitecturesMock mocks the architecures for ControlPlane nodes
func ControlPlaneArchitecturesMock(arch ...string) {
	nodeinfo.GetControlPlaneArchitectures = func() []string {
		return arch
	}
}

// WorkloadsArchitecturesMock mocks the architecures for compute nodes
func WorkloadsArchitecturesMock(arch ...string) {
	nodeinfo.GetWorkloadsArchitectures = func() []string {
		return arch
	}
}

// DefaultArchitectureMock mocks the default architecture
func DefaultArchitectureMock(arch string) {
	nodeinfo.GetDefaultArchitecture = func() string {
		return arch
	}
}
