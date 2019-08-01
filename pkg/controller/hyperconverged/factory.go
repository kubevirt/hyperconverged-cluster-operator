package hyperconverged

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	//"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/reference"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	sspv1 "github.com/MarSik/kubevirt-ssp-operator/pkg/apis/kubevirt/v1"
	networkaddonsv1alpha1 "github.com/kubevirt/cluster-network-addons-operator/pkg/apis/networkaddonsoperator/v1alpha1"
	networkaddonsnames "github.com/kubevirt/cluster-network-addons-operator/pkg/names"
	hcov1alpha1 "github.com/kubevirt/hyperconverged-cluster-operator/pkg/apis/hco/v1alpha1"
	objectreferencesv1 "github.com/openshift/custom-resource-status/objectreferences/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubevirtv1 "kubevirt.io/client-go/api/v1"
	cdiv1alpha1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
)

const UndefinedNamespace string = ""
const OpenshiftNamespace string = "openshift"

// The set of resources managed by the HCO
func (r *ReconcileHyperConverged) getAllResources(cr *hcov1alpha1.HyperConverged, request reconcile.Request) []runtime.Object {
	return []runtime.Object{
		newKubeVirtConfigForCR(cr, request.Namespace),
		newKubeVirtForCR(cr, request.Namespace),
		newCDIForCR(cr, UndefinedNamespace),
		newNetworkAddonsForCR(cr, UndefinedNamespace),
		newKubeVirtCommonTemplateBundleForCR(cr, OpenshiftNamespace),
		newKubeVirtNodeLabellerBundleForCR(cr, request.Namespace),
		newKubeVirtTemplateValidatorForCR(cr, request.Namespace),
		newKubeVirtMetricsAggregationForCR(cr, request.Namespace),
	}
}

func newKubeVirtConfigForCR(cr *hcov1alpha1.HyperConverged, namespace string) *corev1.ConfigMap {
	labels := map[string]string{
		"app": cr.Name,
	}
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubevirt-config",
			Labels:    labels,
			Namespace: namespace,
		},
		Data: map[string]string{
			"feature-gates": "DataVolumes,SRIOV,LiveMigration,CPUManager,CPUNodeDiscovery",
		},
	}
}

func (r *ReconcileHyperConverged) ensureKubeVirtConfig(instance *hcov1alpha1.HyperConverged, logger logr.Logger, request reconcile.Request) error {
	kubevirtConfig := newKubeVirtConfigForCR(instance, request.Namespace)
	if err := controllerutil.SetControllerReference(instance, kubevirtConfig, r.scheme); err != nil {
		return err
	}

	key, err := client.ObjectKeyFromObject(kubevirtConfig)
	if err != nil {
		logger.Error(err, "Failed to get object key for kubevirt config")
	}

	found := &corev1.ConfigMap{}
	err = r.client.Get(context.TODO(), key, found)
	if err != nil && errors.IsNotFound(err) {
		logger.Info("Creating kubevirt config")
		return r.client.Create(context.TODO(), kubevirtConfig)
	}

	if err != nil {
		return err
	}

	logger.Info("KubeVirt config already exists", "KubeVirtConfig.Namespace", found.Namespace, "KubeVirtConfig.Name", found.Name)
	// Add it to the list of RelatedObjects if found
	objectRef, err := reference.GetReference(r.scheme, found)
	if err != nil {
		return err
	}
	objectreferencesv1.SetObjectReference(&instance.Status.RelatedObjects, *objectRef)

	return nil
}

// newKubeVirtForCR returns a KubeVirt CR
func newKubeVirtForCR(cr *hcov1alpha1.HyperConverged, namespace string) *kubevirtv1.KubeVirt {
	labels := map[string]string{
		"app": cr.Name,
	}
	return &kubevirtv1.KubeVirt{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubevirt-" + cr.Name,
			Labels:    labels,
			Namespace: namespace,
		},
	}
}

