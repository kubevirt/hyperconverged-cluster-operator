package wasp_agent

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
)

func TestWaspAgent(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Wasp Agent Suite")
}

var _ = BeforeSuite(func() {
	commontestutils.CommonBeforeSuite()
})
