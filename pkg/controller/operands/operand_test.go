package operands

import (
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	cdiv1beta1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
)

var _ = Describe("Test operator.go", func() {
	Context("Test applyAnnotationPatch", func() {
		It("Should fail for bad json", func() {
			spec := &cdiv1beta1.CDISpec{}

			err := applyAnnotationPatch(spec, `{]`)
			Expect(err).To(HaveOccurred())
			fmt.Fprintf(GinkgoWriter, "Expected error: %v\n", err)
			Expect(err).To(HaveOccurred())
		})

		It("Should fail for single patch object (instead of an array)", func() {
			spec := &cdiv1beta1.CDISpec{}

			err := applyAnnotationPatch(spec, `{"op": "add", "path": "/config/featureGates/-", "value": "fg1"}`)
			Expect(err).To(HaveOccurred())
			fmt.Fprintf(GinkgoWriter, "Expected error: %v\n", err)
			Expect(err).To(HaveOccurred())
		})

		It("Should fail for unknown op in a patch object", func() {
			spec := &cdiv1beta1.CDISpec{}

			err := applyAnnotationPatch(spec, `[{"op": "unknown", "path": "/config/featureGates/-", "value": "fg1"}]`)
			Expect(err).To(HaveOccurred())
			fmt.Fprintf(GinkgoWriter, "Expected error: %v\n", err)
			Expect(err).To(HaveOccurred())
		})

		It("Should fail for adding to a not exist object", func() {
			spec := &cdiv1beta1.CDISpec{}

			err := applyAnnotationPatch(spec, `[{"op": "add", "path": "/config/filesystemOverhead/global", "value": "65"}]`)
			Expect(err).To(HaveOccurred())
			fmt.Fprintf(GinkgoWriter, "Expected error: %v\n", err)
			Expect(err).To(HaveOccurred())
		})

		It("Should fail for removing non-exist field", func() {
			spec := &cdiv1beta1.CDISpec{
				Config: &cdiv1beta1.CDIConfigSpec{
					FilesystemOverhead: &cdiv1beta1.FilesystemOverhead{},
				},
			}

			err := applyAnnotationPatch(spec, `[{"op": "remove", "path": "/config/filesystemOverhead/global"}]`)
			Expect(err).To(HaveOccurred())
			fmt.Fprintf(GinkgoWriter, "Expected error: %v\n", err)
			Expect(err).To(HaveOccurred())
		})
	})
})
