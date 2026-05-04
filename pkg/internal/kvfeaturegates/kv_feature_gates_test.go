package kvfeaturegates_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/kvfeaturegates"
)

func TestKvFeatureGates(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "KV Feature Gates Suite")
}

var _ = Describe("KV Beta Feature Gates", func() {
	It("just make sure init is working, and does not panic", func() {
		_ = kvfeaturegates.GetBetaFeatureGates()
	})
})
