package main

import (
	"fmt"
	"strings"

	"github.com/rhobs/operator-observability-toolkit/pkg/docs"

	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/monitoring/hyperconverged/metrics"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/monitoring/hyperconverged/rules"
)

const title = `# Hyperconverged Cluster Operator metrics title`

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
	rulesList := rules.ListRecordingRules()

	docsString := docs.BuildMetricsDocs(metricsList, rulesList)
	docsString = strings.Replace(docsString, "# Operator Metrics", title, 1)
	fmt.Print(docsString)
}
