package validator

import (
	"context"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	openshiftconfigv1 "github.com/openshift/api/config/v1"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	networkaddonsv1 "github.com/kubevirt/cluster-network-addons-operator/pkg/apis/networkaddonsoperator/v1"
	kubevirtcorev1 "kubevirt.io/api/core/v1"
	cdiv1beta1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	sdkapi "kubevirt.io/controller-lifecycle-operator-sdk/api"
	sspv1beta3 "kubevirt.io/ssp-operator/api/v1beta3"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/handlers"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/tlssecprofile"
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

	getFakeClient := func(hco *hcov1.HyperConverged) *commontestutils.HcoTestClient {
		GinkgoHelper()

		v1beta1HC := &hcov1beta1.HyperConverged{}
		Expect(v1beta1HC.ConvertFrom(hco)).To(Succeed())

		kv, err := handlers.NewKubeVirt(v1beta1HC)
		Expect(err).ToNot(HaveOccurred())

		cdi, err := handlers.NewCDI(v1beta1HC)
		Expect(err).ToNot(HaveOccurred())

		cna, err := handlers.NewNetworkAddons(v1beta1HC)
		Expect(err).ToNot(HaveOccurred())

		ssp, _, err := handlers.NewSSP(v1beta1HC)
		Expect(err).ToNot(HaveOccurred())

		return commontestutils.InitClient([]client.Object{hco, kv, cdi, cna, ssp})
	}

	codecFactory := serializer.NewCodecFactory(s)
	hcoCodec := codecFactory.LegacyCodec(hcov1.SchemeGroupVersion, hcov1beta1.SchemeGroupVersion)

	decoder := admission.NewDecoder(s)

	var (
		cr        *hcov1.HyperConverged
		dryRun    bool
		wh        *WebhookHandler
		v1beta1WH *WebhookV1Beta1Handler
		cli       client.Client
	)

	JustBeforeEach(func() {
		v1beta1WH = NewWebhookV1Beta1Handler(GinkgoLogr, cli, decoder, HcoValidNamespace, true)
		wh = NewWebhookHandler(GinkgoLogr, cli, decoder, HcoValidNamespace, true, v1beta1WH)
	})

	BeforeEach(func() {
		Expect(os.Setenv("OPERATOR_NAMESPACE", HcoValidNamespace)).To(Succeed())
		cr = &hcov1.HyperConverged{}
		Expect(commontestutils.NewHco().ConvertTo(cr)).To(Succeed())
		dryRun = false
	})

	Context("Check create validation webhook", func() {
		BeforeEach(func() {
			cli = fake.NewClientBuilder().WithScheme(s).Build()
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

		It("should accept creation of a resource with a valid namespace", func(ctx context.Context) {
			Expect(wh.validateCreate(GinkgoLogr, dryRun, cr).Allowed).To(BeTrue())
		})

		DescribeTable("Validate annotations", func(ctx context.Context, annotations map[string]string, assertion types.GomegaMatcher) {
			cr.Annotations = annotations
			Expect(wh.validateCreate(GinkgoLogr, dryRun, cr).Allowed).To(assertion)
		},
			Entry("should accept creation of a resource with a valid kv annotation",
				map[string]string{common.JSONPatchKVAnnotationName: validKvAnnotation},
				BeTrue(),
			),
			Entry("should reject creation of a resource with an invalid kv annotation",
				map[string]string{common.JSONPatchKVAnnotationName: invalidKvAnnotation},
				BeFalse(),
			),
			Entry("should accept creation of a resource with a valid cdi annotation",
				map[string]string{common.JSONPatchCDIAnnotationName: validCdiAnnotation},
				BeTrue(),
			),
			Entry("should reject creation of a resource with an invalid cdi annotation",
				map[string]string{common.JSONPatchCDIAnnotationName: invalidCdiAnnotation},
				BeFalse(),
			),
			Entry("should accept creation of a resource with a valid cna annotation",
				map[string]string{common.JSONPatchCNAOAnnotationName: validCnaAnnotation},
				BeTrue(),
			),
			Entry("should reject creation of a resource with an invalid cna annotation",
				map[string]string{common.JSONPatchCNAOAnnotationName: invalidCnaAnnotation},
				BeFalse(),
			),
			Entry("should accept creation of a resource with a valid ssp annotation",
				map[string]string{common.JSONPatchSSPAnnotationName: validSspAnnotation},
				BeTrue(),
			),
			Entry("should reject creation of a resource with an invalid ssp annotation",
				map[string]string{common.JSONPatchSSPAnnotationName: invalidSspAnnotation},
				BeFalse(),
			),
		)

		Context("test permitted host devices validation", func() {
			It("should allow unique PCI Host Device", func(ctx context.Context) {
				cr.Spec.PermittedHostDevices = &hcov1.PermittedHostDevices{
					PciHostDevices: []hcov1.PciHostDevice{
						{
							PCIDeviceSelector: "111",
							ResourceName:      "name",
						},
						{
							PCIDeviceSelector: "222",
							ResourceName:      "name",
						},
						{
							PCIDeviceSelector: "333",
							ResourceName:      "name",
						},
					},
				}
				Expect(wh.validateCreate(GinkgoLogr, dryRun, cr).Allowed).To(BeTrue())
			})

			It("should allow unique Mediate Host Device", func(ctx context.Context) {
				cr.Spec.PermittedHostDevices = &hcov1.PermittedHostDevices{
					MediatedDevices: []hcov1.MediatedHostDevice{
						{
							MDEVNameSelector: "111",
							ResourceName:     "name",
						},
						{
							MDEVNameSelector: "222",
							ResourceName:     "name",
						},
						{
							MDEVNameSelector: "333",
							ResourceName:     "name",
						},
					},
				}

				checkAcceptedRequest(wh.validateCreate(GinkgoLogr, dryRun, cr))
			})
		})

		Context("Test DataImportCronTemplates", func() {
			var image1, image2, image3, image4 hcov1.DataImportCronTemplate

			BeforeEach(func() {
				dryRun = false

				image1 = hcov1.DataImportCronTemplate{
					ObjectMeta: metav1.ObjectMeta{Name: "image1"},
					Spec: &cdiv1beta1.DataImportCronSpec{
						Schedule: "1 */12 * * *",
						Template: cdiv1beta1.DataVolume{
							Spec: cdiv1beta1.DataVolumeSpec{
								Source: &cdiv1beta1.DataVolumeSource{
									Registry: &cdiv1beta1.DataVolumeSourceRegistry{URL: ptr.To("docker://someregistry/image1")},
								},
							},
						},
						ManagedDataSource: "image1",
					},
				}

				image2 = hcov1.DataImportCronTemplate{
					ObjectMeta: metav1.ObjectMeta{Name: "image2"},
					Spec: &cdiv1beta1.DataImportCronSpec{
						Schedule: "2 */12 * * *",
						Template: cdiv1beta1.DataVolume{
							Spec: cdiv1beta1.DataVolumeSpec{
								Source: &cdiv1beta1.DataVolumeSource{
									Registry: &cdiv1beta1.DataVolumeSourceRegistry{URL: ptr.To("docker://someregistry/image2")},
								},
							},
						},
						ManagedDataSource: "image2",
					},
				}

				image3 = hcov1.DataImportCronTemplate{
					ObjectMeta: metav1.ObjectMeta{Name: "image3"},
					Spec: &cdiv1beta1.DataImportCronSpec{
						Schedule: "3 */12 * * *",
						Template: cdiv1beta1.DataVolume{
							Spec: cdiv1beta1.DataVolumeSpec{
								Source: &cdiv1beta1.DataVolumeSource{
									Registry: &cdiv1beta1.DataVolumeSourceRegistry{URL: ptr.To("docker://someregistry/image3")},
								},
							},
						},
						ManagedDataSource: "image3",
					},
				}

				image4 = hcov1.DataImportCronTemplate{
					ObjectMeta: metav1.ObjectMeta{Name: "image4"},
					Spec: &cdiv1beta1.DataImportCronSpec{
						Schedule: "4 */12 * * *",
						Template: cdiv1beta1.DataVolume{
							Spec: cdiv1beta1.DataVolumeSpec{
								Source: &cdiv1beta1.DataVolumeSource{
									Registry: &cdiv1beta1.DataVolumeSourceRegistry{URL: ptr.To("docker://someregistry/image4")},
								},
							},
						},
						ManagedDataSource: "image4",
					},
				}

				cr.Spec.DataImportCronTemplates = []hcov1.DataImportCronTemplate{image1, image2, image3, image4}
			})

			It("should allow setting the annotation to true", func(ctx context.Context) {
				cr.Spec.DataImportCronTemplates[0].Annotations = map[string]string{util.DataImportCronEnabledAnnotation: "true"}
				cr.Spec.DataImportCronTemplates[1].Annotations = map[string]string{util.DataImportCronEnabledAnnotation: "TRUE"}
				cr.Spec.DataImportCronTemplates[2].Annotations = map[string]string{util.DataImportCronEnabledAnnotation: "TrUe"}
				cr.Spec.DataImportCronTemplates[3].Annotations = map[string]string{util.DataImportCronEnabledAnnotation: "tRuE"}

				checkAcceptedRequest(wh.validateCreate(GinkgoLogr, dryRun, cr))
			})

			It("should allow setting the annotation to false", func(ctx context.Context) {
				cr.Spec.DataImportCronTemplates[0].Annotations = map[string]string{util.DataImportCronEnabledAnnotation: "false"}
				cr.Spec.DataImportCronTemplates[1].Annotations = map[string]string{util.DataImportCronEnabledAnnotation: "FALSE"}
				cr.Spec.DataImportCronTemplates[2].Annotations = map[string]string{util.DataImportCronEnabledAnnotation: "FaLsE"}
				cr.Spec.DataImportCronTemplates[3].Annotations = map[string]string{util.DataImportCronEnabledAnnotation: "fAlSe"}

				checkAcceptedRequest(wh.validateCreate(GinkgoLogr, dryRun, cr))
			})

			It("should allow setting no annotation", func(ctx context.Context) {
				checkAcceptedRequest(wh.validateCreate(GinkgoLogr, dryRun, cr))
			})

			It("should not allow empty annotation", func(ctx context.Context) {
				cr.Spec.DataImportCronTemplates[0].Annotations = map[string]string{util.DataImportCronEnabledAnnotation: ""}
				cr.Spec.DataImportCronTemplates[1].Annotations = map[string]string{util.DataImportCronEnabledAnnotation: ""}

				checkRejectedRequest(wh.validateCreate(GinkgoLogr, dryRun, cr))
			})

			It("should not allow unknown annotation values", func(ctx context.Context) {
				cr.Spec.DataImportCronTemplates[0].Annotations = map[string]string{util.DataImportCronEnabledAnnotation: "wrong"}
				cr.Spec.DataImportCronTemplates[1].Annotations = map[string]string{util.DataImportCronEnabledAnnotation: "mistake"}

				checkRejectedRequest(wh.validateCreate(GinkgoLogr, dryRun, cr))
			})

			Context("Empty DICT spec", func() {
				It("don't allow if the annotation does not exist", func(ctx context.Context) {
					// empty annotation map
					cr.Spec.DataImportCronTemplates[0].Annotations = map[string]string{}
					cr.Spec.DataImportCronTemplates[0].Spec = nil
					// no annotation map
					cr.Spec.DataImportCronTemplates[1].Spec = nil

					checkRejectedRequest(wh.validateCreate(GinkgoLogr, dryRun, cr))
				})

				It("don't allow if the annotation is true", func(ctx context.Context) {
					cr.Spec.DataImportCronTemplates[0].Annotations = map[string]string{util.DataImportCronEnabledAnnotation: "True"}
					cr.Spec.DataImportCronTemplates[0].Spec = nil
					cr.Spec.DataImportCronTemplates[1].Annotations = map[string]string{util.DataImportCronEnabledAnnotation: "true"}
					cr.Spec.DataImportCronTemplates[1].Spec = nil

					checkRejectedRequest(wh.validateCreate(GinkgoLogr, dryRun, cr))
				})

				It("allow if the annotation is false", func(ctx context.Context) {
					cr.Spec.DataImportCronTemplates[0].Annotations = map[string]string{util.DataImportCronEnabledAnnotation: "False"}
					cr.Spec.DataImportCronTemplates[0].Spec = nil
					cr.Spec.DataImportCronTemplates[1].Annotations = map[string]string{util.DataImportCronEnabledAnnotation: "false"}
					cr.Spec.DataImportCronTemplates[1].Spec = nil

					checkAcceptedRequest(wh.validateCreate(GinkgoLogr, dryRun, cr))
				})
			})
		})

		Context("validate tlsSecurityProfiles", func() {
			updateTLSSecurityProfile := func(minTLSVersion openshiftconfigv1.TLSProtocolVersion, ciphers []string) admission.Response {
				cr.Spec.TLSSecurityProfile = &openshiftconfigv1.TLSSecurityProfile{
					Custom: &openshiftconfigv1.CustomTLSProfile{
						TLSProfileSpec: openshiftconfigv1.TLSProfileSpec{
							MinTLSVersion: minTLSVersion,
							Ciphers:       ciphers,
						},
					},
				}

				return wh.validateCreate(GinkgoLogr, dryRun, cr)
			}

			DescribeTable("should succeed if has any of the HTTP/2-required ciphers",
				func(cipher string) {
					checkAcceptedRequest(
						updateTLSSecurityProfile(openshiftconfigv1.VersionTLS12, []string{"DHE-RSA-AES256-GCM-SHA384", cipher, "DHE-RSA-CHACHA20-POLY1305"}),
					)
				},
				Entry("ECDHE-RSA-AES128-GCM-SHA256", "ECDHE-RSA-AES128-GCM-SHA256"),
				Entry("ECDHE-ECDSA-AES128-GCM-SHA256", "ECDHE-ECDSA-AES128-GCM-SHA256"),
			)

			It("should fail if does not have any of the HTTP/2-required ciphers", func() {
				checkRejectedRequest(
					updateTLSSecurityProfile(openshiftconfigv1.VersionTLS12, []string{"DHE-RSA-AES256-GCM-SHA384", "DHE-RSA-CHACHA20-POLY1305"}),
					"http2: TLSConfig.CipherSuites is missing an HTTP/2-required AES_128_GCM_SHA256 cipher (need at least one of ECDHE-RSA-AES128-GCM-SHA256 or ECDHE-ECDSA-AES128-GCM-SHA256)",
				)
			})

			It("should succeed if does not have any of the HTTP/2-required ciphers but TLS version >= 1.3", func() {
				checkAcceptedRequest(updateTLSSecurityProfile(openshiftconfigv1.VersionTLS13, []string{}))
			})

			It("should fail if does have custom ciphers with TLS version >= 1.3", func() {
				checkRejectedRequest(
					updateTLSSecurityProfile(openshiftconfigv1.VersionTLS13, []string{"TLS_AES_128_GCM_SHA256", "TLS_CHACHA20_POLY1305_SHA256"}),
					"custom ciphers cannot be selected when minTLSVersion is VersionTLS13",
				)
			})

			It("should fail when minTLSVersion is invalid", func() {
				checkRejectedRequest(
					updateTLSSecurityProfile("invalidProtocolVersion", []string{"TLS_AES_128_GCM_SHA256", "TLS_CHACHA20_POLY1305_SHA256"}),
					"invalid value for spec.tlsSecurityProfile.custom.minTLSVersion",
				)
			})

			It("should fail when type is Custom but custom field is nil", func(ctx context.Context) {
				cr.Spec.TLSSecurityProfile = &openshiftconfigv1.TLSSecurityProfile{
					Type:   openshiftconfigv1.TLSProfileCustomType,
					Custom: nil,
				}

				checkRejectedRequest(
					wh.validateCreate(GinkgoLogr, dryRun, cr),
					"missing required field spec.tlsSecurityProfile.custom when type is Custom",
				)
			})
		})

		Context("validate deprecated FGs", func() {
			DescribeTable("should return warning for deprecated feature gate", func(ctx context.Context, fgs hcov1.HyperConvergedFeatureGates, fgNames ...string) {
				cr.Spec.FeatureGates = fgs
				resp := wh.validateCreate(GinkgoLogr, dryRun, cr)
				checkAcceptedRequest(resp, fgNames...)
			},
				Entry("should trigger a warning if the withHostPassthroughCPU=false FG exists in the CR",
					hcov1.HyperConvergedFeatureGates{WithHostPassthroughCPU: ptr.To(false)}, "withHostPassthroughCPU"),
				Entry("should trigger a warning if the withHostPassthroughCPU=true FG exists in the CR",
					hcov1.HyperConvergedFeatureGates{WithHostPassthroughCPU: ptr.To(true)}, "withHostPassthroughCPU"),

				Entry("should trigger a warning if the deployTektonTaskResources=false FG exists in the CR",
					hcov1.HyperConvergedFeatureGates{DeployTektonTaskResources: ptr.To(false)}, "deployTektonTaskResources"),
				Entry("should trigger a warning if the deployTektonTaskResources=true FG exists in the CR",
					hcov1.HyperConvergedFeatureGates{DeployTektonTaskResources: ptr.To(true)}, "deployTektonTaskResources"),

				Entry("should trigger a warning if the enableManagedTenantQuota=false FG exists in the CR",
					hcov1.HyperConvergedFeatureGates{EnableManagedTenantQuota: ptr.To(false)}, "enableManagedTenantQuota"),
				Entry("should trigger a warning if the enableManagedTenantQuota=true FG exists in the CR",
					hcov1.HyperConvergedFeatureGates{EnableManagedTenantQuota: ptr.To(true)}, "enableManagedTenantQuota"),

				Entry("should trigger a warning if the nonRoot=false FG exists in the CR",
					hcov1.HyperConvergedFeatureGates{NonRoot: ptr.To(false)}, "nonRoot"),
				Entry("should trigger a warning if the nonRoot=true FG exists in the CR",
					hcov1.HyperConvergedFeatureGates{NonRoot: ptr.To(true)}, "nonRoot"),

				Entry("should trigger multiple warnings if several deprecated FG exist in the CR",
					hcov1.HyperConvergedFeatureGates{
						NonRoot:                  ptr.To(true),
						EnableManagedTenantQuota: ptr.To(true),
					}, "enableManagedTenantQuota", "nonRoot"),

				Entry("should trigger multiple warnings if several deprecated FG exist in the CR, with some valid FGs",
					hcov1.HyperConvergedFeatureGates{
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

		Context("validate affinity", func() {
			It("should allow empty affinity", func(ctx context.Context) {
				cr.Spec.Infra.NodePlacement = &sdkapi.NodePlacement{
					Affinity: nil,
				}
				cr.Spec.Workloads.NodePlacement = &sdkapi.NodePlacement{
					Affinity: nil,
				}

				checkAcceptedRequest(wh.validateCreate(GinkgoLogr, dryRun, cr))
			})

			It("should allow empty affinity", func(ctx context.Context) {
				cr.Spec.Infra.NodePlacement = &sdkapi.NodePlacement{
					Affinity: &corev1.Affinity{},
				}
				cr.Spec.Workloads.NodePlacement = &sdkapi.NodePlacement{
					Affinity: &corev1.Affinity{},
				}

				checkAcceptedRequest(wh.validateCreate(GinkgoLogr, dryRun, cr))
			})

			It("should allow valid affinity", func(ctx context.Context) {
				cr.Spec.Infra.NodePlacement = &sdkapi.NodePlacement{
					Affinity: &corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									{
										MatchExpressions: []corev1.NodeSelectorRequirement{
											{
												Key:      "kubernetes.io/os",
												Operator: corev1.NodeSelectorOpIn,
												Values:   []string{"linux"},
											},
										},
									},
								},
							},
						},
					},
				}

				cr.Spec.Workloads.NodePlacement = &sdkapi.NodePlacement{
					Affinity: &corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									{
										MatchExpressions: []corev1.NodeSelectorRequirement{
											{
												Key:      "kubernetes.io/os",
												Operator: corev1.NodeSelectorOpIn,
												Values:   []string{"linux"},
											},
										},
									},
								},
							},
						},
					},
				}

				checkAcceptedRequest(wh.validateCreate(GinkgoLogr, dryRun, cr))
			})

			It("should reject invalid workloads affinity: unknown operator", func(ctx context.Context) {
				cr.Spec.Infra.NodePlacement = &sdkapi.NodePlacement{
					Affinity: &corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									{
										MatchExpressions: []corev1.NodeSelectorRequirement{
											{
												Key:      "kubernetes.io/os",
												Operator: corev1.NodeSelectorOpIn,
												Values:   []string{"linux"},
											},
										},
									},
								},
							},
						},
					},
				}

				cr.Spec.Workloads.NodePlacement = &sdkapi.NodePlacement{
					Affinity: &corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									{
										MatchExpressions: []corev1.NodeSelectorRequirement{
											{
												Key:      "kubernetes.io/os",
												Operator: "WrongOperator",
												Values:   []string{"linux"},
											},
										},
									},
								},
							},
						},
					},
				}

				checkRejectedRequest(
					wh.validateCreate(GinkgoLogr, dryRun, cr),
					"invalid workloads node placement affinity:",
					`Unsupported value: "WrongOperator"`,
				)
			})

			It("should reject invalid workloads affinity: more than one value in matchFields", func(ctx context.Context) {
				cr.Spec.Infra.NodePlacement = &sdkapi.NodePlacement{
					Affinity: &corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									{
										MatchExpressions: []corev1.NodeSelectorRequirement{
											{
												Key:      "kubernetes.io/os",
												Operator: corev1.NodeSelectorOpIn,
												Values:   []string{"linux"},
											},
										},
									},
								},
							},
						},
					},
				}

				cr.Spec.Workloads.NodePlacement = &sdkapi.NodePlacement{
					Affinity: &corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									{
										MatchFields: []corev1.NodeSelectorRequirement{
											{
												Key:      "kubernetes.io/os",
												Operator: corev1.NodeSelectorOpIn,
												Values:   []string{"linux", "windows"},
											},
										},
									},
								},
							},
						},
					},
				}

				checkRejectedRequest(
					wh.validateCreate(GinkgoLogr, dryRun, cr),
					"invalid workloads node placement affinity:",
					"must have one element",
				)
			})

			It("should reject invalid infra affinity: unknown operator", func(ctx context.Context) {
				cr.Spec.Infra.NodePlacement = &sdkapi.NodePlacement{
					Affinity: &corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									{
										MatchExpressions: []corev1.NodeSelectorRequirement{
											{
												Key:      "kubernetes.io/os",
												Operator: "WrongOperator",
												Values:   []string{"linux"},
											},
										},
									},
								},
							},
						},
					},
				}

				cr.Spec.Workloads.NodePlacement = &sdkapi.NodePlacement{
					Affinity: &corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									{
										MatchExpressions: []corev1.NodeSelectorRequirement{
											{
												Key:      "kubernetes.io/os",
												Operator: corev1.NodeSelectorOpIn,
												Values:   []string{"linux"},
											},
										},
									},
								},
							},
						},
					},
				}

				checkRejectedRequest(
					wh.validateCreate(GinkgoLogr, dryRun, cr),
					"invalid infra node placement affinity:",
					`Unsupported value: "WrongOperator"`,
				)

			})

			It("should reject invalid infra affinity: more than one value in fieldSelector", func(ctx context.Context) {
				cr.Spec.Infra.NodePlacement = &sdkapi.NodePlacement{
					Affinity: &corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									{
										MatchFields: []corev1.NodeSelectorRequirement{
											{
												Key:      "kubernetes.io/os",
												Operator: corev1.NodeSelectorOpIn,
												Values:   []string{"linux", "windows"},
											},
										},
									},
								},
							},
						},
					},
				}

				cr.Spec.Workloads.NodePlacement = &sdkapi.NodePlacement{
					Affinity: &corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									{
										MatchExpressions: []corev1.NodeSelectorRequirement{
											{
												Key:      "kubernetes.io/os",
												Operator: corev1.NodeSelectorOpIn,
												Values:   []string{"linux"},
											},
										},
									},
								},
							},
						},
					},
				}

				checkRejectedRequest(
					wh.validateCreate(GinkgoLogr, dryRun, cr),
					"invalid infra node placement affinity:",
					"must have one element",
				)
			})
		})

		Context("validate tuning policy", func() {
			It("should return warning for deprecated highBurst tuning policy", func(ctx context.Context) {
				cr.Spec.TuningPolicy = hcov1.HyperConvergedHighBurstProfile //nolint SA1019
				resp := wh.validateCreate(GinkgoLogr, dryRun, cr)
				checkAcceptedRequest(resp, "the highBurst profile is deprecated")
			})

			It("should not return warning when tuning policy is not set", func(ctx context.Context) {
				cr.Spec.TuningPolicy = ""
				checkAcceptedRequest(wh.validateCreate(GinkgoLogr, dryRun, cr))
			})
		})
	})

	Context("validate update validation webhook", func() {

		var v1beta1CR *hcov1beta1.HyperConverged

		BeforeEach(func() {
			cr.Spec.Infra = hcov1.HyperConvergedConfig{
				NodePlacement: newHyperConvergedConfig(),
			}
			cr.Spec.Workloads = hcov1.HyperConvergedConfig{
				NodePlacement: newHyperConvergedConfig(),
			}

			v1beta1CR = &hcov1beta1.HyperConverged{}
			Expect(v1beta1CR.ConvertFrom(cr)).To(Succeed())

			cli = getFakeClient(cr)
		})

		It("should correctly handle a valid update request", func(ctx context.Context) {
			req := newRequest(admissionv1.Update, cr, hcoCodec, false)

			res := wh.Handle(ctx, req)
			Expect(res.Allowed).To(BeTrue())
			Expect(res.Result.Code).To(Equal(int32(200)))
		})

		It("should correctly handle a valid dryrun update request", func(ctx context.Context) {
			req := newRequest(admissionv1.Update, cr, hcoCodec, true)

			res := wh.Handle(ctx, req)
			Expect(res.Allowed).To(BeTrue())
			Expect(res.Result.Code).To(Equal(int32(200)))
		})

		It("should reject malformed update requests", func(ctx context.Context) {
			req := newRequest(admissionv1.Update, cr, hcoCodec, false)
			req.Object = runtime.RawExtension{}

			res := wh.Handle(ctx, req)
			Expect(res.Allowed).To(BeFalse())
			Expect(res.Result.Code).To(Equal(int32(400)))
			Expect(res.Result.Message).To(Equal("there is no content to decode"))

			req = newRequest(admissionv1.Update, cr, hcoCodec, false)
			req.OldObject = runtime.RawExtension{}

			res = wh.Handle(ctx, req)
			Expect(res.Allowed).To(BeFalse())
			Expect(res.Result.Code).To(Equal(int32(400)))
			Expect(res.Result.Message).To(Equal("there is no content to decode"))
		})

		It("should return error if KV CR is missing", func(ctx context.Context) {
			kv := handlers.NewKubeVirtWithNameOnly(v1beta1CR)
			Expect(cli.Delete(ctx, kv)).To(Succeed())

			tlssecprofile.SetHyperConvergedTLSSecurityProfile(nil)

			newHco := &hcov1.HyperConverged{}
			cr.DeepCopyInto(newHco)
			// just do some change to force update
			newHco.Spec.Infra.NodePlacement.NodeSelector["key3"] = "value3"

			checkRejectedRequest(
				wh.validateUpdate(ctx, GinkgoLogr, dryRun, newHco, cr),
				"kubevirts.kubevirt.io",
			)
		})

		It("should return error if dry-run update of KV CR returns error", func(ctx context.Context) {
			cli.(*commontestutils.HcoTestClient).InitiateUpdateErrors(getUpdateError(kvUpdateFailure))

			tlssecprofile.SetHyperConvergedTLSSecurityProfile(nil)

			newHco := &hcov1.HyperConverged{}
			cr.DeepCopyInto(newHco)
			// change something in workloads to trigger dry-run update
			newHco.Spec.Workloads.NodePlacement.NodeSelector["a change"] = "Something else"

			checkRejectedRequest(
				wh.validateUpdate(ctx, GinkgoLogr, dryRun, newHco, cr),
				ErrFakeKvError.Error(),
			)
		})

		It("should return error if CDI CR is missing", func(ctx context.Context) {
			cdi, err := handlers.NewCDI(v1beta1CR)
			Expect(err).ToNot(HaveOccurred())
			Expect(cli.Delete(ctx, cdi)).To(Succeed())

			tlssecprofile.SetHyperConvergedTLSSecurityProfile(nil)

			newHco := &hcov1.HyperConverged{}
			cr.DeepCopyInto(newHco)
			// just do some change to force update
			newHco.Spec.Infra.NodePlacement.NodeSelector["key3"] = "value3"

			checkRejectedRequest(
				wh.validateUpdate(ctx, GinkgoLogr, dryRun, newHco, cr),
				"cdis.cdi.kubevirt.io",
			)
		})

		It("should return error if dry-run update of CDI CR returns error", func(ctx context.Context) {
			cli.(*commontestutils.HcoTestClient).InitiateUpdateErrors(getUpdateError(cdiUpdateFailure))

			newHco := &hcov1.HyperConverged{}
			cr.DeepCopyInto(newHco)
			// change something in workloads to trigger dry-run update
			newHco.Spec.Workloads.NodePlacement.NodeSelector["a change"] = "Something else"

			checkRejectedRequest(
				wh.validateUpdate(ctx, GinkgoLogr, dryRun, newHco, cr),
				ErrFakeCdiError.Error(),
			)
		})

		It("should not return error if dry-run update of ALL CR passes", func(ctx context.Context) {
			cli.(*commontestutils.HcoTestClient).InitiateUpdateErrors(getUpdateError(noFailure))

			newHco := &hcov1.HyperConverged{}
			cr.DeepCopyInto(newHco)
			// change something in workloads to trigger dry-run update
			newHco.Spec.Workloads.NodePlacement.NodeSelector["a change"] = "Something else"

			checkAcceptedRequest(wh.validateUpdate(ctx, GinkgoLogr, dryRun, newHco, cr))
		})

		It("should return error if NetworkAddons CR is missing", func(ctx context.Context) {
			cna, err := handlers.NewNetworkAddons(v1beta1CR)
			Expect(err).ToNot(HaveOccurred())
			Expect(cli.Delete(ctx, cna)).To(Succeed())

			newHco := &hcov1.HyperConverged{}
			cr.DeepCopyInto(newHco)
			// just do some change to force update
			newHco.Spec.Infra.NodePlacement.NodeSelector["key3"] = "value3"

			checkRejectedRequest(
				wh.validateUpdate(ctx, GinkgoLogr, dryRun, newHco, cr),
				"networkaddonsconfigs.networkaddonsoperator.network.kubevirt.io",
			)
		})

		It("should return error if dry-run update of NetworkAddons CR returns error", func(ctx context.Context) {
			cli.(*commontestutils.HcoTestClient).InitiateUpdateErrors(getUpdateError(networkUpdateFailure))

			newHco := &hcov1.HyperConverged{}
			cr.DeepCopyInto(newHco)
			// change something in workloads to trigger dry-run update
			newHco.Spec.Workloads.NodePlacement.NodeSelector["a change"] = "Something else"

			resp := wh.validateUpdate(ctx, GinkgoLogr, dryRun, newHco, cr)
			Expect(resp.String()).To(ContainSubstring(ErrFakeNetworkError.Error()))
		})

		It("should return error if SSP CR is missing", func(ctx context.Context) {
			Expect(cli.Delete(ctx, handlers.NewSSPWithNameOnly(v1beta1CR))).To(Succeed())

			newHco := &hcov1.HyperConverged{}
			cr.DeepCopyInto(newHco)
			// just do some change to force update
			newHco.Spec.Infra.NodePlacement.NodeSelector["key3"] = "value3"

			checkRejectedRequest(
				wh.validateUpdate(ctx, GinkgoLogr, dryRun, newHco, cr),
				"ssps.ssp.kubevirt.io",
			)
		})

		It("should return error if dry-run update of SSP CR returns error", func(ctx context.Context) {
			cli.(*commontestutils.HcoTestClient).InitiateUpdateErrors(getUpdateError(sspUpdateFailure))

			newHco := &hcov1.HyperConverged{}
			cr.DeepCopyInto(newHco)
			// change something in workloads to trigger dry-run update
			newHco.Spec.Workloads.NodePlacement.NodeSelector["a change"] = "Something else"

			resp := wh.validateUpdate(ctx, GinkgoLogr, dryRun, newHco, cr)
			Expect(resp.String()).To(ContainSubstring(ErrFakeSspError.Error()))
		})

		It("should return error if dry-run update is timeout", func(ctx context.Context) {
			cli.(*commontestutils.HcoTestClient).InitiateUpdateErrors(initiateTimeout)

			newHco := &hcov1.HyperConverged{}
			cr.DeepCopyInto(newHco)
			// change something in workloads to trigger dry-run update
			newHco.Spec.Workloads.NodePlacement.NodeSelector["a change"] = "Something else"

			resp := wh.validateUpdate(ctx, GinkgoLogr, dryRun, newHco, cr)
			Expect(resp.String()).To(ContainSubstring(context.DeadlineExceeded.Error()))
		})

		It("should not return error if nothing was changed", func(ctx context.Context) {
			cli.(*commontestutils.HcoTestClient).InitiateUpdateErrors(initiateTimeout)

			newHco := &hcov1.HyperConverged{}
			cr.DeepCopyInto(newHco)

			checkAcceptedRequest(wh.validateUpdate(ctx, GinkgoLogr, dryRun, newHco, cr))
		})

		Context("test permitted host devices update validation", func() {
			It("should allow unique PCI Host Device", func(ctx context.Context) {
				newHco := &hcov1.HyperConverged{}
				cr.DeepCopyInto(newHco)
				newHco.Spec.PermittedHostDevices = &hcov1.PermittedHostDevices{
					PciHostDevices: []hcov1.PciHostDevice{
						{
							PCIDeviceSelector: "111",
							ResourceName:      "name",
						},
						{
							PCIDeviceSelector: "222",
							ResourceName:      "name",
						},
						{
							PCIDeviceSelector: "333",
							ResourceName:      "name",
						},
					},
				}
				checkAcceptedRequest(wh.validateUpdate(ctx, GinkgoLogr, dryRun, newHco, cr))
			})

			It("should allow unique Mediate Host Device", func(ctx context.Context) {
				newHco := &hcov1.HyperConverged{}
				cr.DeepCopyInto(newHco)
				newHco.Spec.PermittedHostDevices = &hcov1.PermittedHostDevices{
					MediatedDevices: []hcov1.MediatedHostDevice{
						{
							MDEVNameSelector: "111",
							ResourceName:     "name",
						},
						{
							MDEVNameSelector: "222",
							ResourceName:     "name",
						},
						{
							MDEVNameSelector: "333",
							ResourceName:     "name",
						},
					},
				}
				checkAcceptedRequest(wh.validateUpdate(ctx, GinkgoLogr, dryRun, newHco, cr))
			})
		})

		/*Context("plain-k8s tests", func() {
			It("should return error in plain-k8s if KV CR is missing", func(ctx context.Context) {
				hco := &hcov1.HyperConverged{}
				cli := getFakeClient(hco)
				kv, err := handlers.NewKubeVirt(hco)
				Expect(err).ToNot(HaveOccurred())
				Expect(cli.Delete(ctx, kv)).To(Succeed())
				wh := NewWebhookV1Beta1Handler(GinkgoLogr, cli, decoder, HcoValidNamespace, false)

				newHco := commontestutils.NewHco()
				newHco.Spec.Infra = hcov1.HyperConvergedConfig{
					NodePlacement: newHyperConvergedConfig(),
				}
				newHco.Spec.Workloads = hcov1.HyperConvergedConfig{
					NodePlacement: newHyperConvergedConfig(),
				}

				Expect(
					wh.ValidateUpdate(ctx, GinkgoLogr, dryRun, newHco, hco),
				).To(MatchError(apierrors.IsNotFound, "not found error"))
			})
		})

		Context("Check LiveMigrationConfiguration", func() {
			var hco *hcov1.HyperConverged

			BeforeEach(func() {
				hco = commontestutils.NewHco()
			})

			It("should ignore if there is no change in live migration", func(ctx context.Context) {
				cli := getFakeClient(hco)

				// Deleting KV here, in order to make sure the that the webhook does not find differences,
				// and so it exits with no error before finding that KV is not there.
				// Later we'll check that there is no error from the webhook, and that will prove that
				// the comparison works.
				kv := handlers.NewKubeVirtWithNameOnly(hco)
				Expect(cli.Delete(ctx, kv)).To(Succeed())

				wh := NewWebhookV1Beta1Handler(GinkgoLogr, cli, decoder, HcoValidNamespace, true)

				newHco := &hcov1.HyperConverged{}
				hco.DeepCopyInto(newHco)

				Expect(wh.ValidateUpdate(ctx, GinkgoLogr, dryRun, newHco, hco)).To(Succeed())
			})

			It("should allow updating of live migration", func(ctx context.Context) {
				cli := getFakeClient(hco)

				wh := NewWebhookV1Beta1Handler(GinkgoLogr, cli, decoder, HcoValidNamespace, true)

				newHco := &hcov1.HyperConverged{}
				hco.DeepCopyInto(newHco)

				// change something in the LiveMigrationConfig field
				hco.Spec.LiveMigrationConfig.CompletionTimeoutPerGiB = ptr.To[int64](200)

				Expect(wh.ValidateUpdate(ctx, GinkgoLogr, dryRun, newHco, hco)).To(Succeed())
			})

			It("should fail if live migration is wrong", func(ctx context.Context) {
				cli := getFakeClient(hco)

				wh := NewWebhookV1Beta1Handler(GinkgoLogr, cli, decoder, HcoValidNamespace, true)

				newHco := &hcov1.HyperConverged{}
				hco.DeepCopyInto(newHco)

				// change something in the LiveMigrationConfig field
				newHco.Spec.LiveMigrationConfig.BandwidthPerMigration = ptr.To("Wrong Value")

				Expect(
					wh.ValidateUpdate(ctx, GinkgoLogr, dryRun, newHco, hco),
				).To(MatchError(ContainSubstring("failed to parse the LiveMigrationConfig.bandwidthPerMigration field")))
			})
		})

		Context("Check CertRotation", func() {
			var hco *hcov1.HyperConverged

			BeforeEach(func() {
				hco = commontestutils.NewHco()
			})

			It("should ignore if there is no change in cert config", func(ctx context.Context) {
				cli := getFakeClient(hco)

				// Deleting KV here, in order to make sure the that the webhook does not find differences,
				// and so it exits with no error before finding that KV is not there.
				// Later we'll check that there is no error from the webhook, and that will prove that
				// the comparison works.
				kv := handlers.NewKubeVirtWithNameOnly(hco)
				Expect(cli.Delete(ctx, kv)).To(Succeed())

				wh := NewWebhookV1Beta1Handler(GinkgoLogr, cli, decoder, HcoValidNamespace, true)

				newHco := &hcov1.HyperConverged{}
				hco.DeepCopyInto(newHco)

				Expect(wh.ValidateUpdate(ctx, GinkgoLogr, dryRun, newHco, hco)).To(Succeed())
			})

			It("should allow updating of cert config", func(ctx context.Context) {
				cli := getFakeClient(hco)

				wh := NewWebhookV1Beta1Handler(GinkgoLogr, cli, decoder, HcoValidNamespace, true)

				newHco := &hcov1.HyperConverged{}
				hco.DeepCopyInto(newHco)

				// change something in the CertConfig fields
				newHco.Spec.CertConfig.CA.Duration.Duration = hco.Spec.CertConfig.CA.Duration.Duration * 2
				newHco.Spec.CertConfig.CA.RenewBefore.Duration = hco.Spec.CertConfig.CA.RenewBefore.Duration * 2
				newHco.Spec.CertConfig.Server.Duration.Duration = hco.Spec.CertConfig.Server.Duration.Duration * 2
				newHco.Spec.CertConfig.Server.RenewBefore.Duration = hco.Spec.CertConfig.Server.RenewBefore.Duration * 2

				Expect(wh.ValidateUpdate(ctx, GinkgoLogr, dryRun, newHco, hco)).To(Succeed())
			})

			DescribeTable("should fail if cert config is wrong",
				func(ctx context.Context, newHco hcov1.HyperConverged, errorMsg string) {
					cli := getFakeClient(hco)

					wh := NewWebhookV1Beta1Handler(GinkgoLogr, cli, decoder, HcoValidNamespace, true)

					err := wh.ValidateUpdate(ctx, GinkgoLogr, dryRun, &newHco, hco)
					Expect(err).To(MatchError(ContainSubstring(errorMsg)))
				},
				Entry("certConfig.ca.duration is too short",
					hcov1.HyperConverged{
						ObjectMeta: metav1.ObjectMeta{
							Name:      util.HyperConvergedName,
							Namespace: HcoValidNamespace,
						},
						Spec: hcov1.HyperConvergedSpec{
							CertConfig: hcov1.HyperConvergedCertConfig{
								CA: hcov1.CertRotateConfigCA{
									Duration:    &metav1.Duration{Duration: 8 * time.Minute},
									RenewBefore: &metav1.Duration{Duration: 24 * time.Hour},
								},
								Server: hcov1.CertRotateConfigServer{
									Duration:    &metav1.Duration{Duration: 24 * time.Hour},
									RenewBefore: &metav1.Duration{Duration: 12 * time.Hour},
								},
							},
						},
					},
					"spec.certConfig.ca.duration: value is too small"),
				Entry("certConfig.ca.renewBefore is too short",
					hcov1.HyperConverged{
						ObjectMeta: metav1.ObjectMeta{
							Name:      util.HyperConvergedName,
							Namespace: HcoValidNamespace,
						},
						Spec: hcov1.HyperConvergedSpec{
							CertConfig: hcov1.HyperConvergedCertConfig{
								CA: hcov1.CertRotateConfigCA{
									Duration:    &metav1.Duration{Duration: 48 * time.Hour},
									RenewBefore: &metav1.Duration{Duration: 8 * time.Minute},
								},
								Server: hcov1.CertRotateConfigServer{
									Duration:    &metav1.Duration{Duration: 24 * time.Hour},
									RenewBefore: &metav1.Duration{Duration: 12 * time.Hour},
								},
							},
						},
					},
					"spec.certConfig.ca.renewBefore: value is too small"),
				Entry("certConfig.server.duration is too short",
					hcov1.HyperConverged{
						ObjectMeta: metav1.ObjectMeta{
							Name:      util.HyperConvergedName,
							Namespace: HcoValidNamespace,
						},
						Spec: hcov1.HyperConvergedSpec{
							CertConfig: hcov1.HyperConvergedCertConfig{
								CA: hcov1.CertRotateConfigCA{
									Duration:    &metav1.Duration{Duration: 48 * time.Hour},
									RenewBefore: &metav1.Duration{Duration: 24 * time.Hour},
								},
								Server: hcov1.CertRotateConfigServer{
									Duration:    &metav1.Duration{Duration: 8 * time.Minute},
									RenewBefore: &metav1.Duration{Duration: 12 * time.Hour},
								},
							},
						},
					},
					"spec.certConfig.server.duration: value is too small"),
				Entry("certConfig.server.renewBefore is too short",
					hcov1.HyperConverged{
						ObjectMeta: metav1.ObjectMeta{
							Name:      util.HyperConvergedName,
							Namespace: HcoValidNamespace,
						},
						Spec: hcov1.HyperConvergedSpec{
							CertConfig: hcov1.HyperConvergedCertConfig{
								CA: hcov1.CertRotateConfigCA{
									Duration:    &metav1.Duration{Duration: 48 * time.Hour},
									RenewBefore: &metav1.Duration{Duration: 24 * time.Hour},
								},
								Server: hcov1.CertRotateConfigServer{
									Duration:    &metav1.Duration{Duration: 24 * time.Hour},
									RenewBefore: &metav1.Duration{Duration: 8 * time.Minute},
								},
							},
						},
					},
					"spec.certConfig.server.renewBefore: value is too small"),
				Entry("ca: duration is smaller than renewBefore",
					hcov1.HyperConverged{
						ObjectMeta: metav1.ObjectMeta{
							Name:      util.HyperConvergedName,
							Namespace: HcoValidNamespace,
						},
						Spec: hcov1.HyperConvergedSpec{
							CertConfig: hcov1.HyperConvergedCertConfig{
								CA: hcov1.CertRotateConfigCA{
									Duration:    &metav1.Duration{Duration: 23 * time.Hour},
									RenewBefore: &metav1.Duration{Duration: 24 * time.Hour},
								},
								Server: hcov1.CertRotateConfigServer{
									Duration:    &metav1.Duration{Duration: 24 * time.Hour},
									RenewBefore: &metav1.Duration{Duration: 12 * time.Hour},
								},
							},
						},
					},
					"spec.certConfig.ca: duration is smaller than renewBefore"),
				Entry("server: duration is smaller than renewBefore",
					hcov1.HyperConverged{
						ObjectMeta: metav1.ObjectMeta{
							Name:      util.HyperConvergedName,
							Namespace: HcoValidNamespace,
						},
						Spec: hcov1.HyperConvergedSpec{
							CertConfig: hcov1.HyperConvergedCertConfig{
								CA: hcov1.CertRotateConfigCA{
									Duration:    &metav1.Duration{Duration: 48 * time.Hour},
									RenewBefore: &metav1.Duration{Duration: 24 * time.Hour},
								},
								Server: hcov1.CertRotateConfigServer{
									Duration:    &metav1.Duration{Duration: 11 * time.Hour},
									RenewBefore: &metav1.Duration{Duration: 12 * time.Hour},
								},
							},
						},
					},
					"spec.certConfig.server: duration is smaller than renewBefore"),
				Entry("ca.duration is smaller than server.duration",
					hcov1.HyperConverged{
						ObjectMeta: metav1.ObjectMeta{
							Name:      util.HyperConvergedName,
							Namespace: HcoValidNamespace,
						},
						Spec: hcov1.HyperConvergedSpec{
							CertConfig: hcov1.HyperConvergedCertConfig{
								CA: hcov1.CertRotateConfigCA{
									Duration:    &metav1.Duration{Duration: 48 * time.Hour},
									RenewBefore: &metav1.Duration{Duration: 24 * time.Hour},
								},
								Server: hcov1.CertRotateConfigServer{
									Duration:    &metav1.Duration{Duration: 96 * time.Hour},
									RenewBefore: &metav1.Duration{Duration: 12 * time.Hour},
								},
							},
						},
					},
					"spec.certConfig: ca.duration is smaller than server.duration"),
			)

		})

		Context("validate tlsSecurityProfiles", func() {
			var hco *hcov1.HyperConverged

			BeforeEach(func() {
				hco = commontestutils.NewHco()
			})

			updateTLSSecurityProfile := func(ctx context.Context, minTLSVersion openshiftconfigv1.TLSProtocolVersion, ciphers []string) error {
				cli := getFakeClient(hco)

				wh := NewWebhookV1Beta1Handler(GinkgoLogr, cli, decoder, HcoValidNamespace, true)

				newHco := &hcov1.HyperConverged{}
				hco.DeepCopyInto(newHco)

				newHco.Spec.TLSSecurityProfile = &openshiftconfigv1.TLSSecurityProfile{
					Custom: &openshiftconfigv1.CustomTLSProfile{
						TLSProfileSpec: openshiftconfigv1.TLSProfileSpec{
							MinTLSVersion: minTLSVersion,
							Ciphers:       ciphers,
						},
					},
				}

				return wh.ValidateUpdate(ctx, GinkgoLogr, dryRun, newHco, hco)
			}

			DescribeTable("should succeed if has any of the HTTP/2-required ciphers",
				func(ctx context.Context, cipher string) {
					Expect(
						updateTLSSecurityProfile(ctx, openshiftconfigv1.VersionTLS12, []string{"DHE-RSA-AES256-GCM-SHA384", cipher, "DHE-RSA-CHACHA20-POLY1305"}),
					).To(Succeed())
				},
				Entry("ECDHE-RSA-AES128-GCM-SHA256", "ECDHE-RSA-AES128-GCM-SHA256"),
				Entry("ECDHE-ECDSA-AES128-GCM-SHA256", "ECDHE-ECDSA-AES128-GCM-SHA256"),
			)

			It("should fail if does not have any of the HTTP/2-required ciphers", func(ctx context.Context) {
				err := updateTLSSecurityProfile(ctx, openshiftconfigv1.VersionTLS12, []string{"DHE-RSA-AES256-GCM-SHA384", "DHE-RSA-CHACHA20-POLY1305"})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("http2: TLSConfig.CipherSuites is missing an HTTP/2-required AES_128_GCM_SHA256 cipher (need at least one of ECDHE-RSA-AES128-GCM-SHA256 or ECDHE-ECDSA-AES128-GCM-SHA256)"))
			})

			It("should succeed if does not have any of the HTTP/2-required ciphers but TLS version >= 1.3", func(ctx context.Context) {
				Expect(
					updateTLSSecurityProfile(ctx, openshiftconfigv1.VersionTLS13, []string{}),
				).To(Succeed())
			})

			It("should fail if does have custom ciphers with TLS version >= 1.3", func(ctx context.Context) {
				err := updateTLSSecurityProfile(ctx, openshiftconfigv1.VersionTLS13, []string{"TLS_AES_128_GCM_SHA256", "TLS_CHACHA20_POLY1305_SHA256"})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("custom ciphers cannot be selected when minTLSVersion is VersionTLS13"))
			})

			It("should fail when minTLSVersion is invalid", func(ctx context.Context) {
				err := updateTLSSecurityProfile(ctx, "invalidProtocolVersion", []string{"TLS_AES_128_GCM_SHA256", "TLS_CHACHA20_POLY1305_SHA256"})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid value for spec.tlsSecurityProfile.custom.minTLSVersion"))
			})

			It("should fail when type is Custom but custom field is nil", func(ctx context.Context) {
				cli := getFakeClient(hco)

				wh := NewWebhookV1Beta1Handler(GinkgoLogr, cli, decoder, HcoValidNamespace, true)

				newHco := hco.DeepCopy()
				newHco.Spec.TLSSecurityProfile = &openshiftconfigv1.TLSSecurityProfile{
					Type:   openshiftconfigv1.TLSProfileCustomType,
					Custom: nil,
				}

				Expect(wh.ValidateUpdate(ctx, GinkgoLogr, dryRun, newHco, hco)).To(MatchError(ContainSubstring("missing required field spec.tlsSecurityProfile.custom when type is Custom")))
			})
		})

		Context("validate deprecated FGs", func() {
			DescribeTable("should return warning for deprecated feature gate", func(ctx context.Context, fgs hcov1.HyperConvergedFeatureGates, fgNames ...string) {
				newHCO := cr.DeepCopy()
				newHCO.Spec.FeatureGates = fgs

				err := wh.ValidateUpdate(ctx, GinkgoLogr, dryRun, newHCO, cr)

				Expect(err).To(HaveOccurred())
				expected := &ValidationWarning{}
				Expect(errors.As(err, &expected)).To(BeTrue())

				Expect(expected.warnings).To(HaveLen(len(fgNames)))
				for _, fgName := range fgNames {
					Expect(expected.warnings).To(ContainElements(ContainSubstring(fgName)))
				}
			},
				Entry("should trigger a warning if the withHostPassthroughCPU=false FG exists in the CR",
					hcov1.HyperConvergedFeatureGates{WithHostPassthroughCPU: ptr.To(false)}, "withHostPassthroughCPU"),
				Entry("should trigger a warning if the withHostPassthroughCPU=true FG exists in the CR",
					hcov1.HyperConvergedFeatureGates{WithHostPassthroughCPU: ptr.To(true)}, "withHostPassthroughCPU"),

				Entry("should trigger a warning if the deployTektonTaskResources=false FG exists in the CR",
					hcov1.HyperConvergedFeatureGates{DeployTektonTaskResources: ptr.To(false)}, "deployTektonTaskResources"),
				Entry("should trigger a warning if the deployTektonTaskResources=true FG exists in the CR",
					hcov1.HyperConvergedFeatureGates{DeployTektonTaskResources: ptr.To(true)}, "deployTektonTaskResources"),

				Entry("should trigger a warning if the enableManagedTenantQuota=false FG exists in the CR",
					hcov1.HyperConvergedFeatureGates{EnableManagedTenantQuota: ptr.To(false)}, "enableManagedTenantQuota"),
				Entry("should trigger a warning if the enableManagedTenantQuota=true FG exists in the CR",
					hcov1.HyperConvergedFeatureGates{EnableManagedTenantQuota: ptr.To(true)}, "enableManagedTenantQuota"),

				Entry("should trigger a warning if the nonRoot=false FG exists in the CR",
					hcov1.HyperConvergedFeatureGates{NonRoot: ptr.To(false)}, "nonRoot"),
				Entry("should trigger a warning if the nonRoot=true FG exists in the CR",
					hcov1.HyperConvergedFeatureGates{NonRoot: ptr.To(true)}, "nonRoot"),

				Entry("should trigger multiple warnings if several deprecated FG exist in the CR",
					hcov1.HyperConvergedFeatureGates{
						NonRoot:                  ptr.To(true),
						EnableManagedTenantQuota: ptr.To(true),
					}, "enableManagedTenantQuota", "nonRoot"),

				Entry("should trigger multiple warnings if several deprecated FG exist in the CR, with some valid FGs",
					hcov1.HyperConvergedFeatureGates{
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

				err := wh.ValidateUpdate(ctx, GinkgoLogr, dryRun, newHCO, cr)

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
				cli := getFakeClient(cr)
				wh := NewWebhookV1Beta1Handler(GinkgoLogr, cli, decoder, HcoValidNamespace, true)
				newHCO := cr.DeepCopy()
				cr.Spec.FeatureGates.EnableApplicationAwareQuota = newFG
				newHCO.Spec.FeatureGates.EnableApplicationAwareQuota = oldFG

				Expect(wh.ValidateUpdate(ctx, GinkgoLogr, dryRun, newHCO, cr)).To(Succeed())
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

				err := wh.ValidateUpdate(ctx, GinkgoLogr, dryRun, newHCO, cr)

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
				cli := getFakeClient(cr)
				wh := NewWebhookV1Beta1Handler(GinkgoLogr, cli, decoder, HcoValidNamespace, true)
				newHCO := cr.DeepCopy()
				cr.Spec.FeatureGates.EnableCommonBootImageImport = newFG
				newHCO.Spec.FeatureGates.EnableCommonBootImageImport = oldFG

				Expect(wh.ValidateUpdate(ctx, GinkgoLogr, dryRun, newHCO, cr)).To(Succeed())
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

				err := wh.ValidateUpdate(ctx, GinkgoLogr, dryRun, newHCO, cr)

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
				cli := getFakeClient(cr)
				wh := NewWebhookV1Beta1Handler(GinkgoLogr, cli, decoder, HcoValidNamespace, true)
				newHCO := cr.DeepCopy()
				cr.Spec.FeatureGates.DeployVMConsoleProxy = newFG
				newHCO.Spec.FeatureGates.DeployVMConsoleProxy = oldFG

				Expect(wh.ValidateUpdate(ctx, GinkgoLogr, dryRun, newHCO, cr)).To(Succeed())
			},
				Entry("should not trigger warning if deployVmConsoleProxy (true) disappeared", ptr.To(true), nil),
				Entry("should not trigger warning if deployVmConsoleProxy (false) disappeared", ptr.To(false), nil),
				Entry("should not trigger warning if deployVmConsoleProxy (true) wasn't changed", ptr.To(true), ptr.To(true)),
				Entry("should not trigger warning if deployVmConsoleProxy (false) wasn't changed", ptr.To(false), ptr.To(false)),
			)

			//nolint:staticcheck
			DescribeTable("should not return warning for deployKubeSecondaryDNS if not change", func(ctx context.Context, newFG, oldFG *bool) {
				cli := getFakeClient(cr)
				wh := NewWebhookV1Beta1Handler(GinkgoLogr, cli, decoder, HcoValidNamespace, true)
				newHCO := cr.DeepCopy()
				cr.Spec.FeatureGates.DeployKubeSecondaryDNS = newFG
				newHCO.Spec.FeatureGates.DeployKubeSecondaryDNS = oldFG

				Expect(wh.ValidateUpdate(ctx, GinkgoLogr, dryRun, newHCO, cr)).To(Succeed())
			},
				Entry("should not trigger warning if deployKubeSecondaryDNS (true) disappeared", ptr.To(true), nil),
				Entry("should not trigger warning if deployKubeSecondaryDNS (false) disappeared", ptr.To(false), nil),
				Entry("should not trigger warning if deployKubeSecondaryDNS (true) wasn't changed", ptr.To(true), ptr.To(true)),
				Entry("should not trigger warning if deployKubeSecondaryDNS (false) wasn't changed", ptr.To(false), ptr.To(false)),
			)
		})

		Context("validate tuning policy on update", func() {
			It("should return warning for deprecated highBurst tuning policy", func(ctx context.Context) {
				newHCO := cr.DeepCopy()
				newHCO.Spec.TuningPolicy = hcov1.HyperConvergedHighBurstProfile //nolint SA1019
				err := wh.ValidateUpdate(ctx, GinkgoLogr, dryRun, newHCO, cr)
				Expect(err).To(HaveOccurred())
				expected := &ValidationWarning{}
				Expect(errors.As(err, &expected)).To(BeTrue())
				Expect(expected.warnings).To(HaveLen(1))
				Expect(expected.warnings[0]).To(ContainSubstring("highBurst profile is deprecated"))
				Expect(expected.warnings[0]).To(ContainSubstring("v1.16.0"))
			})

			It("should not return warning when tuning policy is not set", func(ctx context.Context) {
				newHCO := cr.DeepCopy()
				newHCO.Spec.TuningPolicy = ""
				Expect(wh.ValidateUpdate(ctx, GinkgoLogr, dryRun, newHCO, cr)).To(Succeed())
			})
		})*/
	})

	Context("validate delete validation webhook", func() {
		var v1Beta1CR *hcov1beta1.HyperConverged

		BeforeEach(func() {
			v1Beta1CR = &hcov1beta1.HyperConverged{}
			Expect(v1Beta1CR.ConvertFrom(cr)).To(Succeed())

			cli = getFakeClient(cr)
		})

		It("should correctly handle a valid delete request", func(ctx context.Context) {
			req := newRequest(admissionv1.Delete, cr, hcoCodec, false)

			res := wh.Handle(ctx, req)
			Expect(res.Allowed).To(BeTrue())
			Expect(res.Result.Code).To(Equal(int32(200)))
		})

		It("should correctly handle a valid dryrun delete request", func(ctx context.Context) {
			req := newRequest(admissionv1.Delete, cr, hcoCodec, true)

			res := wh.Handle(ctx, req)
			Expect(res.Allowed).To(BeTrue())
			Expect(res.Result.Code).To(Equal(int32(200)))
		})

		It("should reject a malformed delete request", func(ctx context.Context) {
			req := newRequest(admissionv1.Delete, cr, hcoCodec, false)
			req.OldObject = req.Object
			req.Object = runtime.RawExtension{}

			res := wh.Handle(ctx, req)
			Expect(res.Allowed).To(BeFalse())
			Expect(res.Result.Code).To(Equal(int32(400)))
			Expect(res.Result.Message).To(Equal("there is no content to decode"))
		})

		It("should validate deletion", func(ctx context.Context) {
			req := newRequest(admissionv1.Delete, cr, hcoCodec, true)

			checkAcceptedRequest(wh.Handle(ctx, req))

			By("Validate that KV still exists, as it a dry-run deletion")
			kv := handlers.NewKubeVirtWithNameOnly(v1Beta1CR)
			Expect(util.GetRuntimeObject(ctx, cli, kv)).To(Succeed())

			By("Validate that CDI still exists, as it a dry-run deletion")
			cdi := handlers.NewCDIWithNameOnly(v1Beta1CR)
			Expect(util.GetRuntimeObject(ctx, cli, cdi)).To(Succeed())
		})

		It("should reject if KV deletion fails", func(ctx context.Context) {
			cli.(*commontestutils.HcoTestClient).InitiateDeleteErrors(func(obj client.Object) error {
				if unstructed, ok := obj.(runtime.Unstructured); ok {
					kind := unstructed.GetObjectKind()
					if kind.GroupVersionKind().Kind == "KubeVirt" {
						return ErrFakeKvError
					}
				}
				return nil
			})

			req := newRequest(admissionv1.Delete, cr, hcoCodec, true)
			checkRejectedRequest(wh.Handle(ctx, req), ErrFakeKvError.Error())
		})

		It("should reject if CDI deletion fails", func(ctx context.Context) {
			cli.(*commontestutils.HcoTestClient).InitiateDeleteErrors(func(obj client.Object) error {
				if unstructed, ok := obj.(runtime.Unstructured); ok {
					kind := unstructed.GetObjectKind()
					if kind.GroupVersionKind().Kind == "CDI" {
						return ErrFakeCdiError
					}
				}
				return nil
			})

			req := newRequest(admissionv1.Delete, cr, hcoCodec, true)
			checkRejectedRequest(wh.Handle(ctx, req), ErrFakeCdiError.Error())
		})

		It("should ignore if KV does not exist", func(ctx context.Context) {
			kv := handlers.NewKubeVirtWithNameOnly(v1Beta1CR)
			Expect(cli.Delete(ctx, kv)).To(Succeed())

			req := newRequest(admissionv1.Delete, cr, hcoCodec, false)
			checkAcceptedRequest(wh.Handle(ctx, req))
		})

		It("should reject if getting KV failed for not-not-exists error", func(ctx context.Context) {
			cli.(*commontestutils.HcoTestClient).InitiateGetErrors(func(key client.ObjectKey) error {
				if key.Name == "kubevirt-kubevirt-hyperconverged" {
					return ErrFakeKvError
				}
				return nil
			})

			req := newRequest(admissionv1.Delete, cr, hcoCodec, true)
			checkRejectedRequest(wh.Handle(ctx, req), ErrFakeKvError.Error())
		})

		It("should ignore if CDI does not exist", func(ctx context.Context) {
			cdi := handlers.NewCDIWithNameOnly(v1Beta1CR)
			Expect(cli.Delete(ctx, cdi)).To(Succeed())

			req := newRequest(admissionv1.Delete, cr, hcoCodec, false)
			checkAcceptedRequest(wh.Handle(ctx, req))
		})

		It("should reject if getting CDI failed for not-not-exists error", func(ctx context.Context) {
			cli.(*commontestutils.HcoTestClient).InitiateGetErrors(func(key client.ObjectKey) error {
				if key.Name == "cdi-kubevirt-hyperconverged" {
					return ErrFakeCdiError
				}
				return nil
			})

			req := newRequest(admissionv1.Delete, cr, hcoCodec, true)
			checkRejectedRequest(wh.Handle(ctx, req), ErrFakeCdiError.Error())
		})
	})

	Context("unsupported annotation", func() {

		BeforeEach(func() {
			cli = getFakeClient(cr)
		})

		DescribeTable("should accept if annotation is valid",
			func(ctx context.Context, annotationName, annotation string) {
				newHco := &hcov1.HyperConverged{}
				cr.DeepCopyInto(newHco)
				cr.Annotations = map[string]string{annotationName: annotation}

				checkAcceptedRequest(wh.validateUpdate(ctx, GinkgoLogr, dryRun, newHco, cr))
			},
			Entry("should accept if kv annotation is valid", common.JSONPatchKVAnnotationName, validKvAnnotation),
			Entry("should accept if cdi annotation is valid", common.JSONPatchCDIAnnotationName, validCdiAnnotation),
			Entry("should accept if cna annotation is valid", common.JSONPatchCNAOAnnotationName, validCnaAnnotation),
			Entry("should accept if ssp annotation is valid", common.JSONPatchSSPAnnotationName, validSspAnnotation),
		)

		DescribeTable("should reject if annotation is invalid",
			func(ctx context.Context, annotationName, annotation string) {
				cli := getFakeClient(cr)
				cli.InitiateUpdateErrors(initiateTimeout)

				newHco := &hcov1.HyperConverged{}
				cr.DeepCopyInto(newHco)
				newHco.Annotations = map[string]string{annotationName: annotation}

				checkRejectedRequest(
					wh.validateUpdate(ctx, GinkgoLogr, false, newHco, cr),
					"invalid jsonPatch in the "+annotationName,
				)
			},
			Entry("should reject if kv annotation is invalid", common.JSONPatchKVAnnotationName, invalidKvAnnotation),
			Entry("should reject if cdi annotation is invalid", common.JSONPatchCDIAnnotationName, invalidCdiAnnotation),
			Entry("should reject if cna annotation is invalid", common.JSONPatchCNAOAnnotationName, invalidCnaAnnotation),
			Entry("should accept if ssp annotation is invalid", common.JSONPatchSSPAnnotationName, invalidSspAnnotation),
		)
	})

	Context("hcoTLSConfigCache", func() {
		var hcoTLSConfigCache *openshiftconfigv1.TLSSecurityProfile

		intermediateTLSSecurityProfile := openshiftconfigv1.TLSSecurityProfile{
			Type:         openshiftconfigv1.TLSProfileIntermediateType,
			Intermediate: &openshiftconfigv1.IntermediateTLSProfile{},
		}
		initialTLSSecurityProfile := intermediateTLSSecurityProfile
		oldTLSSecurityProfile := openshiftconfigv1.TLSSecurityProfile{
			Type: openshiftconfigv1.TLSProfileOldType,
			Old:  &openshiftconfigv1.OldTLSProfile{},
		}
		modernTLSSecurityProfile := openshiftconfigv1.TLSSecurityProfile{
			Type:   openshiftconfigv1.TLSProfileModernType,
			Modern: &openshiftconfigv1.ModernTLSProfile{},
		}

		BeforeEach(func() {
			origSetHyperConvergedProfile := tlssecprofile.SetHyperConvergedTLSSecurityProfile

			tlssecprofile.SetHyperConvergedTLSSecurityProfile = func(hcoTLSSecurityProfile *openshiftconfigv1.TLSSecurityProfile) {
				hcoTLSConfigCache = hcoTLSSecurityProfile
			}

			tlssecprofile.SetHyperConvergedTLSSecurityProfile(&initialTLSSecurityProfile)

			DeferCleanup(func() {
				tlssecprofile.SetHyperConvergedTLSSecurityProfile = origSetHyperConvergedProfile
			})
		})

		Context("create", func() {

			It("should update hcoTLSConfigCache creating a resource not in dry run mode", func(ctx context.Context) {
				Expect(hcoTLSConfigCache).To(Equal(&initialTLSSecurityProfile))
				cr.Spec.TLSSecurityProfile = &modernTLSSecurityProfile
				checkAcceptedRequest(wh.validateCreate(GinkgoLogr, false, cr))
				Expect(hcoTLSConfigCache).To(Equal(&modernTLSSecurityProfile))
			})

			It("should not update hcoTLSConfigCache creating a resource in dry run mode", func(ctx context.Context) {
				Expect(hcoTLSConfigCache).To(Equal(&initialTLSSecurityProfile))
				cr.Spec.TLSSecurityProfile = &modernTLSSecurityProfile
				checkAcceptedRequest(wh.validateCreate(GinkgoLogr, true, cr))
				Expect(hcoTLSConfigCache).ToNot(Equal(&modernTLSSecurityProfile))
			})

			It("should not update hcoTLSConfigCache if the create request is refused", func(ctx context.Context) {
				Expect(hcoTLSConfigCache).To(Equal(&initialTLSSecurityProfile))
				cr.Spec.TLSSecurityProfile = &modernTLSSecurityProfile
				cr.Namespace = ResourceInvalidNamespace

				cr.Spec.DataImportCronTemplates = []hcov1.DataImportCronTemplate{
					{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{
								util.DataImportCronEnabledAnnotation: "a-non-boolean-value",
							},
						},
					},
				}

				checkRejectedRequest(wh.validateCreate(GinkgoLogr, false, cr))
				Expect(hcoTLSConfigCache).To(Equal(&initialTLSSecurityProfile))
			})
		})

		Context("update", func() {

			It("should update hcoTLSConfigCache updating a resource not in dry run mode", func(ctx context.Context) {
				cli := getFakeClient(cr)
				cli.InitiateUpdateErrors(getUpdateError(noFailure))

				tlssecprofile.SetHyperConvergedTLSSecurityProfile(nil)

				newCr := &hcov1.HyperConverged{}
				cr.DeepCopyInto(newCr)
				newCr.Spec.TLSSecurityProfile = &oldTLSSecurityProfile

				checkAcceptedRequest(wh.validateUpdate(ctx, GinkgoLogr, false, newCr, cr))
				Expect(hcoTLSConfigCache).To(Equal(&oldTLSSecurityProfile))
			})

			It("should not update hcoTLSConfigCache updating a resource in dry run mode", func(ctx context.Context) {
				cli := getFakeClient(cr)
				cli.InitiateUpdateErrors(getUpdateError(noFailure))

				tlssecprofile.SetHyperConvergedTLSSecurityProfile(&initialTLSSecurityProfile)

				newCr := &hcov1.HyperConverged{}
				cr.DeepCopyInto(newCr)
				newCr.Spec.TLSSecurityProfile = &oldTLSSecurityProfile

				checkAcceptedRequest(wh.validateUpdate(ctx, GinkgoLogr, true, newCr, cr))
				Expect(hcoTLSConfigCache).To(Equal(&initialTLSSecurityProfile))
			})

			It("should not update hcoTLSConfigCache if the update request is refused", func(ctx context.Context) {
				cli.(*commontestutils.HcoTestClient).InitiateUpdateErrors(getUpdateError(cdiUpdateFailure))

				tlssecprofile.SetHyperConvergedTLSSecurityProfile(&initialTLSSecurityProfile)
				newCr := &hcov1.HyperConverged{}
				cr.DeepCopyInto(newCr)
				newCr.Spec.TLSSecurityProfile = &oldTLSSecurityProfile

				checkRejectedRequest(wh.validateUpdate(ctx, GinkgoLogr, false, newCr, cr))
				Expect(hcoTLSConfigCache).To(Equal(&initialTLSSecurityProfile))
			})
		})
	})
})

func checkAcceptedRequest(resp admission.Response, warnings ...string) {
	GinkgoHelper()

	Expect(resp.Allowed).To(BeTrueBecause("should accept the request; %v", resp))

	Expect(resp.Warnings).To(HaveLen(len(warnings)))
	for _, warning := range warnings {
		Expect(resp.Warnings).To(ContainElement(ContainSubstring(warning)))
	}
}

func checkRejectedRequest(resp admission.Response, reasons ...string) {
	GinkgoHelper()

	Expect(resp.Allowed).To(BeFalseBecause("should reject the request; %v", resp))

	for _, reason := range reasons {
		Expect(resp.Result.Message).To(ContainSubstring(reason))
	}
}
