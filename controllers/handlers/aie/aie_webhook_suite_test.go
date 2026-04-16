package aie

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
)

func TestAIEWebhook(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "AIE Webhook Suite")
}

var _ = BeforeSuite(func() {
	commontestutils.CommonBeforeSuite()
})
