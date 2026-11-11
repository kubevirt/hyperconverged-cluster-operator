package featuregates

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestFeatureGate(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "FeatureGate Suite")

	BeforeSuite(func() {
		origFeatureGatesDetails := featureGatesDetails
		DeferCleanup(func() {
			featureGatesDetails = origFeatureGatesDetails
		})
	})
}
