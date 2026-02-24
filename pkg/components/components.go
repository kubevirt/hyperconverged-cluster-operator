package components

import (
	persesv1alpha1 "github.com/rhobs/perses-operator/api/v1alpha1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	cnaoapi "github.com/kubevirt/cluster-network-addons-operator/pkg/apis/networkaddonsoperator/v1"
	kvapi "kubevirt.io/api/core"
	aaqapi "kubevirt.io/application-aware-quota/staging/src/kubevirt.io/application-aware-quota-api/pkg/apis/core"
	cdiapi "kubevirt.io/containerized-data-importer-api/pkg/apis/core"
	migrationapi "kubevirt.io/kubevirt-migration-operator/api/v1alpha1"
	sspapi "kubevirt.io/ssp-operator/api/v1beta3"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

const DisableOperandDeletionAnnotation = "console.openshift.io/disable-operand-delete"

const (
	crName = util.HyperConvergedName
)

type DeploymentOperatorParams struct {
	Namespace                string
	Image                    string
	WebhookImage             string
	CliDownloadsImage        string
	KVUIPluginImage          string
	KVUIProxyImage           string
	PasstImage               string
	PasstCNIImage            string
	WaspAgentImage           string
	ImagePullPolicy          string
	ConversionContainer      string
	VmwareContainer          string
	VirtIOWinContainer       string
	Smbios                   string
	Machinetype              string
	Amd64MachineType         string
	Arm64MachineType         string
	S390xMachineType         string
	HcoKvIoVersion           string
	KubevirtVersion          string
	KvVirtLancherOsVersion   string
	CdiVersion               string
	CnaoVersion              string
	SspVersion               string
	HppoVersion              string
	MtqVersion               string
	AaqVersion               string
	MigrationOperatorVersion string
	Env                      []corev1.EnvVar
	AddNetworkPolicyLabels   bool
}

func GetDeploymentSpecOperator(params *DeploymentOperatorParams) appsv1.DeploymentSpec {
	envs := buildEnvVars(params)

	return appsv1.DeploymentSpec{
		Replicas: ptr.To[int32](1),
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"name": util.HCOOperatorName,
			},
		},
		Strategy: appsv1.DeploymentStrategy{
			Type: appsv1.RollingUpdateDeploymentStrategyType,
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: getLabelsWithNetworkPolicies(util.HCOOperatorName, params),
			},
			Spec: corev1.PodSpec{
				ServiceAccountName: util.HCOOperatorName,
				SecurityContext:    GetStdPodSecurityContext(),
				Containers: []corev1.Container{
					{
						Name:            util.HCOOperatorName,
						Image:           params.Image,
						ImagePullPolicy: corev1.PullPolicy(params.ImagePullPolicy),
						Command:         stringListToSlice(util.HCOOperatorName),
						ReadinessProbe:  getReadinessProbe(util.ReadinessEndpointName, util.HealthProbePort),
						LivenessProbe:   getLivenessProbe(util.LivenessEndpointName, util.HealthProbePort),
						Env:             envs,
						Resources: corev1.ResourceRequirements{
							Requests: map[corev1.ResourceName]resource.Quantity{
								corev1.ResourceCPU:    resource.MustParse("10m"),
								corev1.ResourceMemory: resource.MustParse("96Mi"),
							},
						},
						SecurityContext:          GetStdContainerSecurityContext(),
						TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
						Ports: []corev1.ContainerPort{
							getMetricsPort(),
						},
					},
				},
				PriorityClassName: "system-cluster-critical",
			},
		},
	}
}

