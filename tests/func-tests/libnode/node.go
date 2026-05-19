package libnode

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const WorkerNodeLabel = "node-role.kubernetes.io/worker"

func IsSingleWorkerCluster(ctx context.Context, cli client.Client) (bool, error) {
	workerNodes, err := ListWorkerNodes(ctx, cli)
	if err != nil {
		return false, err
	}
	return len(workerNodes) == 1, nil
}

func ListWorkerNodes(ctx context.Context, cli client.Client) ([]corev1.Node, error) {
	nodeList := &corev1.NodeList{}
	if err := cli.List(ctx, nodeList, client.MatchingLabels{WorkerNodeLabel: ""}); err != nil {
		return nil, err
	}
	return nodeList.Items, nil
}
