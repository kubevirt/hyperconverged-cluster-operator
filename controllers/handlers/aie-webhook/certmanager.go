package aie_webhook

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
)

var (
	issuerGVK = schema.GroupVersionKind{
		Group:   "cert-manager.io",
		Version: "v1",
		Kind:    "Issuer",
	}
	certificateGVK = schema.GroupVersionKind{
		Group:   "cert-manager.io",
		Version: "v1",
		Kind:    "Certificate",
	}
)

func NewAIEWebhookIssuerHandler(Client client.Client, Scheme *runtime.Scheme) operands.Operand {
	return operands.NewConditionalHandler(
		operands.NewUnstructuredHandler(Client, Scheme, issuerGVK, newAIEWebhookIssuer),
		shouldDeployAIEWebhook,
		func(hc *hcov1beta1.HyperConverged) client.Object {
			return newAIEWebhookIssuerWithNameOnly(hc)
		},
	)
}

func NewAIEWebhookCertificateHandler(Client client.Client, Scheme *runtime.Scheme) operands.Operand {
	return operands.NewConditionalHandler(
		operands.NewUnstructuredHandler(Client, Scheme, certificateGVK, newAIEWebhookCertificate),
		shouldDeployAIEWebhook,
		func(hc *hcov1beta1.HyperConverged) client.Object {
			return newAIEWebhookCertificateWithNameOnly(hc)
		},
	)
}

func newAIEWebhookIssuerWithNameOnly(hc *hcov1beta1.HyperConverged) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(issuerGVK)
	obj.SetName(aieWebhookIssuerName)
	obj.SetNamespace(hc.Namespace)
	obj.SetLabels(labelsMap(hc))
	return obj
}

func newAIEWebhookIssuer(hc *hcov1beta1.HyperConverged) *unstructured.Unstructured {
	obj := newAIEWebhookIssuerWithNameOnly(hc)
	obj.Object["spec"] = map[string]interface{}{
		"selfSigned": map[string]interface{}{},
	}
	return obj
}

func newAIEWebhookCertificateWithNameOnly(hc *hcov1beta1.HyperConverged) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(certificateGVK)
	obj.SetName(aieWebhookCertificateName)
	obj.SetNamespace(hc.Namespace)
	obj.SetLabels(labelsMap(hc))
	return obj
}

func newAIEWebhookCertificate(hc *hcov1beta1.HyperConverged) *unstructured.Unstructured {
	obj := newAIEWebhookCertificateWithNameOnly(hc)
	obj.Object["spec"] = map[string]interface{}{
		"secretName": aieWebhookCertificateName,
		"issuerRef": map[string]interface{}{
			"name": aieWebhookIssuerName,
			"kind": "Issuer",
		},
		"dnsNames": []interface{}{
			fmt.Sprintf("%s.%s.svc", aieWebhookName, hc.Namespace),
			fmt.Sprintf("%s.%s.svc.cluster.local", aieWebhookName, hc.Namespace),
		},
	}
	return obj
}

func labelsMap(hc *hcov1beta1.HyperConverged) map[string]string {
	return operands.GetLabels(hc, appComponent)
}