func buildEnvVars(params *DeploymentOperatorParams) []corev1.EnvVar {
	envs := append([]corev1.EnvVar{
		{
			// deprecated: left here for CI test.
			Name:  util.OperatorWebhookModeEnv,
			Value: "false",
		},
		{
			Name:  util.ContainerAppName,
			Value: util.ContainerOperatorApp,
		},
		{
			Name:  "KVM_EMULATION",
			Value: "",
		},
		{
			Name:  "OPERATOR_IMAGE",
			Value: params.Image,
		},
		{
			Name:  "OPERATOR_NAME",
			Value: util.HCOOperatorName,
		},
		{
			Name:  "OPERATOR_NAMESPACE",
			Value: params.Namespace,
		},
		{
			Name: "POD_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		},
		{
			Name:  "VIRTIOWIN_CONTAINER",
			Value: params.VirtIOWinContainer,
		},
		{
			Name:  "SMBIOS",
			Value: params.Smbios,
		},
		{
			Name:  "MACHINETYPE",
			Value: params.Machinetype,
		},
		{
			Name:  "AMD64_MACHINETYPE",
			Value: params.Amd64MachineType,
		},
		{
			Name:  "ARM64_MACHINETYPE",
			Value: params.Arm64MachineType,
		},
		{
			Name:  "S390X_MACHINETYPE",
			Value: params.S390xMachineType,
		},
		{
			Name:  util.HcoKvIoVersionName,
			Value: params.HcoKvIoVersion,
		},
		{
			Name:  util.KubevirtVersionEnvV,
			Value: params.KubevirtVersion,
		},
		{
			Name:  util.CdiVersionEnvV,
			Value: params.CdiVersion,
		},
		{
			Name:  util.CnaoVersionEnvV,
			Value: params.CnaoVersion,
		},
		{
			Name:  util.SspVersionEnvV,
			Value: params.SspVersion,
		},
		{
			Name:  util.HppoVersionEnvV,
			Value: params.HppoVersion,
		},
		{
			Name:  util.AaqVersionEnvV,
			Value: params.AaqVersion,
		},
		{
			Name:  util.MigrationOperatorVersionEnvV,
			Value: params.MigrationOperatorVersion,
		},
		{
			Name:  util.KVUIPluginImageEnvV,
			Value: params.KVUIPluginImage,
		},
		{
			Name:  util.KVUIProxyImageEnvV,
			Value: params.KVUIProxyImage,
		},
		{
			Name:  util.PasstImageEnvV,
			Value: params.PasstImage,
		},
		{
			Name:  util.PasstCNIImageEnvV,
			Value: params.PasstCNIImage,
		},
		{
			Name:  util.WaspAgentImageEnvV,
			Value: params.WaspAgentImage,
		},
	}, params.Env...)

	if params.KvVirtLancherOsVersion != "" {
		envs = append(envs, corev1.EnvVar{
			Name:  util.KvVirtLauncherOSVersionEnvV,
			Value: params.KvVirtLancherOsVersion,
		})
	}

	if params.AddNetworkPolicyLabels {
		envs = append(envs, corev1.EnvVar{
			Name:  util.DeployNetworkPoliciesEnvV,
			Value: "true",
		})
	}

	return envs
}

func GetDeploymentSpecCliDownloads(params *DeploymentOperatorParams) appsv1.DeploymentSpec {
	return appsv1.DeploymentSpec{
		Replicas: ptr.To[int32](1),
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"name": util.CLIDownloadsName,
			},
		},
		Strategy: appsv1.DeploymentStrategy{
			Type: appsv1.RollingUpdateDeploymentStrategyType,
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: GetLabels(util.CLIDownloadsName, params.HcoKvIoVersion),
			},
			Spec: corev1.PodSpec{
				ServiceAccountName:           util.CLIDownloadsName,
				AutomountServiceAccountToken: ptr.To(false),
				SecurityContext:              GetStdPodSecurityContext(),
				Containers: []corev1.Container{
					{
						Name:            "server",
						Image:           params.CliDownloadsImage,
						ImagePullPolicy: corev1.PullPolicy(params.ImagePullPolicy),
						Resources: corev1.ResourceRequirements{
							Requests: map[corev1.ResourceName]resource.Quantity{
								corev1.ResourceCPU:    resource.MustParse("10m"),
								corev1.ResourceMemory: resource.MustParse("96Mi"),
							},
						},
						Ports: []corev1.ContainerPort{
							{
								Protocol:      corev1.ProtocolTCP,
								ContainerPort: util.CliDownloadsServerPort,
							},
						},
						SecurityContext:          GetStdContainerSecurityContext(),
						ReadinessProbe:           getReadinessProbe("/health", util.CliDownloadsServerPort),
						LivenessProbe:            getLivenessProbe("/health", util.CliDownloadsServerPort),
						TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
					},
				},
				PriorityClassName: "system-cluster-critical",
			},
		},
	}
}

