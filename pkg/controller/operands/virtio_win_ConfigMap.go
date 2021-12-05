package operands

import (
	"errors"
	"os"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/pkg/apis/hco/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/controller/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"

	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

const virtioWinCmName = "virtio-win"

// **** Virtio-Win ConfigMap Handler ****
type virtioWinCmHandler genericOperand

func newVirtioWinCmHandler(Client client.Client, Scheme *runtime.Scheme) *virtioWinCmHandler {
	return &virtioWinCmHandler{
		Client:                 Client,
		Scheme:                 Scheme,
		crType:                 "ConfigMap",
		removeExistingOwner:    false,
		setControllerReference: true,
		hooks:                  &virtioWinCmHooks{},
	}
}
func NewVirtioWinCm(cr *hcov1beta1.HyperConverged, namespace string) (*corev1.ConfigMap, error) {
	virtiowinContainer := os.Getenv("VIRTIOWIN_CONTAINER")
	if virtiowinContainer == "" {
		return nil, errors.New("kv-virtiowin-image-name was not specified")
	}

	virtioWincm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      virtioWinCmName,
			Labels:    getLabels(cr, hcoutil.AppComponentDeployment),
			Namespace: namespace,
		},
		Data: map[string]string{
			"virtio-win-image": virtiowinContainer,
		},
	}
	return virtioWincm, nil
}

type virtioWinCmHooks struct{}

func (h virtioWinCmHooks) getFullCr(hc *hcov1beta1.HyperConverged) (client.Object, error) {
	return NewVirtioWinCm(hc, hc.Namespace)
}
func (h virtioWinCmHooks) getEmptyCr() client.Object { return &corev1.ConfigMap{} }

func (h virtioWinCmHooks) getObjectMeta(cr runtime.Object) *metav1.ObjectMeta {
	return &cr.(*corev1.ConfigMap).ObjectMeta
}

func (h *virtioWinCmHooks) updateCr(req *common.HcoRequest, Client client.Client, exists runtime.Object, required runtime.Object) (bool, bool, error) {
	virtioWinCm, ok1 := required.(*corev1.ConfigMap)
	found, ok2 := exists.(*corev1.ConfigMap)
	if !ok1 || !ok2 {
		return false, false, errors.New("can't convert to a ConfigMap")
	}

	needsUpdate := false

	if !reflect.DeepEqual(found.Data, virtioWinCm.Data) {
		virtioWinCm.DeepCopyInto(found)
		needsUpdate = true
	}

	if !reflect.DeepEqual(found.Labels, virtioWinCm.Labels) {
		util.DeepCopyLabels(&virtioWinCm.ObjectMeta, &found.ObjectMeta)
		needsUpdate = true
	}

	if needsUpdate {
		req.Logger.Info("Updating existing virtio-win ConfigMap to its default values")

		if req.HCOTriggered {
			req.Logger.Info("Updating existing virtio-win ConfigMap to new opinionated values")
		} else {
			req.Logger.Info("Reconciling an externally updated virtio-win ConfigMap to its opinionated values")
		}

		err := Client.Update(req.Ctx, found)
		if err != nil {
			return false, false, err
		}
		return true, !req.HCOTriggered, nil
	}

	return false, false, nil
}

// **** Virtio-Win ConfigMap Role Handler ****

type virtioWinCmReaderRoleHandler genericOperand

func newVirtioWinCmReaderRoleHandler(Client client.Client, Scheme *runtime.Scheme) *virtioWinCmReaderRoleHandler {
	return &virtioWinCmReaderRoleHandler{
		Client:                 Client,
		Scheme:                 Scheme,
		crType:                 "Role",
		removeExistingOwner:    false,
		setControllerReference: true,
		hooks:                  &virtioWinCmReaderRoleHooks{},
	}
}

type virtioWinCmReaderRoleHooks struct{}

