package bearer_token_controller

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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
		Expect(ep.Scheme).To(Equal("https"))
		Expect(ep.Authorization).ToNot(BeNil())
		Expect(ep.Authorization.Credentials).ToNot(BeNil())
		Expect(ep.Authorization.Credentials.Name).To(Equal(secretName))
		Expect(ep.Authorization.Credentials.Key).To(Equal("token"))
		Expect(*ep.TLSConfig.InsecureSkipVerify).To(BeTrue())
	})
})