func GetLabels(name, hcoKvIoVersion string) map[string]string {
	return map[string]string{
		"name":                 name,
		util.AppLabelVersion:   hcoKvIoVersion,
		util.AppLabelPartOf:    util.HyperConvergedCluster,
		util.AppLabelComponent: string(util.AppComponentDeployment),
	}
}

func getLabelsWithNetworkPolicies(deploymentName string, params *DeploymentOperatorParams) map[string]string {
	labels := GetLabels(deploymentName, params.HcoKvIoVersion)
	if params.AddNetworkPolicyLabels {
		labels[util.AllowEgressToDNSAndAPIServerLabel] = "true"
		labels[util.AllowIngressToMetricsEndpointLabel] = "true"
	}

	return labels
}

func GetStdPodSecurityContext() *corev1.PodSecurityContext {
	return &corev1.PodSecurityContext{
		RunAsNonRoot: ptr.To(true),
		SeccompProfile: &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		},
	}
}

func GetStdContainerSecurityContext() *corev1.SecurityContext {
	return &corev1.SecurityContext{
		AllowPrivilegeEscalation: ptr.To(false),
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{"ALL"},
		},
	}
}

// Currently we are abusing the pod readiness to signal to OLM that HCO is not ready
// for an upgrade. This has a lot of side effects, one of this is the validating webhook
// being not able to receive traffic when exposed by a pod that is not reporting ready=true.
// This can cause a lot of side effects if not deadlocks when the system reach a status where,
// for any possible reason, HCO pod cannot be ready and so HCO pod cannot validate any further update or
// delete request on HCO CR.
// A proper solution is properly use the readiness probe only to report the pod readiness and communicate
// status to OLM via conditions once OLM will be ready for:
// https://github.com/operator-framework/enhancements/blob/master/enhancements/operator-conditions.md
// in the meanwhile a quick (but dirty!) solution is to expose the same hco binary on two distinct pods:
// the first one will run only the controller and the second one (almost always ready) just the validating
// webhook one.
func GetDeploymentSpecWebhook(params *DeploymentOperatorParams) appsv1.DeploymentSpec {
	return appsv1.DeploymentSpec{
		Replicas: ptr.To[int32](1),
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"name": util.HCOWebhookName,
			},
		},
		Strategy: appsv1.DeploymentStrategy{
			Type: appsv1.RollingUpdateDeploymentStrategyType,
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: getLabelsWithNetworkPolicies(util.HCOWebhookName, params),
			},
			Spec: corev1.PodSpec{
				ServiceAccountName: util.HCOOperatorName,
				SecurityContext:    GetStdPodSecurityContext(),
				Containers: []corev1.Container{
					{
						Name:            util.HCOWebhookName,
						Image:           params.WebhookImage,
						ImagePullPolicy: corev1.PullPolicy(params.ImagePullPolicy),
						Command:         stringListToSlice(util.HCOWebhookName),
						ReadinessProbe:  getReadinessProbe(util.ReadinessEndpointName, util.HealthProbePort),
						LivenessProbe:   getLivenessProbe(util.LivenessEndpointName, util.HealthProbePort),
						Env: append([]corev1.EnvVar{
							{
								// deprecated: left here for CI test.
								Name:  util.OperatorWebhookModeEnv,
								Value: "true",
							},
							{
								Name:  util.ContainerAppName,
								Value: util.ContainerWebhookApp,
							},
							{
								Name:  "OPERATOR_IMAGE",
								Value: params.WebhookImage,
							},
							{
								Name:  "OPERATOR_NAME",
								Value: util.HCOWebhookName,
							},
							{
								Name:  "OPERATOR_NAMESPACE",
								Value: params.Namespace,
							},
							{
								Name: "POD_NAME",
								ValueFrom: &corev1.EnvVarSource{
									FieldRef: &corev1.ObjectFieldSelector{
										FieldPath: "metadata.name",
									},
								},
							},
							{
								Name:  util.HcoKvIoVersionName,
								Value: params.HcoKvIoVersion,
							},
						}, params.Env...),
						Resources: corev1.ResourceRequirements{
							Requests: map[corev1.ResourceName]resource.Quantity{
								corev1.ResourceCPU:    resource.MustParse("5m"),
								corev1.ResourceMemory: resource.MustParse("48Mi"),
							},
						},
						SecurityContext:          GetStdContainerSecurityContext(),
						TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
						Ports: []corev1.ContainerPort{
							getWebhookPort(),
							getMetricsPort(),
						},
					},
				},
				PriorityClassName: "system-node-critical",
			},
		},
	}
}

