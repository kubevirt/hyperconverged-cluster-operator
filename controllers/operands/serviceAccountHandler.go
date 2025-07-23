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

type newSvcAccountFunc func(hc *hcov1beta1.HyperConverged) *corev1.ServiceAccount

func NewServiceAccountHandler(Client client.Client, Scheme *runtime.Scheme, newCrFunc newSvcAccountFunc) *GenericOperand {
	return NewGenericOperand(Client, Scheme, "ServiceAccount", &serviceAccountHooks{newCrFunc: newCrFunc}, false)
}

type serviceAccountHooks struct {
	newCrFunc newSvcAccountFunc
}

func (h serviceAccountHooks) GetFullCr(hc *hcov1beta1.HyperConverged) (client.Object, error) {
	return h.newCrFunc(hc), nil
}

func (serviceAccountHooks) GetEmptyCr() client.Object {
	return &corev1.ServiceAccount{}
}

func (serviceAccountHooks) JustBeforeComplete(_ *common.HcoRequest) { /* no implementation */ }

func (serviceAccountHooks) UpdateCR(req *common.HcoRequest, Client client.Client, exists runtime.Object, required runtime.Object) (bool, bool, error) {
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
