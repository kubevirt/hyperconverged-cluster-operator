package validator

import (
	"context"
	"errors"
	"os"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	networkaddonsv1 "github.com/kubevirt/cluster-network-addons-operator/pkg/apis/networkaddonsoperator/v1"
	kubevirtcorev1 "kubevirt.io/api/core/v1"
	cdiv1beta1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	sspv1beta3 "kubevirt.io/ssp-operator/api/v1beta3"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/handlers"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

var _ = Describe("v1beta1 webhooks validator", func() {
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
	v1Beta1Codec := codecFactory.LegacyCodec(hcov1beta1.SchemeGroupVersion)

	cli := fake.NewClientBuilder().WithScheme(s).Build()
	decoder := admission.NewDecoder(s)

	wh := NewV1Beta1WebhookHandler(logger, cli, decoder, HcoValidNamespace, true, nil)

	var (
		dryRun bool
		cr     *hcov1beta1.HyperConverged
	)

	BeforeEach(func() {
		Expect(os.Setenv("OPERATOR_NAMESPACE", HcoValidNamespace)).To(Succeed())
		dryRun = false
		cr = commontestutils.NewHco()
	})

	Context("Check create validation webhook", func() {
		Context("check update request", func() {
			It("should correctly handle a valid creation request", func(ctx context.Context) {
				ctx = logr.NewContext(ctx, GinkgoLogr)
				req := newV1Beta1Request(admissionv1.Create, cr, v1Beta1Codec, false)

				res := wh.Handle(ctx, req)
				Expect(res.Allowed).To(BeTrue())
				Expect(res.Result.Code).To(Equal(int32(200)))
			})

			It("should correctly handle a valid dryrun creation request", func(ctx context.Context) {
				req := newV1Beta1Request(admissionv1.Create, cr, v1Beta1Codec, true)
				ctx = logr.NewContext(ctx, GinkgoLogr)

				res := wh.Handle(ctx, req)
				Expect(res.Allowed).To(BeTrue())
				Expect(res.Result.Code).To(Equal(int32(200)))
			})

			It("should reject malformed creation requests", func(ctx context.Context) {
				req := newV1Beta1Request(admissionv1.Create, cr, v1Beta1Codec, false)
				ctx = logr.NewContext(ctx, GinkgoLogr)

				req.OldObject = req.Object
				req.Object = runtime.RawExtension{}

				res := wh.Handle(ctx, req)
				Expect(res.Allowed).To(BeFalse())
				Expect(res.Result.Code).To(Equal(int32(400)))
				Expect(res.Result.Message).To(Equal(decodeErrorMsg))

				req = newV1Beta1Request(admissionv1.Create, cr, v1Beta1Codec, false)
				req.Operation = "MALFORMED"

				res = wh.Handle(ctx, req)
				Expect(res.Allowed).To(BeFalse())
				Expect(res.Result.Code).To(Equal(int32(400)))
				Expect(res.Result.Message).To(Equal(`unknown operation request "MALFORMED"`))
			})
		})

		// Only v1beta1 specific tests. All the rest are tested in v1
		Context("check ValidateCreate", func() {
			It("should accept creation of a resource with a valid namespace", func(ctx context.Context) {
				Expect(wh.ValidateCreate(ctx, dryRun, cr)).To(Succeed())
			})

		})

		Context("validate deprecated FGs", func() {
			DescribeTable("should return warning for deprecated feature gate", func(ctx context.Context, fgs hcov1beta1.HyperConvergedFeatureGates, fgNames ...string) {
				cr.Spec.FeatureGates = fgs
				err := wh.ValidateCreate(ctx, dryRun, cr)
				Expect(err).To(HaveOccurred())
				expected := &ValidationWarning{}
				Expect(errors.As(err, &expected)).To(BeTrue())

				Expect(expected.warnings).To(HaveLen(len(fgNames)))
				for _, fgName := range fgNames {
					Expect(expected.warnings).To(ContainElements(ContainSubstring(fgName)))
				}
			},
				Entry("should trigger a warning if the withHostPassthroughCPU=false FG exists in the CR",
					hcov1beta1.HyperConvergedFeatureGates{WithHostPassthroughCPU: ptr.To(false)}, "withHostPassthroughCPU"),
				Entry("should trigger a warning if the withHostPassthroughCPU=true FG exists in the CR",
					hcov1beta1.HyperConvergedFeatureGates{WithHostPassthroughCPU: ptr.To(true)}, "withHostPassthroughCPU"),

				Entry("should trigger a warning if the deployTektonTaskResources=false FG exists in the CR",
					hcov1beta1.HyperConvergedFeatureGates{DeployTektonTaskResources: ptr.To(false)}, "deployTektonTaskResources"),
				Entry("should trigger a warning if the deployTektonTaskResources=true FG exists in the CR",
					hcov1beta1.HyperConvergedFeatureGates{DeployTektonTaskResources: ptr.To(true)}, "deployTektonTaskResources"),

				Entry("should trigger a warning if the enableManagedTenantQuota=false FG exists in the CR",
					hcov1beta1.HyperConvergedFeatureGates{EnableManagedTenantQuota: ptr.To(false)}, "enableManagedTenantQuota"),
				Entry("should trigger a warning if the enableManagedTenantQuota=true FG exists in the CR",
					hcov1beta1.HyperConvergedFeatureGates{EnableManagedTenantQuota: ptr.To(true)}, "enableManagedTenantQuota"),

				Entry("should trigger a warning if the nonRoot=false FG exists in the CR",
					hcov1beta1.HyperConvergedFeatureGates{NonRoot: ptr.To(false)}, "nonRoot"),
				Entry("should trigger a warning if the nonRoot=true FG exists in the CR",
					hcov1beta1.HyperConvergedFeatureGates{NonRoot: ptr.To(true)}, "nonRoot"),

				Entry("should trigger multiple warnings if several deprecated FG exist in the CR",
					hcov1beta1.HyperConvergedFeatureGates{
						NonRoot:                  ptr.To(true),
						EnableManagedTenantQuota: ptr.To(true),
					}, "enableManagedTenantQuota", "nonRoot"),

				Entry("should trigger multiple warnings if several deprecated FG exist in the CR, with some valid FGs",
					hcov1beta1.HyperConvergedFeatureGates{
						DownwardMetrics:             ptr.To(true),
						NonRoot:                     ptr.To(false),
						EnableCommonBootImageImport: ptr.To(true),
						EnableApplicationAwareQuota: ptr.To(false),
						EnableManagedTenantQuota:    ptr.To(false),
						DeployVMConsoleProxy:        ptr.To(false),
						DeployKubeSecondaryDNS:      ptr.To(false),
					}, "enableManagedTenantQuota", "nonRoot", "enableApplicationAwareQuota", "enableCommonBootImageImport", "deployVmConsoleProxy"),
			)
		})
	})

	Context("validate update validation webhook", func() {
		BeforeEach(func() {
			cr.Spec.Infra = hcov1beta1.HyperConvergedConfig{
				NodePlacement: newHyperConvergedConfig(),
			}
			cr.Spec.Workloads = hcov1beta1.HyperConvergedConfig{
				NodePlacement: newHyperConvergedConfig(),
			}
		})

		Context("check update request", func() {
			It("should correctly handle a valid update request", func(ctx context.Context) {
				req := newV1Beta1Request(admissionv1.Update, cr, v1Beta1Codec, false)
				ctx = logr.NewContext(ctx, GinkgoLogr)

				res := wh.Handle(ctx, req)
				Expect(res.Allowed).To(BeTrue())
				Expect(res.Result.Code).To(Equal(int32(200)))
			})

			It("should correctly handle a valid dryrun update request", func(ctx context.Context) {
				req := newV1Beta1Request(admissionv1.Update, cr, v1Beta1Codec, true)
				ctx = logr.NewContext(ctx, GinkgoLogr)

				res := wh.Handle(ctx, req)
				Expect(res.Allowed).To(BeTrue())
				Expect(res.Result.Code).To(Equal(int32(200)))
			})

			It("should reject update requests with no object", func(ctx context.Context) {
				req := newV1Beta1Request(admissionv1.Update, cr, v1Beta1Codec, false)
				req.Object = runtime.RawExtension{}
				ctx = logr.NewContext(ctx, GinkgoLogr)

				res := wh.Handle(ctx, req)
				Expect(res.Allowed).To(BeFalse())
				Expect(res.Result.Code).To(Equal(int32(400)))
				Expect(res.Result.Message).To(Equal(decodeErrorMsg))
			})

			It("should reject update requests with no oldObject", func(ctx context.Context) {
				req := newV1Beta1Request(admissionv1.Update, cr, v1Beta1Codec, false)
				req.OldObject = runtime.RawExtension{}
				ctx = logr.NewContext(ctx, GinkgoLogr)

				res := wh.Handle(ctx, req)
				Expect(res.Allowed).To(BeFalse())
				Expect(res.Result.Code).To(Equal(int32(400)))
				Expect(res.Result.Message).To(Equal(decodeErrorMsg))
			})
		})

		// Only v1beta1 specific tests. All the rest are tested in v1
		Context("check ValidateUpdate", func() {
			Context("validate deprecated FGs", func() {
				DescribeTable("should return warning for deprecated feature gate", func(ctx context.Context, fgs hcov1beta1.HyperConvergedFeatureGates, fgNames ...string) {
					newHCO := cr.DeepCopy()
					newHCO.Spec.FeatureGates = fgs

					err := wh.ValidateUpdate(ctx, dryRun, newHCO, cr)

					Expect(err).To(HaveOccurred())
					expected := &ValidationWarning{}
					Expect(errors.As(err, &expected)).To(BeTrue())

					Expect(expected.warnings).To(HaveLen(len(fgNames)))
					for _, fgName := range fgNames {
						Expect(expected.warnings).To(ContainElements(ContainSubstring(fgName)))
					}
				},
					Entry("should trigger a warning if the withHostPassthroughCPU=false FG exists in the CR",
						hcov1beta1.HyperConvergedFeatureGates{WithHostPassthroughCPU: ptr.To(false)}, "withHostPassthroughCPU"),
					Entry("should trigger a warning if the withHostPassthroughCPU=true FG exists in the CR",
						hcov1beta1.HyperConvergedFeatureGates{WithHostPassthroughCPU: ptr.To(true)}, "withHostPassthroughCPU"),

					Entry("should trigger a warning if the deployTektonTaskResources=false FG exists in the CR",
						hcov1beta1.HyperConvergedFeatureGates{DeployTektonTaskResources: ptr.To(false)}, "deployTektonTaskResources"),
					Entry("should trigger a warning if the deployTektonTaskResources=true FG exists in the CR",
						hcov1beta1.HyperConvergedFeatureGates{DeployTektonTaskResources: ptr.To(true)}, "deployTektonTaskResources"),

					Entry("should trigger a warning if the enableManagedTenantQuota=false FG exists in the CR",
						hcov1beta1.HyperConvergedFeatureGates{EnableManagedTenantQuota: ptr.To(false)}, "enableManagedTenantQuota"),
					Entry("should trigger a warning if the enableManagedTenantQuota=true FG exists in the CR",
						hcov1beta1.HyperConvergedFeatureGates{EnableManagedTenantQuota: ptr.To(true)}, "enableManagedTenantQuota"),

					Entry("should trigger a warning if the nonRoot=false FG exists in the CR",
						hcov1beta1.HyperConvergedFeatureGates{NonRoot: ptr.To(false)}, "nonRoot"),
					Entry("should trigger a warning if the nonRoot=true FG exists in the CR",
						hcov1beta1.HyperConvergedFeatureGates{NonRoot: ptr.To(true)}, "nonRoot"),

					Entry("should trigger multiple warnings if several deprecated FG exist in the CR",
						hcov1beta1.HyperConvergedFeatureGates{
							NonRoot:                  ptr.To(true),
							EnableManagedTenantQuota: ptr.To(true),
						}, "enableManagedTenantQuota", "nonRoot"),

					Entry("should trigger multiple warnings if several deprecated FG exist in the CR, with some valid FGs",
						hcov1beta1.HyperConvergedFeatureGates{
							DownwardMetrics:             ptr.To(true),
							NonRoot:                     ptr.To(false),
							EnableCommonBootImageImport: ptr.To(true),
							EnableManagedTenantQuota:    ptr.To(false),
						}, "enableManagedTenantQuota", "nonRoot", "enableCommonBootImageImport"),
				)
			})

			Context("validate moved FG on update", func() {
				//nolint:staticcheck
				DescribeTable("should return warning for enableApplicationAwareQuota on update", func(ctx context.Context, newFG, oldFG *bool) {
					newHCO := cr.DeepCopy()
					cr.Spec.FeatureGates.EnableApplicationAwareQuota = newFG
					newHCO.Spec.FeatureGates.EnableApplicationAwareQuota = oldFG

					err := wh.ValidateUpdate(ctx, dryRun, newHCO, cr)

					Expect(err).To(HaveOccurred())
					expected := &ValidationWarning{}
					Expect(errors.As(err, &expected)).To(BeTrue())

					Expect(expected.warnings).To(HaveLen(1))
					Expect(expected.warnings).To(ContainElements(ContainSubstring("enableApplicationAwareQuota")))
				},
					Entry("should trigger warning if enableApplicationAwareQuota appeared as true", nil, ptr.To(true)),
					Entry("should trigger warning if enableApplicationAwareQuota appeared as false", nil, ptr.To(false)),
					Entry("should trigger warning if enableApplicationAwareQuota has changed from true to false", ptr.To(true), ptr.To(false)),
					Entry("should trigger warning if enableApplicationAwareQuota has changed from false to true", ptr.To(false), ptr.To(true)),
				)

				//nolint:staticcheck
				DescribeTable("should not return warning for enableApplicationAwareQuota if not change", func(ctx context.Context, newFG, oldFG *bool) {
					cli := getFakeV1Beta1Client(cr)
					wh := NewV1Beta1WebhookHandler(logger, cli, decoder, HcoValidNamespace, true, nil)
					newHCO := cr.DeepCopy()
					cr.Spec.FeatureGates.EnableApplicationAwareQuota = newFG
					newHCO.Spec.FeatureGates.EnableApplicationAwareQuota = oldFG

					Expect(wh.ValidateUpdate(ctx, dryRun, newHCO, cr)).To(Succeed())
				},
					Entry("should not trigger warning if enableApplicationAwareQuota (true) disappeared", ptr.To(true), nil),
					Entry("should not trigger warning if enableApplicationAwareQuota (false) disappeared", ptr.To(false), nil),
					Entry("should not trigger warning if enableApplicationAwareQuota (true) wasn't changed", ptr.To(true), ptr.To(true)),
					Entry("should not trigger warning if enableApplicationAwareQuota (false) wasn't changed", ptr.To(false), ptr.To(false)),
				)

				//nolint:staticcheck
				DescribeTable("should return warning for enableCommonBootImageImport on update", func(ctx context.Context, newFG, oldFG *bool) {
					newHCO := cr.DeepCopy()
					cr.Spec.FeatureGates.EnableCommonBootImageImport = newFG
					newHCO.Spec.FeatureGates.EnableCommonBootImageImport = oldFG

					err := wh.ValidateUpdate(ctx, dryRun, newHCO, cr)

					Expect(err).To(HaveOccurred())
					expected := &ValidationWarning{}
					Expect(errors.As(err, &expected)).To(BeTrue())

					Expect(expected.warnings).To(HaveLen(1))
					Expect(expected.warnings).To(ContainElements(ContainSubstring("enableCommonBootImageImport")))
				},
					Entry("should trigger warning if enableCommonBootImageImport appeared as true", nil, ptr.To(true)),
					Entry("should trigger warning if enableCommonBootImageImport appeared as false", nil, ptr.To(false)),
					Entry("should trigger warning if enableCommonBootImageImport has changed from true to false", ptr.To(true), ptr.To(false)),
					Entry("should trigger warning if enableCommonBootImageImport has changed from false to true", ptr.To(false), ptr.To(true)),
				)

				//nolint:staticcheck
				DescribeTable("should not return warning for enableCommonBootImageImport if not change", func(ctx context.Context, newFG, oldFG *bool) {
					cli := getFakeV1Beta1Client(cr)
					wh := NewV1Beta1WebhookHandler(logger, cli, decoder, HcoValidNamespace, true, nil)
					newHCO := cr.DeepCopy()
					cr.Spec.FeatureGates.EnableCommonBootImageImport = newFG
					newHCO.Spec.FeatureGates.EnableCommonBootImageImport = oldFG

					Expect(wh.ValidateUpdate(ctx, dryRun, newHCO, cr)).To(Succeed())
				},
					Entry("should not trigger warning if enableCommonBootImageImport (true) disappeared", ptr.To(true), nil),
					Entry("should not trigger warning if enableCommonBootImageImport (false) disappeared", ptr.To(false), nil),
					Entry("should not trigger warning if enableCommonBootImageImport (true) wasn't changed", ptr.To(true), ptr.To(true)),
					Entry("should not trigger warning if enableCommonBootImageImport (false) wasn't changed", ptr.To(false), ptr.To(false)),
				)

				//nolint:staticcheck
				DescribeTable("should return warning for deployVmConsoleProxy on update", func(ctx context.Context, newFG, oldFG *bool) {
					newHCO := cr.DeepCopy()
					cr.Spec.FeatureGates.DeployVMConsoleProxy = newFG
					newHCO.Spec.FeatureGates.DeployVMConsoleProxy = oldFG

					err := wh.ValidateUpdate(ctx, dryRun, newHCO, cr)

					Expect(err).To(HaveOccurred())
					expected := &ValidationWarning{}
					Expect(errors.As(err, &expected)).To(BeTrue())

					Expect(expected.warnings).To(HaveLen(1))
					Expect(expected.warnings).To(ContainElements(ContainSubstring("deployVmConsoleProxy")))
				},
					Entry("should trigger warning if deployVmConsoleProxy appeared as true", nil, ptr.To(true)),
					Entry("should trigger warning if deployVmConsoleProxy appeared as false", nil, ptr.To(false)),
					Entry("should trigger warning if deployVmConsoleProxy has changed from true to false", ptr.To(true), ptr.To(false)),
					Entry("should trigger warning if deployVmConsoleProxy has changed from false to true", ptr.To(false), ptr.To(true)),
				)

				//nolint:staticcheck
				DescribeTable("should not return warning for deployVmConsoleProxy if not change", func(ctx context.Context, newFG, oldFG *bool) {
					cli := getFakeV1Beta1Client(cr)
					wh := NewV1Beta1WebhookHandler(logger, cli, decoder, HcoValidNamespace, true, nil)
					newHCO := cr.DeepCopy()
					cr.Spec.FeatureGates.DeployVMConsoleProxy = newFG
					newHCO.Spec.FeatureGates.DeployVMConsoleProxy = oldFG

					Expect(wh.ValidateUpdate(ctx, dryRun, newHCO, cr)).To(Succeed())
				},
					Entry("should not trigger warning if deployVmConsoleProxy (true) disappeared", ptr.To(true), nil),
					Entry("should not trigger warning if deployVmConsoleProxy (false) disappeared", ptr.To(false), nil),
					Entry("should not trigger warning if deployVmConsoleProxy (true) wasn't changed", ptr.To(true), ptr.To(true)),
					Entry("should not trigger warning if deployVmConsoleProxy (false) wasn't changed", ptr.To(false), ptr.To(false)),
				)

				//nolint:staticcheck
				DescribeTable("should not return warning for deployKubeSecondaryDNS if not change", func(ctx context.Context, newFG, oldFG *bool) {
					cli := getFakeV1Beta1Client(cr)
					wh := NewV1Beta1WebhookHandler(logger, cli, decoder, HcoValidNamespace, true, nil)
					newHCO := cr.DeepCopy()
					cr.Spec.FeatureGates.DeployKubeSecondaryDNS = newFG
					newHCO.Spec.FeatureGates.DeployKubeSecondaryDNS = oldFG

					Expect(wh.ValidateUpdate(ctx, dryRun, newHCO, cr)).To(Succeed())
				},
					Entry("should not trigger warning if deployKubeSecondaryDNS (true) disappeared", ptr.To(true), nil),
					Entry("should not trigger warning if deployKubeSecondaryDNS (false) disappeared", ptr.To(false), nil),
					Entry("should not trigger warning if deployKubeSecondaryDNS (true) wasn't changed", ptr.To(true), ptr.To(true)),
					Entry("should not trigger warning if deployKubeSecondaryDNS (false) wasn't changed", ptr.To(false), ptr.To(false)),
				)
			})
		})
	})

	Context("validate delete validation webhook", func() {
		Context("check delete request", func() {
			It("should correctly handle a valid delete request", func(ctx context.Context) {
				req := newV1Beta1Request(admissionv1.Delete, cr, v1Beta1Codec, false)
				ctx = logr.NewContext(ctx, GinkgoLogr)

				res := wh.Handle(ctx, req)
				Expect(res.Allowed).To(BeTrue())
				Expect(res.Result.Code).To(Equal(int32(200)))
			})

			It("should correctly handle a valid dryrun delete request", func(ctx context.Context) {
				req := newV1Beta1Request(admissionv1.Delete, cr, v1Beta1Codec, true)
				ctx = logr.NewContext(ctx, GinkgoLogr)

				res := wh.Handle(ctx, req)
				Expect(res.Allowed).To(BeTrue())
				Expect(res.Result.Code).To(Equal(int32(200)))
			})

			It("should reject a malformed delete request", func(ctx context.Context) {
				req := newV1Beta1Request(admissionv1.Delete, cr, v1Beta1Codec, false)
				req.OldObject = req.Object
				req.Object = runtime.RawExtension{}
				ctx = logr.NewContext(ctx, GinkgoLogr)

				res := wh.Handle(ctx, req)
				Expect(res.Allowed).To(BeFalse())
				Expect(res.Result.Code).To(Equal(int32(400)))
				Expect(res.Result.Message).To(Equal(decodeErrorMsg))
			})
		})

		Context("check ValidateDelete", func() {
			It("should validate deletion", func(ctx context.Context) {
				cli := getFakeV1Beta1Client(cr)

				wh := NewV1Beta1WebhookHandler(logger, cli, decoder, HcoValidNamespace, true, nil)

				Expect(wh.ValidateDelete(ctx, dryRun, cr)).To(Succeed())

				By("Validate that KV still exists, as it a dry-run deletion")
				kv := handlers.NewKubeVirtWithNameOnly(cr)
				Expect(util.GetRuntimeObject(ctx, cli, kv)).To(Succeed())

				By("Validate that CDI still exists, as it a dry-run deletion")
				cdi := handlers.NewCDIWithNameOnly(cr)
				Expect(util.GetRuntimeObject(ctx, cli, cdi)).To(Succeed())
			})
		})
	})

	Context("MediatedDeviceTypes", func() {
		var cr *hcov1beta1.HyperConverged
		var newCr *hcov1beta1.HyperConverged

		BeforeEach(func() {
			Expect(os.Setenv("OPERATOR_NAMESPACE", HcoValidNamespace)).To(Succeed())
			cr = commontestutils.NewHco()
			cr.Spec.MediatedDevicesConfiguration = nil
			newCr = cr.DeepCopy()
		})

		DescribeTable("Check mediatedDevicesTypes -> mediatedDeviceTypes transition", func(ctx context.Context, mDConfiguration *hcov1beta1.MediatedDevicesConfiguration, expected types.GomegaMatcher) {
			// create
			newCr.Spec.MediatedDevicesConfiguration = mDConfiguration
			Expect(wh.ValidateCreate(ctx, false, newCr)).To(expected)

			// update
			cli := getFakeV1Beta1Client(cr)
			cli.InitiateUpdateErrors(getUpdateError(noFailure))
			whU := NewV1Beta1WebhookHandler(logger, cli, decoder, HcoValidNamespace, true, nil)
			Expect(whU.ValidateUpdate(ctx, false, newCr, cr)).To(expected)
		},
			Entry("should not fail with no configuration",
				nil,
				Succeed(),
			),
			Entry("should not fail if using only mediatedDeviceTypes",
				&hcov1beta1.MediatedDevicesConfiguration{
					MediatedDeviceTypes: []string{"nvidia-222", "nvidia-230"},
					NodeMediatedDeviceTypes: []hcov1beta1.NodeMediatedDeviceTypesConfig{
						{
							NodeSelector: map[string]string{
								"testLabel1": "true",
							},
							MediatedDeviceTypes: []string{
								"nvidia-223",
							},
						},
						{
							NodeSelector: map[string]string{
								"testLabel2": "true",
							},
							MediatedDeviceTypes: []string{
								"nvidia-229",
							},
						},
					},
				},
				Succeed(),
			),
			Entry("should not fail if using only deprecated APIs",
				&hcov1beta1.MediatedDevicesConfiguration{
					MediatedDevicesTypes: []string{"nvidia-222", "nvidia-230"},
					NodeMediatedDeviceTypes: []hcov1beta1.NodeMediatedDeviceTypesConfig{
						{
							NodeSelector: map[string]string{
								"testLabel1": "true",
							},
							MediatedDevicesTypes: []string{
								"nvidia-223",
							},
						},
						{
							NodeSelector: map[string]string{
								"testLabel2": "true",
							},
							MediatedDevicesTypes: []string{
								"nvidia-229",
							},
						},
					},
				},
				Succeed(),
			),
			Entry("should not fail if correctly using both mediatedDeviceTypes and deprecated APIs",
				&hcov1beta1.MediatedDevicesConfiguration{
					MediatedDevicesTypes: []string{"nvidia-222", "nvidia-230"},
					MediatedDeviceTypes:  []string{"nvidia-222", "nvidia-230"},
					NodeMediatedDeviceTypes: []hcov1beta1.NodeMediatedDeviceTypesConfig{
						{
							NodeSelector: map[string]string{
								"testLabel1": "true",
							},
							MediatedDevicesTypes: []string{
								"nvidia-223",
							},
							MediatedDeviceTypes: []string{
								"nvidia-223",
							},
						},
						{
							NodeSelector: map[string]string{
								"testLabel2": "true",
							},
							MediatedDevicesTypes: []string{
								"nvidia-229",
							},
							MediatedDeviceTypes: []string{
								"nvidia-229",
							},
						},
					},
				},
				Succeed(),
			),
			Entry("should fail if mixing mediatedDeviceTypes and deprecated APIs on spec.mediatedDevicesConfiguration.mediatedDeviceTypes",
				&hcov1beta1.MediatedDevicesConfiguration{
					MediatedDevicesTypes: []string{"nvidia-222", "nvidia-230"},
					MediatedDeviceTypes:  []string{"nvidia-222"},
					NodeMediatedDeviceTypes: []hcov1beta1.NodeMediatedDeviceTypesConfig{
						{
							NodeSelector: map[string]string{
								"testLabel1": "true",
							},
							MediatedDevicesTypes: []string{
								"nvidia-223",
							},
							MediatedDeviceTypes: []string{
								"nvidia-223",
							},
						},
						{
							NodeSelector: map[string]string{
								"testLabel2": "true",
							},
							MediatedDevicesTypes: []string{
								"nvidia-229",
							},
							MediatedDeviceTypes: []string{
								"nvidia-229",
							},
						},
					},
				},
				Not(Succeed()),
			),
			Entry("should fail if mixing mediatedDeviceTypes and deprecated APIs on spec.mediatedDevicesConfiguration.nodeMediatedDeviceTypes[1].mediatedDeviceTypes",
				&hcov1beta1.MediatedDevicesConfiguration{
					MediatedDevicesTypes: []string{"nvidia-222", "nvidia-230"},
					MediatedDeviceTypes:  []string{"nvidia-222", "nvidia-230"},
					NodeMediatedDeviceTypes: []hcov1beta1.NodeMediatedDeviceTypesConfig{
						{
							NodeSelector: map[string]string{
								"testLabel1": "true",
							},
							MediatedDevicesTypes: []string{
								"nvidia-223",
							},
							MediatedDeviceTypes: []string{
								"nvidia-223",
							},
						},
						{
							NodeSelector: map[string]string{
								"testLabel2": "true",
							},
							MediatedDevicesTypes: []string{
								"nvidia-229",
							},
							MediatedDeviceTypes: []string{
								"nvidia-229", "nvidia-230",
							},
						},
					},
				},
				Not(Succeed()),
			),
		)

	})

})

func newV1Beta1Request(operation admissionv1.Operation, cr *hcov1beta1.HyperConverged, encoder runtime.Encoder, dryrun bool) admission.Request {
	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			DryRun:    ptr.To(dryrun),
			Operation: operation,
			Resource: metav1.GroupVersionResource{
				Group:    hcov1beta1.SchemeGroupVersion.Group,
				Version:  hcov1beta1.SchemeGroupVersion.Version,
				Resource: "testresource",
			},
			UID: "test-uid",
		},
	}

	switch operation {
	case admissionv1.Create:
		req.Object = runtime.RawExtension{
			Raw:    []byte(runtime.EncodeOrDie(encoder, cr)),
			Object: cr,
		}
	case admissionv1.Update:
		req.Object = runtime.RawExtension{
			Raw:    []byte(runtime.EncodeOrDie(encoder, cr)),
			Object: cr,
		}
		req.OldObject = runtime.RawExtension{
			Raw:    []byte(runtime.EncodeOrDie(encoder, cr)),
			Object: cr,
		}
	case admissionv1.Delete:
		req.OldObject = runtime.RawExtension{
			Raw:    []byte(runtime.EncodeOrDie(encoder, cr)),
			Object: cr,
		}
	default:
		req.Object = runtime.RawExtension{}
		req.OldObject = runtime.RawExtension{}
	}

	return req
}
