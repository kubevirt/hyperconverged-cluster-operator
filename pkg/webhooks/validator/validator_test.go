package validator

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	openshiftconfigv1 "github.com/openshift/api/config/v1"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
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
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

const (
	ResourceInvalidNamespace = "an-arbitrary-namespace"
	HcoValidNamespace        = "kubevirt-hyperconverged"
)

var (
	logger = zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)).WithName("hyperconverged-resource")
)

const (
	validKvAnnotation = `[
					{
						"op": "add",
						"path": "/spec/configuration/cpuRequest",
						"value": "12m"
					},
					{
						"op": "add",
						"path": "/spec/configuration/developerConfiguration",
						"value": {"featureGates": ["fg1"]}
					},
					{
						"op": "add",
						"path": "/spec/configuration/developerConfiguration/featureGates/-",
						"value": "fg2"
					}
			]`
	validCdiAnnotation = `[
				{
					"op": "add",
					"path": "/spec/config/featureGates/-",
					"value": "fg1"
				},
				{
					"op": "add",
					"path": "/spec/config/filesystemOverhead",
					"value": {"global": "50", "storageClass": {"AAA": "75", "BBB": "25"}}
				}
			]`
	validCnaAnnotation = `[
					{
						"op": "add",
						"path": "/spec/kubeMacPool",
						"value": {"rangeStart": "1.1.1.1.1.1", "rangeEnd": "5.5.5.5.5.5" }
					},
					{
						"op": "add",
						"path": "/spec/imagePullPolicy",
						"value": "Always"
					}
			]`
	validSspAnnotation = `[
					{
						"op": "replace",
						"path": "/spec/templateValidator/replicas",
						"value": 5
					}
			]`
	invalidKvAnnotation  = `[{"op": "wrongOp", "path": "/spec/configuration/cpuRequest", "value": "12m"}]`
	invalidCdiAnnotation = `[{"op": "wrongOp", "path": "/spec/config/featureGates/-", "value": "fg1"}]`
	invalidCnaAnnotation = `[{"op": "wrongOp", "path": "/spec/kubeMacPool", "value": {"rangeStart": "1.1.1.1.1.1", "rangeEnd": "5.5.5.5.5.5" }}]`
	invalidSspAnnotation = `[{"op": "wrongOp", "path": "/spec/templateValidator/replicas", "value": 5}]`
)

