package netresinjector

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
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/nodeinfo"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/tlssecprofile"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

var (
	resourceCPURequest    = resource.MustParse("10m")
	resourceMemoryRequest = resource.MustParse("50Mi")
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
			Labels:    operands.GetLabels(hcoutil.AppComponentNetResInjector),
		},
	}
}

func newDeployment(hc *hcov1.HyperConverged) *appsv1.Deployment {
	image := os.Getenv(hcoutil.NetworkResourcesInjectorImageEnvV)

	cipherNames, minTLSVersion := tlssecprofile.GetCipherSuitesAndMinTLSVersion(hc.Spec.Security.TLSSecurityProfile)
	ianaCiphers := crypto.OpenSSLToIANACipherSuites(cipherNames)

	var replicas int32
	if nodeinfo.IsInfrastructureHighlyAvailable() {
		replicas = int32(2)
	} else {
		replicas = int32(1)
	}

	selectorLabels := map[string]string{
		hcoutil.AppLabel:          hcoutil.HyperConvergedName,
		hcoutil.AppLabelComponent: string(hcoutil.AppComponentNetResInjector),
	}

	podLabels := operands.GetLabels(hcoutil.AppComponentNetResInjector)

	// Configure affinity to prefer control plane nodes
	// Use PreferredDuringScheduling to support HyperShift (worker-only) and SNO clusters
	affinity := &corev1.Affinity{
		NodeAffinity: &corev1.NodeAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.PreferredSchedulingTerm{
				// Prefer standard Kubernetes control-plane nodes
				{
					Preference: corev1.NodeSelectorTerm{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{
								Key:      nodeinfo.LabelNodeRoleControlPlane,
								Operator: corev1.NodeSelectorOpExists,
							},
						},
					},
					Weight: 100,
				},
				// Prefer KubeVirt control-plane nodes (HyperShift)
				{
					Preference: corev1.NodeSelectorTerm{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{
								Key:      nodeinfo.LabelNodeRoleKubevirtControlPlane,
								Operator: corev1.NodeSelectorOpExists,
							},
						},
					},
					Weight: 100,
				},
				// Avoid worker-only nodes when control plane nodes are available
				{
					Preference: corev1.NodeSelectorTerm{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{
								Key:      nodeinfo.LabelNodeRoleWorker,
								Operator: corev1.NodeSelectorOpDoesNotExist,
							},
						},
					},
					Weight: 50,
				},
			},
		},
		PodAntiAffinity: &corev1.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
				{
					PodAffinityTerm: corev1.PodAffinityTerm{
						LabelSelector: &metav1.LabelSelector{
							MatchExpressions: []metav1.LabelSelectorRequirement{
								{
									Key:      hcoutil.AppLabelComponent,
									Operator: metav1.LabelSelectorOpIn,
									Values:   []string{string(hcoutil.AppComponentNetResInjector)},
								},
							},
						},
						TopologyKey: corev1.LabelHostname,
					},
					Weight: 1,
				},
			},
		},
	}

	dep := NewDeploymentWithNameOnly()
	dep.Spec = appsv1.DeploymentSpec{
		Replicas: &replicas,
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
				NodeSelector: map[string]string{
					"kubernetes.io/os": "linux",
				},
				Tolerations: []corev1.Toleration{
					{
						Key:      "CriticalAddonsOnly",
						Operator: corev1.TolerationOpExists,
					},
					{
						Effect:   corev1.TaintEffectNoSchedule,
						Key:      nodeinfo.LabelNodeRoleControlPlane,
						Operator: corev1.TolerationOpExists,
					},
				},
				SecurityContext: &corev1.PodSecurityContext{
					RunAsNonRoot: new(true),
				},
				Containers: []corev1.Container{
					{
						Name:    "webhook-server",
						Image:   image,
						Command: []string{"webhook"},
						Args:    tlsArgs(minTLSVersion, ianaCiphers),
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

func tlsArgs(minTLSVersion openshiftconfigv1.TLSProtocolVersion, ianaCiphers []string) []string {
	args := []string{
		"-bind-address=0.0.0.0",
		"-port=6443",
		"-tls-private-key-file=" + tlsMountPath + "/tls.key",
		"-tls-cert-file=" + tlsMountPath + "/tls.crt",
		"-insecure=true",
		"-logtostderr=true",
		"-alsologtostderr=true",
	}

	if minTLSVersion != "" {
		args = append(args, "-tls-min-version="+string(minTLSVersion))
	}
	if minTLSVersion < openshiftconfigv1.VersionTLS13 && len(ianaCiphers) > 0 {
		args = append(args, "-tls-cipher-suites="+strings.Join(ianaCiphers, ","))
	}

	return args
}