func (r *ReconcileHyperConverged) ensureKubeVirt(instance *hcov1alpha1.HyperConverged, logger logr.Logger, request reconcile.Request) error {
	virt := newKubeVirtForCR(instance, request.Namespace)
	if err := controllerutil.SetControllerReference(instance, virt, r.scheme); err != nil {
		return err
	}

	key, err := client.ObjectKeyFromObject(virt)
	if err != nil {
		logger.Error(err, "Failed to get object key for KubeVirt")
	}

	found := &kubevirtv1.KubeVirt{}
	err = r.client.Get(context.TODO(), key, found)
	if err != nil && errors.IsNotFound(err) {
		logger.Info("Creating kubevirt")
		return r.client.Create(context.TODO(), virt)
	}

	if err != nil {
		return err
	}

	logger.Info("KubeVirt already exists", "KubeVirt.Namespace", found.Namespace, "KubeVirt.Name", found.Name)
	if found.Status.Conditions != nil {
		// TODO: uncomment when kubevirt release with conditions
		// for _, condition := range found.Status.Conditions {
		// 	switch condition.Type {
		// 	case kubevirt.KubeVirtConditionAvailable:
		// 		if condition.Status == corev1.ConditionFalse {
		// 			logger.Info("KubeVirt is not 'Available'")
		// 			conditionsv1.SetStatusCondition(r.conditions, conditionsv1.Condition{
		// 				Type:    conditionsv1.ConditionType(condition.Type),
		// 				Status:  corev1.ConditionStatus(condition.Status),
		// 				Reason:  string(condition.Reason),
		// 				Message: string(condition.Message),
		// 			})
		// 		}
		// 	case kubevirt.KubeVirtConditionProgressing:
		// 		if condition.Status == corev1.ConditionTrue {
		// 			logger.Info("KubeVirt is 'Progressing'")
		// 			conditionsv1.SetStatusCondition(r.conditions, conditionsv1.Condition{
		// 				Type:    conditionsv1.ConditionType(condition.Type),
		// 				Status:  corev1.ConditionStatus(condition.Status),
		// 				Reason:  string(condition.Reason),
		// 				Message: string(condition.Message),
		// 			})
		// 		}
		// 	case kubevirt.KubeVirtConditionDegraded:
		// 		if condition.Status == corev1.ConditionTrue {
		// 			logger.Info("KubeVirt is 'Degraded'")
		// 			conditionsv1.SetStatusCondition(r.conditions, conditionsv1.Condition{
		// 				Type:    conditionsv1.ConditionType(condition.Type),
		// 				Status:  corev1.ConditionStatus(condition.Status),
		// 				Reason:  string(condition.Reason),
		// 				Message: string(condition.Message),
		// 			})
		// 		}
		// 	}
		// }
	}

	// Add it to the list of RelatedObjects if found
	objectRef, err := reference.GetReference(r.scheme, found)
	if err != nil {
		return err
	}
	objectreferencesv1.SetObjectReference(&instance.Status.RelatedObjects, *objectRef)
	return r.client.Status().Update(context.TODO(), instance)
}

// newCDIForCr returns a CDI CR
func newCDIForCR(cr *hcov1alpha1.HyperConverged, namespace string) *cdiv1alpha1.CDI {
	labels := map[string]string{
		"app": cr.Name,
	}
	return &cdiv1alpha1.CDI{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cdi-" + cr.Name,
			Labels:    labels,
			Namespace: namespace,
		},
	}
}

func (r *ReconcileHyperConverged) ensureCDI(instance *hcov1alpha1.HyperConverged, logger logr.Logger, request reconcile.Request) error {
	cdi := newCDIForCR(instance, UndefinedNamespace)
	if err := controllerutil.SetControllerReference(instance, cdi, r.scheme); err != nil {
		return err
	}

	key, err := client.ObjectKeyFromObject(cdi)
	if err != nil {
		logger.Error(err, "Failed to get object key for CDI")
	}

	found := &cdiv1alpha1.CDI{}
	err = r.client.Get(context.TODO(), key, found)
	if err != nil && errors.IsNotFound(err) {
		logger.Info("Creating CDI")
		return r.client.Create(context.TODO(), cdi)
	}

	if err != nil {
		return err
	}

	logger.Info("CDI already exists", "CDI.Namespace", found.Namespace, "CDI.Name", found.Name)
	// TODO: Evaluate conditions
	if found.Status.Conditions != nil {
	}

	// Add it to the list of RelatedObjects if found
	objectRef, err := reference.GetReference(r.scheme, found)
	if err != nil {
		return err
	}
	objectreferencesv1.SetObjectReference(&instance.Status.RelatedObjects, *objectRef)
	return r.client.Status().Update(context.TODO(), instance)
}

// newNetworkAddonsForCR returns a NetworkAddonsConfig CR
func newNetworkAddonsForCR(cr *hcov1alpha1.HyperConverged, namespace string) *networkaddonsv1alpha1.NetworkAddonsConfig {
	labels := map[string]string{
		"app": cr.Name,
	}
	return &networkaddonsv1alpha1.NetworkAddonsConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      networkaddonsnames.OPERATOR_CONFIG,
			Labels:    labels,
			Namespace: namespace,
		},
		Spec: networkaddonsv1alpha1.NetworkAddonsConfigSpec{
			Multus:      &networkaddonsv1alpha1.Multus{},
			LinuxBridge: &networkaddonsv1alpha1.LinuxBridge{},
			KubeMacPool: &networkaddonsv1alpha1.KubeMacPool{},
		},
	}
}

