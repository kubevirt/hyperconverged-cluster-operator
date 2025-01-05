package upgradepatch

import (
	"os"
	"path"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
)

const (
	pkgDirectory = "pkg/upgradepatch"
	testFilesLoc = "test-files"
)

var origFile string

var _ = Describe("upgradePatches", func() {

	BeforeEach(func() {
		wd, _ := os.Getwd()
		origFile = path.Join(wd, "upgradePatches.json")
		Expect(commontestutils.CopyFile(origFile+".orig", origFile)).To(Succeed())
		hcoUpgradeChangesRead = false
	})

	AfterEach(func() {
		Expect(os.Remove(origFile + ".orig")).To(Succeed())
		hcoUpgradeChangesRead = false
	})

	Context("readUpgradeChangesFromFile", func() {

		var hco *hcov1beta1.HyperConverged
		var req *common.HcoRequest

		BeforeEach(func() {
			hco = commontestutils.NewHco()
			req = commontestutils.NewReq(hco)
		})

		AfterEach(func() {
			Expect(commontestutils.CopyFile(origFile, origFile+".orig")).To(Succeed())
		})

		It("should correctly parse and validate actual upgradePatches.json", func() {
			Expect(ValidateUpgradePatches(req)).To(Succeed())
		})

		It("should correctly parse and validate empty upgradePatches", func() {
			Expect(copyTestFile("empty.json")).To(Succeed())
			Expect(ValidateUpgradePatches(req)).To(Succeed())
		})

		It("should fail parsing upgradePatches with bad json", func() {
			Expect(copyTestFile("badJson.json")).To(Succeed())

			err := ValidateUpgradePatches(req)
			Expect(err).To(MatchError(HavePrefix("invalid character")))
		})

		Context("hcoCRPatchList", func() {

			It("should fail validating upgradePatches with bad semver ranges", func() {
				Expect(copyTestFile("badSemverRange.json")).To(Succeed())

				err := ValidateUpgradePatches(req)
				Expect(err).To(MatchError(HavePrefix("Could not get version from string:")))
			})

			DescribeTable(
				"should fail validating upgradePatches with bad patches",
				func(filename, message string) {
					Expect(copyTestFile(filename)).To(Succeed())
					Expect(ValidateUpgradePatches(req)).To(MatchError(HavePrefix(message)))
				},
				Entry(
					"bad operation kind",
					"badPatches1.json",
					"Unexpected kind:",
				),
				Entry(
					"not on spec",
					"badPatches2.json",
					"can only modify spec fields",
				),
				Entry(
					"unexisting path",
					"badPatches3.json",
					"replace operation does not apply: doc is missing path:",
				),
			)

			DescribeTable(
				"should handle MissingPathOnRemove according to jsonPatchApplyOptions",
				func(filename string, expectedErr bool, message string) {
					Expect(copyTestFile(filename)).To(Succeed())

					err := ValidateUpgradePatches(req)
					if expectedErr {
						Expect(err).To(MatchError(HavePrefix(message)))
					} else {
						Expect(err).ToNot(HaveOccurred())
					}
				},
				Entry(
					"without jsonPatchApplyOptions",
					"badPatches4.json",
					true,
					"remove operation does not apply: doc is missing path: ",
				),
				Entry(
					"with AllowMissingPathOnRemove on jsonPatchApplyOptions",
					"badPatches5.json",
					false,
					"",
				),
				Entry(
					"without jsonPatchApplyOptions",
					"badPatches6.json",
					true,
					"add operation does not apply: doc is missing path: ",
				),
				Entry(
					"with EnsurePathExistsOnAdd on jsonPatchApplyOptions",
					"badPatches7.json",
					false,
					"",
				),
			)

		})

		Context("objectsToBeRemoved", func() {

			It("should fail validating upgradePatches with bad semver ranges", func() {
				Expect(copyTestFile("badSemverRangeOR.json")).To(Succeed())
				Expect(ValidateUpgradePatches(req)).To(MatchError(HavePrefix("Could not get version from string:")))
			})

			DescribeTable(
				"should fail validating upgradePatches with bad patches",
				func(filename, message string) {
					Expect(copyTestFile(filename)).To(Succeed())
					Expect(ValidateUpgradePatches(req)).To(MatchError(HavePrefix(message)))
				},
				Entry(
					"empty object kind",
					"badObject1.json",
					"missing object kind",
				),
				Entry(
					"missing object kind",
					"badObject1m.json",
					"missing object kind",
				),
				Entry(
					"empty object API version",
					"badObject2.json",
					"missing object API version",
				),
				Entry(
					"missing object API version",
					"badObject2m.json",
					"missing object API version",
				),
				Entry(
					"empty object name",
					"badObject3.json",
					"missing object name",
				),
				Entry(
					"missing object name",
					"badObject3m.json",
					"missing object name",
				),
			)

		})

	})

})

func copyTestFile(filename string) error {
	return commontestutils.CopyFile(origFile, path.Join(getTestFilesLocation(), filename))
}

func getTestFilesLocation() string {
	wd, err := os.Getwd()
	Expect(err).ToNot(HaveOccurred())
	if strings.HasSuffix(wd, pkgDirectory) {
		return testFilesLoc
	}
	return path.Join(pkgDirectory, testFilesLoc)
}

func TestUpgradePatch(t *testing.T) {
	RegisterFailHandler(Fail)

	var (
		destFile string
	)

	BeforeSuite(func() {
		wd, _ := os.Getwd()
		destFile = path.Join(wd, "upgradePatches.json")
		Expect(commontestutils.CopyFile(destFile, path.Join(getTestFilesLocation(), "upgradePatches.json"))).To(Succeed())
	})

	AfterSuite(func() {
		Expect(os.Remove(destFile)).To(Succeed())
	})

	RunSpecs(t, "Upgrade Patches Suite")
}
