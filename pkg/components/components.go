package components

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/blang/semver/v4"
	csvVersion "github.com/operator-framework/api/pkg/lib/version"
	csvv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
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
	crName              = util.HyperConvergedName
	packageName         = util.HyperConvergedName
	hcoDeploymentName   = "hco-operator"
	hcoWhDeploymentName = "hco-webhook"
	certVolume          = "apiservice-cert"

	kubevirtProjectName = "KubeVirt project"
	rbacVersionV1       = "rbac.authorization.k8s.io/v1"
)

var deploymentType = metav1.TypeMeta{
	APIVersion: "apps/v1",
	Kind:       "Deployment",
}

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

func GetDeploymentOperator(params *DeploymentOperatorParams) appsv1.Deployment {
	return appsv1.Deployment{
		TypeMeta: deploymentType,
		ObjectMeta: metav1.ObjectMeta{
			Name: util.HCOOperatorName,
			Labels: map[string]string{
				"name": util.HCOOperatorName,
			},
		},
		Spec: GetDeploymentSpecOperator(params),
	}
}

func GetDeploymentWebhook(params *DeploymentOperatorParams) appsv1.Deployment {
	deploy := appsv1.Deployment{
		TypeMeta: deploymentType,
		ObjectMeta: metav1.ObjectMeta{
			Name: util.HCOWebhookName,
			Labels: map[string]string{
				"name": util.HCOWebhookName,
			},
		},
		Spec: GetDeploymentSpecWebhook(params),
	}

	InjectVolumesForWebHookCerts(&deploy)
	return deploy
}

func GetDeploymentCliDownloads(params *DeploymentOperatorParams) appsv1.Deployment {
	return appsv1.Deployment{
		TypeMeta: deploymentType,
		ObjectMeta: metav1.ObjectMeta{
			Name: util.CLIDownloadsName,
			Labels: map[string]string{
				"name": util.CLIDownloadsName,
			},
		},
		Spec: GetDeploymentSpecCliDownloads(params),
	}
}

func GetServiceWebhook() corev1.Service {
	return corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: util.HCOWebhookName + "-service",
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"name": util.HCOWebhookName,
			},
			Ports: []corev1.ServicePort{
				{
					Name:       strconv.Itoa(util.WebhookPort),
					Port:       util.WebhookPort,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromInt32(util.WebhookPort),
				},
			},
			Type: corev1.ServiceTypeClusterIP,
		},
	}
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
				Labels: getLabels(util.CLIDownloadsName, params.HcoKvIoVersion),
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

func getLabels(name, hcoKvIoVersion string) map[string]string {
	return map[string]string{
		"name":                 name,
		util.AppLabelVersion:   hcoKvIoVersion,
		util.AppLabelPartOf:    util.HyperConvergedCluster,
		util.AppLabelComponent: string(util.AppComponentDeployment),
	}
}

func getLabelsWithNetworkPolicies(deploymentName string, params *DeploymentOperatorParams) map[string]string {
	labels := getLabels(deploymentName, params.HcoKvIoVersion)
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

func GetClusterRole() rbacv1.ClusterRole {
	return rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			APIVersion: rbacVersionV1,
			Kind:       "ClusterRole",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: util.HCOOperatorName,
			Labels: map[string]string{
				"name": util.HCOOperatorName,
			},
		},
		Rules: GetClusterPermissions(),
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
		{
			APIGroups: emptyAPIGroup,
			Resources: stringListToSlice("secrets"),
			Verbs:     stringListToSlice("get", "list", "watch", "create", "update", "delete"),
		},
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
			Resources: stringListToSlice("customresourcedefinitions/status"),
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

func GetServiceAccount(namespace string) corev1.ServiceAccount {
	return createServiceAccount(namespace, util.HCOOperatorName)
}

func GetCLIDownloadServiceAccount(namespace string) corev1.ServiceAccount {
	return createServiceAccount(namespace, util.CLIDownloadsName)
}

func createServiceAccount(namespace, name string) corev1.ServiceAccount {
	return corev1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ServiceAccount",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"name": name,
			},
		},
	}
}

func GetClusterRoleBinding(namespace string) rbacv1.ClusterRoleBinding {
	return rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: rbacVersionV1,
			Kind:       "ClusterRoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: util.HCOOperatorName,
			Labels: map[string]string{
				"name": util.HCOOperatorName,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     util.HCOOperatorName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      util.HCOOperatorName,
				Namespace: namespace,
			},
		},
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

