package operands

import (
	"reflect"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

const (
	certManagerAPIVersion = "cert-manager.io/v1"
	certificateKind       = "Certificate"
)

// NewCertificate returns a cert-manager Certificate in the operator namespace, issued by the
// selfsigned Issuer HCO deploys on Kubernetes.
func NewCertificate(name, secretName string, dnsNames []string, component hcoutil.AppComponent) *unstructured.Unstructured {
	dns := make([]any, len(dnsNames))
	for i, n := range dnsNames {
		dns[i] = n
	}
	cert := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": certManagerAPIVersion,
			"kind":       certificateKind,
			"metadata": map[string]any{
				"name":      name,
				"namespace": hcoutil.GetOperatorNamespaceFromEnv(),
			},
			"spec": map[string]any{
				"secretName": secretName,
				"dnsNames":   dns,
				"issuerRef": map[string]any{
					"name": "selfsigned",
				},
			},
		},
	}
	cert.SetLabels(GetLabels(component))
	return cert
}

// NewCertificateHandler reconciles a cert-manager Certificate built by NewCertificate.
func NewCertificateHandler(cli client.Client, scheme *runtime.Scheme, required *unstructured.Unstructured) *GenericOperand {
	return NewGenericOperand(cli, scheme, certificateKind, &certHooks{required: required}, false)
}

type certHooks struct {
	required *unstructured.Unstructured
}

func (h *certHooks) GetFullCr(_ *hcov1.HyperConverged) (client.Object, error) {
	return h.required.DeepCopy(), nil
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
