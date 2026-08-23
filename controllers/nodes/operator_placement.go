package nodes

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/nodeinfo"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/ownresources"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

// HandleClassicOperatorPlacementLabeling labels nodes selected by the operator
// Deployment nodeSelector with node-role.kubevirt.io/control-plane so virt-operator
// can schedule there on classic OpenShift clusters.
//
// virt-operator has node affinity for kubernetes control-plane/master nodes.
// OLM Subscription config.nodeSelector cannot replace that affinity, so placing
// operators on infra nodes AND the required affinity together is unsatisfiable
// unless those infra nodes also carry the kubevirt control-plane label (an OR term
// in virt-operator's affinity, owned by the KubeVirt repo).
//
// The selector is read from HCO's own Deployment (OLM name hco-operator, not the
// ServiceAccount name hyperconverged-cluster-operator). OLM applies the same
// Subscription config to every CSV operator Deployment. HCO has no required
// control-plane affinity, so it can schedule first. That Deployment is not in
// HCO's restricted cache (app=kubevirt-hyperconverged), so callers should pass
// an API reader for the Get.
func HandleClassicOperatorPlacementLabeling(ctx context.Context, cli client.Client, nodeName string, logger logr.Logger) error {
	return labelClassicOperatorPlacement(ctx, cli, cli, nodeName, logger)
}

func labelClassicOperatorPlacement(ctx context.Context, cli client.Client, reader client.Reader, nodeName string, logger logr.Logger) error {
	node := &corev1.Node{}
	if err := cli.Get(ctx, client.ObjectKey{Name: nodeName}, node); err != nil {
		if errors.IsNotFound(err) {
			logger.V(1).Info("Node not found, skipping operator placement labeling", "node", nodeName)
			return nil
		}
		return fmt.Errorf("failed to get node %s for operator placement labeling: %w", nodeName, err)
	}

	selector, err := getOperatorNodeSelector(ctx, reader)
	if err != nil {
		return err
	}

	return labelNodeForOperatorPlacement(ctx, cli, node, selector, logger)
}

func operatorDeploymentName() string {
	if name := ownresources.GetDeploymentRef().Name; name != "" {
		return name
	}
	// OLM CSV install strategy names this Deployment hco-operator
	// (hcoutil.OperatorName), not the SA/container name HCOOperatorName.
	return hcoutil.OperatorName
}

func getOperatorNodeSelector(ctx context.Context, reader client.Reader) (map[string]string, error) {
	ns := hcoutil.GetOperatorNamespaceFromEnv()
	name := operatorDeploymentName()

	dep := &appsv1.Deployment{}
	err := reader.Get(ctx, client.ObjectKey{Name: name, Namespace: ns}, dep)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get %s deployment: %w", name, err)
	}

	return dep.Spec.Template.Spec.NodeSelector, nil
}

func hasCustomOperatorPlacement(selector map[string]string) bool {
	for key := range selector {
		if key != corev1.LabelOSStable {
			return true
		}
	}
	return false
}

func nodeMatchesSelector(node *corev1.Node, selector map[string]string) bool {
	if len(selector) == 0 {
		return false
	}
	for key, value := range selector {
		current, exists := node.Labels[key]
		if !exists || current != value {
			return false
		}
	}
	return true
}

func isRealKubernetesControlPlane(node *corev1.Node) bool {
	if _, hasMaster := node.Labels[nodeinfo.LabelNodeRoleMaster]; hasMaster {
		return true
	}
	cpVal, hasCP := node.Labels[nodeinfo.LabelNodeRoleControlPlane]
	return hasCP && cpVal != hypershiftLabelValue
}

func shouldReceiveKubevirtCPLabel(node *corev1.Node, selector map[string]string) bool {
	if !hasCustomOperatorPlacement(selector) {
		return false
	}
	if isRealKubernetesControlPlane(node) {
		return false
	}
	return nodeMatchesSelector(node, selector)
}

func labelNodeForOperatorPlacement(ctx context.Context, cli client.Client, node *corev1.Node, selector map[string]string, logger logr.Logger) error {
	wantLabel := shouldReceiveKubevirtCPLabel(node, selector)
	current, hasLabel := node.Labels[nodeinfo.LabelNodeRoleKubevirtControlPlane]
	hasHCOLabel := hasLabel && current == hypershiftLabelValue

	if wantLabel == hasHCOLabel {
		return nil
	}

	patch := client.MergeFrom(node.DeepCopy())
	if node.Labels == nil {
		node.Labels = map[string]string{}
	}

	if wantLabel {
		logger.Info("Adding kubevirt control-plane label for operator node placement",
			"node", node.Name,
			"label", nodeinfo.LabelNodeRoleKubevirtControlPlane,
			"labelValue", hypershiftLabelValue,
		)
		node.Labels[nodeinfo.LabelNodeRoleKubevirtControlPlane] = hypershiftLabelValue
	} else {
		logger.Info("Removing kubevirt control-plane label; node is not selected by operator placement",
			"node", node.Name,
			"label", nodeinfo.LabelNodeRoleKubevirtControlPlane,
		)
		delete(node.Labels, nodeinfo.LabelNodeRoleKubevirtControlPlane)
	}

	if err := cli.Patch(ctx, node, patch); err != nil {
		return fmt.Errorf("failed to patch node %s for operator placement labeling: %w", node.Name, err)
	}

	logger.Info("Successfully patched node labels for operator placement", "node", node.Name)
	return nil
}
