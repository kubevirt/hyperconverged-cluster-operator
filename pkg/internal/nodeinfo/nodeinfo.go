package nodeinfo

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func HandleNodeChanges(ctx context.Context, cl client.Client) error {
	nodes, err := getNodes(ctx, cl)
	if err != nil {
		return err
	}

	setHighAvailabilityMode(nodes)

	return nil
}

func getNodes(ctx context.Context, cl client.Client) ([]corev1.Node, error) {
	nodesList := &corev1.NodeList{}
	err := cl.List(ctx, nodesList)
	if err != nil {
		return nil, err
	}

	return nodesList.Items, nil
}
