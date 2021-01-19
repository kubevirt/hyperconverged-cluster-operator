package operands

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	jsonpatch "github.com/evanphx/json-patch"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/pkg/apis/hco/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/controller/common"
	conditionsv1 "github.com/openshift/custom-resource-status/conditions/v1"
	objectreferencesv1 "github.com/openshift/custom-resource-status/objectreferences/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/reference"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Operand interface {
	ensure(req *common.HcoRequest) *EnsureResult
	reset()
}

// Handles a specific resource (a CR, a configMap and so on), to be run during reconciliation
type genericOperand struct {
	// K8s client
	Client client.Client
	Scheme *runtime.Scheme
	// printable resource name
	crType string
	// In some cases, Previous versions used to have HCO-operator (scope namespace)
	// as the owner of some resources (scope cluster).
	// It's not legal, so remove that.
	removeExistingOwner bool
	// Should the handler add the controller reference
	setControllerReference bool
	// Is it a custom resource
	isCr bool
	// Set of resource handler hooks, to be implement in each handler
	hooks hcoResourceHooks
}

// Set of resource handler hooks, to be implement in each handler
type hcoResourceHooks interface {
	// Generate the required resource, with all the required fields)
	getFullCr(*hcov1beta1.HyperConverged) (client.Object, error)
	// Generate an empty resource, to be used as the input of the client.Get method. After calling this method, it will
	// contains the actual values in K8s.
	getEmptyCr() client.Object
	// optional validation before starting the ensure work
	validate() error
	// an optional hook that is called just after getting the resource from K8s
	postFound(*common.HcoRequest, runtime.Object) error
	// check if there is a change between the required resource and the resource read from K8s, and update K8s accordingly.
	updateCr(*common.HcoRequest, client.Client, runtime.Object, runtime.Object) (bool, bool, error)
	// get the CR conditions, if exists
	getConditions(runtime.Object) []conditionsv1.Condition
	// on upgrade mode, check if the CR is already with the expected version
	checkComponentVersion(runtime.Object) bool
	// cast he specific resource to *metav1.ObjectMeta
	getObjectMeta(runtime.Object) *metav1.ObjectMeta
	// reset handler cached, if exists
	reset()
}

func (h *genericOperand) ensure(req *common.HcoRequest) *EnsureResult {
	cr, err := h.hooks.getFullCr(req.Instance)
	if err != nil {
		return &EnsureResult{
			Err: err,
		}
	}

	res := NewEnsureResult(cr)
	if err = h.hooks.validate(); err != nil {
		return res.Error(err)
	}

	ref, ok := cr.(metav1.Object)
	if h.setControllerReference {
		if !ok {
			return res.Error(fmt.Errorf("can't convert %T to k8s.io/apimachinery/pkg/apis/meta/v1.Object", cr))
		}
		if err := controllerutil.SetControllerReference(req.Instance, ref, h.Scheme); err != nil {
			return res.Error(err)
		}
	}

	key := client.ObjectKeyFromObject(cr)
	res.SetName(key.Name)
	found := h.hooks.getEmptyCr()
	err = h.Client.Get(req.Ctx, key, found)
	if err != nil {
		if apierrors.IsNotFound(err) {
			req.Logger.Info("Creating " + h.crType)
			err = h.Client.Create(req.Ctx, cr)
			if err != nil {
				req.Logger.Error(err, "Failed to create object for "+h.crType)
				return res.Error(err)
			}
			return res.SetCreated().SetName(key.Name)
		}
		return res.Error(err)
	}

	req.Logger.Info(h.crType+" already exists", h.crType+".Namespace", key.Namespace, h.crType+".Name", key.Name)

	if err = h.hooks.postFound(req, found); err != nil {
		return res.Error(err)
	}

	if h.removeExistingOwner {
		existingOwners := h.hooks.getObjectMeta(found).GetOwnerReferences()

		if len(existingOwners) > 0 {
			req.Logger.Info(h.crType + " has owners, removing...")
			err = h.Client.Update(req.Ctx, found)
			if err != nil {
				req.Logger.Error(err, fmt.Sprintf("Failed to remove %s's previous owners", h.crType))
			}
		}

		if err == nil {
			// do that only once
			h.removeExistingOwner = false
		}
	}

	updated, overwritten, err := h.hooks.updateCr(req, h.Client, found, cr)
	if err != nil {
		return res.Error(err)
	}
	if updated {
		res.SetUpdated()
		if overwritten {
			res.SetOverwritten()
		}
		return res
	}

	// Add it to the list of RelatedObjects if found
	objectRef, err := reference.GetReference(h.Scheme, found)
	if err != nil {
		return res.Error(err)
	}
	objectreferencesv1.SetObjectReference(&req.Instance.Status.RelatedObjects, *objectRef)

	if h.isCr {
		// Handle KubeVirt resource conditions
		isReady := handleComponentConditions(req, h.crType, h.hooks.getConditions(found))

		upgradeDone := req.UpgradeMode && isReady && h.hooks.checkComponentVersion(found)
		return res.SetUpgradeDone(upgradeDone)
	}
	// For resources that are not CRs, such as priority classes or a config map, there is no new version to upgrade
	return res.SetUpgradeDone(req.ComponentUpgradeInProgress)
}

func (h *genericOperand) reset() {
	h.hooks.reset()
}

