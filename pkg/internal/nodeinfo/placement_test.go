package nodeinfo

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("InjectKubeVirtControlPlanePlacement", func() {
	It("should do nothing when podSpec is nil", func() {
		Expect(func() { InjectKubeVirtControlPlanePlacement(nil) }).ToNot(Panic())
	})

	It("should add the kubevirt control-plane term and toleration", func() {
		podSpec := &corev1.PodSpec{
			Affinity: &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{{
							MatchExpressions: []corev1.NodeSelectorRequirement{{
								Key:      LabelNodeRoleControlPlane,
								Operator: corev1.NodeSelectorOpExists,
							}},
						}},
					},
				},
			},
			Tolerations: []corev1.Toleration{{
				Key:      LabelNodeRoleControlPlane,
				Operator: corev1.TolerationOpExists,
				Effect:   corev1.TaintEffectNoSchedule,
			}},
		}

		InjectKubeVirtControlPlanePlacement(podSpec)

		required := podSpec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution
		Expect(required.NodeSelectorTerms).To(HaveLen(2))
		Expect(required.NodeSelectorTerms[1].MatchExpressions[0].Key).To(Equal(LabelNodeRoleKubevirtControlPlane))
		Expect(required.NodeSelectorTerms[1].MatchExpressions[0].Operator).To(Equal(corev1.NodeSelectorOpExists))

		Expect(podSpec.Tolerations).To(HaveLen(2))
		Expect(podSpec.Tolerations[1].Key).To(Equal(LabelNodeRoleKubevirtControlPlane))
		Expect(podSpec.Tolerations[1].Operator).To(Equal(corev1.TolerationOpExists))
		Expect(podSpec.Tolerations[1].Effect).To(Equal(corev1.TaintEffectNoSchedule))
	})

	It("should be idempotent", func() {
		podSpec := &corev1.PodSpec{}
		InjectKubeVirtControlPlanePlacement(podSpec)
		InjectKubeVirtControlPlanePlacement(podSpec)

		required := podSpec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution
		Expect(required.NodeSelectorTerms).To(HaveLen(1))
		Expect(podSpec.Tolerations).To(HaveLen(1))
	})
})
