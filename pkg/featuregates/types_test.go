package featuregates_test

import (
	"encoding/json"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/featuregates"
)

func TestFeatureGate(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "FeatureGate Suite")
}

var _ = Describe("FeatureGate", func() {
	Context("Phase - json", func() {
		DescribeTable("MarshalJSON", func(phase featuregates.Phase, expected string) {
			b, err := json.Marshal(phase)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(b)).To(Equal(expected))
		},
			Entry("unknown", featuregates.PhaseUnknown, `"unknown"`),
			Entry("real unknown", featuregates.Phase(99), `"unknown"`),
			Entry("GA", featuregates.PhaseGA, `"GA"`),
			Entry("beta", featuregates.PhaseBeta, `"beta"`),
			Entry("alpha", featuregates.PhaseAlpha, `"alpha"`),
			Entry("Deprecated", featuregates.PhaseDeprecated, `"deprecated"`),
		)

		DescribeTable("UnmarshalJSON", func(jsonStr string, expected featuregates.Phase) {
			var phase featuregates.Phase
			err := json.Unmarshal([]byte(jsonStr), &phase)
			Expect(err).NotTo(HaveOccurred())
			Expect(phase).To(Equal(expected))
		},
			Entry("unknown", `"unknown"`, featuregates.PhaseUnknown),
			Entry("real unknown", `"something else"`, featuregates.PhaseUnknown),
			Entry("GA", `"GA"`, featuregates.PhaseGA),
			Entry("beta", `"beta"`, featuregates.PhaseBeta),
			Entry("alpha", `"alpha"`, featuregates.PhaseAlpha),
			Entry("Deprecated", `"deprecated"`, featuregates.PhaseDeprecated),
		)
	})

	Context("FeatureGate json", func() {
		const (
			name        = "fgName"
			description = "feature gate description"
			fgJson      = `{"name":"` + name + `","phase":"GA","description":"` + description + `"}`
		)
		It("should marshal JSON", func() {
			fg := featuregates.FeatureGate{
				Name:        name,
				Phase:       featuregates.PhaseGA,
				Description: description,
			}

			b, err := json.Marshal(fg)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(b)).To(Equal(fgJson))
		})

		It("should unmarshal JSON", func() {
			fg := featuregates.FeatureGate{}
			err := json.Unmarshal([]byte(fgJson), &fg)
			Expect(err).NotTo(HaveOccurred())
			Expect(fg.Name).To(Equal(name))
			Expect(fg.Phase).To(Equal(featuregates.PhaseGA))
			Expect(fg.Description).To(Equal(description))
		})
	})

	Context("FeatureGates.Sort", func() {
		It("should sort by phase order: GA, beta, alpha, deprecated", func() {
			fgs := featuregates.FeatureGates{
				{Name: "dep1", Phase: featuregates.PhaseDeprecated},
				{Name: "alpha1", Phase: featuregates.PhaseAlpha},
				{Name: "beta1", Phase: featuregates.PhaseBeta},
				{Name: "ga1", Phase: featuregates.PhaseGA},
			}
			fgs.Sort()
			Expect(fgs[0].Phase).To(Equal(featuregates.PhaseGA))
			Expect(fgs[1].Phase).To(Equal(featuregates.PhaseBeta))
			Expect(fgs[2].Phase).To(Equal(featuregates.PhaseAlpha))
			Expect(fgs[3].Phase).To(Equal(featuregates.PhaseDeprecated))
		})

		It("should sort by name within the same phase", func() {
			fgs := featuregates.FeatureGates{
				{Name: "charlie", Phase: featuregates.PhaseAlpha},
				{Name: "alice", Phase: featuregates.PhaseAlpha},
				{Name: "bob", Phase: featuregates.PhaseAlpha},
			}
			fgs.Sort()
			Expect(fgs[0].Name).To(Equal("alice"))
			Expect(fgs[1].Name).To(Equal("bob"))
			Expect(fgs[2].Name).To(Equal("charlie"))
		})

		It("should sort by phase first, then by name", func() {
			fgs := featuregates.FeatureGates{
				{Name: "zAlpha", Phase: featuregates.PhaseAlpha},
				{Name: "bDeprecated", Phase: featuregates.PhaseDeprecated},
				{Name: "aBeta", Phase: featuregates.PhaseBeta},
				{Name: "aAlpha", Phase: featuregates.PhaseAlpha},
				{Name: "aDeprecated", Phase: featuregates.PhaseDeprecated},
				{Name: "zBeta", Phase: featuregates.PhaseBeta},
			}
			fgs.Sort()
			Expect(fgs).To(Equal(featuregates.FeatureGates{
				{Name: "aBeta", Phase: featuregates.PhaseBeta},
				{Name: "zBeta", Phase: featuregates.PhaseBeta},
				{Name: "aAlpha", Phase: featuregates.PhaseAlpha},
				{Name: "zAlpha", Phase: featuregates.PhaseAlpha},
				{Name: "aDeprecated", Phase: featuregates.PhaseDeprecated},
				{Name: "bDeprecated", Phase: featuregates.PhaseDeprecated},
			}))
		})

		It("should handle empty slice", func() {
			fgs := featuregates.FeatureGates{}
			fgs.Sort()
			Expect(fgs).To(BeEmpty())
		})

		It("should handle single element", func() {
			fgs := featuregates.FeatureGates{
				{Name: "only", Phase: featuregates.PhaseAlpha},
			}
			fgs.Sort()
			Expect(fgs[0].Name).To(Equal("only"))
		})
	})
})
