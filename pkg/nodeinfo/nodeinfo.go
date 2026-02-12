package nodeinfo

import internal "github.com/kubevirt/hyperconverged-cluster-operator/pkg/internal/nodeinfo"

const (
	S390X = internal.S390X

	// LabelNodeRoleControlPlane is the label used to identify control plane nodes
	LabelNodeRoleControlPlane = internal.LabelNodeRoleControlPlane
	// LabelNodeRoleMaster is the old label used to identify control plane nodes
	LabelNodeRoleMaster = internal.LabelNodeRoleMaster
	// LabelNodeRoleWorker is the label used to identify worker nodes
	LabelNodeRoleWorker = internal.LabelNodeRoleWorker
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
