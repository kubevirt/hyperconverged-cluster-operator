package nodeinfo

import internal "github.com/kubevirt/hyperconverged-cluster-operator/pkg/internal/nodeinfo"

const (
	// LabelNodeRoleArbiter is the label used to identify arbiter nodes
	LabelNodeRoleArbiter = internal.LabelNodeRoleArbiter
)

var (
	HandleNodeChanges = internal.HandleNodeChanges

	IsControlPlaneHighlyAvailable   = internal.IsControlPlaneHighlyAvailable
	IsControlPlaneNodeExists        = internal.IsControlPlaneNodeExists
	IsInfrastructureHighlyAvailable = internal.IsInfrastructureHighlyAvailable

	GetControlPlaneArchitectures = internal.GetControlPlaneArchitectures
	GetWorkloadsArchitectures    = internal.GetWorkloadsArchitectures
)
