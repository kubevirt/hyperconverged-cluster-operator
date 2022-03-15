package operands

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/reference"

	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commonTestUtils"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	cdiv1beta1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

var _ = Describe("Test operator.go", func() {
	Context("Test applyAnnotationPatch", func() {
		It("Should fail for bad json", func() {
			obj := &cdiv1beta1.CDI{}

			err := applyAnnotationPatch(obj, `{]`)
			Expect(err).To(HaveOccurred())
			fmt.Fprintf(GinkgoWriter, "Expected error: %v\n", err)
		})

		It("Should fail for single patch object (instead of an array)", func() {
			obj := &cdiv1beta1.CDI{}

			err := applyAnnotationPatch(obj, `{"op": "add", "path": "/spec/config/featureGates/-", "value": "fg1"}`)
			Expect(err).To(HaveOccurred())
			fmt.Fprintf(GinkgoWriter, "Expected error: %v\n", err)
		})

		It("Should fail for unknown op in a patch object", func() {
			obj := &cdiv1beta1.CDI{}

			err := applyAnnotationPatch(obj, `[{"op": "unknown", "path": "/spec/config/featureGates/-", "value": "fg1"}]`)
			Expect(err).To(HaveOccurred())
			fmt.Fprintf(GinkgoWriter, "Expected error: %v\n", err)
		})

		It("Should fail for wrong path - not starts with '/spec/' - patch object", func() {
			obj := &cdiv1beta1.CDI{}

			err := applyAnnotationPatch(obj, `[{"op": "add", "path": "/config/featureGates/-", "value": "fg1"}]`)
			Expect(err).To(HaveOccurred())
			fmt.Fprintf(GinkgoWriter, "Expected error: %v\n", err)
		})

		It("Should fail for adding to a not exist object", func() {
			obj := &cdiv1beta1.CDI{}

			err := applyAnnotationPatch(obj, `[{"op": "add", "path": "/spec/config/filesystemOverhead/global", "value": "65"}]`)
			Expect(err).To(HaveOccurred())
			fmt.Fprintf(GinkgoWriter, "Expected error: %v\n", err)
		})

		It("Should fail for removing non-exist field", func() {
			obj := &cdiv1beta1.CDI{
				Spec: cdiv1beta1.CDISpec{
					Config: &cdiv1beta1.CDIConfigSpec{
						FilesystemOverhead: &cdiv1beta1.FilesystemOverhead{},
					},
				},
			}

			err := applyAnnotationPatch(obj, `[{"op": "remove", "path": "/spec/config/filesystemOverhead/global"}]`)
			Expect(err).To(HaveOccurred())
			fmt.Fprintf(GinkgoWriter, "Expected error: %v\n", err)
		})

		It("Should apply annotation if everything is corrct", func() {
			obj := &cdiv1beta1.CDI{
				Spec: cdiv1beta1.CDISpec{
					Config: &cdiv1beta1.CDIConfigSpec{
						FilesystemOverhead: &cdiv1beta1.FilesystemOverhead{},
					},
				},
			}

			err := applyAnnotationPatch(obj, `[{"op": "add", "path": "/spec/config/filesystemOverhead/global", "value": "55"}]`)
			Expect(err).ToNot(HaveOccurred())
			Expect(obj.Spec.Config).NotTo(BeNil())
			Expect(obj.Spec.Config.FilesystemOverhead).NotTo(BeNil())
			Expect(obj.Spec.Config.FilesystemOverhead.Global).Should(BeEquivalentTo("55"))
		})
	})

	Context("Test addCrToTheRelatedObjectList", func() {
		It("Should return error when apiVersion, kind and name missing", func() {
			hco := commonTestUtils.NewHco()
			req := commonTestUtils.NewReq(hco)
			found := &cdiv1beta1.CDI{}

			operand := genericOperand{Scheme: scheme.Scheme}
			err := operand.addCrToTheRelatedObjectList(req, found)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("object reference must have, at a minimum: apiVersion, kind, and name"))
		})

		It("Should add into the list when it is missing", func() {
			hco := commonTestUtils.NewHco()
			req := commonTestUtils.NewReq(hco)
			found := &cdiv1beta1.CDI{
				TypeMeta: metav1.TypeMeta{
					Kind:       "CDI",
					APIVersion: "cdi.kubevirt.io/v1beta1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "cdi-kubevirt-hyperconverged",
				},
			}

			operand := genericOperand{Scheme: scheme.Scheme}
			err := operand.addCrToTheRelatedObjectList(req, found)
			Expect(err).ToNot(HaveOccurred())

			foundRef, err := reference.GetReference(operand.Scheme, found)
			Expect(err).ToNot(HaveOccurred())
			Expect(hco.Status.RelatedObjects).To(ContainElement(*foundRef))
		})

		It("Should update the list and set StatusDirty=true when the resourceVersion is different", func() {
			const oldVersion = "111"
			const newVersion = "112"
			hco := commonTestUtils.NewHco()
			req := commonTestUtils.NewReq(hco)
			found := &cdiv1beta1.CDI{
				TypeMeta: metav1.TypeMeta{
					Kind:       "CDI",
					APIVersion: "cdi.kubevirt.io/v1beta1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            "cdi-kubevirt-hyperconverged",
					ResourceVersion: oldVersion,
				},
			}

			operand := genericOperand{Scheme: scheme.Scheme}
			err := operand.addCrToTheRelatedObjectList(req, found)
			Expect(err).ToNot(HaveOccurred())

			oldRef, err := reference.GetReference(operand.Scheme, found)
			Expect(err).ToNot(HaveOccurred())

			// update resource version
			found.ResourceVersion = newVersion
			err = operand.addCrToTheRelatedObjectList(req, found)
			Expect(err).ToNot(HaveOccurred())

			newRef, err := reference.GetReference(operand.Scheme, found)
			Expect(err).ToNot(HaveOccurred())

			Expect(hco.Status.RelatedObjects).To(ContainElement(*newRef))
			Expect(hco.Status.RelatedObjects).ToNot(ContainElement(*oldRef))
			Expect(req.StatusDirty).To(BeTrue())
		})
	})

})
