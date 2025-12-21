package alerts

import (
	"context"
	"maps"
	"sync"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/authorization"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

const (
	secretName = "hco-bearer-auth"
)

type CreateSecretFunc func(namespace string, owner metav1.OwnerReference, token string) *corev1.Secret
type SecretReconciler struct {
	theSecret    *corev1.Secret
	lock         *sync.RWMutex
	createSecret CreateSecretFunc
	secretName   string
	refresher    Refresher
}

func NewSecretReconciler(namespace string, owner metav1.OwnerReference, secretName string, createSecret CreateSecretFunc, rfr Refresher) *SecretReconciler {
	token, err := authorization.CreateToken()
	if err != nil {
		logger.Error(err, "failed to create bearer token")
		return nil
	}

	return &SecretReconciler{
		theSecret:    createSecret(namespace, owner, token),
		lock:         &sync.RWMutex{},
		createSecret: createSecret,
		secretName:   secretName,
		refresher:    rfr,
	}
}

func (r *SecretReconciler) Kind() string {
	return "Secret"
}

func (r *SecretReconciler) ResourceName() string {
	return r.secretName
}

func (r *SecretReconciler) GetFullResource() client.Object {
	r.lock.RLock()
	defer r.lock.RUnlock()

	return r.theSecret.DeepCopy()
}

func (r *SecretReconciler) refreshToken(token string) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	r.theSecret = r.createSecret(r.theSecret.Namespace, r.theSecret.OwnerReferences[0], token)

	r.refresher.setShouldRefresh()

	return nil
}

func (r *SecretReconciler) EmptyObject() client.Object {
	return &corev1.Secret{}
}

// UpdateExistingResource checks if the secret already exists and has the correct token.
// If it does, it returns nil. If the secret exists but the token is incorrect, it deletes the old secret
// and creates a new one with the updated token. If the secret does not exist, it creates a new one.
// It deletes the old secret to force Prometheus to reload the configuration.
func (r *SecretReconciler) UpdateExistingResource(ctx context.Context, cl client.Client, resource client.Object, logger logr.Logger) (client.Object, bool, error) {
	found := resource.(*corev1.Secret)

	origLabels := maps.Clone(found.GetLabels())

	token, err := authorization.CreateToken()
	if err != nil {
		return nil, false, err
	}

	// Check if the secret has the correct token
	if string(found.Data["token"]) == token {
		return r.onlyReconcileLabels(ctx, cl, found)
	}

	// If the token is incorrect, delete the old secret and create a new one
	logger.Info("the Secret token is outdated, deleting the old secret and creating a new one", "namespace", found.Namespace, "name", found.Name)
	if err = cl.Delete(ctx, found); err != nil {
		if !errors.IsNotFound(err) {
			logger.Error(err, "failed to delete old secret")
			return nil, false, err
		}
	}

	if err = r.refreshToken(token); err != nil {
		logger.Error(err, "failed to refresh token")
		return nil, false, err
	}

	sec := r.GetFullResource()
	// restore custom labels
	labels := sec.GetLabels()
	for origKey, origVal := range origLabels {
		if _, ok := labels[origKey]; !ok {
			labels[origKey] = origVal
		}
	}

	if err = cl.Create(ctx, sec); err != nil {
		logger.Error(err, "failed to create new secret")
		return nil, false, err
	}

	logger.Info("successfully created the new secret", "namespace", sec.GetNamespace(), "name", sec.GetName())

	return sec, true, nil
}

func (r *SecretReconciler) onlyReconcileLabels(ctx context.Context, cl client.Client, found *corev1.Secret) (client.Object, bool, error) {
	patch, err := hcoutil.MergeLabelsJSONPatch(&r.theSecret.ObjectMeta, &found.ObjectMeta)
	if err != nil {
		return nil, false, err
	}

	if patch == nil {
		return found, false, nil
	}

	err = cl.Patch(ctx, found, client.RawPatch(types.JSONPatchType, patch))
	if err != nil {
		return nil, false, err
	}

	return found, true, nil
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
			Labels:          hcoutil.GetLabels(hcoutil.HyperConvergedName, hcoutil.AppComponentMonitoring),
			OwnerReferences: []metav1.OwnerReference{owner},
		},
		StringData: map[string]string{
			"token": token,
		},
	}
}
