package validator

import (
	"context"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	networkaddonsv1 "github.com/kubevirt/cluster-network-addons-operator/pkg/apis/networkaddonsoperator/v1"
	kubevirtcorev1 "kubevirt.io/api/core/v1"
	cdiv1beta1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	sspv1beta3 "kubevirt.io/ssp-operator/api/v1beta3"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

var _ = Describe("v1 webhooks validator", func() {
	s := scheme.Scheme
	for _, f := range []func(*runtime.Scheme) error{
		hcov1.AddToScheme,
		hcov1beta1.AddToScheme,
		cdiv1beta1.AddToScheme,
		kubevirtcorev1.AddToScheme,
		networkaddonsv1.AddToScheme,
		sspv1beta3.AddToScheme,
	} {
		Expect(f(s)).To(Succeed())
	}

	codecFactory := serializer.NewCodecFactory(s)
	hcoCodec := codecFactory.LegacyCodec(hcov1.SchemeGroupVersion, hcov1beta1.SchemeGroupVersion)

	cli := fake.NewClientBuilder().WithScheme(s).Build()
	decoder := admission.NewDecoder(s)

	v1beta1WH := NewWebhookV1Beta1Handler(GinkgoLogr, cli, decoder, HcoValidNamespace, true)
	wh := NewWebhookHandler(GinkgoLogr, cli, decoder, HcoValidNamespace, true, v1beta1WH)

	Context("Check create validation webhook", func() {
		var cr *hcov1.HyperConverged

		BeforeEach(func() {
			Expect(os.Setenv("OPERATOR_NAMESPACE", HcoValidNamespace)).To(Succeed())
			cr = &hcov1.HyperConverged{}
			Expect(commontestutils.NewHco().ConvertTo(cr)).To(Succeed())
		})

		It("should correctly handle a valid creation request", func(ctx context.Context) {
			req := newRequest(admissionv1.Create, cr, hcoCodec, false)

			res := wh.Handle(ctx, req)
			Expect(res.Allowed).To(BeTrue())
			Expect(res.Result.Code).To(Equal(int32(200)))
		})

		It("should correctly handle a valid dryrun creation request", func(ctx context.Context) {
			req := newRequest(admissionv1.Create, cr, hcoCodec, true)

			res := wh.Handle(ctx, req)
			Expect(res.Allowed).To(BeTrue())
			Expect(res.Result.Code).To(Equal(int32(200)))
		})

		It("should reject malformed creation requests", func(ctx context.Context) {
			req := newRequest(admissionv1.Create, cr, hcoCodec, false)
			req.OldObject = req.Object
			req.Object = runtime.RawExtension{}

			res := wh.Handle(ctx, req)
			Expect(res.Allowed).To(BeFalse())
			Expect(res.Result.Code).To(Equal(int32(400)))
			Expect(res.Result.Message).To(Equal("there is no content to decode"))

			req = newRequest(admissionv1.Create, cr, hcoCodec, false)
			req.Operation = "MALFORMED"

			res = wh.Handle(ctx, req)
			Expect(res.Allowed).To(BeFalse())
			Expect(res.Result.Code).To(Equal(int32(400)))
			Expect(res.Result.Message).To(Equal("unknown operation request \"MALFORMED\""))
		})
	})

	Context("validate update validation webhook", func() {
		var hco *hcov1.HyperConverged

		BeforeEach(func() {
			hco = &hcov1.HyperConverged{}
			Expect(commontestutils.NewHco().ConvertTo(hco)).To(Succeed())
			hco.Spec.Infra = hcov1.HyperConvergedConfig{
				NodePlacement: newHyperConvergedConfig(),
			}
			hco.Spec.Workloads = hcov1.HyperConvergedConfig{
				NodePlacement: newHyperConvergedConfig(),
			}
		})

		It("should correctly handle a valid update request", func(ctx context.Context) {
			req := newRequest(admissionv1.Update, hco, hcoCodec, false)

			res := wh.Handle(ctx, req)
			Expect(res.Allowed).To(BeTrue())
			Expect(res.Result.Code).To(Equal(int32(200)))
		})

		It("should correctly handle a valid dryrun update request", func(ctx context.Context) {
			req := newRequest(admissionv1.Update, hco, hcoCodec, true)

			res := wh.Handle(ctx, req)
			Expect(res.Allowed).To(BeTrue())
			Expect(res.Result.Code).To(Equal(int32(200)))
		})

		It("should reject malformed update requests", func(ctx context.Context) {
			req := newRequest(admissionv1.Update, hco, hcoCodec, false)
			req.Object = runtime.RawExtension{}

			res := wh.Handle(ctx, req)
			Expect(res.Allowed).To(BeFalse())
			Expect(res.Result.Code).To(Equal(int32(400)))
			Expect(res.Result.Message).To(Equal("there is no content to decode"))

			req = newRequest(admissionv1.Update, hco, hcoCodec, false)
			req.OldObject = runtime.RawExtension{}

			res = wh.Handle(ctx, req)
			Expect(res.Allowed).To(BeFalse())
			Expect(res.Result.Code).To(Equal(int32(400)))
			Expect(res.Result.Message).To(Equal("there is no content to decode"))
		})
	})

	Context("validate delete validation webhook", func() {
		var hco *hcov1.HyperConverged

		BeforeEach(func() {
			hco = &hcov1.HyperConverged{
				ObjectMeta: metav1.ObjectMeta{
					Name:      util.HyperConvergedName,
					Namespace: HcoValidNamespace,
				},
			}
		})

		It("should correctly handle a valid delete request", func(ctx context.Context) {
			req := newRequest(admissionv1.Delete, hco, hcoCodec, false)

			res := wh.Handle(ctx, req)
			Expect(res.Allowed).To(BeTrue())
			Expect(res.Result.Code).To(Equal(int32(200)))
		})

		It("should correctly handle a valid dryrun delete request", func(ctx context.Context) {
			req := newRequest(admissionv1.Delete, hco, hcoCodec, true)

			res := wh.Handle(ctx, req)
			Expect(res.Allowed).To(BeTrue())
			Expect(res.Result.Code).To(Equal(int32(200)))
		})

		It("should reject a malformed delete request", func(ctx context.Context) {
			req := newRequest(admissionv1.Delete, hco, hcoCodec, false)
			req.OldObject = req.Object
			req.Object = runtime.RawExtension{}

			res := wh.Handle(ctx, req)
			Expect(res.Allowed).To(BeFalse())
			Expect(res.Result.Code).To(Equal(int32(400)))
			Expect(res.Result.Message).To(Equal("there is no content to decode"))
		})
	})
})
