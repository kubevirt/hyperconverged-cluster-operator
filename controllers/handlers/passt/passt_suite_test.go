package passt_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
)

func TestPasst(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Passt Suite")
}

var _ = BeforeSuite(func() {
	commontestutils.CommonBeforeSuite()
})
