package hyperconverged

import (
	_ "embed"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/dirtest"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/upgradepatch"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

//go:embed test-files/upgradePatches/upgradePatches.json
var upgradePatchesFileContent []byte

func TestHyperconverged(t *testing.T) {
	RegisterFailHandler(Fail)

	getClusterInfo := hcoutil.GetClusterInfo

	BeforeSuite(func() {
		hcoutil.GetClusterInfo = func() hcoutil.ClusterInfo {
			return &commontestutils.ClusterInfoMock{}
		}

		pwdFS := dirtest.New(dirtest.WithFile(upgradepatch.UpgradeChangesFileLocation, upgradePatchesFileContent))
		Expect(upgradepatch.Init(pwdFS, GinkgoLogr)).To(Succeed())
	})

	AfterSuite(func() {
		hcoutil.GetClusterInfo = getClusterInfo
	})

	RunSpecs(t, "Hyperconverged Suite")
}
