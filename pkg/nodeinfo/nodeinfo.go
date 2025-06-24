package nodeinfo

import internal "github.com/kubevirt/hyperconverged-cluster-operator/pkg/internal/nodeinfo"

const S390X = internal.S390X

var (
	HandleNodeChanges = internal.HandleNodeChanges

	IsControlPlaneHighlyAvailable   = internal.IsControlPlaneHighlyAvailable
	IsControlPlaneNodeExists        = internal.IsControlPlaneNodeExists
	IsInfrastructureHighlyAvailable = internal.IsInfrastructureHighlyAvailable

	GetControlPlaneArchitectures = internal.GetControlPlaneArchitectures
	GetWorkloadsArchitectures    = internal.GetWorkloadsArchitectures
)
