package aie_webhook

import (
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
)

func NewAIEWebhookConfigMapHandler(_ logr.Logger, Client client.Client, Scheme *runtime.Scheme, hc *hcov1beta1.HyperConverged) (operands.Operand, error) {
	cm := newAIEWebhookConfigMap(hc)
	return operands.NewConditionalHandler(
		operands.NewCmHandler(Client, Scheme, cm),
		shouldDeployAIEWebhook,
		func(hc *hcov1beta1.HyperConverged) client.Object {
			return newAIEWebhookConfigMapWithNameOnly(hc)
		},
	), nil
}

func newAIEWebhookConfigMapWithNameOnly(hc *hcov1beta1.HyperConverged) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      aieWebhookConfigMapName,
			Namespace: hc.Namespace,
			Labels:    operands.GetLabels(hc, appComponent),
		},
	}
}

func newAIEWebhookConfigMap(hc *hcov1beta1.HyperConverged) *corev1.ConfigMap {
	cm := newAIEWebhookConfigMapWithNameOnly(hc)
	cm.Data = map[string]string{
		"config.yaml": "rules:\n",
	}
	return cm
}
