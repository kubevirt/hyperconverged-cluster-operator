package bearer_token_controller

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/alerts"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

var _ = Describe("NewMetricsService", func() {
	It("uses default selector name when env var is not set", func() {
		owner := metav1.OwnerReference{Name: "o"}
		svc := NewMetricsService(commontestutils.Namespace, owner)
		Expect(svc.Spec.Selector).To(HaveKeyWithValue("name", hcoutil.HCOWebhookName))
		Expect(svc.Spec.Ports).To(HaveLen(1))
		Expect(svc.Spec.Ports[0].Port).To(Equal(hcoutil.MetricsPort))
		Expect(svc.Spec.Ports[0].Name).To(Equal(alerts.OperatorPortName))
		Expect(svc.Spec.Ports[0].Protocol).To(Equal(corev1.ProtocolTCP))
	})

	It("uses default selector name from the env var if it set", func() {
		varVal, varExists := os.LookupEnv(alerts.OperatorNameEnv)
		DeferCleanup(func() {
			if varExists {
				Expect(os.Setenv(alerts.OperatorNameEnv, varVal)).To(Succeed())
			} else {
				Expect(os.Unsetenv(alerts.OperatorNameEnv)).To(Succeed())
			}
		})

		const customWebhookName = "custom-webhook-name"
		Expect(os.Setenv(alerts.OperatorNameEnv, customWebhookName)).To(Succeed())

		owner := metav1.OwnerReference{Name: "o"}
		svc := NewMetricsService(commontestutils.Namespace, owner)
		Expect(svc.Spec.Selector).To(HaveKeyWithValue("name", customWebhookName))
		Expect(svc.Spec.Ports).To(HaveLen(1))
		Expect(svc.Spec.Ports[0].Port).To(Equal(hcoutil.MetricsPort))
		Expect(svc.Spec.Ports[0].Name).To(Equal(alerts.OperatorPortName))
		Expect(svc.Spec.Ports[0].Protocol).To(Equal(corev1.ProtocolTCP))
	})
})
