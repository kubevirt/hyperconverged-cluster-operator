package nodeinfo

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"

	"kubevirt.io/controller-lifecycle-operator-sdk/api"

	"github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
)

var _ = Describe("test hasWorkloadRequirements", func() {
	DescribeTable("should select the node according to the HyperConverged CR", func(hc *v1beta1.HyperConverged, matcher types.GomegaMatcher) {
		Expect(hasWorkloadRequirements(hc)).To(matcher)
	},
		Entry("nil NodePlacement", nil, BeFalse()),
		Entry("empty Workloads settings", &v1beta1.HyperConverged{Spec: v1beta1.HyperConvergedSpec{Workloads: v1beta1.HyperConvergedConfig{}}}, BeFalse()),
		Entry("empty NodePlacement", &v1beta1.HyperConverged{Spec: v1beta1.HyperConvergedSpec{Workloads: v1beta1.HyperConvergedConfig{
			NodePlacement: &api.NodePlacement{},
		}}}, BeFalse()),
		Entry("empty NodeSelector", &v1beta1.HyperConverged{Spec: v1beta1.HyperConvergedSpec{Workloads: v1beta1.HyperConvergedConfig{
			NodePlacement: &api.NodePlacement{
				NodeSelector: map[string]string{},
			},
		}}}, BeFalse()),
		Entry("non-empty NodeSelector", &v1beta1.HyperConverged{Spec: v1beta1.HyperConvergedSpec{Workloads: v1beta1.HyperConvergedConfig{
			NodePlacement: &api.NodePlacement{
				NodeSelector: map[string]string{
					"label": "value",
				},
			},
		}}}, BeTrue()),
		Entry("empty Affinity", &v1beta1.HyperConverged{Spec: v1beta1.HyperConvergedSpec{Workloads: v1beta1.HyperConvergedConfig{
			NodePlacement: &api.NodePlacement{
				Affinity: &corev1.Affinity{},
			},
		}}}, BeFalse()),
		Entry("empty NodeAffinity", &v1beta1.HyperConverged{Spec: v1beta1.HyperConvergedSpec{Workloads: v1beta1.HyperConvergedConfig{
			NodePlacement: &api.NodePlacement{
				Affinity: &corev1.Affinity{
					NodeAffinity: &corev1.NodeAffinity{},
				},
			},
		}}}, BeFalse()),
		Entry("non-empty NodeAffinity", &v1beta1.HyperConverged{Spec: v1beta1.HyperConvergedSpec{Workloads: v1beta1.HyperConvergedConfig{
			NodePlacement: &api.NodePlacement{
				Affinity: &corev1.Affinity{
					NodeAffinity: &corev1.NodeAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{},
					},
				},
			},
		}}}, BeTrue()),
	)
})
