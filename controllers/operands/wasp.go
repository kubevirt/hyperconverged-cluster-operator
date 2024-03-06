package operands

import (
	"errors"
	"maps"
	"os"
	"reflect"

	"k8s.io/apimachinery/pkg/util/intstr"

	rbacv1 "k8s.io/api/rbac/v1"

	sdkapi "kubevirt.io/controller-lifecycle-operator-sdk/api"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

const (
	// For simplicity matters, re-use the virt-handler SA. That will spare
	// the HCO controller from deploying custom privileged SA with SCC.
	waspServiceAccount = "kubevirt-handler"
	// Set dry run mode for e2e testing purposes
	waspDryRunAnnotation = "wasp.hyperconverged.io/dry-run"
)

func newWaspHandler(Client client.Client, Scheme *runtime.Scheme) Operand {
	return &conditionalHandler{
		operand: &genericOperand{
			Client: Client,
			Scheme: Scheme,
			crType: "DaemonSet",
			hooks:  &waspHooks{},
		},
		getCRWithName: func(hc *hcov1beta1.HyperConverged) client.Object {
			return NewWaspWithNameOnly(hc)
		},
		shouldDeploy: func(hc *hcov1beta1.HyperConverged) bool {
			// convert from anon to private
			return hc.Spec.FeatureGates.EnableHigherDensityWithSwap != nil &&
				*hc.Spec.FeatureGates.EnableHigherDensityWithSwap
		},
	}
}

type waspHooks struct {
	cache *appsv1.DaemonSet
}

func (h *waspHooks) getFullCr(hc *hcov1beta1.HyperConverged) (client.Object, error) {
	if h.cache == nil {
		h.cache = NewWasp(hc)
	}

	return h.cache, nil
}

func (*waspHooks) getEmptyCr() client.Object { return &appsv1.DaemonSet{} }

func (*waspHooks) updateCr(
	req *common.HcoRequest,
	Client client.Client,
	exists runtime.Object,
	required runtime.Object) (bool, bool, error) {
	daemonset, ok1 := required.(*appsv1.DaemonSet)
	found, ok2 := exists.(*appsv1.DaemonSet)

	if !ok1 || !ok2 {
		return false, false, errors.New("can't convert to Daemonset")
	}
	if !hasCorrectDaemonSetFields(found, daemonset) {
		if req.HCOTriggered {
			req.Logger.Info("Updating existing wasp daemonset to new opinionated values", "name", daemonset.Name)
		} else {
			req.Logger.Info("Reconciling an externally updated wasp daemonset to its opinionated values", "name", daemonset.Name)
		}
		if shouldRecreateDaemonset(found, daemonset) {
			err := Client.Delete(req.Ctx, found, &client.DeleteOptions{})
			if err != nil {
				return false, false, err
			}
			err = Client.Create(req.Ctx, daemonset, &client.CreateOptions{})
			if err != nil {
				return false, false, err
			}
			return true, !req.HCOTriggered, nil
		}
		hcoutil.MergeLabels(&daemonset.ObjectMeta, &found.ObjectMeta)
		daemonset.Spec.DeepCopyInto(&found.Spec)
		err := Client.Update(req.Ctx, found)
		if err != nil {
			return false, false, err
		}
		return true, !req.HCOTriggered, nil
	}

	return false, false, nil
}

func (*waspHooks) justBeforeComplete(_ *common.HcoRequest) { /* no implementation */ }

func (*waspHooks) getConditions(cr runtime.Object) []metav1.Condition {
	return dsConditionsToK8s(cr.(*appsv1.DaemonSet).Status.Conditions)
}

func NewWasp(hc *hcov1beta1.HyperConverged) *appsv1.DaemonSet {
	waspImage, _ := os.LookupEnv(hcoutil.WaspImageEnvV)
	wasp := NewWaspWithNameOnly(hc)
	wasp.Namespace = hc.Namespace
	wasp.Spec = appsv1.DaemonSetSpec{
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"name": "wasp",
			},
		},
		UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
			Type: appsv1.RollingUpdateDaemonSetStrategyType,
			RollingUpdate: &appsv1.RollingUpdateDaemonSet{
				MaxUnavailable: ptr.To(intstr.FromString("10%")),
				MaxSurge:       ptr.To(intstr.FromString("0%")),
			},
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"name": "wasp",
				},
			},
			Spec: corev1.PodSpec{
				RestartPolicy:                 corev1.RestartPolicyAlways,
				ServiceAccountName:            waspServiceAccount,
				HostPID:                       true,
				HostUsers:                     ptr.To(true),
				TerminationGracePeriodSeconds: ptr.To(int64(30)),
				Containers: []corev1.Container{
					{
						Name:            "wasp-agent",
						Image:           waspImage,
						ImagePullPolicy: corev1.PullIfNotPresent,
						Env: []corev1.EnvVar{
							{
								Name:  "FSROOT",
								Value: "/host",
							},
							{
								Name:  "SWAPINNES",
								Value: "5",
							},
							{
								Name:  "LOOP",
								Value: "true",
							},
							{
								Name:  "DEBUG",
								Value: "",
							},
						},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("100m"),
								corev1.ResourceMemory: resource.MustParse("50M"),
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
						},
					},
				},
				Volumes: []corev1.Volume{
					{
						Name: "host",
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{
								Path: "/",
							},
						},
					},
				},
			},
		},
	}

	injectPlacement(hc.Spec.Workloads.NodePlacement, &wasp.Spec)
	if _, ok := hc.Annotations[waspDryRunAnnotation]; ok {
		wasp.Spec.Template.Spec.Containers[0].Env =
			append(wasp.Spec.Template.Spec.Containers[0].Env,
				corev1.EnvVar{
					Name:  "DRY_RUN",
					Value: "true",
				})
	}
	return wasp
}

