package components

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

const (
	crName = util.HyperConvergedName
)

func GetStdPodSecurityContext() *corev1.PodSecurityContext {
	return &corev1.PodSecurityContext{
		RunAsNonRoot: new(true),
		SeccompProfile: &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		},
	}
}

func GetStdContainerSecurityContext() *corev1.SecurityContext {
	return &corev1.SecurityContext{
		AllowPrivilegeEscalation: new(false),
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{"ALL"},
		},
	}
}

var GetOperatorCR = GetOperatorV1CR

func GetOperatorV1beta1CR() *hcov1beta1.HyperConverged {
	defaultScheme := runtime.NewScheme()
	_ = hcov1beta1.AddToScheme(defaultScheme)
	_ = hcov1beta1.RegisterDefaults(defaultScheme)
	defaultHco := &hcov1beta1.HyperConverged{
		TypeMeta: metav1.TypeMeta{
			APIVersion: hcov1beta1.APIVersion,
			Kind:       util.HyperConvergedKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: crName,
		}}
	defaultScheme.Default(defaultHco)
	return defaultHco
}

func GetOperatorV1CR() *hcov1.HyperConverged {
	defaultScheme := runtime.NewScheme()
	_ = hcov1.AddToScheme(defaultScheme)
	_ = hcov1.RegisterDefaults(defaultScheme)
	defaultHco := &hcov1.HyperConverged{
		TypeMeta: metav1.TypeMeta{
			APIVersion: hcov1.APIVersion,
			Kind:       util.HyperConvergedKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: crName,
		}}
	defaultScheme.Default(defaultHco)
	return defaultHco
}
