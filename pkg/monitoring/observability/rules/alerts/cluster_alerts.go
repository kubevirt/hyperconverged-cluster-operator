package alerts

import (
	"fmt"
	"strings"

	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

// Network interface flag bitmasks for Linux interface flags
// See: https://github.com/torvalds/linux/blob/master/include/uapi/linux/if.h
const (
	IFF_UP       = 1
	IFF_RUNNING  = 64
	IFF_LOWER_UP = 65536
)

var ignoredInterfacesForNetworkDown = []string{
	"lo",          // loopback interface
	"tunbr",       // tunnel bridge
	"veth.+",      // virtual ethernet devices
	"ovs-system",  // OVS internal system interface
	"genev_sys.+", // OVN Geneve overlay/encapsulation interfaces
	"br-int",      // OVN integration bridge
}

func clusterAlerts() []promv1.Rule {
	return []promv1.Rule{
		{
			Alert: "HighCPUWorkload",
			Expr:  intstr.FromString("instance:node_cpu_utilisation:rate1m >= 0.9"),
			For:   ptr.To[promv1.Duration]("5m"),
			Annotations: map[string]string{
				"summary":     "High CPU usage on host {{ $labels.instance }}",
				"description": "CPU utilization for {{ $labels.instance }} has been above 90% for more than 5 minutes.",
			},
			Labels: map[string]string{
				"severity":               "warning",
				"operator_health_impact": "none",
			},
		},
		{
			Alert: "HAControlPlaneDown",
			Expr:  intstr.FromString("kube_node_role{role='control-plane'} * on(node) kube_node_status_condition{condition='Ready',status='true'} == 0"),
			For:   ptr.To[promv1.Duration]("5m"),
			Annotations: map[string]string{
				"summary":     "Control plane node {{ $labels.node }} is not ready",
				"description": "Control plane node {{ $labels.node }} has been not ready for more than 5 minutes.",
			},
			Labels: map[string]string{
				"severity":               "critical",
				"operator_health_impact": "none",
			},
		},
		{
			Alert: "NodeNetworkInterfaceDown",
			Expr: intstr.FromString(fmt.Sprintf(`count by (instance) (
					(node_network_flags %% %d) >= %d											# IFF_UP is set
					and
					(node_network_flags %% %d) < %d											# IFF_RUNNING is NOT set
					and
					(node_network_flags %% %d) < %d										    # IFF_LOWER_UP is NOT set
					and
					on(device) node_network_up == 1											    # Interface is up
					and
					on(device) (node_network_flags unless node_network_flags{device=~"%s"})     # Excluding ignored interfaces
				) > 0`, (IFF_UP << 1), IFF_UP, (IFF_RUNNING << 1), IFF_RUNNING, (IFF_LOWER_UP << 1), IFF_LOWER_UP, strings.Join(ignoredInterfacesForNetworkDown, "|"))),
			For: ptr.To[promv1.Duration]("5m"),
			Annotations: map[string]string{
				"summary":     "Network interfaces are down",
				"description": "{{ $value }} network devices have been down on instance {{ $labels.instance }} for more than 5 minutes.",
			},
			Labels: map[string]string{
				"severity":               "warning",
				"operator_health_impact": "none",
			},
		},
		{
			Alert: "PersistentVolumeFillingUp",
			Expr: intstr.FromString(`
				(
					kubelet_volume_stats_available_bytes{job="kubelet",metrics_path="/metrics"}
					/
					kubelet_volume_stats_capacity_bytes{job="kubelet",metrics_path="/metrics"}
				) < 0.10
				and kubelet_volume_stats_used_bytes{job="kubelet",metrics_path="/metrics"} > 0
				and predict_linear(kubelet_volume_stats_available_bytes{job="kubelet",metrics_path="/metrics"}[6h], 4 * 24 * 3600) < 0
				unless on (cluster, namespace, persistentvolumeclaim) kube_persistentvolumeclaim_access_mode{access_mode="ReadOnlyMany"} == 1
			`),
			For: ptr.To[promv1.Duration]("5m"),
			Annotations: map[string]string{
				"summary":     "PersistentVolume is filling up",
				"description": "Based on recent sampling, the PersistentVolume claimed by {{ $labels.persistentvolumeclaim }} in Namespace {{ $labels.namespace }} is expected to fill up within four days. Currently {{ $value | humanizePercentage }} is available.",
			},
			Labels: map[string]string{
				"severity":               "warning",
				"operator_health_impact": "none",
			},
		},
		{
			Alert: "HighNodeCPUFrequency",
			Expr: intstr.FromString(`
				node_cpu_frequency_hertz > 0
				and on(instance, cpu)
				node_cpu_frequency_hertz - on(instance, cpu) group_left() node_cpu_frequency_max_hertz * 0.8 > 0
			`),
			For: ptr.To[promv1.Duration]("5m"),
			Annotations: map[string]string{
				"summary":     "High CPU frequency detected on node {{ $labels.instance }}",
				"description": "CPU frequency on node {{ $labels.instance }} (CPU {{ $labels.cpu }}) is {{ $value | humanize }}Hz, which is above 80% of the maximum frequency. This may indicate high CPU utilization or thermal throttling.",
			},
			Labels: map[string]string{
				"severity":               "warning",
				"operator_health_impact": "none",
			},
		},
		{
			Alert: "DuplicateWaspAgentDSDetected",
			Expr: intstr.FromString(
				`count(kube_daemonset_metadata_generation{namespace="wasp",daemonset="wasp-agent"}) > 0
					and kubevirt_hco_memory_overcommit_percentage > 100
			`),
			For: ptr.To[promv1.Duration]("1m"),
			Annotations: map[string]string{
				"summary":     "Duplicate wasp-agent deployment detected",
				"description": "Two wasp-agent deployments exist in the cluster. Please follow the instructions mentioned in the runbook to remove the duplicate deployment.",
				"runbook_url": "https://github.com/openshift/runbooks/blob/master/alerts/openshift-virtualization-operator/DuplicateWaspAgentDSDetected.md",
			},
			Labels: map[string]string{
				"severity":               "warning",
				"operator_health_impact": "none",
			},
		},
		{
			Alert: "DeprecatedMachineType",
			Expr: intstr.FromString(`
			  kubevirt_vm_info
			  * on(machine_type) group_left()
				max(kubevirt_node_deprecated_machine_types) by (machine_type)
			`),
			For: ptr.To(promv1.Duration("5m")),
			Annotations: map[string]string{
				"summary":     "Virtual Machine '{{ $labels.name }}' in namespace '{{ $labels.namespace }}' is using a deprecated machine type.",
				"description": "Virtual Machine '{{ $labels.name }}' in namespace '{{ $labels.namespace }}' is using machine type '{{ $labels.machine_type }}', which is deprecated. Current status: '{{ $labels.status_group }}'.",
			},
			Labels: map[string]string{
				"severity":               "warning",
				"operator_health_impact": "none",
			},
		},
	}
}
