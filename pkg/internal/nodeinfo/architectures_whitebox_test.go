package nodeinfo

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"

	"kubevirt.io/controller-lifecycle-operator-sdk/api"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
)

var _ = Describe("test hasWorkloadRequirements", func() {
	DescribeTable("should select the node according to the HyperConverged CR", func(hc *hcov1.HyperConverged, matcher types.GomegaMatcher) {
		Expect(hasWorkloadRequirements(hc)).To(matcher)
	},
		Entry("nil HyperConverged", nil, BeFalse()),
		Entry("nil NodePlacement", &hcov1.HyperConverged{Spec: hcov1.HyperConvergedSpec{Deployment: hcov1.DeploymentConfig{}}}, BeFalse()),
		Entry("empty NodePlacements", &hcov1.HyperConverged{Spec: hcov1.HyperConvergedSpec{Deployment: hcov1.DeploymentConfig{NodePlacements: &hcov1.NodePlacements{}}}}, BeFalse()),
		Entry("empty Workloads settings", &hcov1.HyperConverged{Spec: hcov1.HyperConvergedSpec{Deployment: hcov1.DeploymentConfig{NodePlacements: &hcov1.NodePlacements{Workload: &api.NodePlacement{}}}}}, BeFalse()),
		Entry("empty NodeSelector", &hcov1.HyperConverged{Spec: hcov1.HyperConvergedSpec{Deployment: hcov1.DeploymentConfig{NodePlacements: &hcov1.NodePlacements{Workload: &api.NodePlacement{
			NodeSelector: map[string]string{},
		}}}}}, BeFalse()),
		Entry("not-empty NodeSelector", &hcov1.HyperConverged{Spec: hcov1.HyperConvergedSpec{Deployment: hcov1.DeploymentConfig{NodePlacements: &hcov1.NodePlacements{Workload: &api.NodePlacement{
			NodeSelector: map[string]string{
				"label": "value",
			},
		}}}}}, BeTrue()),
		Entry("empty Affinity", &hcov1.HyperConverged{Spec: hcov1.HyperConvergedSpec{Deployment: hcov1.DeploymentConfig{NodePlacements: &hcov1.NodePlacements{Workload: &api.NodePlacement{
			Affinity: &corev1.Affinity{},
		}}}}}, BeFalse()),
		Entry("empty NodeAffinity", &hcov1.HyperConverged{Spec: hcov1.HyperConvergedSpec{Deployment: hcov1.DeploymentConfig{NodePlacements: &hcov1.NodePlacements{Workload: &api.NodePlacement{
			Affinity: &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{},
			},
		}}}}}, BeFalse()),
		Entry("non-empty NodeAffinity", &hcov1.HyperConverged{Spec: hcov1.HyperConvergedSpec{Deployment: hcov1.DeploymentConfig{NodePlacements: &hcov1.NodePlacements{Workload: &api.NodePlacement{
			Affinity: &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{},
				},
			},
		}}}}}, BeTrue()),
	)
})
