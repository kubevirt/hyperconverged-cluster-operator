package metrics

import (
	"strings"

	ioprometheusclient "github.com/prometheus/client_model/go"
	"github.com/rhobs/operator-observability-toolkit/pkg/operatormetrics"
)

const (
	counterLabelCompName = "component_name"
	counterLabelAnnName  = "annotation_name"

	hyperConvergedExists    = 1.0
	hyperConvergedNotExists = 0.0
)

const (
	SystemHealthStatusHealthy float64 = iota
	SystemHealthStatusWarning
	SystemHealthStatusError
)

const (
	counterLabelDICTName = "data_import_cron_name"
	counterLabelDSName   = "managed_data_source_name"

	hasSupportedArchitectures   = float64(1)
	hasNoSupportedArchitectures = float64(0)

	hasArchitectureAnnotation   = float64(1)
	hasNoArchitectureAnnotation = float64(0)
)

var (
	operatorMetrics = []operatormetrics.Metric{
		overwrittenModifications,
		unsafeModifications,
		hyperConvergedCRExists,
		systemHealthStatus,
		dictWithSupportedArchitectures,
		dictWithArchitectureAnnotation,
		memoryOvercommitPercentage,
	}

	overwrittenModifications = operatormetrics.NewCounterVec(
		operatormetrics.MetricOpts{
			Name: "kubevirt_hco_out_of_band_modifications_total",
			Help: "Count of out-of-band modifications overwritten by HCO",
		},
		[]string{counterLabelCompName},
	)

	unsafeModifications = operatormetrics.NewGaugeVec(
		operatormetrics.MetricOpts{
			Name: "kubevirt_hco_unsafe_modifications",
			Help: "Count of unsafe modifications in the HyperConverged annotations",
		},
		[]string{counterLabelAnnName},
	)

	hyperConvergedCRExists = operatormetrics.NewGauge(
		operatormetrics.MetricOpts{
			Name: "kubevirt_hco_hyperconverged_cr_exists",
			Help: "Indicates whether the HyperConverged custom resource exists (1) or not (0)",
		},
	)

	systemHealthStatus = operatormetrics.NewGauge(
		operatormetrics.MetricOpts{
			Name: "kubevirt_hco_system_health_status",
			Help: "Indicates whether the system health status is healthy (0), warning (1), or error (2), by aggregating the conditions of HCO and its secondary resources",
		},
	)

	dictWithSupportedArchitectures = operatormetrics.NewGaugeVec(
		operatormetrics.MetricOpts{
			Name: "kubevirt_hco_dataimportcrontemplate_with_supported_architectures",
			Help: "Indicates whether the DataImportCronTemplate has supported architectures (1) or not (0)",
		},
		[]string{counterLabelDICTName, counterLabelDSName},
	)

	dictWithArchitectureAnnotation = operatormetrics.NewGaugeVec(
		operatormetrics.MetricOpts{
			Name: "kubevirt_hco_dataimportcrontemplate_with_architecture_annotation",
			Help: "Indicates whether the DataImportCronTemplate has the ssp.kubevirt.io/dict.architectures annotation (1) or not (0)",
		},
		[]string{counterLabelDICTName, counterLabelDSName},
	)

	memoryOvercommitPercentage = operatormetrics.NewGauge(
		operatormetrics.MetricOpts{
			Name: "kubevirt_hco_memory_overcommit_percentage",
			Help: "Indicates the cluster-wide configured VM memory overcommit percentage",
		},
	)
)

// IncOverwrittenModifications increments counter by 1
func IncOverwrittenModifications(kind, name string) {
	overwrittenModifications.WithLabelValues(getLabelsForObj(kind, name)).Inc()
}

// GetOverwrittenModificationsCount returns current value of counter. If error is not nil then value is undefined
func GetOverwrittenModificationsCount(kind, name string) (float64, error) {
	dto := &ioprometheusclient.Metric{}
	err := overwrittenModifications.WithLabelValues(getLabelsForObj(kind, name)).Write(dto)
	value := dto.Counter.GetValue()

	if err != nil {
		return 0, err
	}
	return value, nil
}

// SetHCOMetricMemoryOvercommitPercentage sets the memory overcommit percentage from the HCO CR
func SetHCOMetricMemoryOvercommitPercentage(percentage float64) {
	memoryOvercommitPercentage.Set(percentage)
}

// GetHCOMetricMemoryOvercommitPercentage returns current value of gauge. If error is not nil then value is undefined
func GetHCOMetricMemoryOvercommitPercentage() (float64, error) {
	dto := &ioprometheusclient.Metric{}
	err := memoryOvercommitPercentage.Write(dto)
	value := dto.Gauge.GetValue()

	if err != nil {
		return 0, err
	}
	return value, nil
}

