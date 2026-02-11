package nodeinfo

import (
	"sync/atomic"
)

const (
	// LabelNodeRoleControlPlane is the label used to identify control plane nodes
	LabelNodeRoleControlPlane = "node-role.kubernetes.io/control-plane"
	// LabelNodeRoleMaster is the old label used to identify control plane nodes
	LabelNodeRoleMaster = "node-role.kubernetes.io/master"
	// LabelNodeRoleWorker is the label used to identify worker nodes
	LabelNodeRoleWorker = "node-role.kubernetes.io/worker"
	// LabelNodeRoleArbiter is the label used to identify arbiter nodes
	LabelNodeRoleArbiter = "node-role.kubernetes.io/arbiter"
)

var (
	controlPlaneHighlyAvailable   atomic.Bool
	controlPlaneNodeExist         atomic.Bool
	infrastructureHighlyAvailable atomic.Bool
)

func IsControlPlaneHighlyAvailable() bool {
	return controlPlaneHighlyAvailable.Load()
}

func IsControlPlaneNodeExists() bool {
	return controlPlaneNodeExist.Load()
}

func IsInfrastructureHighlyAvailable() bool {
	return infrastructureHighlyAvailable.Load()
}