var _ = Describe("v1 webhooks validator", func() {
	s := scheme.Scheme
	for _, f := range []func(*runtime.Scheme) error{
		hcov1.AddToScheme,
		cdiv1beta1.AddToScheme,
		kubevirtcorev1.AddToScheme,
		networkaddonsv1.AddToScheme,
		sspv1beta3.AddToScheme,
	} {
		Expect(f(s)).To(Succeed())
	}

	codecFactory := serializer.NewCodecFactory(s)
	v1Codec := codecFactory.LegacyCodec(hcov1.SchemeGroupVersion)

	cli := fake.NewClientBuilder().WithScheme(s).Build()
	decoder := admission.NewDecoder(s)

	wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true, nil)

	var dryRun bool
	BeforeEach(func() {
		Expect(os.Setenv("OPERATOR_NAMESPACE", HcoValidNamespace)).To(Succeed())
		dryRun = false
	})

	Context("Check create validation webhook", func() {
		Context("check create request", func() {
			var v1cr *hcov1.HyperConverged

			BeforeEach(func() {
				v1cr = commontestutils.NewV1HCO()
			})

			It("should correctly handle a valid creation request", func(ctx context.Context) {
				ctx = logr.NewContext(ctx, GinkgoLogr)
				req := newRequest(admissionv1.Create, v1cr, v1Codec, false)

				res := wh.Handle(ctx, req)
				Expect(res.Allowed).To(BeTrue())
				Expect(res.Result.Code).To(Equal(int32(200)))
			})

			It("should correctly handle a valid dryrun creation request", func(ctx context.Context) {
				req := newRequest(admissionv1.Create, v1cr, v1Codec, true)

				res := wh.Handle(ctx, req)
				Expect(res.Allowed).To(BeTrue())
				Expect(res.Result.Code).To(Equal(int32(200)))
			})

			It("should reject malformed creation requests", func(ctx context.Context) {
				ctx = logr.NewContext(ctx, GinkgoLogr)
				req := newRequest(admissionv1.Create, v1cr, v1Codec, false)
				req.OldObject = req.Object
				req.Object = runtime.RawExtension{}

				res := wh.Handle(ctx, req)
				Expect(res.Allowed).To(BeFalse())
				Expect(res.Result.Code).To(Equal(int32(400)))
				Expect(res.Result.Message).To(Equal(decodeErrorMsg))

				req = newRequest(admissionv1.Create, v1cr, v1Codec, false)
				req.Operation = "MALFORMED"

				res = wh.Handle(ctx, req)
				Expect(res.Allowed).To(BeFalse())
				Expect(res.Result.Code).To(Equal(int32(400)))
				Expect(res.Result.Message).To(Equal("unknown operation request \"MALFORMED\""))
			})
		})

		Context("check ValidateCreate", func() {
			var cr *hcov1beta1.HyperConverged
			BeforeEach(func() {
				cr = commontestutils.NewHco()
			})

			It("should accept creation of a resource with a valid namespace", func(ctx context.Context) {
				Expect(wh.ValidateCreate(ctx, dryRun, cr)).To(Succeed())
			})

			DescribeTable("Validate annotations", func(ctx context.Context, annotations map[string]string, assertion types.GomegaMatcher) {
				cr.Annotations = annotations
				Expect(wh.ValidateCreate(ctx, dryRun, cr)).To(assertion)
			},
				Entry("should accept creation of a resource with a valid kv annotation",
					map[string]string{common.JSONPatchKVAnnotationName: validKvAnnotation},
					Succeed(),
				),
				Entry("should reject creation of a resource with an invalid kv annotation",
					map[string]string{common.JSONPatchKVAnnotationName: invalidKvAnnotation},
					Not(Succeed()),
				),
				Entry("should accept creation of a resource with a valid cdi annotation",
					map[string]string{common.JSONPatchCDIAnnotationName: validCdiAnnotation},
					Succeed(),
				),
				Entry("should reject creation of a resource with an invalid cdi annotation",
					map[string]string{common.JSONPatchCDIAnnotationName: invalidCdiAnnotation},
					Not(Succeed()),
				),
				Entry("should accept creation of a resource with a valid cna annotation",
					map[string]string{common.JSONPatchCNAOAnnotationName: validCnaAnnotation},
					Succeed(),
				),
				Entry("should reject creation of a resource with an invalid cna annotation",
					map[string]string{common.JSONPatchCNAOAnnotationName: invalidCnaAnnotation},
					Not(Succeed()),
				),
				Entry("should accept creation of a resource with a valid ssp annotation",
					map[string]string{common.JSONPatchSSPAnnotationName: validSspAnnotation},
					Succeed(),
				),
				Entry("should reject creation of a resource with an invalid ssp annotation",
					map[string]string{common.JSONPatchSSPAnnotationName: invalidSspAnnotation},
					Not(Succeed()),
				),
			)

			Context("test permitted host devices validation", func() {
				It("should allow unique PCI Host Device", func(ctx context.Context) {
					cr.Spec.PermittedHostDevices = &hcov1beta1.PermittedHostDevices{
						PciHostDevices: []hcov1beta1.PciHostDevice{
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
					Expect(wh.ValidateCreate(ctx, dryRun, cr)).To(Succeed())
				})

				It("should allow unique Mediate Host Device", func(ctx context.Context) {
					cr.Spec.PermittedHostDevices = &hcov1beta1.PermittedHostDevices{
						MediatedDevices: []hcov1beta1.MediatedHostDevice{
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
					Expect(wh.ValidateCreate(ctx, dryRun, cr)).To(Succeed())
				})
			})

			Context("Test DataImportCronTemplates", func() {
				var image1, image2, image3, image4 hcov1beta1.DataImportCronTemplate

				BeforeEach(func() {
					dryRun = false

					image1 = hcov1beta1.DataImportCronTemplate{
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

					image2 = hcov1beta1.DataImportCronTemplate{
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

					image3 = hcov1beta1.DataImportCronTemplate{
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

					image4 = hcov1beta1.DataImportCronTemplate{
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

					cr.Spec.DataImportCronTemplates = []hcov1beta1.DataImportCronTemplate{image1, image2, image3, image4}
				})

				It("should allow setting the annotation to true", func(ctx context.Context) {
					cr.Spec.DataImportCronTemplates[0].Annotations = map[string]string{util.DataImportCronEnabledAnnotation: "true"}
					cr.Spec.DataImportCronTemplates[1].Annotations = map[string]string{util.DataImportCronEnabledAnnotation: "TRUE"}
					cr.Spec.DataImportCronTemplates[2].Annotations = map[string]string{util.DataImportCronEnabledAnnotation: "TrUe"}
					cr.Spec.DataImportCronTemplates[3].Annotations = map[string]string{util.DataImportCronEnabledAnnotation: "tRuE"}

					Expect(wh.ValidateCreate(ctx, dryRun, cr)).To(Succeed())
				})

				It("should allow setting the annotation to false", func(ctx context.Context) {
					cr.Spec.DataImportCronTemplates[0].Annotations = map[string]string{util.DataImportCronEnabledAnnotation: "false"}
					cr.Spec.DataImportCronTemplates[1].Annotations = map[string]string{util.DataImportCronEnabledAnnotation: "FALSE"}
					cr.Spec.DataImportCronTemplates[2].Annotations = map[string]string{util.DataImportCronEnabledAnnotation: "FaLsE"}
					cr.Spec.DataImportCronTemplates[3].Annotations = map[string]string{util.DataImportCronEnabledAnnotation: "fAlSe"}

					Expect(wh.ValidateCreate(ctx, dryRun, cr)).To(Succeed())
				})

				It("should allow setting no annotation", func(ctx context.Context) {
					Expect(wh.ValidateCreate(ctx, dryRun, cr)).To(Succeed())
				})

				It("should not allow empty annotation", func(ctx context.Context) {
					cr.Spec.DataImportCronTemplates[0].Annotations = map[string]string{util.DataImportCronEnabledAnnotation: ""}
					cr.Spec.DataImportCronTemplates[1].Annotations = map[string]string{util.DataImportCronEnabledAnnotation: ""}

					Expect(wh.ValidateCreate(ctx, dryRun, cr)).ToNot(Succeed())
				})

				It("should not allow unknown annotation values", func(ctx context.Context) {
					cr.Spec.DataImportCronTemplates[0].Annotations = map[string]string{util.DataImportCronEnabledAnnotation: "wrong"}
					cr.Spec.DataImportCronTemplates[1].Annotations = map[string]string{util.DataImportCronEnabledAnnotation: "mistake"}

					Expect(wh.ValidateCreate(ctx, dryRun, cr)).ToNot(Succeed())
				})

				Context("Empty DICT spec", func() {
					It("don't allow if the annotation does not exist", func(ctx context.Context) {
						// empty annotation map
						cr.Spec.DataImportCronTemplates[0].Annotations = map[string]string{}
						cr.Spec.DataImportCronTemplates[0].Spec = nil
						// no annotation map
						cr.Spec.DataImportCronTemplates[1].Spec = nil

						Expect(wh.ValidateCreate(ctx, dryRun, cr)).ToNot(Succeed())
					})

					It("don't allow if the annotation is true", func(ctx context.Context) {
						cr.Spec.DataImportCronTemplates[0].Annotations = map[string]string{util.DataImportCronEnabledAnnotation: "True"}
						cr.Spec.DataImportCronTemplates[0].Spec = nil
						cr.Spec.DataImportCronTemplates[1].Annotations = map[string]string{util.DataImportCronEnabledAnnotation: "true"}
						cr.Spec.DataImportCronTemplates[1].Spec = nil

						Expect(wh.ValidateCreate(ctx, dryRun, cr)).ToNot(Succeed())
					})

					It("allow if the annotation is false", func(ctx context.Context) {
						cr.Spec.DataImportCronTemplates[0].Annotations = map[string]string{util.DataImportCronEnabledAnnotation: "False"}
						cr.Spec.DataImportCronTemplates[0].Spec = nil
						cr.Spec.DataImportCronTemplates[1].Annotations = map[string]string{util.DataImportCronEnabledAnnotation: "false"}
						cr.Spec.DataImportCronTemplates[1].Spec = nil

						Expect(wh.ValidateCreate(ctx, dryRun, cr)).To(Succeed())
					})
				})
			})

			Context("validate tlsSecurityProfiles", func() {

				updateTLSSecurityProfile := func(ctx context.Context, minTLSVersion openshiftconfigv1.TLSProtocolVersion, ciphers []string) error {
					cr.Spec.TLSSecurityProfile = &openshiftconfigv1.TLSSecurityProfile{
						Custom: &openshiftconfigv1.CustomTLSProfile{
							TLSProfileSpec: openshiftconfigv1.TLSProfileSpec{
								MinTLSVersion: minTLSVersion,
								Ciphers:       ciphers,
							},
						},
					}

					return wh.ValidateCreate(ctx, dryRun, cr)
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
			})

			Context("validate affinity", func() {
				It("should allow empty affinity", func(ctx context.Context) {
					cr.Spec.Infra.NodePlacement = &sdkapi.NodePlacement{
						Affinity: nil,
					}
					cr.Spec.Workloads.NodePlacement = &sdkapi.NodePlacement{
						Affinity: nil,
					}

					Expect(wh.ValidateCreate(ctx, dryRun, cr)).To(Succeed())
				})

				It("should allow empty affinity", func(ctx context.Context) {
					cr.Spec.Infra.NodePlacement = &sdkapi.NodePlacement{
						Affinity: &corev1.Affinity{},
					}
					cr.Spec.Workloads.NodePlacement = &sdkapi.NodePlacement{
						Affinity: &corev1.Affinity{},
					}

					Expect(wh.ValidateCreate(ctx, dryRun, cr)).To(Succeed())
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

					Expect(wh.ValidateCreate(ctx, dryRun, cr)).To(Succeed())
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

					Expect(wh.ValidateCreate(ctx, dryRun, cr)).To(MatchError(
						And(
							ContainSubstring("invalid workloads node placement affinity:"),
							ContainSubstring(`Unsupported value: "WrongOperator"`),
						),
					))
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

					Expect(wh.ValidateCreate(ctx, dryRun, cr)).To(MatchError(
						And(
							ContainSubstring("invalid workloads node placement affinity:"),
							ContainSubstring("must have one element"),
						),
					))
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

					Expect(wh.ValidateCreate(ctx, dryRun, cr)).To(MatchError(
						And(
							ContainSubstring("invalid infra node placement affinity:"),
							ContainSubstring(`Unsupported value: "WrongOperator"`),
						),
					))
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

					Expect(wh.ValidateCreate(ctx, dryRun, cr)).To(MatchError(
						And(
							ContainSubstring("invalid infra node placement affinity:"),
							ContainSubstring("must have one element"),
						),
					))
				})
			})

			Context("validate tuning policy", func() {
				It("should return warning for deprecated highBurst tuning policy", func(ctx context.Context) {
					cr.Spec.TuningPolicy = hcov1beta1.HyperConvergedHighBurstProfile //nolint SA1019
					err := wh.ValidateCreate(ctx, dryRun, cr)
					Expect(err).To(HaveOccurred())
					expected := &ValidationWarning{}
					Expect(errors.As(err, &expected)).To(BeTrue())
					Expect(expected.warnings).To(HaveLen(1))
					Expect(expected.warnings[0]).To(ContainSubstring("highBurst profile is deprecated"))
					Expect(expected.warnings[0]).To(ContainSubstring("v1.16.0"))
				})

				It("should not return warning when tuning policy is not set", func(ctx context.Context) {
					cr.Spec.TuningPolicy = ""
					Expect(wh.ValidateCreate(ctx, dryRun, cr)).To(Succeed())
				})
			})
		})
	})

	Context("validate update validation webhook", func() {

		Context("check update request", func() {
			var hco *hcov1.HyperConverged

			BeforeEach(func() {
				hco = commontestutils.NewV1HCO()
			})

			It("should correctly handle a valid update request", func(ctx context.Context) {
				ctx = logr.NewContext(ctx, GinkgoLogr)
				req := newRequest(admissionv1.Update, hco, v1Codec, false)

				res := wh.Handle(ctx, req)
				Expect(res.Allowed).To(BeTrue())
				Expect(res.Result.Code).To(Equal(int32(200)))
			})

			It("should correctly handle a valid dryrun update request", func(ctx context.Context) {
				ctx = logr.NewContext(ctx, GinkgoLogr)
				req := newRequest(admissionv1.Update, hco, v1Codec, true)

				res := wh.Handle(ctx, req)
				Expect(res.Allowed).To(BeTrue())
				Expect(res.Result.Code).To(Equal(int32(200)))
			})

			It("should reject update requests with no object", func(ctx context.Context) {
				ctx = logr.NewContext(ctx, GinkgoLogr)
				req := newRequest(admissionv1.Update, hco, v1Codec, false)
				req.Object = runtime.RawExtension{}

				res := wh.Handle(ctx, req)
				Expect(res.Allowed).To(BeFalse())
				Expect(res.Result.Code).To(Equal(int32(400)))
				Expect(res.Result.Message).To(Equal(decodeErrorMsg))
			})

			It("should reject update requests with no oldObject", func(ctx context.Context) {
				ctx = logr.NewContext(ctx, GinkgoLogr)
				req := newRequest(admissionv1.Update, hco, v1Codec, false)
				req.OldObject = runtime.RawExtension{}

				res := wh.Handle(ctx, req)
				Expect(res.Allowed).To(BeFalse())
				Expect(res.Result.Code).To(Equal(int32(400)))
				Expect(res.Result.Message).To(Equal(decodeErrorMsg))
			})
		})

		Context("check ValidateUpdate", func() {
			var hco *hcov1beta1.HyperConverged

			BeforeEach(func() {
				hco = commontestutils.NewHco()
				hco.Spec.Infra = hcov1beta1.HyperConvergedConfig{
					NodePlacement: newHyperConvergedConfig(),
				}
				hco.Spec.Workloads = hcov1beta1.HyperConvergedConfig{
					NodePlacement: newHyperConvergedConfig(),
				}
			})

			It("should return error if KV CR is missing", func(ctx context.Context) {
				cli := getFakeClient(hco)

				kv := handlers.NewKubeVirtWithNameOnly(hco)
				Expect(cli.Delete(ctx, kv)).To(Succeed())

				wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true, nil)

				newHco := &hcov1beta1.HyperConverged{}
				hco.DeepCopyInto(newHco)
				// just do some change to force update
				newHco.Spec.Infra.NodePlacement.NodeSelector["key3"] = "value3"

				err := wh.ValidateUpdate(ctx, dryRun, newHco, hco)
				Expect(err).To(MatchError(apierrors.IsNotFound, "not found error"))
				Expect(err).To(MatchError(ContainSubstring("kubevirts.kubevirt.io")))
			})

			It("should return error if dry-run update of KV CR returns error", func(ctx context.Context) {
				cli := getFakeClient(hco)
				cli.InitiateUpdateErrors(getUpdateError(kvUpdateFailure))

				wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true, nil)

				newHco := &hcov1beta1.HyperConverged{}
				hco.DeepCopyInto(newHco)
				// change something in workloads to trigger dry-run update
				newHco.Spec.Workloads.NodePlacement.NodeSelector["a change"] = "Something else"

				err := wh.ValidateUpdate(ctx, dryRun, newHco, hco)
				Expect(err).To(MatchError(ErrFakeKvError))
			})

			It("should return error if CDI CR is missing", func(ctx context.Context) {
				cli := getFakeClient(hco)
				cdi, err := handlers.NewCDI(hco)
				Expect(err).ToNot(HaveOccurred())
				Expect(cli.Delete(ctx, cdi)).To(Succeed())

				wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true, nil)

				newHco := &hcov1beta1.HyperConverged{}
				hco.DeepCopyInto(newHco)
				// just do some change to force update
				newHco.Spec.Infra.NodePlacement.NodeSelector["key3"] = "value3"

				err = wh.ValidateUpdate(ctx, dryRun, newHco, hco)
				Expect(err).To(MatchError(apierrors.IsNotFound, "not found error"))
				Expect(err).To(MatchError(ContainSubstring("cdis.cdi.kubevirt.io")))
			})

			It("should return error if dry-run update of CDI CR returns error", func(ctx context.Context) {
				cli := getFakeClient(hco)
				cli.InitiateUpdateErrors(getUpdateError(cdiUpdateFailure))
				wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true, nil)

				newHco := &hcov1beta1.HyperConverged{}
				hco.DeepCopyInto(newHco)
				// change something in workloads to trigger dry-run update
				newHco.Spec.Workloads.NodePlacement.NodeSelector["a change"] = "Something else"

				err := wh.ValidateUpdate(ctx, dryRun, newHco, hco)
				Expect(err).To(MatchError(ErrFakeCdiError))
			})

			It("should not return error if dry-run update of ALL CR passes", func(ctx context.Context) {
				cli := getFakeClient(hco)
				cli.InitiateUpdateErrors(getUpdateError(noFailure))

				wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true, nil)

				newHco := &hcov1beta1.HyperConverged{}
				hco.DeepCopyInto(newHco)
				// change something in workloads to trigger dry-run update
				newHco.Spec.Workloads.NodePlacement.NodeSelector["a change"] = "Something else"

				Expect(wh.ValidateUpdate(ctx, dryRun, newHco, hco)).To(Succeed())
			})

			It("should return error if NetworkAddons CR is missing", func(ctx context.Context) {
				cli := getFakeClient(hco)
				cna, err := handlers.NewNetworkAddons(hco)
				Expect(err).ToNot(HaveOccurred())
				Expect(cli.Delete(ctx, cna)).To(Succeed())
				wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true, nil)

				newHco := &hcov1beta1.HyperConverged{}
				hco.DeepCopyInto(newHco)
				// just do some change to force update
				newHco.Spec.Infra.NodePlacement.NodeSelector["key3"] = "value3"

				err = wh.ValidateUpdate(ctx, dryRun, newHco, hco)
				Expect(err).To(MatchError(apierrors.IsNotFound, "not found error"))
				Expect(err).To(MatchError(ContainSubstring("networkaddonsconfigs.networkaddonsoperator.network.kubevirt.io")))
			})

			It("should return error if dry-run update of NetworkAddons CR returns error", func(ctx context.Context) {
				cli := getFakeClient(hco)
				cli.InitiateUpdateErrors(getUpdateError(networkUpdateFailure))

				wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true, nil)

				newHco := &hcov1beta1.HyperConverged{}
				hco.DeepCopyInto(newHco)
				// change something in workloads to trigger dry-run update
				newHco.Spec.Workloads.NodePlacement.NodeSelector["a change"] = "Something else"

				err := wh.ValidateUpdate(ctx, dryRun, newHco, hco)
				Expect(err).To(MatchError(ErrFakeNetworkError))
			})

			It("should return error if SSP CR is missing", func(ctx context.Context) {
				cli := getFakeClient(hco)

				Expect(cli.Delete(ctx, handlers.NewSSPWithNameOnly(hco))).To(Succeed())
				wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true, nil)

				newHco := &hcov1beta1.HyperConverged{}
				hco.DeepCopyInto(newHco)
				// just do some change to force update
				newHco.Spec.Infra.NodePlacement.NodeSelector["key3"] = "value3"

				err := wh.ValidateUpdate(ctx, dryRun, newHco, hco)
				Expect(err).To(MatchError(apierrors.IsNotFound, "not found error"))
				Expect(err).To(MatchError(ContainSubstring("ssps.ssp.kubevirt.io")))
			})

			It("should return error if dry-run update of SSP CR returns error", func(ctx context.Context) {
				cli := getFakeClient(hco)
				cli.InitiateUpdateErrors(getUpdateError(sspUpdateFailure))
				wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true, nil)

				newHco := &hcov1beta1.HyperConverged{}
				hco.DeepCopyInto(newHco)
				// change something in workloads to trigger dry-run update
				newHco.Spec.Workloads.NodePlacement.NodeSelector["a change"] = "Something else"

				err := wh.ValidateUpdate(ctx, dryRun, newHco, hco)
				Expect(err).To(MatchError(ErrFakeSspError))

			})

			It("should return error if dry-run update is timeout", func(ctx context.Context) {
				cli := getFakeClient(hco)
				cli.InitiateUpdateErrors(initiateTimeout)

				wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true, nil)

				newHco := &hcov1beta1.HyperConverged{}
				hco.DeepCopyInto(newHco)
				// change something in workloads to trigger dry-run update
				newHco.Spec.Workloads.NodePlacement.NodeSelector["a change"] = "Something else"

				err := wh.ValidateUpdate(ctx, dryRun, newHco, hco)
				Expect(err).To(MatchError(context.DeadlineExceeded))
			})

			It("should not return error if nothing was changed", func(ctx context.Context) {
				cli := getFakeClient(hco)
				cli.InitiateUpdateErrors(initiateTimeout)

				wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true, nil)

				newHco := &hcov1beta1.HyperConverged{}
				hco.DeepCopyInto(newHco)

				Expect(wh.ValidateUpdate(ctx, dryRun, newHco, hco)).To(Succeed())
			})

			Context("test permitted host devices update validation", func() {
				It("should allow unique PCI Host Device", func(ctx context.Context) {
					cli := getFakeClient(hco)
					wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true, nil)

					newHco := &hcov1beta1.HyperConverged{}
					hco.DeepCopyInto(newHco)
					newHco.Spec.PermittedHostDevices = &hcov1beta1.PermittedHostDevices{
						PciHostDevices: []hcov1beta1.PciHostDevice{
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
					Expect(wh.ValidateUpdate(ctx, dryRun, newHco, hco)).To(Succeed())
				})

				It("should allow unique Mediate Host Device", func(ctx context.Context) {
					cli := getFakeClient(hco)
					wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true, nil)

					newHco := &hcov1beta1.HyperConverged{}
					hco.DeepCopyInto(newHco)
					newHco.Spec.PermittedHostDevices = &hcov1beta1.PermittedHostDevices{
						MediatedDevices: []hcov1beta1.MediatedHostDevice{
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
					Expect(wh.ValidateUpdate(ctx, dryRun, newHco, hco)).To(Succeed())
				})
			})

			Context("plain-k8s tests", func() {
				It("should return error in plain-k8s if KV CR is missing", func(ctx context.Context) {
					hco := &hcov1beta1.HyperConverged{}
					cli := getFakeClient(hco)
					kv, err := handlers.NewKubeVirt(hco)
					Expect(err).ToNot(HaveOccurred())
					Expect(cli.Delete(ctx, kv)).To(Succeed())
					wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, false, nil)

					newHco := commontestutils.NewHco()
					newHco.Spec.Infra = hcov1beta1.HyperConvergedConfig{
						NodePlacement: newHyperConvergedConfig(),
					}
					newHco.Spec.Workloads = hcov1beta1.HyperConvergedConfig{
						NodePlacement: newHyperConvergedConfig(),
					}

					Expect(
						wh.ValidateUpdate(ctx, dryRun, newHco, hco),
					).To(MatchError(apierrors.IsNotFound, "not found error"))
				})
			})

			Context("Check LiveMigrationConfiguration", func() {
				It("should ignore if there is no change in live migration", func(ctx context.Context) {
					cli := getFakeClient(hco)

					// Deleting KV here, in order to make sure the that the webhook does not find differences,
					// and so it exits with no error before finding that KV is not there.
					// Later we'll check that there is no error from the webhook, and that will prove that
					// the comparison works.
					kv := handlers.NewKubeVirtWithNameOnly(hco)
					Expect(cli.Delete(context.TODO(), kv)).To(Succeed())

					wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true, nil)

					newHco := &hcov1beta1.HyperConverged{}
					hco.DeepCopyInto(newHco)

					Expect(wh.ValidateUpdate(ctx, dryRun, newHco, hco)).To(Succeed())
				})

				It("should allow updating of live migration", func(ctx context.Context) {
					cli := getFakeClient(hco)

					wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true, nil)

					newHco := &hcov1beta1.HyperConverged{}
					hco.DeepCopyInto(newHco)

					// change something in the LiveMigrationConfig field
					hco.Spec.LiveMigrationConfig.CompletionTimeoutPerGiB = ptr.To[int64](200)

					Expect(wh.ValidateUpdate(ctx, dryRun, newHco, hco)).To(Succeed())
				})

				It("should fail if live migration is wrong", func(ctx context.Context) {
					cli := getFakeClient(hco)

					wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true, nil)

					newHco := &hcov1beta1.HyperConverged{}
					hco.DeepCopyInto(newHco)

					// change something in the LiveMigrationConfig field
					newHco.Spec.LiveMigrationConfig.BandwidthPerMigration = ptr.To("Wrong Value")

					Expect(
						wh.ValidateUpdate(ctx, dryRun, newHco, hco),
					).To(MatchError(ContainSubstring("failed to parse the LiveMigrationConfig.bandwidthPerMigration field")))
				})
			})

			Context("Check CertRotation", func() {
				It("should ignore if there is no change in cert config", func(ctx context.Context) {
					cli := getFakeClient(hco)

					// Deleting KV here, in order to make sure the that the webhook does not find differences,
					// and so it exits with no error before finding that KV is not there.
					// Later we'll check that there is no error from the webhook, and that will prove that
					// the comparison works.
					kv := handlers.NewKubeVirtWithNameOnly(hco)
					Expect(cli.Delete(ctx, kv)).To(Succeed())

					wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true, nil)

					newHco := &hcov1beta1.HyperConverged{}
					hco.DeepCopyInto(newHco)

					Expect(wh.ValidateUpdate(ctx, dryRun, newHco, hco)).To(Succeed())
				})

				It("should allow updating of cert config", func(ctx context.Context) {
					cli := getFakeClient(hco)

					wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true, nil)

					newHco := &hcov1beta1.HyperConverged{}
					hco.DeepCopyInto(newHco)

					// change something in the CertConfig fields
					newHco.Spec.CertConfig.CA.Duration.Duration = hco.Spec.CertConfig.CA.Duration.Duration * 2
					newHco.Spec.CertConfig.CA.RenewBefore.Duration = hco.Spec.CertConfig.CA.RenewBefore.Duration * 2
					newHco.Spec.CertConfig.Server.Duration.Duration = hco.Spec.CertConfig.Server.Duration.Duration * 2
					newHco.Spec.CertConfig.Server.RenewBefore.Duration = hco.Spec.CertConfig.Server.RenewBefore.Duration * 2

					Expect(wh.ValidateUpdate(ctx, dryRun, newHco, hco)).To(Succeed())
				})

				DescribeTable("should fail if cert config is wrong",
					func(ctx context.Context, newHco hcov1beta1.HyperConverged, errorMsg string) {
						cli := getFakeClient(hco)

						wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true, nil)

						err := wh.ValidateUpdate(ctx, dryRun, &newHco, hco)
						Expect(err).To(MatchError(ContainSubstring(errorMsg)))
					},
					Entry("certConfig.ca.duration is too short",
						hcov1beta1.HyperConverged{
							ObjectMeta: metav1.ObjectMeta{
								Name:      util.HyperConvergedName,
								Namespace: HcoValidNamespace,
							},
							Spec: hcov1beta1.HyperConvergedSpec{
								CertConfig: hcov1beta1.HyperConvergedCertConfig{
									CA: hcov1beta1.CertRotateConfigCA{
										Duration:    &metav1.Duration{Duration: 8 * time.Minute},
										RenewBefore: &metav1.Duration{Duration: 24 * time.Hour},
									},
									Server: hcov1beta1.CertRotateConfigServer{
										Duration:    &metav1.Duration{Duration: 24 * time.Hour},
										RenewBefore: &metav1.Duration{Duration: 12 * time.Hour},
									},
								},
							},
						},
						"spec.certConfig.ca.duration: value is too small"),
					Entry("certConfig.ca.renewBefore is too short",
						hcov1beta1.HyperConverged{
							ObjectMeta: metav1.ObjectMeta{
								Name:      util.HyperConvergedName,
								Namespace: HcoValidNamespace,
							},
							Spec: hcov1beta1.HyperConvergedSpec{
								CertConfig: hcov1beta1.HyperConvergedCertConfig{
									CA: hcov1beta1.CertRotateConfigCA{
										Duration:    &metav1.Duration{Duration: 48 * time.Hour},
										RenewBefore: &metav1.Duration{Duration: 8 * time.Minute},
									},
									Server: hcov1beta1.CertRotateConfigServer{
										Duration:    &metav1.Duration{Duration: 24 * time.Hour},
										RenewBefore: &metav1.Duration{Duration: 12 * time.Hour},
									},
								},
							},
						},
						"spec.certConfig.ca.renewBefore: value is too small"),
					Entry("certConfig.server.duration is too short",
						hcov1beta1.HyperConverged{
							ObjectMeta: metav1.ObjectMeta{
								Name:      util.HyperConvergedName,
								Namespace: HcoValidNamespace,
							},
							Spec: hcov1beta1.HyperConvergedSpec{
								CertConfig: hcov1beta1.HyperConvergedCertConfig{
									CA: hcov1beta1.CertRotateConfigCA{
										Duration:    &metav1.Duration{Duration: 48 * time.Hour},
										RenewBefore: &metav1.Duration{Duration: 24 * time.Hour},
									},
									Server: hcov1beta1.CertRotateConfigServer{
										Duration:    &metav1.Duration{Duration: 8 * time.Minute},
										RenewBefore: &metav1.Duration{Duration: 12 * time.Hour},
									},
								},
							},
						},
						"spec.certConfig.server.duration: value is too small"),
					Entry("certConfig.server.renewBefore is too short",
						hcov1beta1.HyperConverged{
							ObjectMeta: metav1.ObjectMeta{
								Name:      util.HyperConvergedName,
								Namespace: HcoValidNamespace,
							},
							Spec: hcov1beta1.HyperConvergedSpec{
								CertConfig: hcov1beta1.HyperConvergedCertConfig{
									CA: hcov1beta1.CertRotateConfigCA{
										Duration:    &metav1.Duration{Duration: 48 * time.Hour},
										RenewBefore: &metav1.Duration{Duration: 24 * time.Hour},
									},
									Server: hcov1beta1.CertRotateConfigServer{
										Duration:    &metav1.Duration{Duration: 24 * time.Hour},
										RenewBefore: &metav1.Duration{Duration: 8 * time.Minute},
									},
								},
							},
						},
						"spec.certConfig.server.renewBefore: value is too small"),
					Entry("ca: duration is smaller than renewBefore",
						hcov1beta1.HyperConverged{
							ObjectMeta: metav1.ObjectMeta{
								Name:      util.HyperConvergedName,
								Namespace: HcoValidNamespace,
							},
							Spec: hcov1beta1.HyperConvergedSpec{
								CertConfig: hcov1beta1.HyperConvergedCertConfig{
									CA: hcov1beta1.CertRotateConfigCA{
										Duration:    &metav1.Duration{Duration: 23 * time.Hour},
										RenewBefore: &metav1.Duration{Duration: 24 * time.Hour},
									},
									Server: hcov1beta1.CertRotateConfigServer{
										Duration:    &metav1.Duration{Duration: 24 * time.Hour},
										RenewBefore: &metav1.Duration{Duration: 12 * time.Hour},
									},
								},
							},
						},
						"spec.certConfig.ca: duration is smaller than renewBefore"),
					Entry("server: duration is smaller than renewBefore",
						hcov1beta1.HyperConverged{
							ObjectMeta: metav1.ObjectMeta{
								Name:      util.HyperConvergedName,
								Namespace: HcoValidNamespace,
							},
							Spec: hcov1beta1.HyperConvergedSpec{
								CertConfig: hcov1beta1.HyperConvergedCertConfig{
									CA: hcov1beta1.CertRotateConfigCA{
										Duration:    &metav1.Duration{Duration: 48 * time.Hour},
										RenewBefore: &metav1.Duration{Duration: 24 * time.Hour},
									},
									Server: hcov1beta1.CertRotateConfigServer{
										Duration:    &metav1.Duration{Duration: 11 * time.Hour},
										RenewBefore: &metav1.Duration{Duration: 12 * time.Hour},
									},
								},
							},
						},
						"spec.certConfig.server: duration is smaller than renewBefore"),
					Entry("ca.duration is smaller than server.duration",
						hcov1beta1.HyperConverged{
							ObjectMeta: metav1.ObjectMeta{
								Name:      util.HyperConvergedName,
								Namespace: HcoValidNamespace,
							},
							Spec: hcov1beta1.HyperConvergedSpec{
								CertConfig: hcov1beta1.HyperConvergedCertConfig{
									CA: hcov1beta1.CertRotateConfigCA{
										Duration:    &metav1.Duration{Duration: 48 * time.Hour},
										RenewBefore: &metav1.Duration{Duration: 24 * time.Hour},
									},
									Server: hcov1beta1.CertRotateConfigServer{
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
				updateTLSSecurityProfile := func(ctx context.Context, minTLSVersion openshiftconfigv1.TLSProtocolVersion, ciphers []string) error {
					cli := getFakeClient(hco)

					wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true, nil)

					newHco := &hcov1beta1.HyperConverged{}
					hco.DeepCopyInto(newHco)

					newHco.Spec.TLSSecurityProfile = &openshiftconfigv1.TLSSecurityProfile{
						Custom: &openshiftconfigv1.CustomTLSProfile{
							TLSProfileSpec: openshiftconfigv1.TLSProfileSpec{
								MinTLSVersion: minTLSVersion,
								Ciphers:       ciphers,
							},
						},
					}

					return wh.ValidateUpdate(ctx, dryRun, newHco, hco)
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
			})

			Context("validate tuning policy on update", func() {
				It("should return warning for deprecated highBurst tuning policy", func(ctx context.Context) {
					newHCO := hco.DeepCopy()
					newHCO.Spec.TuningPolicy = hcov1beta1.HyperConvergedHighBurstProfile //nolint SA1019
					err := wh.ValidateUpdate(ctx, dryRun, newHCO, hco)
					Expect(err).To(HaveOccurred())
					expected := &ValidationWarning{}
					Expect(errors.As(err, &expected)).To(BeTrue())
					Expect(expected.warnings).To(HaveLen(1))
					Expect(expected.warnings[0]).To(ContainSubstring("highBurst profile is deprecated"))
					Expect(expected.warnings[0]).To(ContainSubstring("v1.16.0"))
				})

				It("should not return warning when tuning policy is not set", func(ctx context.Context) {
					newHCO := hco.DeepCopy()
					newHCO.Spec.TuningPolicy = ""
					Expect(wh.ValidateUpdate(ctx, dryRun, newHCO, hco)).To(Succeed())
				})
			})

			Context("unsupported annotation", func() {
				DescribeTable("should accept if annotation is valid",
					func(ctx context.Context, annotationName, annotation string) {
						cli := getFakeClient(hco)
						wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true, nil)

						newHco := &hcov1beta1.HyperConverged{}
						hco.DeepCopyInto(newHco)
						hco.Annotations = map[string]string{annotationName: annotation}

						Expect(wh.ValidateUpdate(ctx, dryRun, newHco, hco)).To(Succeed())
					},
					Entry("should accept if kv annotation is valid", common.JSONPatchKVAnnotationName, validKvAnnotation),
					Entry("should accept if cdi annotation is valid", common.JSONPatchCDIAnnotationName, validCdiAnnotation),
					Entry("should accept if cna annotation is valid", common.JSONPatchCNAOAnnotationName, validCnaAnnotation),
					Entry("should accept if ssp annotation is valid", common.JSONPatchSSPAnnotationName, validSspAnnotation),
				)

				DescribeTable("should reject if annotation is invalid",
					func(ctx context.Context, annotationName, annotation string) {
						cli := getFakeClient(hco)
						cli.InitiateUpdateErrors(initiateTimeout)

						wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true, nil)

						newHco := &hcov1beta1.HyperConverged{}
						hco.DeepCopyInto(newHco)
						newHco.Annotations = map[string]string{annotationName: annotation}

						Expect(wh.ValidateUpdate(context.TODO(), false, newHco, hco)).To(MatchError(ContainSubstring("invalid jsonPatch in the %s", annotationName)))
					},
					Entry("should reject if kv annotation is invalid", common.JSONPatchKVAnnotationName, invalidKvAnnotation),
					Entry("should reject if cdi annotation is invalid", common.JSONPatchCDIAnnotationName, invalidCdiAnnotation),
					Entry("should reject if cna annotation is invalid", common.JSONPatchCNAOAnnotationName, invalidCnaAnnotation),
					Entry("should accept if ssp annotation is invalid", common.JSONPatchSSPAnnotationName, invalidSspAnnotation),
				)
			})
		})
	})

	Context("validate delete validation webhook", func() {
		Context("check delete request", func() {
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
				req := newRequest(admissionv1.Delete, hco, v1Codec, false)
				ctx = logr.NewContext(ctx, GinkgoLogr)

				res := wh.Handle(ctx, req)
				Expect(res.Allowed).To(BeTrue())
				Expect(res.Result.Code).To(Equal(int32(200)))
			})

			It("should correctly handle a valid dryrun delete request", func(ctx context.Context) {
				req := newRequest(admissionv1.Delete, hco, v1Codec, true)
				ctx = logr.NewContext(ctx, GinkgoLogr)

				res := wh.Handle(ctx, req)
				Expect(res.Allowed).To(BeTrue())
				Expect(res.Result.Code).To(Equal(int32(200)))
			})

			It("should reject a malformed delete request", func(ctx context.Context) {
				req := newRequest(admissionv1.Delete, hco, v1Codec, false)
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
			var hco *hcov1beta1.HyperConverged

			BeforeEach(func() {
				hco = &hcov1beta1.HyperConverged{
					ObjectMeta: metav1.ObjectMeta{
						Name:      util.HyperConvergedName,
						Namespace: HcoValidNamespace,
					},
				}
			})

			It("should validate deletion", func(ctx context.Context) {
				cli := getFakeClient(hco)

				wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true, nil)

				Expect(wh.ValidateDelete(ctx, dryRun, hco)).To(Succeed())

				By("Validate that KV still exists, as it a dry-run deletion")
				kv := handlers.NewKubeVirtWithNameOnly(hco)
				Expect(util.GetRuntimeObject(context.TODO(), cli, kv)).To(Succeed())

				By("Validate that CDI still exists, as it a dry-run deletion")
				cdi := handlers.NewCDIWithNameOnly(hco)
				Expect(util.GetRuntimeObject(context.TODO(), cli, cdi)).To(Succeed())
			})

			It("should reject if KV deletion fails", func(ctx context.Context) {
				cli := getFakeClient(hco)

				wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true, nil)

				cli.InitiateDeleteErrors(func(obj client.Object) error {
					if unstructed, ok := obj.(runtime.Unstructured); ok {
						kind := unstructed.GetObjectKind()
						if kind.GroupVersionKind().Kind == "KubeVirt" {
							return ErrFakeKvError
						}
					}
					return nil
				})

				err := wh.ValidateDelete(ctx, dryRun, hco)
				Expect(err).To(MatchError(ErrFakeKvError))
			})

			It("should reject if CDI deletion fails", func(ctx context.Context) {
				cli := getFakeClient(hco)

				wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true, nil)

				cli.InitiateDeleteErrors(func(obj client.Object) error {
					if unstructed, ok := obj.(runtime.Unstructured); ok {
						kind := unstructed.GetObjectKind()
						if kind.GroupVersionKind().Kind == "CDI" {
							return ErrFakeCdiError
						}
					}
					return nil
				})

				err := wh.ValidateDelete(ctx, dryRun, hco)
				Expect(err).To(MatchError(ErrFakeCdiError))
			})

			It("should ignore if KV does not exist", func(ctx context.Context) {
				cli := getFakeClient(hco)

				kv := handlers.NewKubeVirtWithNameOnly(hco)
				Expect(cli.Delete(ctx, kv)).To(Succeed())

				wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true, nil)

				Expect(wh.ValidateDelete(ctx, dryRun, hco)).To(Succeed())
			})

			It("should reject if getting KV failed for not-not-exists error", func(ctx context.Context) {
				cli := getFakeClient(hco)

				wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true, nil)

				cli.InitiateGetErrors(func(key client.ObjectKey) error {
					if key.Name == "kubevirt-kubevirt-hyperconverged" {
						return ErrFakeKvError
					}
					return nil
				})

				err := wh.ValidateDelete(ctx, dryRun, hco)
				Expect(err).To(MatchError(ErrFakeKvError))
			})

			It("should ignore if CDI does not exist", func(ctx context.Context) {
				cli := getFakeClient(hco)

				cdi := handlers.NewCDIWithNameOnly(hco)
				Expect(cli.Delete(ctx, cdi)).To(Succeed())

				wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true, nil)

				Expect(wh.ValidateDelete(ctx, dryRun, hco)).To(Succeed())
			})

			It("should reject if getting CDI failed for not-not-exists error", func(ctx context.Context) {
				cli := getFakeClient(hco)

				wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true, nil)

				cli.InitiateGetErrors(func(key client.ObjectKey) error {
					if key.Name == "cdi-kubevirt-hyperconverged" {
						return ErrFakeCdiError
					}
					return nil
				})

				err := wh.ValidateDelete(ctx, dryRun, hco)
				Expect(err).To(MatchError(ErrFakeCdiError))
			})
		})
	})

	Context("hcoTLSConfigCache", func() {
		var cr *hcov1beta1.HyperConverged

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
			hcoTLSConfigCache = &initialTLSSecurityProfile
			cr = commontestutils.NewHco()
		})

		Context("create", func() {

			It("should update hcoTLSConfigCache creating a resource not in dry run mode", func(ctx context.Context) {
				Expect(hcoTLSConfigCache).To(Equal(&initialTLSSecurityProfile))
				cr.Spec.TLSSecurityProfile = &modernTLSSecurityProfile
				Expect(wh.ValidateCreate(ctx, false, cr)).To(Succeed())
				Expect(hcoTLSConfigCache).To(Equal(&modernTLSSecurityProfile))
			})

			It("should not update hcoTLSConfigCache creating a resource in dry run mode", func(ctx context.Context) {
				Expect(hcoTLSConfigCache).To(Equal(&initialTLSSecurityProfile))
				cr.Spec.TLSSecurityProfile = &modernTLSSecurityProfile
				Expect(wh.ValidateCreate(ctx, true, cr)).To(Succeed())
				Expect(hcoTLSConfigCache).ToNot(Equal(&modernTLSSecurityProfile))
			})

			It("should not update hcoTLSConfigCache if the create request is refused", func(ctx context.Context) {
				Expect(hcoTLSConfigCache).To(Equal(&initialTLSSecurityProfile))
				cr.Spec.TLSSecurityProfile = &modernTLSSecurityProfile
				cr.Namespace = ResourceInvalidNamespace

				cr.Spec.DataImportCronTemplates = []hcov1beta1.DataImportCronTemplate{
					{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{
								util.DataImportCronEnabledAnnotation: "a-non-boolean-value",
							},
						},
					},
				}

				Expect(wh.ValidateCreate(ctx, false, cr)).ToNot(Succeed())
				Expect(hcoTLSConfigCache).To(Equal(&initialTLSSecurityProfile))
			})
		})

		Context("update", func() {

			It("should update hcoTLSConfigCache updating a resource not in dry run mode", func(ctx context.Context) {
				cli := getFakeClient(cr)
				cli.InitiateUpdateErrors(getUpdateError(noFailure))

				wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true, nil)

				newCr := &hcov1beta1.HyperConverged{}
				cr.DeepCopyInto(newCr)
				newCr.Spec.TLSSecurityProfile = &oldTLSSecurityProfile

				Expect(wh.ValidateUpdate(ctx, false, newCr, cr)).To(Succeed())
				Expect(hcoTLSConfigCache).To(Equal(&oldTLSSecurityProfile))
			})

			It("should not update hcoTLSConfigCache updating a resource in dry run mode", func(ctx context.Context) {
				cli := getFakeClient(cr)
				cli.InitiateUpdateErrors(getUpdateError(noFailure))

				wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true, &initialTLSSecurityProfile)

				newCr := &hcov1beta1.HyperConverged{}
				cr.DeepCopyInto(newCr)
				newCr.Spec.TLSSecurityProfile = &oldTLSSecurityProfile

				Expect(wh.ValidateUpdate(ctx, true, newCr, cr)).To(Succeed())
				Expect(hcoTLSConfigCache).To(Equal(&initialTLSSecurityProfile))
			})

			It("should not update hcoTLSConfigCache if the update request is refused", func(ctx context.Context) {
				cli := getFakeClient(cr)
				cli.InitiateUpdateErrors(getUpdateError(cdiUpdateFailure))

				wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true, &initialTLSSecurityProfile)

				newCr := &hcov1beta1.HyperConverged{}
				cr.DeepCopyInto(newCr)
				newCr.Spec.TLSSecurityProfile = &oldTLSSecurityProfile

				err := wh.ValidateUpdate(ctx, false, newCr, cr)
				Expect(err).To(MatchError(ErrFakeCdiError))
				Expect(hcoTLSConfigCache).To(Equal(&initialTLSSecurityProfile))
			})

		})

		Context("delete", func() {

			It("should reset hcoTLSConfigCache deleting a resource not in dry run mode", func(ctx context.Context) {
				cli := getFakeClient(cr)
				wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true, nil)

				hcoTLSConfigCache = &modernTLSSecurityProfile

				Expect(wh.ValidateDelete(ctx, false, cr)).To(Succeed())
				Expect(hcoTLSConfigCache).To(BeNil())
			})

			It("should not update hcoTLSConfigCache deleting a resource in dry run mode", func(ctx context.Context) {
				cli := getFakeClient(cr)
				wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true, nil)

				hcoTLSConfigCache = &modernTLSSecurityProfile

				Expect(wh.ValidateDelete(ctx, true, cr)).To(Succeed())
				Expect(hcoTLSConfigCache).To(Equal(&modernTLSSecurityProfile))
			})

			It("should not update hcoTLSConfigCache if the delete request is refused", func(ctx context.Context) {
				cli := getFakeClient(cr)
				wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true, nil)

				hcoTLSConfigCache = &modernTLSSecurityProfile
				cli.InitiateDeleteErrors(func(obj client.Object) error {
					if unstructed, ok := obj.(runtime.Unstructured); ok {
						kind := unstructed.GetObjectKind()
						if kind.GroupVersionKind().Kind == "KubeVirt" {
							return ErrFakeKvError
						}
					}
					return nil
				})

				err := wh.ValidateDelete(ctx, false, cr)
				Expect(err).To(MatchError(ErrFakeKvError))
				Expect(hcoTLSConfigCache).To(Equal(&modernTLSSecurityProfile))
			})

		})

		Context("selectCipherSuitesAndMinTLSVersion", func() {
			const namespace = "kubevirt-hyperconverged"

			var apiServer *openshiftconfigv1.APIServer
			var cl *commontestutils.HcoTestClient

			BeforeEach(func() {
				_ = os.Setenv("OPERATOR_NAMESPACE", namespace)

				clusterVersion := &openshiftconfigv1.ClusterVersion{
					ObjectMeta: metav1.ObjectMeta{
						Name: "version",
					},
					Spec: openshiftconfigv1.ClusterVersionSpec{
						ClusterID: "clusterId",
					},
				}
				infrastructure := &openshiftconfigv1.Infrastructure{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster",
					},
					Status: openshiftconfigv1.InfrastructureStatus{
						ControlPlaneTopology:   openshiftconfigv1.HighlyAvailableTopologyMode,
						InfrastructureTopology: openshiftconfigv1.HighlyAvailableTopologyMode,
						PlatformStatus: &openshiftconfigv1.PlatformStatus{
							Type: "mocked",
						},
					},
				}
				ingress := &openshiftconfigv1.Ingress{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster",
					},
					Spec: openshiftconfigv1.IngressSpec{
						Domain: "domain",
					},
				}
				apiServer = &openshiftconfigv1.APIServer{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster",
					},
					Spec: openshiftconfigv1.APIServerSpec{},
				}
				namespace := &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: namespace,
					},
				}
				dns := &openshiftconfigv1.DNS{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster",
					},
					Spec: openshiftconfigv1.DNSSpec{
						BaseDomain: commontestutils.BaseDomain,
					},
				}
				ipv4network := &openshiftconfigv1.Network{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster",
					},
					Status: openshiftconfigv1.NetworkStatus{
						ClusterNetwork: []openshiftconfigv1.ClusterNetworkEntry{
							{
								CIDR: "10.128.0.0/14",
							},
						},
					},
				}

				resources := []client.Object{clusterVersion, infrastructure, ingress, apiServer, namespace, dns, ipv4network}
				cl = commontestutils.InitClient(resources)
			})

			DescribeTable("should consume ApiServer config if HCO one is not explicitly set",
				func(initApiTlsSecurityProfile, initHCOTlsSecurityProfile, midApiTlsSecurityProfile, midHCOTlsSecurityProfile, finApiTlsSecurityProfile, finHCOTlsSecurityProfile *openshiftconfigv1.TLSSecurityProfile, initExpected, midExpected, finExpected openshiftconfigv1.TLSProtocolVersion) {
					hcoTLSConfigCache = initHCOTlsSecurityProfile
					apiServer.Spec.TLSSecurityProfile = initApiTlsSecurityProfile
					Expect(cl.Update(context.TODO(), apiServer)).To(Succeed())
					Expect(util.GetClusterInfo().Init(context.TODO(), cl, logger)).To(Succeed())
					ci := util.GetClusterInfo()
					Expect(ci.IsOpenshift()).To(BeTrue())

					_, minTypedTLSVersion := SelectCipherSuitesAndMinTLSVersion()
					Expect(minTypedTLSVersion).To(Equal(initExpected))

					apiServer.Spec.TLSSecurityProfile = midApiTlsSecurityProfile
					Expect(cl.Update(context.TODO(), apiServer)).To(Succeed())
					hcoTLSConfigCache = midHCOTlsSecurityProfile
					Expect(util.GetClusterInfo().RefreshAPIServerCR(context.TODO(), cl)).To(Succeed())

					_, minTypedTLSVersion = SelectCipherSuitesAndMinTLSVersion()
					Expect(minTypedTLSVersion).To(Equal(midExpected))

					apiServer.Spec.TLSSecurityProfile = finApiTlsSecurityProfile
					Expect(cl.Update(context.TODO(), apiServer)).To(Succeed())
					hcoTLSConfigCache = finHCOTlsSecurityProfile

					Expect(util.GetClusterInfo().RefreshAPIServerCR(context.TODO(), cl)).To(Succeed())
					_, minTypedTLSVersion = SelectCipherSuitesAndMinTLSVersion()
					Expect(minTypedTLSVersion).To(Equal(finExpected))
				},
				Entry("nil on APIServer, nil on HCO -> old on API server -> nil on API server",
					nil,
					nil,
					&oldTLSSecurityProfile,
					nil,
					nil,
					nil,
					openshiftconfigv1.TLSProfiles[openshiftconfigv1.TLSProfileIntermediateType].MinTLSVersion,
					openshiftconfigv1.TLSProfiles[openshiftconfigv1.TLSProfileOldType].MinTLSVersion,
					openshiftconfigv1.TLSProfiles[openshiftconfigv1.TLSProfileIntermediateType].MinTLSVersion,
				),
				Entry("nil on APIServer, nil on HCO -> modern on HCO -> nil on HCO",
					nil,
					nil,
					nil,
					&modernTLSSecurityProfile,
					nil,
					nil,
					openshiftconfigv1.TLSProfiles[openshiftconfigv1.TLSProfileIntermediateType].MinTLSVersion,
					openshiftconfigv1.TLSProfiles[openshiftconfigv1.TLSProfileModernType].MinTLSVersion,
					openshiftconfigv1.TLSProfiles[openshiftconfigv1.TLSProfileIntermediateType].MinTLSVersion,
				),
				Entry("old on APIServer, nil on HCO -> intermediate on HCO -> old on API server",
					&oldTLSSecurityProfile,
					nil,
					&oldTLSSecurityProfile,
					&intermediateTLSSecurityProfile,
					&oldTLSSecurityProfile,
					nil,
					openshiftconfigv1.TLSProfiles[openshiftconfigv1.TLSProfileOldType].MinTLSVersion,
					openshiftconfigv1.TLSProfiles[openshiftconfigv1.TLSProfileIntermediateType].MinTLSVersion,
					openshiftconfigv1.TLSProfiles[openshiftconfigv1.TLSProfileOldType].MinTLSVersion,
				),
				Entry("old on APIServer, modern on HCO -> intermediate on HCO -> modern on API server, intermediate on HCO",
					&oldTLSSecurityProfile,
					&modernTLSSecurityProfile,
					&oldTLSSecurityProfile,
					&intermediateTLSSecurityProfile,
					&modernTLSSecurityProfile,
					&intermediateTLSSecurityProfile,
					openshiftconfigv1.TLSProfiles[openshiftconfigv1.TLSProfileModernType].MinTLSVersion,
					openshiftconfigv1.TLSProfiles[openshiftconfigv1.TLSProfileIntermediateType].MinTLSVersion,
					openshiftconfigv1.TLSProfiles[openshiftconfigv1.TLSProfileIntermediateType].MinTLSVersion,
				),
			)

		})

	})
})

