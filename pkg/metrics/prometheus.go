package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log"
	"net/http"
)

func Handler(MaxRequestsInFlight int) http.Handler {
	return promhttp.InstrumentMetricHandler(
		prometheus.DefaultRegisterer,
		promhttp.HandlerFor(
			prometheus.DefaultGatherer,
			promhttp.HandlerOpts{
				MaxRequestsInFlight: MaxRequestsInFlight,
			}),
	)
}

func NewPrometheusScraper(ch chan<- prometheus.Metric) *prometheusScraper {
	return &prometheusScraper{ch: ch}
}

type prometheusScraper struct {
	ch chan<- prometheus.Metric
}

func (ps *prometheusScraper) Report(socketFile string) {
	defer func() {
		if err := recover(); err != nil {
			log.Panicf("collector goroutine panicked for VM %s: %s", socketFile, err)
		}
	}()

	ps.newMetric(overwrittenModifications)
	ps.newMetric(unsafeModifications)
}

func (ps *prometheusScraper) newMetric(pd metricDesc) {
	desc := prometheus.NewDesc(
		pd.fqName,
		pd.help,
		pd.constLabelPairs,
		nil,
	)

	mv, err := prometheus.NewConstMetric(desc, pd.prometheusType, 1024, pd.constLabelPairs...)
	if err != nil {
		panic(err)
	}
	ps.ch <- mv
}
