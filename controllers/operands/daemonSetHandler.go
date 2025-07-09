package operands

import (
	"errors"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

type newDaemonSetFunc func(hc *hcov1beta1.HyperConverged, isOpenShift bool) *appsv1.DaemonSet

func NewDaemonSetHandler(Client client.Client, Scheme *runtime.Scheme, isOpenShift bool, newCrFunc newDaemonSetFunc) *GenericOperand {
	return NewGenericOperand(Client, Scheme, "DaemonSet", &daemonSetHooks{newCrFunc: newCrFunc, isOpenShift: isOpenShift}, false)
}

type daemonSetHooks struct {
	newCrFunc   newDaemonSetFunc
	isOpenShift bool
}

func (h daemonSetHooks) GetFullCr(hc *hcov1beta1.HyperConverged) (client.Object, error) {
	return h.newCrFunc(hc, h.isOpenShift), nil
}

func (daemonSetHooks) GetEmptyCr() client.Object {
	return &appsv1.DaemonSet{}
}

func (daemonSetHooks) JustBeforeComplete(_ *common.HcoRequest) { /* no implementation */ }

func (daemonSetHooks) UpdateCR(req *common.HcoRequest, Client client.Client, exists runtime.Object, required runtime.Object) (bool, bool, error) {
	return updateDaemonSet(req, Client, exists, required)
}

func updateDaemonSet(req *common.HcoRequest, Client client.Client, exists runtime.Object, required runtime.Object) (bool, bool, error) {
	daemonSet, ok1 := required.(*appsv1.DaemonSet)
	found, ok2 := exists.(*appsv1.DaemonSet)
	if !ok1 || !ok2 {
		return false, false, errors.New("can't convert to DaemonSet")
	}
	if !hasDaemonSetRightFields(found, daemonSet) {
		if req.HCOTriggered {
			req.Logger.Info("Updating existing DaemonSet Spec to new opinionated values")
		} else {
			req.Logger.Info("Reconciling an externally updated DaemonSet's Spec to its opinionated values")
		}
		util.MergeLabels(&daemonSet.ObjectMeta, &found.ObjectMeta)
		daemonSet.Spec.DeepCopyInto(&found.Spec)
		err := Client.Update(req.Ctx, found)
		if err != nil {
			return false, false, err
		}
		return true, !req.HCOTriggered, nil
	}
	return false, false, nil
}

func hasDaemonSetRightFields(found *appsv1.DaemonSet, required *appsv1.DaemonSet) bool {
	return util.CompareLabels(required, found)
}
