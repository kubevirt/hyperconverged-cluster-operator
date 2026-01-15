package cleanup

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("test cleanup", func() {
	Context("CleanOutput", func() {
		It("should remove empty metadata: {}", func() {
			input := []byte(`- metadata:
    name: test
  template:
    metadata: {}
    spec:
      source:
        registry:
          url: docker://test
`)
			result := CleanOutput(input)
			Expect(result).ToNot(ContainSubstring("metadata: {}"))
			Expect(result).To(ContainSubstring("template:"))
			Expect(result).To(ContainSubstring("spec:"))
		})

		It("should remove metadata: null", func() {
			input := []byte(`- metadata:
    name: test
  template:
    metadata: null
    spec:
      source:
        registry:
          url: docker://test
`)
			result := CleanOutput(input)
			Expect(result).ToNot(ContainSubstring("metadata: null"))
			Expect(result).To(ContainSubstring("template:"))
			Expect(result).To(ContainSubstring("spec:"))
		})

		It("should not remove metadata with content", func() {
			input := []byte(`- metadata:
    name: test
  template:
    metadata:
      labels:
        app: test
    spec:
      source:
        registry:
          url: docker://test
`)
			result := CleanOutput(input)
			Expect(result).To(ContainSubstring("metadata:"))
			Expect(result).To(ContainSubstring("labels:"))
		})

		It("should remove status: {}", func() {
			input := []byte(`- metadata:
    name: test
  status: {}
`)
			result := CleanOutput(input)
			Expect(result).ToNot(ContainSubstring("status: {}"))
		})

		It("should remove creationTimestamp: null", func() {
			input := []byte(`- metadata:
    name: test
    creationTimestamp: null
`)
			result := CleanOutput(input)
			Expect(result).ToNot(ContainSubstring("creationTimestamp: null"))
		})

		It("should quote schedule values", func() {
			input := []byte(`- spec:
  schedule: 0 */12 * * *
`)
			result := CleanOutput(input)
			Expect(result).To(ContainSubstring(`schedule: "0 */12 * * *"`))
		})
	})
})

func TestCleanup(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "cleanup suite")
}
