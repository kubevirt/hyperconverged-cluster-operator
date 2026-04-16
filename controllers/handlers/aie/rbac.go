package aie

import (
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

func NewAIEWebhookClusterRoleHandler(cli client.Client, Scheme *runtime.Scheme) operands.Operand {
	return operands.NewConditionalHandler(
		operands.NewClusterRoleHandler(cli, Scheme, newAIEWebhookClusterRole()),
		shouldDeployAIE,
		func(hc *hcov1beta1.HyperConverged) client.Object {
			return NewAIEWebhookClusterRoleWithNameOnly()
		},
	)
}

func NewAIEWebhookClusterRoleBindingHandler(cli client.Client, Scheme *runtime.Scheme) operands.Operand {
	return operands.NewConditionalHandler(
		operands.NewClusterRoleBindingHandler(cli, Scheme, newAIEWebhookClusterRoleBinding()),
		shouldDeployAIE,
		func(hc *hcov1beta1.HyperConverged) client.Object {
			return NewAIEWebhookClusterRoleBindingWithNameOnly()
		},
	)
}

func NewAIEWebhookClusterRoleWithNameOnly() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			APIVersion: rbacv1.SchemeGroupVersion.String(),
			Kind:       "ClusterRole",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   aieWebhookClusterRoleName,
			Labels: operands.GetLabels(appComponent),
		},
	}
}

func newAIEWebhookClusterRole() *rbacv1.ClusterRole {
	cr := NewAIEWebhookClusterRoleWithNameOnly()
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

func NewAIEWebhookClusterRoleBindingWithNameOnly() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: rbacv1.SchemeGroupVersion.String(),
			Kind:       "ClusterRoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   aieWebhookClusterRoleName,
			Labels: operands.GetLabels(appComponent),
		},
	}
}

func newAIEWebhookClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	crb := NewAIEWebhookClusterRoleBindingWithNameOnly()
	crb.RoleRef = rbacv1.RoleRef{
		Kind:     "ClusterRole",
		Name:     aieWebhookClusterRoleName,
		APIGroup: "rbac.authorization.k8s.io",
	}
	crb.Subjects = []rbacv1.Subject{
		{
			Kind:      "ServiceAccount",
			Name:      aieWebhookServiceAccountName,
			Namespace: hcoutil.GetOperatorNamespaceFromEnv(),
		},
	}
	return crb
}
