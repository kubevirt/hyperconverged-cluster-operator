package networkpolicy

import (
	"slices"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

const ( // for network policies
	k8sDNSNamespaceSelector = "kube-system"
	k8sDNSPodSelectorLabel  = "k8s-app"
	k8sDNSPodSelectorVal    = "kube-dns"
	k8sDNSPort              = int32(53)

	openshiftDNSNamespaceSelector = "openshift-dns"
	openshiftDNSPodSelectorLabel  = "dns.operator.openshift.io/daemonset-dns"
	openshiftDNSPodSelectorVal    = "default"
	openshiftDNSPort              = int32(5353)

	apiServerPort int32 = 6443
)

type Option func(policy *networkingv1.NetworkPolicy)

func WithEgress(rule networkingv1.NetworkPolicyEgressRule) Option {
	return func(policy *networkingv1.NetworkPolicy) {
		policy.Spec.Egress = append(policy.Spec.Egress, rule)
		if !slices.Contains(policy.Spec.PolicyTypes, networkingv1.PolicyTypeEgress) {
			policy.Spec.PolicyTypes = append(policy.Spec.PolicyTypes, networkingv1.PolicyTypeEgress)
		}
	}
}

func WithIngress(rule networkingv1.NetworkPolicyIngressRule) Option {
	return func(policy *networkingv1.NetworkPolicy) {
		policy.Spec.Ingress = append(policy.Spec.Ingress, rule)
		if !slices.Contains(policy.Spec.PolicyTypes, networkingv1.PolicyTypeIngress) {
			policy.Spec.PolicyTypes = append(policy.Spec.PolicyTypes, networkingv1.PolicyTypeIngress)
		}
	}
}

func WithDNSEgress() Option {
	var (
		dnsNamespcaeSelector = k8sDNSNamespaceSelector
		dnsPodSelectorLabel  = k8sDNSPodSelectorLabel
		dnsPodSelectorVal    = k8sDNSPodSelectorVal
		dnsPort              = k8sDNSPort
	)

	if hcoutil.GetClusterInfo().IsOpenshift() {
		dnsNamespcaeSelector = openshiftDNSNamespaceSelector
		dnsPodSelectorLabel = openshiftDNSPodSelectorLabel
		dnsPodSelectorVal = openshiftDNSPodSelectorVal
		dnsPort = openshiftDNSPort
	}

	rule := networkingv1.NetworkPolicyEgressRule{
		Ports: []networkingv1.NetworkPolicyPort{
			{
				Protocol: new(corev1.ProtocolTCP),
				Port:     new(intstr.FromInt32(dnsPort)),
			},
			{
				Protocol: new(corev1.ProtocolUDP),
				Port:     new(intstr.FromInt32(dnsPort)),
			},
		},
		To: []networkingv1.NetworkPolicyPeer{
			{
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						hcoutil.KubernetesMetadataName: dnsNamespcaeSelector,
					},
				},
				PodSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						dnsPodSelectorLabel: dnsPodSelectorVal,
					},
				},
			},
		},
	}

	return WithEgress(rule)
}

func WithAPIServerEgress() Option {
	rule := networkingv1.NetworkPolicyEgressRule{
		Ports: []networkingv1.NetworkPolicyPort{
			{
				Port:     &intstr.IntOrString{Type: intstr.Int, IntVal: apiServerPort},
				Protocol: new(corev1.ProtocolTCP),
			},
		},
	}

	return WithEgress(rule)
}

func WithPodMatchSelector(selector map[string]string) Option {
	return func(policy *networkingv1.NetworkPolicy) {
		policy.Spec.PodSelector.MatchLabels = selector
	}
}

func WithIngressToTCPPorts(from []networkingv1.NetworkPolicyPeer, ports ...int32) Option {
	rule := networkingv1.NetworkPolicyIngressRule{}
	for _, port := range ports {
		rule.Ports = append(rule.Ports, networkingv1.NetworkPolicyPort{
			Protocol: new(corev1.ProtocolTCP),
			Port:     &intstr.IntOrString{Type: intstr.Int, IntVal: port},
		})
	}

	rule.From = from

	return WithIngress(rule)
}
