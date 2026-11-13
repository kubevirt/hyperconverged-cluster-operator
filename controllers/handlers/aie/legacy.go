package aie

import (
	"context"
	"fmt"
	"slices"
	"time"

	admissionv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/reference"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

const (
	legacyWebhookName           = "kubevirt-aie-webhook"
	legacyWebhookReadyCondition = "KubevirtAieWebhookReady"
)

type legacyWebhookCleaner struct {
	cli    client.Client
	scheme *runtime.Scheme
}

// NewLegacyWebhookCleaner removes the retired operand even when AIE is disabled.
func NewLegacyWebhookCleaner(cli client.Client, scheme *runtime.Scheme) operands.Operand {
	return &legacyWebhookCleaner{cli: cli, scheme: scheme}
}

func (*legacyWebhookCleaner) Reset() {}

func (h *legacyWebhookCleaner) Ensure(req *common.HcoRequest) *operands.EnsureResult {
	res := operands.NewEnsureResult(&admissionv1.MutatingWebhookConfiguration{ObjectMeta: metav1.ObjectMeta{Name: legacyWebhookName}})
	ctx, cancel := context.WithTimeout(req.Ctx, 10*time.Second)
	defer cancel()
	objects := legacyWebhookObjects(req.Instance.Namespace)
	policies := &networkingv1.NetworkPolicyList{}
	if err := h.cli.List(ctx, policies, client.InNamespace(req.Instance.Namespace), client.MatchingLabels{
		hcoutil.AppLabel:          req.Instance.Name,
		hcoutil.AppLabelComponent: "aie-webhook",
	}); err != nil {
		return res.Error(err)
	}
	for _, policy := range policies.Items {
		objects = append(objects, policy.DeepCopy())
	}
	for i, obj := range objects {
		// Confirm that the fail-closed admission registration is gone before
		// deleting its server. Retry any failed cleanup on the next reconcile.
		if _, err := hcoutil.EnsureDeleted(ctx, h.cli, obj, req.Instance.Name, req.Logger, false, true, true); err != nil {
			return res.Error(err)
		}
		if i == 0 {
			// Do not tear down a server while an unowned admission registration
			// with its name still exists and may point to that server.
			err := h.cli.Get(ctx, client.ObjectKeyFromObject(obj), obj)
			if err == nil {
				return res.Error(fmt.Errorf("AIE webhook admission registration still exists; refusing to remove its server"))
			}
			if !apierrors.IsNotFound(err) {
				return res.Error(err)
			}
		}
		ref, err := reference.GetReference(h.scheme, obj)
		if err != nil {
			return res.Error(err)
		}
		before := len(req.Instance.Status.RelatedObjects)
		req.Instance.Status.RelatedObjects = slices.DeleteFunc(req.Instance.Status.RelatedObjects, func(existing corev1.ObjectReference) bool {
			return existing.Kind == ref.Kind && existing.Name == ref.Name && existing.Namespace == ref.Namespace
		})
		req.StatusDirty = req.StatusDirty || before != len(req.Instance.Status.RelatedObjects)
	}
	if apimeta.RemoveStatusCondition(&req.Instance.Status.Conditions, legacyWebhookReadyCondition) {
		req.StatusDirty = true
	}
	return res.SetUpgradeDone(req.ComponentUpgradeInProgress)
}

func legacyWebhookObjects(namespace string) []client.Object {
	namespaced := metav1.ObjectMeta{Name: legacyWebhookName, Namespace: namespace}
	cluster := metav1.ObjectMeta{Name: legacyWebhookName}
	return []client.Object{
		&admissionv1.MutatingWebhookConfiguration{ObjectMeta: cluster},
		&appsv1.Deployment{ObjectMeta: namespaced},
		&corev1.Service{ObjectMeta: namespaced},
		&corev1.ServiceAccount{ObjectMeta: namespaced},
		&rbacv1.ClusterRoleBinding{ObjectMeta: cluster},
		&rbacv1.ClusterRole{ObjectMeta: cluster},
		&networkingv1.NetworkPolicy{ObjectMeta: namespaced},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: legacyWebhookName + "-tls", Namespace: namespace}},
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "kubevirt-aie-launcher-config", Namespace: namespace}},
	}
}