var (
	emptyAPIGroup = []string{""}
)

func GetClusterPermissions() []rbacv1.PolicyRule {
	const configOpenshiftIO = "config.openshift.io"
	const operatorOpenshiftIO = "operator.openshift.io"
	return []rbacv1.PolicyRule{
		{
			APIGroups: stringListToSlice(util.APIVersionGroup),
			Resources: stringListToSlice("hyperconvergeds"),
			Verbs:     stringListToSlice("get", "list", "update", "watch"),
		},
		{
			APIGroups: stringListToSlice(util.APIVersionGroup),
			Resources: stringListToSlice("hyperconvergeds/finalizers", "hyperconvergeds/status"),
			Verbs:     stringListToSlice("get", "list", "create", "update", "watch"),
		},
		roleWithAllPermissions(kvapi.GroupName, stringListToSlice("kubevirts", "kubevirts/finalizers")),
		roleWithAllPermissions(cdiapi.GroupName, stringListToSlice("cdis", "cdis/finalizers")),
		roleWithAllPermissions(sspapi.GroupVersion.Group, stringListToSlice("ssps", "ssps/finalizers")),
		roleWithAllPermissions(cnaoapi.GroupVersion.Group, stringListToSlice("networkaddonsconfigs", "networkaddonsconfigs/finalizers")),
		roleWithAllPermissions(aaqapi.GroupName, stringListToSlice("aaqs", "aaqs/finalizers")),
		roleWithAllPermissions(migrationapi.GroupVersion.Group, stringListToSlice("migcontrollers", "migcontrollers/finalizers")),
		roleWithAllPermissions("", stringListToSlice("configmaps")),
		{
			APIGroups: emptyAPIGroup,
			Resources: stringListToSlice("events"),
			Verbs:     stringListToSlice("get", "list", "watch", "create", "patch"),
		},
		roleWithAllPermissions("", stringListToSlice("services")),
		{
			APIGroups: emptyAPIGroup,
			Resources: stringListToSlice("pods", "nodes"),
			Verbs:     stringListToSlice("get", "list", "watch", "patch"),
		},
		roleWithAllPermissions("", stringListToSlice("secrets")),
		{
			APIGroups: emptyAPIGroup,
			Resources: stringListToSlice("endpoints"),
			Verbs:     stringListToSlice("get", "list", "delete", "watch"),
		},
		{
			APIGroups: emptyAPIGroup,
			Resources: stringListToSlice("namespaces"),
			Verbs:     stringListToSlice("get", "list", "watch", "patch", "update"),
		},
		{
			APIGroups: stringListToSlice("apps"),
			Resources: stringListToSlice("deployments", "replicasets", "daemonsets"),
			Verbs:     stringListToSlice("get", "list", "watch", "create", "update", "delete"),
		},
		roleWithAllPermissions("rbac.authorization.k8s.io",
			stringListToSlice("roles", "clusterroles", "rolebindings", "clusterrolebindings")),
		{
			APIGroups: stringListToSlice("apiextensions.k8s.io"),
			Resources: stringListToSlice("customresourcedefinitions"),
			Verbs:     stringListToSlice("get", "list", "watch", "delete"),
		},
		{
			APIGroups: stringListToSlice("apiextensions.k8s.io"),
			Resources: stringListToSlice("customresourcedefinitions/status", "customresourcedefinitions/finalizers"),
			Verbs:     stringListToSlice("get", "list", "watch", "patch", "update"),
		},
		roleWithAllPermissions("monitoring.coreos.com", stringListToSlice("servicemonitors", "prometheusrules")),
		{
			APIGroups: stringListToSlice("operators.coreos.com"),
			Resources: stringListToSlice("clusterserviceversions"),
			Verbs:     stringListToSlice("get", "list", "watch", "update", "patch"),
		},
		{
			APIGroups: stringListToSlice("scheduling.k8s.io"),
			Resources: stringListToSlice("priorityclasses"),
			Verbs:     stringListToSlice("get", "list", "watch", "create", "delete", "patch"),
		},
		{
			APIGroups: stringListToSlice("admissionregistration.k8s.io"),
			Resources: stringListToSlice("validatingwebhookconfigurations"),
			Verbs:     stringListToSlice("list", "watch", "update", "patch"),
		},
		roleWithAllPermissions("console.openshift.io", stringListToSlice("consoleclidownloads", "consolequickstarts")),
		{
			APIGroups: stringListToSlice(configOpenshiftIO),
			Resources: stringListToSlice("clusterversions", "infrastructures", "networks"),
			Verbs:     stringListToSlice("get", "list"),
		},
		{
			APIGroups: stringListToSlice(configOpenshiftIO),
			Resources: stringListToSlice("ingresses"),
			Verbs:     stringListToSlice("get", "list", "watch"),
		},
		{
			APIGroups: stringListToSlice(configOpenshiftIO),
			Resources: stringListToSlice("ingresses/status"),
			Verbs:     stringListToSlice("update"),
		},
		{
			APIGroups: stringListToSlice(configOpenshiftIO),
			Resources: stringListToSlice("apiservers"),
			Verbs:     stringListToSlice("get", "list", "watch"),
		},
		{
			APIGroups: stringListToSlice(operatorOpenshiftIO),
			Resources: stringListToSlice("kubedeschedulers"),
			Verbs:     stringListToSlice("get", "list", "watch"),
		},
		{
			APIGroups: stringListToSlice(configOpenshiftIO),
			Resources: stringListToSlice("dnses"),
			Verbs:     stringListToSlice("get"),
		},
		roleWithAllPermissions("coordination.k8s.io", stringListToSlice("leases")),
		roleWithAllPermissions("route.openshift.io", stringListToSlice("routes")),
		{
			APIGroups: stringListToSlice("route.openshift.io"),
			Resources: stringListToSlice("routes/custom-host"),
			Verbs:     stringListToSlice("create", "update", "patch"),
		},
		{
			APIGroups: stringListToSlice("operators.coreos.com"),
			Resources: stringListToSlice("operatorconditions"),
			Verbs:     stringListToSlice("get", "list", "watch", "update", "patch"),
		},
		roleWithAllPermissions("image.openshift.io", stringListToSlice("imagestreams")),
		roleWithAllPermissions("console.openshift.io", stringListToSlice("consoleplugins")),
		{
			APIGroups: stringListToSlice("operator.openshift.io"),
			Resources: stringListToSlice("consoles"),
			Verbs:     stringListToSlice("get", "list", "watch", "update"),
		},
		{
			APIGroups: stringListToSlice("monitoring.coreos.com"),
			Resources: stringListToSlice("alertmanagers", "alertmanagers/api"),
			Verbs:     stringListToSlice("get", "list", "create", "delete"),
		},
		{
			APIGroups: stringListToSlice(""),
			Resources: stringListToSlice("serviceaccounts"),
			Verbs:     stringListToSlice("get", "list", "watch", "create", "update", "delete"),
		},
		{
			APIGroups: stringListToSlice("k8s.cni.cncf.io"),
			Resources: stringListToSlice("network-attachment-definitions"),
			Verbs:     stringListToSlice("get", "list", "watch", "create", "update", "delete"),
		},
		{
			APIGroups: stringListToSlice("security.openshift.io"),
			Resources: stringListToSlice("securitycontextconstraints"),
			Verbs:     stringListToSlice("get", "list", "watch", "create", "update", "delete"),
		},
		{
			APIGroups: stringListToSlice(networkingv1.GroupName),
			Resources: stringListToSlice("networkpolicies"),
			Verbs:     stringListToSlice("get", "list", "watch", "create", "update", "delete"),
		},
		{
			APIGroups: stringListToSlice(admissionregistrationv1.GroupName),
			Resources: stringListToSlice("validatingadmissionpolicies", "validatingadmissionpolicybindings"),
			Verbs:     stringListToSlice("get", "list", "watch", "create", "update", "delete"),
		},
		{
			APIGroups: stringListToSlice(persesv1alpha1.GroupVersion.Group),
			Resources: stringListToSlice("persesdashboards", "persesdatasources"),
			Verbs:     stringListToSlice("get", "list", "watch", "create", "update", "delete"),
		},
	}
}

