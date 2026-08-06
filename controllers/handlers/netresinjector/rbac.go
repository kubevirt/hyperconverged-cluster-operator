package netresinjector

import (
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

func NewClusterRoleHandler(cli client.Client, scheme *runtime.Scheme) operands.Operand {
	return operands.NewConditionalHandler(
		operands.NewClusterRoleHandler(cli, scheme, newClusterRole()),
		shouldDeploy,
		func(hc *hcov1.HyperConverged) client.Object {
			return NewClusterRoleWithNameOnly()
		},
	)
}

func NewClusterRoleBindingHandler(cli client.Client, scheme *runtime.Scheme) operands.Operand {
	return operands.NewConditionalHandler(
		operands.NewClusterRoleBindingHandler(cli, scheme, newClusterRoleBinding()),
		shouldDeploy,
		func(hc *hcov1.HyperConverged) client.Object {
			return NewClusterRoleBindingWithNameOnly()
		},
	)
}

func NewClusterRoleWithNameOnly() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			APIVersion: rbacv1.SchemeGroupVersion.String(),
			Kind:       "ClusterRole",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   clusterRoleName,
			Labels: operands.GetLabels(hcoutil.AppComponentNetResInjector),
		},
	}
}

func newClusterRole() *rbacv1.ClusterRole {
	cr := NewClusterRoleWithNameOnly()
	cr.Rules = []rbacv1.PolicyRule{
		{
			APIGroups: []string{"k8s.cni.cncf.io"},
			Resources: []string{"network-attachment-definitions"},
			Verbs:     []string{"watch", "list", "get"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"configmaps"},
			Verbs:     []string{"watch", "list", "get"},
		},
	}
	return cr
}

func NewClusterRoleBindingWithNameOnly() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: rbacv1.SchemeGroupVersion.String(),
			Kind:       "ClusterRoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   clusterRoleName + "-role-binding",
			Labels: operands.GetLabels(hcoutil.AppComponentNetResInjector),
		},
	}
}

func newClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	crb := NewClusterRoleBindingWithNameOnly()
	crb.RoleRef = rbacv1.RoleRef{
		Kind:     "ClusterRole",
		Name:     clusterRoleName,
		APIGroup: "rbac.authorization.k8s.io",
	}
	crb.Subjects = []rbacv1.Subject{
		{
			Kind:      "ServiceAccount",
			Name:      serviceAccountName,
			Namespace: hcoutil.GetOperatorNamespaceFromEnv(),
		},
	}
	return crb
}
