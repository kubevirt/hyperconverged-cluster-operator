package main

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/metrics"
)

type fakeCollector struct {
}

func (fc fakeCollector) Describe(_ chan<- *prometheus.Desc) {
}

//Collect needs to report all metrics to see it in docs
func (fc fakeCollector) Collect(ch chan<- prometheus.Metric) {
	ps := metrics.NewPrometheusScraper(ch)
	ps.Report("test")
}

func RegisterFakeCollector() {
	prometheus.MustRegister(fakeCollector{})
}
