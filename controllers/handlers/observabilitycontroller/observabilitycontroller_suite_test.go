package observabilitycontroller

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/util/fake/clusterinfo"
)

func TestObservabilityController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Observability Controller Suite")
}

var _ = BeforeSuite(func() {
	commontestutils.CommonBeforeSuite()
	hcoutil.GetClusterInfo = clusterinfo.NewGetClusterInfo()
})
