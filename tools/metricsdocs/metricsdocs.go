package main

import (
	"fmt"

	"github.com/rhobs/operator-observability-toolkit/pkg/docs"

	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/monitoring/hyperconverged/collectors"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/monitoring/hyperconverged/metrics"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/monitoring/hyperconverged/rules"
)

const title = `Hyperconverged Cluster Operator metrics`

func main() {
	err := metrics.SetupMetrics()
	if err != nil {
		panic(err)
	}

	err = rules.SetupRules()
	if err != nil {
		panic(err)
	}

	metricsList := metrics.ListMetrics()
	metricsList = append(metricsList, collectors.CollectorMetrics()...)
	rulesList := rules.ListRecordingRules()

	docsString := docs.BuildMetricsDocs(title, metricsList, rulesList)
	fmt.Print(docsString)
}
