package operands

import (
	"errors"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

func NewServiceHandler(Client client.Client, Scheme *runtime.Scheme, svc *corev1.Service) *GenericOperand {
	h := &GenericOperand{
		Client:                 Client,
		Scheme:                 Scheme,
		crType:                 "Service",
		hooks:                  &serviceHooks{svc: svc},
		setControllerReference: true,
	}

	return h
}

type serviceHooks struct {
	svc *corev1.Service
}

func (h serviceHooks) GetFullCr(_ *hcov1beta1.HyperConverged) (client.Object, error) {
	return h.svc.DeepCopy(), nil
}

func (serviceHooks) GetEmptyCr() client.Object {
	return &corev1.Service{}
}

func (serviceHooks) UpdateCR(req *common.HcoRequest, Client client.Client, exists runtime.Object, required runtime.Object) (bool, bool, error) {
	return updateService(req, Client, exists, required)
}

func updateService(req *common.HcoRequest, Client client.Client, exists runtime.Object, required runtime.Object) (bool, bool, error) {
	service, ok1 := required.(*corev1.Service)
	found, ok2 := exists.(*corev1.Service)
	if !ok1 || !ok2 {
		return false, false, errors.New("can't convert to Service")
	}
	if !HasServiceRightFields(found, service) {
		if req.HCOTriggered {
			req.Logger.Info("Updating existing Service Spec to new opinionated values")
		} else {
			req.Logger.Info("Reconciling an externally updated Service's Spec to its opinionated values")
		}
		util.MergeLabels(&service.ObjectMeta, &found.ObjectMeta)
		service.Spec.ClusterIP = found.Spec.ClusterIP
		service.Spec.DeepCopyInto(&found.Spec)
		err := Client.Update(req.Ctx, found)
		if err != nil {
			return false, false, err
		}
		return true, !req.HCOTriggered, nil
	}
	return false, false, nil
}

// HasServiceRightFields checks if the found Service object has the right fields
// We need to check only certain fields of Service object. Since there
// are some fields in the Spec that are set by k8s like "clusterIP", "ipFamilyPolicy", etc.
// When we compare current spec with expected spec by using reflect.DeepEqual, it
// never returns true.
func HasServiceRightFields(found *corev1.Service, required *corev1.Service) bool {
	return util.CompareLabels(required, found) &&
		reflect.DeepEqual(required.Spec.Selector, found.Spec.Selector) &&
		reflect.DeepEqual(required.Spec.Ports, found.Spec.Ports)
}