func (h virtioWinCmReaderRoleHooks) getFullCr(hc *hcov1beta1.HyperConverged) (client.Object, error) {
	return NewVirtioWinCmReaderRole(hc, hc.Namespace), nil
}
func (h virtioWinCmReaderRoleHooks) getEmptyCr() client.Object { return &rbacv1.Role{} }
func (h virtioWinCmReaderRoleHooks) getObjectMeta(cr runtime.Object) *metav1.ObjectMeta {
	return &cr.(*rbacv1.Role).ObjectMeta
}
func (h *virtioWinCmReaderRoleHooks) updateCr(req *common.HcoRequest, Client client.Client, exists runtime.Object, required runtime.Object) (bool, bool, error) {
	configReaderRole, ok1 := required.(*rbacv1.Role)
	found, ok2 := exists.(*rbacv1.Role)
	if !ok1 || !ok2 {
		return false, false, errors.New("can't convert to a Role")
	}

	if !reflect.DeepEqual(found.Labels, configReaderRole.Labels) ||
		!reflect.DeepEqual(found.Rules, configReaderRole.Rules) {

		req.Logger.Info("Updating existing Config Reader Role to its default values")

		found.Rules = make([]rbacv1.PolicyRule, len(configReaderRole.Rules))
		for i := range configReaderRole.Rules {
			configReaderRole.Rules[i].DeepCopyInto(&found.Rules[i])
		}
		util.DeepCopyLabels(&configReaderRole.ObjectMeta, &found.ObjectMeta)

		err := Client.Update(req.Ctx, found)
		if err != nil {
			return false, false, err
		}
		return true, !req.HCOTriggered, nil
	}

	return false, false, nil
}

// **** Virtio-Win ConfigMap RoleBinding Handler ****
type virtioWinCmReaderRoleBindingHandler genericOperand

func newVirtioWinCmReaderRoleBindingHandler(Client client.Client, Scheme *runtime.Scheme) *virtioWinCmReaderRoleBindingHandler {
	return &virtioWinCmReaderRoleBindingHandler{
		Client:                 Client,
		Scheme:                 Scheme,
		crType:                 "RoleBinding",
		removeExistingOwner:    false,
		setControllerReference: true,
		hooks:                  &virtioWinCmReaderRoleBindingHooks{},
	}
}

type virtioWinCmReaderRoleBindingHooks struct{}

func (h virtioWinCmReaderRoleBindingHooks) getFullCr(hc *hcov1beta1.HyperConverged) (client.Object, error) {
	return NewVirtioWinCmReaderRoleBinding(hc, hc.Namespace), nil
}
func (h virtioWinCmReaderRoleBindingHooks) getEmptyCr() client.Object { return &rbacv1.RoleBinding{} }
func (h virtioWinCmReaderRoleBindingHooks) getObjectMeta(cr runtime.Object) *metav1.ObjectMeta {
	return &cr.(*rbacv1.RoleBinding).ObjectMeta
}
func (h *virtioWinCmReaderRoleBindingHooks) updateCr(req *common.HcoRequest, Client client.Client, exists runtime.Object, required runtime.Object) (bool, bool, error) {
	configReaderRoleBinding, ok1 := required.(*rbacv1.RoleBinding)
	found, ok2 := exists.(*rbacv1.RoleBinding)
	if !ok1 || !ok2 {
		return false, false, errors.New("can't convert to a RoleBinding")
	}

	if !reflect.DeepEqual(found.Labels, configReaderRoleBinding.Labels) ||
		!reflect.DeepEqual(found.Subjects, configReaderRoleBinding.Subjects) ||
		!reflect.DeepEqual(found.RoleRef, configReaderRoleBinding.RoleRef) {
		req.Logger.Info("Updating existing Config Reader RoleBinding to its default values")

		found.Subjects = make([]rbacv1.Subject, len(configReaderRoleBinding.Subjects))
		copy(found.Subjects, configReaderRoleBinding.Subjects)
		found.RoleRef = configReaderRoleBinding.RoleRef
		util.DeepCopyLabels(&configReaderRoleBinding.ObjectMeta, &found.ObjectMeta)

		err := Client.Update(req.Ctx, found)
		if err != nil {
			return false, false, err
		}
		return true, !req.HCOTriggered, nil
	}

	return false, false, nil
}

func NewVirtioWinCmReaderRole(cr *hcov1beta1.HyperConverged, namespace string) *rbacv1.Role {
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      virtioWinCmName,
			Labels:    getLabels(cr, hcoutil.AppComponentDeployment),
			Namespace: namespace,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups:     []string{""},
				Resources:     []string{"configmaps"},
				ResourceNames: []string{virtioWinCmName},
				Verbs:         []string{"get"},
			},
		},
	}
}

func NewVirtioWinCmReaderRoleBinding(cr *hcov1beta1.HyperConverged, namespace string) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      virtioWinCmName,
			Labels:    getLabels(cr, hcoutil.AppComponentStorage),
			Namespace: namespace,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     virtioWinCmName,
		},
		Subjects: []rbacv1.Subject{
			{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Group",
				Name:     "system:authenticated",
			},
		},
	}
}