func (r *ReconcileHyperConverged) ensureNetworkAddons(instance *hcov1alpha1.HyperConverged, logger logr.Logger, request reconcile.Request) error {
	networkAddons := newNetworkAddonsForCR(instance, UndefinedNamespace)
	if err := controllerutil.SetControllerReference(instance, networkAddons, r.scheme); err != nil {
		return err
	}

	key, err := client.ObjectKeyFromObject(networkAddons)
	if err != nil {
		logger.Error(err, "Failed to get object key for Network Addons")
	}

	found := &networkaddonsv1alpha1.NetworkAddonsConfig{}
	err = r.client.Get(context.TODO(), key, found)
	if err != nil && errors.IsNotFound(err) {
		logger.Info("Creating Network Addons")
		return r.client.Create(context.TODO(), networkAddons)
	}

	if err != nil {
		return err
	}

	logger.Info("NetworkAddonsConfig already exists", "NetworkAddonsConfig.Namespace", found.Namespace, "NetworkAddonsConfig.Name", found.Name)
	// TODO: Evaluate conditions

	// Add it to the list of RelatedObjects if found
	objectRef, err := reference.GetReference(r.scheme, found)
	if err != nil {
		return err
	}
	objectreferencesv1.SetObjectReference(&instance.Status.RelatedObjects, *objectRef)
	return r.client.Status().Update(context.TODO(), instance)
}

func newKubeVirtCommonTemplateBundleForCR(cr *hcov1alpha1.HyperConverged, namespace string) *sspv1.KubevirtCommonTemplatesBundle {
	labels := map[string]string{
		"app": cr.Name,
	}
	return &sspv1.KubevirtCommonTemplatesBundle{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "common-templates-" + cr.Name,
			Labels:    labels,
			Namespace: OpenshiftNamespace,
		},
	}
}

func (r *ReconcileHyperConverged) ensureKubeVirtCommonTemplateBundle(instance *hcov1alpha1.HyperConverged, logger logr.Logger, request reconcile.Request) error {
	kvCTB := newKubeVirtCommonTemplateBundleForCR(instance, OpenshiftNamespace)
	if err := controllerutil.SetControllerReference(instance, kvCTB, r.scheme); err != nil {
		return err
	}

	key, err := client.ObjectKeyFromObject(kvCTB)
	if err != nil {
		logger.Error(err, "Failed to get object key for KubeVirt Common Templates Bundle")
	}

	found := &sspv1.KubevirtCommonTemplatesBundle{}
	err = r.client.Get(context.TODO(), key, found)
	if err != nil && errors.IsNotFound(err) {
		logger.Info("Creating KubeVirt Common Templates Bundle")
		return r.client.Create(context.TODO(), kvCTB)
	}

	if err != nil {
		return err
	}

	logger.Info("KubeVirt Common Templates Bundle already exists", "bundle.Namespace", found.Namespace, "bundle.Name", found.Name)
	// TODO: Evaluate conditions

	// Add it to the list of RelatedObjects if found
	objectRef, err := reference.GetReference(r.scheme, found)
	if err != nil {
		return err
	}
	objectreferencesv1.SetObjectReference(&instance.Status.RelatedObjects, *objectRef)
	return r.client.Status().Update(context.TODO(), instance)
}

func newKubeVirtNodeLabellerBundleForCR(cr *hcov1alpha1.HyperConverged, namespace string) *sspv1.KubevirtNodeLabellerBundle {
	labels := map[string]string{
		"app": cr.Name,
	}
	return &sspv1.KubevirtNodeLabellerBundle{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "node-labeller-" + cr.Name,
			Labels:    labels,
			Namespace: namespace,
		},
	}
}

