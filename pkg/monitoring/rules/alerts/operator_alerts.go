package alerts

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	outOfBandUpdateAlert              = "KubeVirtCRModified"
	unsafeModificationAlert           = "UnsupportedHCOModification"
	installationNotCompletedAlert     = "HCOInstallationIncomplete"
	singleStackIPv6Alert              = "SingleStackIPv6Unsupported"
	MisconfiguredDeschedulerAlert     = "HCOMisconfiguredDescheduler"
	VMOutdatedMachineTypeAlert        = "VMHasOutdatedMachineType"
	minSupportedVirtLauncherOSVersion = 8
	severityAlertLabelKey             = "severity"
	healthImpactAlertLabelKey         = "operator_health_impact"
)

func operatorAlerts() []promv1.Rule {
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
			For:   ptr.To(promv1.Duration("1h")),
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
			Alert: singleStackIPv6Alert,
			Expr:  intstr.FromString("kubevirt_hco_single_stack_ipv6 == 1"),
			Annotations: map[string]string{
				"description": "KubeVirt Hyperconverged is not supported on a single stack IPv6 cluster",
				"summary":     "KubeVirt Hyperconverged is not supported on a single stack IPv6 cluster",
			},
			Labels: map[string]string{
				severityAlertLabelKey:     "critical",
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
	}

	if rule, created := createVMOutdatedMachineTypeRule(); created {
		rules = append(rules, rule)
	}

	return rules
}

func createVMOutdatedMachineTypeRule() (promv1.Rule, bool) {
	logger := logf.Log.WithName("operator-alerts")
	rhelVersion, exists := os.LookupEnv("VIRT_LAUNCHER_OS_VERSION")
	if !exists {
		return promv1.Rule{}, false
	}

	virtLauncherOSVersion, err := strconv.Atoi(rhelVersion)
	if err != nil {
		logger.Error(err, "Error parsing VIRT_LAUNCHER_OS_VERSION")
		return promv1.Rule{}, false
	}

	if virtLauncherOSVersion > minSupportedVirtLauncherOSVersion {
		rule := promv1.Rule{
			Alert: VMOutdatedMachineTypeAlert,
			Expr:  intstr.FromString(getMachineTypeVersionExpr(minSupportedVirtLauncherOSVersion, virtLauncherOSVersion)),
			Annotations: map[string]string{
				"description": "There are virtual machines using an outdated machine type that need to be patched.",
				"summary":     "{{ $value }} virtual machines are using an outdated machine type.",
			},
			Labels: map[string]string{
				severityAlertLabelKey:     "warning",
				healthImpactAlertLabelKey: "none",
			},
		}
		return rule, true
	}

	return promv1.Rule{}, false
}

func getMachineTypeVersionExpr(mn, mx int) string {
	var versions []string
	for v := mn; v < mx; v++ {
		versions = append(versions, fmt.Sprintf(".*rhel%d.*", v))
	}
	vers := strings.Join(versions, "|")
	return fmt.Sprintf(`count(kubevirt_vmi_info{guest_os_machine=~%q} and on(name, namespace) kubevirt_vm_info{status=~"Running|Stopped"}) > 0`, vers)
}
