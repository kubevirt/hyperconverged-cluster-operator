package mutator

import (
	"context"
	"fmt"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gomodules.xyz/jsonpatch/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	kubevirtcorev1 "kubevirt.io/api/core/v1"
	cdiv1beta1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	goldenimages "github.com/kubevirt/hyperconverged-cluster-operator/controllers/handlers/golden-images"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

const (
	v1DICTAnnotationPathTemplate = dictsPathTemplate + dictAnnotationPath
)

var _ = Describe("test HyperConverged v1 mutator", func() {
	var (
		cr      *hcov1.HyperConverged
		cli     client.Client
		mutator *HyperConvergedMutator
	)

	mutatorScheme = scheme.Scheme
	Expect(hcov1.AddToScheme(mutatorScheme)).To(Succeed())
	BeforeEach(func() {
		Expect(os.Setenv("OPERATOR_NAMESPACE", HcoValidNamespace)).To(Succeed())
		cr = &hcov1.HyperConverged{
			ObjectMeta: metav1.ObjectMeta{
				Name:      util.HyperConvergedName,
				Namespace: HcoValidNamespace,
			},
			Spec: hcov1.HyperConvergedSpec{
				Virtualization: hcov1.VirtualizationConfig{
					EvictionStrategy: ptr.To(kubevirtcorev1.EvictionStrategyLiveMigrate),
				},
			},
		}

		cli = commontestutils.InitClient(nil)
		mutator = initHCMutator(mutatorScheme, cli)
	})

	const v1EvictionStrategyPath = "/spec/virtualization/evictionStrategy"

	Context("Check mutating webhook for create operation", func() {

		var (
			ksmPatch = jsonpatch.JsonPatchOperation{
				Operation: "add",
				Path:      "/spec/virtualization/ksmConfiguration",
				Value:     kubevirtcorev1.KSMConfiguration{},
			}
		)

		DescribeTable("check dict annotation on create", func(ctx context.Context, annotations map[string]string, expectedPatches *jsonpatch.JsonPatchOperation) {
			cr.Spec.WorkloadSources.DataImportCronTemplates = []hcov1.DataImportCronTemplate{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "dictName",
						Annotations: annotations,
					},
				},
			}

			req := admission.Request{AdmissionRequest: newCreateRequest(cr, testCodec)}

			res := mutator.Handle(ctx, req)
			Expect(res.Allowed).To(BeTrue())

			if expectedPatches == nil {
				Expect(res.Patches).To(HaveLen(1))
			} else {
				Expect(res.Patches).To(HaveLen(2))
				Expect(res.Patches).To(ContainElement(*expectedPatches))
			}

			Expect(res.Patches).To(ContainElement(ksmPatch))
		},
			Entry("no annotations", nil, &jsonpatch.JsonPatchOperation{
				Operation: "add",
				Path:      fmt.Sprintf(v1DICTAnnotationPathTemplate, 0),
				Value:     map[string]string{goldenimages.CDIImmediateBindAnnotation: "true"},
			}),
			Entry("different annotations", map[string]string{"something/else": "value"}, &jsonpatch.JsonPatchOperation{
				Operation: "add",
				Path:      fmt.Sprintf(v1DICTAnnotationPathTemplate+dictImmediateAnnotationPath, 0),
				Value:     "true",
			}),
			Entry("annotation=true", map[string]string{goldenimages.CDIImmediateBindAnnotation: "true"}, nil),
			Entry("annotation=false", map[string]string{goldenimages.CDIImmediateBindAnnotation: "false"}, nil),
		)

		DescribeTable("check dict spec on create", func(ctx context.Context, spec *cdiv1beta1.DataImportCronSpec, expectedPatches []jsonpatch.JsonPatchOperation) {
			cr.Spec.WorkloadSources.DataImportCronTemplates = []hcov1.DataImportCronTemplate{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "dictName",
						Annotations: map[string]string{goldenimages.CDIImmediateBindAnnotation: "true"},
					},
					Spec: spec,
				},
			}

			cr.Spec.Virtualization.KSMConfiguration = &kubevirtcorev1.KSMConfiguration{}

			req := admission.Request{AdmissionRequest: newCreateRequest(cr, testCodec)}

			res := mutator.Handle(ctx, req)
			Expect(res.Allowed).To(BeTrue())

			if expectedPatches == nil {
				Expect(res.Patches).To(BeEmpty())
			} else {
				Expect(res.Patches).To(Equal(expectedPatches))
			}
		},
			Entry("spec is nil", nil, nil),
			Entry("empty spec", &cdiv1beta1.DataImportCronSpec{}, []jsonpatch.JsonPatchOperation{
				{
					Operation: "add",
					Path:      fmt.Sprintf(dictsPathTemplate+retentionPolicyPath, 0),
					Value:     cdiv1beta1.DataImportCronRetainNone,
				}, {
					Operation: "add",
					Path:      fmt.Sprintf(dictsPathTemplate+importsToKeepPath, 0),
					Value:     1,
				},
			}),
			Entry("retentionPolicy is missing", &cdiv1beta1.DataImportCronSpec{ImportsToKeep: ptr.To[int32](1)}, []jsonpatch.JsonPatchOperation{
				{
					Operation: "add",
					Path:      fmt.Sprintf(dictsPathTemplate+retentionPolicyPath, 0),
					Value:     cdiv1beta1.DataImportCronRetainNone,
				},
			}),
			Entry("importsToKeep is missing",
				&cdiv1beta1.DataImportCronSpec{
					RetentionPolicy: ptr.To(cdiv1beta1.DataImportCronRetainNone),
				},
				[]jsonpatch.JsonPatchOperation{
					{
						Operation: "add",
						Path:      fmt.Sprintf(dictsPathTemplate+importsToKeepPath, 0),
						Value:     1,
					},
				}),
		)

		It("should handle multiple DICTs", func(ctx context.Context) {
			cr.Spec.WorkloadSources.DataImportCronTemplates = []hcov1.DataImportCronTemplate{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "no-annotation",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "different-annotation",
						Annotations: map[string]string{"something/else": "value"},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "annotation-true",
						Annotations: map[string]string{goldenimages.CDIImmediateBindAnnotation: "true"},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "annotation-true",
						Annotations: map[string]string{goldenimages.CDIImmediateBindAnnotation: "false"},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "missing-retentionr-policy",
						Annotations: map[string]string{goldenimages.CDIImmediateBindAnnotation: "false"},
					},
					Spec: &cdiv1beta1.DataImportCronSpec{
						ImportsToKeep: ptr.To[int32](1),
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "missing-import-to-keep",
						Annotations: map[string]string{goldenimages.CDIImmediateBindAnnotation: "false"},
					},
					Spec: &cdiv1beta1.DataImportCronSpec{
						RetentionPolicy: ptr.To(cdiv1beta1.DataImportCronRetainNone),
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "missing-retentionr-policy-and-import-to-keep",
						Annotations: map[string]string{goldenimages.CDIImmediateBindAnnotation: "false"},
					},
					Spec: &cdiv1beta1.DataImportCronSpec{},
				},
			}

			req := admission.Request{AdmissionRequest: newCreateRequest(cr, testCodec)}

			res := mutator.Handle(ctx, req)
			Expect(res.Allowed).To(BeTrue())

			Expect(res.Patches).To(HaveLen(7))
			Expect(res.Patches).To(Equal([]jsonpatch.JsonPatchOperation{
				{
					Operation: "add",
					Path:      fmt.Sprintf(v1DICTAnnotationPathTemplate, 0),
					Value:     map[string]string{goldenimages.CDIImmediateBindAnnotation: "true"},
				},
				{
					Operation: "add",
					Path:      fmt.Sprintf(v1DICTAnnotationPathTemplate+dictImmediateAnnotationPath, 1),
					Value:     "true",
				},
				{
					Operation: "add",
					Path:      fmt.Sprintf(dictsPathTemplate+retentionPolicyPath, 4),
					Value:     cdiv1beta1.DataImportCronRetainNone,
				},
				{
					Operation: "add",
					Path:      fmt.Sprintf(dictsPathTemplate+importsToKeepPath, 5),
					Value:     1,
				},
				{
					Operation: "add",
					Path:      fmt.Sprintf(dictsPathTemplate+retentionPolicyPath, 6),
					Value:     cdiv1beta1.DataImportCronRetainNone,
				},
				{
					Operation: "add",
					Path:      fmt.Sprintf(dictsPathTemplate+importsToKeepPath, 6),
					Value:     1,
				},
				ksmPatch,
			}))
		})

		Context("Check defaults for cluster level EvictionStrategy", func() {

			DescribeTable("check EvictionStrategy default", func(ctx context.Context, SNO bool, strategy *kubevirtcorev1.EvictionStrategy, patches []jsonpatch.JsonPatchOperation) {
				cr.Status.InfrastructureHighlyAvailable = ptr.To(!SNO)

				cr.Spec.Virtualization.EvictionStrategy = strategy

				req := admission.Request{AdmissionRequest: newCreateRequest(cr, testCodec)}

				res := mutator.Handle(ctx, req)
				Expect(res.Allowed).To(BeTrue())

				patches = append(patches, ksmPatch)
				Expect(res.Patches).To(Equal(patches))
			},
				Entry("should set EvictionStrategyNone if not set and on SNO",
					true,
					nil,
					[]jsonpatch.JsonPatchOperation{{
						Operation: "replace",
						Path:      v1EvictionStrategyPath,
						Value:     kubevirtcorev1.EvictionStrategyNone,
					}},
				),
				Entry("should not override EvictionStrategy if set and on SNO - 1",
					true,
					ptr.To(kubevirtcorev1.EvictionStrategyNone),
					nil,
				),
				Entry("should not override EvictionStrategy if set and on SNO - 2",
					true,
					ptr.To(kubevirtcorev1.EvictionStrategyLiveMigrate),
					nil,
				),
				Entry("should not override EvictionStrategy if set and on SNO - 3",
					true,
					ptr.To(kubevirtcorev1.EvictionStrategyExternal),
					nil,
				),
				Entry("should set EvictionStrategyLiveMigrate if not set and not on SNO",
					false,
					nil,
					[]jsonpatch.JsonPatchOperation{{
						Operation: "replace",
						Path:      v1EvictionStrategyPath,
						Value:     kubevirtcorev1.EvictionStrategyLiveMigrate,
					}},
				),
				Entry("should not override EvictionStrategy if set and not on SNO - 1",
					false,
					ptr.To(kubevirtcorev1.EvictionStrategyNone),
					nil,
				),
				Entry("should not override EvictionStrategy if set and not on SNO - 2",
					false,
					ptr.To(kubevirtcorev1.EvictionStrategyLiveMigrate),
					nil,
				),
				Entry("should not override EvictionStrategy if set and not on SNO - 3",
					false,
					ptr.To(kubevirtcorev1.EvictionStrategyExternal),
					nil,
				),
			)
		})

		It("should enable KSM by default", func(ctx context.Context) {
			req := admission.Request{AdmissionRequest: newCreateRequest(cr, testCodec)}

			res := mutator.Handle(ctx, req)
			Expect(res.Allowed).To(BeTrue())

			Expect(res.Patches).To(Equal([]jsonpatch.JsonPatchOperation{ksmPatch}))
		})

		It("should not enable KSM, if already set", func(ctx context.Context) {
			cr.Spec.Virtualization.KSMConfiguration = &kubevirtcorev1.KSMConfiguration{}
			req := admission.Request{AdmissionRequest: newCreateRequest(cr, testCodec)}

			res := mutator.Handle(ctx, req)
			Expect(res.Allowed).To(BeTrue())

			Expect(res.Patches).To(BeEmpty())
		})

		Context("mutation of tuningPolicy", func() {
			It("should not mutate an empty tuningPolicy", func(ctx context.Context) {
				cr.Spec.Virtualization.TuningPolicy = ""
				cr.Spec.Virtualization.KSMConfiguration = &kubevirtcorev1.KSMConfiguration{}
				req := admission.Request{AdmissionRequest: newCreateRequest(cr, testCodec)}

				res := mutator.Handle(ctx, req)
				Expect(res.Allowed).To(BeTrue())

				Expect(res.Patches).To(BeEmpty())
			})

			It("should not mutate the 'annotation' tuningPolicy", func(ctx context.Context) {
				cr.Spec.Virtualization.TuningPolicy = hcov1.HyperConvergedAnnotationTuningPolicy
				cr.Spec.Virtualization.KSMConfiguration = &kubevirtcorev1.KSMConfiguration{}

				req := admission.Request{AdmissionRequest: newCreateRequest(cr, testCodec)}

				res := mutator.Handle(ctx, req)
				Expect(res.Allowed).To(BeTrue())

				Expect(res.Patches).To(BeEmpty())
			})

			It("should drop the 'highBurst' tuningPolicy", func(ctx context.Context) {
				cr.Spec.Virtualization.TuningPolicy = hcov1beta1.HyperConvergedHighBurstProfile //nolint SA1019
				cr.Spec.Virtualization.KSMConfiguration = &kubevirtcorev1.KSMConfiguration{}
				req := admission.Request{AdmissionRequest: newCreateRequest(cr, testCodec)}

				res := mutator.Handle(ctx, req)
				Expect(res.Allowed).To(BeTrue())

				Expect(res.Patches).To(Equal([]jsonpatch.JsonPatchOperation{
					{
						Operation: "remove",
						Path:      "/spec/virtualization/tuningPolicy",
					},
				}))
			})
		})
	})

	Context("Check mutating webhook for update operation", func() {
		DescribeTable("check dict annotation on update", func(ctx context.Context, annotations map[string]string, expectedPatches *jsonpatch.JsonPatchOperation) {
			origCR := cr.DeepCopy()
			cr.Spec.WorkloadSources.DataImportCronTemplates = []hcov1.DataImportCronTemplate{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "dictName",
						Annotations: annotations,
					},
				},
			}

			req := admission.Request{AdmissionRequest: newUpdateRequest(origCR, cr, testCodec)}

			res := mutator.Handle(ctx, req)
			Expect(res.Allowed).To(BeTrue())

			if expectedPatches == nil {
				Expect(res.Patches).To(BeEmpty())
			} else {
				Expect(res.Patches).To(HaveLen(1))
				Expect(res.Patches[0]).To(Equal(*expectedPatches))
			}
		},
			Entry("no annotations", nil, &jsonpatch.JsonPatchOperation{
				Operation: "add",
				Path:      fmt.Sprintf(v1DICTAnnotationPathTemplate, 0),
				Value:     map[string]string{goldenimages.CDIImmediateBindAnnotation: "true"},
			}),
			Entry("different annotations", map[string]string{"something/else": "value"}, &jsonpatch.JsonPatchOperation{
				Operation: "add",
				Path:      fmt.Sprintf(v1DICTAnnotationPathTemplate+dictImmediateAnnotationPath, 0),
				Value:     "true",
			}),
			Entry("annotation=true", map[string]string{goldenimages.CDIImmediateBindAnnotation: "true"}, nil),
			Entry("annotation=false", map[string]string{goldenimages.CDIImmediateBindAnnotation: "false"}, nil),
		)

		DescribeTable("check dict spec on update", func(ctx context.Context, spec *cdiv1beta1.DataImportCronSpec, expectedPatches []jsonpatch.JsonPatchOperation) {
			origCR := cr.DeepCopy()
			cr.Spec.WorkloadSources.DataImportCronTemplates = []hcov1.DataImportCronTemplate{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "dictName",
						Annotations: map[string]string{goldenimages.CDIImmediateBindAnnotation: "true"},
					},
					Spec: spec,
				},
			}

			req := admission.Request{AdmissionRequest: newUpdateRequest(origCR, cr, testCodec)}

			res := mutator.Handle(ctx, req)
			Expect(res.Allowed).To(BeTrue())

			if expectedPatches == nil {
				Expect(res.Patches).To(BeEmpty())
			} else {
				Expect(res.Patches).To(Equal(expectedPatches))
			}
		},
			Entry("spec is nil", nil, nil),
			Entry("empty spec", &cdiv1beta1.DataImportCronSpec{}, []jsonpatch.JsonPatchOperation{
				{
					Operation: "add",
					Path:      fmt.Sprintf(dictsPathTemplate+retentionPolicyPath, 0),
					Value:     cdiv1beta1.DataImportCronRetainNone,
				}, {
					Operation: "add",
					Path:      fmt.Sprintf(dictsPathTemplate+importsToKeepPath, 0),
					Value:     1,
				},
			}),
			Entry("retentionPolicy is missing", &cdiv1beta1.DataImportCronSpec{ImportsToKeep: ptr.To[int32](1)}, []jsonpatch.JsonPatchOperation{
				{
					Operation: "add",
					Path:      fmt.Sprintf(dictsPathTemplate+retentionPolicyPath, 0),
					Value:     cdiv1beta1.DataImportCronRetainNone,
				},
			}),
			Entry("importsToKeep is missing",
				&cdiv1beta1.DataImportCronSpec{
					RetentionPolicy: ptr.To(cdiv1beta1.DataImportCronRetainNone),
				},
				[]jsonpatch.JsonPatchOperation{
					{
						Operation: "add",
						Path:      fmt.Sprintf(dictsPathTemplate+importsToKeepPath, 0),
						Value:     1,
					},
				}),
		)

		It("should handle multiple DICTs on update", func(ctx context.Context) {
			origCR := cr.DeepCopy()

			cr.Spec.WorkloadSources.DataImportCronTemplates = []hcov1.DataImportCronTemplate{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "no-annotation",
					},
					Spec: &cdiv1beta1.DataImportCronSpec{
						// same as the HCO's default values; should not override existing value
						RetentionPolicy: ptr.To(cdiv1beta1.DataImportCronRetainNone),
						ImportsToKeep:   ptr.To[int32](1),
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "different-annotation",
						Annotations: map[string]string{"something/else": "value"},
					},
					Spec: &cdiv1beta1.DataImportCronSpec{
						// same as the CDI's default values; should not override existing value
						RetentionPolicy: ptr.To(cdiv1beta1.DataImportCronRetainAll),
						ImportsToKeep:   ptr.To[int32](3),
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "annotation-true",
						Annotations: map[string]string{goldenimages.CDIImmediateBindAnnotation: "true"},
					},
					Spec: &cdiv1beta1.DataImportCronSpec{
						// same as the HCO's default values; should not override existing value
						RetentionPolicy: ptr.To(cdiv1beta1.DataImportCronRetainNone),
						ImportsToKeep:   ptr.To[int32](1),
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "annotation-true",
						Annotations: map[string]string{goldenimages.CDIImmediateBindAnnotation: "false"},
					},
					Spec: &cdiv1beta1.DataImportCronSpec{
						// same as the HCO's default values; should not override existing value
						RetentionPolicy: ptr.To(cdiv1beta1.DataImportCronRetainNone),
						ImportsToKeep:   ptr.To[int32](1),
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "missing-retentionr-policy",
						Annotations: map[string]string{goldenimages.CDIImmediateBindAnnotation: "false"},
					},
					Spec: &cdiv1beta1.DataImportCronSpec{
						ImportsToKeep: ptr.To[int32](1),
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "missing-import-to-keep",
						Annotations: map[string]string{goldenimages.CDIImmediateBindAnnotation: "false"},
					},
					Spec: &cdiv1beta1.DataImportCronSpec{
						RetentionPolicy: ptr.To(cdiv1beta1.DataImportCronRetainNone),
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "missing-retentionr-policy-and-import-to-keep",
						Annotations: map[string]string{goldenimages.CDIImmediateBindAnnotation: "false"},
					},
					Spec: &cdiv1beta1.DataImportCronSpec{},
				},
			}

			req := admission.Request{AdmissionRequest: newUpdateRequest(origCR, cr, testCodec)}

			res := mutator.Handle(ctx, req)
			Expect(res.Allowed).To(BeTrue())

			Expect(res.Patches).To(HaveLen(6))
			Expect(res.Patches).To(Equal([]jsonpatch.JsonPatchOperation{
				{
					Operation: "add",
					Path:      fmt.Sprintf(v1DICTAnnotationPathTemplate, 0),
					Value:     map[string]string{goldenimages.CDIImmediateBindAnnotation: "true"},
				},
				{
					Operation: "add",
					Path:      fmt.Sprintf(v1DICTAnnotationPathTemplate+dictImmediateAnnotationPath, 1),
					Value:     "true",
				},
				{
					Operation: "add",
					Path:      fmt.Sprintf(dictsPathTemplate+retentionPolicyPath, 4),
					Value:     cdiv1beta1.DataImportCronRetainNone,
				},
				{
					Operation: "add",
					Path:      fmt.Sprintf(dictsPathTemplate+importsToKeepPath, 5),
					Value:     1,
				},
				{
					Operation: "add",
					Path:      fmt.Sprintf(dictsPathTemplate+retentionPolicyPath, 6),
					Value:     cdiv1beta1.DataImportCronRetainNone,
				},
				{
					Operation: "add",
					Path:      fmt.Sprintf(dictsPathTemplate+importsToKeepPath, 6),
					Value:     1,
				},
			}))
		})

		Context("Check defaults for cluster level EvictionStrategy", func() {

			DescribeTable("check EvictionStrategy default", func(ctx context.Context, SNO bool, strategy *kubevirtcorev1.EvictionStrategy, patches []jsonpatch.JsonPatchOperation) {
				origCR := cr.DeepCopy()
				cr.Status.InfrastructureHighlyAvailable = ptr.To(!SNO)

				cr.Spec.Virtualization.EvictionStrategy = strategy

				req := admission.Request{AdmissionRequest: newUpdateRequest(origCR, cr, testCodec)}

				res := mutator.Handle(ctx, req)
				Expect(res.Allowed).To(BeTrue())

				Expect(res.Patches).To(Equal(patches))
			},
				Entry("should set EvictionStrategyNone if not set and on SNO",
					true,
					nil,
					[]jsonpatch.JsonPatchOperation{{
						Operation: "replace",
						Path:      v1EvictionStrategyPath,
						Value:     kubevirtcorev1.EvictionStrategyNone,
					}},
				),
				Entry("should not override EvictionStrategy if set and on SNO - 1",
					true,
					ptr.To(kubevirtcorev1.EvictionStrategyNone),
					nil,
				),
				Entry("should not override EvictionStrategy if set and on SNO - 2",
					true,
					ptr.To(kubevirtcorev1.EvictionStrategyLiveMigrate),
					nil,
				),
				Entry("should not override EvictionStrategy if set and on SNO - 3",
					true,
					ptr.To(kubevirtcorev1.EvictionStrategyExternal),
					nil,
				),
				Entry("should set EvictionStrategyLiveMigrate if not set and not on SNO",
					false,
					nil,
					[]jsonpatch.JsonPatchOperation{{
						Operation: "replace",
						Path:      v1EvictionStrategyPath,
						Value:     kubevirtcorev1.EvictionStrategyLiveMigrate,
					}},
				),
				Entry("should not override EvictionStrategy if set and not on SNO - 1",
					false,
					ptr.To(kubevirtcorev1.EvictionStrategyNone),
					nil,
				),
				Entry("should not override EvictionStrategy if set and not on SNO - 2",
					false,
					ptr.To(kubevirtcorev1.EvictionStrategyLiveMigrate),
					nil,
				),
				Entry("should not override EvictionStrategy if set and not on SNO - 3",
					false,
					ptr.To(kubevirtcorev1.EvictionStrategyExternal),
					nil,
				),
			)
		})

		Context("mutation of tuningPolicy", func() {
			var origCR *hcov1.HyperConverged

			BeforeEach(func() {
				origCR = cr.DeepCopy()
			})

			It("should not mutate an empty tuningPolicy", func(ctx context.Context) {
				cr.Spec.Virtualization.TuningPolicy = ""
				req := admission.Request{AdmissionRequest: newUpdateRequest(origCR, cr, testCodec)}

				res := mutator.Handle(ctx, req)
				Expect(res.Allowed).To(BeTrue())

				Expect(res.Patches).To(BeEmpty())
			})

			It("should not mutate the 'annotation' tuningPolicy", func(ctx context.Context) {
				cr.Spec.Virtualization.TuningPolicy = hcov1.HyperConvergedAnnotationTuningPolicy

				req := admission.Request{AdmissionRequest: newUpdateRequest(origCR, cr, testCodec)}

				res := mutator.Handle(ctx, req)
				Expect(res.Allowed).To(BeTrue())

				Expect(res.Patches).To(BeEmpty())
			})

			It("should drop the 'highBurst' tuningPolicy", func(ctx context.Context) {
				cr.Spec.Virtualization.TuningPolicy = hcov1beta1.HyperConvergedHighBurstProfile //nolint SA1019
				req := admission.Request{AdmissionRequest: newUpdateRequest(origCR, cr, testCodec)}

				res := mutator.Handle(ctx, req)
				Expect(res.Allowed).To(BeTrue())

				Expect(res.Patches).To(Equal([]jsonpatch.JsonPatchOperation{
					{
						Operation: "remove",
						Path:      "/spec/virtualization/tuningPolicy",
					},
				}))
			})
		})
	})
})

func initHCMutator(s *runtime.Scheme, testClient client.Client) *HyperConvergedMutator {
	decoder := admission.NewDecoder(s)
	mutator := NewHyperConvergedMutator(testClient, decoder)

	return mutator
}
