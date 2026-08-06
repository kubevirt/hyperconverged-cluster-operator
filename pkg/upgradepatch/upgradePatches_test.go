package upgradepatch

import (
	"bytes"
	"slices"
	"sync"
	"testing"

	"github.com/blang/semver/v4"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"k8s.io/utils/ptr"

	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/dirtest"
)

var (
	hcCRBytesOrig []byte
)

func resetOnce() {
	once = &sync.Once{}
}

func TestUpgradePatch(t *testing.T) {
	RegisterFailHandler(Fail)

	BeforeSuite(func() {
		hcCRBytesOrig = slices.Clone(hcCRBytes)
	})

	AfterSuite(func() {
		hcCRBytes = hcCRBytesOrig
		resetOnce()
	})

	RunSpecs(t, "Upgrade Patches Suite")
}

var _ = Describe("upgradePatches", func() {

	Context("readUpgradeChangesFromFile", func() {
		It("should correctly parse and validate actual upgradePatches.json", func() {
			reader := bytes.NewReader(upgradePatchesFileContent)
			Expect(readJsonFromReader(reader)).To(Succeed())
		})

		It("should correctly parse and validate empty upgradePatches", func() {
			reader := bytes.NewReader(emptyFileContent)
			Expect(readJsonFromReader(reader)).To(Succeed())
		})

		Context("hcoCRPatchList", func() {
			DescribeTable(
				"should fail validating upgradePatches with bad patches",
				func(fileContent []byte, message string) {
					reader := bytes.NewReader(fileContent)
					Expect(readJsonFromReader(reader)).To(MatchError(HavePrefix(message)))
				},
				Entry(
					"invalid character",
					badJsonFileContent,
					"invalid character",
				),
				Entry(
					"bad semver range",
					badSemverRangeFileContent,
					"Could not get version from string:",
				),
				Entry(
					"bad operation kind",
					badPatches1FileContent,
					"Unexpected kind:",
				),
				Entry(
					"not on spec",
					badPatches2FileContent,
					"can only modify spec fields",
				),
				Entry(
					"unexisting path",
					badPatches3FileContent,
					"replace operation does not apply: doc is missing path:",
				),
			)

			DescribeTable(
				"should handle MissingPathOnRemove according to jsonPatchApplyOptions",
				func(fileContent []byte, expectedErr bool, message string) {
					reader := bytes.NewReader(fileContent)
					err := readJsonFromReader(reader)

					if expectedErr {
						Expect(err).To(MatchError(HavePrefix(message)))
					} else {
						Expect(err).ToNot(HaveOccurred())
					}
				},
				Entry(
					"without jsonPatchApplyOptions",
					badPatches4FileContent,
					true,
					"remove operation does not apply: doc is missing path: ",
				),
				Entry(
					"with AllowMissingPathOnRemove on jsonPatchApplyOptions",
					badPatches5FileContent,
					false,
					"",
				),
				Entry(
					"without jsonPatchApplyOptions",
					badPatches6FileContent,
					true,
					"add operation does not apply: doc is missing path: ",
				),
				Entry(
					"with EnsurePathExistsOnAdd on jsonPatchApplyOptions",
					badPatches7FileContent,
					false,
					"",
				),
			)

		})

		Context("objectsToBeRemoved", func() {
			DescribeTable(
				"should fail validating upgradePatches with bad patches",
				func(fileContent []byte, message string) {
					reader := bytes.NewReader(fileContent)
					Expect(readJsonFromReader(reader)).To(MatchError(HavePrefix(message)))
				},
				Entry(
					"bad semver ranges",
					badSemverRangeORFileContent,
					"Could not get version from string:",
				),
				Entry(
					"empty object kind",
					badObject1FileContent,
					"missing object kind",
				),
				Entry(
					"missing object kind",
					badObject1mFileContent,
					"missing object kind",
				),
				Entry(
					"empty object API version",
					badObject2FileContent,
					"missing object API version",
				),
				Entry(
					"missing object API version",
					badObject2mFileContent,
					"missing object API version",
				),
				Entry(
					"empty object name",
					badObject3FileContent,
					"missing object name",
				),
				Entry(
					"missing object name",
					badObject3mFileContent,
					"missing object name",
				),
			)
		})
	})

	Context("check semverRange type", func() {
		DescribeTable("check isAffectedRange", func(verRange, ver string, m types.GomegaMatcher) {
			vr, err := newSemverRange(verRange)
			Expect(err).NotTo(HaveOccurred())

			v, err := semver.Parse(ver)
			Expect(err).NotTo(HaveOccurred())
			Expect(vr.isAffectedRange(v)).To(m)
		},
			Entry("", ">4.16.0", "4.16.9", BeTrue()),
			Entry("", ">4.16.0", "4.17.2", BeTrue()),
			Entry("", ">4.16.0", "4.16.0", BeFalse()),
			Entry("", ">4.16.0", "4.15.2", BeFalse()),

			Entry("", "<=4.17.0", "4.16.4", BeTrue()),
			Entry("", "<=4.17.0", "4.17.2", BeFalse()),
			Entry("", "<4.17.3", "4.17.2", BeTrue()),
			Entry("", "<4.17.3", "4.17.3", BeFalse()),
			Entry("", "<4.17.3", "4.17.4", BeFalse()),
			Entry("", "<4.17.3", "4.16.9", BeTrue()),
		)
	})

	Context("check patches", func() {
		BeforeEach(func() {
			resetOnce()
			hcCRBytes = slices.Clone(hcCRBytesOrig)
		})

		It("should not modify CR when hcoCRPatchList is empty", func() {
			pwdFS := dirtest.New(dirtest.WithFile("upgradePatches.json", emptyFileContent))
			Expect(Init(pwdFS, GinkgoLogr)).To(Succeed())

			hc := commontestutils.NewHco()
			ver, err := semver.Parse("1.18.5")
			Expect(err).NotTo(HaveOccurred())

			newHc, err := ApplyUpgradePatch(GinkgoLogr, hc, ver)
			Expect(err).NotTo(HaveOccurred())

			Expect(newHc.Spec).To(Equal(hc.Spec))
		})

		It("should apply test+replace patch when version is in range", func() {
			pwdFS := dirtest.New(dirtest.WithFile("upgradePatches.json", upgradePatchesFileContent))
			Expect(Init(pwdFS, GinkgoLogr)).To(Succeed())

			hc := commontestutils.NewHco()
			Expect(hc.Spec.Virtualization.VirtualMachineOptions.DisableFreePageReporting).To(HaveValue(BeFalse()))

			ver, err := semver.Parse("1.18.5")
			Expect(err).NotTo(HaveOccurred())

			newHc, err := ApplyUpgradePatch(GinkgoLogr, hc, ver)
			Expect(err).NotTo(HaveOccurred())

			Expect(newHc.Spec.Virtualization.VirtualMachineOptions.DisableFreePageReporting).To(HaveValue(BeTrue()))
		})

		It("should not apply patch when version is out of range", func() {
			pwdFS := dirtest.New(dirtest.WithFile("upgradePatches.json", upgradePatchesFileContent))
			Expect(Init(pwdFS, GinkgoLogr)).To(Succeed())

			hc := commontestutils.NewHco()
			Expect(hc.Spec.Virtualization.VirtualMachineOptions.DisableFreePageReporting).To(HaveValue(BeFalse()))

			ver, err := semver.Parse("1.19.0")
			Expect(err).NotTo(HaveOccurred())

			newHc, err := ApplyUpgradePatch(GinkgoLogr, hc, ver)
			Expect(err).NotTo(HaveOccurred())

			Expect(newHc.Spec.Virtualization.VirtualMachineOptions.DisableFreePageReporting).To(HaveValue(BeFalse()))
		})

		It("should skip test+replace when test value does not match", func() {
			pwdFS := dirtest.New(dirtest.WithFile("upgradePatches.json", upgradePatchesFileContent))
			Expect(Init(pwdFS, GinkgoLogr)).To(Succeed())

			hc := commontestutils.NewHco()
			hc.Spec.Virtualization.VirtualMachineOptions.DisableFreePageReporting = ptr.To(true)

			ver, err := semver.Parse("1.18.5")
			Expect(err).NotTo(HaveOccurred())

			newHc, err := ApplyUpgradePatch(GinkgoLogr, hc, ver)
			Expect(err).NotTo(HaveOccurred())

			Expect(newHc.Spec.Virtualization.VirtualMachineOptions.DisableFreePageReporting).To(HaveValue(BeTrue()))
		})

		It("should apply remove patch", func() {
			pwdFS := dirtest.New(dirtest.WithFile("upgradePatches.json", upgradePatchesFileContent))
			Expect(Init(pwdFS, GinkgoLogr)).To(Succeed())

			hc := commontestutils.NewHco()

			ver, err := semver.Parse("1.18.0")
			Expect(err).NotTo(HaveOccurred())

			Expect(hc.Spec.Virtualization.VirtualMachineOptions).ToNot(BeNil())
			Expect(hc.Spec.Virtualization.VirtualMachineOptions.DisableSerialConsoleLog).To(HaveValue(BeFalse()))
			newHc, err := ApplyUpgradePatch(GinkgoLogr, hc, ver)
			Expect(err).NotTo(HaveOccurred())

			Expect(newHc).NotTo(BeNil())
			Expect(newHc.Spec.Virtualization.VirtualMachineOptions).ToNot(BeNil())
			Expect(newHc.Spec.Virtualization.VirtualMachineOptions.DisableSerialConsoleLog).To(BeNil())
		})

		It("should apply remove patch with allowMissingPathOnRemove", func() {
			pwdFS := dirtest.New(dirtest.WithFile("upgradePatches.json", missingPathOnRemove))
			Expect(Init(pwdFS, GinkgoLogr)).To(Succeed())

			hc := commontestutils.NewHco()
			hc.Spec.Virtualization.VirtualMachineOptions = nil

			ver, err := semver.Parse("1.18.0")
			Expect(err).NotTo(HaveOccurred())

			newHc, err := ApplyUpgradePatch(GinkgoLogr, hc, ver)
			Expect(err).NotTo(HaveOccurred())

			Expect(newHc).NotTo(BeNil())
			Expect(newHc.Spec.Virtualization.VirtualMachineOptions).To(BeNil())
		})
	})
})
