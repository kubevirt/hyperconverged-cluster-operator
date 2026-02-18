package aie_webhook

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
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

	Context("ConfigMap deployment", func() {
		It("should not create if DeployAIEWebhook is false", func() {
			hco.Spec.FeatureGates.DeployAIEWebhook = ptr.To(false)
			cl = commontestutils.InitClient([]client.Object{hco})

			handler := NewAIEWebhookConfigMapHandler(cl, commontestutils.GetScheme())
			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Created).To(BeFalse())

			foundCMs := &corev1.ConfigMapList{}
			Expect(cl.List(context.Background(), foundCMs)).To(Succeed())
			Expect(foundCMs.Items).To(BeEmpty())
		})

		It("should create configmap when DeployAIEWebhook is true", func() {
			hco.Spec.FeatureGates.DeployAIEWebhook = ptr.To(true)
			cl = commontestutils.InitClient([]client.Object{hco})

			handler := NewAIEWebhookConfigMapHandler(cl, commontestutils.GetScheme())
			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Created).To(BeTrue())

			foundCMs := &corev1.ConfigMapList{}
			Expect(cl.List(context.Background(), foundCMs)).To(Succeed())
			Expect(foundCMs.Items).To(HaveLen(1))
			Expect(foundCMs.Items[0].Name).To(Equal("kubevirt-aie-launcher-config"))
			Expect(foundCMs.Items[0].Data).To(HaveKey("config.yaml"))
		})

		It("should delete configmap when DeployAIEWebhook is set to false", func() {
			hco.Spec.FeatureGates.DeployAIEWebhook = ptr.To(false)
			cm, err := newAIEWebhookConfigMap(hco)
			Expect(err).ToNot(HaveOccurred())
			cl = commontestutils.InitClient([]client.Object{hco, cm})

			handler := NewAIEWebhookConfigMapHandler(cl, commontestutils.GetScheme())
			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Deleted).To(BeTrue())

			foundCMs := &corev1.ConfigMapList{}
			Expect(cl.List(context.Background(), foundCMs)).To(Succeed())
			Expect(foundCMs.Items).To(BeEmpty())
		})
	})

	Context("ConfigMap content rendering", func() {
		It("should render empty rules when no AIEWebhookConfig is set", func() {
			yaml := renderConfigYAML(hco)
			Expect(yaml).To(Equal("rules:\n"))
		})

		It("should render rules from HCO spec", func() {
			hco.Spec.AIEWebhookConfig = &hcov1beta1.AIEWebhookConfiguration{
				Rules: []hcov1beta1.AIELauncherRule{
					{
						Name:  "gpu-rule",
						Image: "registry.example.com/aie-launcher:v1",
						Selector: hcov1beta1.AIESelector{
							DeviceNames: []string{"nvidia.com/A100", "nvidia.com/H100"},
						},
					},
					{
						Name:  "label-rule",
						Image: "registry.example.com/aie-launcher:v2",
						Selector: hcov1beta1.AIESelector{
							VMLabels: &hcov1beta1.AIEVMLabels{
								MatchLabels: map[string]string{
									"aie.kubevirt.io/launcher": "true",
								},
							},
						},
					},
				},
			}

			yaml := renderConfigYAML(hco)
			Expect(yaml).To(ContainSubstring("gpu-rule"))
			Expect(yaml).To(ContainSubstring("nvidia.com/A100"))
			Expect(yaml).To(ContainSubstring("nvidia.com/H100"))
			Expect(yaml).To(ContainSubstring("label-rule"))
			Expect(yaml).To(ContainSubstring("aie.kubevirt.io/launcher"))
		})
	})
})
