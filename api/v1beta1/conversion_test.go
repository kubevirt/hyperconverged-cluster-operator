package v1beta1_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	"github.com/kubevirt/hyperconverged-cluster-operator/api/v1/featuregates"
	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
)

func TestConversionSuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Conversion Suite")
}

var _ = Describe("Feature Gates Conversion", func() {

	Describe("ConvertV1Beta1FGsToV1FGs", func() {

		It("should return empty slice when all feature gates are nil", func() {
			src := hcov1beta1.HyperConvergedFeatureGates{}

			result := hcov1beta1.ConvertV1Beta1FGsToV1FGs(src)

			Expect(result).To(BeEmpty())
		})

		It("should return empty slice when all feature gates are false", func() {
			src := hcov1beta1.HyperConvergedFeatureGates{
				DownwardMetrics:                ptr.To(false),
				DeployKubeSecondaryDNS:         ptr.To(false),
				DisableMDevConfiguration:       ptr.To(false),
				PersistentReservation:          ptr.To(false),
				AlignCPUs:                      ptr.To(false),
				EnableMultiArchBootImageImport: ptr.To(false),
				DecentralizedLiveMigration:     ptr.To(false),
				DeclarativeHotplugVolumes:      ptr.To(false),
				VideoConfig:                    ptr.To(true), // beta FG is enabled by default
				ObjectGraph:                    ptr.To(false),
			}

			result := hcov1beta1.ConvertV1Beta1FGsToV1FGs(src)

			Expect(result).To(BeEmpty())
		})

		It("should enable downwardMetrics when set to true", func() {
			src := hcov1beta1.HyperConvergedFeatureGates{
				DownwardMetrics: ptr.To(true),
			}

			result := hcov1beta1.ConvertV1Beta1FGsToV1FGs(src)

			Expect(result).To(HaveLen(1))
			Expect(result).To(ContainElement(featuregates.FeatureGate{
				Name:    "downwardMetrics",
				Enabled: ptr.To(featuregates.Enabled),
			}))
		})

		It("should enable deployKubeSecondaryDNS when set to true", func() {
			src := hcov1beta1.HyperConvergedFeatureGates{
				DeployKubeSecondaryDNS: ptr.To(true),
			}

			result := hcov1beta1.ConvertV1Beta1FGsToV1FGs(src)

			Expect(result).To(HaveLen(1))
			Expect(result).To(ContainElement(featuregates.FeatureGate{
				Name:    "deployKubeSecondaryDNS",
				Enabled: ptr.To(featuregates.Enabled),
			}))
		})

		It("should enable disableMDevConfiguration when set to true", func() {
			src := hcov1beta1.HyperConvergedFeatureGates{
				DisableMDevConfiguration: ptr.To(true),
			}

			result := hcov1beta1.ConvertV1Beta1FGsToV1FGs(src)

			Expect(result).To(HaveLen(1))
			Expect(result).To(ContainElement(featuregates.FeatureGate{
				Name:    "disableMDevConfiguration",
				Enabled: ptr.To(featuregates.Enabled),
			}))
		})

		It("should enable persistentReservation when set to true", func() {
			src := hcov1beta1.HyperConvergedFeatureGates{
				PersistentReservation: ptr.To(true),
			}

			result := hcov1beta1.ConvertV1Beta1FGsToV1FGs(src)

			Expect(result).To(HaveLen(1))
			Expect(result).To(ContainElement(featuregates.FeatureGate{
				Name:    "persistentReservation",
				Enabled: ptr.To(featuregates.Enabled),
			}))
		})

		It("should enable alignCPUs when set to true", func() {
			src := hcov1beta1.HyperConvergedFeatureGates{
				AlignCPUs: ptr.To(true),
			}

			result := hcov1beta1.ConvertV1Beta1FGsToV1FGs(src)

			Expect(result).To(HaveLen(1))
			Expect(result).To(ContainElement(featuregates.FeatureGate{
				Name:    "alignCPUs",
				Enabled: ptr.To(featuregates.Enabled),
			}))
		})

		It("should enable enableMultiArchBootImageImport when set to true", func() {
			src := hcov1beta1.HyperConvergedFeatureGates{
				EnableMultiArchBootImageImport: ptr.To(true),
			}

			result := hcov1beta1.ConvertV1Beta1FGsToV1FGs(src)

			Expect(result).To(HaveLen(1))
			Expect(result).To(ContainElement(featuregates.FeatureGate{
				Name:    "enableMultiArchBootImageImport",
				Enabled: ptr.To(featuregates.Enabled),
			}))
		})

		It("should enable decentralizedLiveMigration when set to true", func() {
			src := hcov1beta1.HyperConvergedFeatureGates{
				DecentralizedLiveMigration: ptr.To(true),
			}

			result := hcov1beta1.ConvertV1Beta1FGsToV1FGs(src)

			Expect(result).To(HaveLen(1))
			Expect(result).To(ContainElement(featuregates.FeatureGate{
				Name:    "decentralizedLiveMigration",
				Enabled: ptr.To(featuregates.Enabled),
			}))
		})

		It("should enable declarativeHotplugVolumes when set to true", func() {
			src := hcov1beta1.HyperConvergedFeatureGates{
				DeclarativeHotplugVolumes: ptr.To(true),
			}

			result := hcov1beta1.ConvertV1Beta1FGsToV1FGs(src)

			Expect(result).To(HaveLen(1))
			Expect(result).To(ContainElement(featuregates.FeatureGate{
				Name:    "declarativeHotplugVolumes",
				Enabled: ptr.To(featuregates.Enabled),
			}))
		})

		It("should disable videoConfig when set to false", func() {
			src := hcov1beta1.HyperConvergedFeatureGates{
				VideoConfig: ptr.To(false),
			}

			result := hcov1beta1.ConvertV1Beta1FGsToV1FGs(src)

			Expect(result).To(HaveLen(1))
			Expect(result).To(ContainElement(featuregates.FeatureGate{
				Name:    "videoConfig",
				Enabled: ptr.To(featuregates.Disabled),
			}))
		})

		It("should enable objectGraph when set to true", func() {
			src := hcov1beta1.HyperConvergedFeatureGates{
				ObjectGraph: ptr.To(true),
			}

			result := hcov1beta1.ConvertV1Beta1FGsToV1FGs(src)

			Expect(result).To(HaveLen(1))
			Expect(result).To(ContainElement(featuregates.FeatureGate{
				Name:    "objectGraph",
				Enabled: ptr.To(featuregates.Enabled),
			}))
		})

		It("should convert multiple enabled feature gates", func() {
			src := hcov1beta1.HyperConvergedFeatureGates{
				DownwardMetrics:       ptr.To(true),
				AlignCPUs:             ptr.To(true),
				VideoConfig:           ptr.To(false),
				PersistentReservation: ptr.To(true),
			}

			result := hcov1beta1.ConvertV1Beta1FGsToV1FGs(src)

			Expect(result).To(HaveLen(4))
			Expect(result).To(ContainElement(featuregates.FeatureGate{
				Name:    "downwardMetrics",
				Enabled: ptr.To(featuregates.Enabled),
			}))
			Expect(result).To(ContainElement(featuregates.FeatureGate{
				Name:    "alignCPUs",
				Enabled: ptr.To(featuregates.Enabled),
			}))
			Expect(result).To(ContainElement(featuregates.FeatureGate{
				Name:    "videoConfig",
				Enabled: ptr.To(featuregates.Disabled),
			}))
			Expect(result).To(ContainElement(featuregates.FeatureGate{
				Name:    "persistentReservation",
				Enabled: ptr.To(featuregates.Enabled),
			}))
		})

		It("should convert all feature gates when all are enabled", func() {
			src := hcov1beta1.HyperConvergedFeatureGates{
				DownwardMetrics:                ptr.To(true),
				DeployKubeSecondaryDNS:         ptr.To(true),
				DisableMDevConfiguration:       ptr.To(true),
				PersistentReservation:          ptr.To(true),
				AlignCPUs:                      ptr.To(true),
				EnableMultiArchBootImageImport: ptr.To(true),
				DecentralizedLiveMigration:     ptr.To(true),
				DeclarativeHotplugVolumes:      ptr.To(true),
				VideoConfig:                    ptr.To(false), // false means disable, so entry is added
				ObjectGraph:                    ptr.To(true),
			}

			result := hcov1beta1.ConvertV1Beta1FGsToV1FGs(src)

			Expect(result).To(HaveLen(10))
		})

	})

	Describe("ConvertV1FGsToV1Beta1FGs", func() {

		It("should set all fields to false when source is empty", func() {
			src := featuregates.HyperConvergedFeatureGates{}
			dst := hcov1beta1.HyperConvergedFeatureGates{}

			hcov1beta1.ConvertV1FGsToV1Beta1FGs(src, &dst)

			Expect(dst.DownwardMetrics).To(HaveValue(BeFalse()))
			Expect(dst.DeployKubeSecondaryDNS).To(HaveValue(BeFalse()))
			Expect(dst.DisableMDevConfiguration).To(HaveValue(BeFalse()))
			Expect(dst.PersistentReservation).To(HaveValue(BeFalse()))
			Expect(dst.AlignCPUs).To(HaveValue(BeFalse()))
			Expect(dst.EnableMultiArchBootImageImport).To(HaveValue(BeFalse()))
			Expect(dst.DecentralizedLiveMigration).To(HaveValue(BeFalse()))
			Expect(dst.DeclarativeHotplugVolumes).To(HaveValue(BeFalse()))
			// videoConfig is beta, so it defaults to enabled (true) when not in list
			Expect(dst.VideoConfig).To(HaveValue(BeTrue()))
			Expect(dst.ObjectGraph).To(HaveValue(BeFalse()))
		})

		It("should set downwardMetrics to true when enabled in source", func() {
			src := featuregates.HyperConvergedFeatureGates{
				{Name: "downwardMetrics", Enabled: ptr.To(featuregates.Enabled)},
			}
			dst := hcov1beta1.HyperConvergedFeatureGates{}

			hcov1beta1.ConvertV1FGsToV1Beta1FGs(src, &dst)

			Expect(dst.DownwardMetrics).To(HaveValue(BeTrue()))
		})

		It("should set deployKubeSecondaryDNS to true when enabled in source", func() {
			src := featuregates.HyperConvergedFeatureGates{
				{Name: "deployKubeSecondaryDNS", Enabled: ptr.To(featuregates.Enabled)},
			}
			dst := hcov1beta1.HyperConvergedFeatureGates{}

			hcov1beta1.ConvertV1FGsToV1Beta1FGs(src, &dst)

			Expect(dst.DeployKubeSecondaryDNS).To(HaveValue(BeTrue()))
		})

		It("should set disableMDevConfiguration to true when enabled in source", func() {
			src := featuregates.HyperConvergedFeatureGates{
				{Name: "disableMDevConfiguration", Enabled: ptr.To(featuregates.Enabled)},
			}
			dst := hcov1beta1.HyperConvergedFeatureGates{}

			hcov1beta1.ConvertV1FGsToV1Beta1FGs(src, &dst)

			Expect(dst.DisableMDevConfiguration).To(HaveValue(BeTrue()))
		})

		It("should set persistentReservation to true when enabled in source", func() {
			src := featuregates.HyperConvergedFeatureGates{
				{Name: "persistentReservation", Enabled: ptr.To(featuregates.Enabled)},
			}
			dst := hcov1beta1.HyperConvergedFeatureGates{}

			hcov1beta1.ConvertV1FGsToV1Beta1FGs(src, &dst)

			Expect(dst.PersistentReservation).To(HaveValue(BeTrue()))
		})

		It("should set alignCPUs to true when enabled in source", func() {
			src := featuregates.HyperConvergedFeatureGates{
				{Name: "alignCPUs", Enabled: ptr.To(featuregates.Enabled)},
			}
			dst := hcov1beta1.HyperConvergedFeatureGates{}

			hcov1beta1.ConvertV1FGsToV1Beta1FGs(src, &dst)

			Expect(dst.AlignCPUs).To(HaveValue(BeTrue()))
		})

		It("should set enableMultiArchBootImageImport to true when enabled in source", func() {
			src := featuregates.HyperConvergedFeatureGates{
				{Name: "enableMultiArchBootImageImport", Enabled: ptr.To(featuregates.Enabled)},
			}
			dst := hcov1beta1.HyperConvergedFeatureGates{}

			hcov1beta1.ConvertV1FGsToV1Beta1FGs(src, &dst)

			Expect(dst.EnableMultiArchBootImageImport).To(HaveValue(BeTrue()))
		})

		It("should set decentralizedLiveMigration to true when enabled in source", func() {
			src := featuregates.HyperConvergedFeatureGates{
				{Name: "decentralizedLiveMigration", Enabled: ptr.To(featuregates.Enabled)},
			}
			dst := hcov1beta1.HyperConvergedFeatureGates{}

			hcov1beta1.ConvertV1FGsToV1Beta1FGs(src, &dst)

			Expect(dst.DecentralizedLiveMigration).To(HaveValue(BeTrue()))
		})

		It("should set declarativeHotplugVolumes to true when enabled in source", func() {
			src := featuregates.HyperConvergedFeatureGates{
				{Name: "declarativeHotplugVolumes", Enabled: ptr.To(featuregates.Enabled)},
			}
			dst := hcov1beta1.HyperConvergedFeatureGates{}

			hcov1beta1.ConvertV1FGsToV1Beta1FGs(src, &dst)

			Expect(dst.DeclarativeHotplugVolumes).To(HaveValue(BeTrue()))
		})

		It("should set videoConfig to false when disabled in source", func() {
			src := featuregates.HyperConvergedFeatureGates{
				{Name: "videoConfig", Enabled: ptr.To(featuregates.Disabled)},
			}
			dst := hcov1beta1.HyperConvergedFeatureGates{}

			hcov1beta1.ConvertV1FGsToV1Beta1FGs(src, &dst)

			Expect(dst.VideoConfig).To(HaveValue(BeFalse()))
		})

		It("should set objectGraph to true when enabled in source", func() {
			src := featuregates.HyperConvergedFeatureGates{
				{Name: "objectGraph", Enabled: ptr.To(featuregates.Enabled)},
			}
			dst := hcov1beta1.HyperConvergedFeatureGates{}

			hcov1beta1.ConvertV1FGsToV1Beta1FGs(src, &dst)

			Expect(dst.ObjectGraph).To(HaveValue(BeTrue()))
		})

		It("should convert multiple feature gates", func() {
			src := featuregates.HyperConvergedFeatureGates{
				{Name: "downwardMetrics", Enabled: ptr.To(featuregates.Enabled)},
				{Name: "alignCPUs", Enabled: ptr.To(featuregates.Enabled)},
				{Name: "videoConfig", Enabled: ptr.To(featuregates.Disabled)},
				{Name: "persistentReservation", Enabled: ptr.To(featuregates.Enabled)},
			}
			dst := hcov1beta1.HyperConvergedFeatureGates{}

			hcov1beta1.ConvertV1FGsToV1Beta1FGs(src, &dst)

			Expect(dst.DownwardMetrics).To(HaveValue(BeTrue()))
			Expect(dst.AlignCPUs).To(HaveValue(BeTrue()))
			Expect(dst.VideoConfig).To(HaveValue(BeFalse()))
			Expect(dst.PersistentReservation).To(HaveValue(BeTrue()))
		})

	})

	Describe("Round-trip conversion", func() {

		It("should preserve enabled feature gates through v1beta1 -> v1 -> v1beta1 conversion", func() {
			original := hcov1beta1.HyperConvergedFeatureGates{
				DownwardMetrics:                ptr.To(true),
				DeployKubeSecondaryDNS:         ptr.To(true),
				DisableMDevConfiguration:       ptr.To(false),
				PersistentReservation:          ptr.To(true),
				AlignCPUs:                      ptr.To(false),
				EnableMultiArchBootImageImport: ptr.To(true),
				DecentralizedLiveMigration:     ptr.To(false),
				DeclarativeHotplugVolumes:      ptr.To(true),
				VideoConfig:                    ptr.To(false),
				ObjectGraph:                    ptr.To(true),
			}

			// Convert to v1
			v1FGs := hcov1beta1.ConvertV1Beta1FGsToV1FGs(original)

			// Convert back to v1beta1
			result := hcov1beta1.HyperConvergedFeatureGates{}
			hcov1beta1.ConvertV1FGsToV1Beta1FGs(v1FGs, &result)

			Expect(result.DownwardMetrics).To(HaveValue(Equal(*original.DownwardMetrics)))
			Expect(result.DeployKubeSecondaryDNS).To(HaveValue(Equal(*original.DeployKubeSecondaryDNS)))
			Expect(result.DisableMDevConfiguration).To(HaveValue(Equal(*original.DisableMDevConfiguration)))
			Expect(result.PersistentReservation).To(HaveValue(Equal(*original.PersistentReservation)))
			Expect(result.AlignCPUs).To(HaveValue(Equal(*original.AlignCPUs)))
			Expect(result.EnableMultiArchBootImageImport).To(HaveValue(Equal(*original.EnableMultiArchBootImageImport)))
			Expect(result.DecentralizedLiveMigration).To(HaveValue(Equal(*original.DecentralizedLiveMigration)))
			Expect(result.DeclarativeHotplugVolumes).To(HaveValue(Equal(*original.DeclarativeHotplugVolumes)))
			Expect(result.VideoConfig).To(HaveValue(Equal(*original.VideoConfig)))
			Expect(result.ObjectGraph).To(HaveValue(Equal(*original.ObjectGraph)))
		})

		It("should preserve feature gates through v1 -> v1beta1 -> v1 conversion", func() {
			original := featuregates.HyperConvergedFeatureGates{
				{Name: "downwardMetrics", Enabled: ptr.To(featuregates.Enabled)},
				{Name: "deployKubeSecondaryDNS", Enabled: ptr.To(featuregates.Enabled)},
				{Name: "alignCPUs", Enabled: ptr.To(featuregates.Enabled)},
				{Name: "videoConfig", Enabled: ptr.To(featuregates.Disabled)},
			}

			// Convert to v1beta1
			v1beta1FGs := hcov1beta1.HyperConvergedFeatureGates{}
			hcov1beta1.ConvertV1FGsToV1Beta1FGs(original, &v1beta1FGs)

			// Convert back to v1
			result := hcov1beta1.ConvertV1Beta1FGsToV1FGs(v1beta1FGs)

			// Check that the enabled feature gates are preserved
			Expect(result).To(ContainElement(featuregates.FeatureGate{
				Name:    "downwardMetrics",
				Enabled: ptr.To(featuregates.Enabled),
			}))
			Expect(result).To(ContainElement(featuregates.FeatureGate{
				Name:    "deployKubeSecondaryDNS",
				Enabled: ptr.To(featuregates.Enabled),
			}))
			Expect(result).To(ContainElement(featuregates.FeatureGate{
				Name:    "alignCPUs",
				Enabled: ptr.To(featuregates.Enabled),
			}))
			Expect(result).To(ContainElement(featuregates.FeatureGate{
				Name:    "videoConfig",
				Enabled: ptr.To(featuregates.Disabled),
			}))
		})

	})

})
