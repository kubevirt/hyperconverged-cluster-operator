package handlers

import (
	"errors"
	"os"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

const virtioWinCmName = "virtio-win"

// NewVirtioWinCmHandler creates the Virtio-Win ConfigMap Handler
func NewVirtioWinCmHandler(cli client.Client, Scheme *runtime.Scheme) (operands.Operand, error) {
	virtioWincm, err := NewVirtioWinCm()
	if err != nil {
		return nil, err
	}
	return operands.NewCmHandler(cli, Scheme, virtioWincm), nil
}

// NewVirtioWinCmReaderRoleHandler creates the Virtio-Win ConfigMap Role Handler
func NewVirtioWinCmReaderRoleHandler(cli client.Client, Scheme *runtime.Scheme) operands.Operand {
	return operands.NewRoleHandler(cli, Scheme, NewVirtioWinCmReaderRole())
}

// NewVirtioWinCmReaderRoleBindingHandler creates the Virtio-Win ConfigMap RoleBinding Handler
func NewVirtioWinCmReaderRoleBindingHandler(cli client.Client, Scheme *runtime.Scheme) operands.Operand {
	return operands.NewRoleBindingHandler(cli, Scheme, NewVirtioWinCmReaderRoleBinding())
}

func NewVirtioWinCm() (*corev1.ConfigMap, error) {
	virtiowinContainer := os.Getenv("VIRTIOWIN_CONTAINER")
	if virtiowinContainer == "" {
		return nil, errors.New("kv-virtiowin-image-name was not specified")
	}

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      virtioWinCmName,
			Labels:    operands.GetLabels(hcoutil.AppComponentDeployment),
			Namespace: hcoutil.GetOperatorNamespaceFromEnv(),
		},
		Data: map[string]string{
			"virtio-win-image": virtiowinContainer,
		},
	}, nil
}

func NewVirtioWinCmReaderRole() *rbacv1.Role {
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      virtioWinCmName,
			Labels:    operands.GetLabels(hcoutil.AppComponentDeployment),
			Namespace: hcoutil.GetOperatorNamespaceFromEnv(),
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

func NewVirtioWinCmReaderRoleBinding() *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      virtioWinCmName,
			Labels:    operands.GetLabels(hcoutil.AppComponentStorage),
			Namespace: hcoutil.GetOperatorNamespaceFromEnv(),
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
