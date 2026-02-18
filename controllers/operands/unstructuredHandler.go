package operands

import (
	"errors"
	"maps"
	"reflect"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

type newUnstructuredFunc func(hc *hcov1beta1.HyperConverged) *unstructured.Unstructured

func NewUnstructuredHandler(Client client.Client, Scheme *runtime.Scheme, gvk schema.GroupVersionKind, newCrFunc newUnstructuredFunc) *GenericOperand {
	return NewGenericOperand(Client, Scheme, gvk.Kind, &unstructuredHooks{gvk: gvk, newCrFunc: newCrFunc}, false)
}

type unstructuredHooks struct {
	gvk       schema.GroupVersionKind
	newCrFunc newUnstructuredFunc
}

func (h *unstructuredHooks) GetFullCr(hc *hcov1beta1.HyperConverged) (client.Object, error) {
	return h.newCrFunc(hc), nil
}

func (h *unstructuredHooks) GetEmptyCr() client.Object {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(h.gvk)
	return obj
}

func (h *unstructuredHooks) UpdateCR(req *common.HcoRequest, Client client.Client, exists runtime.Object, required runtime.Object) (bool, bool, error) {
	requiredU, ok1 := required.(*unstructured.Unstructured)
	foundU, ok2 := exists.(*unstructured.Unstructured)
	if !ok1 || !ok2 {
		return false, false, errors.New("can't convert to Unstructured")
	}

	requiredSpec, _, _ := unstructured.NestedMap(requiredU.Object, "spec")
	foundSpec, _, _ := unstructured.NestedMap(foundU.Object, "spec")

	labelChanged := !util.CompareLabels(requiredU, foundU)

	if !reflect.DeepEqual(requiredSpec, foundSpec) || labelChanged {
		if req.HCOTriggered {
			req.Logger.Info("Updating existing "+h.gvk.Kind+" to new opinionated values", "name", foundU.GetName())
		} else {
			req.Logger.Info("Reconciling an externally updated "+h.gvk.Kind+" to its opinionated values", "name", foundU.GetName())
		}

		// Merge labels (Unstructured doesn't have ObjectMeta, so do it via accessors)
		mergedLabels := foundU.GetLabels()
		if mergedLabels == nil {
			mergedLabels = make(map[string]string)
		}
		maps.Copy(mergedLabels, requiredU.GetLabels())
		foundU.SetLabels(mergedLabels)

		if requiredSpec != nil {
			if err := unstructured.SetNestedMap(foundU.Object, requiredSpec, "spec"); err != nil {
				return false, false, err
			}
		}

		err := Client.Update(req.Ctx, foundU)
		if err != nil {
			return false, false, err
		}
		return true, !req.HCOTriggered, nil
	}

	return false, false, nil
}
