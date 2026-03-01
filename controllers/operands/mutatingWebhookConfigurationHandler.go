package operands

import (
	"errors"
	"reflect"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

type newMutatingWebhookConfigurationFunc func(hc *hcov1beta1.HyperConverged) *admissionregistrationv1.MutatingWebhookConfiguration

func NewMutatingWebhookConfigurationHandler(Client client.Client, Scheme *runtime.Scheme, required *admissionregistrationv1.MutatingWebhookConfiguration) *GenericOperand {
	return NewGenericOperand(Client, Scheme, "MutatingWebhookConfiguration", &mutatingWebhookConfigurationHooks{required: required}, false)
}

type mutatingWebhookConfigurationHooks struct {
	required *admissionregistrationv1.MutatingWebhookConfiguration
}

func (h *mutatingWebhookConfigurationHooks) GetFullCr(_ *hcov1beta1.HyperConverged) (client.Object, error) {
	return h.required.DeepCopy(), nil
}

func (h *mutatingWebhookConfigurationHooks) GetEmptyCr() client.Object {
	return &admissionregistrationv1.MutatingWebhookConfiguration{}
}

func (h *mutatingWebhookConfigurationHooks) UpdateCR(req *common.HcoRequest, Client client.Client, exists runtime.Object, required runtime.Object) (bool, bool, error) {
	mwc, ok1 := required.(*admissionregistrationv1.MutatingWebhookConfiguration)
	found, ok2 := exists.(*admissionregistrationv1.MutatingWebhookConfiguration)
	if !ok1 || !ok2 {
		return false, false, errors.New("can't convert to MutatingWebhookConfiguration")
	}

	if !util.CompareLabels(mwc, found) ||
		!reflect.DeepEqual(mwc.Webhooks, found.Webhooks) ||
		!reflect.DeepEqual(mwc.Annotations, found.Annotations) {

		if req.HCOTriggered {
			req.Logger.Info("Updating existing MutatingWebhookConfiguration to new opinionated values", "name", found.Name)
		} else {
			req.Logger.Info("Reconciling an externally updated MutatingWebhookConfiguration to its opinionated values", "name", found.Name)
		}

		util.MergeLabels(&mwc.ObjectMeta, &found.ObjectMeta)
		found.Webhooks = make([]admissionregistrationv1.MutatingWebhook, len(mwc.Webhooks))
		for i := range mwc.Webhooks {
			mwc.Webhooks[i].DeepCopyInto(&found.Webhooks[i])
		}
		if mwc.Annotations != nil {
			if found.Annotations == nil {
				found.Annotations = make(map[string]string)
			}
			for k, v := range mwc.Annotations {
				found.Annotations[k] = v
			}
		}

		err := Client.Update(req.Ctx, found)
		if err != nil {
			return false, false, err
		}
		return true, !req.HCOTriggered, nil
	}

	return false, false, nil
}
