package collectors

import (
	"context"

	"github.com/rhobs/operator-observability-toolkit/pkg/operatormetrics"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/nodeinfo"
)

const (
	multiArchBootImagesFeatureEnabled  = float64(1)
	multiArchBootImagesFeatureDisabled = float64(0)
)

var (
	multiArchBootImagesStatus = operatormetrics.NewGauge(
		operatormetrics.MetricOpts{
			Name: "kubevirt_hco_multi_arch_boot_images_enabled",
			Help: "indicates if the Multi-Arch Boot Images feature is enabled (1) or not (0)",
		},
	)

	logger = logf.Log.WithName("operator-metrics-collector")
)

func getMultiArchBootImagesStatusCollector(cli client.Client, operatorNamespace string) operatormetrics.Collector {
	return operatormetrics.Collector{
		Metrics: []operatormetrics.Metric{
			multiArchBootImagesStatus,
		},
		CollectCallback: getMultiArchBootImagesStatusCallback(cli, operatorNamespace),
	}
}

func getMultiArchBootImagesStatusCallback(cli client.Client, operatorNamespace string) func() []operatormetrics.CollectorResult {
	return func() []operatormetrics.CollectorResult {
		hc := hcov1beta1.HyperConverged{}
		key := client.ObjectKey{Name: hcov1beta1.HyperConvergedName, Namespace: operatorNamespace}
		if err := cli.Get(context.TODO(), key, &hc); err != nil {
			if !errors.IsNotFound(err) {
				logger.Info("HyperConverged not found")
			} else {
				logger.Error(err, "can't read HyperConverged")
			}
			// Don't set the metric if the HyperConverged CR does not exist
			return []operatormetrics.CollectorResult{}
		}

		if len(nodeinfo.GetWorkloadsArchitectures()) <= 1 {
			// Don't set the metric if the cluster is not multi-architecture
			return []operatormetrics.CollectorResult{}
		}

		if len(hc.Status.DataImportCronTemplates) == 0 {
			// Don't set the metric if no common nor custom DataImportCronTemplates found
			return []operatormetrics.CollectorResult{}
		}

		// Set the metric based on the FeatureGate value
		value := multiArchBootImagesFeatureDisabled
		if ptr.Deref(hc.Spec.FeatureGates.EnableMultiArchBootImageImport, false) {
			logger.Info("Multi-Arch boot images feature is enabled")
			value = multiArchBootImagesFeatureEnabled
		} else {
			logger.Info("Multi-Arch boot images feature is disabled, but running on a multi-arch cluster with boot images enabled")
		}

		return []operatormetrics.CollectorResult{
			{
				Metric: multiArchBootImagesStatus,
				Value:  value,
			},
		}
	}
}
