package nodeinfo

import (
	"sync/atomic"
)

const (
	// LabelNodeRoleControlPlane is the label used to identify control plane nodes
	LabelNodeRoleControlPlane = "node-role.kubernetes.io/control-plane"
	// LabelNodeRoleKubevirtControlPlane is the label used by HCO on HCP clusters to allow
	// virt-operator scheduling without mislabeling worker nodes as Kubernetes control plane nodes
	LabelNodeRoleKubevirtControlPlane = "node-role.kubevirt.io/control-plane"
	// LabelNodeRoleMaster is the old label used to identify control plane nodes
	LabelNodeRoleMaster = "node-role.kubernetes.io/master"
	// LabelNodeRoleWorker is the label used to identify worker nodes
	LabelNodeRoleWorker = "node-role.kubernetes.io/worker"
	// LabelNodeRoleArbiter is the label used to identify arbiter nodes
	LabelNodeRoleArbiter = "node-role.kubernetes.io/arbiter"
)

var (
	controlPlaneHighlyAvailable   atomic.Bool
	controlPlaneMultiNode         atomic.Bool
	controlPlaneNodeExist         atomic.Bool
	infrastructureHighlyAvailable atomic.Bool
)

func IsControlPlaneHighlyAvailable() bool {
	return controlPlaneHighlyAvailable.Load()
}

// IsControlPlaneMultiNode reports whether there is more than one control
// plane node to spread replicas across. Unlike IsControlPlaneHighlyAvailable,
// this does not require an arbiter or a third node, so it also covers
// two-node control planes (e.g. Dual Replica / TNF topologies).
func IsControlPlaneMultiNode() bool {
	return controlPlaneMultiNode.Load()
}

func IsControlPlaneNodeExists() bool {
	return controlPlaneNodeExist.Load()
}

func IsInfrastructureHighlyAvailable() bool {
	return infrastructureHighlyAvailable.Load()
}
