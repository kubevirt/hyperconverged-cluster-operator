package observabilitycontroller

import (
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/networkpolicy"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

const (
	npName                = deploymentName + "-np"
	observCtrlMetricsPort = hcoutil.MetricsPort
)

func NewNetworkPolicyHandler(cli client.Client, scheme *runtime.Scheme) operands.Operand {
	return operands.NewConditionalHandler(newNetworkPolicyHandler(cli, scheme), shouldDeploy, npNameOnly)
}

func npNameOnly(_ *hcov1.HyperConverged) client.Object {
	return &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      npName,
			Namespace: hcoutil.GetOperatorNamespaceFromEnv(),
		},
	}
}

func newNetworkPolicyHandler(cl client.Client, schm *runtime.Scheme) *operands.GenericOperand {
	return operands.NewNetworkPolicyHandler(cl, schm, newNetworkPolicy())
}

func newNetworkPolicy() *networkingv1.NetworkPolicy {
	builder := networkpolicy.NewBuilder(npName,
		hcoutil.AppComponentObservability,
		networkpolicy.WithPodMatchSelector(map[string]string{
			hcoutil.AppLabel:          hcoutil.HyperConvergedName,
			hcoutil.AppLabelComponent: string(hcoutil.AppComponentObservability),
		}),
		networkpolicy.WithDNSEgress(),
		networkpolicy.WithAPIServerEgress(),
		// metrics:
		networkpolicy.WithIngressToTCPPorts(nil, observCtrlMetricsPort),
	)

	return builder.Build()
}
