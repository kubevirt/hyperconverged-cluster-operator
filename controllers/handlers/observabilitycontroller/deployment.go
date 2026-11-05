package observabilitycontroller

import (
	"os"
	"slices"
	"strings"

	openshiftconfigv1 "github.com/openshift/api/config/v1"
	securityv1 "github.com/openshift/api/security/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/tlssecprofile"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

const (
	vmstatsCertMountPath    = "/etc/monitoring/clientcertificates"
	vmstatsCertVolumeName   = "monitoring-client-cert"
	vmstatsClientCertSecret = "kubevirt-virt-handler-monitoring-client-certs"
	enableVMStatsArg        = "--enable-vmstats"
	vmstatsCertPathArg      = "--vmstats-cert-path=" + vmstatsCertMountPath
)

var (
	resourceCPURequest    = resource.MustParse("10m")
	resourceMemoryRequest = resource.MustParse("64Mi")
	resourceMemoryLimit   = resource.MustParse("128Mi")
)

func NewDeploymentHandler(cli client.Client, scheme *runtime.Scheme) operands.Operand {
	return operands.NewConditionalHandler(
		operands.NewDeploymentHandler(cli, scheme, newDeployment, hcov1.ConditionKubevirtObservabilityControllerReady),
		shouldDeploy,
		func(hc *hcov1.HyperConverged) client.Object {
			return NewDeploymentWithNameOnly()
		},
	)
}

func NewDeploymentWithNameOnly() *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: hcoutil.GetOperatorNamespaceFromEnv(),
			Labels:    operands.GetLabels(hcoutil.AppComponentObservability),
		},
	}
}

