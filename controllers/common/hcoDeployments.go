package common

import (
	"k8s.io/utils/ptr"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
)

// ShouldDeployNetworkResourcesInjector checks if network-resources-injector should be deployed
func ShouldDeployNetworkResourcesInjector(hc *hcov1.HyperConverged) bool {
	return ptr.Deref(hc.Spec.Deployment.DeployNetworkResourcesInjector, true)
}
