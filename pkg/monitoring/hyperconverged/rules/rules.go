package rules

import (
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/rhobs/operator-observability-toolkit/pkg/operatorrules"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/monitoring/hyperconverged/rules/alerts"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/monitoring/hyperconverged/rules/recordingrules"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

const (
	ruleName = hcoutil.HyperConvergedName + "-prometheus-rule"
)

var operatorRegistry = operatorrules.NewRegistry()

// SetupRules registers the recording rules and alerts in the operator's
// registry. The installNamespace parameter is the namespace OpenShift
// Virtualization is installed in; it scopes the operator-health alerts to the
// control-plane pods and CSVs.
func SetupRules(installNamespace string) error {
	err := recordingrules.Register(operatorRegistry)
	if err != nil {
		return err
	}

	err = alerts.Register(operatorRegistry, installNamespace)
	if err != nil {
		return err
	}

	return nil
}

func BuildPrometheusRule(namespace string, owner metav1.OwnerReference) (*promv1.PrometheusRule, error) {
	rules, err := operatorRegistry.BuildPrometheusRule(
		ruleName,
		namespace,
		hcoutil.GetLabels(hcoutil.HyperConvergedName, hcoutil.AppComponentMonitoring),
	)
	if err != nil {
		return nil, err
	}

	rules.OwnerReferences = []metav1.OwnerReference{owner}

	return rules, nil
}

func ListRecordingRules() []operatorrules.RecordingRule {
	return operatorRegistry.ListRecordingRules()
}

func ListAlerts() []promv1.Rule {
	return operatorRegistry.ListAlerts()
}