func newHyperConvergedConfig() *sdkapi.NodePlacement {
	return &sdkapi.NodePlacement{
		NodeSelector: map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
		Affinity: &corev1.Affinity{
			NodeAffinity: &corev1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{
						{
							MatchExpressions: []corev1.NodeSelectorRequirement{
								{Key: "key1", Operator: "In", Values: []string{"value11", "value12"}},
								{Key: "key2", Operator: "In", Values: []string{"value21", "value22"}},
							},
							MatchFields: []corev1.NodeSelectorRequirement{
								{Key: "key1", Operator: "In", Values: []string{"value1"}},
								{Key: "key2", Operator: "In", Values: []string{"value2"}},
							},
						},
					},
				},
			},
		},
		Tolerations: []corev1.Toleration{
			{Key: "key1", Operator: "In", Value: "value1", Effect: "effect1", TolerationSeconds: ptr.To[int64](1)},
			{Key: "key2", Operator: "In", Value: "value2", Effect: "effect2", TolerationSeconds: ptr.To[int64](2)},
		},
	}
}

func getFakeClient(hco *hcov1beta1.HyperConverged) *commontestutils.HcoTestClient {
	kv, err := handlers.NewKubeVirt(hco)
	Expect(err).ToNot(HaveOccurred())

	cdi, err := handlers.NewCDI(hco)
	Expect(err).ToNot(HaveOccurred())

	cna, err := handlers.NewNetworkAddons(hco)
	Expect(err).ToNot(HaveOccurred())

	ssp, _, err := handlers.NewSSP(hco)
	Expect(err).ToNot(HaveOccurred())

	v1hc := &hcov1.HyperConverged{}
	Expect(hco.ConvertTo(v1hc)).To(Succeed())

	return commontestutils.InitClient([]client.Object{v1hc, kv, cdi, cna, ssp})
}

