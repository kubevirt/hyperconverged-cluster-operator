package observabilitycontroller

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
			return newClusterRoleWithNameOnly()
		},
	)
}

func NewClusterRoleBindingHandler(cli client.Client, scheme *runtime.Scheme) operands.Operand {
	return operands.NewConditionalHandler(
		operands.NewClusterRoleBindingHandler(cli, scheme, newClusterRoleBinding()),
		shouldDeploy,
		func(hc *hcov1.HyperConverged) client.Object {
			return newClusterRoleBindingWithNameOnly()
		},
	)
}

func newClusterRoleWithNameOnly() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			APIVersion: rbacv1.SchemeGroupVersion.String(),
			Kind:       "ClusterRole",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   clusterRoleName,
			Labels: operands.GetLabels(hcoutil.AppComponentObservability),
		},
	}
}

func newClusterRole() *rbacv1.ClusterRole {
	cr := newClusterRoleWithNameOnly()
	cr.Rules = []rbacv1.PolicyRule{
		{
			APIGroups: []string{"kubevirt.io"},
			Resources: []string{"virtualmachines", "virtualmachineinstances", "virtualmachineinstancemigrations", "kubevirts"},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{"instancetype.kubevirt.io"},
			Resources: []string{"virtualmachineinstancetypes", "virtualmachineclusterinstancetypes", "virtualmachinepreferences", "virtualmachineclusterpreferences"},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"pods", "persistentvolumeclaims"},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"configmaps"},
			Verbs:     []string{"get"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"services", "secrets"},
			Verbs:     []string{"get", "list", "watch", "create", "update", "delete"},
		},
		{
			APIGroups: []string{"apps"},
			Resources: []string{"controllerrevisions"},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{"monitoring.coreos.com"},
			Resources: []string{"prometheusrules", "servicemonitors"},
			Verbs:     []string{"get", "list", "watch", "create", "update", "delete"},
		},
		{
			APIGroups: []string{"authentication.k8s.io"},
			Resources: []string{"tokenreviews"},
			Verbs:     []string{"create"},
		},
		{
			APIGroups: []string{"authorization.k8s.io"},
			Resources: []string{"subjectaccessreviews"},
			Verbs:     []string{"create"},
		},
	}
	return cr
}

func newClusterRoleBindingWithNameOnly() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: rbacv1.SchemeGroupVersion.String(),
			Kind:       "ClusterRoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   clusterRoleBindingName,
			Labels: operands.GetLabels(hcoutil.AppComponentObservability),
		},
	}
}

func newClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	crb := newClusterRoleBindingWithNameOnly()
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
