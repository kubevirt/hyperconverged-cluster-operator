package netresinjector

import (
	"k8s.io/utils/ptr"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
)

const (
	clusterRoleName    = "cnv-network-resources-injector"
	serviceAccountName = "cnv-network-resources-injector-sa"
	serviceName        = "cnv-network-resources-injector-service"
	deploymentName     = "cnv-network-resources-injector"
	tlsSecretName      = "cnv-network-resources-injector-secret"
	tlsCertificateName = "cnv-network-resources-injector-cert"
	tlsMountPath       = "/etc/tls"
	webhookConfigName  = "cnv-network-resources-injector-config"
)

// shouldDeploy checks if network-resources-injector should be deployed
func shouldDeploy(hc *hcov1.HyperConverged) bool {
	return ptr.Deref(hc.Spec.Deployment.DeployNetworkResourcesInjector, true)
}
