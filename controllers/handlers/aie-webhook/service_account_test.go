package aie_webhook

import (
	"context"
	"maps"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

var _ = Describe("AIE Webhook Service Account", func() {
	var (
		hco *hcov1beta1.HyperConverged
		req *common.HcoRequest
		cl  client.Client
	)

	BeforeEach(func() {
		hco = commontestutils.NewHco()
		hco.Annotations = make(map[string]string)
		req = commontestutils.NewReq(hco)
	})

	Context("newAIEWebhookServiceAccount", func() {
		It("should have all default values", func() {
			sa := newAIEWebhookServiceAccount(hco)
			Expect(sa.Name).To(Equal("kubevirt-aie-webhook"))
			Expect(sa.Namespace).To(BeEquivalentTo(hco.Namespace))
			Expect(sa.Labels).To(HaveKeyWithValue(hcoutil.AppLabel, hcoutil.HyperConvergedName))
			Expect(sa.Labels).To(HaveKeyWithValue(hcoutil.AppLabelComponent, string(hcoutil.AppComponentAIEWebhook)))
		})
	})

	Context("AIE webhook service account deployment", func() {
		It("should not create if DeployAIEWebhook is false", func() {
			hco.Spec.FeatureGates.DeployAIEWebhook = ptr.To(false)
			cl = commontestutils.InitClient([]client.Object{hco})

			handler := NewAIEWebhookServiceAccountHandler(cl, commontestutils.GetScheme())
			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Created).To(BeFalse())
			Expect(res.Updated).To(BeFalse())
			Expect(res.Deleted).To(BeFalse())

			foundSAs := &corev1.ServiceAccountList{}
			Expect(cl.List(context.Background(), foundSAs)).To(Succeed())
			Expect(foundSAs.Items).To(BeEmpty())
		})

		It("should delete service account when DeployAIEWebhook is set to false", func() {
			hco.Spec.FeatureGates.DeployAIEWebhook = ptr.To(false)
			sa := newAIEWebhookServiceAccount(hco)
			cl = commontestutils.InitClient([]client.Object{hco, sa})

			handler := NewAIEWebhookServiceAccountHandler(cl, commontestutils.GetScheme())
			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Name).To(Equal(sa.Name))
			Expect(res.Created).To(BeFalse())
			Expect(res.Updated).To(BeFalse())
			Expect(res.Deleted).To(BeTrue())

			foundSAs := &corev1.ServiceAccountList{}
			Expect(cl.List(context.Background(), foundSAs)).To(Succeed())
			Expect(foundSAs.Items).To(BeEmpty())
		})

		It("should create service account when DeployAIEWebhook is true", func() {
			hco.Spec.FeatureGates.DeployAIEWebhook = ptr.To(true)
			cl = commontestutils.InitClient([]client.Object{hco})

			handler := NewAIEWebhookServiceAccountHandler(cl, commontestutils.GetScheme())
			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Name).To(Equal("kubevirt-aie-webhook"))
			Expect(res.Created).To(BeTrue())
			Expect(res.Updated).To(BeFalse())
			Expect(res.Deleted).To(BeFalse())

			foundSAs := &corev1.ServiceAccountList{}
			Expect(cl.List(context.Background(), foundSAs)).To(Succeed())
			Expect(foundSAs.Items).To(HaveLen(1))
			Expect(foundSAs.Items[0].Name).To(Equal("kubevirt-aie-webhook"))
		})
	})

	Context("AIE webhook service account update", func() {
		It("should reconcile labels if they are missing while preserving user labels", func() {
			hco.Spec.FeatureGates.DeployAIEWebhook = ptr.To(true)
			sa := newAIEWebhookServiceAccount(hco)
			expectedLabels := maps.Clone(sa.Labels)
			delete(sa.Labels, "app.kubernetes.io/component")
			sa.Labels["user-added-label"] = "user-value"
			cl = commontestutils.InitClient([]client.Object{hco, sa})
			handler := NewAIEWebhookServiceAccountHandler(cl, commontestutils.GetScheme())

			res := handler.Ensure(req)
			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Created).To(BeFalse())
			Expect(res.Updated).To(BeTrue())
			Expect(res.Deleted).To(BeFalse())

			foundSA := &corev1.ServiceAccount{}
			Expect(cl.Get(context.Background(), client.ObjectKey{Name: "kubevirt-aie-webhook", Namespace: hco.Namespace}, foundSA)).To(Succeed())

			for key, value := range expectedLabels {
				Expect(foundSA.Labels).To(HaveKeyWithValue(key, value))
			}
			Expect(foundSA.Labels).To(HaveKeyWithValue("user-added-label", "user-value"))
		})
	})
})
