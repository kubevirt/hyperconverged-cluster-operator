package observabilitycontroller

import (
	"os"
	"strings"

	openshiftconfigv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/library-go/pkg/crypto"
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

var (
	resourceCPURequest    = resource.MustParse("10m")
	resourceMemoryRequest = resource.MustParse("64Mi")
	resourceMemoryLimit   = resource.MustParse("128Mi")
)

func NewDeploymentHandler(cli client.Client, scheme *runtime.Scheme) operands.Operand {
	return operands.NewConditionalHandler(
		operands.NewDeploymentHandler(cli, scheme, newDeployment),
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

	cipherNames, minTLSVersion := tlssecprofile.GetCipherSuitesAndMinTLSVersion(hc.Spec.Security.TLSSecurityProfile)
	ianaCiphers := crypto.OpenSSLToIANACipherSuites(cipherNames)

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
	}
	if minTLSVersion != "" {
		args = append(args, "--tls-min-version="+string(minTLSVersion))
	}
	if minTLSVersion < openshiftconfigv1.VersionTLS13 && len(ianaCiphers) > 0 {
		args = append(args, "--tls-cipher-suites="+strings.Join(ianaCiphers, ","))
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

	return dep
}
