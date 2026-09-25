package collectors

import (
	"github.com/rhobs/operator-observability-toolkit/pkg/operatormetrics"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func SetupCollectors(cli client.Client, namespace string) error {
	return operatormetrics.RegisterCollector(
		getMultiArchBootImagesStatusCollector(cli, namespace),
		getFeatureGateEnabledCollector(cli, namespace),
	)
}

// CollectorMetrics returns collector-owned metrics so docs and the metric
// linter can list them without constructing a Kubernetes client.
func CollectorMetrics() []operatormetrics.Metric {
	return []operatormetrics.Metric{
		multiArchBootImagesStatus,
		featureGateEnabled,
	}
}
