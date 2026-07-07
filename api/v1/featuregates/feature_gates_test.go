package featuregates_test

import (
	"encoding/json"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gtypes "github.com/onsi/gomega/types"
	"sigs.k8s.io/yaml"

	"github.com/kubevirt/hyperconverged-cluster-operator/api/v1/featuregates"
)

func TestFeatureGatesSuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "FeatureGates Suite")
}

var _ = Describe("FeatureGate", func() {
	Context("Marshal", func() {
		It("should marshal an enabled feature gate", func() {
			fg := featuregates.FeatureGate{
				Name:  "fgName",
				State: new(featuregates.Enabled),
			}

			jsonBytes, err := json.Marshal(fg)
			Expect(err).ToNot(HaveOccurred())
			Expect(jsonBytes).To(MatchJSON(`{"name":"fgName"}`))
		})

		It("should marshal a disabled feature gate", func() {
			fg := featuregates.FeatureGate{
				Name:  "fgName",
				State: new(featuregates.Disabled),
			}

			jsonBytes, err := json.Marshal(fg)
			Expect(err).ToNot(HaveOccurred())
			Expect(jsonBytes).To(MatchJSON(`{"name":"fgName","state":"Disabled"}`))
		})

		It("should marshal an enabled feature gate pointer", func() {
			fg := &featuregates.FeatureGate{
				Name:  "fgName",
				State: new(featuregates.Enabled),
			}

			jsonBytes, err := json.Marshal(fg)
			Expect(err).ToNot(HaveOccurred())
			Expect(jsonBytes).To(MatchJSON(`{"name":"fgName"}`))
		})

		It("should marshal a disabled feature gate pointer", func() {
			fg := &featuregates.FeatureGate{
				Name:  "fgName",
				State: new(featuregates.Disabled),
			}

			jsonBytes, err := json.Marshal(fg)
			Expect(err).ToNot(HaveOccurred())
			Expect(jsonBytes).To(MatchJSON(`{"name":"fgName","state":"Disabled"}`))
		})

		It("should marshal a feature gate without enabled field, as enabled", func() {
			fg := featuregates.FeatureGate{
				Name: "fgName",
			}

			jsonBytes, err := json.Marshal(fg)
			Expect(err).ToNot(HaveOccurred())
			Expect(jsonBytes).To(MatchJSON(`{"name":"fgName"}`))
		})

		It("should marshal a FG array", func() {
			fgs := featuregates.HyperConvergedFeatureGates{
				{
					Name: "noEnabledField",
				},
				{
					Name:  "enabledFG",
					State: new(featuregates.Enabled),
				},
				{
					Name:  "disabledFG",
					State: new(featuregates.Disabled),
				},
			}

			jsonBytes, err := json.Marshal(fgs)
			Expect(err).ToNot(HaveOccurred())
			Expect(jsonBytes).To(MatchJSON(`[{"name":"noEnabledField"}, {"name": "enabledFG"}, {"name": "disabledFG", "state": "Disabled"}]`))
		})

		It("should yaml marshal a FG array", func() {
			fgs := featuregates.HyperConvergedFeatureGates{
				{
					Name: "noEnabledField",
				},
				{
					Name:  "enabledFG",
					State: new(featuregates.Enabled),
				},
				{
					Name:  "disabledFG",
					State: new(featuregates.Disabled),
				},
			}

			yamlBytes, err := yaml.Marshal(fgs)
			Expect(err).ToNot(HaveOccurred())
			Expect(yamlBytes).To(MatchYAML(`- name: noEnabledField
- name: enabledFG
- state: Disabled
  name: disabledFG`,
			))
		})
	})

	Context("Unmarshal", func() {
		It("should unmarshal an enabled feature gate", func() {
			fgBytes := []byte(`{"name":"fgName", "enabled":"True"}`)
			fg := featuregates.FeatureGate{}
			Expect(json.Unmarshal(fgBytes, &fg)).To(Succeed())
			Expect(fg.Name).To(Equal("fgName"))
			Expect(fg.State).To(HaveValue(Equal(featuregates.Enabled)))
		})

		It("should unmarshal a disabled feature gate", func() {
			fgBytes := []byte(`{"name":"fgName", "state":"Disabled"}`)
			fg := featuregates.FeatureGate{}
			Expect(json.Unmarshal(fgBytes, &fg)).To(Succeed())
			Expect(fg.Name).To(Equal("fgName"))
			Expect(fg.State).To(HaveValue(Equal(featuregates.Disabled)))
		})

		It("should unmarshal a feature gate w/o enabled field, as enabled FG", func() {
			fgBytes := []byte(`{"name":"fgName"}`)
			fg := featuregates.FeatureGate{}
			Expect(json.Unmarshal(fgBytes, &fg)).To(Succeed())
			Expect(fg.Name).To(Equal("fgName"))
			Expect(fg.State).To(HaveValue(Equal(featuregates.Enabled)))
		})

		It("should unmarshal an array of FGs", func() {
			fgBytes := []byte(`[{"name":"noEnabledField"}, {"name": "enabledFG", "state": "Enabled"}, {"name": "disabledFG", "state": "Disabled"}]`)
			fgs := featuregates.HyperConvergedFeatureGates{}

			Expect(json.Unmarshal(fgBytes, &fgs)).To(Succeed())
			Expect(fgs).To(HaveLen(3))
			Expect(fgs).To(ContainElements(
				featuregates.FeatureGate{Name: "noEnabledField", State: new(featuregates.Enabled)},
				featuregates.FeatureGate{Name: "enabledFG", State: new(featuregates.Enabled)},
				featuregates.FeatureGate{Name: "disabledFG", State: new(featuregates.Disabled)},
			))
		})

		It("should unmarshal a yaml array of FGs", func() {
			fgBytes := []byte(`- name: noEnabledField
- name: enabledFG
  state: Enabled
- state: Disabled
  name: disabledFG`,
			)
			fgs := featuregates.HyperConvergedFeatureGates{}

			Expect(yaml.Unmarshal(fgBytes, &fgs)).To(Succeed())
			Expect(fgs).To(HaveLen(3))
			Expect(fgs).To(ContainElements(
				featuregates.FeatureGate{Name: "noEnabledField", State: new(featuregates.Enabled)},
				featuregates.FeatureGate{Name: "enabledFG", State: new(featuregates.Enabled)},
				featuregates.FeatureGate{Name: "disabledFG", State: new(featuregates.Disabled)},
			))
		})
	})
})

