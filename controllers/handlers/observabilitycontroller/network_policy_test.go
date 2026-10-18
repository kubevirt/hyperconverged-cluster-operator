package observabilitycontroller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

var _ = Describe("Observability Controller NetworkPolicy", func() {
	Context("newNetworkPolicy", func() {
		It("should have all default values", func() {
			np := newNetworkPolicy()

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

			// DNS egress + API server egress
			Expect(np.Spec.Egress).To(HaveLen(2))

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
	})

	Context("NetworkPolicy spec drift", func() {
		It("should restore the Spec if it is modified", func() {
			hco := commontestutils.NewHco()
			hco.Spec.FeatureGates.Enable(featureGateName)
			req := commontestutils.NewReq(hco)

			expected := newNetworkPolicy()
			modifiedNP := newNetworkPolicy()
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
	})
})
