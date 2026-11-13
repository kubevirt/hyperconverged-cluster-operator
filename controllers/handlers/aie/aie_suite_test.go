package aie

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
)

func TestAIE(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "AIE Launcher Suite")
}

var _ = BeforeSuite(func() {
	commontestutils.CommonBeforeSuite()
})