var _ = Describe("Feature Gates", func() {
	DescribeTable("check IsEnabled", func(fgs featuregates.HyperConvergedFeatureGates, fgName string, expected gtypes.GomegaMatcher) {
		Expect(fgs.IsEnabled(fgName)).To(expected)
	},
		Entry("unknown FG; in list", featuregates.HyperConvergedFeatureGates{{Name: "unknown", State: new(featuregates.Enabled)}}, "unknown", BeFalse()),
		Entry("unknown FG; not in list", featuregates.HyperConvergedFeatureGates{{Name: "deployKubeSecondaryDNS", State: new(featuregates.Enabled)}}, "unknown", BeFalse()),

		Entry("known alpha FG; in list; enabled", featuregates.HyperConvergedFeatureGates{{Name: "downwardMetrics", State: new(featuregates.Enabled)}}, "downwardMetrics", BeTrue()),
		Entry("known alpha FG; in list; disabled", featuregates.HyperConvergedFeatureGates{{Name: "downwardMetrics", State: new(featuregates.Disabled)}}, "downwardMetrics", BeFalse()),
		Entry("known alpha FG; not in list;", featuregates.HyperConvergedFeatureGates{{Name: "deployKubeSecondaryDNS", State: new(featuregates.Enabled)}}, "downwardMetrics", BeFalse()),

		Entry("known alpha FG with different casing; in list; enabled", featuregates.HyperConvergedFeatureGates{{Name: "DownwardMetricS", State: new(featuregates.Enabled)}}, "downwardMetrics", BeTrue()),
		Entry("known alpha FG with different casing; in list; disabled", featuregates.HyperConvergedFeatureGates{{Name: "DownwardMetricS", State: new(featuregates.Disabled)}}, "downwardMetrics", BeFalse()),

		Entry("known beta FG; in list; enabled", featuregates.HyperConvergedFeatureGates{{Name: "declarativeHotplugVolumes", State: new(featuregates.Enabled)}}, "declarativeHotplugVolumes", BeTrue()),
		Entry("known beta FG; in list; disabled", featuregates.HyperConvergedFeatureGates{{Name: "declarativeHotplugVolumes", State: new(featuregates.Disabled)}}, "declarativeHotplugVolumes", BeFalse()),
		Entry("known beta FG; not in list;", featuregates.HyperConvergedFeatureGates{{Name: "deployKubeSecondaryDNS", State: new(featuregates.Enabled)}}, "declarativeHotplugVolumes", BeTrue()),

		Entry("known beta FG with different casing; in list; enabled", featuregates.HyperConvergedFeatureGates{{Name: "DeclarativeHotplugVolumeS", State: new(featuregates.Enabled)}}, "declarativeHotplugVolumes", BeTrue()),
		Entry("known beta FG with different casing; in list; disabled", featuregates.HyperConvergedFeatureGates{{Name: "DeclarativeHotplugVolumeS", State: new(featuregates.Disabled)}}, "declarativeHotplugVolumes", BeFalse()),

		Entry("known deprecated FG; in list; enabled", featuregates.HyperConvergedFeatureGates{{Name: "withHostPassthroughCPU", State: new(featuregates.Enabled)}}, "withHostPassthroughCPU", BeFalse()),
		Entry("known deprecated FG; in list; disabled", featuregates.HyperConvergedFeatureGates{{Name: "withHostPassthroughCPU", State: new(featuregates.Disabled)}}, "withHostPassthroughCPU", BeFalse()),
		Entry("known deprecated FG; not in list;", featuregates.HyperConvergedFeatureGates{{Name: "deployKubeSecondaryDNS", State: new(featuregates.Enabled)}}, "withHostPassthroughCPU", BeFalse()),
	)

	Context("check Enable", func() {
		It("should add to nil", func() {
			var fgs featuregates.HyperConvergedFeatureGates

			fgs.Enable("aaa")

			Expect(fgs).To(HaveLen(1))
			Expect(fgs).To(ContainElement(Equal(featuregates.FeatureGate{Name: "aaa", State: new(featuregates.Enabled)})))
		})

		It("should add to non-empty", func() {
			fgs := featuregates.HyperConvergedFeatureGates{
				{Name: "aaa", State: new(featuregates.Enabled)},
			}

			fgs.Enable("bbb")

			Expect(fgs).To(HaveLen(2))
			Expect(fgs).To(ContainElement(Equal(featuregates.FeatureGate{Name: "aaa", State: new(featuregates.Enabled)})))
			Expect(fgs).To(ContainElement(Equal(featuregates.FeatureGate{Name: "bbb", State: new(featuregates.Enabled)})))
		})

		It("should update if already exist - first item", func() {
			fgs := featuregates.HyperConvergedFeatureGates{
				{Name: "aaa", State: new(featuregates.Disabled)},
				{Name: "bbb", State: new(featuregates.Disabled)},
				{Name: "ccc", State: new(featuregates.Disabled)},
			}

			fgs.Enable("aaa")

			Expect(fgs).To(HaveLen(3))
			Expect(fgs).To(ContainElement(Equal(featuregates.FeatureGate{Name: "aaa", State: new(featuregates.Enabled)})))
			Expect(fgs).To(ContainElement(Equal(featuregates.FeatureGate{Name: "bbb", State: new(featuregates.Disabled)})))
			Expect(fgs).To(ContainElement(Equal(featuregates.FeatureGate{Name: "ccc", State: new(featuregates.Disabled)})))
		})

		It("should update if already exist - middle item", func() {
			fgs := featuregates.HyperConvergedFeatureGates{
				{Name: "aaa", State: new(featuregates.Disabled)},
				{Name: "bbb", State: new(featuregates.Disabled)},
				{Name: "ccc", State: new(featuregates.Disabled)},
			}

			fgs.Enable("bbb")

			Expect(fgs).To(HaveLen(3))
			Expect(fgs).To(ContainElement(Equal(featuregates.FeatureGate{Name: "aaa", State: new(featuregates.Disabled)})))
			Expect(fgs).To(ContainElement(Equal(featuregates.FeatureGate{Name: "bbb", State: new(featuregates.Enabled)})))
			Expect(fgs).To(ContainElement(Equal(featuregates.FeatureGate{Name: "ccc", State: new(featuregates.Disabled)})))
		})

		It("should update if already exist - last item", func() {
			fgs := featuregates.HyperConvergedFeatureGates{
				{Name: "aaa", State: new(featuregates.Disabled)},
				{Name: "bbb", State: new(featuregates.Disabled)},
				{Name: "ccc", State: new(featuregates.Disabled)},
			}

			fgs.Enable("ccc")

			Expect(fgs).To(HaveLen(3))
			Expect(fgs).To(ContainElement(Equal(featuregates.FeatureGate{Name: "aaa", State: new(featuregates.Disabled)})))
			Expect(fgs).To(ContainElement(Equal(featuregates.FeatureGate{Name: "bbb", State: new(featuregates.Disabled)})))
			Expect(fgs).To(ContainElement(Equal(featuregates.FeatureGate{Name: "ccc", State: new(featuregates.Enabled)})))
		})
	})

	Context("check Disable", func() {
		It("should add to nil", func() {
			var fgs featuregates.HyperConvergedFeatureGates

			fgs.Disable("aaa")

			Expect(fgs).To(HaveLen(1))
			Expect(fgs).To(ContainElement(Equal(featuregates.FeatureGate{Name: "aaa", State: new(featuregates.Disabled)})))
		})

		It("should add to non-empty", func() {
			fgs := featuregates.HyperConvergedFeatureGates{
				{Name: "aaa", State: new(featuregates.Enabled)},
			}

			fgs.Disable("bbb")

			Expect(fgs).To(HaveLen(2))
			Expect(fgs).To(ContainElement(Equal(featuregates.FeatureGate{Name: "aaa", State: new(featuregates.Enabled)})))
			Expect(fgs).To(ContainElement(Equal(featuregates.FeatureGate{Name: "bbb", State: new(featuregates.Disabled)})))
		})

		It("should update if already exist - first item", func() {
			fgs := featuregates.HyperConvergedFeatureGates{
				{Name: "aaa", State: new(featuregates.Enabled)},
				{Name: "bbb", State: new(featuregates.Enabled)},
				{Name: "ccc", State: new(featuregates.Enabled)},
			}

			fgs.Disable("aaa")

			Expect(fgs).To(HaveLen(3))
			Expect(fgs).To(ContainElement(Equal(featuregates.FeatureGate{Name: "aaa", State: new(featuregates.Disabled)})))
			Expect(fgs).To(ContainElement(Equal(featuregates.FeatureGate{Name: "bbb", State: new(featuregates.Enabled)})))
			Expect(fgs).To(ContainElement(Equal(featuregates.FeatureGate{Name: "ccc", State: new(featuregates.Enabled)})))
		})

		It("should update if already exist - middle item", func() {
			fgs := featuregates.HyperConvergedFeatureGates{
				{Name: "aaa", State: new(featuregates.Enabled)},
				{Name: "bbb", State: new(featuregates.Enabled)},
				{Name: "ccc", State: new(featuregates.Enabled)},
			}

			fgs.Disable("bbb")

			Expect(fgs).To(HaveLen(3))
			Expect(fgs).To(ContainElement(Equal(featuregates.FeatureGate{Name: "aaa", State: new(featuregates.Enabled)})))
			Expect(fgs).To(ContainElement(Equal(featuregates.FeatureGate{Name: "bbb", State: new(featuregates.Disabled)})))
			Expect(fgs).To(ContainElement(Equal(featuregates.FeatureGate{Name: "ccc", State: new(featuregates.Enabled)})))
		})

		It("should update if already exist - last item", func() {
			fgs := featuregates.HyperConvergedFeatureGates{
				{Name: "aaa", State: new(featuregates.Enabled)},
				{Name: "bbb", State: new(featuregates.Enabled)},
				{Name: "ccc", State: new(featuregates.Enabled)},
			}

			fgs.Disable("ccc")

			Expect(fgs).To(HaveLen(3))
			Expect(fgs).To(ContainElement(Equal(featuregates.FeatureGate{Name: "aaa", State: new(featuregates.Enabled)})))
			Expect(fgs).To(ContainElement(Equal(featuregates.FeatureGate{Name: "bbb", State: new(featuregates.Enabled)})))
			Expect(fgs).To(ContainElement(Equal(featuregates.FeatureGate{Name: "ccc", State: new(featuregates.Disabled)})))
		})
	})

	Context("check IsExplicitlyEnabled", func() {
		It("should not find if the list is nil", func() {
			var fgs featuregates.HyperConvergedFeatureGates

			enabled, found := fgs.IsExplicitlyEnabled("aFeatureGateName")
			Expect(enabled).To(BeFalse())
			Expect(found).To(BeFalse())
		})

		It("should not find if the list is empty", func() {
			fgs := featuregates.HyperConvergedFeatureGates{}

			enabled, found := fgs.IsExplicitlyEnabled("aFeatureGateName")
			Expect(enabled).To(BeFalse())
			Expect(found).To(BeFalse())
		})

		DescribeTable("should", func(fgName string, expectedEnabled, expectedFound bool) {
			fgs := featuregates.HyperConvergedFeatureGates{
				{Name: "implicitlyEnabled"},
				{Name: "explicitlyEnabled", State: new(featuregates.Enabled)},
				{Name: "disabled", State: new(featuregates.Disabled)},
			}

			enabled, found := fgs.IsExplicitlyEnabled(fgName)
			Expect(enabled).To(Equal(expectedEnabled))
			Expect(found).To(Equal(expectedFound))
		},
			Entry("not find if the fg is not in the list", "notFoundFG", false, false),
			Entry("find if the fg is implicitly enabled", "implicitlyEnabled", true, true),
			Entry("find if the fg is explicitly enabled", "explicitlyEnabled", true, true),
			Entry("find if the fg is disabled", "disabled", false, true),
		)
	})
})
