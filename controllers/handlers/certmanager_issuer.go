package handlers

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
	issuerName            = "selfsigned"
	issuerKind            = "Issuer"
)

func NewCertManagerIssuerHandler(cli client.Client, scheme *runtime.Scheme) operands.Operand {
	return operands.NewGenericOperand(cli, scheme, issuerKind, &issuerHooks{}, false)
}

type issuerHooks struct{}

func (h *issuerHooks) GetFullCr(_ *hcov1.HyperConverged) (client.Object, error) {
	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": certManagerAPIVersion,
			"kind":       issuerKind,
			"metadata": map[string]any{
				"name":      issuerName,
				"namespace": hcoutil.GetOperatorNamespaceFromEnv(),
			},
			"spec": map[string]any{
				"selfSigned": map[string]any{},
			},
		},
	}
	_ = unstructured.SetNestedStringMap(obj.Object, operands.GetLabels(hcoutil.AppComponentCompute), "metadata", "labels")
	return obj, nil
}

func (h *issuerHooks) GetEmptyCr() client.Object {
	return &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": certManagerAPIVersion,
			"kind":       issuerKind,
		},
	}
}

func (h *issuerHooks) UpdateCR(req *common.HcoRequest, Client client.Client, exists, desired runtime.Object) (bool, bool, error) {
	existingIssuer, ok := exists.(*unstructured.Unstructured)
	if !ok {
		return false, false, nil
	}

	desiredIssuer, ok := desired.(*unstructured.Unstructured)
	if !ok {
		return false, false, nil
	}

	existingLabels, _, _ := unstructured.NestedStringMap(existingIssuer.Object, "metadata", "labels")
	desiredLabels, _, _ := unstructured.NestedStringMap(desiredIssuer.Object, "metadata", "labels")

	if !reflect.DeepEqual(existingLabels, desiredLabels) {
		_ = unstructured.SetNestedStringMap(existingIssuer.Object, desiredLabels, "metadata", "labels")
		if err := Client.Update(req.Ctx, existingIssuer); err != nil {
			return false, false, err
		}
		return true, !req.HCOTriggered, nil
	}

	return false, false, nil
}
