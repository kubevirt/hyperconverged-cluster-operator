package aie_webhook

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
)

func NewAIEWebhookConfigMapHandler(Client client.Client, Scheme *runtime.Scheme) operands.Operand {
	return operands.NewConditionalHandler(
		operands.NewDynamicCmHandler(Client, Scheme, newAIEWebhookConfigMap),
		shouldDeployAIEWebhook,
		func(hc *hcov1beta1.HyperConverged) client.Object {
			return newAIEWebhookConfigMapWithNameOnly(hc)
		},
	)
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

func newAIEWebhookConfigMap(hc *hcov1beta1.HyperConverged) (*corev1.ConfigMap, error) {
	cm := newAIEWebhookConfigMapWithNameOnly(hc)
	cm.Data = map[string]string{
		"config.yaml": renderConfigYAML(hc),
	}
	return cm, nil
}

func renderConfigYAML(hc *hcov1beta1.HyperConverged) string {
	var b strings.Builder
	b.WriteString("rules:\n")

	if hc.Spec.AIEWebhookConfig == nil {
		return b.String()
	}

	for _, rule := range hc.Spec.AIEWebhookConfig.Rules {
		b.WriteString(fmt.Sprintf("- name: %q\n", rule.Name))
		b.WriteString(fmt.Sprintf("  image: %q\n", rule.Image))
		b.WriteString("  selector:\n")

		if len(rule.Selector.DeviceNames) > 0 {
			b.WriteString("    deviceNames:\n")
			for _, dn := range rule.Selector.DeviceNames {
				b.WriteString(fmt.Sprintf("    - %q\n", dn))
			}
		}

		if rule.Selector.VMLabels != nil && len(rule.Selector.VMLabels.MatchLabels) > 0 {
			b.WriteString("    vmLabels:\n")
			b.WriteString("      matchLabels:\n")
			for k, v := range rule.Selector.VMLabels.MatchLabels {
				b.WriteString(fmt.Sprintf("        %s: %q\n", k, v))
			}
		}
	}

	return b.String()
}
