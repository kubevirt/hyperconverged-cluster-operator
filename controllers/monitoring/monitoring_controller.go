package monitoring

import (
	"context"
	"fmt"
	"reflect"

	operatorhandler "github.com/operator-framework/operator-lib/handler"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

const (
	alertRuleGroup                = "kubevirt.hyperconverged.rules"
	outOfBandUpdateAlert          = "KubevirtHyperconvergedClusterOperatorCRModification"
	unsafeModificationAlert       = "KubevirtHyperconvergedClusterOperatorUSModification"
	installationNotCompletedAlert = "KubevirtHyperconvergedClusterOperatorInstallationNotCompletedAlert"
	severityAlertLabelKey         = "severity"
	partOfAlertLabelKey           = "kubernetes_operator_part_of"
	partOfAlertLabelValue         = "kubevirt"
	componentAlertLabelKey        = "kubernetes_operator_component"
	componentAlertLabelValue      = "hyperconverged-cluster-operator"
	ruleName                      = hcoutil.HyperConvergedName + "-prometheus-rule"
)

var (
	log = logf.Log.WithName(ruleName)

	runbookUrlTemplate = "https://kubevirt.io/monitoring/runbooks/%s"

	outOfBandUpdateRunbookUrl          = fmt.Sprintf(runbookUrlTemplate, outOfBandUpdateAlert)
	unsafeModificationRunbookUrl       = fmt.Sprintf(runbookUrlTemplate, unsafeModificationAlert)
	installationNotCompletedRunbookUrl = fmt.Sprintf(runbookUrlTemplate, installationNotCompletedAlert)
)

type AlertRuleReconciler struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client     client.Client
	scheme     *runtime.Scheme
	namespace  string
	deployment *appsv1.Deployment
	theRule    *monitoringv1.PrometheusRule
}

func newAlertRuleReconciler(mgr manager.Manager, ci hcoutil.ClusterInfo, namespace string) *AlertRuleReconciler {

	r := &AlertRuleReconciler{
		client:     mgr.GetClient(),
		scheme:     mgr.GetScheme(),
		namespace:  namespace,
		deployment: ci.GetDeployment(),
	}

	r.theRule = r.newPrometheusRule()

	return r
}

func RegisterReconciler(mgr manager.Manager, ci hcoutil.ClusterInfo) error {
	namespace, err := hcoutil.GetOperatorNamespaceFromEnv()
	if err != nil {
		return err
	}

	return add(mgr, newAlertRuleReconciler(mgr, ci, namespace))
}

func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("alert-rule-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	err = c.Watch(
		&source.Kind{Type: &monitoringv1.PrometheusRule{}},
		&operatorhandler.InstrumentedEnqueueRequestForObject{},
		predicate.Or(predicate.GenerationChangedPredicate{}, predicate.AnnotationChangedPredicate{}),
	)

	return err
}

var _ reconcile.Reconciler = &AlertRuleReconciler{}

func (r *AlertRuleReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {

	logger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	logger.Info("Reconciling the PrometheusRule", "namespacedName.Name", request.NamespacedName.Name, "namespacedName.Namespace", request.NamespacedName.Namespace)

	rule := &monitoringv1.PrometheusRule{}
	logger.Info("Reading the ")
	err := r.client.Get(ctx, request.NamespacedName, rule)

	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Can't find the Prometheus rule; creating a new one")
			err := r.client.Create(ctx, r.theRule.DeepCopy())
			if err != nil {
				logger.Error(err, "failed to create PrometheusRule")
				return reconcile.Result{}, err
			}
			logger.Info("successfully created the PrometheusRule")
			return reconcile.Result{}, nil
		}

		logger.Error(err, "unexpected error while reading the PrometheusRule")
		return reconcile.Result{}, err
	}

	if !reflect.DeepEqual(r.theRule.Spec, rule.Spec) {
		logger.Info("updating the PrometheusRule")
		err = r.client.Update(ctx, rule)
		if err != nil {
			logger.Error(err, "failed to update the PrometheusRule")
			return reconcile.Result{}, err
		}
	}

	logger.V(5).Info("nothing to do")
	return reconcile.Result{}, nil
}

