package alerts

import (
	"context"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/authorization"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

const (
	secretName = "hco-bearer-auth"
)

type secretReconciler struct {
	theSecret *corev1.Secret
}

func newSecretReconciler(namespace string, owner metav1.OwnerReference) *secretReconciler {
	token, err := authorization.CreateToken()
	if err != nil {
		logger.Error(err, "failed to create bearer token")
		return nil
	}

	return &secretReconciler{
		theSecret: NewSecret(namespace, owner, token),
	}
}

func (r *secretReconciler) Kind() string {
	return "Secret"
}

func (r *secretReconciler) ResourceName() string {
	return secretName
}

func (r *secretReconciler) GetFullResource() client.Object {
	return r.theSecret.DeepCopy()
}

func (r *secretReconciler) EmptyObject() client.Object {
	return &corev1.Secret{}
}

// UpdateExistingResource checks if the secret already exists and has the correct token.
// If it does, it returns nil. If the secret exists but the token is incorrect, it deletes the old secret
// and creates a new one with the updated token. If the secret does not exist, it creates a new one.
// It deletes the old secret to force Prometheus to reload the configuration.
func (r *secretReconciler) UpdateExistingResource(ctx context.Context, cl client.Client, resource client.Object, logger logr.Logger) (client.Object, bool, error) {
	found := resource.(*corev1.Secret)

	token, err := authorization.CreateToken()
	if err != nil {
		return nil, false, err
	}

	// Check if the secret has the correct token
	if string(found.Data["token"]) == token {
		return nil, false, nil
	}

	// If the the token is incorrect, delete the old secret and create a new one
	if err := cl.Delete(ctx, found); err != nil {
		if !errors.IsNotFound(err) {
			logger.Error(err, "failed to delete old secret")
			return nil, false, err
		}
	}

	newSecret := NewSecret(r.theSecret.Namespace, r.theSecret.OwnerReferences[0], token)
	if err := cl.Create(ctx, newSecret); err != nil {
		logger.Error(err, "failed to create new secret")
		return nil, false, err
	}

	return newSecret, true, nil
}

func NewSecret(namespace string, owner metav1.OwnerReference, token string) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            secretName,
			Namespace:       namespace,
			Labels:          hcoutil.GetLabels(hcoutil.HyperConvergedName, hcoutil.AppComponentMonitoring),
			OwnerReferences: []metav1.OwnerReference{owner},
		},
		StringData: map[string]string{
			"token": token,
		},
	}
}
