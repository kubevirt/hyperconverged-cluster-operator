package aie_webhook

import (
	log "github.com/go-logr/logr"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
)

func NewAIEWebhookClusterRoleHandler(
	_ log.Logger, Client client.Client, Scheme *runtime.Scheme, hc *hcov1beta1.HyperConverged,
) (operands.Operand, error) {
	return operands.NewConditionalHandler(
		operands.NewClusterRoleHandler(Client, Scheme, newAIEWebhookClusterRole(hc)),
		shouldDeployAIEWebhook,
		func(hc *hcov1beta1.HyperConverged) client.Object {
			return NewAIEWebhookClusterRoleWithNameOnly(hc)
		},
	), nil
}

func NewAIEWebhookClusterRoleBindingHandler(
	_ log.Logger, Client client.Client, Scheme *runtime.Scheme, hc *hcov1beta1.HyperConverged,
) (operands.Operand, error) {
	return operands.NewConditionalHandler(
		operands.NewClusterRoleBindingHandler(Client, Scheme, newAIEWebhookClusterRoleBinding(hc)),
		shouldDeployAIEWebhook,
		func(hc *hcov1beta1.HyperConverged) client.Object {
			return NewAIEWebhookClusterRoleBindingWithNameOnly(hc)
		},
	), nil
}

func NewAIEWebhookClusterRoleWithNameOnly(hc *hcov1beta1.HyperConverged) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			APIVersion: rbacv1.SchemeGroupVersion.String(),
			Kind:       "ClusterRole",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   aieWebhookClusterRoleName,
			Labels: operands.GetLabels(hc, appComponent),
		},
	}
}

func newAIEWebhookClusterRole(hc *hcov1beta1.HyperConverged) *rbacv1.ClusterRole {
	cr := NewAIEWebhookClusterRoleWithNameOnly(hc)
	cr.Rules = []rbacv1.PolicyRule{
		{
			APIGroups: []string{"kubevirt.io"},
			Resources: []string{"virtualmachineinstances"},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"configmaps"},
			Verbs:     []string{"get", "list", "watch"},
		},
	}
	return cr
}

func NewAIEWebhookClusterRoleBindingWithNameOnly(hc *hcov1beta1.HyperConverged) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: rbacv1.SchemeGroupVersion.String(),
			Kind:       "ClusterRoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   aieWebhookClusterRoleName,
			Labels: operands.GetLabels(hc, appComponent),
		},
	}
}

func newAIEWebhookClusterRoleBinding(hc *hcov1beta1.HyperConverged) *rbacv1.ClusterRoleBinding {
	crb := NewAIEWebhookClusterRoleBindingWithNameOnly(hc)
	crb.RoleRef = rbacv1.RoleRef{
		Kind:     "ClusterRole",
		Name:     aieWebhookClusterRoleName,
		APIGroup: "rbac.authorization.k8s.io",
	}
	crb.Subjects = []rbacv1.Subject{
		{
			Kind:      "ServiceAccount",
			Name:      aieWebhookServiceAccountName,
			Namespace: hc.Namespace,
		},
	}
	return crb
}
