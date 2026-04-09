package wasp_agent

import (
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

func NewWaspAgentClusterRoleBindingHandler(cli client.Client, Scheme *runtime.Scheme) operands.Operand {
	return operands.NewConditionalHandler(
		operands.NewClusterRoleBindingHandler(cli, Scheme, newWaspAgentClusterRoleBinding()),
		shouldDeployWaspAgent,
		func(hc *hcov1beta1.HyperConverged) client.Object {
			return newWaspAgentClusterRoleBindingWithNameOnly()
		},
	)
}

func newWaspAgentClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	crb := newWaspAgentClusterRoleBindingWithNameOnly()

	crb.RoleRef = rbacv1.RoleRef{
		Kind:     "ClusterRole",
		Name:     clusterRoleName,
		APIGroup: "rbac.authorization.k8s.io",
	}

	crb.Subjects = []rbacv1.Subject{
		{
			Kind:      "ServiceAccount",
			Name:      waspAgentServiceAccountName,
			Namespace: hcoutil.GetOperatorNamespaceFromEnv(),
		},
	}

	return crb
}

func newWaspAgentClusterRoleBindingWithNameOnly() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: rbacv1.SchemeGroupVersion.String(),
			Kind:       "ClusterRoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   clusterRoleName,
			Labels: operands.GetLabels(AppComponentWaspAgent),
		},
	}
}

func NewWaspAgentClusterRoleHandler(Client client.Client, Scheme *runtime.Scheme) operands.Operand {
	return operands.NewConditionalHandler(
		operands.NewClusterRoleHandler(Client, Scheme, newWaspAgentClusterRole()),
		shouldDeployWaspAgent,
		func(hc *hcov1beta1.HyperConverged) client.Object {
			return newWaspAgentClusterRoleWithNameOnly()
		},
	)
}

func newWaspAgentClusterRole() *rbacv1.ClusterRole {
	cr := newWaspAgentClusterRoleWithNameOnly()
	cr.Rules = getClusterPolicyRules()

	return cr
}

func newWaspAgentClusterRoleWithNameOnly() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			APIVersion: rbacv1.SchemeGroupVersion.String(),
			Kind:       "ClusterRole",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   clusterRoleName,
			Labels: operands.GetLabels(AppComponentWaspAgent),
		},
	}
}

func getClusterPolicyRules() []rbacv1.PolicyRule {
	return []rbacv1.PolicyRule{
		{
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"pods",
			},
			Verbs: []string{
				"watch",
				"list",
			},
		},
	}
}