func roleWithAllPermissions(apiGroup string, resources []string) rbacv1.PolicyRule {
	return rbacv1.PolicyRule{
		APIGroups: stringListToSlice(apiGroup),
		Resources: resources,
		Verbs:     stringListToSlice("get", "list", "watch", "create", "update", "delete", "patch"),
	}
}

func GetOperatorCR() *hcov1beta1.HyperConverged {
	defaultScheme := runtime.NewScheme()
	_ = hcov1beta1.AddToScheme(defaultScheme)
	_ = hcov1beta1.RegisterDefaults(defaultScheme)
	defaultHco := &hcov1beta1.HyperConverged{
		TypeMeta: metav1.TypeMeta{
			APIVersion: util.APIVersion,
			Kind:       util.HyperConvergedKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: crName,
		}}
	defaultScheme.Default(defaultHco)
	return defaultHco
}

func getReadinessProbe(endpoint string, port int32) *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: endpoint,
				Port: intstr.IntOrString{
					Type:   intstr.Int,
					IntVal: port,
				},
				Scheme: corev1.URISchemeHTTP,
			},
		},
		InitialDelaySeconds: 5,
		PeriodSeconds:       5,
		FailureThreshold:    1,
	}
}

func getLivenessProbe(endpoint string, port int32) *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: endpoint,
				Port: intstr.IntOrString{
					Type:   intstr.Int,
					IntVal: port,
				},
				Scheme: corev1.URISchemeHTTP,
			},
		},
		InitialDelaySeconds: 30,
		PeriodSeconds:       5,
		FailureThreshold:    1,
	}
}

func getMetricsPort() corev1.ContainerPort {
	return corev1.ContainerPort{
		Name:          util.MetricsPortName,
		ContainerPort: util.MetricsPort,
		Protocol:      corev1.ProtocolTCP,
	}
}

func getWebhookPort() corev1.ContainerPort {
	return corev1.ContainerPort{
		Name:          util.WebhookPortName,
		ContainerPort: util.WebhookPort,
		Protocol:      corev1.ProtocolTCP,
	}
}

func stringListToSlice(words ...string) []string {
	return words
}