func NewWaspWithNameOnly(hc *hcov1beta1.HyperConverged) *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "wasp-agent",
			Labels:    getLabels(hc, hcoutil.AppComponentWasp),
			Namespace: hc.Namespace,
		},
	}
}

func dsConditionsToK8s(conditions []appsv1.DaemonSetCondition) []metav1.Condition {
	if len(conditions) == 0 {
		return nil
	}

	newCond := make([]metav1.Condition, len(conditions))
	for i, c := range conditions {
		newCond[i] = metav1.Condition{
			Type:    string(c.Type),
			Reason:  c.Reason,
			Status:  metav1.ConditionStatus(c.Status),
			Message: c.Message,
		}
	}

	return newCond
}

// We need to check only certain fields in the daemonset resource, since some of the fields
// are being set by k8s.
func hasCorrectDaemonSetFields(found *appsv1.DaemonSet, required *appsv1.DaemonSet) bool {
	return hcoutil.CompareLabels(found, required) &&
		reflect.DeepEqual(found.Spec.Selector, required.Spec.Selector) &&
		reflect.DeepEqual(found.Spec.Template.Spec.Containers, required.Spec.Template.Spec.Containers) &&
		reflect.DeepEqual(found.Spec.Template.Spec.ServiceAccountName, required.Spec.Template.Spec.ServiceAccountName) &&
		reflect.DeepEqual(found.Spec.Template.Spec.PriorityClassName, required.Spec.Template.Spec.PriorityClassName) &&
		reflect.DeepEqual(found.Spec.Template.Spec.Affinity, required.Spec.Template.Spec.Affinity) &&
		reflect.DeepEqual(found.Spec.Template.Spec.NodeSelector, required.Spec.Template.Spec.NodeSelector) &&
		reflect.DeepEqual(found.Spec.Template.Spec.Tolerations, required.Spec.Template.Spec.Tolerations)
}

func injectPlacement(nodePlacement *sdkapi.NodePlacement, spec *appsv1.DaemonSetSpec) {
	spec.Template.Spec.NodeSelector = nil
	spec.Template.Spec.Affinity = nil
	spec.Template.Spec.Tolerations = nil

	if nodePlacement != nil {
		if nodePlacement.NodeSelector != nil {
			spec.Template.Spec.NodeSelector = maps.Clone(nodePlacement.NodeSelector)
		}
		if nodePlacement.Affinity != nil {
			spec.Template.Spec.Affinity = nodePlacement.Affinity.DeepCopy()
		}
		if nodePlacement.Tolerations != nil {
			spec.Template.Spec.Tolerations = make([]corev1.Toleration, len(nodePlacement.Tolerations))
			copy(spec.Template.Spec.Tolerations, nodePlacement.Tolerations)
		}
	}
}
func shouldRecreateDaemonset(found, required *appsv1.DaemonSet) bool {
	// updating LabelSelector (it's immutable) would be rejected by API server; create new Deployment instead
	return !reflect.DeepEqual(found.Spec.Selector, required.Spec.Selector)
}

func newWaspClusterRoleHandler(Client client.Client, Scheme *runtime.Scheme) Operand {
	return &conditionalHandler{
		operand: &genericOperand{
			Client: Client,
			Scheme: Scheme,
			crType: "ClusterRole",
			hooks:  &waspClusterRoleHooks{},
		},
		getCRWithName: func(hc *hcov1beta1.HyperConverged) client.Object {
			return NewWaspClusterRoleWithNameOnly(hc)
		},
		shouldDeploy: func(hc *hcov1beta1.HyperConverged) bool {
			return hc.Spec.FeatureGates.EnableHigherDensityWithSwap != nil &&
				*hc.Spec.FeatureGates.EnableHigherDensityWithSwap
		},
	}
}

type waspClusterRoleHooks struct {
	required *rbacv1.ClusterRole
}

func (h *waspClusterRoleHooks) getFullCr(hc *hcov1beta1.HyperConverged) (client.Object, error) {
	if h.required == nil {
		h.required = newWaspClusterRole(hc)
	}

	return h.required, nil
}

