package virtimport

import (
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/util/fake/clusterinfo"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/virtimport"
)

func TestVirtImport(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "VirtImport Handlers Suite")
}

var _ = BeforeSuite(func() {
	commontestutils.CommonBeforeSuite()
	hcoutil.GetClusterInfo = clusterinfo.NewGetClusterInfo()
})

func getBundle() *virtimport.Bundle {
	b, err := virtimport.GetBundle()
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
	return b
}

func newOperands(cl client.Client) []operands.Operand {
	ops, err := NewOperands(cl, commontestutils.GetScheme())
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
	return ops
}
