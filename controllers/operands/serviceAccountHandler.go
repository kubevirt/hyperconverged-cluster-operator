package operands

import (
	"errors"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

func newServiceAccountHandler(Client client.Client, Scheme *runtime.Scheme, newCrFunc newSvcAccountFunc) Operand {
	return &conditionalHandler{
		operand: &genericOperand{
			Client: Client,
			Scheme: Scheme,
			crType: "ServiceAccount",
			hooks:  &serviceAccountHooks{newCrFunc: newCrFunc},
		},
		shouldDeploy: func(hc *hcov1beta1.HyperConverged) bool {
			return hc.Spec.FeatureGates.PasstNetworkBinding != nil && *hc.Spec.FeatureGates.PasstNetworkBinding
		},
		getCRWithName: func(hc *hcov1beta1.HyperConverged) client.Object {
			return NewPasstBindingCNISA(hc)
		},
	}
}

type newSvcAccountFunc func(hc *hcov1beta1.HyperConverged) *corev1.ServiceAccount

type serviceAccountHooks struct {
	newCrFunc newSvcAccountFunc
}

func (h serviceAccountHooks) getFullCr(hc *hcov1beta1.HyperConverged) (client.Object, error) {
	return h.newCrFunc(hc), nil
}

func (serviceAccountHooks) getEmptyCr() client.Object {
	return &corev1.ServiceAccount{}
}

func (serviceAccountHooks) justBeforeComplete(_ *common.HcoRequest) { /* no implementation */ }

func (serviceAccountHooks) updateCr(req *common.HcoRequest, Client client.Client, exists runtime.Object, required runtime.Object) (bool, bool, error) {
	return updateServiceAccount(req, Client, exists, required)
}

func updateServiceAccount(req *common.HcoRequest, Client client.Client, exists runtime.Object, required runtime.Object) (bool, bool, error) {
	serviceAccount, ok1 := required.(*corev1.ServiceAccount)
	found, ok2 := exists.(*corev1.ServiceAccount)
	if !ok1 || !ok2 {
		return false, false, errors.New("can't convert to ServiceAccount")
	}
	if !hasServiceAccountRightFields(found, serviceAccount) {
		if req.HCOTriggered {
			req.Logger.Info("Updating existing ServiceAccount Spec to new opinionated values")
		} else {
			req.Logger.Info("Reconciling an externally updated ServiceAccount's Spec to its opinionated values")
		}
		util.MergeLabels(&serviceAccount.ObjectMeta, &found.ObjectMeta)
		err := Client.Update(req.Ctx, found)
		if err != nil {
			return false, false, err
		}
		return true, !req.HCOTriggered, nil
	}
	return false, false, nil
}

func hasServiceAccountRightFields(found *corev1.ServiceAccount, required *corev1.ServiceAccount) bool {
	return util.CompareLabels(required, found)
}
