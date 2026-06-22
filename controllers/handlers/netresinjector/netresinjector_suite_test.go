package netresinjector

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
)

func TestNetResInjector(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Network Resources Injector Suite")
}

var _ = BeforeSuite(func() {
	commontestutils.CommonBeforeSuite()
})
