package netresinjector

import (
	"k8s.io/utils/ptr"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
)

const (
	clusterRoleName    = "virt-network-resources-injector"
	serviceAccountName = "virt-network-resources-injector-sa"
	serviceName        = "virt-network-resources-injector-service"
	deploymentName     = "virt-network-resources-injector"
	tlsSecretName      = "virt-network-resources-injector-secret"
	tlsCertificateName = "virt-network-resources-injector-cert"
	tlsMountPath       = "/etc/tls"
	webhookConfigName  = "virt-network-resources-injector-config"
	serverPort         = 6443
	healthCheckPort    = 8444
)

func shouldDeploy(hc *hcov1.HyperConverged) bool {
	return ptr.Deref(hc.Spec.Deployment.DeployNetworkResourcesInjector, true)
}
