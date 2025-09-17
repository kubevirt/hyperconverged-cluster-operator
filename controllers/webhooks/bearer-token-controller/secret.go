package bearer_token_controller

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/alerts"
)

const (
	secretName = "hco-webhook-bearer-auth"
)

func newWHSecretReconciler(namespace string, owner metav1.OwnerReference, refresher alerts.Refresher) *alerts.SecretReconciler {
	return alerts.NewSecretReconciler(namespace, owner, secretName, newSecret, refresher)
}

func newSecret(namespace string, owner metav1.OwnerReference, token string) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            secretName,
			Namespace:       namespace,
			Labels:          getLabels(),
			OwnerReferences: []metav1.OwnerReference{owner},
		},
		StringData: map[string]string{
			"token": token,
		},
	}
}
