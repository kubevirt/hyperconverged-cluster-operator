package networkpolicy

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	networkingv1 "k8s.io/api/networking/v1"

	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

const testComponent = hcoutil.AppComponentUIProxy

var _ = Describe("Builder", func() {
	It("should set name, namespace and labels, with an empty spec, when built with no options", func() {
		np := NewBuilder("my-np", testComponent).Build()

		Expect(np.Name).To(Equal("my-np"))
		Expect(np.Namespace).To(Equal(commontestutils.Namespace))
		Expect(np.Labels).To(Equal(hcoutil.GetLabels(hcoutil.HyperConvergedName, testComponent)))

		Expect(np.Spec.PodSelector.MatchLabels).To(BeEmpty())
		Expect(np.Spec.Ingress).To(BeEmpty())
		Expect(np.Spec.Egress).To(BeEmpty())
		Expect(np.Spec.PolicyTypes).To(BeEmpty())
	})

	It("should apply the given options, in order, to the built NetworkPolicy", func() {
		selector := map[string]string{"app": "my-app"}

		np := NewBuilder("my-np", testComponent,
			WithPodMatchSelector(selector),
			WithIngressToTCPPorts(nil, 8443),
			WithAPIServerEgress(),
		).Build()

		Expect(np.Spec.PodSelector.MatchLabels).To(Equal(selector))
		Expect(np.Spec.Ingress).To(HaveLen(1))
		Expect(np.Spec.Egress).To(HaveLen(1))
		Expect(np.Spec.PolicyTypes).To(ConsistOf(
			networkingv1.PolicyTypeIngress,
			networkingv1.PolicyTypeEgress,
		))
	})

	It("should apply options added after construction via WithOption", func() {
		builder := NewBuilder("my-np", testComponent)
		builder.WithOption(WithPodMatchSelector(map[string]string{"app": "late"}))

		np := builder.Build()

		Expect(np.Spec.PodSelector.MatchLabels).To(Equal(map[string]string{"app": "late"}))
	})
})
