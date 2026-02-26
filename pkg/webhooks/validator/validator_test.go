package validator

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

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

	"github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/handlers"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/tlssecprofile"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

const (
	ResourceInvalidNamespace = "an-arbitrary-namespace"
	HcoValidNamespace        = "kubevirt-hyperconverged"
)

var (
	logger = zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)).WithName("hyperconverged-resource")
)

func TestValidatorWebhook(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Validator Webhooks Suite")
}

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

var _ = Describe("webhooks validator", func() {
	s := scheme.Scheme
	for _, f := range []func(*runtime.Scheme) error{
		v1beta1.AddToScheme,
		cdiv1beta1.AddToScheme,
		kubevirtcorev1.AddToScheme,
		networkaddonsv1.AddToScheme,
		sspv1beta3.AddToScheme,
	} {
		Expect(f(s)).To(Succeed())
	}

	codecFactory := serializer.NewCodecFactory(s)
	v1beta1Codec := codecFactory.LegacyCodec(v1beta1.SchemeGroupVersion)

	cli := fake.NewClientBuilder().WithScheme(s).Build()
	decoder := admission.NewDecoder(s)

	wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true)

	Context("Check create validation webhook", func() {
		var cr *v1beta1.HyperConverged
		var dryRun bool
		var ctx context.Context
		BeforeEach(func() {
			Expect(os.Setenv("OPERATOR_NAMESPACE", HcoValidNamespace)).To(Succeed())
			cr = commontestutils.NewHco()
			dryRun = false
			ctx = context.TODO()
		})

		It("should correctly handle a valid creation request", func() {
			req := newRequest(admissionv1.Create, cr, v1beta1Codec, false)

			res := wh.Handle(ctx, req)
			Expect(res.Allowed).To(BeTrue())
			Expect(res.Result.Code).To(Equal(int32(200)))
		})

		It("should correctly handle a valid dryrun creation request", func() {
			req := newRequest(admissionv1.Create, cr, v1beta1Codec, true)

			res := wh.Handle(ctx, req)
			Expect(res.Allowed).To(BeTrue())
			Expect(res.Result.Code).To(Equal(int32(200)))
		})

		It("should reject malformed creation requests", func() {
			req := newRequest(admissionv1.Create, cr, v1beta1Codec, false)
			req.OldObject = req.Object
			req.Object = runtime.RawExtension{}

			res := wh.Handle(ctx, req)
			Expect(res.Allowed).To(BeFalse())
			Expect(res.Result.Code).To(Equal(int32(400)))
			Expect(res.Result.Message).To(Equal("there is no content to decode"))

			req = newRequest(admissionv1.Create, cr, v1beta1Codec, false)
			req.Operation = "MALFORMED"

			res = wh.Handle(ctx, req)
			Expect(res.Allowed).To(BeFalse())
			Expect(res.Result.Code).To(Equal(int32(400)))
			Expect(res.Result.Message).To(Equal("unknown operation request \"MALFORMED\""))
		})

		It("should accept creation of a resource with a valid namespace", func() {
			Expect(wh.ValidateCreate(ctx, dryRun, cr)).To(Succeed())
		})

		DescribeTable("Validate annotations", func(annotations map[string]string, assertion types.GomegaMatcher) {
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
			It("should allow unique PCI Host Device", func() {
				cr.Spec.PermittedHostDevices = &v1beta1.PermittedHostDevices{
					PciHostDevices: []v1beta1.PciHostDevice{
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

			It("should allow unique Mediate Host Device", func() {
				cr.Spec.PermittedHostDevices = &v1beta1.PermittedHostDevices{
					MediatedDevices: []v1beta1.MediatedHostDevice{
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
			var image1, image2, image3, image4 v1beta1.DataImportCronTemplate

			var dryRun bool
			var ctx context.Context

			BeforeEach(func() {
				dryRun = false
				ctx = context.TODO()

				image1 = v1beta1.DataImportCronTemplate{
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

				image2 = v1beta1.DataImportCronTemplate{
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

				image3 = v1beta1.DataImportCronTemplate{
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

				image4 = v1beta1.DataImportCronTemplate{
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

				cr.Spec.DataImportCronTemplates = []v1beta1.DataImportCronTemplate{image1, image2, image3, image4}
			})

			It("should allow setting the annotation to true", func() {
				cr.Spec.DataImportCronTemplates[0].Annotations = map[string]string{util.DataImportCronEnabledAnnotation: "true"}
				cr.Spec.DataImportCronTemplates[1].Annotations = map[string]string{util.DataImportCronEnabledAnnotation: "TRUE"}
				cr.Spec.DataImportCronTemplates[2].Annotations = map[string]string{util.DataImportCronEnabledAnnotation: "TrUe"}
				cr.Spec.DataImportCronTemplates[3].Annotations = map[string]string{util.DataImportCronEnabledAnnotation: "tRuE"}

				Expect(wh.ValidateCreate(ctx, dryRun, cr)).To(Succeed())
			})

			It("should allow setting the annotation to false", func() {
				cr.Spec.DataImportCronTemplates[0].Annotations = map[string]string{util.DataImportCronEnabledAnnotation: "false"}
				cr.Spec.DataImportCronTemplates[1].Annotations = map[string]string{util.DataImportCronEnabledAnnotation: "FALSE"}
				cr.Spec.DataImportCronTemplates[2].Annotations = map[string]string{util.DataImportCronEnabledAnnotation: "FaLsE"}
				cr.Spec.DataImportCronTemplates[3].Annotations = map[string]string{util.DataImportCronEnabledAnnotation: "fAlSe"}

				Expect(wh.ValidateCreate(ctx, dryRun, cr)).To(Succeed())
			})

			It("should allow setting no annotation", func() {
				Expect(wh.ValidateCreate(ctx, dryRun, cr)).To(Succeed())
			})

			It("should not allow empty annotation", func() {
				cr.Spec.DataImportCronTemplates[0].Annotations = map[string]string{util.DataImportCronEnabledAnnotation: ""}
				cr.Spec.DataImportCronTemplates[1].Annotations = map[string]string{util.DataImportCronEnabledAnnotation: ""}

				Expect(wh.ValidateCreate(ctx, dryRun, cr)).ToNot(Succeed())
			})

			It("should not allow unknown annotation values", func() {
				cr.Spec.DataImportCronTemplates[0].Annotations = map[string]string{util.DataImportCronEnabledAnnotation: "wrong"}
				cr.Spec.DataImportCronTemplates[1].Annotations = map[string]string{util.DataImportCronEnabledAnnotation: "mistake"}

				Expect(wh.ValidateCreate(ctx, dryRun, cr)).ToNot(Succeed())
			})

			Context("Empty DICT spec", func() {
				It("don't allow if the annotation does not exist", func() {
					// empty annotation map
					cr.Spec.DataImportCronTemplates[0].Annotations = map[string]string{}
					cr.Spec.DataImportCronTemplates[0].Spec = nil
					// no annotation map
					cr.Spec.DataImportCronTemplates[1].Spec = nil

					Expect(wh.ValidateCreate(ctx, dryRun, cr)).ToNot(Succeed())
				})

				It("don't allow if the annotation is true", func() {
					cr.Spec.DataImportCronTemplates[0].Annotations = map[string]string{util.DataImportCronEnabledAnnotation: "True"}
					cr.Spec.DataImportCronTemplates[0].Spec = nil
					cr.Spec.DataImportCronTemplates[1].Annotations = map[string]string{util.DataImportCronEnabledAnnotation: "true"}
					cr.Spec.DataImportCronTemplates[1].Spec = nil

					Expect(wh.ValidateCreate(ctx, dryRun, cr)).ToNot(Succeed())
				})

				It("allow if the annotation is false", func() {
					cr.Spec.DataImportCronTemplates[0].Annotations = map[string]string{util.DataImportCronEnabledAnnotation: "False"}
					cr.Spec.DataImportCronTemplates[0].Spec = nil
					cr.Spec.DataImportCronTemplates[1].Annotations = map[string]string{util.DataImportCronEnabledAnnotation: "false"}
					cr.Spec.DataImportCronTemplates[1].Spec = nil

					Expect(wh.ValidateCreate(ctx, dryRun, cr)).To(Succeed())
				})
			})
		})

		Context("validate tlsSecurityProfiles", func() {
			var dryRun bool
			var ctx context.Context

			BeforeEach(func() {
				dryRun = false
				ctx = context.TODO()
			})

			updateTLSSecurityProfile := func(minTLSVersion openshiftconfigv1.TLSProtocolVersion, ciphers []string) error {
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
				func(cipher string) {
					Expect(
						updateTLSSecurityProfile(openshiftconfigv1.VersionTLS12, []string{"DHE-RSA-AES256-GCM-SHA384", cipher, "DHE-RSA-CHACHA20-POLY1305"}),
					).To(Succeed())
				},
				Entry("ECDHE-RSA-AES128-GCM-SHA256", "ECDHE-RSA-AES128-GCM-SHA256"),
				Entry("ECDHE-ECDSA-AES128-GCM-SHA256", "ECDHE-ECDSA-AES128-GCM-SHA256"),
			)

			It("should fail if does not have any of the HTTP/2-required ciphers", func() {
				err := updateTLSSecurityProfile(openshiftconfigv1.VersionTLS12, []string{"DHE-RSA-AES256-GCM-SHA384", "DHE-RSA-CHACHA20-POLY1305"})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("http2: TLSConfig.CipherSuites is missing an HTTP/2-required AES_128_GCM_SHA256 cipher (need at least one of ECDHE-RSA-AES128-GCM-SHA256 or ECDHE-ECDSA-AES128-GCM-SHA256)"))
			})

			It("should succeed if does not have any of the HTTP/2-required ciphers but TLS version >= 1.3", func() {
				Expect(
					updateTLSSecurityProfile(openshiftconfigv1.VersionTLS13, []string{}),
				).To(Succeed())
			})

			It("should fail if does have custom ciphers with TLS version >= 1.3", func() {
				err := updateTLSSecurityProfile(openshiftconfigv1.VersionTLS13, []string{"TLS_AES_128_GCM_SHA256", "TLS_CHACHA20_POLY1305_SHA256"})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("custom ciphers cannot be selected when minTLSVersion is VersionTLS13"))
			})

			It("should fail when minTLSVersion is invalid", func() {
				err := updateTLSSecurityProfile("invalidProtocolVersion", []string{"TLS_AES_128_GCM_SHA256", "TLS_CHACHA20_POLY1305_SHA256"})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid value for spec.tlsSecurityProfile.custom.minTLSVersion"))
			})

			It("should fail when type is Custom but custom field is nil", func() {
				cr.Spec.TLSSecurityProfile = &openshiftconfigv1.TLSSecurityProfile{
					Type:   openshiftconfigv1.TLSProfileCustomType,
					Custom: nil,
				}

				Expect(wh.ValidateCreate(ctx, dryRun, cr)).To(MatchError(ContainSubstring("missing required field spec.tlsSecurityProfile.custom when type is Custom")))
			})
		})

		Context("validate deprecated FGs", func() {
			DescribeTable("should return warning for deprecated feature gate", func(fgs v1beta1.HyperConvergedFeatureGates, fgNames ...string) {
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
					v1beta1.HyperConvergedFeatureGates{WithHostPassthroughCPU: ptr.To(false)}, "withHostPassthroughCPU"),
				Entry("should trigger a warning if the withHostPassthroughCPU=true FG exists in the CR",
					v1beta1.HyperConvergedFeatureGates{WithHostPassthroughCPU: ptr.To(true)}, "withHostPassthroughCPU"),

				Entry("should trigger a warning if the deployTektonTaskResources=false FG exists in the CR",
					v1beta1.HyperConvergedFeatureGates{DeployTektonTaskResources: ptr.To(false)}, "deployTektonTaskResources"),
				Entry("should trigger a warning if the deployTektonTaskResources=true FG exists in the CR",
					v1beta1.HyperConvergedFeatureGates{DeployTektonTaskResources: ptr.To(true)}, "deployTektonTaskResources"),

				Entry("should trigger a warning if the enableManagedTenantQuota=false FG exists in the CR",
					v1beta1.HyperConvergedFeatureGates{EnableManagedTenantQuota: ptr.To(false)}, "enableManagedTenantQuota"),
				Entry("should trigger a warning if the enableManagedTenantQuota=true FG exists in the CR",
					v1beta1.HyperConvergedFeatureGates{EnableManagedTenantQuota: ptr.To(true)}, "enableManagedTenantQuota"),

				Entry("should trigger a warning if the nonRoot=false FG exists in the CR",
					v1beta1.HyperConvergedFeatureGates{NonRoot: ptr.To(false)}, "nonRoot"),
				Entry("should trigger a warning if the nonRoot=true FG exists in the CR",
					v1beta1.HyperConvergedFeatureGates{NonRoot: ptr.To(true)}, "nonRoot"),

				Entry("should trigger multiple warnings if several deprecated FG exist in the CR",
					v1beta1.HyperConvergedFeatureGates{
						NonRoot:                  ptr.To(true),
						EnableManagedTenantQuota: ptr.To(true),
					}, "enableManagedTenantQuota", "nonRoot"),

				Entry("should trigger multiple warnings if several deprecated FG exist in the CR, with some valid FGs",
					v1beta1.HyperConvergedFeatureGates{
						DownwardMetrics:             ptr.To(true),
						NonRoot:                     ptr.To(false),
						EnableCommonBootImageImport: ptr.To(true),
						EnableApplicationAwareQuota: ptr.To(false),
						EnableManagedTenantQuota:    ptr.To(false),
						DeployVMConsoleProxy:        ptr.To(false),
						DeployKubeSecondaryDNS:      ptr.To(false),
					}, "enableManagedTenantQuota", "nonRoot", "enableApplicationAwareQuota", "enableCommonBootImageImport", "deployVmConsoleProxy"),
			)

			It("should return warning when disableMDevConfiguration is set", func() {
				//nolint:staticcheck // Testing deprecated FG warning.
				cr.Spec.FeatureGates.DisableMDevConfiguration = ptr.To(true)
				cr.Spec.MediatedDevicesConfiguration = &v1beta1.MediatedDevicesConfiguration{
					MediatedDeviceTypes: []string{"nvidia-222"},
					Enabled:             ptr.To(false),
				}
				err := wh.ValidateCreate(ctx, dryRun, cr)
				Expect(err).To(HaveOccurred())
				var vw *ValidationWarning
				Expect(errors.As(err, &vw)).To(BeTrue())
				Expect(vw.Warnings()).To(ContainElement(ContainSubstring("disableMDevConfiguration")))
			})
		})

		Context("validate MDev feature gate and enabled", func() {
			DescribeTable("create: reject when FG and enabled are inconsistent",
				func(fg *bool, enabled *bool, expectReject bool) {
					//nolint:staticcheck // Testing deprecated FG validation.
					cr.Spec.FeatureGates.DisableMDevConfiguration = fg
					cr.Spec.MediatedDevicesConfiguration = &v1beta1.MediatedDevicesConfiguration{
						MediatedDeviceTypes: []string{"nvidia-222"},
						Enabled:             enabled,
					}
					err := wh.ValidateCreate(ctx, dryRun, cr)
					if expectReject {
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("disableMDevConfiguration"))
						Expect(err.Error()).To(ContainSubstring("enabled"))
					} else {
						// Success or deprecation warning (ValidationWarning) are both allowed
						if err != nil {
							var vw *ValidationWarning
							Expect(errors.As(err, &vw)).To(BeTrue())
						}
					}
				},
				Entry("reject FG true and enabled true", ptr.To(true), ptr.To(true), true),
				Entry("reject FG false and enabled false", ptr.To(false), ptr.To(false), true),
				Entry("allow FG nil (ignore FG)", nil, ptr.To(true), false),
				Entry("allow FG nil and enabled nil", nil, nil, false),
				Entry("allow FG true and enabled false", ptr.To(true), ptr.To(false), false),
				Entry("allow FG false and enabled true", ptr.To(false), ptr.To(true), false),
				Entry("allow FG false and enabled nil (default true)", ptr.To(false), nil, false),
			)
		})

		Context("validate affinity", func() {
			It("should allow empty affinity", func() {
				cr.Spec.Infra.NodePlacement = &sdkapi.NodePlacement{
					Affinity: nil,
				}
				cr.Spec.Workloads.NodePlacement = &sdkapi.NodePlacement{
					Affinity: nil,
				}

				Expect(wh.ValidateCreate(ctx, dryRun, cr)).To(Succeed())
			})

			It("should allow empty affinity", func() {
				cr.Spec.Infra.NodePlacement = &sdkapi.NodePlacement{
					Affinity: &corev1.Affinity{},
				}
				cr.Spec.Workloads.NodePlacement = &sdkapi.NodePlacement{
					Affinity: &corev1.Affinity{},
				}

				Expect(wh.ValidateCreate(ctx, dryRun, cr)).To(Succeed())
			})

			It("should allow valid affinity", func() {
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

			It("should reject invalid workloads affinity: unknown operator", func() {
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

			It("should reject invalid workloads affinity: more than one value in matchFields", func() {
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

			It("should reject invalid infra affinity: unknown operator", func() {
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

			It("should reject invalid infra affinity: more than one value in fieldSelector", func() {
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
			It("should return warning for deprecated highBurst tuning policy", func() {
				cr.Spec.TuningPolicy = v1beta1.HyperConvergedHighBurstProfile //nolint SA1019
				err := wh.ValidateCreate(ctx, dryRun, cr)
				Expect(err).To(HaveOccurred())
				expected := &ValidationWarning{}
				Expect(errors.As(err, &expected)).To(BeTrue())
				Expect(expected.warnings).To(HaveLen(1))
				Expect(expected.warnings[0]).To(ContainSubstring("highBurst profile is deprecated"))
				Expect(expected.warnings[0]).To(ContainSubstring("v1.16.0"))
			})

			It("should not return warning when tuning policy is not set", func() {
				cr.Spec.TuningPolicy = ""
				Expect(wh.ValidateCreate(ctx, dryRun, cr)).To(Succeed())
			})
		})
	})

	Context("validate update validation webhook", func() {

		var hco *v1beta1.HyperConverged
		var dryRun bool
		var ctx context.Context

		BeforeEach(func() {
			hco = commontestutils.NewHco()
			hco.Spec.Infra = v1beta1.HyperConvergedConfig{
				NodePlacement: newHyperConvergedConfig(),
			}
			hco.Spec.Workloads = v1beta1.HyperConvergedConfig{
				NodePlacement: newHyperConvergedConfig(),
			}
			dryRun = false
			ctx = context.TODO()
		})

		It("should correctly handle a valid update request", func() {
			req := newRequest(admissionv1.Update, hco, v1beta1Codec, false)

			res := wh.Handle(ctx, req)
			Expect(res.Allowed).To(BeTrue())
			Expect(res.Result.Code).To(Equal(int32(200)))
		})

		It("should correctly handle a valid dryrun update request", func() {
			req := newRequest(admissionv1.Update, hco, v1beta1Codec, true)

			res := wh.Handle(ctx, req)
			Expect(res.Allowed).To(BeTrue())
			Expect(res.Result.Code).To(Equal(int32(200)))
		})

		It("should reject malformed update requests", func() {
			req := newRequest(admissionv1.Update, hco, v1beta1Codec, false)
			req.Object = runtime.RawExtension{}

			res := wh.Handle(ctx, req)
			Expect(res.Allowed).To(BeFalse())
			Expect(res.Result.Code).To(Equal(int32(400)))
			Expect(res.Result.Message).To(Equal("there is no content to decode"))

			req = newRequest(admissionv1.Update, hco, v1beta1Codec, false)
			req.OldObject = runtime.RawExtension{}

			res = wh.Handle(ctx, req)
			Expect(res.Allowed).To(BeFalse())
			Expect(res.Result.Code).To(Equal(int32(400)))
			Expect(res.Result.Message).To(Equal("there is no content to decode"))
		})

		Context("validate MDev feature gate and enabled on update", func() {
			It("reject when both FG and enabled changed and are inconsistent", func() {
				exists := commontestutils.NewHco()
				//nolint:staticcheck // Testing deprecated FG.
				exists.Spec.FeatureGates.DisableMDevConfiguration = ptr.To(false)
				exists.Spec.MediatedDevicesConfiguration = &v1beta1.MediatedDevicesConfiguration{
					MediatedDeviceTypes: []string{"nvidia-222"},
					Enabled:             ptr.To(false),
				}
				requested := exists.DeepCopy()
				//nolint:staticcheck // Testing deprecated FG.
				requested.Spec.FeatureGates.DisableMDevConfiguration = ptr.To(true)
				// Use a new MediatedDevicesConfiguration so we don't share the pointer with exists
				requested.Spec.MediatedDevicesConfiguration = &v1beta1.MediatedDevicesConfiguration{
					MediatedDeviceTypes: []string{"nvidia-222"},
					Enabled:             ptr.To(true), // both changed; inconsistent: FG true but enabled true
				}
				err := wh.ValidateUpdate(ctx, dryRun, requested, exists)
				Expect(err).To(HaveOccurred())
				Expect(err).NotTo(BeAssignableToTypeOf(&ValidationWarning{}))
				Expect(err.Error()).To(ContainSubstring("both changed"))
				Expect(err.Error()).To(ContainSubstring("disableMDevConfiguration"))
			})
			It("allow when only enabled changed (mutator will sync FG)", func() {
				exists := commontestutils.NewHco()
				//nolint:staticcheck // Testing deprecated FG.
				exists.Spec.FeatureGates.DisableMDevConfiguration = ptr.To(true)
				exists.Spec.MediatedDevicesConfiguration = &v1beta1.MediatedDevicesConfiguration{
					MediatedDeviceTypes: []string{"nvidia-222"},
					Enabled:             ptr.To(false),
				}
				requested := exists.DeepCopy()
				requested.Spec.MediatedDevicesConfiguration.Enabled = ptr.To(true) // only enabled changed; mutator will set FG = false
				err := wh.ValidateUpdate(ctx, dryRun, requested, exists)
				if err != nil {
					var vw *ValidationWarning
					Expect(errors.As(err, &vw)).To(BeTrue())
				}
			})
			It("allow when only FG changed", func() {
				exists := commontestutils.NewHco()
				//nolint:staticcheck // Testing deprecated FG.
				exists.Spec.FeatureGates.DisableMDevConfiguration = ptr.To(true)
				exists.Spec.MediatedDevicesConfiguration = &v1beta1.MediatedDevicesConfiguration{
					MediatedDeviceTypes: []string{"nvidia-222"},
					Enabled:             ptr.To(false),
				}
				requested := exists.DeepCopy()
				//nolint:staticcheck // Testing deprecated FG.
				requested.Spec.FeatureGates.DisableMDevConfiguration = ptr.To(false)
				// enabled stays false; mutator will set it to nil. Validator: only FG changed so we don't reject.
				err := wh.ValidateUpdate(ctx, dryRun, requested, exists)
				if err != nil {
					var vw *ValidationWarning
					Expect(errors.As(err, &vw)).To(BeTrue())
				}
			})
		})

		It("should return error if KV CR is missing", func() {
			ctx := context.TODO()
			cli := getFakeClient(hco)

			kv := handlers.NewKubeVirtWithNameOnly(hco)
			Expect(cli.Delete(ctx, kv)).To(Succeed())

			wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true)
			tlssecprofile.SetHyperConvergedTLSSecurityProfile(nil)

			newHco := &v1beta1.HyperConverged{}
			hco.DeepCopyInto(newHco)
			// just do some change to force update
			newHco.Spec.Infra.NodePlacement.NodeSelector["key3"] = "value3"

			err := wh.ValidateUpdate(ctx, dryRun, newHco, hco)
			Expect(err).To(MatchError(apierrors.IsNotFound, "not found error"))
			Expect(err).To(MatchError(ContainSubstring("kubevirts.kubevirt.io")))
		})

		It("should return error if dry-run update of KV CR returns error", func() {
			cli := getFakeClient(hco)
			cli.InitiateUpdateErrors(getUpdateError(kvUpdateFailure))

			wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true)
			tlssecprofile.SetHyperConvergedTLSSecurityProfile(nil)

			newHco := &v1beta1.HyperConverged{}
			hco.DeepCopyInto(newHco)
			// change something in workloads to trigger dry-run update
			newHco.Spec.Workloads.NodePlacement.NodeSelector["a change"] = "Something else"

			err := wh.ValidateUpdate(ctx, dryRun, newHco, hco)
			Expect(err).To(MatchError(ErrFakeKvError))
		})

		It("should return error if CDI CR is missing", func() {
			ctx := context.TODO()
			cli := getFakeClient(hco)
			cdi, err := handlers.NewCDI(hco)
			Expect(err).ToNot(HaveOccurred())
			Expect(cli.Delete(ctx, cdi)).To(Succeed())

			wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true)
			tlssecprofile.SetHyperConvergedTLSSecurityProfile(nil)

			newHco := &v1beta1.HyperConverged{}
			hco.DeepCopyInto(newHco)
			// just do some change to force update
			newHco.Spec.Infra.NodePlacement.NodeSelector["key3"] = "value3"

			err = wh.ValidateUpdate(ctx, dryRun, newHco, hco)
			Expect(err).To(MatchError(apierrors.IsNotFound, "not found error"))
			Expect(err).To(MatchError(ContainSubstring("cdis.cdi.kubevirt.io")))
		})

		It("should return error if dry-run update of CDI CR returns error", func() {
			cli := getFakeClient(hco)
			cli.InitiateUpdateErrors(getUpdateError(cdiUpdateFailure))
			wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true)

			newHco := &v1beta1.HyperConverged{}
			hco.DeepCopyInto(newHco)
			// change something in workloads to trigger dry-run update
			newHco.Spec.Workloads.NodePlacement.NodeSelector["a change"] = "Something else"

			err := wh.ValidateUpdate(ctx, dryRun, newHco, hco)
			Expect(err).To(MatchError(ErrFakeCdiError))
		})

		It("should not return error if dry-run update of ALL CR passes", func() {
			cli := getFakeClient(hco)
			cli.InitiateUpdateErrors(getUpdateError(noFailure))

			wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true)

			newHco := &v1beta1.HyperConverged{}
			hco.DeepCopyInto(newHco)
			// change something in workloads to trigger dry-run update
			newHco.Spec.Workloads.NodePlacement.NodeSelector["a change"] = "Something else"

			Expect(wh.ValidateUpdate(ctx, dryRun, newHco, hco)).To(Succeed())
		})

		It("should return error if NetworkAddons CR is missing", func() {
			ctx := context.TODO()
			cli := getFakeClient(hco)
			cna, err := handlers.NewNetworkAddons(hco)
			Expect(err).ToNot(HaveOccurred())
			Expect(cli.Delete(ctx, cna)).To(Succeed())
			wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true)

			newHco := &v1beta1.HyperConverged{}
			hco.DeepCopyInto(newHco)
			// just do some change to force update
			newHco.Spec.Infra.NodePlacement.NodeSelector["key3"] = "value3"

			err = wh.ValidateUpdate(ctx, dryRun, newHco, hco)
			Expect(err).To(MatchError(apierrors.IsNotFound, "not found error"))
			Expect(err).To(MatchError(ContainSubstring("networkaddonsconfigs.networkaddonsoperator.network.kubevirt.io")))
		})

		It("should return error if dry-run update of NetworkAddons CR returns error", func() {
			cli := getFakeClient(hco)
			cli.InitiateUpdateErrors(getUpdateError(networkUpdateFailure))

			wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true)

			newHco := &v1beta1.HyperConverged{}
			hco.DeepCopyInto(newHco)
			// change something in workloads to trigger dry-run update
			newHco.Spec.Workloads.NodePlacement.NodeSelector["a change"] = "Something else"

			err := wh.ValidateUpdate(ctx, dryRun, newHco, hco)
			Expect(err).To(MatchError(ErrFakeNetworkError))
		})

		It("should return error if SSP CR is missing", func() {
			ctx := context.TODO()
			cli := getFakeClient(hco)

			Expect(cli.Delete(ctx, handlers.NewSSPWithNameOnly(hco))).To(Succeed())
			wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true)

			newHco := &v1beta1.HyperConverged{}
			hco.DeepCopyInto(newHco)
			// just do some change to force update
			newHco.Spec.Infra.NodePlacement.NodeSelector["key3"] = "value3"

			err := wh.ValidateUpdate(ctx, dryRun, newHco, hco)
			Expect(err).To(MatchError(apierrors.IsNotFound, "not found error"))
			Expect(err).To(MatchError(ContainSubstring("ssps.ssp.kubevirt.io")))
		})

		It("should return error if dry-run update of SSP CR returns error", func() {
			cli := getFakeClient(hco)
			cli.InitiateUpdateErrors(getUpdateError(sspUpdateFailure))
			wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true)

			newHco := &v1beta1.HyperConverged{}
			hco.DeepCopyInto(newHco)
			// change something in workloads to trigger dry-run update
			newHco.Spec.Workloads.NodePlacement.NodeSelector["a change"] = "Something else"

			err := wh.ValidateUpdate(ctx, dryRun, newHco, hco)
			Expect(err).To(MatchError(ErrFakeSspError))

		})

		It("should return error if dry-run update is timeout", func() {
			cli := getFakeClient(hco)
			cli.InitiateUpdateErrors(initiateTimeout)

			wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true)

			newHco := &v1beta1.HyperConverged{}
			hco.DeepCopyInto(newHco)
			// change something in workloads to trigger dry-run update
			newHco.Spec.Workloads.NodePlacement.NodeSelector["a change"] = "Something else"

			err := wh.ValidateUpdate(ctx, dryRun, newHco, hco)
			Expect(err).To(MatchError(context.DeadlineExceeded))
		})

		It("should not return error if nothing was changed", func() {
			cli := getFakeClient(hco)
			cli.InitiateUpdateErrors(initiateTimeout)

			wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true)

			newHco := &v1beta1.HyperConverged{}
			hco.DeepCopyInto(newHco)

			Expect(wh.ValidateUpdate(ctx, dryRun, newHco, hco)).To(Succeed())
		})

		Context("test permitted host devices update validation", func() {
			It("should allow unique PCI Host Device", func() {
				cli := getFakeClient(hco)
				wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true)

				newHco := &v1beta1.HyperConverged{}
				hco.DeepCopyInto(newHco)
				newHco.Spec.PermittedHostDevices = &v1beta1.PermittedHostDevices{
					PciHostDevices: []v1beta1.PciHostDevice{
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

			It("should allow unique Mediate Host Device", func() {
				cli := getFakeClient(hco)
				wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true)

				newHco := &v1beta1.HyperConverged{}
				hco.DeepCopyInto(newHco)
				newHco.Spec.PermittedHostDevices = &v1beta1.PermittedHostDevices{
					MediatedDevices: []v1beta1.MediatedHostDevice{
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
			It("should return error in plain-k8s if KV CR is missing", func() {
				hco := &v1beta1.HyperConverged{}
				ctx := context.TODO()
				cli := getFakeClient(hco)
				kv, err := handlers.NewKubeVirt(hco)
				Expect(err).ToNot(HaveOccurred())
				Expect(cli.Delete(ctx, kv)).To(Succeed())
				wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, false)

				newHco := commontestutils.NewHco()
				newHco.Spec.Infra = v1beta1.HyperConvergedConfig{
					NodePlacement: newHyperConvergedConfig(),
				}
				newHco.Spec.Workloads = v1beta1.HyperConvergedConfig{
					NodePlacement: newHyperConvergedConfig(),
				}

				Expect(
					wh.ValidateUpdate(ctx, dryRun, newHco, hco),
				).To(MatchError(apierrors.IsNotFound, "not found error"))
			})
		})

		Context("Check LiveMigrationConfiguration", func() {
			var hco *v1beta1.HyperConverged

			BeforeEach(func() {
				hco = commontestutils.NewHco()
			})

			It("should ignore if there is no change in live migration", func() {
				cli := getFakeClient(hco)

				// Deleting KV here, in order to make sure the that the webhook does not find differences,
				// and so it exits with no error before finding that KV is not there.
				// Later we'll check that there is no error from the webhook, and that will prove that
				// the comparison works.
				kv := handlers.NewKubeVirtWithNameOnly(hco)
				Expect(cli.Delete(context.TODO(), kv)).To(Succeed())

				wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true)

				newHco := &v1beta1.HyperConverged{}
				hco.DeepCopyInto(newHco)

				Expect(wh.ValidateUpdate(ctx, dryRun, newHco, hco)).To(Succeed())
			})

			It("should allow updating of live migration", func() {
				cli := getFakeClient(hco)

				wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true)

				newHco := &v1beta1.HyperConverged{}
				hco.DeepCopyInto(newHco)

				// change something in the LiveMigrationConfig field
				hco.Spec.LiveMigrationConfig.CompletionTimeoutPerGiB = ptr.To[int64](200)

				Expect(wh.ValidateUpdate(ctx, dryRun, newHco, hco)).To(Succeed())
			})

			It("should fail if live migration is wrong", func() {
				cli := getFakeClient(hco)

				wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true)

				newHco := &v1beta1.HyperConverged{}
				hco.DeepCopyInto(newHco)

				// change something in the LiveMigrationConfig field
				newHco.Spec.LiveMigrationConfig.BandwidthPerMigration = ptr.To("Wrong Value")

				Expect(
					wh.ValidateUpdate(ctx, dryRun, newHco, hco),
				).To(MatchError(ContainSubstring("failed to parse the LiveMigrationConfig.bandwidthPerMigration field")))
			})
		})

		Context("Check CertRotation", func() {
			var hco *v1beta1.HyperConverged

			BeforeEach(func() {
				hco = commontestutils.NewHco()
			})

			It("should ignore if there is no change in cert config", func() {
				cli := getFakeClient(hco)

				// Deleting KV here, in order to make sure the that the webhook does not find differences,
				// and so it exits with no error before finding that KV is not there.
				// Later we'll check that there is no error from the webhook, and that will prove that
				// the comparison works.
				kv := handlers.NewKubeVirtWithNameOnly(hco)
				Expect(cli.Delete(context.TODO(), kv)).To(Succeed())

				wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true)

				newHco := &v1beta1.HyperConverged{}
				hco.DeepCopyInto(newHco)

				Expect(wh.ValidateUpdate(ctx, dryRun, newHco, hco)).To(Succeed())
			})

			It("should allow updating of cert config", func() {
				cli := getFakeClient(hco)

				wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true)

				newHco := &v1beta1.HyperConverged{}
				hco.DeepCopyInto(newHco)

				// change something in the CertConfig fields
				newHco.Spec.CertConfig.CA.Duration.Duration = hco.Spec.CertConfig.CA.Duration.Duration * 2
				newHco.Spec.CertConfig.CA.RenewBefore.Duration = hco.Spec.CertConfig.CA.RenewBefore.Duration * 2
				newHco.Spec.CertConfig.Server.Duration.Duration = hco.Spec.CertConfig.Server.Duration.Duration * 2
				newHco.Spec.CertConfig.Server.RenewBefore.Duration = hco.Spec.CertConfig.Server.RenewBefore.Duration * 2

				Expect(wh.ValidateUpdate(ctx, dryRun, newHco, hco)).To(Succeed())
			})

			DescribeTable("should fail if cert config is wrong",
				func(newHco v1beta1.HyperConverged, errorMsg string) {
					cli := getFakeClient(hco)

					wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true)

					err := wh.ValidateUpdate(ctx, dryRun, &newHco, hco)
					Expect(err).To(MatchError(ContainSubstring(errorMsg)))
				},
				Entry("certConfig.ca.duration is too short",
					v1beta1.HyperConverged{
						ObjectMeta: metav1.ObjectMeta{
							Name:      util.HyperConvergedName,
							Namespace: HcoValidNamespace,
						},
						Spec: v1beta1.HyperConvergedSpec{
							CertConfig: v1beta1.HyperConvergedCertConfig{
								CA: v1beta1.CertRotateConfigCA{
									Duration:    &metav1.Duration{Duration: 8 * time.Minute},
									RenewBefore: &metav1.Duration{Duration: 24 * time.Hour},
								},
								Server: v1beta1.CertRotateConfigServer{
									Duration:    &metav1.Duration{Duration: 24 * time.Hour},
									RenewBefore: &metav1.Duration{Duration: 12 * time.Hour},
								},
							},
						},
					},
					"spec.certConfig.ca.duration: value is too small"),
				Entry("certConfig.ca.renewBefore is too short",
					v1beta1.HyperConverged{
						ObjectMeta: metav1.ObjectMeta{
							Name:      util.HyperConvergedName,
							Namespace: HcoValidNamespace,
						},
						Spec: v1beta1.HyperConvergedSpec{
							CertConfig: v1beta1.HyperConvergedCertConfig{
								CA: v1beta1.CertRotateConfigCA{
									Duration:    &metav1.Duration{Duration: 48 * time.Hour},
									RenewBefore: &metav1.Duration{Duration: 8 * time.Minute},
								},
								Server: v1beta1.CertRotateConfigServer{
									Duration:    &metav1.Duration{Duration: 24 * time.Hour},
									RenewBefore: &metav1.Duration{Duration: 12 * time.Hour},
								},
							},
						},
					},
					"spec.certConfig.ca.renewBefore: value is too small"),
				Entry("certConfig.server.duration is too short",
					v1beta1.HyperConverged{
						ObjectMeta: metav1.ObjectMeta{
							Name:      util.HyperConvergedName,
							Namespace: HcoValidNamespace,
						},
						Spec: v1beta1.HyperConvergedSpec{
							CertConfig: v1beta1.HyperConvergedCertConfig{
								CA: v1beta1.CertRotateConfigCA{
									Duration:    &metav1.Duration{Duration: 48 * time.Hour},
									RenewBefore: &metav1.Duration{Duration: 24 * time.Hour},
								},
								Server: v1beta1.CertRotateConfigServer{
									Duration:    &metav1.Duration{Duration: 8 * time.Minute},
									RenewBefore: &metav1.Duration{Duration: 12 * time.Hour},
								},
							},
						},
					},
					"spec.certConfig.server.duration: value is too small"),
				Entry("certConfig.server.renewBefore is too short",
					v1beta1.HyperConverged{
						ObjectMeta: metav1.ObjectMeta{
							Name:      util.HyperConvergedName,
							Namespace: HcoValidNamespace,
						},
						Spec: v1beta1.HyperConvergedSpec{
							CertConfig: v1beta1.HyperConvergedCertConfig{
								CA: v1beta1.CertRotateConfigCA{
									Duration:    &metav1.Duration{Duration: 48 * time.Hour},
									RenewBefore: &metav1.Duration{Duration: 24 * time.Hour},
								},
								Server: v1beta1.CertRotateConfigServer{
									Duration:    &metav1.Duration{Duration: 24 * time.Hour},
									RenewBefore: &metav1.Duration{Duration: 8 * time.Minute},
								},
							},
						},
					},
					"spec.certConfig.server.renewBefore: value is too small"),
				Entry("ca: duration is smaller than renewBefore",
					v1beta1.HyperConverged{
						ObjectMeta: metav1.ObjectMeta{
							Name:      util.HyperConvergedName,
							Namespace: HcoValidNamespace,
						},
						Spec: v1beta1.HyperConvergedSpec{
							CertConfig: v1beta1.HyperConvergedCertConfig{
								CA: v1beta1.CertRotateConfigCA{
									Duration:    &metav1.Duration{Duration: 23 * time.Hour},
									RenewBefore: &metav1.Duration{Duration: 24 * time.Hour},
								},
								Server: v1beta1.CertRotateConfigServer{
									Duration:    &metav1.Duration{Duration: 24 * time.Hour},
									RenewBefore: &metav1.Duration{Duration: 12 * time.Hour},
								},
							},
						},
					},
					"spec.certConfig.ca: duration is smaller than renewBefore"),
				Entry("server: duration is smaller than renewBefore",
					v1beta1.HyperConverged{
						ObjectMeta: metav1.ObjectMeta{
							Name:      util.HyperConvergedName,
							Namespace: HcoValidNamespace,
						},
						Spec: v1beta1.HyperConvergedSpec{
							CertConfig: v1beta1.HyperConvergedCertConfig{
								CA: v1beta1.CertRotateConfigCA{
									Duration:    &metav1.Duration{Duration: 48 * time.Hour},
									RenewBefore: &metav1.Duration{Duration: 24 * time.Hour},
								},
								Server: v1beta1.CertRotateConfigServer{
									Duration:    &metav1.Duration{Duration: 11 * time.Hour},
									RenewBefore: &metav1.Duration{Duration: 12 * time.Hour},
								},
							},
						},
					},
					"spec.certConfig.server: duration is smaller than renewBefore"),
				Entry("ca.duration is smaller than server.duration",
					v1beta1.HyperConverged{
						ObjectMeta: metav1.ObjectMeta{
							Name:      util.HyperConvergedName,
							Namespace: HcoValidNamespace,
						},
						Spec: v1beta1.HyperConvergedSpec{
							CertConfig: v1beta1.HyperConvergedCertConfig{
								CA: v1beta1.CertRotateConfigCA{
									Duration:    &metav1.Duration{Duration: 48 * time.Hour},
									RenewBefore: &metav1.Duration{Duration: 24 * time.Hour},
								},
								Server: v1beta1.CertRotateConfigServer{
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
			var hco *v1beta1.HyperConverged

			BeforeEach(func() {
				hco = commontestutils.NewHco()
			})

			updateTLSSecurityProfile := func(minTLSVersion openshiftconfigv1.TLSProtocolVersion, ciphers []string) error {
				cli := getFakeClient(hco)

				wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true)

				newHco := &v1beta1.HyperConverged{}
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
				func(cipher string) {
					Expect(
						updateTLSSecurityProfile(openshiftconfigv1.VersionTLS12, []string{"DHE-RSA-AES256-GCM-SHA384", cipher, "DHE-RSA-CHACHA20-POLY1305"}),
					).To(Succeed())
				},
				Entry("ECDHE-RSA-AES128-GCM-SHA256", "ECDHE-RSA-AES128-GCM-SHA256"),
				Entry("ECDHE-ECDSA-AES128-GCM-SHA256", "ECDHE-ECDSA-AES128-GCM-SHA256"),
			)

			It("should fail if does not have any of the HTTP/2-required ciphers", func() {
				err := updateTLSSecurityProfile(openshiftconfigv1.VersionTLS12, []string{"DHE-RSA-AES256-GCM-SHA384", "DHE-RSA-CHACHA20-POLY1305"})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("http2: TLSConfig.CipherSuites is missing an HTTP/2-required AES_128_GCM_SHA256 cipher (need at least one of ECDHE-RSA-AES128-GCM-SHA256 or ECDHE-ECDSA-AES128-GCM-SHA256)"))
			})

			It("should succeed if does not have any of the HTTP/2-required ciphers but TLS version >= 1.3", func() {
				Expect(
					updateTLSSecurityProfile(openshiftconfigv1.VersionTLS13, []string{}),
				).To(Succeed())
			})

			It("should fail if does have custom ciphers with TLS version >= 1.3", func() {
				err := updateTLSSecurityProfile(openshiftconfigv1.VersionTLS13, []string{"TLS_AES_128_GCM_SHA256", "TLS_CHACHA20_POLY1305_SHA256"})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("custom ciphers cannot be selected when minTLSVersion is VersionTLS13"))
			})

			It("should fail when minTLSVersion is invalid", func() {
				err := updateTLSSecurityProfile("invalidProtocolVersion", []string{"TLS_AES_128_GCM_SHA256", "TLS_CHACHA20_POLY1305_SHA256"})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid value for spec.tlsSecurityProfile.custom.minTLSVersion"))
			})

			It("should fail when type is Custom but custom field is nil", func() {
				cli := getFakeClient(hco)

				wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true)

				newHco := hco.DeepCopy()
				newHco.Spec.TLSSecurityProfile = &openshiftconfigv1.TLSSecurityProfile{
					Type:   openshiftconfigv1.TLSProfileCustomType,
					Custom: nil,
				}

				Expect(wh.ValidateUpdate(ctx, dryRun, newHco, hco)).To(MatchError(ContainSubstring("missing required field spec.tlsSecurityProfile.custom when type is Custom")))
			})
		})

		Context("validate deprecated FGs", func() {
			DescribeTable("should return warning for deprecated feature gate", func(fgs v1beta1.HyperConvergedFeatureGates, fgNames ...string) {
				newHCO := hco.DeepCopy()
				newHCO.Spec.FeatureGates = fgs

				err := wh.ValidateUpdate(ctx, dryRun, newHCO, hco)

				Expect(err).To(HaveOccurred())
				expected := &ValidationWarning{}
				Expect(errors.As(err, &expected)).To(BeTrue())

				Expect(expected.warnings).To(HaveLen(len(fgNames)))
				for _, fgName := range fgNames {
					Expect(expected.warnings).To(ContainElements(ContainSubstring(fgName)))
				}
			},
				Entry("should trigger a warning if the withHostPassthroughCPU=false FG exists in the CR",
					v1beta1.HyperConvergedFeatureGates{WithHostPassthroughCPU: ptr.To(false)}, "withHostPassthroughCPU"),
				Entry("should trigger a warning if the withHostPassthroughCPU=true FG exists in the CR",
					v1beta1.HyperConvergedFeatureGates{WithHostPassthroughCPU: ptr.To(true)}, "withHostPassthroughCPU"),

				Entry("should trigger a warning if the deployTektonTaskResources=false FG exists in the CR",
					v1beta1.HyperConvergedFeatureGates{DeployTektonTaskResources: ptr.To(false)}, "deployTektonTaskResources"),
				Entry("should trigger a warning if the deployTektonTaskResources=true FG exists in the CR",
					v1beta1.HyperConvergedFeatureGates{DeployTektonTaskResources: ptr.To(true)}, "deployTektonTaskResources"),

				Entry("should trigger a warning if the enableManagedTenantQuota=false FG exists in the CR",
					v1beta1.HyperConvergedFeatureGates{EnableManagedTenantQuota: ptr.To(false)}, "enableManagedTenantQuota"),
				Entry("should trigger a warning if the enableManagedTenantQuota=true FG exists in the CR",
					v1beta1.HyperConvergedFeatureGates{EnableManagedTenantQuota: ptr.To(true)}, "enableManagedTenantQuota"),

				Entry("should trigger a warning if the nonRoot=false FG exists in the CR",
					v1beta1.HyperConvergedFeatureGates{NonRoot: ptr.To(false)}, "nonRoot"),
				Entry("should trigger a warning if the nonRoot=true FG exists in the CR",
					v1beta1.HyperConvergedFeatureGates{NonRoot: ptr.To(true)}, "nonRoot"),

				Entry("should trigger multiple warnings if several deprecated FG exist in the CR",
					v1beta1.HyperConvergedFeatureGates{
						NonRoot:                  ptr.To(true),
						EnableManagedTenantQuota: ptr.To(true),
					}, "enableManagedTenantQuota", "nonRoot"),

				Entry("should trigger multiple warnings if several deprecated FG exist in the CR, with some valid FGs",
					v1beta1.HyperConvergedFeatureGates{
						DownwardMetrics:             ptr.To(true),
						NonRoot:                     ptr.To(false),
						EnableCommonBootImageImport: ptr.To(true),
						EnableManagedTenantQuota:    ptr.To(false),
					}, "enableManagedTenantQuota", "nonRoot", "enableCommonBootImageImport"),
			)
		})

		Context("validate moved FG on update", func() {
			//nolint:staticcheck
			DescribeTable("should return warning for enableApplicationAwareQuota on update", func(newFG, oldFG *bool) {
				newHCO := hco.DeepCopy()
				hco.Spec.FeatureGates.EnableApplicationAwareQuota = newFG
				newHCO.Spec.FeatureGates.EnableApplicationAwareQuota = oldFG

				err := wh.ValidateUpdate(ctx, dryRun, newHCO, hco)

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
			DescribeTable("should not return warning for enableApplicationAwareQuota if not change", func(newFG, oldFG *bool) {
				cli := getFakeClient(hco)
				wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true)
				newHCO := hco.DeepCopy()
				hco.Spec.FeatureGates.EnableApplicationAwareQuota = newFG
				newHCO.Spec.FeatureGates.EnableApplicationAwareQuota = oldFG

				Expect(wh.ValidateUpdate(ctx, dryRun, newHCO, hco)).To(Succeed())
			},
				Entry("should not trigger warning if enableApplicationAwareQuota (true) disappeared", ptr.To(true), nil),
				Entry("should not trigger warning if enableApplicationAwareQuota (false) disappeared", ptr.To(false), nil),
				Entry("should not trigger warning if enableApplicationAwareQuota (true) wasn't changed", ptr.To(true), ptr.To(true)),
				Entry("should not trigger warning if enableApplicationAwareQuota (false) wasn't changed", ptr.To(false), ptr.To(false)),
			)

			//nolint:staticcheck
			DescribeTable("should return warning for enableCommonBootImageImport on update", func(newFG, oldFG *bool) {
				newHCO := hco.DeepCopy()
				hco.Spec.FeatureGates.EnableCommonBootImageImport = newFG
				newHCO.Spec.FeatureGates.EnableCommonBootImageImport = oldFG

				err := wh.ValidateUpdate(ctx, dryRun, newHCO, hco)

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
			DescribeTable("should not return warning for enableCommonBootImageImport if not change", func(newFG, oldFG *bool) {
				cli := getFakeClient(hco)
				wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true)
				newHCO := hco.DeepCopy()
				hco.Spec.FeatureGates.EnableCommonBootImageImport = newFG
				newHCO.Spec.FeatureGates.EnableCommonBootImageImport = oldFG

				Expect(wh.ValidateUpdate(ctx, dryRun, newHCO, hco)).To(Succeed())
			},
				Entry("should not trigger warning if enableCommonBootImageImport (true) disappeared", ptr.To(true), nil),
				Entry("should not trigger warning if enableCommonBootImageImport (false) disappeared", ptr.To(false), nil),
				Entry("should not trigger warning if enableCommonBootImageImport (true) wasn't changed", ptr.To(true), ptr.To(true)),
				Entry("should not trigger warning if enableCommonBootImageImport (false) wasn't changed", ptr.To(false), ptr.To(false)),
			)

			//nolint:staticcheck
			DescribeTable("should return warning for deployVmConsoleProxy on update", func(newFG, oldFG *bool) {
				newHCO := hco.DeepCopy()
				hco.Spec.FeatureGates.DeployVMConsoleProxy = newFG
				newHCO.Spec.FeatureGates.DeployVMConsoleProxy = oldFG

				err := wh.ValidateUpdate(ctx, dryRun, newHCO, hco)

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
			DescribeTable("should not return warning for deployVmConsoleProxy if not change", func(newFG, oldFG *bool) {
				cli := getFakeClient(hco)
				wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true)
				newHCO := hco.DeepCopy()
				hco.Spec.FeatureGates.DeployVMConsoleProxy = newFG
				newHCO.Spec.FeatureGates.DeployVMConsoleProxy = oldFG

				Expect(wh.ValidateUpdate(ctx, dryRun, newHCO, hco)).To(Succeed())
			},
				Entry("should not trigger warning if deployVmConsoleProxy (true) disappeared", ptr.To(true), nil),
				Entry("should not trigger warning if deployVmConsoleProxy (false) disappeared", ptr.To(false), nil),
				Entry("should not trigger warning if deployVmConsoleProxy (true) wasn't changed", ptr.To(true), ptr.To(true)),
				Entry("should not trigger warning if deployVmConsoleProxy (false) wasn't changed", ptr.To(false), ptr.To(false)),
			)

			//nolint:staticcheck
			DescribeTable("should not return warning for deployKubeSecondaryDNS if not change", func(newFG, oldFG *bool) {
				cli := getFakeClient(hco)
				wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true)
				newHCO := hco.DeepCopy()
				hco.Spec.FeatureGates.DeployKubeSecondaryDNS = newFG
				newHCO.Spec.FeatureGates.DeployKubeSecondaryDNS = oldFG

				Expect(wh.ValidateUpdate(ctx, dryRun, newHCO, hco)).To(Succeed())
			},
				Entry("should not trigger warning if deployKubeSecondaryDNS (true) disappeared", ptr.To(true), nil),
				Entry("should not trigger warning if deployKubeSecondaryDNS (false) disappeared", ptr.To(false), nil),
				Entry("should not trigger warning if deployKubeSecondaryDNS (true) wasn't changed", ptr.To(true), ptr.To(true)),
				Entry("should not trigger warning if deployKubeSecondaryDNS (false) wasn't changed", ptr.To(false), ptr.To(false)),
			)
		})

		Context("validate tuning policy on update", func() {
			It("should return warning for deprecated highBurst tuning policy", func() {
				newHCO := hco.DeepCopy()
				newHCO.Spec.TuningPolicy = v1beta1.HyperConvergedHighBurstProfile //nolint SA1019
				err := wh.ValidateUpdate(ctx, dryRun, newHCO, hco)
				Expect(err).To(HaveOccurred())
				expected := &ValidationWarning{}
				Expect(errors.As(err, &expected)).To(BeTrue())
				Expect(expected.warnings).To(HaveLen(1))
				Expect(expected.warnings[0]).To(ContainSubstring("highBurst profile is deprecated"))
				Expect(expected.warnings[0]).To(ContainSubstring("v1.16.0"))
			})

			It("should not return warning when tuning policy is not set", func() {
				newHCO := hco.DeepCopy()
				newHCO.Spec.TuningPolicy = ""
				Expect(wh.ValidateUpdate(ctx, dryRun, newHCO, hco)).To(Succeed())
			})
		})
	})

	Context("validate delete validation webhook", func() {
		var hco *v1beta1.HyperConverged
		var dryRun bool
		var ctx context.Context

		BeforeEach(func() {
			hco = &v1beta1.HyperConverged{
				ObjectMeta: metav1.ObjectMeta{
					Name:      util.HyperConvergedName,
					Namespace: HcoValidNamespace,
				},
			}
			dryRun = false
			ctx = context.TODO()
		})

		It("should correctly handle a valid delete request", func() {
			req := newRequest(admissionv1.Delete, hco, v1beta1Codec, false)

			res := wh.Handle(ctx, req)
			Expect(res.Allowed).To(BeTrue())
			Expect(res.Result.Code).To(Equal(int32(200)))
		})

		It("should correctly handle a valid dryrun delete request", func() {
			req := newRequest(admissionv1.Delete, hco, v1beta1Codec, true)

			res := wh.Handle(ctx, req)
			Expect(res.Allowed).To(BeTrue())
			Expect(res.Result.Code).To(Equal(int32(200)))
		})

		It("should reject a malformed delete request", func() {
			req := newRequest(admissionv1.Delete, hco, v1beta1Codec, false)
			req.OldObject = req.Object
			req.Object = runtime.RawExtension{}

			res := wh.Handle(ctx, req)
			Expect(res.Allowed).To(BeFalse())
			Expect(res.Result.Code).To(Equal(int32(400)))
			Expect(res.Result.Message).To(Equal("there is no content to decode"))
		})

		It("should validate deletion", func() {
			cli := getFakeClient(hco)

			wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true)

			Expect(wh.ValidateDelete(ctx, dryRun, hco)).To(Succeed())

			By("Validate that KV still exists, as it a dry-run deletion")
			kv := handlers.NewKubeVirtWithNameOnly(hco)
			Expect(util.GetRuntimeObject(context.TODO(), cli, kv)).To(Succeed())

			By("Validate that CDI still exists, as it a dry-run deletion")
			cdi := handlers.NewCDIWithNameOnly(hco)
			Expect(util.GetRuntimeObject(context.TODO(), cli, cdi)).To(Succeed())
		})

		It("should reject if KV deletion fails", func() {
			cli := getFakeClient(hco)

			wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true)

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

		It("should reject if CDI deletion fails", func() {
			cli := getFakeClient(hco)

			wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true)

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

		It("should ignore if KV does not exist", func() {
			cli := getFakeClient(hco)
			ctx := context.TODO()

			kv := handlers.NewKubeVirtWithNameOnly(hco)
			Expect(cli.Delete(ctx, kv)).To(Succeed())

			wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true)

			Expect(wh.ValidateDelete(ctx, dryRun, hco)).To(Succeed())
		})

		It("should reject if getting KV failed for not-not-exists error", func() {
			cli := getFakeClient(hco)

			wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true)

			cli.InitiateGetErrors(func(key client.ObjectKey) error {
				if key.Name == "kubevirt-kubevirt-hyperconverged" {
					return ErrFakeKvError
				}
				return nil
			})

			err := wh.ValidateDelete(ctx, dryRun, hco)
			Expect(err).To(MatchError(ErrFakeKvError))
		})

		It("should ignore if CDI does not exist", func() {
			cli := getFakeClient(hco)
			ctx := context.TODO()

			cdi := handlers.NewCDIWithNameOnly(hco)
			Expect(cli.Delete(ctx, cdi)).To(Succeed())

			wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true)

			Expect(wh.ValidateDelete(ctx, dryRun, hco)).To(Succeed())
		})

		It("should reject if getting CDI failed for not-not-exists error", func() {
			cli := getFakeClient(hco)

			wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true)

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

	Context("unsupported annotation", func() {
		var hco *v1beta1.HyperConverged
		BeforeEach(func() {
			Expect(os.Setenv("OPERATOR_NAMESPACE", HcoValidNamespace)).To(Succeed())
			hco = commontestutils.NewHco()
		})

		DescribeTable("should accept if annotation is valid",
			func(annotationName, annotation string) {
				cli := getFakeClient(hco)
				wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true)

				dryRun := false
				ctx := context.TODO()

				newHco := &v1beta1.HyperConverged{}
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
			func(annotationName, annotation string) {
				cli := getFakeClient(hco)
				cli.InitiateUpdateErrors(initiateTimeout)

				wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true)

				newHco := &v1beta1.HyperConverged{}
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

	Context("hcoTLSConfigCache", func() {
		var cr *v1beta1.HyperConverged
		var ctx context.Context
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
			Expect(os.Setenv("OPERATOR_NAMESPACE", HcoValidNamespace)).To(Succeed())
			cr = commontestutils.NewHco()
			ctx = context.TODO()
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

			It("should update hcoTLSConfigCache creating a resource not in dry run mode", func() {
				Expect(hcoTLSConfigCache).To(Equal(&initialTLSSecurityProfile))
				cr.Spec.TLSSecurityProfile = &modernTLSSecurityProfile
				Expect(wh.ValidateCreate(ctx, false, cr)).To(Succeed())
				Expect(hcoTLSConfigCache).To(Equal(&modernTLSSecurityProfile))
			})

			It("should not update hcoTLSConfigCache creating a resource in dry run mode", func() {
				Expect(hcoTLSConfigCache).To(Equal(&initialTLSSecurityProfile))
				cr.Spec.TLSSecurityProfile = &modernTLSSecurityProfile
				Expect(wh.ValidateCreate(ctx, true, cr)).To(Succeed())
				Expect(hcoTLSConfigCache).ToNot(Equal(&modernTLSSecurityProfile))
			})

			It("should not update hcoTLSConfigCache if the create request is refused", func() {
				Expect(hcoTLSConfigCache).To(Equal(&initialTLSSecurityProfile))
				cr.Spec.TLSSecurityProfile = &modernTLSSecurityProfile
				cr.Namespace = ResourceInvalidNamespace

				cr.Spec.DataImportCronTemplates = []v1beta1.DataImportCronTemplate{
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

			It("should update hcoTLSConfigCache updating a resource not in dry run mode", func() {
				cli := getFakeClient(cr)
				cli.InitiateUpdateErrors(getUpdateError(noFailure))

				wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true)
				tlssecprofile.SetHyperConvergedTLSSecurityProfile(nil)

				newCr := &v1beta1.HyperConverged{}
				cr.DeepCopyInto(newCr)
				newCr.Spec.TLSSecurityProfile = &oldTLSSecurityProfile

				Expect(wh.ValidateUpdate(ctx, false, newCr, cr)).To(Succeed())
				Expect(hcoTLSConfigCache).To(Equal(&oldTLSSecurityProfile))
			})

			It("should not update hcoTLSConfigCache updating a resource in dry run mode", func() {
				cli := getFakeClient(cr)
				cli.InitiateUpdateErrors(getUpdateError(noFailure))

				wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true)
				tlssecprofile.SetHyperConvergedTLSSecurityProfile(&initialTLSSecurityProfile)

				newCr := &v1beta1.HyperConverged{}
				cr.DeepCopyInto(newCr)
				newCr.Spec.TLSSecurityProfile = &oldTLSSecurityProfile

				Expect(wh.ValidateUpdate(ctx, true, newCr, cr)).To(Succeed())
				Expect(hcoTLSConfigCache).To(Equal(&initialTLSSecurityProfile))
			})

			It("should not update hcoTLSConfigCache if the update request is refused", func() {
				cli := getFakeClient(cr)
				cli.InitiateUpdateErrors(getUpdateError(cdiUpdateFailure))

				wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true)
				tlssecprofile.SetHyperConvergedTLSSecurityProfile(&initialTLSSecurityProfile)
				newCr := &v1beta1.HyperConverged{}
				cr.DeepCopyInto(newCr)
				newCr.Spec.TLSSecurityProfile = &oldTLSSecurityProfile

				err := wh.ValidateUpdate(ctx, false, newCr, cr)
				Expect(err).To(MatchError(ErrFakeCdiError))
				Expect(hcoTLSConfigCache).To(Equal(&initialTLSSecurityProfile))
			})

		})

		Context("delete", func() {

			It("should reset hcoTLSConfigCache deleting a resource not in dry run mode", func() {
				cli := getFakeClient(cr)
				wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true)
				tlssecprofile.SetHyperConvergedTLSSecurityProfile(&modernTLSSecurityProfile)

				Expect(wh.ValidateDelete(ctx, false, cr)).To(Succeed())
				Expect(hcoTLSConfigCache).To(BeNil())
			})

			It("should not update hcoTLSConfigCache deleting a resource in dry run mode", func() {
				cli := getFakeClient(cr)
				wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true)
				tlssecprofile.SetHyperConvergedTLSSecurityProfile(nil)

				hcoTLSConfigCache = &modernTLSSecurityProfile

				Expect(wh.ValidateDelete(ctx, true, cr)).To(Succeed())
				Expect(hcoTLSConfigCache).To(Equal(&modernTLSSecurityProfile))
			})

			It("should not update hcoTLSConfigCache if the delete request is refused", func() {
				cli := getFakeClient(cr)
				wh := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true)
				tlssecprofile.SetHyperConvergedTLSSecurityProfile(&modernTLSSecurityProfile)

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
	})

	Context("MediatedDeviceTypes", func() {
		var cr *v1beta1.HyperConverged
		var newCr *v1beta1.HyperConverged
		var ctx context.Context

		BeforeEach(func() {
			Expect(os.Setenv("OPERATOR_NAMESPACE", HcoValidNamespace)).To(Succeed())
			cr = commontestutils.NewHco()
			cr.Spec.MediatedDevicesConfiguration = nil
			newCr = cr.DeepCopy()
			ctx = context.TODO()
		})

		DescribeTable("Check mediatedDevicesTypes -> mediatedDeviceTypes transition", func(mDConfiguration *v1beta1.MediatedDevicesConfiguration, expected types.GomegaMatcher) {
			// create
			newCr.Spec.MediatedDevicesConfiguration = mDConfiguration
			Expect(wh.ValidateCreate(ctx, false, newCr)).To(expected)

			// update
			cli := getFakeClient(cr)
			cli.InitiateUpdateErrors(getUpdateError(noFailure))
			whU := NewWebhookHandler(logger, cli, decoder, HcoValidNamespace, true)
			Expect(whU.ValidateUpdate(ctx, false, newCr, cr)).To(expected)
		},
			Entry("should not fail with no configuration",
				nil,
				Succeed(),
			),
			Entry("should not fail if using only mediatedDeviceTypes",
				&v1beta1.MediatedDevicesConfiguration{
					MediatedDeviceTypes: []string{"nvidia-222", "nvidia-230"},
					NodeMediatedDeviceTypes: []v1beta1.NodeMediatedDeviceTypesConfig{
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
				&v1beta1.MediatedDevicesConfiguration{
					MediatedDevicesTypes: []string{"nvidia-222", "nvidia-230"},
					NodeMediatedDeviceTypes: []v1beta1.NodeMediatedDeviceTypesConfig{
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
				&v1beta1.MediatedDevicesConfiguration{
					MediatedDevicesTypes: []string{"nvidia-222", "nvidia-230"},
					MediatedDeviceTypes:  []string{"nvidia-222", "nvidia-230"},
					NodeMediatedDeviceTypes: []v1beta1.NodeMediatedDeviceTypesConfig{
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
				&v1beta1.MediatedDevicesConfiguration{
					MediatedDevicesTypes: []string{"nvidia-222", "nvidia-230"},
					MediatedDeviceTypes:  []string{"nvidia-222"},
					NodeMediatedDeviceTypes: []v1beta1.NodeMediatedDeviceTypesConfig{
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
				&v1beta1.MediatedDevicesConfiguration{
					MediatedDevicesTypes: []string{"nvidia-222", "nvidia-230"},
					MediatedDeviceTypes:  []string{"nvidia-222", "nvidia-230"},
					NodeMediatedDeviceTypes: []v1beta1.NodeMediatedDeviceTypesConfig{
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

func getFakeClient(hco *v1beta1.HyperConverged) *commontestutils.HcoTestClient {
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

func newRequest(operation admissionv1.Operation, cr *v1beta1.HyperConverged, encoder runtime.Encoder, dryrun bool) admission.Request {
	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			DryRun:    ptr.To(dryrun),
			Operation: operation,
			Resource: metav1.GroupVersionResource{
				Group:    v1beta1.SchemeGroupVersion.Group,
				Version:  v1beta1.SchemeGroupVersion.Version,
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
