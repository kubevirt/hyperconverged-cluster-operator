package metrics

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"testing"
)

func TestMetrics(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Metrics Suite")
}

var _ = Describe("metrics suite", func() {
	Context("Overwritten Modifications", func() {
		It("should be empty when starting", func() {
			count, err := HcoMetrics.GetOverwrittenModificationsCount("kind", "name")
			Expect(err).ToNot(HaveOccurred())
			Expect(count).Should(BeEquivalentTo(0))
		})

		It("should update the counter", func() {
			Expect(HcoMetrics.IncOverwrittenModifications("kind", "name")).Should(Succeed())
			count, err := HcoMetrics.GetOverwrittenModificationsCount("kind", "name")
			Expect(err).ToNot(HaveOccurred())
			Expect(count).Should(BeEquivalentTo(1))
		})
	})

	Context("Unsafe Modifications", func() {
		It("should be empty when starting", func() {
			count, err := HcoMetrics.GetUnsafeModificationsCount("annotation")
			Expect(err).ToNot(HaveOccurred())
			Expect(count).Should(BeEquivalentTo(0))
		})

		It("should update the counter", func() {
			Expect(HcoMetrics.SetUnsafeModificationCount(3, "annotation")).Should(Succeed())
			count, err := HcoMetrics.GetUnsafeModificationsCount("annotation")
			Expect(err).ToNot(HaveOccurred())
			Expect(count).Should(BeEquivalentTo(3))
		})

		It("should reduce the counter", func() {
			Expect(HcoMetrics.SetUnsafeModificationCount(1, "annotation")).Should(Succeed())
			count, err := HcoMetrics.GetUnsafeModificationsCount("annotation")
			Expect(err).ToNot(HaveOccurred())
			Expect(count).Should(BeEquivalentTo(1))
		})
	})

	Context("HyperConverged Exists", func() {
		It("should be false when starting", func() {
			isExists, err := HcoMetrics.IsHCOMetricHyperConvergedExists()
			Expect(err).ToNot(HaveOccurred())
			Expect(isExists).Should(BeFalse())
		})

		It("should set to true", func() {
			Expect(HcoMetrics.SetHCOMetricHyperConvergedExists()).Should(Succeed())
			isExists, err := HcoMetrics.IsHCOMetricHyperConvergedExists()
			Expect(err).ToNot(HaveOccurred())
			Expect(isExists).Should(BeTrue())
		})

		It("should set to false", func() {
			Expect(HcoMetrics.SetHCOMetricHyperConvergedNotExists()).Should(Succeed())
			isExists, err := HcoMetrics.IsHCOMetricHyperConvergedExists()
			Expect(err).ToNot(HaveOccurred())
			Expect(isExists).Should(BeFalse())
		})
	})
})
