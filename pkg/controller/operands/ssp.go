package operands

import (
	"fmt"
	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/pkg/apis/hco/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/controller/common"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
	sspv1 "github.com/kubevirt/kubevirt-ssp-operator/pkg/apis/kubevirt/v1"
	objectreferencesv1 "github.com/openshift/custom-resource-status/objectreferences/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/reference"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// ===== CommonTemplateBundle =====

type commonTemplateBundleHandler struct {
	client       client.Client
	scheme       *runtime.Scheme
	eventEmitter hcoutil.EventEmitter
}

func newCommonTemplateBundleHandler(c client.Client, s *runtime.Scheme, ee hcoutil.EventEmitter) *commonTemplateBundleHandler {
	return &commonTemplateBundleHandler{
		client:       c,
		scheme:       s,
		eventEmitter: ee,
	}
}

func (h commonTemplateBundleHandler) ensure(req *common.HcoRequest) *EnsureResult {
	kvCTB := req.Instance.NewKubeVirtCommonTemplateBundle()
	res := NewEnsureResult(kvCTB)

	key, err := client.ObjectKeyFromObject(kvCTB)
	if err != nil {
		req.Logger.Error(err, "Failed to get object key for KubeVirt Common Templates Bundle")
	}

	res.SetName(key.Name)
	found := &sspv1.KubevirtCommonTemplatesBundle{}

	err = h.client.Get(req.Ctx, key, found)
	if err != nil {
		if apierrors.IsNotFound(err) {
			req.Logger.Info("Creating KubeVirt Common Templates Bundle")
			err = h.client.Create(req.Ctx, kvCTB)
			if err == nil {
				h.eventEmitter.EmitCreatedEvent(req.Instance, getResourceType(kvCTB), key.Name)
				return res.SetCreated()
			}
		}
		return res.Error(err)
	}

	existingOwners := found.GetOwnerReferences()

	// Previous versions used to have HCO-operator (namespace: kubevirt-hyperconverged)
	// as the owner of kvCTB (namespace: OpenshiftNamespace).
	// It's not legal, so remove that.
	if len(existingOwners) > 0 {
		req.Logger.Info("kvCTB has owners, removing...")
		found.SetOwnerReferences([]metav1.OwnerReference{})
		err = h.client.Update(req.Ctx, found)
		if err != nil {
			req.Logger.Error(err, "Failed to remove kvCTB's previous owners")
		}
	}

	req.Logger.Info("KubeVirt Common Templates Bundle already exists", "bundle.Namespace", found.Namespace, "bundle.Name", found.Name)

	// Add it to the list of RelatedObjects if found
	objectRef, err := reference.GetReference(h.scheme, found)
	if err != nil {
		return res.Error(err)
	}
	objectreferencesv1.SetObjectReference(&req.Instance.Status.RelatedObjects, *objectRef)

	isReady := handleComponentConditions(req, "KubevirtCommonTemplatesBundle", found.Status.Conditions)

	upgradeInProgress := false
	if isReady {
		upgradeInProgress = req.UpgradeMode && checkComponentVersion(hcoutil.SspVersionEnvV, found.Status.ObservedVersion)
		if (upgradeInProgress || !req.UpgradeMode) && shouldRemoveOldCrd[commonTemplatesBundleOldCrdName] {
			if removeCrd(h.client, req, commonTemplatesBundleOldCrdName) {
				shouldRemoveOldCrd[commonTemplatesBundleOldCrdName] = false
			}
		}
	}

	return res.SetUpgradeDone(req.ComponentUpgradeInProgress && upgradeInProgress)
}

// ===== NodeLabellerBundle =====

type nodeLabellerBundleHandler struct {
	client       client.Client
	scheme       *runtime.Scheme
	eventEmitter hcoutil.EventEmitter
}

func newNodeLabellerBundleHandler(c client.Client, s *runtime.Scheme, ee hcoutil.EventEmitter) *nodeLabellerBundleHandler {
	return &nodeLabellerBundleHandler{
		client:       c,
		scheme:       s,
		eventEmitter: ee,
	}
}

func (h nodeLabellerBundleHandler) ensure(req *common.HcoRequest) *EnsureResult {
	kvNLB := NewKubeVirtNodeLabellerBundleForCR(req.Instance, req.Namespace)
	res := NewEnsureResult(kvNLB)

	if err := controllerutil.SetControllerReference(req.Instance, kvNLB, h.scheme); err != nil {
		return res.Error(err)
	}

	key, err := client.ObjectKeyFromObject(kvNLB)
	if err != nil {
		req.Logger.Error(err, "Failed to get object key for KubeVirt Node Labeller Bundle")
	}

	res.SetName(key.Name)
	found := &sspv1.KubevirtNodeLabellerBundle{}

	err = h.client.Get(req.Ctx, key, found)
	if err != nil {
		if apierrors.IsNotFound(err) {
			req.Logger.Info("Creating KubeVirt Node Labeller Bundle")
			err = h.client.Create(req.Ctx, kvNLB)
			if err == nil {
				h.eventEmitter.EmitCreatedEvent(req.Instance, getResourceType(kvNLB), key.Name)
				return res.SetCreated()
			}
		}
		return res.Error(err)
	}

	req.Logger.Info("KubeVirt Node Labeller Bundle already exists", "bundle.Namespace", found.Namespace, "bundle.Name", found.Name)

	// Add it to the list of RelatedObjects if found
	objectRef, err := reference.GetReference(h.scheme, found)
	if err != nil {
		return res.Error(err)
	}
	objectreferencesv1.SetObjectReference(&req.Instance.Status.RelatedObjects, *objectRef)

	isReady := handleComponentConditions(req, "KubevirtNodeLabellerBundle", found.Status.Conditions)

	upgradeInProgress := false
	if isReady {
		upgradeInProgress = req.UpgradeMode && checkComponentVersion(hcoutil.SspVersionEnvV, found.Status.ObservedVersion)
		if (upgradeInProgress || !req.UpgradeMode) && shouldRemoveOldCrd[nodeLabellerBundlesOldCrdName] {
			if removeCrd(h.client, req, nodeLabellerBundlesOldCrdName) {
				shouldRemoveOldCrd[nodeLabellerBundlesOldCrdName] = false
			}
		}
	}

	return res.SetUpgradeDone(req.ComponentUpgradeInProgress && upgradeInProgress)

}

func NewKubeVirtNodeLabellerBundleForCR(cr *hcov1beta1.HyperConverged, namespace string) *sspv1.KubevirtNodeLabellerBundle {
	labels := map[string]string{
		hcoutil.AppLabel: cr.Name,
	}
	return &sspv1.KubevirtNodeLabellerBundle{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "node-labeller-" + cr.Name,
			Labels:    labels,
			Namespace: namespace,
		},
		Spec: sspv1.ComponentSpec{
			// UseKVM: isKVMAvailable(),
		},
	}
}

