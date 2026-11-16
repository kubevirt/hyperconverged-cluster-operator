package operands

import (
	"errors"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

func NewCRDHandler(cli client.Client, scheme *runtime.Scheme, required *apiextensionsv1.CustomResourceDefinition) *GenericOperand {
	// Compare against the CRD as the apiserver stores it, with its defaults filled in.
	required = required.DeepCopy()
	apiextensionsv1.SetObjectDefaults_CustomResourceDefinition(required)
	return NewGenericOperand(cli, scheme, "CustomResourceDefinition", &crdHooks{required: required}, false)
}

type crdHooks struct {
	required *apiextensionsv1.CustomResourceDefinition
}

func (h *crdHooks) GetFullCr(_ *hcov1.HyperConverged) (client.Object, error) {
	return h.required.DeepCopy(), nil
}

func (h *crdHooks) GetEmptyCr() client.Object {
	return &apiextensionsv1.CustomResourceDefinition{}
}

func (h *crdHooks) UpdateCR(req *common.HcoRequest, cli client.Client, exists runtime.Object, required runtime.Object) (bool, bool, error) {
	crd, ok1 := required.(*apiextensionsv1.CustomResourceDefinition)
	found, ok2 := exists.(*apiextensionsv1.CustomResourceDefinition)
	if !ok1 || !ok2 {
		return false, false, errors.New("can't convert to CustomResourceDefinition")
	}

	if found.DeletionTimestamp != nil {
		// It can only be recreated once the apiserver finished deleting it and its CRs. A watch on
		// the CRD triggers the reconcile that recreates it.
		req.Logger.Info("Waiting for the CustomResourceDefinition to be deleted before recreating it", "name", found.Name)
		return false, false, nil
	}

	if !hcoutil.CompareLabels(crd, found) || !equality.Semantic.DeepEqual(crd.Spec, found.Spec) {
		logUpdate(req, "CustomResourceDefinition", found.Name)
		hcoutil.MergeLabels(&crd.ObjectMeta, &found.ObjectMeta)
		crd.Spec.DeepCopyInto(&found.Spec)

		if err := cli.Update(req.Ctx, found); err != nil {
			return false, false, err
		}
		return true, !req.HCOTriggered, nil
	}

	return false, false, nil
}
