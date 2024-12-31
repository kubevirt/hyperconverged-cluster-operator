package upgradepatches

import (
	"os"
	"path"
	"strings"
	"sync"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
)

func TestUpgradePatches(t *testing.T) {
	RegisterFailHandler(Fail)

	var (
		testFilesLocation = getTestFilesLocation() + "/upgradePatches"
		destFile          string
		origOnce          *sync.Once
	)

	BeforeSuite(func() {
		origOnce = once
		wd, _ := os.Getwd()
		destFile = path.Join(wd, "upgradePatches.json")
		Expect(commontestutils.CopyFile(destFile, path.Join(testFilesLocation, "upgradePatches.json"))).To(Succeed())

	})

	AfterSuite(func() {
		once = origOnce
		Expect(os.Remove(destFile)).To(Succeed())
	})

	RunSpecs(t, "Upgrade Patches Suite")
}

const (
	pkgDirectory = "pkg/upgradepatches"
	testFilesLoc = "test-files"
)

func getTestFilesLocation() string {
	wd, err := os.Getwd()
	Expect(err).ToNot(HaveOccurred())
	if strings.HasSuffix(wd, pkgDirectory) {
		return testFilesLoc
	}
	return path.Join(pkgDirectory, testFilesLoc)
}

func copyTestFile(filename string) error {
	testFilesLocation := getTestFilesLocation() + "/upgradePatches"
	return commontestutils.CopyFile(origFile, path.Join(testFilesLocation, filename))
}

func resetOnce() {
	once = &sync.Once{}
}
