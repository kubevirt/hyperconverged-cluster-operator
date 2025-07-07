package operands

import (
	"errors"
	"reflect"
	"sync"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

type newDaemonSetFunc func(hc *hcov1beta1.HyperConverged) *appsv1.DaemonSet

func NewDaemonSetHandler(Client client.Client, Scheme *runtime.Scheme, newCrFunc newDaemonSetFunc) *GenericOperand {
	return NewGenericOperand(Client, Scheme, "DaemonSet", &daemonSetHooks{newCrFunc: newCrFunc}, false)
}

type daemonSetHooks struct {
	sync.Mutex
	newCrFunc newDaemonSetFunc
	cache     *appsv1.DaemonSet
}

func (h *daemonSetHooks) GetFullCr(hc *hcov1beta1.HyperConverged) (client.Object, error) {
	h.Lock()
	defer h.Unlock()

	if h.cache == nil {
		h.cache = h.newCrFunc(hc)
	}
	return h.cache, nil
}

func (*daemonSetHooks) GetEmptyCr() client.Object {
	return &appsv1.DaemonSet{}
}

func (h *daemonSetHooks) Reset() {
	h.Lock()
	defer h.Unlock()

	h.cache = nil
}

func (*daemonSetHooks) JustBeforeComplete(_ *common.HcoRequest) { /* no implementation */ }

func (*daemonSetHooks) UpdateCR(req *common.HcoRequest, Client client.Client, exists runtime.Object, required runtime.Object) (bool, bool, error) {
	return updateDaemonSet(req, Client, exists, required)
}

func updateDaemonSet(req *common.HcoRequest, Client client.Client, exists runtime.Object, required runtime.Object) (bool, bool, error) {
	daemonSet, ok1 := required.(*appsv1.DaemonSet)
	found, ok2 := exists.(*appsv1.DaemonSet)
	if !ok1 || !ok2 {
		return false, false, errors.New("can't convert to DaemonSet")
	}

	if !hasCorrectDaemonSetFields(found, daemonSet) {
		if req.HCOTriggered {
			req.Logger.Info("Updating existing DaemonSet to new opinionated values", "name", daemonSet.Name)
		} else {
			req.Logger.Info("Reconciling an externally updated DaemonSet to its opinionated values", "name", daemonSet.Name)
		}
		if shouldRecreateDaemonSet(found, daemonSet) {
			err := Client.Delete(req.Ctx, found, &client.DeleteOptions{})
			if err != nil {
				return false, false, err
			}
			err = Client.Create(req.Ctx, daemonSet, &client.CreateOptions{})
			if err != nil {
				return false, false, err
			}
			return true, !req.HCOTriggered, nil
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

// hasCorrectDaemonSetFields compares only the fields that are intentionally set by HCO,
// ignoring fields that are automatically set by Kubernetes
func hasCorrectDaemonSetFields(found *appsv1.DaemonSet, required *appsv1.DaemonSet) bool {
	return util.CompareLabels(required, found) &&
		reflect.DeepEqual(found.Spec.Selector, required.Spec.Selector) &&
		reflect.DeepEqual(found.Spec.UpdateStrategy, required.Spec.UpdateStrategy) &&
		reflect.DeepEqual(found.Spec.Template.Spec.Containers, required.Spec.Template.Spec.Containers) &&
		reflect.DeepEqual(found.Spec.Template.Spec.ServiceAccountName, required.Spec.Template.Spec.ServiceAccountName) &&
		reflect.DeepEqual(found.Spec.Template.Spec.PriorityClassName, required.Spec.Template.Spec.PriorityClassName) &&
		reflect.DeepEqual(found.Spec.Template.Spec.Volumes, required.Spec.Template.Spec.Volumes) &&
		reflect.DeepEqual(found.Spec.Template.Spec.Affinity, required.Spec.Template.Spec.Affinity) &&
		reflect.DeepEqual(found.Spec.Template.Spec.NodeSelector, required.Spec.Template.Spec.NodeSelector) &&
		reflect.DeepEqual(found.Spec.Template.Spec.Tolerations, required.Spec.Template.Spec.Tolerations)
}

func shouldRecreateDaemonSet(found, required *appsv1.DaemonSet) bool {
	// updating LabelSelector (it's immutable) would be rejected by API server; create new DaemonSet instead
	return !reflect.DeepEqual(found.Spec.Selector, required.Spec.Selector)
}
