package operands

import (
	"errors"
	"reflect"

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

func NewNetworkPolicyHandler(Client client.Client, Scheme *runtime.Scheme, required *networkingv1.NetworkPolicy) *GenericOperand {
	return NewGenericOperand(Client, Scheme, "NetworkPolicy", newNetworkPolicyHook(required), false)
}

type networkPolicyHook struct {
	required *networkingv1.NetworkPolicy
}

func newNetworkPolicyHook(required *networkingv1.NetworkPolicy) *networkPolicyHook {
	return &networkPolicyHook{
		required: required,
	}
}

func (nph *networkPolicyHook) GetFullCr(_ *hcov1beta1.HyperConverged) (client.Object, error) {
	return nph.required.DeepCopy(), nil
}

func (nph *networkPolicyHook) GetEmptyCr() client.Object {
	return &networkingv1.NetworkPolicy{}
}

func (nph *networkPolicyHook) UpdateCR(req *common.HcoRequest, cli client.Client, exists, required runtime.Object) (bool, bool, error) {
	np, ok1 := required.(*networkingv1.NetworkPolicy)
	found, ok2 := exists.(*networkingv1.NetworkPolicy)
	if !ok1 || !ok2 {
		return false, false, errors.New("can't convert to NetworkPolicy")
	}

	if !reflect.DeepEqual(found.Spec, np.Spec) || !hcoutil.CompareLabels(np, found) {
		overwritten := false

		if req.HCOTriggered {
			req.Logger.Info("Updating existing NetworkPolicy's Spec to new opinionated values")
		} else {
			req.Logger.Info("Reconciling an externally updated NetworkPolicy's Spec to its opinionated values")
			overwritten = true
		}

		hcoutil.MergeLabels(&np.ObjectMeta, &found.ObjectMeta)
		np.Spec.DeepCopyInto(&found.Spec)

		err := cli.Update(req.Ctx, found)
		if err != nil {
			return false, false, err
		}
		return true, overwritten, nil
	}
	return false, false, nil
}
