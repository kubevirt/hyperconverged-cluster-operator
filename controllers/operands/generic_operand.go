package operands

import (
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

// GenericOperand Handles a specific resource (a CR, a configMap and so on), to be run during reconciliation
type GenericOperand struct {
	// K8s client
	client.Client

	Scheme *runtime.Scheme

	// printable resource name
	crType string
	// Should the handler add the controller reference
	setControllerReference bool
	// Set of resource handler hooks, to be implemented in each handler
	hooks HCOResourceHooks
}

func NewGenericOperand(client client.Client, scheme *runtime.Scheme, crType string, hooks HCOResourceHooks, setControllerReference bool) *GenericOperand {
	return &GenericOperand{
		Client:                 client,
		Scheme:                 scheme,
		crType:                 crType,
		setControllerReference: setControllerReference,
		hooks:                  hooks,
	}
}

func (h *GenericOperand) Ensure(req *common.HcoRequest) *EnsureResult {
	cr, err := h.hooks.GetFullCr(req.Instance)
	if err != nil {
		return &EnsureResult{
			Err: err,
		}
	}

	res := NewEnsureResult(cr)

	if err := h.doSetControllerReference(req, cr); err != nil {
		return res.Error(err)
	}

	key := client.ObjectKeyFromObject(cr)
	res.SetName(key.Name)
	found := h.hooks.GetEmptyCr()
	err = h.Get(req.Ctx, key, found)
	if err != nil {
		if apierrors.IsNotFound(err) {
			res = h.createNewCr(req, cr, res)
			if apierrors.IsAlreadyExists(res.Err) {
				// we failed trying to create it due to a caching error
				// or we neither tried because we know that the object is already there for sure,
				// but we cannot get it due to a bad cache hit.
				// Let's try updating it bypassing the client cache mechanism
				return h.handleExistingCrSkipCache(req, key, found, cr, res)
			}
		} else {
			return res.Error(err)
		}
	} else {
		res = h.handleExistingCr(req, key, found, cr, res)
	}

	if res.Err == nil {
		h.hooks.JustBeforeComplete(req)
	}

	return res
}

func (h *GenericOperand) handleExistingCr(req *common.HcoRequest, key client.ObjectKey, found client.Object, cr client.Object, res *EnsureResult) *EnsureResult {
	req.Logger.Info(h.crType+" already exists", h.crType+".Namespace", key.Namespace, h.crType+".Name", key.Name)

	updated, overwritten, err := h.hooks.UpdateCR(req, h.Client, found, cr)
	if err != nil {
		return res.Error(err)
	}
	if updated {
		// refresh the object
		err = h.Get(req.Ctx, key, found)
		if err != nil {
			return res.Error(err)
		}
	}

	// update resourceVersions of objects in relatedObjects
	if err = h.addCrToTheRelatedObjectList(req, found); err != nil {
		return res.Error(err)
	}

	if updated {
		req.StatusDirty = true
		return res.SetUpdated().SetOverwritten(overwritten)
	}

	if opr, ok := h.hooks.(HCOOperandHooks); ok { // for operands, perform some more checks
		return h.completeEnsureOperands(req, opr, found, res)
	}
	// For resources that are not CRs, such as priority classes or a config map, there is no new version to upgrade
	return res.SetUpgradeDone(req.ComponentUpgradeInProgress)
}

func (h *GenericOperand) handleExistingCrSkipCache(req *common.HcoRequest, key client.ObjectKey, found client.Object, cr client.Object, res *EnsureResult) *EnsureResult {
	cfg, configerr := config.GetConfig()
	if configerr != nil {
		req.Logger.Error(configerr, "failed creating a config for a custom client")
		return &EnsureResult{
			Err: configerr,
		}
	}
	apiClient, acerr := client.New(cfg, client.Options{
		Scheme: h.Scheme,
	})
	if acerr != nil {
		req.Logger.Error(acerr, "failed creating a custom client to bypass the cache")
		return &EnsureResult{
			Err: acerr,
		}
	}
	geterr := apiClient.Get(req.Ctx, key, found)
	if geterr != nil {
		req.Logger.Error(geterr, "failed trying to get the object bypassing the cache")
		return &EnsureResult{
			Err: geterr,
		}
	}
	originalClient := h.Client
	// this is not exactly thread safe,
	// but we are not supposed to call twice in parallel
	// the handler for a single CR
	h.Client = apiClient
	existingcrresult := h.handleExistingCr(req, key, found, cr, res)
	h.Client = originalClient
	return existingcrresult
}

func (h *GenericOperand) completeEnsureOperands(req *common.HcoRequest, opr HCOOperandHooks, found client.Object, res *EnsureResult) *EnsureResult {
	// Handle KubeVirt resource conditions
	isReady := handleComponentConditions(req, h.crType, opr.GetConditions(found))

	versionUpdated := opr.CheckComponentVersion(found)
	if isReady && !versionUpdated {
		req.Logger.Info(fmt.Sprintf("could not complete the upgrade process. %s is not with the expected version. Check %s observed version in the status field of its CR", h.crType, h.crType))
	}

	upgradeDone := req.UpgradeMode && isReady && versionUpdated
	return res.SetUpgradeDone(upgradeDone)
}

func (h *GenericOperand) addCrToTheRelatedObjectList(req *common.HcoRequest, found client.Object) error {

	changed, err := hcoutil.AddCrToTheRelatedObjectList(&req.Instance.Status.RelatedObjects, found, h.Scheme)
	if err != nil {
		return err
	}

	if changed {
		req.StatusDirty = true
	}
	return nil
}

func (h *GenericOperand) doSetControllerReference(req *common.HcoRequest, cr client.Object) error {
	if h.setControllerReference {
		ref, ok := cr.(metav1.Object)
		if !ok {
			return fmt.Errorf("can't convert %T to k8s.io/apimachinery/pkg/apis/meta/v1.Object", cr)
		}
		if err := controllerutil.SetControllerReference(req.Instance, ref, h.Scheme); err != nil {
			return err
		}
	}
	return nil
}

func (h *GenericOperand) createNewCr(req *common.HcoRequest, cr client.Object, res *EnsureResult) *EnsureResult {
	req.Logger.Info("Creating " + h.crType)
	if cr.GetResourceVersion() != "" {
		cr.SetResourceVersion("")
	}
	err := h.Create(req.Ctx, cr)
	if err != nil {
		req.Logger.Error(err, "Failed to create object for "+h.crType)
		return res.Error(err)
	}
	return res.SetCreated()
}

func (h *GenericOperand) Reset() {
	if r, ok := h.hooks.(Reseter); ok {
		r.Reset()
	}
}

func (h *GenericOperand) GetFullCr(hc *hcov1beta1.HyperConverged) (client.Object, error) {
	return h.hooks.GetFullCr(hc)
}

// handleComponentConditions - read and process a sub-component conditions.
// returns true if the the conditions indicates "ready" state and false if not.
func handleComponentConditions(req *common.HcoRequest, component string, componentConds []metav1.Condition) bool {
	if len(componentConds) == 0 {
		getConditionsForNewCr(req, component)
		return false
	}

	return setConditionsByOperandConditions(req, component, componentConds)
}

func setConditionsByOperandConditions(req *common.HcoRequest, component string, componentConds []metav1.Condition) bool {
	isReady := true
	foundAvailableCond := false
	foundProgressingCond := false
	foundDegradedCond := false
	for _, condition := range componentConds {
		switch condition.Type {
		case hcov1beta1.ConditionAvailable:
			foundAvailableCond = true
			isReady = handleOperandAvailableCond(req, component, condition) && isReady

		case hcov1beta1.ConditionProgressing:
			foundProgressingCond = true
			isReady = handleOperandProgressingCond(req, component, condition) && isReady

		case hcov1beta1.ConditionDegraded:
			foundDegradedCond = true
			isReady = handleOperandDegradedCond(req, component, condition) && isReady

		case hcov1beta1.ConditionUpgradeable:
			handleOperandUpgradeableCond(req, component, condition)
		}
	}

	if !foundAvailableCond {
		componentNotAvailable(req, component, `missing "Available" condition`)
	}

	return isReady && foundAvailableCond && foundProgressingCond && foundDegradedCond
}

func getConditionsForNewCr(req *common.HcoRequest, component string) {
	reason := fmt.Sprintf("%sConditions", component)
	message := fmt.Sprintf("%s resource has no conditions", component)
	req.Logger.Info(fmt.Sprintf("%s's resource is not reporting Conditions on it's Status", component))
	req.Conditions.SetStatusCondition(metav1.Condition{
		Type:               hcov1beta1.ConditionAvailable,
		Status:             metav1.ConditionFalse,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: req.Instance.Generation,
	})
	req.Conditions.SetStatusCondition(metav1.Condition{
		Type:               hcov1beta1.ConditionProgressing,
		Status:             metav1.ConditionTrue,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: req.Instance.Generation,
	})
	req.Conditions.SetStatusCondition(metav1.Condition{
		Type:               hcov1beta1.ConditionUpgradeable,
		Status:             metav1.ConditionFalse,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: req.Instance.Generation,
	})
}

func handleOperandAvailableCond(req *common.HcoRequest, component string, condition metav1.Condition) bool {
	if condition.Status == metav1.ConditionFalse {
		msg := fmt.Sprintf("%s is not available: %v", component, condition.Message)
		componentNotAvailable(req, component, msg)
		return false
	}
	return true
}

func componentNotAvailable(req *common.HcoRequest, component string, msg string) {
	req.Logger.Info(fmt.Sprintf("%s is not 'Available'", component))
	req.Conditions.SetStatusCondition(metav1.Condition{
		Type:               hcov1beta1.ConditionAvailable,
		Status:             metav1.ConditionFalse,
		Reason:             fmt.Sprintf("%sNotAvailable", component),
		Message:            msg,
		ObservedGeneration: req.Instance.Generation,
	})
}