func newDeployment(hc *hcov1.HyperConverged) *appsv1.Deployment {
	image := os.Getenv(hcoutil.ObservabilityControllerImageEnvV)

	profile := tlssecprofile.GetTLSSecurityProfile(hc.Spec.Security.TLSSecurityProfile)

	selectorLabels := map[string]string{
		hcoutil.AppLabel:          hcoutil.HyperConvergedName,
		hcoutil.AppLabelComponent: string(hcoutil.AppComponentObservability),
	}

	podLabels := operands.GetLabels(hcoutil.AppComponentObservability)

	args := []string{
		"--metrics-bind-address=:8443",
		"--metrics-secure=true",
		"--health-probe-bind-address=:8081",
		"--leader-elect=false",
		"--tls-security-profile=" + string(profile.Type),
	}

	if profile.Type == openshiftconfigv1.TLSProfileCustomType {
		args = append(args, "--tls-min-version="+string(profile.Custom.MinTLSVersion))

		if ciphers := strings.Join(profile.Custom.Ciphers, ","); ciphers != "" {
			args = append(args, "--tls-ciphers="+ciphers)
		}

		groups := joinTLSGroups(profile.Custom.Groups)
		if groups != "" {
			args = append(args, "--tls-groups="+groups)
		}
	}

	vmStatsEnabled := hc.Spec.FeatureGates.IsEnabled(observeVMStatFeatureGate)
	if vmStatsEnabled {
		args = append(args, enableVMStatsArg, vmstatsCertPathArg)
	}

	if flag := observabilityAllowlistFlag("--metrics-allowlist", getMetricsAllowlist(hc)); flag != "" {
		args = append(args, flag)
	}
	if flag := observabilityAllowlistFlag("--alerts-allowlist", getAlertsAllowlist(hc)); flag != "" {
		args = append(args, flag)
	}
	if flag := observabilityAllowlistFlag("--recording-rules-allowlist", getRecordingRulesAllowlist(hc)); flag != "" {
		args = append(args, flag)
	}

	dep := NewDeploymentWithNameOnly()
	dep.Spec = appsv1.DeploymentSpec{
		Replicas: new(int32(1)),
		Selector: &metav1.LabelSelector{
			MatchLabels: selectorLabels,
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: podLabels,
				Annotations: map[string]string{
					securityv1.RequiredSCCAnnotation: "restricted-v2",
				},
			},
			Spec: corev1.PodSpec{
				ServiceAccountName: serviceAccountName,
				PriorityClassName:  "system-cluster-critical",
				Tolerations: []corev1.Toleration{
					{
						Key:      "CriticalAddonsOnly",
						Operator: corev1.TolerationOpExists,
					},
				},
				SecurityContext: &corev1.PodSecurityContext{
					RunAsNonRoot: new(true),
					SeccompProfile: &corev1.SeccompProfile{
						Type: corev1.SeccompProfileTypeRuntimeDefault,
					},
				},
				Containers: []corev1.Container{
					{
						Name:  "manager",
						Image: image,
						Args:  args,
						Env: []corev1.EnvVar{
							{
								Name: "POD_NAMESPACE",
								ValueFrom: &corev1.EnvVarSource{
									FieldRef: &corev1.ObjectFieldSelector{
										FieldPath: "metadata.namespace",
									},
								},
							},
							{
								Name: "POD_SERVICE_ACCOUNT",
								ValueFrom: &corev1.EnvVarSource{
									FieldRef: &corev1.ObjectFieldSelector{
										FieldPath: "spec.serviceAccountName",
									},
								},
							},
						},
						Ports: []corev1.ContainerPort{
							{
								ContainerPort: 8443,
								Name:          "metrics",
								Protocol:      corev1.ProtocolTCP,
							},
							{
								ContainerPort: 8081,
								Name:          "health",
								Protocol:      corev1.ProtocolTCP,
							},
						},
						ImagePullPolicy: corev1.PullIfNotPresent,
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resourceCPURequest,
								corev1.ResourceMemory: resourceMemoryRequest,
							},
							Limits: corev1.ResourceList{
								corev1.ResourceMemory: resourceMemoryLimit,
							},
						},
						SecurityContext: &corev1.SecurityContext{
							AllowPrivilegeEscalation: new(false),
							ReadOnlyRootFilesystem:   new(true),
							RunAsNonRoot:             new(true),
							Capabilities: &corev1.Capabilities{
								Drop: []corev1.Capability{"ALL"},
							},
							SeccompProfile: &corev1.SeccompProfile{
								Type: corev1.SeccompProfileTypeRuntimeDefault,
							},
						},
						LivenessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path: "/healthz",
									Port: intstr.FromInt32(8081),
								},
							},
							InitialDelaySeconds: 15,
							PeriodSeconds:       20,
						},
						ReadinessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path: "/readyz",
									Port: intstr.FromInt32(8081),
								},
							},
							InitialDelaySeconds: 5,
							PeriodSeconds:       10,
						},
						TerminationMessagePath:   corev1.TerminationMessagePathDefault,
						TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
					},
				},
			},
		},
	}

	if vmStatsEnabled {
		// The secret is created by KubeVirt in this namespace. optional lets the
		// pod schedule before the secret exists; the controller exits until the
		// client certificate is mounted.
		dep.Spec.Template.Spec.Volumes = []corev1.Volume{
			{
				Name: vmstatsCertVolumeName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName:  vmstatsClientCertSecret,
						DefaultMode: new(int32(420)),
						Optional:    new(true),
					},
				},
			},
		}
		dep.Spec.Template.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{
			{
				Name:      vmstatsCertVolumeName,
				MountPath: vmstatsCertMountPath,
				ReadOnly:  true,
			},
		}
	}

	return dep
}

func joinTLSGroups(tlsGroups []openshiftconfigv1.TLSGroup) string {
	if len(tlsGroups) == 0 {
		return ""
	}

	return strings.Join(slices.Collect(func(yield func(string) bool) {
		for _, group := range tlsGroups {
			if !yield(string(group)) {
				return
			}
		}
	}), ",")
}

func getMetricsAllowlist(hc *hcov1.HyperConverged) []string {
	if hc.Spec.Observability == nil || hc.Spec.Observability.Workloads == nil {
		return nil
	}
	return slices.Clone(hc.Spec.Observability.Workloads.AllowedMetrics)
}

func getAlertsAllowlist(hc *hcov1.HyperConverged) []string {
	if hc.Spec.Observability == nil {
		return nil
	}
	return slices.Clone(hc.Spec.Observability.AllowedAlerts)
}

func getRecordingRulesAllowlist(hc *hcov1.HyperConverged) []string {
	if hc.Spec.Observability == nil {
		return nil
	}
	return slices.Clone(hc.Spec.Observability.AllowedRecordingRules)
}

func observabilityAllowlistFlag(flagName string, values []string) string {
	if len(values) == 0 {
		return ""
	}
	return flagName + "=" + strings.Join(values, ",")
}
