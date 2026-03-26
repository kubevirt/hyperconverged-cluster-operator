package aie_webhook

import (
	"context"

	log "github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
)

var _ = Describe("AIE Webhook ConfigMap", func() {
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

	Context("newAIEWebhookConfigMap", func() {
		It("should create a ConfigMap with empty rules", func() {
			cm := newAIEWebhookConfigMap(hco)
			Expect(cm.Name).To(Equal("kubevirt-aie-launcher-config"))
			Expect(cm.Data).To(HaveKey("config.yaml"))
			Expect(cm.Data["config.yaml"]).To(Equal("rules:\n"))
		})
	})

	Context("ConfigMap deployment", func() {
		It("should not create if deploy-aie-webhook annotation is absent", func() {
			cl = commontestutils.InitClient([]client.Object{hco})

			handler, err := NewAIEWebhookConfigMapHandler(log.Logger{}, cl, commontestutils.GetScheme(), hco)
			Expect(err).ToNot(HaveOccurred())
			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Created).To(BeFalse())

			foundCMs := &corev1.ConfigMapList{}
			Expect(cl.List(context.Background(), foundCMs)).To(Succeed())
			Expect(foundCMs.Items).To(BeEmpty())
		})

		It("should create configmap when deploy-aie-webhook annotation is true", func() {
			hco.Annotations[DeployAIEWebhookAnnotation] = "true"
			cl = commontestutils.InitClient([]client.Object{hco})

			handler, err := NewAIEWebhookConfigMapHandler(log.Logger{}, cl, commontestutils.GetScheme(), hco)
			Expect(err).ToNot(HaveOccurred())
			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Created).To(BeTrue())

			foundCMs := &corev1.ConfigMapList{}
			Expect(cl.List(context.Background(), foundCMs)).To(Succeed())
			Expect(foundCMs.Items).To(HaveLen(1))
			Expect(foundCMs.Items[0].Name).To(Equal("kubevirt-aie-launcher-config"))
			Expect(foundCMs.Items[0].Data).To(HaveKeyWithValue("config.yaml", "rules:\n"))
		})

		It("should delete configmap when deploy-aie-webhook annotation is removed", func() {
			cm := newAIEWebhookConfigMap(hco)
			cl = commontestutils.InitClient([]client.Object{hco, cm})

			handler, err := NewAIEWebhookConfigMapHandler(log.Logger{}, cl, commontestutils.GetScheme(), hco)
			Expect(err).ToNot(HaveOccurred())
			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Deleted).To(BeTrue())

			foundCMs := &corev1.ConfigMapList{}
			Expect(cl.List(context.Background(), foundCMs)).To(Succeed())
			Expect(foundCMs.Items).To(BeEmpty())
		})
	})
})
