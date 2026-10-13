package operands

import (
	"errors"
	"fmt"
	"reflect"
	"sync"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

type newDeploymentFunc func(hc *hcov1.HyperConverged) *appsv1.Deployment

func NewDeploymentHandler(cli client.Client, scheme *runtime.Scheme, deploymentGenerator newDeploymentFunc, conditionType string) *GenericOperand {
	return NewGenericOperand(cli, scheme, "Deployment", newDeploymentHooks(deploymentGenerator, conditionType), false)
}

func SetDeploymentCondition(req *common.HcoRequest, conditionType string, status metav1.ConditionStatus, reason, message string) {
	cond := metav1.Condition{
		Type:               conditionType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: req.Instance.Generation,
	}
	changed := meta.SetStatusCondition(&req.Instance.Status.Conditions, cond)
	if changed {
		req.StatusDirty = true
	}
}

func RemoveDeploymentCondition(req *common.HcoRequest, conditionType string) {
	changed := meta.RemoveStatusCondition(&req.Instance.Status.Conditions, conditionType)
	if changed {
		req.StatusDirty = true
	}
}

type deploymentHooks struct {
	sync.Mutex
	deploymentGenerator newDeploymentFunc
	conditionType       string
	cache               *appsv1.Deployment
}

func newDeploymentHooks(deploymentGenerator newDeploymentFunc, conditionType string) *deploymentHooks {
	return &deploymentHooks{
		deploymentGenerator: deploymentGenerator,
		conditionType:       conditionType,
	}
}

func (h *deploymentHooks) Cleanup(req *common.HcoRequest) error {
	if h.conditionType == "" {
		return nil
	}
	RemoveDeploymentCondition(req, h.conditionType)
	return nil
}

func (h *deploymentHooks) PostEnsure(req *common.HcoRequest, cli client.Client) error {
	if h.conditionType == "" {
		return nil
	}

	fullCr, err := h.GetFullCr(req.Instance)
	if err != nil {
		return err
	}

	dep := &appsv1.Deployment{}
	key := client.ObjectKeyFromObject(fullCr)
	if err := cli.Get(req.Ctx, key, dep); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		SetDeploymentCondition(req, h.conditionType, metav1.ConditionFalse, "DeploymentNotFound",
			fmt.Sprintf("%s deployment not found", key.Name))
		return nil
	}

	desired := int32(0)
	if dep.Spec.Replicas != nil {
		desired = *dep.Spec.Replicas
	}
	if desired > 0 &&
		dep.Status.ObservedGeneration >= dep.Generation &&
		dep.Status.UpdatedReplicas == desired &&
		dep.Status.Replicas == desired &&
		dep.Status.ReadyReplicas == desired {
		SetDeploymentCondition(req, h.conditionType, metav1.ConditionTrue, "DeploymentReady", "All replicas are ready")
	} else {
		SetDeploymentCondition(req, h.conditionType, metav1.ConditionFalse, "DeploymentNotReady",
			fmt.Sprintf("%d/%d replicas ready", dep.Status.ReadyReplicas, desired))
	}

	return nil
}

func (h *deploymentHooks) Reset() {
	h.Lock()
	defer h.Unlock()

	h.cache = nil
}

func (h *deploymentHooks) GetFullCr(hc *hcov1.HyperConverged) (client.Object, error) {
	h.Lock()
	defer h.Unlock()

	if h.cache == nil {
		h.cache = h.deploymentGenerator(hc)
	}

	return h.cache, nil
}

func (h *deploymentHooks) GetEmptyCr() client.Object {
	return &appsv1.Deployment{}
}

func (h *deploymentHooks) UpdateCR(req *common.HcoRequest, Client client.Client, exists runtime.Object, required runtime.Object) (bool, bool, error) {
	deployment, ok1 := required.(*appsv1.Deployment)
	found, ok2 := exists.(*appsv1.Deployment)

	if !ok1 || !ok2 {
		return false, false, errors.New("can't convert to Deployment")
	}
	if !hasCorrectDeploymentFields(found, deployment) {
		if req.HCOTriggered {
			req.Logger.Info("Updating existing Deployment to new opinionated values", "name", deployment.Name)
		} else {
			req.Logger.Info("Reconciling an externally updated Deployment to its opinionated values", "name", deployment.Name)
		}
		if shouldRecreate(found, deployment) {
			err := Client.Delete(req.Ctx, found, &client.DeleteOptions{})
			if err != nil {
				return false, false, err
			}
			err = Client.Create(req.Ctx, deployment, &client.CreateOptions{})
			if err != nil {
				return false, false, err
			}
			return true, !req.HCOTriggered, nil
		}
		util.MergeLabels(&deployment.ObjectMeta, &found.ObjectMeta)
		deployment.Spec.DeepCopyInto(&found.Spec)
		err := Client.Update(req.Ctx, found)
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
	return util.CompareLabels(required, found) &&
		reflect.DeepEqual(found.Spec.Selector, required.Spec.Selector) &&
		reflect.DeepEqual(found.Spec.Replicas, required.Spec.Replicas) &&
		reflect.DeepEqual(found.Spec.Template.Spec.Containers, required.Spec.Template.Spec.Containers) &&
		reflect.DeepEqual(found.Spec.Template.Spec.ServiceAccountName, required.Spec.Template.Spec.ServiceAccountName) &&
		reflect.DeepEqual(found.Spec.Template.Spec.PriorityClassName, required.Spec.Template.Spec.PriorityClassName) &&
		reflect.DeepEqual(found.Spec.Template.Spec.Affinity, required.Spec.Template.Spec.Affinity) &&
		reflect.DeepEqual(found.Spec.Template.Spec.NodeSelector, required.Spec.Template.Spec.NodeSelector) &&
		reflect.DeepEqual(found.Spec.Template.Spec.Tolerations, required.Spec.Template.Spec.Tolerations)
}

func shouldRecreate(found, required *appsv1.Deployment) bool {
	// updating LabelSelector (it's immutable) would be rejected by API server; create new Deployment instead
	return !reflect.DeepEqual(found.Spec.Selector, required.Spec.Selector)
}
