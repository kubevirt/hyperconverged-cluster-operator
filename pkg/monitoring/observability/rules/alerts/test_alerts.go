package alerts

import (
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func testAlerts() []promv1.Rule {
	return []promv1.Rule{
		{
			Alert: "Test",
			Expr:  intstr.FromString("test == 10"),
			Annotations: map[string]string{
				"description": "Test",
				"summary":     "Test",
			},
			Labels: map[string]string{
				"severity":               "critical",
				"operator_health_impact": "critical",
			},
		},
	}
}
