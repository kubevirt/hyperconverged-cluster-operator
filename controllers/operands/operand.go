package operands

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	jsonpatch "github.com/evanphx/json-patch/v5"
	log "github.com/go-logr/logr"
	conditionsv1 "github.com/openshift/custom-resource-status/conditions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
)

type Operand interface {
	Ensure(req *common.HcoRequest) *EnsureResult
	Reseter
}

type CRGetter interface {
	// GetFullCr Generate the required resource, with all the required fields)
	GetFullCr(*hcov1beta1.HyperConverged) (client.Object, error)
}

// HCOResourceHooks Set of resource handler hooks, to be implement in each handler
type HCOResourceHooks interface {
	CRGetter

	// GetEmptyCr Generate an empty resource, to be used as the input of the client.Get method. After calling this method, it will
	// contain the actual values in K8s.
	GetEmptyCr() client.Object
	// UpdateCR check if there is a change between the required resource and the resource read from K8s, and update K8s accordingly.
	UpdateCR(*common.HcoRequest, client.Client, runtime.Object, runtime.Object) (bool, bool, error)
	// JustBeforeComplete last hook before completing the operand handling
	JustBeforeComplete(req *common.HcoRequest)
}

// HCOOperandHooks Set of operand handler hooks, to be implement in each handler
type HCOOperandHooks interface {
	HCOResourceHooks
	// GetConditions get the CR conditions, if exists
	GetConditions(runtime.Object) []metav1.Condition
	// CheckComponentVersion on upgrade mode, check if the CR is already with the expected version
	CheckComponentVersion(runtime.Object) bool
}

type Reseter interface {
	// Reset handler cached, if exists
	Reset()
}

type GetHandler func(log.Logger, client.Client, *runtime.Scheme, *hcov1beta1.HyperConverged) (Operand, error)
type GetHandlers func(log.Logger, client.Client, *runtime.Scheme, *hcov1beta1.HyperConverged, fs.FS) ([]Operand, error)

func handleOperandDegradedCond(req *common.HcoRequest, component string, condition metav1.Condition) bool {
	if condition.Status == metav1.ConditionTrue {
		req.Logger.Info(fmt.Sprintf("%s is 'Degraded'", component))
		req.Conditions.SetStatusCondition(metav1.Condition{
			Type:               hcov1beta1.ConditionDegraded,
			Status:             metav1.ConditionTrue,
			Reason:             fmt.Sprintf("%sDegraded", component),
			Message:            fmt.Sprintf("%s is degraded: %v", component, condition.Message),
			ObservedGeneration: req.Instance.Generation,
		})

		return false
	}
	return true
}

func handleOperandProgressingCond(req *common.HcoRequest, component string, condition metav1.Condition) bool {
	if condition.Status == metav1.ConditionTrue {
		req.Logger.Info(fmt.Sprintf("%s is 'Progressing'", component))
		req.Conditions.SetStatusCondition(metav1.Condition{
			Type:               hcov1beta1.ConditionProgressing,
			Status:             metav1.ConditionTrue,
			Reason:             fmt.Sprintf("%sProgressing", component),
			Message:            fmt.Sprintf("%s is progressing: %v", component, condition.Message),
			ObservedGeneration: req.Instance.Generation,
		})
		req.Conditions.SetStatusConditionIfUnset(metav1.Condition{
			Type:               hcov1beta1.ConditionUpgradeable,
			Status:             metav1.ConditionFalse,
			Reason:             fmt.Sprintf("%sProgressing", component),
			Message:            fmt.Sprintf("%s is progressing: %v", component, condition.Message),
			ObservedGeneration: req.Instance.Generation,
		})

		return false
	}
	return true
}

func handleOperandUpgradeableCond(req *common.HcoRequest, component string, condition metav1.Condition) {
	if condition.Status == metav1.ConditionFalse {
		req.Upgradeable = false
		req.Logger.Info(fmt.Sprintf("%s is 'Progressing'", component))
		req.Conditions.SetStatusCondition(metav1.Condition{
			Type:               hcov1beta1.ConditionUpgradeable,
			Status:             metav1.ConditionFalse,
			Reason:             fmt.Sprintf("%sNotUpgradeable", component),
			Message:            fmt.Sprintf("%s is not upgradeable: %v", component, condition.Message),
			ObservedGeneration: req.Instance.Generation,
		})
	}
}

func CheckComponentVersion(versionEnvName, actualVersion string) bool {
	expectedVersion := os.Getenv(versionEnvName)
	return expectedVersion != "" && expectedVersion == actualVersion
}

func GetNamespace(defaultNamespace string, opts []string) string {
	if len(opts) > 0 {
		return opts[0]
	}
	return defaultNamespace
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

func ApplyPatchToSpec(hc *hcov1beta1.HyperConverged, annotationName string, obj runtime.Object) error {
	if jsonpathAnnotation, ok := hc.Annotations[annotationName]; ok {
		if err := applyAnnotationPatch(obj, jsonpathAnnotation); err != nil {
			return fmt.Errorf("invalid jsonPatch in the %s annotation: %v", annotationName, err)
		}
	}

	return nil
}

func OSConditionsToK8s(conditions []conditionsv1.Condition) []metav1.Condition {
	if len(conditions) == 0 {
		return nil
	}

	newCond := make([]metav1.Condition, len(conditions))
	for i, c := range conditions {
		newCond[i] = osConditionToK8s(c)
	}

	return newCond
}

func osConditionToK8s(condition conditionsv1.Condition) metav1.Condition {
	return metav1.Condition{
		Type:    string(condition.Type),
		Reason:  condition.Reason,
		Status:  metav1.ConditionStatus(condition.Status),
		Message: condition.Message,
	}
}