func getFakeV1Beta1Client(hco *hcov1beta1.HyperConverged) *commontestutils.HcoTestClient {
	kv, err := handlers.NewKubeVirt(hco)
	Expect(err).ToNot(HaveOccurred())

	cdi, err := handlers.NewCDI(hco)
	Expect(err).ToNot(HaveOccurred())

	cna, err := handlers.NewNetworkAddons(hco)
	Expect(err).ToNot(HaveOccurred())

	ssp, _, err := handlers.NewSSP(hco)
	Expect(err).ToNot(HaveOccurred())

	return commontestutils.InitClient([]client.Object{hco, kv, cdi, cna, ssp})
}

type fakeFailure int

const (
	noFailure fakeFailure = iota
	kvUpdateFailure
	cdiUpdateFailure
	networkUpdateFailure
	sspUpdateFailure
)

var (
	ErrFakeKvError      = errors.New("fake KubeVirt error")
	ErrFakeCdiError     = errors.New("fake CDI error")
	ErrFakeNetworkError = errors.New("fake Network error")
	ErrFakeSspError     = errors.New("fake SSP error")
)

func getUpdateError(failure fakeFailure) commontestutils.FakeWriteErrorGenerator {
	switch failure {
	case kvUpdateFailure:
		return func(obj client.Object) error {
			if _, ok := obj.(*kubevirtcorev1.KubeVirt); ok {
				return ErrFakeKvError
			}
			return nil
		}

	case cdiUpdateFailure:
		return func(obj client.Object) error {
			if _, ok := obj.(*cdiv1beta1.CDI); ok {
				return ErrFakeCdiError
			}
			return nil
		}

	case networkUpdateFailure:
		return func(obj client.Object) error {
			if _, ok := obj.(*networkaddonsv1.NetworkAddonsConfig); ok {
				return ErrFakeNetworkError
			}
			return nil
		}

	case sspUpdateFailure:
		return func(obj client.Object) error {
			if _, ok := obj.(*sspv1beta3.SSP); ok {
				return ErrFakeSspError
			}
			return nil
		}
	default:
		return nil
	}
}

func initiateTimeout(_ client.Object) error {
	time.Sleep(updateDryRunTimeOut + time.Millisecond*100)
	return nil
}

func newRequest(operation admissionv1.Operation, cr *hcov1.HyperConverged, encoder runtime.Encoder, dryrun bool) admission.Request {
	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			DryRun:    ptr.To(dryrun),
			Operation: operation,
			Resource: metav1.GroupVersionResource{
				Group:    hcov1.SchemeGroupVersion.Group,
				Version:  hcov1.SchemeGroupVersion.Version,
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