func (r AlertRuleReconciler) newPrometheusRule() *monitoringv1.PrometheusRule {
	return &monitoringv1.PrometheusRule{
		TypeMeta: metav1.TypeMeta{
			APIVersion: monitoringv1.SchemeGroupVersion.String(),
			Kind:       "PrometheusRule",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      ruleName,
			Labels:    map[string]string{hcoutil.AppLabel: hcoutil.HyperConvergedName},
			Namespace: r.namespace,
			OwnerReferences: []metav1.OwnerReference{
				getDeploymentReference(r.deployment),
			},
		},
		Spec: *NewPrometheusRuleSpec(),
	}
}

// NewPrometheusRuleSpec creates PrometheusRuleSpec for alert rules
func NewPrometheusRuleSpec() *monitoringv1.PrometheusRuleSpec {
	return &monitoringv1.PrometheusRuleSpec{
		Groups: []monitoringv1.RuleGroup{{
			Name: alertRuleGroup,
			Rules: []monitoringv1.Rule{
				{
					Alert: outOfBandUpdateAlert,
					Expr:  intstr.FromString("sum by(component_name) ((round(increase(kubevirt_hco_out_of_band_modifications_count[10m]))>0 and kubevirt_hco_out_of_band_modifications_count offset 10m) or (kubevirt_hco_out_of_band_modifications_count != 0 unless kubevirt_hco_out_of_band_modifications_count offset 10m))"),
					Annotations: map[string]string{
						"description": "Out-of-band modification for {{ $labels.component_name }}.",
						"summary":     "{{ $value }} out-of-band CR modifications were detected in the last 10 minutes.",
						"runbook_url": outOfBandUpdateRunbookUrl,
					},
					Labels: map[string]string{
						severityAlertLabelKey:  "warning",
						partOfAlertLabelKey:    partOfAlertLabelValue,
						componentAlertLabelKey: componentAlertLabelValue,
					},
				},
				{
					Alert: unsafeModificationAlert,
					Expr:  intstr.FromString("sum by(annotation_name) ((kubevirt_hco_unsafe_modification_count)>0)"),
					Annotations: map[string]string{
						"description": "unsafe modification for the {{ $labels.annotation_name }} annotation in the HyperConverged resource.",
						"summary":     "{{ $value }} unsafe modifications were detected in the HyperConverged resource.",
						"runbook_url": unsafeModificationRunbookUrl,
					},
					Labels: map[string]string{
						severityAlertLabelKey:  "info",
						partOfAlertLabelKey:    partOfAlertLabelValue,
						componentAlertLabelKey: componentAlertLabelValue,
					},
				},
				{
					Alert: installationNotCompletedAlert,
					Expr:  intstr.FromString("kubevirt_hco_hyperconverged_cr_exists == 0"),
					Annotations: map[string]string{
						"description": "the installation was not completed; the HyperConverged custom resource is missing. In order to complete the installation of the Hyperconverged Cluster Operator you should create the HyperConverged custom resource.",
						"summary":     "the installation was not completed; to complete the installation, create a HyperConverged custom resource.",
						"runbook_url": installationNotCompletedRunbookUrl,
					},
					For: "1h",
					Labels: map[string]string{
						severityAlertLabelKey:  "info",
						partOfAlertLabelKey:    partOfAlertLabelValue,
						componentAlertLabelKey: componentAlertLabelValue,
					},
				},
				// Recording rules for openshift/cluster-monitoring-operator
				{
					Record: "cluster:vmi_request_cpu_cores:sum",
					Expr:   intstr.FromString(`sum(kube_pod_container_resource_requests{resource="cpu"} and on (pod) kube_pod_status_phase{phase="Running"} * on (pod) group_left kube_pod_labels{ label_kubevirt_io="virt-launcher"} > 0)`),
				},
			},
		}},
	}
}

func getDeploymentReference(deployment *appsv1.Deployment) metav1.OwnerReference {
	gvk := deployment.GroupVersionKind()
	return metav1.OwnerReference{
		APIVersion:         gvk.GroupVersion().String(),
		Kind:               gvk.Kind,
		Name:               deployment.GetName(),
		UID:                deployment.GetUID(),
		BlockOwnerDeletion: pointer.BoolPtr(false),
		Controller:         pointer.BoolPtr(false),
	}
}
