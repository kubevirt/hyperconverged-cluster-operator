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

	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/dirtest"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/components"
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

	//nolint:staticcheck // ignore SA1019 for old code
	Context("check patches", func() {
		BeforeEach(func() {
			resetOnce()
			pwdFS := dirtest.New(dirtest.WithFile("upgradePatches.json", upgradePatchesFileContent))
			Expect(Init(pwdFS, GinkgoLogr)).To(Succeed())
			hcCRBytes = slices.Clone(hcCRBytesOrig)
		})

		It("should apply changes as defined in the upgradePatches.json file", func() {
			hc := components.GetOperatorCR()
			hc.Spec.FeatureGates.DeployKubevirtIpamController = ptr.To(false)
			hc.Spec.FeatureGates.EnableManagedTenantQuota = ptr.To(false)
			hc.Spec.FeatureGates.EnableManagedTenantQuota = ptr.To(false)
			hc.Spec.FeatureGates.NonRoot = ptr.To(false)
			hc.Spec.FeatureGates.WithHostPassthroughCPU = ptr.To(false)
			hc.Spec.FeatureGates.PrimaryUserDefinedNetworkBinding = ptr.To(false)

			ver, err := semver.Parse("1.13.9")
			Expect(err).NotTo(HaveOccurred())

			newHc, err := ApplyUpgradePatch(GinkgoLogr, hc, ver)
			Expect(err).NotTo(HaveOccurred())

			Expect(newHc.Spec.FeatureGates.DeployKubevirtIpamController).To(BeNil())
			Expect(newHc.Spec.FeatureGates.EnableManagedTenantQuota).To(BeNil())
			Expect(newHc.Spec.FeatureGates.EnableManagedTenantQuota).To(BeNil())
			Expect(newHc.Spec.FeatureGates.NonRoot).To(BeNil())
			Expect(newHc.Spec.FeatureGates.WithHostPassthroughCPU).To(BeNil())
			Expect(newHc.Spec.FeatureGates.PrimaryUserDefinedNetworkBinding).To(BeNil())
		})

		DescribeTable("Moving the deprecated EnableCommonBootImageImport FG to a new field",
			func(oldFG, newFG *bool, ver semver.Version, assertField, assertFG types.GomegaMatcher) {
				hc := components.GetOperatorCR()
				hc.Spec.FeatureGates.EnableCommonBootImageImport = oldFG
				hc.Spec.EnableCommonBootImageImport = newFG

				newHc, err := ApplyUpgradePatch(GinkgoLogr, hc, ver)
				Expect(err).NotTo(HaveOccurred())

				Expect(newHc.Spec.EnableCommonBootImageImport).To(assertField)
				Expect(newHc.Spec.FeatureGates.EnableCommonBootImageImport).To(assertFG)
			},
			Entry("should move if the old value is not the default value",
				ptr.To(false),
				ptr.To(true),
				semver.MustParse("1.14.0"),
				HaveValue(BeFalse()),
				BeNil(),
			),
			Entry("should not move for newer versions",
				ptr.To(false),
				ptr.To(true),
				semver.MustParse("1.15.0"),
				HaveValue(BeTrue()),
				HaveValue(BeFalse()),
			),
			Entry("should not move if the old FG is empty",
				nil,
				ptr.To(true),
				semver.MustParse("1.14.0"),
				HaveValue(BeTrue()),
				BeNil(),
			),
			Entry("should not move if the old FG is the default one",
				ptr.To(true),
				ptr.To(false),
				semver.MustParse("1.14.0"),
				HaveValue(BeFalse()),
				BeNil(),
			),
		)
	})
})
