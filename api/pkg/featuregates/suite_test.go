package featuregates

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestFeatureGate(t *testing.T) {
	RegisterFailHandler(Fail)
	BeforeSuite(func() {
		origFeatureGatesDetails := featureGatesDetails
		origConfigurableFeatureGates := configurableFeatureGates
		DeferCleanup(func() {
			featureGatesDetails = origFeatureGatesDetails
			configurableFeatureGates = origConfigurableFeatureGates
		})
	})

	RunSpecs(t, "FeatureGate Suite")
}
