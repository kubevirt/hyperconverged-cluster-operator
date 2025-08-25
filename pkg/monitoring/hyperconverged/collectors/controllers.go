package collectors

import (
	"github.com/rhobs/operator-observability-toolkit/pkg/operatormetrics"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func SetupCollectors(cli client.Client, namespace string) error {
	err := operatormetrics.RegisterCollector(
		getMultiArchBootImagesStatusCollector(cli, namespace),
	)

	if err != nil {
		return err
	}

	return nil
}
