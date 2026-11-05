package observabilitycontroller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

var _ = Describe("Observability Controller NetworkPolicy", func() {
	Context("newNetworkPolicy", func() {
		It("should have all default values", func() {
			np := newNetworkPolicy(commontestutils.NewHco())

			Expect(np.Name).To(Equal(npName))
			Expect(np.Namespace).To(Equal(hcoutil.GetOperatorNamespaceFromEnv()))
			Expect(np.Labels).To(HaveKeyWithValue(hcoutil.AppLabel, hcoutil.HyperConvergedName))
			Expect(np.Labels).To(HaveKeyWithValue(hcoutil.AppLabelComponent, string(hcoutil.AppComponentObservability)))

			Expect(np.Spec.PodSelector.MatchLabels).To(Equal(map[string]string{
				hcoutil.AppLabel:          hcoutil.HyperConvergedName,
				hcoutil.AppLabelComponent: string(hcoutil.AppComponentObservability),
			}))

			Expect(np.Spec.PolicyTypes).To(ConsistOf(
				networkingv1.PolicyTypeEgress,
				networkingv1.PolicyTypeIngress,
			))

			// DNS egress + API server egress. virt-handler egress is added only
			// when observeVMStat is enabled.
			Expect(np.Spec.Egress).To(HaveLen(2))
			Expect(np.Spec.Egress).ToNot(ContainElement(vmstatsEgressRule()))

			// metrics ingress, open to any source
			Expect(np.Spec.Ingress).To(HaveLen(1))
			metricsRule := np.Spec.Ingress[0]
			Expect(metricsRule.From).To(BeEmpty())
			Expect(metricsRule.Ports).To(ConsistOf(
				networkingv1.NetworkPolicyPort{
					Protocol: new(corev1.ProtocolTCP),
					Port:     &intstr.IntOrString{Type: intstr.Int, IntVal: 8443},
				},
			))
		})

		It("should allow egress to virt-handler only when observeVMStat is enabled", func() {
			hco := commontestutils.NewHco()
			hco.Spec.FeatureGates.Enable(observeVMStatFeatureGate)

			np := newNetworkPolicy(hco)

			Expect(np.Spec.Egress).To(HaveLen(3))
			Expect(np.Spec.Egress).To(ContainElement(vmstatsEgressRule()))
		})
	})

	Context("NetworkPolicy spec drift", func() {
		It("should restore the Spec if it is modified", func() {
			hco := commontestutils.NewHco()
			hco.Spec.FeatureGates.Enable(deployObsCntrlFGName)
			req := commontestutils.NewReq(hco)

			expected := newNetworkPolicy(hco)
			modifiedNP := newNetworkPolicy(hco)
			modifiedNP.Spec.Ingress = nil
			modifiedNP.Spec.PodSelector.MatchLabels = map[string]string{"tampered": "true"}
			cl := commontestutils.InitClient([]client.Object{hco, modifiedNP})

			handler := NewNetworkPolicyHandler(cl, commontestutils.GetScheme())
			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Updated).To(BeTrue())

			foundNP := &networkingv1.NetworkPolicy{}
			Expect(cl.Get(context.Background(), client.ObjectKey{Name: npName, Namespace: hco.Namespace}, foundNP)).To(Succeed())
			Expect(foundNP.Spec).To(Equal(expected.Spec))
		})

		It("should drop virt-handler egress when observeVMStat is disabled", func() {
			hco := commontestutils.NewHco()
			hco.Spec.FeatureGates.Enable(deployObsCntrlFGName)
			hco.Spec.FeatureGates.Enable(observeVMStatFeatureGate)
			withEgress := newNetworkPolicy(hco)

			hco.Spec.FeatureGates.Disable(observeVMStatFeatureGate)
			req := commontestutils.NewReq(hco)
			cl := commontestutils.InitClient([]client.Object{hco, withEgress})

			handler := NewNetworkPolicyHandler(cl, commontestutils.GetScheme())
			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Updated).To(BeTrue())

			foundNP := &networkingv1.NetworkPolicy{}
			Expect(cl.Get(context.Background(), client.ObjectKey{Name: npName, Namespace: hco.Namespace}, foundNP)).To(Succeed())
			Expect(foundNP.Spec).To(Equal(newNetworkPolicy(hco).Spec))
			Expect(foundNP.Spec.Egress).ToNot(ContainElement(vmstatsEgressRule()))
		})
	})
})

func vmstatsEgressRule() networkingv1.NetworkPolicyEgressRule {
	return networkingv1.NetworkPolicyEgressRule{
		Ports: []networkingv1.NetworkPolicyPort{
			{
				Protocol: new(corev1.ProtocolTCP),
				Port:     &intstr.IntOrString{Type: intstr.Int, IntVal: vmstatsPort},
			},
		},
		To: []networkingv1.NetworkPolicyPeer{
			{
				PodSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						virtHandlerLabelKey: virtHandlerLabelValue,
					},
				},
			},
		},
	}
}
