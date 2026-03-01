package aie_webhook

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
)

func NewAIEWebhookServiceAccountHandler(Client client.Client, Scheme *runtime.Scheme) operands.Operand {
	return operands.NewConditionalHandler(
		operands.NewServiceAccountHandler(Client, Scheme, newAIEWebhookServiceAccount),
		shouldDeployAIEWebhook,
		func(hc *hcov1beta1.HyperConverged) client.Object {
			return newAIEWebhookServiceAccount(hc)
		},
	)
}

func newAIEWebhookServiceAccount(hc *hcov1beta1.HyperConverged) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      aieWebhookServiceAccountName,
			Namespace: hc.Namespace,
			Labels:    operands.GetLabels(hc, appComponent),
		},
	}
}
