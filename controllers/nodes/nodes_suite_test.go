package nodes_test

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

func TestNodes(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Nodes Controller Suite")
}

var origOperatorNamespaceEnv = os.Getenv(hcoutil.OperatorNamespaceEnv)

var _ = BeforeSuite(func() {
	Expect(os.Setenv(hcoutil.OperatorNamespaceEnv, commontestutils.Namespace)).To(Succeed())
})

var _ = AfterSuite(func() {
	Expect(os.Setenv(hcoutil.OperatorNamespaceEnv, origOperatorNamespaceEnv)).To(Succeed())
})
