package operands

import (
	"errors"
	"reflect"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
)

func newDeploymentHandler(Client client.Client, Scheme *runtime.Scheme, required *appsv1.Deployment) Operand {
	h := &genericOperand{
		Client: Client,
		Scheme: Scheme,
		crType: "Deployment",
		hooks:  &deploymentHooks{required: required},
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

func (deploymentHooks) justBeforeComplete(_ *common.HcoRequest) { /* no implementation */ }

func (h deploymentHooks) updateCr(req *common.HcoRequest, Client client.Client, exists runtime.Object, required runtime.Object) (bool, bool, error) {
	requiredDep, ok1 := required.(*appsv1.Deployment)
	foundDep, ok2 := exists.(*appsv1.Deployment)

	if !ok1 || !ok2 {
		return false, false, errors.New("can't convert to Deployment")
	}
	if !hasCorrectDeploymentFields(foundDep, requiredDep) {
		if req.HCOTriggered {
			req.Logger.Info("Updating existing Deployment to new opinionated values", "name", h.required.Name)
		} else {
			req.Logger.Info("Reconciling an externally updated Deployment to its opinionated values", "name", h.required.Name)
		}
		if recreateDep(foundDep.Spec.Selector, requiredDep.Spec.Selector) {
			// updating LabelSelector (it's immutable) would be rejected by API server; create new Deployment instead
			err := Client.Delete(req.Ctx, foundDep, &client.DeleteOptions{})
			if err != nil {
				return false, false, err
			}
			err = Client.Create(req.Ctx, requiredDep, &client.CreateOptions{})
			if err != nil {
				return false, false, err
			}
			requiredDep.DeepCopyInto(foundDep)
			return true, !req.HCOTriggered, nil
		}
		// LabelSelector hasn't changed, so we only update the Deployment
		util.DeepCopyLabels(&requiredDep.ObjectMeta, &foundDep.ObjectMeta)
		requiredDep.DeepCopyInto(foundDep)
		err := Client.Update(req.Ctx, foundDep)
		if err != nil {
			return false, false, err
		}
		return true, !req.HCOTriggered, nil
	}
	return false, false, nil
}

// We need to check only certain fields in the deployment resource, since some of the fields
// are being set by k8s.
func hasCorrectDeploymentFields(found *appsv1.Deployment, required *appsv1.Deployment) bool {
	return reflect.DeepEqual(found.Labels, required.Labels) &&
		reflect.DeepEqual(found.Spec.Selector, required.Spec.Selector) &&
		reflect.DeepEqual(found.Spec.Replicas, required.Spec.Replicas) &&
		reflect.DeepEqual(found.Spec.Template.Spec.Containers, required.Spec.Template.Spec.Containers) &&
		reflect.DeepEqual(found.Spec.Template.Spec.ServiceAccountName, required.Spec.Template.Spec.ServiceAccountName) &&
		reflect.DeepEqual(found.Spec.Template.Spec.PriorityClassName, required.Spec.Template.Spec.PriorityClassName)
}

// recreateDep indicates if the Deployment should be recreated, which is decided based on the diff between LabelSelector
// values for new and existing Deployments
func recreateDep(found, required *metav1.LabelSelector) bool {
	return !reflect.DeepEqual(found, required)
}
