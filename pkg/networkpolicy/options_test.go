package networkpolicy

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/util/fake/clusterinfo"
)

var _ = Describe("WithPodMatchSelector", func() {
	It("should set the pod selector's match labels", func() {
		np := &networkingv1.NetworkPolicy{}
		selector := map[string]string{"app": "my-app"}

		WithPodMatchSelector(selector)(np)

		Expect(np.Spec.PodSelector.MatchLabels).To(Equal(selector))
	})
})

var _ = Describe("WithIngress", func() {
	It("should append the rule and add the Ingress policy type", func() {
		np := &networkingv1.NetworkPolicy{}
		rule := networkingv1.NetworkPolicyIngressRule{
			Ports: []networkingv1.NetworkPolicyPort{{Port: &intstr.IntOrString{Type: intstr.Int, IntVal: 1234}}},
		}

		WithIngress(rule)(np)

		Expect(np.Spec.Ingress).To(ConsistOf(rule))
		Expect(np.Spec.PolicyTypes).To(ConsistOf(networkingv1.PolicyTypeIngress))
	})

	It("should not duplicate the Ingress policy type when applied more than once", func() {
		np := &networkingv1.NetworkPolicy{}
		rule1 := networkingv1.NetworkPolicyIngressRule{}
		rule2 := networkingv1.NetworkPolicyIngressRule{
			Ports: []networkingv1.NetworkPolicyPort{{Port: &intstr.IntOrString{Type: intstr.Int, IntVal: 80}}},
		}

		WithIngress(rule1)(np)
		WithIngress(rule2)(np)

		Expect(np.Spec.Ingress).To(HaveLen(2))
		Expect(np.Spec.PolicyTypes).To(ConsistOf(networkingv1.PolicyTypeIngress))
	})
})

var _ = Describe("WithEgress", func() {
	It("should append the rule and add the Egress policy type", func() {
		np := &networkingv1.NetworkPolicy{}
		rule := networkingv1.NetworkPolicyEgressRule{
			Ports: []networkingv1.NetworkPolicyPort{{Port: &intstr.IntOrString{Type: intstr.Int, IntVal: 1234}}},
		}

		WithEgress(rule)(np)

		Expect(np.Spec.Egress).To(ConsistOf(rule))
		Expect(np.Spec.PolicyTypes).To(ConsistOf(networkingv1.PolicyTypeEgress))
	})

	It("should not duplicate the Egress policy type when applied more than once", func() {
		np := &networkingv1.NetworkPolicy{}
		rule1 := networkingv1.NetworkPolicyEgressRule{}
		rule2 := networkingv1.NetworkPolicyEgressRule{
			Ports: []networkingv1.NetworkPolicyPort{{Port: &intstr.IntOrString{Type: intstr.Int, IntVal: 80}}},
		}

		WithEgress(rule1)(np)
		WithEgress(rule2)(np)

		Expect(np.Spec.Egress).To(HaveLen(2))
		Expect(np.Spec.PolicyTypes).To(ConsistOf(networkingv1.PolicyTypeEgress))
	})
})

var _ = Describe("WithIngressToTCPPorts", func() {
	It("should build an ingress rule with a TCP port per given port and no From peers", func() {
		np := &networkingv1.NetworkPolicy{}

		WithIngressToTCPPorts(nil, 80, 443)(np)

		Expect(np.Spec.Ingress).To(HaveLen(1))
		rule := np.Spec.Ingress[0]
		Expect(rule.From).To(BeNil())
		Expect(rule.Ports).To(ConsistOf(
			networkingv1.NetworkPolicyPort{Protocol: new(corev1.ProtocolTCP), Port: &intstr.IntOrString{Type: intstr.Int, IntVal: 80}},
			networkingv1.NetworkPolicyPort{Protocol: new(corev1.ProtocolTCP), Port: &intstr.IntOrString{Type: intstr.Int, IntVal: 443}},
		))
		Expect(np.Spec.PolicyTypes).To(ConsistOf(networkingv1.PolicyTypeIngress))
	})

	It("should set the given From peers on the rule", func() {
		np := &networkingv1.NetworkPolicy{}
		from := []networkingv1.NetworkPolicyPeer{{
			PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "client"}},
		}}

		WithIngressToTCPPorts(from, 80)(np)

		Expect(np.Spec.Ingress[0].From).To(Equal(from))
	})
})

