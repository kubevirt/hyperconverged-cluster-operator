package aie

import (
	"os"
	"strings"

	log "github.com/go-logr/logr"
	openshiftconfigv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/library-go/pkg/crypto"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/tlssecprofile"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

var (
	resourceCPURequest    = resource.MustParse("100m")
	resourceCPULimit      = resource.MustParse("200m")
	resourceMemoryRequest = resource.MustParse("64Mi")
	resourceMemoryLimit   = resource.MustParse("128Mi")
)

func NewAIEWebhookDeploymentHandler(
	_ log.Logger, Client client.Client, Scheme *runtime.Scheme, hc *hcov1beta1.HyperConverged,
) (operands.Operand, error) {
	return operands.NewConditionalHandler(
		operands.NewDeploymentHandler(Client, Scheme, newAIEWebhookDeployment, hc),
		shouldDeployAIE,
		func(hc *hcov1beta1.HyperConverged) client.Object {
			return newAIEWebhookDeploymentWithNameOnly(hc)
		},
	), nil
}

func newAIEWebhookDeploymentWithNameOnly(hc *hcov1beta1.HyperConverged) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      aieWebhookName,
			Namespace: hc.Namespace,
			Labels:    operands.GetLabels(hc, appComponent),
		},
	}
}

func newAIEWebhookDeployment(hc *hcov1beta1.HyperConverged) *appsv1.Deployment {
	image := os.Getenv(hcoutil.AIEWebhookImageEnvV)

	cipherNames, minTLSVersion := tlssecprofile.GetCipherSuitesAndMinTLSVersion(hc.Spec.TLSSecurityProfile)
	ianaCiphers := crypto.OpenSSLToIANACipherSuites(cipherNames)

	selectorLabels := map[string]string{
		hcoutil.AppLabel:          hcoutil.HyperConvergedName,
		hcoutil.AppLabelComponent: string(appComponent),
	}

	podLabels := operands.GetLabels(hc, appComponent)

	args := []string{
		"--metrics-bind-address=:8443",
		"--metrics-secure=true",
		"--webhook-cert-path=" + aieWebhookCertMountPath,
		"--metrics-cert-path=" + aieWebhookCertMountPath,
	}
	if minTLSVersion != "" {
		args = append(args, "--tls-min-version="+string(minTLSVersion))
	}
	if minTLSVersion < openshiftconfigv1.VersionTLS13 && len(ianaCiphers) > 0 {
		args = append(args, "--tls-cipher-suites="+strings.Join(ianaCiphers, ","))
	}

	dep := newAIEWebhookDeploymentWithNameOnly(hc)
	dep.Spec = appsv1.DeploymentSpec{
		Replicas: ptr.To[int32](1),
		Selector: &metav1.LabelSelector{
			MatchLabels: selectorLabels,
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: podLabels,
			},
			Spec: corev1.PodSpec{
				ServiceAccountName: aieWebhookServiceAccountName,
				Containers: []corev1.Container{
					{
						Name:            "webhook",
						Image:           image,
						ImagePullPolicy: corev1.PullIfNotPresent,
						Args:            args,
						Ports: []corev1.ContainerPort{
							{Name: "https", ContainerPort: 9443, Protocol: corev1.ProtocolTCP},
							{Name: "metrics", ContainerPort: 8443, Protocol: corev1.ProtocolTCP},
							{Name: "health", ContainerPort: 8081, Protocol: corev1.ProtocolTCP},
						},
						Env: []corev1.EnvVar{
							{
								Name: "NAMESPACE",
								ValueFrom: &corev1.EnvVarSource{
									FieldRef: &corev1.ObjectFieldSelector{
										FieldPath:  "metadata.namespace",
										APIVersion: "v1",
									},
								},
							},
						},
						LivenessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path:   "/healthz",
									Port:   intstr.FromInt32(8081),
									Scheme: corev1.URISchemeHTTP,
								},
							},
							InitialDelaySeconds: 5,
							PeriodSeconds:       10,
						},
						ReadinessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path:   "/readyz",
									Port:   intstr.FromInt32(8081),
									Scheme: corev1.URISchemeHTTP,
								},
							},
							InitialDelaySeconds: 5,
							PeriodSeconds:       10,
						},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resourceCPURequest,
								corev1.ResourceMemory: resourceMemoryRequest,
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resourceCPULimit,
								corev1.ResourceMemory: resourceMemoryLimit,
							},
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "tls-cert",
								MountPath: aieWebhookCertMountPath,
								ReadOnly:  true,
							},
						},
						SecurityContext: &corev1.SecurityContext{
							AllowPrivilegeEscalation: ptr.To(false),
							ReadOnlyRootFilesystem:   ptr.To(true),
							RunAsNonRoot:             ptr.To(true),
							Capabilities: &corev1.Capabilities{
								Drop: []corev1.Capability{"ALL"},
							},
							SeccompProfile: &corev1.SeccompProfile{
								Type: corev1.SeccompProfileTypeRuntimeDefault,
							},
						},
					},
				},
				Volumes: []corev1.Volume{
					{
						Name: "tls-cert",
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName: aieWebhookTLSSecretName,
							},
						},
					},
				},
			},
		},
	}

	return dep
}
