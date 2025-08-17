package operands

import (
	"errors"
	"reflect"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

// ********* Role Handler *****************************

func NewRoleHandler(Client client.Client, Scheme *runtime.Scheme, required *rbacv1.Role) *GenericOperand {
	return NewGenericOperand(Client, Scheme, "Role", &roleHooks{required: required}, true)
}

type roleHooks struct {
	required *rbacv1.Role
}

func (h *roleHooks) GetFullCr(_ *hcov1beta1.HyperConverged) (client.Object, error) {
	return h.required.DeepCopy(), nil
}
func (h *roleHooks) GetEmptyCr() client.Object {
	return &rbacv1.Role{}
}
func (h *roleHooks) UpdateCR(req *common.HcoRequest, Client client.Client, exists runtime.Object, _ runtime.Object) (bool, bool, error) {
	role := h.required
	found, ok := exists.(*rbacv1.Role)
	if !ok {
		return false, false, errors.New("can't convert to a Role")
	}

	if !util.CompareLabels(role, found) ||
		!reflect.DeepEqual(role.Rules, found.Rules) {

		req.Logger.Info("Updating existing Role to its default values", "name", found.Name)

		found.Rules = make([]rbacv1.PolicyRule, len(role.Rules))
		for i := range role.Rules {
			role.Rules[i].DeepCopyInto(&found.Rules[i])
		}
		util.MergeLabels(&role.ObjectMeta, &found.ObjectMeta)

		err := Client.Update(req.Ctx, found)
		if err != nil {
			return false, false, err
		}
		return true, !req.HCOTriggered, nil
	}

	return false, false, nil
}

func (*roleHooks) JustBeforeComplete(_ *common.HcoRequest) { /* no implementation */ }

// ********* Cluster Role Handler *****************************

func NewClusterRoleHandler(Client client.Client, Scheme *runtime.Scheme, required *rbacv1.ClusterRole) *GenericOperand {
	return NewGenericOperand(Client, Scheme, "ClusterRole", &clusterRoleHooks{required: required}, false)
}

type clusterRoleHooks struct {
	required *rbacv1.ClusterRole
}

func (h *clusterRoleHooks) GetFullCr(_ *hcov1beta1.HyperConverged) (client.Object, error) {
	return h.required.DeepCopy(), nil
}

func (h *clusterRoleHooks) GetEmptyCr() client.Object { return &rbacv1.ClusterRole{} }

func (h *clusterRoleHooks) UpdateCR(req *common.HcoRequest, Client client.Client, exists runtime.Object, required runtime.Object) (bool, bool, error) {
	role, ok1 := required.(*rbacv1.ClusterRole)
	found, ok2 := exists.(*rbacv1.ClusterRole)
	if !ok1 || !ok2 {
		return false, false, errors.New("can't convert to a Cluster Role")
	}

	if !util.CompareLabels(role, found) ||
		!reflect.DeepEqual(role.Rules, found.Rules) ||
		!reflect.DeepEqual(role.AggregationRule, found.AggregationRule) {

		req.Logger.Info("Updating existing Cluster Role to its default values", "name", found.Name)

		found.Rules = make([]rbacv1.PolicyRule, len(role.Rules))
		for i := range role.Rules {
			role.Rules[i].DeepCopyInto(&found.Rules[i])
		}

		util.MergeLabels(&role.ObjectMeta, &found.ObjectMeta)

		if role.AggregationRule != nil {
			found.AggregationRule = &rbacv1.AggregationRule{}

			found.AggregationRule.ClusterRoleSelectors =
				make([]metav1.LabelSelector, len(role.AggregationRule.ClusterRoleSelectors))

			found.AggregationRule.ClusterRoleSelectors =
				append(found.AggregationRule.ClusterRoleSelectors, role.AggregationRule.ClusterRoleSelectors...)
		}

		err := Client.Update(req.Ctx, found)
		if err != nil {
			return false, false, err
		}
		return true, !req.HCOTriggered, nil
	}

	return false, false, nil
}

func (*clusterRoleHooks) JustBeforeComplete(_ *common.HcoRequest) { /* no implementation */ }

// ********* Role Binding Handler *****************************

func NewRoleBindingHandler(Client client.Client, Scheme *runtime.Scheme, required *rbacv1.RoleBinding) *GenericOperand {
	return NewGenericOperand(Client, Scheme, "RoleBinding", &roleBindingHooks{required: required}, true)
}

type roleBindingHooks struct {
	required *rbacv1.RoleBinding
}

func (h roleBindingHooks) GetFullCr(_ *hcov1beta1.HyperConverged) (client.Object, error) {
	return h.required.DeepCopy(), nil
}
func (h roleBindingHooks) GetEmptyCr() client.Object {
	return &rbacv1.RoleBinding{}
}
func (h roleBindingHooks) UpdateCR(req *common.HcoRequest, Client client.Client, exists runtime.Object, _ runtime.Object) (bool, bool, error) {
	configReaderRoleBinding := h.required
	found, ok := exists.(*rbacv1.RoleBinding)
	if !ok {
		return false, false, errors.New("can't convert to a RoleBinding")
	}

	if !util.CompareLabels(configReaderRoleBinding, found) ||
		!reflect.DeepEqual(configReaderRoleBinding.Subjects, found.Subjects) ||
		!reflect.DeepEqual(configReaderRoleBinding.RoleRef, found.RoleRef) {
		req.Logger.Info("Updating existing RoleBinding to its default values", "name", found.Name)

		found.Subjects = make([]rbacv1.Subject, len(configReaderRoleBinding.Subjects))
		copy(found.Subjects, configReaderRoleBinding.Subjects)
		found.RoleRef = configReaderRoleBinding.RoleRef
		util.MergeLabels(&configReaderRoleBinding.ObjectMeta, &found.ObjectMeta)

		err := Client.Update(req.Ctx, found)
		if err != nil {
			return false, false, err
		}
		return true, !req.HCOTriggered, nil
	}

	return false, false, nil
}

func (roleBindingHooks) JustBeforeComplete(_ *common.HcoRequest) { /* no implementation */ }

// ********* Cluster Role Binding Handler *****************************

func NewClusterRoleBindingHandler(Client client.Client, Scheme *runtime.Scheme, required *rbacv1.ClusterRoleBinding) *GenericOperand {
	return NewGenericOperand(Client, Scheme, "ClusterRoleBinding", &clusterRoleBindingHooks{required: required}, false)
}

type clusterRoleBindingHooks struct {
	required *rbacv1.ClusterRoleBinding
}

func (h clusterRoleBindingHooks) GetFullCr(hc *hcov1beta1.HyperConverged) (client.Object, error) {
	return h.required, nil
}

func (h clusterRoleBindingHooks) GetEmptyCr() client.Object { return &rbacv1.ClusterRoleBinding{} }

func (h clusterRoleBindingHooks) UpdateCR(req *common.HcoRequest, Client client.Client, exists runtime.Object, required runtime.Object) (bool, bool, error) {
	clusterRoleBinding, ok1 := required.(*rbacv1.ClusterRoleBinding)
	found, ok2 := exists.(*rbacv1.ClusterRoleBinding)
	if !ok1 || !ok2 {
		return false, false, errors.New("can't convert to a ClusterRoleBinding")
	}

	if !util.CompareLabels(clusterRoleBinding, found) ||
		!reflect.DeepEqual(clusterRoleBinding.Subjects, found.Subjects) ||
		!reflect.DeepEqual(clusterRoleBinding.RoleRef, found.RoleRef) {
		req.Logger.Info("Updating existing ClusterRoleBinding to its default values", "name", found.Name)

		found.Subjects = make([]rbacv1.Subject, len(clusterRoleBinding.Subjects))
		copy(found.Subjects, clusterRoleBinding.Subjects)
		found.RoleRef = clusterRoleBinding.RoleRef
		util.MergeLabels(&clusterRoleBinding.ObjectMeta, &found.ObjectMeta)

		err := Client.Update(req.Ctx, found)
		if err != nil {
			return false, false, err
		}
		return true, !req.HCOTriggered, nil
	}

	return false, false, nil
}

func (clusterRoleBindingHooks) JustBeforeComplete(_ *common.HcoRequest) { /* no implementation */ }
