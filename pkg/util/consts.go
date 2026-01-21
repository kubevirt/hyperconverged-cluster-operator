package util

import (
	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
)

// pod names
const (
	HCOOperatorName  = "hyperconverged-cluster-operator"
	HCOWebhookName   = "hyperconverged-cluster-webhook"
	CLIDownloadsName = "hyperconverged-cluster-cli-download"
)

// HCO common constants
const (
	OperatorNamespaceEnv         = "OPERATOR_NAMESPACE"
	OperatorWebhookModeEnv       = "WEBHOOK_MODE"
	ContainerAppName             = "APP"
	ContainerOperatorApp         = "OPERATOR"
	ContainerWebhookApp          = "WEBHOOK"
	HcoKvIoVersionName           = "HCO_KV_IO_VERSION"
	KubevirtVersionEnvV          = "KUBEVIRT_VERSION"
	KvVirtLauncherOSVersionEnvV  = "VIRT_LAUNCHER_OS_VERSION"
	CdiVersionEnvV               = "CDI_VERSION"
	CnaoVersionEnvV              = "NETWORK_ADDONS_VERSION"
	SspVersionEnvV               = "SSP_VERSION"
	HppoVersionEnvV              = "HPPO_VERSION"
	AaqVersionEnvV               = "AAQ_VERSION"
	MigrationOperatorVersionEnvV = "MIGRATION_OPERATOR_VERSION"
	KVUIPluginImageEnvV          = "KV_CONSOLE_PLUGIN_IMAGE"
	KVUIProxyImageEnvV           = "KV_CONSOLE_PROXY_IMAGE"
	PasstImageEnvV               = "PASST_SIDECAR_IMAGE"
	PasstCNIImageEnvV            = "PASST_CNI_IMAGE"
	WaspAgentImageEnvV           = "WASP_AGENT_IMAGE"
	DeployNetworkPoliciesEnvV    = "DEPLOY_NETWORK_POLICIES"
)

const (
	HcoValidatingWebhook                    = "validate-hco.kubevirt.io"
	HcoV1Beta1ValidatingWebhook             = "validate-hco-v1beta1.kubevirt.io"
	HcoMutatingWebhookNS                    = "mutate-ns-hco.kubevirt.io"
	HcoMutatingWebhookHyperConverged        = "mutate-hyperconverged-hco.kubevirt.io"
	HcoV1Beta1MutatingWebhookHyperConverged = "mutate-hyperconverged-hco-v1beta1.kubevirt.io"

	HCOWebhookPath                = "/validate-hco-kubevirt-io-v1-hyperconverged"
	HCOWebhookV1Beta1Path         = "/validate-hco-kubevirt-io-v1beta1-hyperconverged"
	HCOMutatingWebhookPath        = "/mutate-hco-kubevirt-io-v1-hyperconverged"
	HCOV1Beta1MutatingWebhookPath = "/mutate-hco-kubevirt-io-v1beta1-hyperconverged"
	HCONSWebhookPath              = "/mutate-ns-hco-kubevirt-io"

	WebhookPort     = 4343
	WebhookPortName = "webhook"

	WebhookCertName       = "apiserver.crt"
	WebhookKeyName        = "apiserver.key"
	DefaultWebhookCertDir = "/apiserver.local.config/certificates"
)

const (
	PrometheusRuleCRDName              = "prometheusrules.monitoring.coreos.com"
	ServiceMonitorCRDName              = "servicemonitors.monitoring.coreos.com"
	DeschedulerCRDName                 = "kubedeschedulers.operator.openshift.io"
	PersesDashboardsCRDName            = "persesdashboards.perses.dev"
	PersesDatasourcesCRDName           = "persesdatasources.perses.dev"
	NetworkAttachmentDefinitionCRDName = "network-attachment-definitions.k8s.cni.cncf.io"
)

const (
	AppLabel           = "app"
	UndefinedNamespace = ""
	OpenshiftNamespace = "openshift"
	AppLabelPrefix     = "app.kubernetes.io"
	AppLabelVersion    = AppLabelPrefix + "/version"
	AppLabelManagedBy  = AppLabelPrefix + "/managed-by"
	AppLabelPartOf     = AppLabelPrefix + "/part-of"
	AppLabelComponent  = AppLabelPrefix + "/component"
)

const (
	APIVersionBeta     = hcov1beta1.APIVersionBeta
	APIVersionV1       = hcov1.APIVersionV1
	CurrentAPIVersion  = hcov1beta1.CurrentAPIVersion
	APIVersionGroup    = hcov1.APIVersionGroup
	APIVersion         = hcov1.APIVersion
	HyperConvergedKind = "HyperConverged"
	// Recommended labels by Kubernetes. See
	// https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/
	// Operator name for managed-by label
	OperatorName = "hco-operator"
	// Value for "part-of" label
	HyperConvergedCluster    = "hyperconverged-cluster"
	OpenshiftNodeSelectorAnn = "openshift.io/node-selector"
	KubernetesMetadataName   = "kubernetes.io/metadata.name"

	// PrometheusNSLabel is the monitoring NS enable label, if the value is "true"
	PrometheusNSLabel = "openshift.io/cluster-monitoring"

	// HyperConvergedName is the name of the HyperConverged resource that will be reconciled
	HyperConvergedName = "kubevirt-hyperconverged"
)

const (
	MetricsHost                 = "0.0.0.0"
	MetricsPort           int32 = 8443
	MetricsPortName             = "metrics"
	HealthProbeHost             = "0.0.0.0"
	HealthProbePort       int32 = 6060
	ReadinessEndpointName       = "/readyz"
	LivenessEndpointName        = "/livez"

	CliDownloadsServerPort int32 = 8080
	UIPluginServerPort     int32 = 9443
	UIProxyServerPort      int32 = 8080

	APIServerCRName      = "cluster"
	DeschedulerCRName    = "cluster"
	DeschedulerNamespace = "openshift-kube-descheduler-operator"

	DataImportCronEnabledAnnotation = "dataimportcrontemplate.kubevirt.io/enable"

	HCOAnnotationPrefix = "hco.kubevirt.io/"
	NPLabelPrefix       = "np.kubevirt.io/"

	// AllowEgressToDNSAndAPIServerLabel if this label is set, the network policy will allow egress to DNS and API server
	AllowEgressToDNSAndAPIServerLabel = NPLabelPrefix + "allow-access-cluster-services"
	// AllowIngressToMetricsEndpointLabel if this label is set, the network policy will allow ingress to the metrics endpoint
	AllowIngressToMetricsEndpointLabel = NPLabelPrefix + "allow-prometheus-access"
)

type AppComponent string

const (
	AppComponentCompute    AppComponent = "compute"
	AppComponentStorage    AppComponent = "storage"
	AppComponentNetwork    AppComponent = "network"
	AppComponentMonitoring AppComponent = "monitoring"
	AppComponentSchedule   AppComponent = "schedule"
	AppComponentDeployment AppComponent = "deployment"
	AppComponentUIPlugin   AppComponent = "kubevirt-console-plugin"
	AppComponentUIProxy    AppComponent = "kubevirt-apiserver-proxy"
	AppComponentUIConfig   AppComponent = "kubevirt-ui-config"
	AppComponentQuotaMngt  AppComponent = "quota-management"
	AppComponentMigration  AppComponent = "migration"
)
