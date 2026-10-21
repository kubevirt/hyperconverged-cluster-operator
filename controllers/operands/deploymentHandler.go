package operands

import (
	"errors"
	"maps"
	"reflect"
	"sync"

	appsv1 "k8s.io/api/apps/v1"
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
		// Preserve pod-template annotations added by other controllers while applying
		// the operator-managed ones (computed before the spec is overwritten below).
		mergedTemplateAnnotations := mergeStringMaps(found.Spec.Template.Annotations, deployment.Spec.Template.Annotations)
		util.MergeLabels(&deployment.ObjectMeta, &found.ObjectMeta)
		deployment.Spec.DeepCopyInto(&found.Spec)
		found.Spec.Template.Annotations = mergedTemplateAnnotations
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
		reflect.DeepEqual(found.Spec.Template.Spec.Tolerations, required.Spec.Template.Spec.Tolerations) &&
		compareTemplateAnnotations(found, required)
}

// compareTemplateAnnotations reports whether every pod-template annotation the
// operator manages (present in required) exists with the same value in found. Annotations
// added by other controllers (present only in found) are ignored, so they neither trigger
// a spurious rollout nor get removed on reconcile.
func compareTemplateAnnotations(found, required *appsv1.Deployment) bool {
	for key, value := range required.Spec.Template.Annotations {
		if foundValue, ok := found.Spec.Template.Annotations[key]; !ok || foundValue != value {
			return false
		}
	}
	return true
}

// mergeStringMaps returns a new map with existing entries overlaid by managed entries
// (managed wins on key conflicts). It returns nil when both inputs are empty.
func mergeStringMaps(existing, managed map[string]string) map[string]string {
	if len(existing) == 0 && len(managed) == 0 {
		return nil
	}
	merged := make(map[string]string, len(existing)+len(managed))
	maps.Copy(merged, existing)
	maps.Copy(merged, managed)
	return merged
}

func shouldRecreate(found, required *appsv1.Deployment) bool {
	// updating LabelSelector (it's immutable) would be rejected by API server; create new Deployment instead
	return !reflect.DeepEqual(found.Spec.Selector, required.Spec.Selector)
}
