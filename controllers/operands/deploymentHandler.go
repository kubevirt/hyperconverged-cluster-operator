package operands

import (
	"errors"
	"reflect"

	appsv1 "k8s.io/api/apps/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/pkg/apis/hco/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/controller/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

func newDeploymentHandler(Client client.Client, Scheme *runtime.Scheme, required *appsv1.Deployment) Operand {
	h := &genericOperand{
		Client:              Client,
		Scheme:              Scheme,
		crType:              "Deployment",
		removeExistingOwner: false,
		hooks:               &deploymentHooks{required: required},
	}

	return h
}

type deploymentHooks struct {
	required *appsv1.Deployment
}

func (h deploymentHooks) getFullCr(_ *hcov1beta1.HyperConverged) (client.Object, error) {
	return h.required.DeepCopy(), nil
}

func (h deploymentHooks) getEmptyCr() client.Object {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: h.required.Name,
		},
	}
}

func (h deploymentHooks) getObjectMeta(cr runtime.Object) *metav1.ObjectMeta {
	return &cr.(*appsv1.Deployment).ObjectMeta
}

func (h deploymentHooks) reset() { /* no implementation */ }

func (h deploymentHooks) updateCr(req *common.HcoRequest, Client client.Client, exists runtime.Object, _ runtime.Object) (bool, bool, error) {
	found, ok := exists.(*appsv1.Deployment)

	if !ok {
		return false, false, errors.New("can't convert to Deployment")
	}

	if !reflect.DeepEqual(found, h.required) ||
		!reflect.DeepEqual(found.Labels, h.required.Labels) {
		if req.HCOTriggered {
			req.Logger.Info("Updating existing Deployment to new opinionated values", "name", h.required.Name)
		} else {
			req.Logger.Info("Reconciling an externally updated Deployment to its opinionated values", "name", h.required.Name)
		}
		util.DeepCopyLabels(&h.required.ObjectMeta, &found.ObjectMeta)
		h.required.DeepCopyInto(found)
		err := Client.Update(req.Ctx, found)
		if err != nil {
			return false, false, err
		}
		return true, !req.HCOTriggered, nil
	}

	return false, false, nil
}