// SetUnsafeModificationCount sets the gauge to the required number
func SetUnsafeModificationCount(count int, unsafeAnnotation string) {
	unsafeModifications.WithLabelValues(getLabelsForUnsafeAnnotation(unsafeAnnotation)).Set(float64(count))
}

// GetUnsafeModificationsCount returns current value of gauge. If error is not nil then value is undefined
func GetUnsafeModificationsCount(unsafeAnnotation string) (float64, error) {
	dto := &ioprometheusclient.Metric{}
	err := unsafeModifications.WithLabelValues(getLabelsForUnsafeAnnotation(unsafeAnnotation)).Write(dto)
	value := dto.Gauge.GetValue()

	if err != nil {
		return 0, err
	}
	return value, nil
}

// SetHCOMetricHyperConvergedExists sets the gauge to 1 (true)
func SetHCOMetricHyperConvergedExists() {
	hyperConvergedCRExists.Set(hyperConvergedExists)
}

// SetHCOMetricHyperConvergedNotExists sets the gauge to 0 (false)
func SetHCOMetricHyperConvergedNotExists() {
	hyperConvergedCRExists.Set(hyperConvergedNotExists)
}

// IsHCOMetricHyperConvergedExists returns true if the HyperConverged custom resource exists; else, return false
func IsHCOMetricHyperConvergedExists() (bool, error) {
	dto := &ioprometheusclient.Metric{}
	err := hyperConvergedCRExists.Write(dto)
	value := dto.Gauge.GetValue()

	if err != nil {
		return false, err
	}

	return value == hyperConvergedExists, nil
}

func SetHCOMetricSystemHealthStatus(status float64) {
	systemHealthStatus.Set(status)
}

func GetHCOMetricSystemHealthStatus() (float64, error) {
	dto := &ioprometheusclient.Metric{}
	err := systemHealthStatus.Write(dto)
	value := dto.Gauge.GetValue()

	if err != nil {
		return 0, err
	}
	return value, nil
}

func SetDICTWithSupportedArchitectures(dictName, dsName string) {
	dictWithSupportedArchitectures.WithLabelValues(getLabelsForDataImportCron(dictName, dsName)).Set(hasSupportedArchitectures)
}

func SetDICTWithNoSupportedArchitectures(dictName, dsName string) {
	dictWithSupportedArchitectures.WithLabelValues(getLabelsForDataImportCron(dictName, dsName)).Set(hasNoSupportedArchitectures)
}

func ResetDICTWithSupportedArchitectures() {
	dictWithSupportedArchitectures.Reset()
}
func IsDICTWithSupportedArchitectures(dictName, dsName string) (bool, error) {
	dto := &ioprometheusclient.Metric{}
	err := dictWithSupportedArchitectures.WithLabelValues(getLabelsForDataImportCron(dictName, dsName)).Write(dto)
	value := dto.Gauge.GetValue()

	if err != nil {
		return false, err
	}

	return value == hasSupportedArchitectures, nil
}

func SetDICTWithArchitectureAnnotation(dictName, dsName string) {
	dictWithArchitectureAnnotation.WithLabelValues(getLabelsForDataImportCron(dictName, dsName)).Set(hasArchitectureAnnotation)
}

func SetDICTWithNoArchitectureAnnotation(dictName, dsName string) {
	dictWithArchitectureAnnotation.WithLabelValues(getLabelsForDataImportCron(dictName, dsName)).Set(hasNoArchitectureAnnotation)
}

func ResetDICTWithArchitectureAnnotation() {
	dictWithArchitectureAnnotation.Reset()
}

func IsDICTWithArchitectureAnnotation(dictName, dsName string) (bool, error) {
	dto := &ioprometheusclient.Metric{}
	err := dictWithArchitectureAnnotation.WithLabelValues(getLabelsForDataImportCron(dictName, dsName)).Write(dto)
	value := dto.Gauge.GetValue()

	if err != nil {
		return false, err
	}

	return value == hasArchitectureAnnotation, nil
}

func getLabelsForObj(kind string, name string) string {
	return strings.ToLower(kind + "/" + name)
}

func getLabelsForUnsafeAnnotation(unsafeAnnotation string) string {
	return strings.ToLower(unsafeAnnotation)
}

func getLabelsForDataImportCron(dictName, dsName string) (string, string) {
	return strings.ToLower(dictName), strings.ToLower(dsName)
}