// ===== TemplateValidator =====

type templateValidatorHandler struct {
	client       client.Client
	scheme       *runtime.Scheme
	eventEmitter hcoutil.EventEmitter
}

func newTemplateValidatorHandler(c client.Client, s *runtime.Scheme, ee hcoutil.EventEmitter) *templateValidatorHandler {
	return &templateValidatorHandler{
		client:       c,
		scheme:       s,
		eventEmitter: ee,
	}
}

func (h templateValidatorHandler) ensure(req *common.HcoRequest) *EnsureResult {
	kvTV := NewKubeVirtTemplateValidatorForCR(req.Instance, req.Namespace)
	res := NewEnsureResult(kvTV)

	if err := controllerutil.SetControllerReference(req.Instance, kvTV, h.scheme); err != nil {
		return res.Error(err)
	}

	key, err := client.ObjectKeyFromObject(kvTV)
	if err != nil {
		req.Logger.Error(err, "Failed to get object key for KubeVirt Template Validator")
	}
	res.SetName(key.Name)

	found := &sspv1.KubevirtTemplateValidator{}
	err = h.client.Get(req.Ctx, key, found)
	if err != nil {
		if apierrors.IsNotFound(err) {
			req.Logger.Info("Creating KubeVirt Template Validator")
			err = h.client.Create(req.Ctx, kvTV)
			if err == nil {
				h.eventEmitter.EmitCreatedEvent(req.Instance, getResourceType(kvTV), key.Name)
				return res.SetCreated()
			}
		}
		return res.Error(err)
	}

	req.Logger.Info("KubeVirt Template Validator already exists", "validator.Namespace", found.Namespace, "validator.Name", found.Name)

	// Add it to the list of RelatedObjects if found
	objectRef, err := reference.GetReference(h.scheme, found)
	if err != nil {
		return res.Error(err)
	}
	objectreferencesv1.SetObjectReference(&req.Instance.Status.RelatedObjects, *objectRef)

	isReady := handleComponentConditions(req, "KubevirtTemplateValidator", found.Status.Conditions)

	upgradeInProgress := false
	if isReady {
		upgradeInProgress = req.UpgradeMode && checkComponentVersion(hcoutil.SspVersionEnvV, found.Status.ObservedVersion)
		if (upgradeInProgress || !req.UpgradeMode) && shouldRemoveOldCrd[templateValidatorsOldCrdName] {
			if removeCrd(h.client, req, templateValidatorsOldCrdName) {
				shouldRemoveOldCrd[templateValidatorsOldCrdName] = false
			}
		}
	}

	return res.SetUpgradeDone(req.ComponentUpgradeInProgress && upgradeInProgress)
}

func NewKubeVirtTemplateValidatorForCR(cr *hcov1beta1.HyperConverged, namespace string) *sspv1.KubevirtTemplateValidator {
	labels := map[string]string{
		hcoutil.AppLabel: cr.Name,
	}
	return &sspv1.KubevirtTemplateValidator{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "template-validator-" + cr.Name,
			Labels:    labels,
			Namespace: namespace,
		},
	}
}

