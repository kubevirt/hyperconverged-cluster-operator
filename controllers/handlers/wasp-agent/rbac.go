package wasp_agent

import (
	log "github.com/go-logr/logr"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
)

func NewWaspAgentClusterRoleBindingHandler(
	_ log.Logger, Client client.Client, Scheme *runtime.Scheme, hc *hcov1beta1.HyperConverged) (operands.Operand, error) {
	return operands.NewConditionalHandler(
		operands.NewClusterRoleBindingHandler(Client, Scheme, newWaspAgentClusterRoleBinding(hc)),
		shouldDeployWaspAgent,
		func(hc *hcov1beta1.HyperConverged) client.Object {
			return newWaspAgentClusterRoleBindingWithNameOnly(hc)
		},
	), nil
}

func newWaspAgentClusterRoleBinding(hc *hcov1beta1.HyperConverged) *rbacv1.ClusterRoleBinding {
	crb := newWaspAgentClusterRoleBindingWithNameOnly(hc)

	crb.RoleRef = rbacv1.RoleRef{
		Kind:     "ClusterRole",
		Name:     clusterRoleName,
		APIGroup: "rbac.authorization.k8s.io",
	}

	crb.Subjects = []rbacv1.Subject{
		{
			Kind:      "ServiceAccount",
			Name:      waspAgentServiceAccountName,
			Namespace: hc.Namespace,
		},
	}

	return crb
}

func newWaspAgentClusterRoleBindingWithNameOnly(hc *hcov1beta1.HyperConverged) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: rbacv1.SchemeGroupVersion.String(),
			Kind:       "ClusterRoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   clusterRoleName,
			Labels: operands.GetLabels(hc, AppComponentWaspAgent),
		},
	}
}

func NewWaspAgentClusterRoleHandler(
	_ log.Logger, Client client.Client, Scheme *runtime.Scheme, hc *hcov1beta1.HyperConverged) (operands.Operand, error) {
	return operands.NewConditionalHandler(
		operands.NewClusterRoleHandler(Client, Scheme, newWaspAgentClusterRole(hc)),
		shouldDeployWaspAgent,
		func(hc *hcov1beta1.HyperConverged) client.Object {
			return newWaspAgentClusterRoleWithNameOnly(hc)
		},
	), nil
}

func newWaspAgentClusterRole(hc *hcov1beta1.HyperConverged) *rbacv1.ClusterRole {
	cr := newWaspAgentClusterRoleWithNameOnly(hc)
	cr.Rules = getClusterPolicyRules()

	return cr
}

func newWaspAgentClusterRoleWithNameOnly(hc *hcov1beta1.HyperConverged) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			APIVersion: rbacv1.SchemeGroupVersion.String(),
			Kind:       "ClusterRole",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   clusterRoleName,
			Labels: operands.GetLabels(hc, AppComponentWaspAgent),
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