// GetInstallStrategyBase returns the basics of an HCO InstallStrategy
func GetInstallStrategyBase(params *DeploymentOperatorParams) *csvv1alpha1.StrategyDetailsDeployment {
	return &csvv1alpha1.StrategyDetailsDeployment{

		DeploymentSpecs: []csvv1alpha1.StrategyDeploymentSpec{
			{
				Name:  hcoDeploymentName,
				Spec:  GetDeploymentSpecOperator(params),
				Label: getLabels(util.HCOOperatorName, params.HcoKvIoVersion),
			},
			{
				Name:  hcoWhDeploymentName,
				Spec:  GetDeploymentSpecWebhook(params),
				Label: getLabels(util.HCOWebhookName, params.HcoKvIoVersion),
			},
			{
				Name:  util.CLIDownloadsName,
				Spec:  GetDeploymentSpecCliDownloads(params),
				Label: getLabels(util.CLIDownloadsName, params.HcoKvIoVersion),
			},
		},
		Permissions: []csvv1alpha1.StrategyDeploymentPermissions{},
		ClusterPermissions: []csvv1alpha1.StrategyDeploymentPermissions{
			{
				ServiceAccountName: util.HCOOperatorName,
				Rules:              GetClusterPermissions(),
			},
			{
				ServiceAccountName: util.CLIDownloadsName,
				Rules:              []rbacv1.PolicyRule{},
			},
		},
	}
}

type CSVBaseParams struct {
	Name            string
	Namespace       string
	DisplayName     string
	MetaDescription string
	Description     string
	Image           string
	Version         semver.Version
	CrdDisplay      string
	Icon            string
}

