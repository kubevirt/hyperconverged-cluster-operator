package alerts

import (
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const openshiftMonitoringNamespace = "openshift-monitoring"

// storageAlerts monitors DM-multipath devices reported by platform node-exporter.
// Based on the DM-multipath alerts in:
// https://github.com/raffaelespazzoli/openshift-monitoring-addson/blob/ac4eedcf0454121af010d8954cf85bd8a975c929/storage/alerts.yaml
func storageAlerts() []promv1.Rule {
	return []promv1.Rule{
		{
			Alert: "MultipathPathDegraded",
			Expr:  intstr.FromString(`node_dmmultipath_device_paths_failed{job="node-exporter"} > 0`),
			For:   new(promv1.Duration("5m")),
			Annotations: map[string]string{
				"description": "For 5 minutes, at least one path for DM-multipath device {{ $labels.device }} ({{ $labels.sysfs_name }}) on {{ $labels.instance }} has been outside the active state (SCSI running or NVMe live) at each evaluation. The failed paths can change between evaluations. Path redundancy is degraded, and I/O may be disrupted if no usable paths remain. Investigate cable connectivity, HBA health, and storage array port status.",
				"summary":     "Multipath device {{ $labels.device }} ({{ $labels.sysfs_name }}) on {{ $labels.instance }} has {{ $value }} failed path(s).",
			},
			Labels: map[string]string{
				"severity":               "warning",
				"namespace":              openshiftMonitoringNamespace,
				"operator_health_impact": "none",
			},
		},
		{
			Alert: "MultipathPathCritical",
			Expr: intstr.FromString(`(
  node_dmmultipath_device_paths_failed{job="node-exporter"}
  >= node_dmmultipath_device_paths{job="node-exporter"} / 2
)
and node_dmmultipath_device_paths_failed{job="node-exporter"} > 0`),
			For: new(promv1.Duration("2m")),
			Annotations: map[string]string{
				"description": "For 2 minutes, at least one path and at least half of the paths for DM-multipath device {{ $labels.device }} ({{ $labels.sysfs_name }}) on {{ $labels.instance }} have been outside the active state at each evaluation. The failed paths can change between evaluations. There are {{ $value }} failed path(s). The device is at risk of, or already experiencing, total path loss. Immediately check storage fabric, HBA ports, and array controller connectivity.",
				"summary":     "Multipath device {{ $labels.device }} ({{ $labels.sysfs_name }}) on {{ $labels.instance }} has at least half its paths failed.",
			},
			Labels: map[string]string{
				"severity":               "critical",
				"namespace":              openshiftMonitoringNamespace,
				"operator_health_impact": "none",
			},
		},
		{
			Alert: "MultipathDeviceSuspended",
			Expr:  intstr.FromString(`node_dmmultipath_device_active{job="node-exporter"} == 0`),
			For:   new(promv1.Duration("1m")),
			Annotations: map[string]string{
				"description": "DM-multipath device {{ $labels.device }} ({{ $labels.sysfs_name }}) on {{ $labels.instance }} has been suspended for 1 minute. I/O through this device is blocked, and workloads using it may hang. Check dmsetup info and dmsetup status, investigate why the device was suspended, and restore it after resolving the underlying issue.",
				"summary":     "Multipath device {{ $labels.device }} ({{ $labels.sysfs_name }}) on {{ $labels.instance }} is suspended.",
			},
			Labels: map[string]string{
				"severity":               "critical",
				"namespace":              openshiftMonitoringNamespace,
				"operator_health_impact": "none",
			},
		},
	}
}
