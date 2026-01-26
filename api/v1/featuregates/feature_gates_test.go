package featuregates_test

import (
	"encoding/json"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gtypes "github.com/onsi/gomega/types"
	"k8s.io/utils/ptr"
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
				State: ptr.To(featuregates.Enabled),
			}

			jsonBytes, err := json.Marshal(fg)
			Expect(err).ToNot(HaveOccurred())
			Expect(jsonBytes).To(MatchJSON(`{"name":"fgName"}`))
		})

		It("should marshal a disabled feature gate", func() {
			fg := featuregates.FeatureGate{
				Name:  "fgName",
				State: ptr.To(featuregates.Disabled),
			}

			jsonBytes, err := json.Marshal(fg)
			Expect(err).ToNot(HaveOccurred())
			Expect(jsonBytes).To(MatchJSON(`{"name":"fgName","state":"Disabled"}`))
		})

		It("should marshal an enabled feature gate pointer", func() {
			fg := &featuregates.FeatureGate{
				Name:  "fgName",
				State: ptr.To(featuregates.Enabled),
			}

			jsonBytes, err := json.Marshal(fg)
			Expect(err).ToNot(HaveOccurred())
			Expect(jsonBytes).To(MatchJSON(`{"name":"fgName"}`))
		})

		It("should marshal a disabled feature gate pointer", func() {
			fg := &featuregates.FeatureGate{
				Name:  "fgName",
				State: ptr.To(featuregates.Disabled),
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
					State: ptr.To(featuregates.Enabled),
				},
				{
					Name:  "disabledFG",
					State: ptr.To(featuregates.Disabled),
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
					State: ptr.To(featuregates.Enabled),
				},
				{
					Name:  "disabledFG",
					State: ptr.To(featuregates.Disabled),
				},
			}

			yamlBytes, err := yaml.Marshal(fgs)
			Expect(err).ToNot(HaveOccurred())
			Expect(yamlBytes).To(MatchYAML(`- name: noEnabledField
- name: enabledFG
- name: disabledFG
  state: "Disabled"`,
			))
		})
	})

	Context("Unmarshal", func() {
		It("should unmarshal an enabled feature gate", func() {
			fgBytes := []byte(`{"name":"fgName", "state":"Enabled"}`)
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
				featuregates.FeatureGate{Name: "noEnabledField", State: ptr.To(featuregates.Enabled)},
				featuregates.FeatureGate{Name: "enabledFG", State: ptr.To(featuregates.Enabled)},
				featuregates.FeatureGate{Name: "disabledFG", State: ptr.To(featuregates.Disabled)},
			))
		})

		It("should unmarshal a yaml array of FGs", func() {
			fgBytes := []byte(`- name: noEnabledField
- name: enabledFG
  enabled: "Enabled"
- name: disabledFG
  state: "Disabled"`,
			)
			fgs := featuregates.HyperConvergedFeatureGates{}

			Expect(yaml.Unmarshal(fgBytes, &fgs)).To(Succeed())
			Expect(fgs).To(HaveLen(3))
			Expect(fgs).To(ContainElements(
				featuregates.FeatureGate{Name: "noEnabledField", State: ptr.To(featuregates.Enabled)},
				featuregates.FeatureGate{Name: "enabledFG", State: ptr.To(featuregates.Enabled)},
				featuregates.FeatureGate{Name: "disabledFG", State: ptr.To(featuregates.Disabled)},
			))
		})
	})
})