// ===== MetricsAggregation =====

type metricsAggregationHandler struct {
	client       client.Client
	scheme       *runtime.Scheme
	eventEmitter hcoutil.EventEmitter
}

func newMetricsAggregationHandler(c client.Client, s *runtime.Scheme, ee hcoutil.EventEmitter) *metricsAggregationHandler {
	return &metricsAggregationHandler{
		client:       c,
		scheme:       s,
		eventEmitter: ee,
	}
}

func (h metricsAggregationHandler) ensure(req *common.HcoRequest) *EnsureResult {
	kubevirtMetricsAggregation := NewKubeVirtMetricsAggregationForCR(req.Instance, req.Namespace)
	res := NewEnsureResult(kubevirtMetricsAggregation)

	err := controllerutil.SetControllerReference(req.Instance, kubevirtMetricsAggregation, h.scheme)
	if err != nil {
		return res.Error(err)
	}

	key, err := client.ObjectKeyFromObject(kubevirtMetricsAggregation)
	if err != nil {
		req.Logger.Error(err, "Failed to get object key for KubeVirt Metrics Aggregation")
	}

	res.SetName(key.Name)
	found := &sspv1.KubevirtMetricsAggregation{}

	err = h.client.Get(req.Ctx, key, found)
	if err != nil {
		if apierrors.IsNotFound(err) {
			req.Logger.Info("Creating KubeVirt Metrics Aggregation")
			err = h.client.Create(req.Ctx, kubevirtMetricsAggregation)
			if err == nil {
				h.eventEmitter.EmitCreatedEvent(req.Instance, getResourceType(kubevirtMetricsAggregation), key.Name)
				return res.SetCreated()
			}
		}
		return res.Error(err)
	}

	req.Logger.Info("KubeVirt Metrics Aggregation already exists", "metrics.Namespace", found.Namespace, "metrics.Name", found.Name)

	// Add it to the list of RelatedObjects if found
	objectRef, err := reference.GetReference(h.scheme, found)
	if err != nil {
		return res.Error(err)
	}
	objectreferencesv1.SetObjectReference(&req.Instance.Status.RelatedObjects, *objectRef)

	isReady := handleComponentConditions(req, "KubeVirtMetricsAggregation", found.Status.Conditions)

	upgradeInProgress := false
	if isReady {
		upgradeInProgress = req.UpgradeMode && checkComponentVersion(hcoutil.SspVersionEnvV, found.Status.ObservedVersion)
		if (upgradeInProgress || !req.UpgradeMode) && shouldRemoveOldCrd[metricsAggregationOldCrdName] {
			if removeCrd(h.client, req, metricsAggregationOldCrdName) {
				shouldRemoveOldCrd[metricsAggregationOldCrdName] = false
			}
		}
	}

	return res.SetUpgradeDone(req.ComponentUpgradeInProgress && upgradeInProgress)
}

func NewKubeVirtMetricsAggregationForCR(cr *hcov1beta1.HyperConverged, namespace string) *sspv1.KubevirtMetricsAggregation {
	labels := map[string]string{
		hcoutil.AppLabel: cr.Name,
	}
	return &sspv1.KubevirtMetricsAggregation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "metrics-aggregation-" + cr.Name,
			Labels:    labels,
			Namespace: namespace,
		},
	}
}

// ========= Old CRDs =========

const (
	commonTemplatesBundleOldCrdName = "kubevirtcommontemplatesbundles.kubevirt.io"
	metricsAggregationOldCrdName    = "kubevirtmetricsaggregations.kubevirt.io"
	nodeLabellerBundlesOldCrdName   = "kubevirtnodelabellerbundles.kubevirt.io"
	templateValidatorsOldCrdName    = "kubevirttemplatevalidators.kubevirt.io"
)

var (
	shouldRemoveOldCrd = map[string]bool{
		commonTemplatesBundleOldCrdName: true,
		metricsAggregationOldCrdName:    true,
		nodeLabellerBundlesOldCrdName:   true,
		templateValidatorsOldCrdName:    true,
	}
)

// return true if not found or if deletion succeeded
func removeCrd(c client.Client, req *common.HcoRequest, crdName string) bool {
	found := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"kind":       "CustomResourceDefinition",
			"apiVersion": "apiextensions.k8s.io/v1",
		},
	}
	key := client.ObjectKey{Namespace: req.Namespace, Name: crdName}
	err := c.Get(req.Ctx, key, found)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			req.Logger.Error(err, fmt.Sprintf("failed to read the %s CRD; %s", crdName, err.Error()))
			return false
		}
	} else {
		err = c.Delete(req.Ctx, found)
		if err != nil {
			req.Logger.Error(err, fmt.Sprintf("failed to remove the %s CRD; %s", crdName, err.Error()))
			return false
		} else {
			req.Logger.Info("successfully removed CRD", "CRD Name", crdName)
		}
	}

	return true
}
