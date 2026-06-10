package netresinjector

import (
	"os"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/nodeinfo"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

var (
	resourceCPURequest    = resource.MustParse("10m")
	resourceMemoryRequest = resource.MustParse("50Mi")
)

func NewDeploymentHandler(cli client.Client, scheme *runtime.Scheme) operands.Operand {
	return operands.NewDeploymentHandler(cli, scheme, newDeployment)
}

func NewDeploymentWithNameOnly() *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: hcoutil.GetOperatorNamespaceFromEnv(),
			Labels:    operands.GetLabels(hcoutil.AppComponentNetResInjector),
		},
	}
}

func newDeployment(_ *hcov1.HyperConverged) *appsv1.Deployment {
	image := os.Getenv(hcoutil.NetworkResourcesInjectorImageEnvV)

	selectorLabels := map[string]string{
		hcoutil.AppLabel:          hcoutil.HyperConvergedName,
		hcoutil.AppLabelComponent: string(hcoutil.AppComponentNetResInjector),
	}

	podLabels := operands.GetLabels(hcoutil.AppComponentNetResInjector)

	infrastructureHighlyAvailable := nodeinfo.IsInfrastructureHighlyAvailable()
	affinity := operands.GetPodAntiAffinity(podLabels[hcoutil.AppLabelComponent], infrastructureHighlyAvailable)

	dep := NewDeploymentWithNameOnly()
	dep.Spec = appsv1.DeploymentSpec{
		Replicas: new(int32(2)),
		Selector: &metav1.LabelSelector{
			MatchLabels: selectorLabels,
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: podLabels,
				Annotations: map[string]string{
					"openshift.io/required-scc": "restricted-v2",
				},
			},
			Spec: corev1.PodSpec{
				ServiceAccountName: serviceAccountName,
				PriorityClassName:  "system-cluster-critical",
				Affinity:           affinity,
				SecurityContext: &corev1.PodSecurityContext{
					RunAsNonRoot: new(true),
				},
				Containers: []corev1.Container{
					{
						Name:    "webhook-server",
						Image:   image,
						Command: []string{"webhook"},
						Args: []string{
							"-bind-address=0.0.0.0",
							"-port=6443",
							"-tls-private-key-file=" + tlsMountPath + "/tls.key",
							"-tls-cert-file=" + tlsMountPath + "/tls.crt",
							"-insecure=true",
							"-logtostderr=true",
							"-alsologtostderr=true",
						},
						Env: []corev1.EnvVar{
							{
								Name: "NAMESPACE",
								ValueFrom: &corev1.EnvVarSource{
									FieldRef: &corev1.ObjectFieldSelector{
										APIVersion: "v1",
										FieldPath:  "metadata.namespace",
									},
								},
							},
						},
						ImagePullPolicy: corev1.PullIfNotPresent,
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resourceCPURequest,
								corev1.ResourceMemory: resourceMemoryRequest,
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
						TerminationMessagePath:   corev1.TerminationMessagePathDefault,
						TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "tls",
								MountPath: tlsMountPath,
								ReadOnly:  true,
							},
						},
					},
				},
				Volumes: []corev1.Volume{
					{
						Name: "tls",
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName: tlsSecretName,
							},
						},
					},
				},
			},
		},
	}

	return dep
}
