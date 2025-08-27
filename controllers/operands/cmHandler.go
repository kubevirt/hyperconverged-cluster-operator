package operands

import (
	"errors"
	"maps"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

func NewCmHandler(Client client.Client, Scheme *runtime.Scheme, required *corev1.ConfigMap) *GenericOperand {
	return NewGenericOperand(Client, Scheme, "ConfigMap", &cmHooks{required: required}, false)
}

type cmHooks struct {
	required *corev1.ConfigMap
}

func (h cmHooks) GetFullCr(_ *hcov1beta1.HyperConverged) (client.Object, error) {
	return h.required.DeepCopy(), nil
}

func (h cmHooks) GetEmptyCr() client.Object {
	return &corev1.ConfigMap{}
}

func (h cmHooks) UpdateCR(req *common.HcoRequest, Client client.Client, exists runtime.Object, _ runtime.Object) (bool, bool, error) {
	found, ok := exists.(*corev1.ConfigMap)

	if !ok {
		return false, false, errors.New("can't convert to Configmap")
	}

	labelChanged := !util.CompareLabels(h.required, found)
	if labelChanged {
		util.MergeLabels(&h.required.ObjectMeta, &found.ObjectMeta)
	}

	// Don't reconcile contents of UI settings config maps
	if label, exist := found.Labels[util.AppLabelComponent]; exist && label == string(util.AppComponentUIConfig) {
		if labelChanged {
			err := Client.Update(req.Ctx, found)
			if err != nil {
				return false, false, err
			}
		}
		return labelChanged, false, nil
	}

	if !reflect.DeepEqual(found.Data, h.required.Data) || labelChanged {
		if req.HCOTriggered {
			req.Logger.Info("Updating existing ConfigMap to new opinionated values", "name", h.required.Name)
		} else {
			req.Logger.Info("Reconciling an externally updated ConfigMap to its opinionated values", "name", h.required.Name)
		}
		found.Data = maps.Clone(h.required.Data)
		err := Client.Update(req.Ctx, found)
		if err != nil {
			return false, false, err
		}
		return true, !req.HCOTriggered, nil
	}

	return labelChanged, false, nil
}
