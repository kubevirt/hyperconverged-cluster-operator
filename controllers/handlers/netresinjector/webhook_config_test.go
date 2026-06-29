package netresinjector

import (
	"context"
	"maps"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

var _ = Describe("Network Resources Injector MutatingWebhookConfiguration", func() {
	var (
		hco *hcov1.HyperConverged
		req *common.HcoRequest
		cl  client.Client
	)

	BeforeEach(func() {
		hco = commontestutils.NewHco()
		req = commontestutils.NewReq(hco)
	})

	Context("newMutatingWebhookConfiguration", func() {
		It("should have all default values", func() {
			mwc := newMutatingWebhookConfiguration()

			Expect(mwc.Name).To(Equal(webhookConfigName))
			Expect(mwc.Labels).To(HaveKeyWithValue(hcoutil.AppLabel, hcoutil.HyperConvergedName))
			Expect(mwc.Labels).To(HaveKeyWithValue(hcoutil.AppLabelComponent, string(hcoutil.AppComponentNetResInjector)))
			Expect(mwc.Annotations).ToNot(BeEmpty())

			Expect(mwc.Webhooks).To(HaveLen(1))
			wh := mwc.Webhooks[0]
			Expect(wh.Name).To(Equal("cnv-network-resources-injector-config.k8s.io"))
			Expect(wh.AdmissionReviewVersions).To(ConsistOf("v1", "v1beta1"))
			Expect(wh.FailurePolicy).ToNot(BeNil())
			Expect(*wh.FailurePolicy).To(Equal(admissionregistrationv1.Fail))
			Expect(wh.Rules).To(HaveLen(1))
			Expect(wh.Rules[0].Operations).To(ContainElement(admissionregistrationv1.Create))
			Expect(wh.Rules[0].Rule.Resources).To(ContainElement("pods"))
			Expect(wh.MatchConditions).To(HaveLen(2))
			Expect(wh.ClientConfig.Service).ToNot(BeNil())
			Expect(wh.ClientConfig.Service.Name).To(Equal(serviceName))
		})
	})

	Context("MutatingWebhookConfiguration handler", func() {
		It("should create MutatingWebhookConfiguration if it does not exist", func() {
			cl = commontestutils.InitClient([]client.Object{hco})

			handler := NewMutatingWebhookConfigurationHandler(cl, commontestutils.GetScheme())
			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Created).To(BeTrue())

			foundMWCs := &admissionregistrationv1.MutatingWebhookConfigurationList{}
			Expect(cl.List(context.Background(), foundMWCs)).To(Succeed())
			Expect(foundMWCs.Items).To(HaveLen(1))
			Expect(foundMWCs.Items[0].Name).To(Equal(webhookConfigName))
		})
	})

	Context("MutatingWebhookConfiguration update", func() {
		It("should update when webhooks are modified externally", func() {
			mwc := newMutatingWebhookConfiguration()
			mwc.Webhooks[0].AdmissionReviewVersions = []string{"v1"}
			cl = commontestutils.InitClient([]client.Object{hco, mwc})

			handler := NewMutatingWebhookConfigurationHandler(cl, commontestutils.GetScheme())
			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Updated).To(BeTrue())

			foundMWC := &admissionregistrationv1.MutatingWebhookConfiguration{}
			Expect(cl.Get(context.Background(), client.ObjectKey{Name: webhookConfigName}, foundMWC)).To(Succeed())
			Expect(foundMWC.Webhooks[0].AdmissionReviewVersions).To(ConsistOf("v1", "v1beta1"))
		})

		It("should not update when nothing changed", func() {
			mwc := newMutatingWebhookConfiguration()
			cl = commontestutils.InitClient([]client.Object{hco, mwc})

			handler := NewMutatingWebhookConfigurationHandler(cl, commontestutils.GetScheme())
			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Updated).To(BeFalse())
		})

		It("should reconcile labels if they are missing while preserving user labels", func() {
			mwc := newMutatingWebhookConfiguration()
			expectedLabels := maps.Clone(mwc.Labels)
			delete(mwc.Labels, hcoutil.AppLabelComponent)
			mwc.Labels["user-added-label"] = "user-value"
			cl = commontestutils.InitClient([]client.Object{hco, mwc})

			handler := NewMutatingWebhookConfigurationHandler(cl, commontestutils.GetScheme())
			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Updated).To(BeTrue())

			foundMWC := &admissionregistrationv1.MutatingWebhookConfiguration{}
			Expect(cl.Get(context.Background(), client.ObjectKey{Name: webhookConfigName}, foundMWC)).To(Succeed())

			for key, value := range expectedLabels {
				Expect(foundMWC.Labels).To(HaveKeyWithValue(key, value))
			}
			Expect(foundMWC.Labels).To(HaveKeyWithValue("user-added-label", "user-value"))
		})
	})

	Context("preserveExistingCABundles", func() {
		It("should copy CA bundle from existing webhook matched by name", func() {
			required := newMutatingWebhookConfiguration()
			existing := required.DeepCopy()
			existing.Webhooks[0].ClientConfig.CABundle = []byte("test-ca-bundle")
			required.Webhooks[0].ClientConfig.CABundle = nil

			preserveExistingCABundles(required, existing)

			Expect(required.Webhooks[0].ClientConfig.CABundle).To(Equal([]byte("test-ca-bundle")))
		})

		It("should not overwrite existing CA bundle in required", func() {
			required := newMutatingWebhookConfiguration()
			existing := required.DeepCopy()
			required.Webhooks[0].ClientConfig.CABundle = []byte("required-ca")
			existing.Webhooks[0].ClientConfig.CABundle = []byte("existing-ca")

			preserveExistingCABundles(required, existing)

			Expect(required.Webhooks[0].ClientConfig.CABundle).To(Equal([]byte("required-ca")))
		})

		It("should handle no matching webhook name", func() {
			required := newMutatingWebhookConfiguration()
			existing := required.DeepCopy()
			existing.Webhooks[0].Name = "different-name"
			existing.Webhooks[0].ClientConfig.CABundle = []byte("test-ca-bundle")
			required.Webhooks[0].ClientConfig.CABundle = nil

			preserveExistingCABundles(required, existing)

			Expect(required.Webhooks[0].ClientConfig.CABundle).To(BeNil())
		})
	})

	Context("hasRequiredAnnotations", func() {
		It("should return true when all required annotations are present", func() {
			existing := map[string]string{"key1": "val1", "key2": "val2", "extra": "val"}
			required := map[string]string{"key1": "val1", "key2": "val2"}
			Expect(hasRequiredAnnotations(existing, required)).To(BeTrue())
		})

		It("should return false when a required annotation is missing", func() {
			existing := map[string]string{"key1": "val1"}
			required := map[string]string{"key1": "val1", "key2": "val2"}
			Expect(hasRequiredAnnotations(existing, required)).To(BeFalse())
		})

		It("should return false when a required annotation has wrong value", func() {
			existing := map[string]string{"key1": "wrong"}
			required := map[string]string{"key1": "val1"}
			Expect(hasRequiredAnnotations(existing, required)).To(BeFalse())
		})
	})
})