func (r *ReconcileHyperConverged) ensureKubeVirtNodeLabellerBundle(instance *hcov1alpha1.HyperConverged, logger logr.Logger, request reconcile.Request) error {
	kvNLB := newKubeVirtNodeLabellerBundleForCR(instance, request.Namespace)
	if err := controllerutil.SetControllerReference(instance, kvNLB, r.scheme); err != nil {
		return err
	}

	key, err := client.ObjectKeyFromObject(kvNLB)
	if err != nil {
		logger.Error(err, "Failed to get object key for KubeVirt Node Labeller Bundle")
	}

	found := &sspv1.KubevirtNodeLabellerBundle{}
	err = r.client.Get(context.TODO(), key, found)
	if err != nil && errors.IsNotFound(err) {
		logger.Info("Creating KubeVirt Node Labeller Bundle")
		return r.client.Create(context.TODO(), kvNLB)
	}

	if err != nil {
		return err
	}

	logger.Info("KubeVirt Node Labeller Bundle already exists", "bundle.Namespace", found.Namespace, "bundle.Name", found.Name)
	// TODO: Evaluate conditions

	// Add it to the list of RelatedObjects if found
	objectRef, err := reference.GetReference(r.scheme, found)
	if err != nil {
		return err
	}
	objectreferencesv1.SetObjectReference(&instance.Status.RelatedObjects, *objectRef)
	return r.client.Status().Update(context.TODO(), instance)
}

func newKubeVirtTemplateValidatorForCR(cr *hcov1alpha1.HyperConverged, namespace string) *sspv1.KubevirtTemplateValidator {
	labels := map[string]string{
		"app": cr.Name,
	}
	return &sspv1.KubevirtTemplateValidator{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "template-validator-" + cr.Name,
			Labels:    labels,
			Namespace: namespace,
		},
	}
}

func (r *ReconcileHyperConverged) ensureKubeVirtTemplateValidator(instance *hcov1alpha1.HyperConverged, logger logr.Logger, request reconcile.Request) error {
	kvTV := newKubeVirtTemplateValidatorForCR(instance, request.Namespace)
	if err := controllerutil.SetControllerReference(instance, kvTV, r.scheme); err != nil {
		return err
	}

	key, err := client.ObjectKeyFromObject(kvTV)
	if err != nil {
		logger.Error(err, "Failed to get object key for KubeVirt Template Validator")
	}

	found := &sspv1.KubevirtTemplateValidator{}
	err = r.client.Get(context.TODO(), key, found)
	if err != nil && errors.IsNotFound(err) {
		logger.Info("Creating KubeVirt Template Validator")
		return r.client.Create(context.TODO(), kvTV)
	}

	if err != nil {
		return err
	}

	logger.Info("KubeVirt Template Validator already exists", "validator.Namespace", found.Namespace, "validator.Name", found.Name)
	// TODO: Evaluate conditions

	// Add it to the list of RelatedObjects if found
	objectRef, err := reference.GetReference(r.scheme, found)
	if err != nil {
		return err
	}
	objectreferencesv1.SetObjectReference(&instance.Status.RelatedObjects, *objectRef)
	return r.client.Status().Update(context.TODO(), instance)
}

func newKubeVirtMetricsAggregationForCR(cr *hcov1alpha1.HyperConverged, namespace string) *sspv1.KubevirtMetricsAggregation {
	labels := map[string]string{
		"app": cr.Name,
	}
	return &sspv1.KubevirtMetricsAggregation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "metrics-aggregation-" + cr.Name,
			Labels:    labels,
			Namespace: namespace,
		},
	}
}

func (r *ReconcileHyperConverged) ensureKubeVirtMetricsAggregation(instance *hcov1alpha1.HyperConverged, logger logr.Logger, request reconcile.Request) error {
	kubevirtMetricsAggregation := newKubeVirtMetricsAggregationForCR(instance, request.Namespace)
	if err := controllerutil.SetControllerReference(instance, kubevirtMetricsAggregation, r.scheme); err != nil {
		return err
	}

	key, err := client.ObjectKeyFromObject(kubevirtMetricsAggregation)
	if err != nil {
		logger.Error(err, "Failed to get object key for KubeVirt Metrics Aggregation")
	}

	found := &sspv1.KubevirtMetricsAggregation{}
	err = r.client.Get(context.TODO(), key, found)
	if err != nil && errors.IsNotFound(err) {
		logger.Info("Creating KubeVirt Metrics Aggregation")
		return r.client.Create(context.TODO(), kubevirtMetricsAggregation)
	}

	if err != nil {
		return err
	}

	logger.Info("KubeVirt Metrics Aggregation already exists", "metrics.Namespace", found.Namespace, "metrics.Name", found.Name)
	// TODO: Evaluate conditions

	// Add it to the list of RelatedObjects if found
	objectRef, err := reference.GetReference(r.scheme, found)
	if err != nil {
		return err
	}
	objectreferencesv1.SetObjectReference(&instance.Status.RelatedObjects, *objectRef)
	return r.client.Status().Update(context.TODO(), instance)
}