func (h *waspClusterRoleHooks) getEmptyCr() client.Object { return &rbacv1.ClusterRole{} }

func (h *waspClusterRoleHooks) updateCr(req *common.HcoRequest, Client client.Client, exists runtime.Object, _ runtime.Object) (bool, bool, error) {
	clusterRole := h.required
	found, ok := exists.(*rbacv1.ClusterRole)
	if !ok {
		return false, false, errors.New("can't convert to a ClusterRole")
	}

	if !hcoutil.CompareLabels(clusterRole, found) ||
		!reflect.DeepEqual(clusterRole.Rules, found.Rules) {

		req.Logger.Info("Updating existing Role to its default values", "name", found.Name)

		found.Rules = make([]rbacv1.PolicyRule, len(clusterRole.Rules))
		for i := range clusterRole.Rules {
			clusterRole.Rules[i].DeepCopyInto(&found.Rules[i])
		}
		hcoutil.MergeLabels(&clusterRole.ObjectMeta, &found.ObjectMeta)

		err := Client.Update(req.Ctx, found)
		if err != nil {
			return false, false, err
		}
		return true, !req.HCOTriggered, nil
	}

	return false, false, nil
}

func (*waspClusterRoleHooks) justBeforeComplete(_ *common.HcoRequest) { /* no implementation */ }

func newWaspClusterRole(hc *hcov1beta1.HyperConverged) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "wasp-agent",
			Labels: getLabels(hc, hcoutil.AppComponentWasp),
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"pods"},
				Verbs:     []string{"get", "list", "watch"},
			},
		},
	}
}

func NewWaspClusterRoleWithNameOnly(hc *hcov1beta1.HyperConverged) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "wasp-agent",
			Labels: getLabels(hc, hcoutil.AppComponentWasp),
		},
	}
}

func newWaspClusterRoleBindingHandler(Client client.Client, Scheme *runtime.Scheme) Operand {
	return &conditionalHandler{
		operand: &genericOperand{
			Client: Client,
			Scheme: Scheme,
			crType: "ClusterRoleBinding",
			hooks:  &waspClusterRoleBindingHooks{},
		},
		getCRWithName: func(hc *hcov1beta1.HyperConverged) client.Object {
			return NewWaspClusterRoleBindingWithNameOnly(hc)
		},
		shouldDeploy: func(hc *hcov1beta1.HyperConverged) bool {
			return hc.Spec.FeatureGates.EnableHigherDensityWithSwap != nil &&
				*hc.Spec.FeatureGates.EnableHigherDensityWithSwap
		},
	}
}

type waspClusterRoleBindingHooks struct {
	required *rbacv1.ClusterRoleBinding
}

func (h *waspClusterRoleBindingHooks) getFullCr(hc *hcov1beta1.HyperConverged) (client.Object, error) {
	if h.required == nil {
		h.required = newWaspClusterRoleBinding(hc)
	}

	return h.required, nil
}
func (h *waspClusterRoleBindingHooks) getEmptyCr() client.Object { return &rbacv1.ClusterRoleBinding{} }

func (h *waspClusterRoleBindingHooks) updateCr(req *common.HcoRequest, Client client.Client, exists runtime.Object, _ runtime.Object) (bool, bool, error) {
	clusterRoleBinding := h.required
	found, ok := exists.(*rbacv1.ClusterRoleBinding)
	if !ok {
		return false, false, errors.New("can't convert to a ClusterRoleBinding")
	}

	if !hcoutil.CompareLabels(clusterRoleBinding, found) ||
		!reflect.DeepEqual(clusterRoleBinding.Subjects, found.Subjects) ||
		!reflect.DeepEqual(clusterRoleBinding.RoleRef, found.RoleRef) {
		req.Logger.Info("Updating existing ClusterRoleBinding to its default values", "name", found.Name)

		found.Subjects = make([]rbacv1.Subject, len(clusterRoleBinding.Subjects))
		copy(found.Subjects, clusterRoleBinding.Subjects)
		found.RoleRef = clusterRoleBinding.RoleRef
		hcoutil.MergeLabels(&clusterRoleBinding.ObjectMeta, &found.ObjectMeta)

		err := Client.Update(req.Ctx, found)
		if err != nil {
			return false, false, err
		}
		return true, !req.HCOTriggered, nil
	}

	return false, false, nil
}

func (*waspClusterRoleBindingHooks) justBeforeComplete(_ *common.HcoRequest) { /* no implementation */ }

func newWaspClusterRoleBinding(hc *hcov1beta1.HyperConverged) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "wasp-agent",
			Labels: getLabels(hc, hcoutil.AppComponentWasp),
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "wasp-agent",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      "kubevirt-handler",
				Namespace: hc.Namespace,
			},
		},
	}
}

func NewWaspClusterRoleBindingWithNameOnly(hc *hcov1beta1.HyperConverged) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "wasp-agent",
			Labels: getLabels(hc, hcoutil.AppComponentWasp),
		},
	}
}
