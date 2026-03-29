package aie_webhook

import (
	"os"

	log "github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

const (
	iommufdDevicePluginName               = "iommufd-device-plugin"
	iommufdDevicePluginServiceAccountName = "iommufd-device-plugin"
	iommufdDevicePluginAppComponent       = hcoutil.AppComponentIOMMUFDDevicePlugin
)

func NewIOMMUFDDevicePluginDaemonSetHandler(
	_ log.Logger, Client client.Client, Scheme *runtime.Scheme, hc *hcov1beta1.HyperConverged,
) (operands.Operand, error) {
	return operands.NewConditionalHandler(
		operands.NewDaemonSetHandler(Client, Scheme, newIOMMUFDDevicePluginDaemonSet),
		shouldDeployAIEWebhook,
		func(hc *hcov1beta1.HyperConverged) client.Object {
			return NewIOMMUFDDevicePluginDaemonSetWithNameOnly(hc)
		},
	), nil
}

func NewIOMMUFDDevicePluginDaemonSetWithNameOnly(hc *hcov1beta1.HyperConverged) *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      iommufdDevicePluginName,
			Namespace: hc.Namespace,
			Labels:    operands.GetLabels(hc, iommufdDevicePluginAppComponent),
		},
	}
}

func newIOMMUFDDevicePluginDaemonSet(hc *hcov1beta1.HyperConverged) *appsv1.DaemonSet {
	image := os.Getenv(hcoutil.IOMMUFDDevicePluginImageEnvV)

	selectorLabels := map[string]string{
		hcoutil.AppLabel:          hcoutil.HyperConvergedName,
		hcoutil.AppLabelComponent: string(iommufdDevicePluginAppComponent),
	}

	podLabels := operands.GetLabels(hc, iommufdDevicePluginAppComponent)

	ds := NewIOMMUFDDevicePluginDaemonSetWithNameOnly(hc)
	ds.Spec = appsv1.DaemonSetSpec{
		Selector: &metav1.LabelSelector{
			MatchLabels: selectorLabels,
		},
		UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
			Type: appsv1.RollingUpdateDaemonSetStrategyType,
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: podLabels,
			},
			Spec: corev1.PodSpec{
				ServiceAccountName: iommufdDevicePluginServiceAccountName,
				Containers: []corev1.Container{
					{
						Name:            iommufdDevicePluginName,
						Image:           image,
						ImagePullPolicy: corev1.PullIfNotPresent,
						Args: []string{
							"-log-level=info",
							"-socket-dir=/var/run/kubevirt/fd-sockets",
						},
						SecurityContext: &corev1.SecurityContext{
							Privileged: ptr.To(true),
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "device-plugins",
								MountPath: "/var/lib/kubelet/device-plugins",
							},
							{
								Name:      "dev",
								MountPath: "/dev",
							},
							{
								Name:      "fd-sockets",
								MountPath: "/var/run/kubevirt/fd-sockets",
							},
						},
					},
				},
				Volumes: []corev1.Volume{
					{
						Name: "device-plugins",
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{
								Path: "/var/lib/kubelet/device-plugins",
							},
						},
					},
					{
						Name: "dev",
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{
								Path: "/dev",
							},
						},
					},
					{
						Name: "fd-sockets",
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{
								Path: "/var/run/kubevirt/fd-sockets",
								Type: ptr.To(corev1.HostPathDirectoryOrCreate),
							},
						},
					},
				},
				PriorityClassName: "system-node-critical",
			},
		},
	}

	return ds
}
