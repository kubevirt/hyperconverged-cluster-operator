package handlers

import (
	"errors"
	"net/url"
	"os"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/downloadhost"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

const virtioWinCmName = "virtio-win"

// NewVirtioWinCmHandler creates the Virtio-Win ConfigMap Handler
func NewVirtioWinCmHandler(cli client.Client, Scheme *runtime.Scheme) (operands.Operand, error) {
	_, err := getVirtioImageName()
	if err != nil {
		return nil, err
	}

	return operands.NewDynamicCmHandler(cli, Scheme, NewVirtioWinCm), nil
}

// NewVirtioWinCmReaderRoleHandler creates the Virtio-Win ConfigMap Role Handler
func NewVirtioWinCmReaderRoleHandler(cli client.Client, Scheme *runtime.Scheme) operands.Operand {
	return operands.NewRoleHandler(cli, Scheme, NewVirtioWinCmReaderRole())
}

// NewVirtioWinCmReaderRoleBindingHandler creates the Virtio-Win ConfigMap RoleBinding Handler
func NewVirtioWinCmReaderRoleBindingHandler(cli client.Client, Scheme *runtime.Scheme) operands.Operand {
	return operands.NewRoleBindingHandler(cli, Scheme, NewVirtioWinCmReaderRoleBinding())
}

const (
	virtioWinImageKey   = "virtio-win-image"
	virtioWinImageDLKey = "virtio-win-image-download-url"
)

func NewVirtioWinCm(_ *hcov1beta1.HyperConverged) (*corev1.ConfigMap, error) {
	virtiowinContainer, err := getVirtioImageName()
	if err != nil {
		return nil, err
	}

	data := map[string]string{
		virtioWinImageKey: virtiowinContainer,
	}

	if imageDLFilePath, envFound := os.LookupEnv(hcoutil.VirtIOWinDataFileEnvV); envFound && imageDLFilePath != "" {
		downloadURL := url.URL{
			Scheme: "https",
			Host:   string(downloadhost.Get().CurrentHost),
			Path:   imageDLFilePath,
		}
		data[virtioWinImageDLKey] = downloadURL.String()
	}

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      virtioWinCmName,
			Labels:    operands.GetLabels(hcoutil.AppComponentDeployment),
			Namespace: hcoutil.GetOperatorNamespaceFromEnv(),
		},
		Data: data,
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

func getVirtioImageName() (string, error) {
	virtiowinContainer := os.Getenv(hcoutil.VirtioWinImageEnvV)
	if virtiowinContainer == "" {
		return "", errors.New("kv-virtiowin-image-name was not specified")
	}

	return virtiowinContainer, nil
}
