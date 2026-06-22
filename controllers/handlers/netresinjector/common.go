package netresinjector

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
