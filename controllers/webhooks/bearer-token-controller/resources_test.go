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

var _ = Describe("Resource builders", func() {
	Describe("NewMetricsService", func() {
		AfterEach(func() { _ = os.Unsetenv(alerts.OperatorNameEnv) })

		It("uses default selector name when env var is not set", func() {
			owner := metav1.OwnerReference{Name: "o"}
			svc := NewMetricsService(commontestutils.Namespace, owner)
			Expect(svc.Spec.Selector).To(HaveKeyWithValue("name", hcoutil.HCOWebhookName))
			Expect(svc.Spec.Ports).To(HaveLen(1))
			Expect(svc.Spec.Ports[0].Port).To(Equal(hcoutil.MetricsPort))
			Expect(svc.Spec.Ports[0].Name).To(Equal(alerts.OperatorPortName))
			Expect(svc.Spec.Ports[0].Protocol).To(Equal(corev1.ProtocolTCP))
		})
	})

	It("newServiceMonitor configures selector, endpoint, auth, and TLS", func() {
		owner := metav1.OwnerReference{Name: "o"}
		sm := newServiceMonitor(commontestutils.Namespace, owner)

		Expect(sm.Spec.Selector.MatchLabels).To(Equal(getLabels()))
		Expect(sm.Spec.Endpoints).To(HaveLen(1))
		ep := sm.Spec.Endpoints[0]
		Expect(ep.Port).To(Equal(alerts.OperatorPortName))
		Expect(ep.Scheme).To(Equal("https"))
		Expect(ep.Authorization).ToNot(BeNil())
		Expect(ep.Authorization.Credentials).ToNot(BeNil())
		Expect(ep.Authorization.Credentials.Name).To(Equal(secretName))
		Expect(ep.Authorization.Credentials.Key).To(Equal("token"))
		Expect(*ep.TLSConfig.InsecureSkipVerify).To(BeTrue())
	})

	It("newSecret builds the expected Secret object", func() {
		owner := metav1.OwnerReference{Name: "o"}
		sec := newSecret(commontestutils.Namespace, owner, "token content")
		Expect(sec.Name).To(Equal(secretName))
		Expect(sec.Namespace).To(Equal(commontestutils.Namespace))
		Expect(sec.Labels).To(Equal(getLabels()))
		Expect(sec.StringData).To(HaveKeyWithValue("token", "token content"))
		Expect(sec.OwnerReferences).To(ContainElement(owner))
	})
})
