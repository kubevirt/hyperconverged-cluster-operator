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

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	goldenimages "github.com/kubevirt/hyperconverged-cluster-operator/controllers/handlers/golden-images"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

var _ = Describe("test HyperConverged mutator", func() {
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
				EvictionStrategy: ptr.To(kubevirtcorev1.EvictionStrategyLiveMigrate),
			},
		}

		cli = commontestutils.InitClient(nil)
		mutator = initHCMutator(mutatorScheme, cli)
	})

	Context("Check mutating webhook for create operation", func() {

		var (
			ksmPatch = jsonpatch.JsonPatchOperation{
				Operation: "add",
				Path:      "/spec/ksmConfiguration",
				Value:     kubevirtcorev1.KSMConfiguration{},
			}
		)

		DescribeTable("check dict annotation on create", func(annotations map[string]string, expectedPatches *jsonpatch.JsonPatchOperation) {
			cr.Spec.DataImportCronTemplates = []hcov1.DataImportCronTemplate{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "dictName",
						Annotations: annotations,
					},
				},
			}

			req := admission.Request{AdmissionRequest: newCreateRequest(cr, testCodec)}

			res := mutator.Handle(context.TODO(), req)
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
				Path:      fmt.Sprintf(annotationPathTemplate, 0),
				Value:     map[string]string{goldenimages.CDIImmediateBindAnnotation: "true"},
			}),
			Entry("different annotations", map[string]string{"something/else": "value"}, &jsonpatch.JsonPatchOperation{
				Operation: "add",
				Path:      fmt.Sprintf(dictAnnotationPathTemplate, 0),
				Value:     "true",
			}),
			Entry("annotation=true", map[string]string{goldenimages.CDIImmediateBindAnnotation: "true"}, nil),
			Entry("annotation=false", map[string]string{goldenimages.CDIImmediateBindAnnotation: "false"}, nil),
		)

		It("should handle multiple DICTs", func() {
			cr.Spec.DataImportCronTemplates = []hcov1.DataImportCronTemplate{
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
			}

			req := admission.Request{AdmissionRequest: newCreateRequest(cr, testCodec)}

			res := mutator.Handle(context.TODO(), req)
			Expect(res.Allowed).To(BeTrue())

			Expect(res.Patches).To(HaveLen(3))
			Expect(res.Patches).To(Equal([]jsonpatch.JsonPatchOperation{
				{
					Operation: "add",
					Path:      fmt.Sprintf(annotationPathTemplate, 0),
					Value:     map[string]string{goldenimages.CDIImmediateBindAnnotation: "true"},
				},
				{
					Operation: "add",
					Path:      fmt.Sprintf(dictAnnotationPathTemplate, 1),
					Value:     "true",
				},
				ksmPatch,
			}))
		})

		Context("Check defaults for cluster level EvictionStrategy", func() {

			DescribeTable("check EvictionStrategy default", func(SNO bool, strategy *kubevirtcorev1.EvictionStrategy, patches []jsonpatch.JsonPatchOperation) {
				cr.Status.InfrastructureHighlyAvailable = ptr.To(!SNO)

				cr.Spec.EvictionStrategy = strategy

				req := admission.Request{AdmissionRequest: newCreateRequest(cr, testCodec)}

				res := mutator.Handle(context.TODO(), req)
				Expect(res.Allowed).To(BeTrue())

				patches = append(patches, ksmPatch)
				Expect(res.Patches).To(Equal(patches))
			},
				Entry("should set EvictionStrategyNone if not set and on SNO",
					true,
					nil,
					[]jsonpatch.JsonPatchOperation{{
						Operation: "replace",
						Path:      "/spec/evictionStrategy",
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
					[]jsonpatch.JsonPatchOperation{jsonpatch.JsonPatchOperation{
						Operation: "replace",
						Path:      "/spec/evictionStrategy",
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

		It("should enable KSM by default", func() {
			req := admission.Request{AdmissionRequest: newCreateRequest(cr, testCodec)}

			res := mutator.Handle(context.TODO(), req)
			Expect(res.Allowed).To(BeTrue())

			Expect(res.Patches).To(Equal([]jsonpatch.JsonPatchOperation{ksmPatch}))
		})

		It("should not enable KSM, if already set", func() {
			cr.Spec.KSMConfiguration = &kubevirtcorev1.KSMConfiguration{}
			req := admission.Request{AdmissionRequest: newCreateRequest(cr, testCodec)}

			res := mutator.Handle(context.TODO(), req)
			Expect(res.Allowed).To(BeTrue())

			Expect(res.Patches).To(BeEmpty())
		})

	})

	Context("Check mutating webhook for update operation", func() {
		DescribeTable("check dict annotation on update", func(annotations map[string]string, expectedPatches *jsonpatch.JsonPatchOperation) {
			origCR := cr.DeepCopy()
			cr.Spec.DataImportCronTemplates = []hcov1.DataImportCronTemplate{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "dictName",
						Annotations: annotations,
					},
				},
			}

			req := admission.Request{AdmissionRequest: newUpdateRequest(origCR, cr, testCodec)}

			res := mutator.Handle(context.TODO(), req)
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
				Path:      fmt.Sprintf(annotationPathTemplate, 0),
				Value:     map[string]string{goldenimages.CDIImmediateBindAnnotation: "true"},
			}),
			Entry("different annotations", map[string]string{"something/else": "value"}, &jsonpatch.JsonPatchOperation{
				Operation: "add",
				Path:      fmt.Sprintf(dictAnnotationPathTemplate, 0),
				Value:     "true",
			}),
			Entry("annotation=true", map[string]string{goldenimages.CDIImmediateBindAnnotation: "true"}, nil),
			Entry("annotation=false", map[string]string{goldenimages.CDIImmediateBindAnnotation: "false"}, nil),
		)

		It("should handle multiple DICTs on update", func() {
			origCR := cr.DeepCopy()

			cr.Spec.DataImportCronTemplates = []hcov1.DataImportCronTemplate{
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
			}

			req := admission.Request{AdmissionRequest: newUpdateRequest(origCR, cr, testCodec)}

			res := mutator.Handle(context.TODO(), req)
			Expect(res.Allowed).To(BeTrue())

			Expect(res.Patches).To(HaveLen(2))
			Expect(res.Patches[0]).To(Equal(jsonpatch.JsonPatchOperation{
				Operation: "add",
				Path:      fmt.Sprintf(annotationPathTemplate, 0),
				Value:     map[string]string{goldenimages.CDIImmediateBindAnnotation: "true"},
			}))
			Expect(res.Patches[1]).To(Equal(jsonpatch.JsonPatchOperation{
				Operation: "add",
				Path:      fmt.Sprintf(dictAnnotationPathTemplate, 1),
				Value:     "true",
			}))
		})

		Context("Check defaults for cluster level EvictionStrategy", func() {

			DescribeTable("check EvictionStrategy default", func(SNO bool, strategy *kubevirtcorev1.EvictionStrategy, patches []jsonpatch.JsonPatchOperation) {
				origCR := cr.DeepCopy()
				cr.Status.InfrastructureHighlyAvailable = ptr.To(!SNO)

				cr.Spec.EvictionStrategy = strategy

				req := admission.Request{AdmissionRequest: newUpdateRequest(origCR, cr, testCodec)}

				res := mutator.Handle(context.TODO(), req)
				Expect(res.Allowed).To(BeTrue())

				Expect(res.Patches).To(Equal(patches))
			},
				Entry("should set EvictionStrategyNone if not set and on SNO",
					true,
					nil,
					[]jsonpatch.JsonPatchOperation{jsonpatch.JsonPatchOperation{
						Operation: "replace",
						Path:      "/spec/evictionStrategy",
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
					[]jsonpatch.JsonPatchOperation{jsonpatch.JsonPatchOperation{
						Operation: "replace",
						Path:      "/spec/evictionStrategy",
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
	})
})

func initHCMutator(s *runtime.Scheme, testClient client.Client) *HyperConvergedMutator {
	decoder := admission.NewDecoder(s)
	mutator := NewHyperConvergedMutator(testClient, decoder)

	return mutator
}
