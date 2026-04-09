package aie

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

func NewAIEWebhookConfigMapHandler(Client client.Client, Scheme *runtime.Scheme) operands.Operand {
	cm := newAIEWebhookConfigMap()
	return operands.NewConditionalHandler(
		operands.NewCmHandler(Client, Scheme, cm),
		shouldDeployAIE,
		func(hc *hcov1beta1.HyperConverged) client.Object {
			return newAIEWebhookConfigMapWithNameOnly()
		},
	)
}

func newAIEWebhookConfigMapWithNameOnly() *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      aieWebhookConfigMapName,
			Namespace: hcoutil.GetOperatorNamespaceFromEnv(),
			Labels:    operands.GetLabels(appComponent),
		},
	}
}

func newAIEWebhookConfigMap() *corev1.ConfigMap {
	cm := newAIEWebhookConfigMapWithNameOnly()
	cm.Data = map[string]string{
		"config.yaml": "rules:\n",
	}
	return cm
}
