package rules

import (
	"fmt"

	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/rhobs/operator-observability-toolkit/pkg/operatorrules"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/monitoring/observability/rules/alerts"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

const prometheusRuleName = "kubevirt-cnv-prometheus-rules"

var operatorRegistry = operatorrules.NewRegistry()

func SetupRules() error {
	return alerts.Register(operatorRegistry)
}

func BuildPrometheusRule(namespace string, owner *metav1.OwnerReference) (*promv1.PrometheusRule, error) {
	rules, err := operatorRegistry.BuildPrometheusRule(
		prometheusRuleName,
		namespace,
		hcoutil.GetLabels(hcoutil.HyperConvergedName, hcoutil.AppComponentMonitoring),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to build PrometheusRule: %v", err)
	}

	rules.OwnerReferences = []metav1.OwnerReference{*owner}

	return rules, nil
}

func ListAlerts() []promv1.Rule {
	return operatorRegistry.ListAlerts()
}
