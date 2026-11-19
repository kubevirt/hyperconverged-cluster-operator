package alerts

import (
	"errors"
	"fmt"
	"os"
	"strings"

	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/rhobs/operator-observability-toolkit/pkg/operatorrules"
)

const (
	prometheusRunbookAnnotationKey = "runbook_url"
	partOfAlertLabelKey            = "kubernetes_operator_part_of"
	partOfAlertLabelValue          = "kubevirt"
	componentAlertLabelKey         = "kubernetes_operator_component"
	componentAlertLabelValue       = "hyperconverged-cluster-operator"
	defaultRunbookURLTemplate      = "https://kubevirt.io/monitoring/runbooks/%s"
	runbookURLTemplateEnv          = "RUNBOOK_URL_TEMPLATE"
)

// Register registers the alerts in the given registry. The installNamespace
// parameter is the namespace OpenShift Virtualization is installed in; it
// scopes the operator-health alerts to the control-plane pods and CSVs.
func Register(operatorRegistry *operatorrules.Registry, installNamespace string) error {
	alerts := [][]promv1.Rule{
		operatorAlerts(installNamespace),
		healthAlerts(),
	}

	runbookURLTemplate := getRunbookURLTemplate()
	for _, alertGroup := range alerts {
		for _, alert := range alertGroup {
			alert.Labels[partOfAlertLabelKey] = partOfAlertLabelValue
			alert.Labels[componentAlertLabelKey] = componentAlertLabelValue
			alert.Annotations[prometheusRunbookAnnotationKey] = fmt.Sprintf(runbookURLTemplate, alert.Alert)
		}

	}

	return operatorRegistry.RegisterAlerts(alerts...)
}

func getRunbookURLTemplate() string {
	runbookURLTemplate, exists := os.LookupEnv(runbookURLTemplateEnv)
	if !exists {
		runbookURLTemplate = defaultRunbookURLTemplate
	}

	if strings.Count(runbookURLTemplate, "%s") != 1 {
		panic(errors.New("runbook URL template must have exactly 1 %s substring"))
	}

	return runbookURLTemplate
}
