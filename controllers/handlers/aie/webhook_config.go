package aie

import (
	"errors"
	"reflect"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

func NewAIEWebhookMutatingWebhookConfigurationHandler(cli client.Client, Scheme *runtime.Scheme) operands.Operand {
	return operands.NewConditionalHandler(
		operands.NewGenericOperand(cli, Scheme, "MutatingWebhookConfiguration",
			&aieWebhookMWCHooks{required: newAIEMutatingWebhookConfiguration()}, false),
		shouldDeployAIE,
		func(hc *hcov1beta1.HyperConverged) client.Object {
			return NewAIEWebhookMutatingWebhookConfigurationWithNameOnly()
		},
	)
}

type aieWebhookMWCHooks struct {
	required *admissionregistrationv1.MutatingWebhookConfiguration
}

func (h *aieWebhookMWCHooks) GetFullCr(_ *hcov1beta1.HyperConverged) (client.Object, error) {
	return h.required.DeepCopy(), nil
}

func (h *aieWebhookMWCHooks) GetEmptyCr() client.Object {
	return &admissionregistrationv1.MutatingWebhookConfiguration{}
}

func (h *aieWebhookMWCHooks) UpdateCR(req *common.HcoRequest, Client client.Client, exists runtime.Object, required runtime.Object) (bool, bool, error) {
	mwc, ok1 := required.(*admissionregistrationv1.MutatingWebhookConfiguration)
	found, ok2 := exists.(*admissionregistrationv1.MutatingWebhookConfiguration)
	if !ok1 || !ok2 {
		return false, false, errors.New("can't convert to MutatingWebhookConfiguration")
	}

	// Preserve caBundle values injected by the cluster (e.g. service-ca operator on OpenShift)
	// before comparing, so we don't trigger unnecessary updates or wipe injected values.
	for i := range mwc.Webhooks {
		if i < len(found.Webhooks) && len(mwc.Webhooks[i].ClientConfig.CABundle) == 0 {
			mwc.Webhooks[i].ClientConfig.CABundle = found.Webhooks[i].ClientConfig.CABundle
		}
	}

	if !util.CompareLabels(mwc, found) ||
		!reflect.DeepEqual(mwc.Webhooks, found.Webhooks) ||
		!hasRequiredAnnotations(found.Annotations, mwc.Annotations) {

		if req.HCOTriggered {
			req.Logger.Info("Updating existing MutatingWebhookConfiguration to new opinionated values", "name", found.Name)
		} else {
			req.Logger.Info("Reconciling an externally updated MutatingWebhookConfiguration to its opinionated values", "name", found.Name)
		}

		util.MergeLabels(&mwc.ObjectMeta, &found.ObjectMeta)
		found.Webhooks = make([]admissionregistrationv1.MutatingWebhook, len(mwc.Webhooks))
		for i := range mwc.Webhooks {
			mwc.Webhooks[i].DeepCopyInto(&found.Webhooks[i])
		}
		if mwc.Annotations != nil {
			if found.Annotations == nil {
				found.Annotations = make(map[string]string)
			}
			for k, v := range mwc.Annotations {
				found.Annotations[k] = v
			}
		}

		err := Client.Update(req.Ctx, found)
		if err != nil {
			return false, false, err
		}
		return true, !req.HCOTriggered, nil
	}

	return false, false, nil
}

func NewAIEWebhookMutatingWebhookConfigurationWithNameOnly() *admissionregistrationv1.MutatingWebhookConfiguration {
	return &admissionregistrationv1.MutatingWebhookConfiguration{
		TypeMeta: metav1.TypeMeta{
			APIVersion: admissionregistrationv1.SchemeGroupVersion.String(),
			Kind:       "MutatingWebhookConfiguration",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   aieWebhookName,
			Labels: operands.GetLabels(appComponent),
		},
	}
}

func newAIEMutatingWebhookConfiguration() *admissionregistrationv1.MutatingWebhookConfiguration {
	mwc := NewAIEWebhookMutatingWebhookConfigurationWithNameOnly()
	if util.GetClusterInfo().IsOpenshift() {
		mwc.Annotations = map[string]string{
			"service.beta.openshift.io/inject-cabundle": "true",
		}
	}
	failPolicy := admissionregistrationv1.Fail
	sideEffects := admissionregistrationv1.SideEffectClassNone
	scope := admissionregistrationv1.NamespacedScope
	mwc.Webhooks = []admissionregistrationv1.MutatingWebhook{
		{
			Name:                    "virt-launcher-mutator.kubevirt.io",
			AdmissionReviewVersions: []string{"v1"},
			SideEffects:             &sideEffects,
			FailurePolicy:           &failPolicy,
			ClientConfig: admissionregistrationv1.WebhookClientConfig{
				Service: &admissionregistrationv1.ServiceReference{
					Name:      aieWebhookName,
					Namespace: util.GetOperatorNamespaceFromEnv(),
					Path:      ptr.To("/mutate-pods"),
					Port:      ptr.To[int32](443),
				},
			},
			ObjectSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"kubevirt.io": "virt-launcher",
				},
			},
			Rules: []admissionregistrationv1.RuleWithOperations{
				{
					Operations: []admissionregistrationv1.OperationType{
						admissionregistrationv1.Create,
					},
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{""},
						APIVersions: []string{"v1"},
						Resources:   []string{"pods"},
						Scope:       &scope,
					},
				},
			},
		},
	}
	return mwc
}

// hasRequiredAnnotations returns true if all required annotations are present
// with the correct values in the existing annotations map. Extra annotations
// added by users or other controllers are ignored.
func hasRequiredAnnotations(existing, required map[string]string) bool {
	for k, v := range required {
		if existing[k] != v {
			return false
		}
	}
	return true
}
