package alerts

import hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"

const (
	defaultMonitoringNamespace   = "monitoring"
	openshiftMonitoringNamespace = "openshift-monitoring"
)

func getMonitoringNamespace(ci hcoutil.ClusterInfo) string {
	if ci.IsOpenshift() {
		return openshiftMonitoringNamespace
	}

	return defaultMonitoringNamespace
}
