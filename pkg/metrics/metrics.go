package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

// HcoMetrics wrapper for all hco metrics
var HcoMetrics = hcoMetrics{prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "kubevirt_hco_out_of_band_modifications_count",
		Help: "Count of out-of-band modifications overwritten by HCO",
	},
	[]string{"component_name"},
)}

// hcoMetrics holds all HCO metrics
type hcoMetrics struct {
	// overwrittenModifications counts out-of-band modifications overwritten by HCO
	overwrittenModifications *prometheus.CounterVec
}

func init() {
	HcoMetrics.init()
}

func (hm *hcoMetrics) init() {
	metrics.Registry.MustRegister(hm.overwrittenModifications)
}

// IncOverwrittenModifications increments counter by 1
func (hm *hcoMetrics) IncOverwrittenModifications(componentName string) {
	hm.overwrittenModifications.With(prometheus.Labels{"component_name": componentName}).Inc()
}

// GetOverwrittenModificationsCount returns current value of counter. If error is not nil then value is undefined
func (hm *hcoMetrics) GetOverwrittenModificationsCount(componentName string) (float64, error) {
	var m = &dto.Metric{}
	err := hm.overwrittenModifications.With(prometheus.Labels{"component_name": componentName}).Write(m)
	return m.Counter.GetValue(), err
}
