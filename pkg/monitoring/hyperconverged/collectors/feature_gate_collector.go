package collectors

import (
	"context"

	"github.com/rhobs/operator-observability-toolkit/pkg/operatormetrics"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/featuregatedetails"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/featuregates"
)

const (
	featureGateEnabledValue  = float64(1)
	featureGateDisabledValue = float64(0)

	featureGateLabelName  = "feature_gate"
	featureGateLabelPhase = "phase"
)

var featureGateEnabled = operatormetrics.NewGaugeVec(
	operatormetrics.MetricOpts{
		Name: "kubevirt_hco_feature_gate_enabled",
		Help: "Indicates whether an alpha or beta HyperConverged feature gate is configured as enabled (1) or disabled (0). Legacy feature gates superseded by dedicated configuration fields are excluded.",
	},
	[]string{featureGateLabelName, featureGateLabelPhase},
)

func getFeatureGateEnabledCollector(cli client.Client, operatorNamespace string) operatormetrics.Collector {
	return operatormetrics.Collector{
		Metrics: []operatormetrics.Metric{
			featureGateEnabled,
		},
		CollectCallback: getFeatureGateEnabledCallback(cli, operatorNamespace),
	}
}

// FeatureGateEnabledMetric exposes the collector metric for documentation and validation.
func FeatureGateEnabledMetric() operatormetrics.Metric {
	return featureGateEnabled
}

func getFeatureGateEnabledCallback(cli client.Client, operatorNamespace string) func() []operatormetrics.CollectorResult {
	return func() []operatormetrics.CollectorResult {
		hc := &hcov1.HyperConverged{}
		key := client.ObjectKey{Name: hcov1.HyperConvergedName, Namespace: operatorNamespace}
		err := cli.Get(context.TODO(), key, hc)
		if err != nil {
			if !errors.IsNotFound(err) {
				logger.Error(err, "can't read HyperConverged CR")
			}
			return []operatormetrics.CollectorResult{}
		}

		return featureGateEnabledSamples(featuregatedetails.ListConfigurableFeatureGates(), hc.Spec.FeatureGates.IsEnabled)
	}
}

func featureGateEnabledSamples(gates []featuregates.FeatureGate, isEnabled func(string) bool) []operatormetrics.CollectorResult {
	results := make([]operatormetrics.CollectorResult, 0, len(gates))
	for _, fg := range gates {
		value := featureGateDisabledValue
		if isEnabled(fg.Name) {
			value = featureGateEnabledValue
		}

		results = append(results, operatormetrics.CollectorResult{
			Metric: featureGateEnabled,
			Labels: []string{fg.Name, fg.Phase.String()},
			Value:  value,
		})
	}

	return results
}
