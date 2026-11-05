package rules

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/rhobs/operator-observability-toolkit/pkg/testutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	It("should include storage health alerts in the CNV PrometheusRule", func() {
		owner := metav1.OwnerReference{Name: "hco", UID: "hco-uid"}
		rule, err := BuildPrometheusRule("openshift-cnv", owner)
		Expect(err).NotTo(HaveOccurred())
		Expect(rule.Namespace).To(Equal("openshift-cnv"))
		Expect(rule.OwnerReferences).To(ConsistOf(owner))

		var storageAlertNames []string
		for _, group := range rule.Spec.Groups {
			for _, alert := range group.Rules {
				switch alert.Alert {
				case "MultipathPathDegraded", "MultipathPathCritical", "MultipathDeviceSuspended":
					storageAlertNames = append(storageAlertNames, alert.Alert)
					Expect(alert.Labels).To(HaveKeyWithValue("namespace", "openshift-monitoring"))
					Expect(alert.Labels).To(HaveKeyWithValue("operator_health_impact", "none"))
					Expect(alert.Labels).To(HaveKeyWithValue("kubernetes_operator_component", "cnv-observability"))
				}
			}
		}
		Expect(storageAlertNames).To(ConsistOf("MultipathPathDegraded", "MultipathPathCritical", "MultipathDeviceSuspended"))
	})
})
