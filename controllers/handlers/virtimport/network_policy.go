package virtimport

import (
	appsv1 "k8s.io/api/apps/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/networkpolicy"
)

// newNetworkPolicy allows the controller DNS and API server egress. The metrics ingress policy
// comes from the bundle.
func newNetworkPolicy(bundleDeployment *appsv1.Deployment) *networkingv1.NetworkPolicy {
	np := networkpolicy.NewBuilder(bundleDeployment.Name+"-np",
		appComponent,
		networkpolicy.WithPodMatchSelector(selectorLabels()),
		networkpolicy.WithDNSEgress(),
		networkpolicy.WithAPIServerEgress(),
	).Build()
	np.TypeMeta = metav1.TypeMeta{APIVersion: networkingv1.SchemeGroupVersion.String(), Kind: "NetworkPolicy"}
	return np
}
