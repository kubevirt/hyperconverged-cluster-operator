package nodeinfo

import (
	"sync/atomic"

	corev1 "k8s.io/api/core/v1"
)

const (
	// LabelNodeRoleControlPlane is the label used to identify control plane nodes
	LabelNodeRoleControlPlane = "node-role.kubernetes.io/control-plane"
	// LabelNodeRoleMaster is the old label used to identify control plane nodes
	LabelNodeRoleMaster = "node-role.kubernetes.io/master"
	// LabelNodeRoleWorker is the label used to identify worker nodes
	LabelNodeRoleWorker = "node-role.kubernetes.io/worker"
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

func setHighAvailabilityMode(nodes []corev1.Node) {
	workerNodeCount := 0
	cpNodeCount := 0

	for _, node := range nodes {
		if _, workerLabelExists := node.Labels[LabelNodeRoleWorker]; workerLabelExists {
			workerNodeCount++
		}

		_, masterLabelExists := node.Labels[LabelNodeRoleMaster]
		_, cpLabelExists := node.Labels[LabelNodeRoleControlPlane]
		if masterLabelExists || cpLabelExists {
			cpNodeCount++
		}
	}

	controlPlaneHighlyAvailable.Store(cpNodeCount >= 3)
	controlPlaneNodeExist.Store(cpNodeCount >= 1)
	infrastructureHighlyAvailable.Store(workerNodeCount >= 2)
}
