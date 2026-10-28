package bearer_token_controller

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/alerts"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

var _ = Describe("NewServiceMonitor", func() {
	When("Running on plain Kubernetes", func() {
		It("should configures selector, endpoint, auth, and TLS", func() {
			owner := metav1.OwnerReference{Name: "o"}
			sm := newServiceMonitor(commontestutils.Namespace, owner, false)

			lbls := getLabels()
			delete(lbls, hcoutil.AppLabelManagedBy)
			delete(lbls, hcoutil.AppLabelVersion)

			Expect(sm.Spec.Selector.MatchLabels).To(Equal(lbls))
			Expect(sm.Spec.Endpoints).To(HaveLen(1))
			ep := sm.Spec.Endpoints[0]
			Expect(ep.Port).To(Equal(alerts.OperatorPortName))

			expectedScheme := monitoringv1.Scheme(new(monitoringv1.SchemeHTTPS).String())
			Expect(ep.Scheme).To(HaveValue(Equal(expectedScheme)))
			Expect(ep.TLSConfig).ToNot(BeNil())
			Expect(ep.TLSConfig.InsecureSkipVerify).To(HaveValue(BeTrue()))
			Expect(ep.Authorization).ToNot(BeNil())
			Expect(ep.Authorization.Credentials).ToNot(BeNil())
			Expect(ep.Authorization.Credentials.Name).To(Equal(secretName))
			Expect(ep.Authorization.Credentials.Key).To(Equal("token"))
			Expect(*ep.TLSConfig.InsecureSkipVerify).To(BeTrue())
			Expect(sm.Spec.ScrapeClassName).To(BeNil())
		})
	})

	When("Running on openshift", func() {
		It("should configures selector, endpoint, auth, and TLS", func() {
			owner := metav1.OwnerReference{Name: "o"}
			sm := newServiceMonitor(commontestutils.Namespace, owner, true)

			lbls := getLabels()
			delete(lbls, hcoutil.AppLabelManagedBy)
			delete(lbls, hcoutil.AppLabelVersion)

			Expect(sm.Spec.Selector.MatchLabels).To(Equal(lbls))
			Expect(sm.Spec.Endpoints).To(HaveLen(1))
			ep := sm.Spec.Endpoints[0]
			Expect(ep.Port).To(Equal(alerts.OperatorPortName))

			expectedScheme := monitoringv1.Scheme(new(monitoringv1.SchemeHTTPS).String())
			Expect(ep.Scheme).To(HaveValue(Equal(expectedScheme)))
			Expect(ep.Authorization).To(BeNil())
			Expect(ep.TLSConfig).ToNot(BeNil())
			Expect(ep.TLSConfig.InsecureSkipVerify).To(HaveValue(BeFalse()))
			Expect(ep.TLSConfig.SafeTLSConfig.ServerName).To(HaveValue(Equal(hcoutil.WebhookMetricServiceName + "." + commontestutils.Namespace + ".svc")))
			Expect(sm.Spec.ScrapeClassName).To(HaveValue(Equal(hcoutil.OpenShiftScrapeClassName)))
		})
	})
})
