package netresinjector

import (
	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
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
)

func shouldDeploy(hc *hcov1.HyperConverged) bool {
	return common.ShouldDeployNetworkResourcesInjector(hc)
}
