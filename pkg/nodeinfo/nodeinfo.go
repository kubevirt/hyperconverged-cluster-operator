package nodeinfo

import internal "github.com/kubevirt/hyperconverged-cluster-operator/pkg/internal/nodeinfo"

var (
	HandleNodeChanges               = internal.HandleNodeChanges
	IsControlPlaneHighlyAvailable   = internal.IsControlPlaneHighlyAvailable
	IsControlPlaneNodeExists        = internal.IsControlPlaneNodeExists
	IsInfrastructureHighlyAvailable = internal.IsInfrastructureHighlyAvailable
)
