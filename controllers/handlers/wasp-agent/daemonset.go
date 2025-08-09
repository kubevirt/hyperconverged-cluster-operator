package wasp_agent

import (
	"maps"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/nodeinfo"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

const (
	clusterRoleName             = "wasp-cluster"
	verbosity                   = "1"
	AppComponentWaspAgent       = "wasp-agent"
	waspAgentServiceAccountName = "wasp"
	waspAgentSCCName            = "wasp"
	NoOverCommitPercentage      = 100
)

var (
	resourceCPU    = resource.MustParse("100m")
	resourceMemory = resource.MustParse("50M")
)

func NewWaspAgentDaemonSetHandler(Client client.Client, Scheme *runtime.Scheme) operands.Operand {
	return operands.NewConditionalHandler(
		operands.NewDaemonSetHandler(Client, Scheme, newWaspAgentDaemonSet),
		shouldDeployWaspAgent,
		func(hc *hcov1beta1.HyperConverged) client.Object {
			return NewWaspAgentWithNameOnly(hc)
		},
	)
}

func NewWaspAgentWithNameOnly(hc *hcov1beta1.HyperConverged) *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      AppComponentWaspAgent,
			Labels:    operands.GetLabels(hc, AppComponentWaspAgent),
			Namespace: hc.Namespace,
		},
	}
}

func newWaspAgentDaemonSet(hc *hcov1beta1.HyperConverged) *appsv1.DaemonSet {
	waspImage, _ := os.LookupEnv(hcoutil.WaspAgentImageEnvV)

	podLabels := operands.GetLabels(hc, AppComponentWaspAgent)
	podLabels[hcoutil.AllowEgressToDNSAndAPIServerLabel] = "true"
	podLabels["name"] = AppComponentWaspAgent

	container := corev1.Container{
		Name:            AppComponentWaspAgent,
		Image:           waspImage,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resourceCPU,
				corev1.ResourceMemory: resourceMemory,
			},
		},
		SecurityContext: &corev1.SecurityContext{
			Privileged: ptr.To(true),
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "host",
				MountPath: "/host",
			},
			{
				Name:      "rootfs",
				MountPath: "/rootfs",
			},
		},
	}
	container.Env = createDaemonSetEnvVar()

	spec := appsv1.DaemonSetSpec{
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"name": AppComponentWaspAgent,
			},
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: podLabels,
			},
			Spec: corev1.PodSpec{
				ServiceAccountName:            waspAgentServiceAccountName,
				HostPID:                       true,
				HostUsers:                     ptr.To(true),
				TerminationGracePeriodSeconds: ptr.To[int64](5),
				Containers:                    []corev1.Container{container},
				Volumes: []corev1.Volume{
					{
						Name: "host",
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{
								Path: "/",
							},
						},
					},
					{
						Name: "rootfs",
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{
								Path: "/",
							},
						},
					},
				},
				PriorityClassName: "system-node-critical",
			},
		},
	}

	ds := NewWaspAgentWithNameOnly(hc)
	ds.Spec = spec

	if hc.Spec.Infra.NodePlacement != nil {
		if hc.Spec.Infra.NodePlacement.NodeSelector != nil {
			ds.Spec.Template.Spec.NodeSelector = maps.Clone(hc.Spec.Infra.NodePlacement.NodeSelector)
		}

		if hc.Spec.Infra.NodePlacement.Affinity != nil {
			ds.Spec.Template.Spec.Affinity = hc.Spec.Infra.NodePlacement.Affinity.DeepCopy()
		}

		if hc.Spec.Infra.NodePlacement.Tolerations != nil {
			ds.Spec.Template.Spec.Tolerations = make([]corev1.Toleration, len(hc.Spec.Infra.NodePlacement.Tolerations))
			copy(ds.Spec.Template.Spec.Tolerations, hc.Spec.Infra.NodePlacement.Tolerations)
		}
	} else {
		affinity := getPodAntiAffinity(ds.Labels[hcoutil.AppLabelComponent], nodeinfo.IsInfrastructureHighlyAvailable())
		ds.Spec.Template.Spec.Affinity = affinity
	}

	return ds
}

func createDaemonSetEnvVar() []corev1.EnvVar {
	return []corev1.EnvVar{
		{
			Name:  "VERBOSITY",
			Value: verbosity,
		},
		{
			Name: "NODE_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "spec.nodeName",
				},
			},
		},
	}
}

func shouldDeployWaspAgent(hc *hcov1beta1.HyperConverged) bool {
	overcommitPercentage := hc.Spec.HigherWorkloadDensity.MemoryOvercommitPercentage
	return overcommitPercentage > NoOverCommitPercentage
}

func getPodAntiAffinity(componentLabel string, infrastructureHighlyAvailable bool) *corev1.Affinity {
	if infrastructureHighlyAvailable {
		return &corev1.Affinity{
			PodAntiAffinity: &corev1.PodAntiAffinity{
				PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
					{
						Weight: 90,
						PodAffinityTerm: corev1.PodAffinityTerm{
							LabelSelector: &metav1.LabelSelector{
								MatchExpressions: []metav1.LabelSelectorRequirement{
									{
										Key:      hcoutil.AppLabelComponent,
										Operator: metav1.LabelSelectorOpIn,
										Values:   []string{componentLabel},
									},
								},
							},
							TopologyKey: corev1.LabelHostname,
						},
					},
				},
			},
		}
	}

	return nil
}
