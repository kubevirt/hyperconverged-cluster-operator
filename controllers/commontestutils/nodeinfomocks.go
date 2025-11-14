package commontestutils

import "github.com/kubevirt/hyperconverged-cluster-operator/pkg/nodeinfo"

var (
	origIsControlPlaneHighlyAvailable = nodeinfo.IsControlPlaneHighlyAvailable
	origIsControlPlaneNodeExists      = nodeinfo.IsControlPlaneNodeExists
	origIsInfraHighlyAvailable        = nodeinfo.IsInfrastructureHighlyAvailable
	origGetControlPlaneArchitectures  = nodeinfo.GetControlPlaneArchitectures
	origGetWorkloadsArchitectures     = nodeinfo.GetWorkloadsArchitectures
)

func ResetNodeInfoMocks() {
	nodeinfo.IsControlPlaneHighlyAvailable = origIsControlPlaneHighlyAvailable
	nodeinfo.IsControlPlaneNodeExists = origIsControlPlaneNodeExists
	nodeinfo.IsInfrastructureHighlyAvailable = origIsInfraHighlyAvailable
	nodeinfo.GetControlPlaneArchitectures = origGetControlPlaneArchitectures
	nodeinfo.GetWorkloadsArchitectures = origGetWorkloadsArchitectures
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

// SpecificArchitectureNodeInfoMocks mocks a specific architecture for the nodes
func SpecificArchitectureNodeInfoMocks(arch string) {
	nodeinfo.GetControlPlaneArchitectures = func() []string {
		return []string{arch}
	}
	nodeinfo.GetWorkloadsArchitectures = func() []string {
		return []string{arch}
	}
}
