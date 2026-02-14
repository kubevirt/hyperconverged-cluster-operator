package nodeinfo

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
)

func HandleNodeChanges(ctx context.Context, cl client.Client, hc *v1beta1.HyperConverged, logger logr.Logger) (bool, error) {
	logger.Info("reading cluster nodes")
	nodes, err := getNodes(ctx, cl)
	if err != nil {
		return false, fmt.Errorf("failed to read the cluster nodes; %v", err)
	}

	return processNodeInfo(nodes, hc), nil
}

func getNodes(ctx context.Context, cl client.Client) ([]corev1.Node, error) {
	nodesList := &corev1.NodeList{}
	err := cl.List(ctx, nodesList)
	if err != nil {
		return nil, err
	}

	return nodesList.Items, nil
}

func processNodeInfo(nodes []corev1.Node, hc *v1beta1.HyperConverged) bool {
	workerNodeCount := 0
	cpNodeCount := 0
	arbiterNodeCount := 0

	workloadArchs := sets.New[string]()
	cpArchs := sets.New[string]()

	isWorkloadNode := isWorkloadNodeFunc(hc)

	for _, node := range nodes {
		arch := node.Status.NodeInfo.Architecture
		if isWorkerNode(node) {
			workerNodeCount++
		}

		if isWorkloadNode(node) {
			workloadArchs.Insert(arch)
		}

		_, masterLabelExists := node.Labels[LabelNodeRoleMaster]
		_, cpLabelExists := node.Labels[LabelNodeRoleControlPlane]
		if masterLabelExists || cpLabelExists {
			cpNodeCount++
			cpArchs.Insert(arch)
		}

		if _, arbiterLabelExists := node.Labels[LabelNodeRoleArbiter]; arbiterLabelExists {
			arbiterNodeCount++
		}
	}

	// remove empty architectures
	workloadArchs.Delete("")
	cpArchs.Delete("")

	newValue := cpNodeCount >= 3 || (cpNodeCount >= 2 && arbiterNodeCount >= 1)
	changed := controlPlaneHighlyAvailable.Swap(newValue) != newValue

	newValue = cpNodeCount >= 1
	changed = controlPlaneNodeExist.Swap(newValue) != newValue || changed

	newValue = workerNodeCount >= 2
	changed = infrastructureHighlyAvailable.Swap(newValue) != newValue || changed

	changed = workloadArchitectures.set(workloadArchs) || changed
	changed = controlPlaneArchitectures.set(cpArchs) || changed

	changed = processCpuModels(nodes) || changed

	return changed
}

func isWorkerNode(node corev1.Node) bool {
	_, exists := node.Labels[LabelNodeRoleWorker]
	return exists
}

func isWorkloadNodeFunc(hc *v1beta1.HyperConverged) func(corev1.Node) bool {
	if hasWorkloadRequirements(hc) {

		workloadMatcher := getWorkloadMatcher(hc)

		return func(node corev1.Node) bool {
			matches, err := workloadMatcher.Match(&node)
			if err != nil { // should not happen, because the validation webhook checks it, but just in case
				return false
			}
			return matches
		}
	}

	return isWorkerNode
}
