package aie_webhook

import (
	"fmt"

	log "github.com/go-logr/logr"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
)

func NewAIEWebhookMutatingWebhookConfigurationHandler(
	_ log.Logger, Client client.Client, Scheme *runtime.Scheme, hc *hcov1beta1.HyperConverged,
) (operands.Operand, error) {
	return operands.NewConditionalHandler(
		operands.NewMutatingWebhookConfigurationHandler(Client, Scheme, newAIEMutatingWebhookConfiguration(hc)),
		shouldDeployAIEWebhook,
		func(hc *hcov1beta1.HyperConverged) client.Object {
			return NewAIEWebhookMutatingWebhookConfigurationWithNameOnly(hc)
		},
	), nil
}

func NewAIEWebhookMutatingWebhookConfigurationWithNameOnly(hc *hcov1beta1.HyperConverged) *admissionregistrationv1.MutatingWebhookConfiguration {
	return &admissionregistrationv1.MutatingWebhookConfiguration{
		TypeMeta: metav1.TypeMeta{
			APIVersion: admissionregistrationv1.SchemeGroupVersion.String(),
			Kind:       "MutatingWebhookConfiguration",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   aieWebhookName,
			Labels: operands.GetLabels(hc, appComponent),
		},
	}
}

func newAIEMutatingWebhookConfiguration(hc *hcov1beta1.HyperConverged) *admissionregistrationv1.MutatingWebhookConfiguration {
	mwc := NewAIEWebhookMutatingWebhookConfigurationWithNameOnly(hc)
	mwc.Annotations = map[string]string{
		"cert-manager.io/inject-ca-from": fmt.Sprintf("%s/%s", hc.Namespace, aieWebhookCertificateName),
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
					Namespace: hc.Namespace,
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
