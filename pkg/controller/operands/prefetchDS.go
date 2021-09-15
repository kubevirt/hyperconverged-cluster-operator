package operands

import (
	"errors"
	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/pkg/apis/hco/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/controller/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"os"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

func newPrefetchDSHandler(Client client.Client, Scheme *runtime.Scheme) Operand {
	h := &genericOperand{
		Client:                 Client,
		Scheme:                 Scheme,
		crType:                 "HCOPrefetchDaemonSet",
		removeExistingOwner:    false,
		setControllerReference: true,
		hooks:                  &prefetchDSHooks{},
	}

	return h
}

type prefetchDSHooks struct {
	cache *appsv1.DaemonSet
}

func (h *prefetchDSHooks) getFullCr(hc *hcov1beta1.HyperConverged) (client.Object, error) {
	if h.cache == nil {
		pds, err := NewPrefetchDS(hc)
		if err != nil {
			return nil, err
		}
		h.cache = pds
	}
	return h.cache, nil
}

func (h *prefetchDSHooks) reset() {
	h.cache = nil
}

func (h *prefetchDSHooks) getEmptyCr() client.Object {
	return &appsv1.DaemonSet{}
}

func (h *prefetchDSHooks) getObjectMeta(cr runtime.Object) *metav1.ObjectMeta {
	return &cr.(*appsv1.DaemonSet).ObjectMeta
}

func (h *prefetchDSHooks) updateCr(req *common.HcoRequest, Client client.Client, exists runtime.Object, required runtime.Object) (bool, bool, error) {

	prefetchDS, ok1 := required.(*appsv1.DaemonSet)
	found, ok2 := exists.(*appsv1.DaemonSet)

	if !ok1 || !ok2 {
		return false, false, errors.New("can't convert to DaemonSet")
	}

	if !reflect.DeepEqual(found.Spec, prefetchDS.Spec) ||
		!reflect.DeepEqual(found.Labels, prefetchDS.Labels) {
		if req.HCOTriggered {
			req.Logger.Info("Updating existing HCO prefetch DaemonSet's Spec to new opinionated values", "name", prefetchDS.Name)
		} else {
			req.Logger.Info("Reconciling an externally updated HCO prefetch DaemonSet's Spec to its opinionated values", "name", prefetchDS.Name)
		}
		util.DeepCopyLabels(&prefetchDS.ObjectMeta, &found.ObjectMeta)
		prefetchDS.Spec.DeepCopyInto(&found.Spec)
		err := Client.Update(req.Ctx, found)
		if err != nil {
			return false, false, err
		}
		return true, !req.HCOTriggered, nil
	}

	return false, false, nil
}

func NewPrefetchDS(hc *hcov1beta1.HyperConverged, opts ...string) (*appsv1.DaemonSet, error) {
	placementWorkloads := hc.Spec.Workloads.NodePlacement
	labels := getLabels(hc, hcoutil.AppComponentDeployment)
	prefetchDSSpec := appsv1.DaemonSetSpec{
		Selector: &metav1.LabelSelector{
			MatchLabels: labels,
		},
		UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
			Type: appsv1.RollingUpdateDaemonSetStrategyType,
			RollingUpdate: &appsv1.RollingUpdateDaemonSet{
				MaxUnavailable: &intstr.IntOrString{
					Type:   intstr.String,
					StrVal: "50%",
				},
			},
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: labels,
			},
			Spec: corev1.PodSpec{
				ServiceAccountName: hcov1beta1.HyperConvergedName,
				InitContainers: []corev1.Container{
					{
						Name:            hcov1beta1.HyperConvergedName + "-prefetch-virt-handler",
						Image:           os.Getenv(util.VirtHandlerImageV),
						ImagePullPolicy: corev1.PullPolicy("IfNotPresent"),
						Command:         []string{"virt-handler", "--help"},
						Resources: v1.ResourceRequirements{
							Requests: map[v1.ResourceName]resource.Quantity{
								v1.ResourceCPU:    resource.MustParse("5m"),
								v1.ResourceMemory: resource.MustParse("48Mi"),
							},
						},
					},
					// TODO: add an entry for each image we want to prefetch
				},
				Containers: []corev1.Container{
					{
						Name:            hcov1beta1.HyperConvergedName + "-prefetch",
						Image:           os.Getenv(util.VirtHandlerImageV),
						ImagePullPolicy: corev1.PullPolicy("IfNotPresent"),
						Command:         []string{"sleep", "infinity"},
						Resources: v1.ResourceRequirements{
							Requests: map[v1.ResourceName]resource.Quantity{
								v1.ResourceCPU:    resource.MustParse("5m"),
								v1.ResourceMemory: resource.MustParse("48Mi"),
							},
						},
					},
				},
				PriorityClassName: "system-cluster-critical",
			},
		},
	}

	if placementWorkloads != nil {
		prefetchDSSpec.Template.Spec.NodeSelector = placementWorkloads.NodeSelector
		prefetchDSSpec.Template.Spec.Tolerations = placementWorkloads.Tolerations
		prefetchDSSpec.Template.Spec.Affinity = placementWorkloads.Affinity
	}

	pds := NewPrefetchDSWithNameOnly(hc, opts...)
	pds.Spec = prefetchDSSpec

	return pds, nil
}

func NewPrefetchDSWithNameOnly(hc *hcov1beta1.HyperConverged, opts ...string) *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "prefetchds-" + hc.Name,
			Labels:    getLabels(hc, hcoutil.AppComponentDeployment),
			Namespace: getNamespace(hc.Namespace, opts),
		},
	}
}