// handleComponentConditions - read and process a sub-component conditions.
// returns true if the the conditions indicates "ready" state and false if not.
func handleComponentConditions(req *common.HcoRequest, component string, componentConds []conditionsv1.Condition) (isReady bool) {
	isReady = true
	if len(componentConds) == 0 {
		isReady = false
		reason := fmt.Sprintf("%sConditions", component)
		message := fmt.Sprintf("%s resource has no conditions", component)
		req.Logger.Info(fmt.Sprintf("%s's resource is not reporting Conditions on it's Status", component))
		req.Conditions.SetStatusCondition(conditionsv1.Condition{
			Type:    conditionsv1.ConditionAvailable,
			Status:  corev1.ConditionFalse,
			Reason:  reason,
			Message: message,
		})
		req.Conditions.SetStatusCondition(conditionsv1.Condition{
			Type:    conditionsv1.ConditionProgressing,
			Status:  corev1.ConditionTrue,
			Reason:  reason,
			Message: message,
		})
		req.Conditions.SetStatusCondition(conditionsv1.Condition{
			Type:    conditionsv1.ConditionUpgradeable,
			Status:  corev1.ConditionFalse,
			Reason:  reason,
			Message: message,
		})
	} else {
		foundAvailableCond := false
		foundProgressingCond := false
		foundDegradedCond := false
		for _, condition := range componentConds {
			switch condition.Type {
			case conditionsv1.ConditionAvailable:
				foundAvailableCond = true
				if condition.Status == corev1.ConditionFalse {
					isReady = false
					msg := fmt.Sprintf("%s is not available: %v", component, string(condition.Message))
					componentNotAvailable(req, component, msg)
				}
			case conditionsv1.ConditionProgressing:
				foundProgressingCond = true
				if condition.Status == corev1.ConditionTrue {
					isReady = false
					req.Logger.Info(fmt.Sprintf("%s is 'Progressing'", component))
					req.Conditions.SetStatusCondition(conditionsv1.Condition{
						Type:    conditionsv1.ConditionProgressing,
						Status:  corev1.ConditionTrue,
						Reason:  fmt.Sprintf("%sProgressing", component),
						Message: fmt.Sprintf("%s is progressing: %v", component, string(condition.Message)),
					})
					req.Conditions.SetStatusCondition(conditionsv1.Condition{
						Type:    conditionsv1.ConditionUpgradeable,
						Status:  corev1.ConditionFalse,
						Reason:  fmt.Sprintf("%sProgressing", component),
						Message: fmt.Sprintf("%s is progressing: %v", component, string(condition.Message)),
					})
				}
			case conditionsv1.ConditionDegraded:
				foundDegradedCond = true
				if condition.Status == corev1.ConditionTrue {
					isReady = false
					req.Logger.Info(fmt.Sprintf("%s is 'Degraded'", component))
					req.Conditions.SetStatusCondition(conditionsv1.Condition{
						Type:    conditionsv1.ConditionDegraded,
						Status:  corev1.ConditionTrue,
						Reason:  fmt.Sprintf("%sDegraded", component),
						Message: fmt.Sprintf("%s is degraded: %v", component, string(condition.Message)),
					})
				}
			}
		}

		if !foundAvailableCond {
			componentNotAvailable(req, component, `missing "Available" condition`)
		}

		isReady = isReady && foundAvailableCond && foundProgressingCond && foundDegradedCond
	}

	return isReady
}

func componentNotAvailable(req *common.HcoRequest, component string, msg string) {
	req.Logger.Info(fmt.Sprintf("%s is not 'Available'", component))
	req.Conditions.SetStatusCondition(conditionsv1.Condition{
		Type:    conditionsv1.ConditionAvailable,
		Status:  corev1.ConditionFalse,
		Reason:  fmt.Sprintf("%sNotAvailable", component),
		Message: msg,
	})
}

func checkComponentVersion(versionEnvName, actualVersion string) bool {
	expectedVersion := os.Getenv(versionEnvName)
	return expectedVersion != "" && expectedVersion == actualVersion
}

func getNamespace(defaultNamespace string, opts []string) string {
	if len(opts) > 0 {
		return opts[0]
	}
	return defaultNamespace
}

func getLabels(hc *hcov1beta1.HyperConverged, component hcoutil.AppComponent) map[string]string {
	hcoName := hcov1beta1.HyperConvergedName

	if hc.Name != "" {
		hcoName = hc.Name
	}

	return map[string]string{
		hcoutil.AppLabel:          hcoName,
		hcoutil.AppLabelManagedBy: hcoutil.OperatorName,
		hcoutil.AppLabelVersion:   hcoutil.GetHcoKvIoVersion(),
		hcoutil.AppLabelPartOf:    hcoutil.HyperConvergedCluster,
		hcoutil.AppLabelComponent: string(component),
	}
}

func applyAnnotationPatch(obj runtime.Object, annotation string) error {
	patches, err := jsonpatch.DecodePatch([]byte(annotation))
	if err != nil {
		return err
	}

	for _, patch := range patches {
		path, err := patch.Path()
		if err != nil {
			return err
		}

		if !strings.HasPrefix(path, "/spec/") {
			return errors.New("can only modify spec fields")
		}
	}

	specBytes, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	patchedBytes, err := patches.Apply(specBytes)
	if err != nil {
		return err
	}
	return json.Unmarshal(patchedBytes, obj)
}

func applyPatchToSpec(hc *hcov1beta1.HyperConverged, annotationName string, obj runtime.Object) error {
	if jsonpathAnnotation, ok := hc.Annotations[annotationName]; ok {
		if err := applyAnnotationPatch(obj, jsonpathAnnotation); err != nil {
			return fmt.Errorf("invalid jsonPatch in the %s annotation: %v", annotationName, err)
		}
	}

	return nil
}
