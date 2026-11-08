package observabilitycontroller

import (
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/networkpolicy"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

const (
	npName                = deploymentName + "-np"
	observCtrlMetricsPort = hcoutil.MetricsPort

	virtHandlerLabelKey   = "kubevirt.io"
	virtHandlerLabelValue = "virt-handler"
	vmstatsPort           = int32(8187)
)

func NewNetworkPolicyHandler(cli client.Client, scheme *runtime.Scheme) operands.Operand {
	return operands.NewConditionalHandler(
		operands.NewDynamicNetworkPolicyHandler(cli, scheme, newNetworkPolicy),
		shouldDeploy,
		npNameOnly,
	)
}

func npNameOnly(_ *hcov1.HyperConverged) client.Object {
	return &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      npName,
			Namespace: hcoutil.GetOperatorNamespaceFromEnv(),
		},
	}
}

func newNetworkPolicy(hc *hcov1.HyperConverged) *networkingv1.NetworkPolicy {
	builder := networkpolicy.NewBuilder(npName,
		hcoutil.AppComponentObservability,
		networkpolicy.WithPodMatchSelector(map[string]string{
			hcoutil.AppLabel:          hcoutil.HyperConvergedName,
			hcoutil.AppLabelComponent: string(hcoutil.AppComponentObservability),
		}),
		networkpolicy.WithDNSEgress(),
		networkpolicy.WithAPIServerEgress(),
		networkpolicy.WithIngressToTCPPorts(nil, observCtrlMetricsPort),
	)

	if hc.Spec.FeatureGates.IsEnabled(observeVMStatFeatureGate) {
		// virt-handler is not hostNetwork, so GetVMStats on TCP 8187 is reached
		// with a same-namespace pod egress rule.
		builder.WithOption(networkpolicy.WithEgress(networkingv1.NetworkPolicyEgressRule{
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
		}))
	}

	return builder.Build()
}
