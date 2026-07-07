package featuregatedetails

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/featuregates"
)

func TestFeatureGateDetails(t *testing.T) {
	RegisterFailHandler(Fail)
	BeforeSuite(func() {
		origFeatureGatesDetails := featureGatesDetails
		DeferCleanup(func() {
			featureGatesDetails = origFeatureGatesDetails
		})
	})

	RunSpecs(t, "Feature Gate Details Suite")
}

var _ = Describe("Feature Gate Details", func() {
	Context("GetFeatureGatePhase", func() {
		It("should return the phase of the feature gate", func() {
			featureGatesDetails = map[string]featuregates.FeatureGate{
				"fg1": {Name: "fg1", Phase: featuregates.PhaseGA},
			}

			phase, exists := GetFeatureGatePhase("fg1")
			Expect(exists).To(BeTrue())
			Expect(phase).To(Equal(featuregates.PhaseGA))
		})

		It("should return the phase of the feature gate, with different casing", func() {
			featureGatesDetails = map[string]featuregates.FeatureGate{
				"fg1": {Name: "fg1", Phase: featuregates.PhaseGA},
			}

			phase, exists := GetFeatureGatePhase("FG1")
			Expect(exists).To(BeTrue())
			Expect(phase).To(Equal(featuregates.PhaseGA))
		})

		It("should return false if feature gate does not exist", func() {
			featureGatesDetails = map[string]featuregates.FeatureGate{
				"exists": {Name: "exists", Phase: featuregates.PhaseGA},
			}

			_, exists := GetFeatureGatePhase("notExist")
			Expect(exists).To(BeFalse())
		})
	})

	Context("ListBetaFeatureGates", func() {
		It("should return the list of beta feature gates", func() {
			featureGatesDetails = map[string]featuregates.FeatureGate{
				"fg1":    {Name: "fg1", Phase: featuregates.PhaseGA},
				"alpha1": {Name: "alpha1", Phase: featuregates.PhaseAlpha},
				"beta1":  {Name: "beta1", Phase: featuregates.PhaseBeta},
				"fg4":    {Name: "fg2", Phase: featuregates.PhaseUnknown},
				"fg5":    {Name: "fg3", Phase: featuregates.PhaseDeprecated},
				"fg6":    {Name: "fg4", Phase: featuregates.PhaseDiscontinued},
				"alpha2": {Name: "alpha2", Phase: featuregates.PhaseAlpha},
				"beta2":  {Name: "beta2", Phase: featuregates.PhaseBeta},
				"beta3":  {Name: "beta3", Phase: featuregates.PhaseBeta},
			}

			betaFGs := ListBetaFeatureGates()
			Expect(betaFGs).To(HaveLen(3))
			Expect(betaFGs).To(ContainElements("beta1", "beta2", "beta3"))
		})

		It("should return an empty list if no beta FG is defined", func() {
			featureGatesDetails = map[string]featuregates.FeatureGate{
				"fg1":    {Name: "fg1", Phase: featuregates.PhaseGA},
				"alpha1": {Name: "alpha1", Phase: featuregates.PhaseAlpha},
				"fg4":    {Name: "fg2", Phase: featuregates.PhaseUnknown},
				"fg5":    {Name: "fg3", Phase: featuregates.PhaseDeprecated},
				"fg6":    {Name: "fg4", Phase: featuregates.PhaseDiscontinued},
				"alpha2": {Name: "alpha2", Phase: featuregates.PhaseAlpha},
			}

			betaFGs := ListBetaFeatureGates()
			Expect(betaFGs).To(BeEmpty())
		})

		It("should return an empty list if no FG is defined", func() {
			featureGatesDetails = nil

			betaFGs := ListBetaFeatureGates()
			Expect(betaFGs).To(BeEmpty())
		})
	})

	Context("ListAlphaFeatureGates", func() {
		It("should return the list of alpha feature gates", func() {
			featureGatesDetails = map[string]featuregates.FeatureGate{
				"fg1":    {Name: "fg1", Phase: featuregates.PhaseGA},
				"alpha1": {Name: "alpha1", Phase: featuregates.PhaseAlpha},
				"beta1":  {Name: "beta1", Phase: featuregates.PhaseBeta},
				"fg4":    {Name: "fg2", Phase: featuregates.PhaseUnknown},
				"fg5":    {Name: "fg3", Phase: featuregates.PhaseDeprecated},
				"fg6":    {Name: "fg4", Phase: featuregates.PhaseDiscontinued},
				"alpha2": {Name: "alpha2", Phase: featuregates.PhaseAlpha},
				"beta2":  {Name: "beta2", Phase: featuregates.PhaseBeta},
				"beta3":  {Name: "beta2", Phase: featuregates.PhaseBeta},
			}

			alphaFGs := ListAlphaFeatureGates()
			Expect(alphaFGs).To(HaveLen(2))
			Expect(alphaFGs).To(ContainElements("alpha1", "alpha2"))
		})

		It("should return an empty list if no alpha FG is defined", func() {
			featureGatesDetails = map[string]featuregates.FeatureGate{
				"fg1":   {Name: "fg1", Phase: featuregates.PhaseGA},
				"beta1": {Name: "beta1", Phase: featuregates.PhaseBeta},
				"fg4":   {Name: "fg2", Phase: featuregates.PhaseUnknown},
				"fg5":   {Name: "fg3", Phase: featuregates.PhaseDeprecated},
				"fg6":   {Name: "fg4", Phase: featuregates.PhaseDiscontinued},
				"beta2": {Name: "beta2", Phase: featuregates.PhaseBeta},
				"beta3": {Name: "beta2", Phase: featuregates.PhaseBeta},
			}

			alphaFGs := ListAlphaFeatureGates()
			Expect(alphaFGs).To(BeEmpty())
		})

		It("should return an empty list if no FG is defined", func() {
			featureGatesDetails = nil

			betaFGs := ListAlphaFeatureGates()
			Expect(betaFGs).To(BeEmpty())
		})
	})
})
