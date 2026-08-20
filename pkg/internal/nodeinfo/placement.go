package nodeinfo

import (
	corev1 "k8s.io/api/core/v1"
)

// InjectKubeVirtControlPlanePlacement appends an additional required NodeSelectorTerm
// and matching toleration for node-role.kubevirt.io/control-plane.
//
// virt-operator's CSV already requires kubernetes control-plane/master nodes. That
// conflicts with OLM Subscription nodeSelectors that place operators on infra
// nodes, and with Hosted Control Plane clusters that have no kubernetes
// control-plane nodes. The kubevirt-specific label is an OR term, so default
// control-plane scheduling is preserved while HCO can label designated worker
// or infra nodes to make them eligible.
func InjectKubeVirtControlPlanePlacement(podSpec *corev1.PodSpec) {
	if podSpec == nil {
		return
	}

	term := corev1.NodeSelectorTerm{
		MatchExpressions: []corev1.NodeSelectorRequirement{{
			Key:      LabelNodeRoleKubevirtControlPlane,
			Operator: corev1.NodeSelectorOpExists,
		}},
	}

	if podSpec.Affinity == nil {
		podSpec.Affinity = &corev1.Affinity{}
	}
	if podSpec.Affinity.NodeAffinity == nil {
		podSpec.Affinity.NodeAffinity = &corev1.NodeAffinity{}
	}
	required := podSpec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution
	if required == nil {
		required = &corev1.NodeSelector{}
		podSpec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution = required
	}
	if !hasKubeVirtControlPlaneTerm(required.NodeSelectorTerms) {
		required.NodeSelectorTerms = append(required.NodeSelectorTerms, term)
	}

	if !hasKubeVirtControlPlaneToleration(podSpec.Tolerations) {
		podSpec.Tolerations = append(podSpec.Tolerations, corev1.Toleration{
			Key:      LabelNodeRoleKubevirtControlPlane,
			Operator: corev1.TolerationOpExists,
			Effect:   corev1.TaintEffectNoSchedule,
		})
	}
}

func hasKubeVirtControlPlaneTerm(terms []corev1.NodeSelectorTerm) bool {
	for _, term := range terms {
		for _, expr := range term.MatchExpressions {
			if expr.Key == LabelNodeRoleKubevirtControlPlane && expr.Operator == corev1.NodeSelectorOpExists {
				return true
			}
		}
	}
	return false
}

func hasKubeVirtControlPlaneToleration(tolerations []corev1.Toleration) bool {
	for _, toleration := range tolerations {
		if toleration.Key == LabelNodeRoleKubevirtControlPlane &&
			toleration.Operator == corev1.TolerationOpExists &&
			toleration.Effect == corev1.TaintEffectNoSchedule {
			return true
		}
	}
	return false
}
