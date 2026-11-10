package featuregates

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Feature Gate Details", func() {
	Context("GetFeatureGatePhase", func() {
		It("should return the phase of the feature gate", func() {
			featureGatesDetails = map[string]FeatureGate{
				"fg1": {Name: "fg1", Phase: PhaseGA},
			}

			phase, exists := GetFeatureGatePhase("fg1")
			Expect(exists).To(BeTrue())
			Expect(phase).To(Equal(PhaseGA))
		})

		It("should return the phase of the feature gate, with different casing", func() {
			featureGatesDetails = map[string]FeatureGate{
				"fg1": {Name: "fg1", Phase: PhaseGA},
			}

			phase, exists := GetFeatureGatePhase("FG1")
			Expect(exists).To(BeTrue())
			Expect(phase).To(Equal(PhaseGA))
		})

		It("should return false if feature gate does not exist", func() {
			featureGatesDetails = map[string]FeatureGate{
				"exists": {Name: "exists", Phase: PhaseGA},
			}

			_, exists := GetFeatureGatePhase("notExist")
			Expect(exists).To(BeFalse())
		})
	})

	Context("ListBetaFeatureGates", func() {
		It("should return the list of beta feature gates", func() {
			featureGatesDetails = map[string]FeatureGate{
				"fg1":    {Name: "fg1", Phase: PhaseGA},
				"alpha1": {Name: "alpha1", Phase: PhaseAlpha},
				"beta1":  {Name: "beta1", Phase: PhaseBeta},
				"fg4":    {Name: "fg2", Phase: PhaseUnknown},
				"fg5":    {Name: "fg3", Phase: PhaseDeprecated},
				"fg6":    {Name: "fg4", Phase: PhaseDiscontinued},
				"alpha2": {Name: "alpha2", Phase: PhaseAlpha},
				"beta2":  {Name: "beta2", Phase: PhaseBeta},
				"beta3":  {Name: "beta3", Phase: PhaseBeta},
			}

			betaFGs := ListBetaFeatureGates()
			Expect(betaFGs).To(HaveLen(3))
			Expect(betaFGs).To(ContainElements("beta1", "beta2", "beta3"))
		})

		It("should return an empty list if no beta FG is defined", func() {
			featureGatesDetails = map[string]FeatureGate{
				"fg1":    {Name: "fg1", Phase: PhaseGA},
				"alpha1": {Name: "alpha1", Phase: PhaseAlpha},
				"fg4":    {Name: "fg2", Phase: PhaseUnknown},
				"fg5":    {Name: "fg3", Phase: PhaseDeprecated},
				"fg6":    {Name: "fg4", Phase: PhaseDiscontinued},
				"alpha2": {Name: "alpha2", Phase: PhaseAlpha},
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
			featureGatesDetails = map[string]FeatureGate{
				"fg1":    {Name: "fg1", Phase: PhaseGA},
				"alpha1": {Name: "alpha1", Phase: PhaseAlpha},
				"beta1":  {Name: "beta1", Phase: PhaseBeta},
				"fg4":    {Name: "fg2", Phase: PhaseUnknown},
				"fg5":    {Name: "fg3", Phase: PhaseDeprecated},
				"fg6":    {Name: "fg4", Phase: PhaseDiscontinued},
				"alpha2": {Name: "alpha2", Phase: PhaseAlpha},
				"beta2":  {Name: "beta2", Phase: PhaseBeta},
				"beta3":  {Name: "beta2", Phase: PhaseBeta},
			}

			alphaFGs := ListAlphaFeatureGates()
			Expect(alphaFGs).To(HaveLen(2))
			Expect(alphaFGs).To(ContainElements("alpha1", "alpha2"))
		})

		It("should return an empty list if no alpha FG is defined", func() {
			featureGatesDetails = map[string]FeatureGate{
				"fg1":   {Name: "fg1", Phase: PhaseGA},
				"beta1": {Name: "beta1", Phase: PhaseBeta},
				"fg4":   {Name: "fg2", Phase: PhaseUnknown},
				"fg5":   {Name: "fg3", Phase: PhaseDeprecated},
				"fg6":   {Name: "fg4", Phase: PhaseDiscontinued},
				"beta2": {Name: "beta2", Phase: PhaseBeta},
				"beta3": {Name: "beta2", Phase: PhaseBeta},
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

	Context("ListConfigurableFeatureGates", func() {
		It("should return only alpha and beta gates sorted by name", func() {
			featureGatesDetails = map[string]FeatureGate{
				"zeta":   {Name: "zeta", Phase: PhaseAlpha},
				"alpha":  {Name: "alpha", Phase: PhaseBeta},
				"ga":     {Name: "ga", Phase: PhaseGA},
				"middle": {Name: "middle", Phase: PhaseDeprecated},
				"gone":   {Name: "gone", Phase: PhaseDiscontinued},
			}
			configurableFeatureGates = buildConfigurableList()

			configurable := ListConfigurableFeatureGates()
			Expect(configurable).To(Equal([]FeatureGate{
				{Name: "alpha", Phase: PhaseBeta},
				{Name: "zeta", Phase: PhaseAlpha},
			}))
		})

		It("should return an empty list if no configurable FG is defined", func() {
			featureGatesDetails = map[string]FeatureGate{
				"ga":         {Name: "ga", Phase: PhaseGA},
				"deprecated": {Name: "deprecated", Phase: PhaseDeprecated},
				"gone":       {Name: "gone", Phase: PhaseDiscontinued},
			}
			configurableFeatureGates = buildConfigurableList()

			Expect(ListConfigurableFeatureGates()).To(BeEmpty())
		})

		It("returns only configurable gates from the catalog", func() {
			Expect(setup(featureGateJson)).To(Succeed())

			gates := ListConfigurableFeatureGates()
			Expect(gates).NotTo(BeEmpty())
			for _, fg := range gates {
				Expect(fg.Name).NotTo(BeEmpty())
				Expect(fg.Phase.IsConfigurable()).To(BeTrue(), "feature gate %s has non-configurable phase %s", fg.Name, fg.Phase)
			}

			gates[0].Name = "mutated"
			Expect(ListConfigurableFeatureGates()[0].Name).NotTo(Equal("mutated"))
		})
	})
})
