package passt

import (
	"fmt"
	"maps"
	"os"
	"sync"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubevirtcorev1 "kubevirt.io/api/core/v1"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/nodeinfo"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

const (
	DeployPasstNetworkBindingAnnotation = "deployPasstNetworkBinding"

	BindingName = "passt"

	networkBindingNADName       = "primary-udn-kubevirt-binding"
	networkBindingNADNamespace  = "default"
	NetworkAttachmentDefinition = networkBindingNADNamespace + "/" + networkBindingNADName

	bindingComputeMemoryOverhead = "500Mi"
)

var (
	passtResourceMemory = resource.MustParse(bindingComputeMemoryOverhead)
	passtImage          string
	passtImageOnce      sync.Once
)

// CheckPasstImagesEnvExists checks if the passt image environment variable exists
func CheckPasstImagesEnvExists() error {
	if _, passtImageVarExists := os.LookupEnv(hcoutil.PasstImageEnvV); !passtImageVarExists {
		return fmt.Errorf("the %s environment variable must be set", hcoutil.PasstImageEnvV)
	}
	if _, passtCNIImageVarExists := os.LookupEnv(hcoutil.PasstCNIImageEnvV); !passtCNIImageVarExists {
		return fmt.Errorf("the %s environment variable must be set", hcoutil.PasstCNIImageEnvV)
	}
	return nil
}

// NetworkBinding creates an InterfaceBindingPlugin for passt network binding
func NetworkBinding(sidecarImage string) kubevirtcorev1.InterfaceBindingPlugin {
	return kubevirtcorev1.InterfaceBindingPlugin{
		NetworkAttachmentDefinition: NetworkAttachmentDefinition,
		SidecarImage:                sidecarImage,
		Migration:                   &kubevirtcorev1.InterfaceBindingMigration{},
		ComputeResourceOverhead: &kubevirtcorev1.ResourceRequirementsWithoutClaims{
			Requests: corev1.ResourceList{
				corev1.ResourceMemory: passtResourceMemory,
			},
		},
	}
}

// NewPasstBindingCNISA creates a ServiceAccount for the passt binding CNI
func NewPasstBindingCNISA(hc *hcov1beta1.HyperConverged) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "passt-binding-cni",
			Namespace: hc.Namespace,
			Labels:    hcoutil.GetLabels(hcoutil.HyperConvergedName, hcoutil.AppComponentNetwork),
		},
	}
}

// NewPasstBindingCNIDaemonSet creates a DaemonSet for the passt binding CNI
func NewPasstBindingCNIDaemonSet(hc *hcov1beta1.HyperConverged, isOpenShift bool) *appsv1.DaemonSet {
	maxUnavailable := intstr.FromString("10%")

	labels := hcoutil.GetLabels(hcoutil.HyperConvergedName, hcoutil.AppComponentNetwork)
	labels["tier"] = "node"

	infrastructureHighlyAvailable := nodeinfo.IsInfrastructureHighlyAvailable()
	affinity := getPodAntiAffinity(labels[hcoutil.AppLabelComponent], infrastructureHighlyAvailable)

	hostpath := "/opt/cni/bin"
	if isOpenShift {
		hostpath = "/var/lib/cni/bin"
	}

	daemonSet := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "passt-binding-cni",
			Namespace: hc.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"name": "passt-binding-cni",
				},
			},
			UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
				Type: appsv1.RollingUpdateDaemonSetStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDaemonSet{
					MaxUnavailable: &maxUnavailable,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"name": "passt-binding-cni",
						"tier": "node",
						"app":  "passt-binding-cni",
					},
					Annotations: map[string]string{
						"description": "passt-binding-cni installs 'passt binding' CNI on cluster nodes",
					},
				},
				Spec: corev1.PodSpec{
					PriorityClassName:  "system-cluster-critical",
					ServiceAccountName: "passt-binding-cni",
					Containers: []corev1.Container{
						{
							Name:  "installer",
							Image: os.Getenv(hcoutil.PasstCNIImageEnvV),
							Command: []string{
								"/bin/sh",
								"-ce",
							},
							Args: []string{
								`ls -la "/cni/kubevirt-passt-binding"
cp -f "/cni/kubevirt-passt-binding" "/opt/cni/bin"
echo "passt binding CNI plugin installation complete..sleep infinity"
sleep 2147483647`,
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("15Mi"),
								},
							},
							SecurityContext: &corev1.SecurityContext{
								Privileged: ptr.To(true),
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "cnibin",
									MountPath: "/opt/cni/bin",
								},
							},
							ImagePullPolicy: corev1.PullIfNotPresent,
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "cnibin",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: hostpath,
								},
							},
						},
					},
				},
			},
		},
	}

	if hc.Spec.Infra.NodePlacement != nil {
		if hc.Spec.Infra.NodePlacement.NodeSelector != nil {
			daemonSet.Spec.Template.Spec.NodeSelector = maps.Clone(hc.Spec.Infra.NodePlacement.NodeSelector)
		} else {
			daemonSet.Spec.Template.Spec.NodeSelector = nil
		}

		if hc.Spec.Infra.NodePlacement.Affinity != nil {
			daemonSet.Spec.Template.Spec.Affinity = hc.Spec.Infra.NodePlacement.Affinity.DeepCopy()
		} else {
			daemonSet.Spec.Template.Spec.Affinity = affinity
		}

		if hc.Spec.Infra.NodePlacement.Tolerations != nil {
			daemonSet.Spec.Template.Spec.Tolerations = make([]corev1.Toleration, len(hc.Spec.Infra.NodePlacement.Tolerations))
			copy(daemonSet.Spec.Template.Spec.Tolerations, hc.Spec.Infra.NodePlacement.Tolerations)
		} else {
			daemonSet.Spec.Template.Spec.Tolerations = nil
		}
	} else {
		daemonSet.Spec.Template.Spec.NodeSelector = nil
		daemonSet.Spec.Template.Spec.Affinity = affinity
		daemonSet.Spec.Template.Spec.Tolerations = nil
	}

	return daemonSet
}

// GetImage gets the passt image from environment variable
func GetImage() string {
	passtImageOnce.Do(func() {
		passtImage = os.Getenv(hcoutil.PasstImageEnvV)
	})
	return passtImage
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

// NewPasstServiceAccountHandler creates a conditional handler for passt ServiceAccount
func NewPasstServiceAccountHandler(Client client.Client, Scheme *runtime.Scheme) operands.Operand {
	return operands.NewConditionalHandler(
		operands.NewServiceAccountHandler(Client, Scheme, NewPasstBindingCNISA),
		func(hc *hcov1beta1.HyperConverged) bool {
			value, ok := hc.Annotations[DeployPasstNetworkBindingAnnotation]
			return ok && value == "true"
		},
		func(hc *hcov1beta1.HyperConverged) client.Object {
			return NewPasstBindingCNISA(hc)
		},
	)
}

// NewPasstDaemonSetHandler creates a conditional handler for passt DaemonSet
func NewPasstDaemonSetHandler(Client client.Client, Scheme *runtime.Scheme, isOpenShift bool) operands.Operand {
	return operands.NewConditionalHandler(
		operands.NewDaemonSetHandler(Client, Scheme, isOpenShift, NewPasstBindingCNIDaemonSet),
		func(hc *hcov1beta1.HyperConverged) bool {
			value, ok := hc.Annotations[DeployPasstNetworkBindingAnnotation]
			return ok && value == "true"
		},
		func(hc *hcov1beta1.HyperConverged) client.Object {
			return NewPasstBindingCNIDaemonSet(hc, isOpenShift)
		},
	)
}
