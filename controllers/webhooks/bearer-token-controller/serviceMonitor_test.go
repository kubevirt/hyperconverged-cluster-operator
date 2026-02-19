package bearer_token_controller

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/alerts"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
)

var _ = Describe("NewServiceMonitor", func() {
	It("should configures selector, endpoint, auth, and TLS", func() {
		owner := metav1.OwnerReference{Name: "o"}
		sm := newServiceMonitor(commontestutils.Namespace, owner)

		Expect(sm.Spec.Selector.MatchLabels).To(Equal(getLabels()))
		Expect(sm.Spec.Endpoints).To(HaveLen(1))
		ep := sm.Spec.Endpoints[0]
		Expect(ep.Port).To(Equal(alerts.OperatorPortName))

		expectedSchemeUpper := monitoringv1.SchemeHTTPS
		expectedScheme := monitoringv1.Scheme(expectedSchemeUpper.String())
		Expect(ep.Scheme).To(HaveValue(Equal(expectedScheme)))
		Expect(ep.Authorization).ToNot(BeNil())
		Expect(ep.Authorization.Credentials).ToNot(BeNil())
		Expect(ep.Authorization.Credentials.Name).To(Equal(secretName))
		Expect(ep.Authorization.Credentials.Key).To(Equal("token"))
		Expect(*ep.TLSConfig.InsecureSkipVerify).To(BeTrue())
	})
})