var _ = Describe("Feature Gates", func() {
	DescribeTable("check IsEnabled", func(fgs featuregates.HyperConvergedFeatureGates, fgName string, expected gtypes.GomegaMatcher) {
		Expect(fgs.IsEnabled(fgName)).To(expected)
	},
		Entry("unknown FG; in list", featuregates.HyperConvergedFeatureGates{{Name: "unknown", State: ptr.To(featuregates.Enabled)}}, "unknown", BeFalse()),
		Entry("unknown FG; not in list", featuregates.HyperConvergedFeatureGates{{Name: "deployKubeSecondaryDNS", State: ptr.To(featuregates.Enabled)}}, "unknown", BeFalse()),

		Entry("known alpha FG; in list; enabled", featuregates.HyperConvergedFeatureGates{{Name: "downwardMetrics", State: ptr.To(featuregates.Enabled)}}, "downwardMetrics", BeTrue()),
		Entry("known alpha FG; in list; disabled", featuregates.HyperConvergedFeatureGates{{Name: "downwardMetrics", State: ptr.To(featuregates.Disabled)}}, "downwardMetrics", BeFalse()),
		Entry("known alpha FG; not in list; disabled", featuregates.HyperConvergedFeatureGates{{Name: "deployKubeSecondaryDNS", State: ptr.To(featuregates.Enabled)}}, "downwardMetrics", BeFalse()),

		Entry("known beta FG; in list; enabled", featuregates.HyperConvergedFeatureGates{{Name: "videoConfig", State: ptr.To(featuregates.Enabled)}}, "videoConfig", BeTrue()),
		Entry("known beta FG; in list; disabled", featuregates.HyperConvergedFeatureGates{{Name: "videoConfig", State: ptr.To(featuregates.Disabled)}}, "videoConfig", BeFalse()),
		Entry("known beta FG; not in list; disabled", featuregates.HyperConvergedFeatureGates{{Name: "deployKubeSecondaryDNS", State: ptr.To(featuregates.Enabled)}}, "videoConfig", BeTrue()),

		Entry("known deprecated FG; in list; enabled", featuregates.HyperConvergedFeatureGates{{Name: "withHostPassthroughCPU", State: ptr.To(featuregates.Enabled)}}, "withHostPassthroughCPU", BeFalse()),
		Entry("known deprecated FG; in list; disabled", featuregates.HyperConvergedFeatureGates{{Name: "withHostPassthroughCPU", State: ptr.To(featuregates.Disabled)}}, "withHostPassthroughCPU", BeFalse()),
		Entry("known deprecated FG; not in list; disabled", featuregates.HyperConvergedFeatureGates{{Name: "deployKubeSecondaryDNS", State: ptr.To(featuregates.Enabled)}}, "withHostPassthroughCPU", BeFalse()),
	)

	Context("check Add", func() {
		It("should add to nil", func() {
			var fgs featuregates.HyperConvergedFeatureGates

			fgs.Add(featuregates.FeatureGate{Name: "aaa", State: ptr.To(featuregates.Enabled)})

			Expect(fgs).To(HaveLen(1))
			Expect(fgs).To(ContainElement(Equal(featuregates.FeatureGate{Name: "aaa", State: ptr.To(featuregates.Enabled)})))
		})

		It("should add to non-empty", func() {
			fgs := featuregates.HyperConvergedFeatureGates{
				{Name: "aaa", State: ptr.To(featuregates.Enabled)},
			}

			fgs.Add(featuregates.FeatureGate{Name: "bbb", State: ptr.To(featuregates.Disabled)})

			Expect(fgs).To(HaveLen(2))
			Expect(fgs).To(ContainElement(Equal(featuregates.FeatureGate{Name: "aaa", State: ptr.To(featuregates.Enabled)})))
			Expect(fgs).To(ContainElement(Equal(featuregates.FeatureGate{Name: "bbb", State: ptr.To(featuregates.Disabled)})))
		})

		It("should update if already exist - first item", func() {
			fgs := featuregates.HyperConvergedFeatureGates{
				{Name: "aaa", State: ptr.To(featuregates.Enabled)},
				{Name: "bbb", State: ptr.To(featuregates.Enabled)},
				{Name: "ccc", State: ptr.To(featuregates.Enabled)},
			}

			fgs.Add(featuregates.FeatureGate{Name: "aaa", State: ptr.To(featuregates.Disabled)})

			Expect(fgs).To(HaveLen(3))
			Expect(fgs).To(ContainElement(Equal(featuregates.FeatureGate{Name: "aaa", State: ptr.To(featuregates.Disabled)})))
			Expect(fgs).To(ContainElement(Equal(featuregates.FeatureGate{Name: "bbb", State: ptr.To(featuregates.Enabled)})))
			Expect(fgs).To(ContainElement(Equal(featuregates.FeatureGate{Name: "ccc", State: ptr.To(featuregates.Enabled)})))
		})

		It("should update if already exist - middle item", func() {
			fgs := featuregates.HyperConvergedFeatureGates{
				{Name: "aaa", State: ptr.To(featuregates.Enabled)},
				{Name: "bbb", State: ptr.To(featuregates.Enabled)},
				{Name: "ccc", State: ptr.To(featuregates.Enabled)},
			}

			fgs.Add(featuregates.FeatureGate{Name: "bbb", State: ptr.To(featuregates.Disabled)})

			Expect(fgs).To(HaveLen(3))
			Expect(fgs).To(ContainElement(Equal(featuregates.FeatureGate{Name: "aaa", State: ptr.To(featuregates.Enabled)})))
			Expect(fgs).To(ContainElement(Equal(featuregates.FeatureGate{Name: "bbb", State: ptr.To(featuregates.Disabled)})))
			Expect(fgs).To(ContainElement(Equal(featuregates.FeatureGate{Name: "ccc", State: ptr.To(featuregates.Enabled)})))
		})

		It("should update if already exist - last item", func() {
			fgs := featuregates.HyperConvergedFeatureGates{
				{Name: "aaa", State: ptr.To(featuregates.Enabled)},
				{Name: "bbb", State: ptr.To(featuregates.Enabled)},
				{Name: "ccc", State: ptr.To(featuregates.Enabled)},
			}

			fgs.Add(featuregates.FeatureGate{Name: "ccc", State: ptr.To(featuregates.Disabled)})

			Expect(fgs).To(HaveLen(3))
			Expect(fgs).To(ContainElement(Equal(featuregates.FeatureGate{Name: "aaa", State: ptr.To(featuregates.Enabled)})))
			Expect(fgs).To(ContainElement(Equal(featuregates.FeatureGate{Name: "bbb", State: ptr.To(featuregates.Enabled)})))
			Expect(fgs).To(ContainElement(Equal(featuregates.FeatureGate{Name: "ccc", State: ptr.To(featuregates.Disabled)})))
		})
	})

	Context("check Enable", func() {
		It("should add to nil", func() {
			var fgs featuregates.HyperConvergedFeatureGates

			fgs.Enable("aaa")

			Expect(fgs).To(HaveLen(1))
			Expect(fgs).To(ContainElement(Equal(featuregates.FeatureGate{Name: "aaa", State: ptr.To(featuregates.Enabled)})))
		})

		It("should add to non-empty", func() {
			fgs := featuregates.HyperConvergedFeatureGates{
				{Name: "aaa", State: ptr.To(featuregates.Enabled)},
			}

			fgs.Enable("bbb")

			Expect(fgs).To(HaveLen(2))
			Expect(fgs).To(ContainElement(Equal(featuregates.FeatureGate{Name: "aaa", State: ptr.To(featuregates.Enabled)})))
			Expect(fgs).To(ContainElement(Equal(featuregates.FeatureGate{Name: "bbb", State: ptr.To(featuregates.Enabled)})))
		})

		It("should update if already exist - first item", func() {
			fgs := featuregates.HyperConvergedFeatureGates{
				{Name: "aaa", State: ptr.To(featuregates.Disabled)},
				{Name: "bbb", State: ptr.To(featuregates.Disabled)},
				{Name: "ccc", State: ptr.To(featuregates.Disabled)},
			}

			fgs.Enable("aaa")

			Expect(fgs).To(HaveLen(3))
			Expect(fgs).To(ContainElement(Equal(featuregates.FeatureGate{Name: "aaa", State: ptr.To(featuregates.Enabled)})))
			Expect(fgs).To(ContainElement(Equal(featuregates.FeatureGate{Name: "bbb", State: ptr.To(featuregates.Disabled)})))
			Expect(fgs).To(ContainElement(Equal(featuregates.FeatureGate{Name: "ccc", State: ptr.To(featuregates.Disabled)})))
		})

		It("should update if already exist - middle item", func() {
			fgs := featuregates.HyperConvergedFeatureGates{
				{Name: "aaa", State: ptr.To(featuregates.Disabled)},
				{Name: "bbb", State: ptr.To(featuregates.Disabled)},
				{Name: "ccc", State: ptr.To(featuregates.Disabled)},
			}

			fgs.Enable("bbb")

			Expect(fgs).To(HaveLen(3))
			Expect(fgs).To(ContainElement(Equal(featuregates.FeatureGate{Name: "aaa", State: ptr.To(featuregates.Disabled)})))
			Expect(fgs).To(ContainElement(Equal(featuregates.FeatureGate{Name: "bbb", State: ptr.To(featuregates.Enabled)})))
			Expect(fgs).To(ContainElement(Equal(featuregates.FeatureGate{Name: "ccc", State: ptr.To(featuregates.Disabled)})))
		})

		It("should update if already exist - last item", func() {
			fgs := featuregates.HyperConvergedFeatureGates{
				{Name: "aaa", State: ptr.To(featuregates.Disabled)},
				{Name: "bbb", State: ptr.To(featuregates.Disabled)},
				{Name: "ccc", State: ptr.To(featuregates.Disabled)},
			}

			fgs.Enable("ccc")

			Expect(fgs).To(HaveLen(3))
			Expect(fgs).To(ContainElement(Equal(featuregates.FeatureGate{Name: "aaa", State: ptr.To(featuregates.Disabled)})))
			Expect(fgs).To(ContainElement(Equal(featuregates.FeatureGate{Name: "bbb", State: ptr.To(featuregates.Disabled)})))
			Expect(fgs).To(ContainElement(Equal(featuregates.FeatureGate{Name: "ccc", State: ptr.To(featuregates.Enabled)})))
		})
	})

	Context("check Disable", func() {
		It("should add to nil", func() {
			var fgs featuregates.HyperConvergedFeatureGates

			fgs.Disable("aaa")

			Expect(fgs).To(HaveLen(1))
			Expect(fgs).To(ContainElement(Equal(featuregates.FeatureGate{Name: "aaa", State: ptr.To(featuregates.Disabled)})))
		})

		It("should add to non-empty", func() {
			fgs := featuregates.HyperConvergedFeatureGates{
				{Name: "aaa", State: ptr.To(featuregates.Enabled)},
			}

			fgs.Disable("bbb")

			Expect(fgs).To(HaveLen(2))
			Expect(fgs).To(ContainElement(Equal(featuregates.FeatureGate{Name: "aaa", State: ptr.To(featuregates.Enabled)})))
			Expect(fgs).To(ContainElement(Equal(featuregates.FeatureGate{Name: "bbb", State: ptr.To(featuregates.Disabled)})))
		})

		It("should update if already exist - first item", func() {
			fgs := featuregates.HyperConvergedFeatureGates{
				{Name: "aaa", State: ptr.To(featuregates.Enabled)},
				{Name: "bbb", State: ptr.To(featuregates.Enabled)},
				{Name: "ccc", State: ptr.To(featuregates.Enabled)},
			}

			fgs.Disable("aaa")

			Expect(fgs).To(HaveLen(3))
			Expect(fgs).To(ContainElement(Equal(featuregates.FeatureGate{Name: "aaa", State: ptr.To(featuregates.Disabled)})))
			Expect(fgs).To(ContainElement(Equal(featuregates.FeatureGate{Name: "bbb", State: ptr.To(featuregates.Enabled)})))
			Expect(fgs).To(ContainElement(Equal(featuregates.FeatureGate{Name: "ccc", State: ptr.To(featuregates.Enabled)})))
		})

		It("should update if already exist - middle item", func() {
			fgs := featuregates.HyperConvergedFeatureGates{
				{Name: "aaa", State: ptr.To(featuregates.Enabled)},
				{Name: "bbb", State: ptr.To(featuregates.Enabled)},
				{Name: "ccc", State: ptr.To(featuregates.Enabled)},
			}

			fgs.Disable("bbb")

			Expect(fgs).To(HaveLen(3))
			Expect(fgs).To(ContainElement(Equal(featuregates.FeatureGate{Name: "aaa", State: ptr.To(featuregates.Enabled)})))
			Expect(fgs).To(ContainElement(Equal(featuregates.FeatureGate{Name: "bbb", State: ptr.To(featuregates.Disabled)})))
			Expect(fgs).To(ContainElement(Equal(featuregates.FeatureGate{Name: "ccc", State: ptr.To(featuregates.Enabled)})))
		})

		It("should update if already exist - last item", func() {
			fgs := featuregates.HyperConvergedFeatureGates{
				{Name: "aaa", State: ptr.To(featuregates.Enabled)},
				{Name: "bbb", State: ptr.To(featuregates.Enabled)},
				{Name: "ccc", State: ptr.To(featuregates.Enabled)},
			}

			fgs.Disable("ccc")

			Expect(fgs).To(HaveLen(3))
			Expect(fgs).To(ContainElement(Equal(featuregates.FeatureGate{Name: "aaa", State: ptr.To(featuregates.Enabled)})))
			Expect(fgs).To(ContainElement(Equal(featuregates.FeatureGate{Name: "bbb", State: ptr.To(featuregates.Enabled)})))
			Expect(fgs).To(ContainElement(Equal(featuregates.FeatureGate{Name: "ccc", State: ptr.To(featuregates.Disabled)})))
		})
	})
})
