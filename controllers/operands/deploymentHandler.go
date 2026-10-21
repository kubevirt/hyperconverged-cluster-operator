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

func NewDeploymentHandler(cli client.Client, Scheme *runtime.Scheme, deploymentGenerator newDeploymentFunc) *GenericOperand {
	return NewGenericOperand(cli, Scheme, "Deployment", newDeploymentHooks(deploymentGenerator), false)
}

// NewDeploymentHandlerWithReadyCondition returns a Deployment handler that also reports a
// readiness condition on the HyperConverged status after each Ensure.
func NewDeploymentHandlerWithReadyCondition(
	cli client.Client,
	scheme *runtime.Scheme,
	deploymentGenerator newDeploymentFunc,
	conditionType string,
) Operand {
	return &deploymentReadyConditionHandler{
		inner:               NewDeploymentHandler(cli, scheme, deploymentGenerator),
		client:              cli,
		conditionType:       conditionType,
		deploymentGenerator: deploymentGenerator,
	}
}

// NewConditionalDeploymentHandlerWithReadyCondition is like NewDeploymentHandlerWithReadyCondition,
// but only deploys when shouldDeploy returns true. When disabled, the readiness condition is removed.
func NewConditionalDeploymentHandlerWithReadyCondition(
	cli client.Client,
	scheme *runtime.Scheme,
	deploymentGenerator newDeploymentFunc,
	shouldDeploy ConditionFunc,
	getCRWithName GetCRWithNameFunc,
	conditionType string,
) Operand {
	return &deploymentReadyConditionHandler{
		inner: NewConditionalHandler(
			NewDeploymentHandler(cli, scheme, deploymentGenerator),
			shouldDeploy,
			getCRWithName,
		),
		client:              cli,
		conditionType:       conditionType,
		deploymentGenerator: deploymentGenerator,
		shouldDeploy:        shouldDeploy,
	}
}

type deploymentReadyConditionHandler struct {
	inner               Operand
	client              client.Client
	conditionType       string
	deploymentGenerator newDeploymentFunc
	shouldDeploy        ConditionFunc
}

func (h *deploymentReadyConditionHandler) Ensure(req *common.HcoRequest) *EnsureResult {
	result := h.inner.Ensure(req)
	if result.Err != nil {
		return result
	}

	if h.shouldDeploy != nil && !h.shouldDeploy(req.Instance) {
		RemoveDeploymentCondition(req, h.conditionType)
		return result
	}

	dep := &appsv1.Deployment{}
	key := client.ObjectKeyFromObject(h.deploymentGenerator(req.Instance))
	if err := h.client.Get(req.Ctx, key, dep); err != nil {
		if !apierrors.IsNotFound(err) {
			result.Err = err
			return result
		}
		SetDeploymentCondition(req, h.conditionType, metav1.ConditionFalse, "DeploymentNotFound",
			fmt.Sprintf("%s deployment not found", key.Name))
		return result
	}

	if dep.Spec.Replicas != nil && *dep.Spec.Replicas > 0 && dep.Status.ReadyReplicas == *dep.Spec.Replicas {
		SetDeploymentCondition(req, h.conditionType, metav1.ConditionTrue, "DeploymentReady", "All replicas are ready")
	} else {
		desired := int32(0)
		if dep.Spec.Replicas != nil {
			desired = *dep.Spec.Replicas
		}
		SetDeploymentCondition(req, h.conditionType, metav1.ConditionFalse, "DeploymentNotReady",
			fmt.Sprintf("%d/%d replicas ready", dep.Status.ReadyReplicas, desired))
	}

	return result
}

func (h *deploymentReadyConditionHandler) Reset() {
	h.inner.Reset()
}

func (h *deploymentReadyConditionHandler) GetFullCr(hc *hcov1.HyperConverged) (client.Object, error) {
	if getter, ok := h.inner.(CRGetter); ok {
		return getter.GetFullCr(hc)
	}
	return nil, nil
}

// SetDeploymentCondition sets a Deployment readiness condition on the HyperConverged status.
// This is the generalized form of the former setNetResInjCondition helper.
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

// RemoveDeploymentCondition removes a Deployment readiness condition from the HyperConverged status.
// This is the generalized form of the former removeNetResInjCondition helper.
func RemoveDeploymentCondition(req *common.HcoRequest, conditionType string) {
	changed := meta.RemoveStatusCondition(&req.Instance.Status.Conditions, conditionType)
	if changed {
		req.StatusDirty = true
	}
}

type deploymentHooks struct {
	sync.Mutex
	deploymentGenerator newDeploymentFunc
	cache               *appsv1.Deployment
}

func newDeploymentHooks(deploymentGenerator newDeploymentFunc) *deploymentHooks {
	return &deploymentHooks{
		deploymentGenerator: deploymentGenerator,
	}
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
