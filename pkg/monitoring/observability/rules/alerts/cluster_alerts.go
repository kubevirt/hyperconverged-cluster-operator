package alerts

import (
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

func clusterAlerts() []promv1.Rule {
	return []promv1.Rule{
		{
			Alert: "HighCPUWorkload",
			Expr:  intstr.FromString("instance:node_cpu_utilisation:rate1m >= 0.9"),
			For:   ptr.To(promv1.Duration("5m")),
			Annotations: map[string]string{
				"summary":     "High CPU usage on host {{ $labels.instance }}",
				"description": "CPU utilization for {{ $labels.instance }} has been above 90% for more than 5 minutes.",
			},
			Labels: map[string]string{
				"severity":               "warning",
				"operator_health_impact": "none",
			},
		},
	}
}
