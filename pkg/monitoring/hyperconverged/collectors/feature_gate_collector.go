package collectors

import (
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
		Help: "Indicates whether a HyperConverged feature gate is enabled (1) or disabled (0). One series is emitted for every known configurable feature gate.",
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

func getFeatureGateEnabledCallback(cli client.Client, operatorNamespace string) func() []operatormetrics.CollectorResult {
	return func() []operatormetrics.CollectorResult {
		gates := featuregatedetails.ListConfigurableFeatureGates()

		hc, err := getHyperConverged(cli, operatorNamespace)
		if err != nil {
			if errors.IsNotFound(err) {
				return featureGateEnabledSamples(nil, gates)
			}

			logger.Error(err, "can't read HyperConverged CR")
			return []operatormetrics.CollectorResult{}
		}

		return featureGateEnabledSamples(hc, gates)
	}
}

func featureGateEnabledSamples(hc *hcov1.HyperConverged, gates []featuregates.FeatureGate) []operatormetrics.CollectorResult {
	results := make([]operatormetrics.CollectorResult, 0, len(gates))
	for _, fg := range gates {
		value := featureGateDisabledValue
		if hc != nil && hc.Spec.FeatureGates.IsEnabled(fg.Name) {
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
