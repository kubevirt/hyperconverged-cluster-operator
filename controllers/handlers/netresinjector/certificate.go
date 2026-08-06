package netresinjector

import (
	"reflect"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

const (
	certManagerAPIVersion = "cert-manager.io/v1"
	certificateKind       = "Certificate"
)

func NewCertManagerCertHandler(cli client.Client, scheme *runtime.Scheme) operands.Operand {
	return operands.NewGenericOperand(cli, scheme, certificateKind, &certHooks{}, false)
}

type certHooks struct{}

func (h *certHooks) GetFullCr(_ *hcov1.HyperConverged) (client.Object, error) {
	ns := hcoutil.GetOperatorNamespaceFromEnv()
	dnsName := serviceName + "." + ns + ".svc"

	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": certManagerAPIVersion,
			"kind":       certificateKind,
			"metadata": map[string]any{
				"name":      tlsCertificateName,
				"namespace": ns,
			},
			"spec": map[string]any{
				"secretName": tlsSecretName,
				"dnsNames": []any{
					dnsName,
				},
				"issuerRef": map[string]any{
					"name": "selfsigned",
				},
			},
		},
	}
	_ = unstructured.SetNestedStringMap(obj.Object, operands.GetLabels(hcoutil.AppComponentNetResInjector), "metadata", "labels")
	return obj, nil
}

func (h *certHooks) GetEmptyCr() client.Object {
	return &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": certManagerAPIVersion,
			"kind":       certificateKind,
		},
	}
}

func (h *certHooks) UpdateCR(req *common.HcoRequest, Client client.Client, exists, desired runtime.Object) (bool, bool, error) {
	existingCert, ok := exists.(*unstructured.Unstructured)
	if !ok {
		return false, false, nil
	}
	desiredCert, ok := desired.(*unstructured.Unstructured)
	if !ok {
		return false, false, nil
	}

	needsUpdate := false

	existingLabels, _, _ := unstructured.NestedStringMap(existingCert.Object, "metadata", "labels")
	desiredLabels, _, _ := unstructured.NestedStringMap(desiredCert.Object, "metadata", "labels")

	if !reflect.DeepEqual(existingLabels, desiredLabels) {
		needsUpdate = true
		_ = unstructured.SetNestedStringMap(existingCert.Object, desiredLabels, "metadata", "labels")
	}

	existingDNS, _, _ := unstructured.NestedStringSlice(existingCert.Object, "spec", "dnsNames")
	desiredDNS, _, _ := unstructured.NestedStringSlice(desiredCert.Object, "spec", "dnsNames")

	if !reflect.DeepEqual(existingDNS, desiredDNS) {
		needsUpdate = true
		_ = unstructured.SetNestedStringSlice(existingCert.Object, desiredDNS, "spec", "dnsNames")
	}

	existingIssuerRef, _, _ := unstructured.NestedMap(existingCert.Object, "spec", "issuerRef")
	desiredIssuerRef, _, _ := unstructured.NestedMap(desiredCert.Object, "spec", "issuerRef")

	if !reflect.DeepEqual(existingIssuerRef, desiredIssuerRef) {
		needsUpdate = true
		_ = unstructured.SetNestedMap(existingCert.Object, desiredIssuerRef, "spec", "issuerRef")
	}

	existingSecretName, _, _ := unstructured.NestedString(existingCert.Object, "spec", "secretName")
	desiredSecretName, _, _ := unstructured.NestedString(desiredCert.Object, "spec", "secretName")

	if existingSecretName != desiredSecretName {
		needsUpdate = true
		_ = unstructured.SetNestedField(existingCert.Object, desiredSecretName, "spec", "secretName")
	}

	if needsUpdate {
		if err := Client.Update(req.Ctx, existingCert); err != nil {
			return false, false, err
		}
		return true, !req.HCOTriggered, nil
	}

	return false, false, nil
}
