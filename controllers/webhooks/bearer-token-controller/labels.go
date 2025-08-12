package bearer_token_controller

import (
	"maps"

	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

var (
	labels = hcoutil.GetLabels(hcoutil.HCOWebhookName, hcoutil.AppComponentMonitoring)
)

func getLabels() map[string]string {
	return maps.Clone(labels)
}
