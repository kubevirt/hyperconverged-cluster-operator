package validator

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	hcofg "github.com/kubevirt/hyperconverged-cluster-operator/api/v1/featuregates"
	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

var _ = Describe("NVIDIA AIE admission", func() {
	type validationCase struct {
		image             string
		gate              *hcofg.State
		legacy            bool
		update            bool
		previouslyEnabled bool
		allowed           bool
	}

	for _, apiVersion := range []string{"v1", "v1beta1"} {
		Context(apiVersion, func() {
			DescribeTable("checks the launcher image before enabling NVIDIA AIE", func(ctx context.Context, test validationCase) {
				GinkgoT().Setenv(hcoutil.AIELauncherImageEnvV, test.image)
				requested := commontestutils.NewHco()
				previous := requested.DeepCopy()
				requested.Annotations = map[string]string{"test.example/update": "true"}
				if test.previouslyEnabled {
					previous.Spec.FeatureGates.Enable(hcofg.NVIDIAAIE)
				}
				if test.gate != nil {
					requested.Spec.FeatureGates = hcofg.HyperConvergedFeatureGates{{Name: hcofg.NVIDIAAIE, State: test.gate}}
				}
				if test.legacy {
					requested.Annotations = map[string]string{"hco.kubevirt.io/deployAIE": "true"}
					previous.Annotations = map[string]string{"hco.kubevirt.io/deployAIE": "true"}
				}
				cli := commontestutils.InitClient(nil)
				scheme := cli.Scheme()
				codec := serializer.NewCodecFactory(scheme).LegacyCodec(hcov1.SchemeGroupVersion, hcov1beta1.SchemeGroupVersion)
				var object, oldObject runtime.Object = requested, previous
				var handler admission.Handler = NewWebhookHandler(GinkgoLogr, cli, admission.NewDecoder(scheme), requested.Namespace, false)
				if apiVersion == "v1beta1" {
					converted := &hcov1beta1.HyperConverged{}
					oldConverted := &hcov1beta1.HyperConverged{}
					Expect(converted.ConvertFrom(requested)).To(Succeed())
					Expect(oldConverted.ConvertFrom(previous)).To(Succeed())
					object, oldObject = converted, oldConverted
					handler = NewWebhookV1Beta1Handler(GinkgoLogr, cli, admission.NewDecoder(scheme), requested.Namespace, false)
				}
				operation := admissionv1.Create
				if test.update {
					operation = admissionv1.Update
				}
				req := newRequest(operation, object, codec, true)
				if test.update {
					encoded, err := runtime.Encode(codec, oldObject)
					Expect(err).NotTo(HaveOccurred())
					req.OldObject = runtime.RawExtension{Raw: encoded}
				}
				response := handler.Handle(ctx, req)
				Expect(response.Allowed).To(Equal(test.allowed), "%+v", response.Result)
				if !test.allowed {
					Expect(response.Result.Message).To(ContainSubstring("does not provide an AIE launcher image"))
				}
				if test.legacy {
					Expect(response.Warnings).To(BeEmpty())
				}
			},
				Entry("allows disabled installations without an image", validationCase{allowed: true}),
				Entry("rejects feature-gate enablement without an image", validationCase{gate: new(hcofg.Enabled)}),
				Entry("ignores the retired annotation on create", validationCase{legacy: true, allowed: true}),
				Entry("allows updates with a leftover annotation", validationCase{legacy: true, update: true, allowed: true}),
				Entry("rejects feature-gate enablement despite a leftover annotation", validationCase{legacy: true, gate: new(hcofg.Enabled), update: true}),
				Entry("rejects enablement with a whitespace-only image", validationCase{gate: new(hcofg.Enabled), image: " \t "}),
				Entry("allows feature-gate enablement with an image", validationCase{gate: new(hcofg.Enabled), image: "registry.example/launcher:test", allowed: true}),
				Entry("allows an explicit Disabled gate with a leftover annotation", validationCase{gate: new(hcofg.Disabled), legacy: true, allowed: true}),
				Entry("rejects enabling on update without an image", validationCase{gate: new(hcofg.Enabled), update: true}),
				Entry("allows other updates to an already-enabled installation", validationCase{gate: new(hcofg.Enabled), update: true, previouslyEnabled: true, allowed: true}),
				Entry("allows disabling after an image disappears", validationCase{gate: new(hcofg.Disabled), update: true, previouslyEnabled: true, allowed: true}),
			)
		})
	}
})
