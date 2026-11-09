package rules

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/rhobs/operator-observability-toolkit/pkg/testutil"
)

func TestRules(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Rules Suite")
}

var _ = Describe("Rules Validation", func() {
	var linter *testutil.Linter

	BeforeEach(func() {
		Expect(SetupRules()).To(Succeed())
		linter = testutil.New()
	})

	It("Should validate alerts", func() {
		linter.AddCustomAlertValidations(
			testutil.ValidateAlertNameLength,
			testutil.ValidateAlertRunbookURLAnnotation,
			testutil.ValidateAlertHealthImpactLabel,
			testutil.ValidateAlertPartOfAndComponentLabels)

		alerts := ListAlerts()
		problems := linter.LintAlerts(alerts)
		Expect(problems).To(BeEmpty())
	})

	It("Should classify DeprecatedMachineType as a namespace-layer virtual machine alert", func() {
		alerts := ListAlerts()
		Expect(alerts).NotTo(BeEmpty())
		foundDeprecatedMachineType := false
		for _, alert := range alerts {
			Expect(alert.Labels).To(HaveKeyWithValue("kubernetes_operator_part_of", "kubevirt"))
			Expect(alert.Labels).To(HaveKeyWithValue("kubernetes_operator_component", "hyperconverged-cluster-operator"))
			if alert.Alert == "DeprecatedMachineType" {
				foundDeprecatedMachineType = true
				Expect(alert.Labels).To(HaveKeyWithValue("openshift_io_alert_rule_component", "openshift-virtualization-virtual-machine"))
				Expect(alert.Labels).To(HaveKeyWithValue("openshift_io_alert_rule_layer", "namespace"))
				Expect(alert.Labels).To(HaveKeyWithValue("operator_health_impact", "none"))
				Expect(alert.Labels).To(HaveKeyWithValue("severity", "warning"))
			} else {
				Expect(alert.Labels).NotTo(HaveKey("openshift_io_alert_rule_component"))
				Expect(alert.Labels).NotTo(HaveKey("openshift_io_alert_rule_layer"))
			}
		}
		Expect(foundDeprecatedMachineType).To(BeTrue())
	})

	It("Should validate recording rules", func() {
		recordingRules := ListRecordingRules()
		problems := linter.LintRecordingRules(recordingRules)
		Expect(problems).To(BeEmpty())
	})
})
