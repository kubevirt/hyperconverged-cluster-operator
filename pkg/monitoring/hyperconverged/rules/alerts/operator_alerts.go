package alerts

import (
	"fmt"

	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// withVMLabel wraps a PromQL expression with label_replace to add a "vm"
// typed resource label derived from the "name" label. This enables the
// monitoring-plugin to navigate from alerts to the VM resource page.
func withVMLabel(expr string) string {
	return fmt.Sprintf(`label_replace(%s, "vm", "$1", "name", "(.+)")`, expr)
}

const (
	outOfBandUpdateAlert             = "KubeVirtCRModified"
	unsafeModificationAlert          = "UnsupportedHCOModification"
	installationNotCompletedAlert    = "HCOInstallationIncomplete"
	MisconfiguredDeschedulerAlert    = "HCOMisconfiguredDescheduler"
	unsupportedArchitecturesAlert    = "HCOGoldenImageWithNoSupportedArchitecture"
	dictWithNoArchAnnotationAlert    = "HCOGoldenImageWithNoArchitectureAnnotation"
	multiArchBootImagesDisabledAlert = "HCOMultiArchGoldenImagesDisabled"
	csvAbnormalAlert                 = "HCOClusterServiceVersionAbnormal"
	componentPodCrashLoopingAlert    = "HCOComponentPodCrashLooping"

	severityAlertLabelKey      = "severity"
	healthImpactAlertLabelKey  = "operator_health_impact"
	ruleComponentAlertLabelKey = "openshift_io_alert_rule_component"
	ruleLayerAlertLabelKey     = "openshift_io_alert_rule_layer"

	virtualMachineComponentAlertLabelValue = "openshift-virtualization-virtual-machine"
	namespaceLayerAlertLabelValue          = "namespace"
)

// csvAbnormalPhases is the union of the abnormal CSV phases that OLM reports
// through the csv_abnormal metric. It mirrors the phases covered by the OLM
// platform alerts CsvAbnormalFailedOver2Min and CsvAbnormalOver30Min.
const csvAbnormalPhases = "Failed|Replacing|Pending|Deleting|Unknown"

func operatorAlerts(installNamespace string, isManagedByOLM bool) []promv1.Rule {
	rules := []promv1.Rule{
		{
			Alert: outOfBandUpdateAlert,
			Expr:  intstr.FromString("sum by(component_name) ((round(increase(kubevirt_hco_out_of_band_modifications_total[10m]))>0 and kubevirt_hco_out_of_band_modifications_total offset 10m) or (kubevirt_hco_out_of_band_modifications_total != 0 unless kubevirt_hco_out_of_band_modifications_total offset 10m))"),
			Annotations: map[string]string{
				"description": "Out-of-band modification for {{ $labels.component_name }}.",
				"summary":     "{{ $value }} out-of-band CR modifications were detected in the last 10 minutes.",
			},
			Labels: map[string]string{
				severityAlertLabelKey:     "warning",
				healthImpactAlertLabelKey: "warning",
			},
		},
		{
			Alert: unsafeModificationAlert,
			Expr:  intstr.FromString("sum by(annotation_name, namespace) ((kubevirt_hco_unsafe_modifications)>0)"),
			Annotations: map[string]string{
				"description": "unsafe modification for the {{ $labels.annotation_name }} annotation in the HyperConverged resource.",
				"summary":     "{{ $value }} unsafe modifications were detected in the HyperConverged resource.",
			},
			Labels: map[string]string{
				severityAlertLabelKey:     "info",
				healthImpactAlertLabelKey: "none",
			},
		},
		{
			Alert: installationNotCompletedAlert,
			Expr:  intstr.FromString("kubevirt_hco_hyperconverged_cr_exists == 0"),
			For:   new(promv1.Duration("1h")),
			Annotations: map[string]string{
				"description": "the installation was not completed; the HyperConverged custom resource is missing. In order to complete the installation of the Hyperconverged Cluster Operator you should create the HyperConverged custom resource.",
				"summary":     "the installation was not completed; to complete the installation, create a HyperConverged custom resource.",
			},
			Labels: map[string]string{
				severityAlertLabelKey:     "info",
				healthImpactAlertLabelKey: "critical",
			},
		},
		{
			Alert: MisconfiguredDeschedulerAlert,
			Expr:  intstr.FromString("kubevirt_hco_misconfigured_descheduler == 1"),
			Annotations: map[string]string{
				"description": "Kube Descheduler is not correctly configured for KubeVirt",
				"summary":     "Kube Descheduler is not correctly configured for KubeVirt",
			},
			Labels: map[string]string{
				severityAlertLabelKey:     "critical",
				healthImpactAlertLabelKey: "critical",
			},
		},
		{
			Alert: unsupportedArchitecturesAlert,
			Expr:  intstr.FromString("kubevirt_hco_dataimportcrontemplate_with_supported_architectures == 0"),
			Annotations: map[string]string{
				"description": "The cluster has no nodes with architecture that matches any of the architectures of the {{ $labels.data_import_cron_name }} DataImportCronTemplate (for the {{ $labels.managed_data_source_name }} DataSource). This golden image will not be available for use in the cluster.",
				"summary":     "DataImportCronTemplate with no supported architecture was detected in the HyperConverged resource.",
			},
			Labels: map[string]string{
				severityAlertLabelKey:     "warning",
				healthImpactAlertLabelKey: "none",
			},
		},
		{
			Alert: dictWithNoArchAnnotationAlert,
			Expr:  intstr.FromString("kubevirt_hco_dataimportcrontemplate_with_architecture_annotation == 0"),
			Annotations: map[string]string{
				"description": "The {{ $labels.data_import_cron_name }} DataImportCronTemplate (for the {{ $labels.managed_data_source_name }} DataSource) does not have the 'ssp.kubevirt.io/dict.architectures' annotation. Using this golden image for VM boot disk, may cause the VM to fail.",
				"summary":     "DataImportCronTemplate without 'ssp.kubevirt.io/dict.architectures' annotation was detected in the HyperConverged resource.",
			},
			Labels: map[string]string{
				severityAlertLabelKey:     "warning",
				healthImpactAlertLabelKey: "none",
			},
		},
		{
			Alert: multiArchBootImagesDisabledAlert,
			Expr:  intstr.FromString("kubevirt_hco_multi_arch_boot_images_enabled == 0"),
			Annotations: map[string]string{
				"description": "The cluster is multi-arch, but the enableMultiArchBootImageImport feature gate is disabled. This may cause issues with VM booting on nodes with different architectures.",
				"summary":     "Multi-arch boot images feature is disabled in a multi-arch cluster.",
			},
			Labels: map[string]string{
				severityAlertLabelKey:     "warning",
				healthImpactAlertLabelKey: "none",
			},
		},
		{
			Alert: "DeprecatedMachineType",
			Expr: intstr.FromString(withVMLabel(`
			  kubevirt_vm_info
			  * on(machine_type) group_left()
				max(kubevirt_node_deprecated_machine_types) by (machine_type)
			`)),
			For: new(promv1.Duration("5m")),
			Annotations: map[string]string{
				"summary":     "Virtual Machine '{{ $labels.name }}' in namespace '{{ $labels.namespace }}' is using a deprecated machine type.",
				"description": "Virtual Machine '{{ $labels.name }}' in namespace '{{ $labels.namespace }}' is using machine type '{{ $labels.machine_type }}', which is deprecated. Current status: '{{ $labels.status_group }}'.",
			},
			Labels: map[string]string{
				severityAlertLabelKey:      "warning",
				healthImpactAlertLabelKey:  "none",
				ruleComponentAlertLabelKey: virtualMachineComponentAlertLabelValue,
				ruleLayerAlertLabelKey:     namespaceLayerAlertLabelValue,
			},
		},
	}

	// The ClusterServiceVersion resource only exists on clusters managed by
	// OLM v0; when the operator is deployed by OLM v1 (ClusterExtension)
	// there is no CSV to watch.
	if isManagedByOLM {
		rules = append(rules, csvAbnormalAlertRule(installNamespace))
	}

	rules = append(rules, componentPodCrashLoopingAlertRule(installNamespace))

	return rules
}

// csvAbnormalAlertRule returns the CSV abnormal alert. The CSV resource, and
// the csv_abnormal metric OLM exposes for it, only exist on clusters managed
// by OLM v0; register the alert only on such clusters.
func csvAbnormalAlertRule(installNamespace string) promv1.Rule {
	return promv1.Rule{
		Alert: csvAbnormalAlert,
		// Select the current phase so the alert stops pending as soon as the
		// CSV recovers, instead of lagging behind a lookback window.
		Expr: intstr.FromString(fmt.Sprintf(
			`csv_abnormal{namespace=%q, phase=~%q} > 0`,
			installNamespace, csvAbnormalPhases)),
		For: new(promv1.Duration("30m")),
		Annotations: map[string]string{
			"summary":     "ClusterServiceVersion {{ $labels.name }} in namespace {{ $labels.namespace }} is in an abnormal phase for more than 30 minutes.",
			"description": "The ClusterServiceVersion {{ $labels.name }} version {{ $labels.version }} in namespace {{ $labels.namespace }} is in an abnormal phase ({{ $labels.phase }}: {{ $labels.reason }}) for more than 30 minutes. OLM cannot complete the installation or upgrade of this OpenShift Virtualization component, so the operator health is impacted.",
		},
		Labels: map[string]string{
			severityAlertLabelKey:     "warning",
			healthImpactAlertLabelKey: "critical",
		},
	}
}

// componentPodCrashLoopingAlertRule returns the component pod crash-looping
// alert. The max_over_time window keeps the expression true across the brief
// gaps where the container is running between crashes, so the pending
// duration is not reset while the pod keeps crash looping.
func componentPodCrashLoopingAlertRule(installNamespace string) promv1.Rule {
	return promv1.Rule{
		Alert: componentPodCrashLoopingAlert,
		Expr: intstr.FromString(fmt.Sprintf(
			`max_over_time(kube_pod_container_status_waiting_reason{reason="CrashLoopBackOff", namespace=%q}[5m]) >= 1`,
			installNamespace)),
		For: new(promv1.Duration("15m")),
		Annotations: map[string]string{
			"summary":     "Container {{ $labels.container }} in pod {{ $labels.pod }} in namespace {{ $labels.namespace }} has been crash looping for more than 15 minutes.",
			"description": "The container {{ $labels.container }} in pod {{ $labels.pod }} in namespace {{ $labels.namespace }}, which is part of the OpenShift Virtualization control plane, has been in CrashLoopBackOff for more than 15 minutes. The operator health is impacted while a component pod keeps restarting.",
		},
		Labels: map[string]string{
			severityAlertLabelKey:     "warning",
			healthImpactAlertLabelKey: "warning",
		},
	}
}
