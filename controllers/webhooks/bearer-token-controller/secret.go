package bearer_token_controller

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/alerts"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/authorization"
)

const (
	secretName = "hco-webhook-bearer-auth"
)

var logger = logf.Log.WithName("init-bearer-token")

func newWHSecretReconciler(namespace string, owner metav1.OwnerReference) *alerts.SecretReconciler {
	token, err := authorization.CreateToken()
	if err != nil {
		logger.Error(err, "failed to create bearer token")
		return nil
	}

	return alerts.CreateSecretReconciler(newSecret(namespace, owner, token))
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