var _ = Describe("WithAPIServerEgress", func() {
	It("should build an egress rule allowing TCP to the API server port, to any destination", func() {
		np := &networkingv1.NetworkPolicy{}

		WithAPIServerEgress()(np)

		Expect(np.Spec.Egress).To(ConsistOf(networkingv1.NetworkPolicyEgressRule{
			Ports: []networkingv1.NetworkPolicyPort{
				{Port: &intstr.IntOrString{Type: intstr.Int, IntVal: apiServerPort}, Protocol: new(corev1.ProtocolTCP)},
			},
		}))
		Expect(np.Spec.PolicyTypes).To(ConsistOf(networkingv1.PolicyTypeEgress))
	})
})

var _ = Describe("WithDNSEgress", func() {
	var origGetClusterInfo func() hcoutil.ClusterInfo

	BeforeEach(func() {
		origGetClusterInfo = hcoutil.GetClusterInfo
	})

	AfterEach(func() {
		hcoutil.GetClusterInfo = origGetClusterInfo
	})

	It("should target the k8s kube-dns Service when not running on OpenShift", func() {
		hcoutil.GetClusterInfo = clusterinfo.NewGetClusterInfo(clusterinfo.WithIsOpenshift(false))
		np := &networkingv1.NetworkPolicy{}

		WithDNSEgress()(np)

		Expect(np.Spec.Egress).To(HaveLen(1))
		rule := np.Spec.Egress[0]
		Expect(rule.Ports).To(ConsistOf(
			networkingv1.NetworkPolicyPort{Protocol: new(corev1.ProtocolTCP), Port: new(intstr.FromInt32(53))},
			networkingv1.NetworkPolicyPort{Protocol: new(corev1.ProtocolUDP), Port: new(intstr.FromInt32(53))},
		))
		Expect(rule.To).To(HaveLen(1))
		Expect(rule.To[0].NamespaceSelector.MatchLabels).To(Equal(map[string]string{hcoutil.KubernetesMetadataName: "kube-system"}))
		Expect(rule.To[0].PodSelector.MatchLabels).To(Equal(map[string]string{"k8s-app": "kube-dns"}))
		Expect(np.Spec.PolicyTypes).To(ConsistOf(networkingv1.PolicyTypeEgress))
	})

	It("should target the OpenShift DNS daemonset when running on OpenShift", func() {
		hcoutil.GetClusterInfo = clusterinfo.NewGetClusterInfo(clusterinfo.WithIsOpenshift(true))
		np := &networkingv1.NetworkPolicy{}

		WithDNSEgress()(np)

		Expect(np.Spec.Egress).To(HaveLen(1))
		rule := np.Spec.Egress[0]
		Expect(rule.Ports).To(ConsistOf(
			networkingv1.NetworkPolicyPort{Protocol: new(corev1.ProtocolTCP), Port: new(intstr.FromInt32(5353))},
			networkingv1.NetworkPolicyPort{Protocol: new(corev1.ProtocolUDP), Port: new(intstr.FromInt32(5353))},
		))
		Expect(rule.To).To(HaveLen(1))
		Expect(rule.To[0].NamespaceSelector.MatchLabels).To(Equal(map[string]string{hcoutil.KubernetesMetadataName: "openshift-dns"}))
		Expect(rule.To[0].PodSelector.MatchLabels).To(Equal(map[string]string{"dns.operator.openshift.io/daemonset-dns": "default"}))
	})
})
