package virtimport

import (
	appsv1 "k8s.io/api/apps/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/networkpolicy"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

// newNetworkPolicy allows the controller DNS and API server egress. It serves nothing, metrics
// are off, so it allows no ingress.
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

func selectorLabels() map[string]string {
	return map[string]string{
		hcoutil.AppLabel:          hcoutil.HyperConvergedName,
		hcoutil.AppLabelComponent: string(appComponent),
	}
}