// GetCSVBase returns a base HCO CSV without an InstallStrategy
func GetCSVBase(params *CSVBaseParams) *csvv1alpha1.ClusterServiceVersion {
	almExamples, _ := json.Marshal(
		map[string]interface{}{
			"apiVersion": util.APIVersion,
			"kind":       util.HyperConvergedKind,
			"metadata": map[string]interface{}{
				"name":      packageName,
				"namespace": params.Namespace,
				"annotations": map[string]string{
					"deployOVS": "false",
				},
			},
			"spec": map[string]interface{}{},
		})

	// Explicitly fail on unvalidated (for any reason) requests:
	// this can make removing HCO CR harder if HCO webhook is not able
	// to really validate the requests.
	// In that case the user can only directly remove the
	// ValidatingWebhookConfiguration object first (eventually bypassing the OLM if needed).
	// so failurePolicy = admissionregistrationv1.Fail

	validatingWebhook := csvv1alpha1.WebhookDescription{
		GenerateName:            util.HcoValidatingWebhook,
		Type:                    csvv1alpha1.ValidatingAdmissionWebhook,
		DeploymentName:          hcoWhDeploymentName,
		ContainerPort:           util.WebhookPort,
		AdmissionReviewVersions: stringListToSlice("v1beta1", "v1"),
		SideEffects:             ptr.To(admissionregistrationv1.SideEffectClassNone),
		FailurePolicy:           ptr.To(admissionregistrationv1.Fail),
		TimeoutSeconds:          ptr.To[int32](10),
		Rules: []admissionregistrationv1.RuleWithOperations{
			{
				Operations: []admissionregistrationv1.OperationType{
					admissionregistrationv1.Create,
					admissionregistrationv1.Delete,
					admissionregistrationv1.Update,
				},
				Rule: admissionregistrationv1.Rule{
					APIGroups:   stringListToSlice(util.APIVersionGroup),
					APIVersions: stringListToSlice(util.APIVersionBeta),
					Resources:   stringListToSlice("hyperconvergeds"),
				},
			},
		},
		WebhookPath: ptr.To(util.HCOWebhookPath),
	}

	mutatingNamespaceWebhook := csvv1alpha1.WebhookDescription{
		GenerateName:            util.HcoMutatingWebhookNS,
		Type:                    csvv1alpha1.MutatingAdmissionWebhook,
		DeploymentName:          hcoWhDeploymentName,
		ContainerPort:           util.WebhookPort,
		AdmissionReviewVersions: stringListToSlice("v1beta1", "v1"),
		SideEffects:             ptr.To(admissionregistrationv1.SideEffectClassNoneOnDryRun),
		FailurePolicy:           ptr.To(admissionregistrationv1.Fail),
		TimeoutSeconds:          ptr.To[int32](10),
		ObjectSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{util.KubernetesMetadataName: params.Namespace},
		},
		Rules: []admissionregistrationv1.RuleWithOperations{
			{
				Operations: []admissionregistrationv1.OperationType{
					admissionregistrationv1.Delete,
				},
				Rule: admissionregistrationv1.Rule{
					APIGroups:   []string{""},
					APIVersions: stringListToSlice("v1"),
					Resources:   stringListToSlice("namespaces"),
				},
			},
		},
		WebhookPath: ptr.To(util.HCONSWebhookPath),
	}

	mutatingHyperConvergedWebhook := csvv1alpha1.WebhookDescription{
		GenerateName:            util.HcoMutatingWebhookHyperConverged,
		Type:                    csvv1alpha1.MutatingAdmissionWebhook,
		DeploymentName:          hcoWhDeploymentName,
		ContainerPort:           util.WebhookPort,
		AdmissionReviewVersions: stringListToSlice("v1beta1", "v1"),
		SideEffects:             ptr.To(admissionregistrationv1.SideEffectClassNoneOnDryRun),
		FailurePolicy:           ptr.To(admissionregistrationv1.Fail),
		TimeoutSeconds:          ptr.To[int32](10),
		Rules: []admissionregistrationv1.RuleWithOperations{
			{
				Operations: []admissionregistrationv1.OperationType{
					admissionregistrationv1.Create,
					admissionregistrationv1.Update,
				},
				Rule: admissionregistrationv1.Rule{
					APIGroups:   stringListToSlice(util.APIVersionGroup),
					APIVersions: stringListToSlice(util.APIVersionBeta),
					Resources:   stringListToSlice("hyperconvergeds"),
				},
			},
		},
		WebhookPath: ptr.To(util.HCOMutatingWebhookPath),
	}

	return &csvv1alpha1.ClusterServiceVersion{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "operators.coreos.com/v1alpha1",
			Kind:       "ClusterServiceVersion",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%v.v%v", params.Name, params.Version.String()),
			Namespace: params.Namespace,
			Annotations: map[string]string{
				"alm-examples":                   string(almExamples),
				"capabilities":                   "Deep Insights",
				"certified":                      "false",
				"categories":                     "OpenShift Optional",
				"containerImage":                 params.Image,
				DisableOperandDeletionAnnotation: "true",
				"createdAt":                      time.Now().Format("2006-01-02 15:04:05"),
				"description":                    params.MetaDescription,
				"repository":                     "https://github.com/kubevirt/hyperconverged-cluster-operator",
				"support":                        "false",
				"operatorframework.io/suggested-namespace":         params.Namespace,
				"operatorframework.io/initialization-resource":     string(almExamples),
				"operators.openshift.io/infrastructure-features":   `["disconnected","proxy-aware"]`, // TODO: deprecated, remove once all the tools support "features.operators.openshift.io/*"
				"features.operators.openshift.io/disconnected":     "true",
				"features.operators.openshift.io/fips-compliant":   "false",
				"features.operators.openshift.io/proxy-aware":      "true",
				"features.operators.openshift.io/cnf":              "false",
				"features.operators.openshift.io/cni":              "true",
				"features.operators.openshift.io/csi":              "true",
				"features.operators.openshift.io/tls-profiles":     "true",
				"features.operators.openshift.io/token-auth-aws":   "false",
				"features.operators.openshift.io/token-auth-azure": "false",
				"features.operators.openshift.io/token-auth-gcp":   "false",
				"openshift.io/required-scc":                        "restricted-v2",
			},
		},
		Spec: csvv1alpha1.ClusterServiceVersionSpec{
			DisplayName: params.DisplayName,
			Description: params.Description,
			Keywords:    stringListToSlice("KubeVirt", "Virtualization"),
			Version:     csvVersion.OperatorVersion{Version: params.Version},
			Maintainers: []csvv1alpha1.Maintainer{
				{
					Name:  kubevirtProjectName,
					Email: "kubevirt-dev@googlegroups.com",
				},
			},
			Maturity: "alpha",
			Provider: csvv1alpha1.AppLink{
				Name: kubevirtProjectName,
				// https://github.com/operator-framework/operator-courier/issues/173
				// URL:  "https://kubevirt.io",
			},
			Links: []csvv1alpha1.AppLink{
				{
					Name: kubevirtProjectName,
					URL:  "https://kubevirt.io",
				},
				{
					Name: "Source Code",
					URL:  "https://github.com/kubevirt/hyperconverged-cluster-operator",
				},
			},
			Icon: []csvv1alpha1.Icon{
				{
					MediaType: "image/svg+xml",
					Data:      params.Icon,
				},
			},
			Labels: map[string]string{
				"alm-owner-kubevirt": packageName,
				"operated-by":        packageName,
			},
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"alm-owner-kubevirt": packageName,
					"operated-by":        packageName,
				},
			},
			InstallModes: []csvv1alpha1.InstallMode{
				{
					Type:      csvv1alpha1.InstallModeTypeOwnNamespace,
					Supported: false,
				},
				{
					Type:      csvv1alpha1.InstallModeTypeSingleNamespace,
					Supported: false,
				},
				{
					Type:      csvv1alpha1.InstallModeTypeMultiNamespace,
					Supported: false,
				},
				{
					Type:      csvv1alpha1.InstallModeTypeAllNamespaces,
					Supported: true,
				},
			},
			// Skip this in favor of having a separate function to get
			// the actual StrategyDetailsDeployment when merging CSVs
			InstallStrategy: csvv1alpha1.NamedInstallStrategy{},
			WebhookDefinitions: []csvv1alpha1.WebhookDescription{
				validatingWebhook,
				mutatingNamespaceWebhook,
				mutatingHyperConvergedWebhook,
			},
			CustomResourceDefinitions: csvv1alpha1.CustomResourceDefinitions{
				Owned: []csvv1alpha1.CRDDescription{
					{
						Name:        "hyperconvergeds.hco.kubevirt.io",
						Version:     util.CurrentAPIVersion,
						Kind:        util.HyperConvergedKind,
						DisplayName: params.CrdDisplay + " Deployment",
						Description: "Represents the deployment of " + params.CrdDisplay,
						// TODO: move this to annotations on hyperconverged_types.go once kubebuilder
						// properly supports SpecDescriptors as the operator-sdk already does
						SpecDescriptors: []csvv1alpha1.SpecDescriptor{
							{
								DisplayName: "Infra components node affinity",
								Description: "nodeAffinity describes node affinity scheduling rules for the infra pods.",
								Path:        "infra.nodePlacement.affinity.nodeAffinity",
								XDescriptors: stringListToSlice(
									"urn:alm:descriptor:com.tectonic.ui:nodeAffinity",
								),
							},
							{
								DisplayName: "Infra components pod affinity",
								Description: "podAffinity describes pod affinity scheduling rules for the infra pods.",
								Path:        "infra.nodePlacement.affinity.podAffinity",
								XDescriptors: stringListToSlice(
									"urn:alm:descriptor:com.tectonic.ui:podAffinity",
								),
							},
							{
								DisplayName: "Infra components pod anti-affinity",
								Description: "podAntiAffinity describes pod anti affinity scheduling rules for the infra pods.",
								Path:        "infra.nodePlacement.affinity.podAntiAffinity",
								XDescriptors: stringListToSlice(
									"urn:alm:descriptor:com.tectonic.ui:podAntiAffinity",
								),
							},
							{
								DisplayName: "Workloads components node affinity",
								Description: "nodeAffinity describes node affinity scheduling rules for the workloads pods.",
								Path:        "workloads.nodePlacement.affinity.nodeAffinity",
								XDescriptors: stringListToSlice(
									"urn:alm:descriptor:com.tectonic.ui:nodeAffinity",
								),
							},
							{
								DisplayName: "Workloads components pod affinity",
								Description: "podAffinity describes pod affinity scheduling rules for the workloads pods.",
								Path:        "workloads.nodePlacement.affinity.podAffinity",
								XDescriptors: stringListToSlice(
									"urn:alm:descriptor:com.tectonic.ui:podAffinity",
								),
							},
							{
								DisplayName: "Workloads components pod anti-affinity",
								Description: "podAntiAffinity describes pod anti affinity scheduling rules for the workloads pods.",
								Path:        "workloads.nodePlacement.affinity.podAntiAffinity",
								XDescriptors: stringListToSlice(
									"urn:alm:descriptor:com.tectonic.ui:podAntiAffinity",
								),
							},
							{
								DisplayName: "HIDDEN FIELDS - operator version",
								Description: "HIDDEN FIELDS - operator version.",
								Path:        "version",
								XDescriptors: stringListToSlice(
									"urn:alm:descriptor:com.tectonic.ui:hidden",
								),
							},
						},
						StatusDescriptors: []csvv1alpha1.StatusDescriptor{},
					},
				},
				Required: []csvv1alpha1.CRDDescription{},
			},
		},
	}
}

func InjectVolumesForWebHookCerts(deploy *appsv1.Deployment) {
	// check if there is already a volume for api certificates
	for _, vol := range deploy.Spec.Template.Spec.Volumes {
		if vol.Name == certVolume {
			return
		}
	}

	volume := corev1.Volume{
		Name: certVolume,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName:  deploy.Name + "-service-cert",
				DefaultMode: ptr.To[int32](420),
				Items: []corev1.KeyToPath{
					{
						Key:  "tls.crt",
						Path: util.WebhookCertName,
					},
					{
						Key:  "tls.key",
						Path: util.WebhookKeyName,
					},
				},
			},
		},
	}
	deploy.Spec.Template.Spec.Volumes = append(deploy.Spec.Template.Spec.Volumes, volume)

	for index, container := range deploy.Spec.Template.Spec.Containers {
		deploy.Spec.Template.Spec.Containers[index].VolumeMounts = append(container.VolumeMounts,
			corev1.VolumeMount{
				Name:      certVolume,
				MountPath: util.DefaultWebhookCertDir,
			})
	}
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
