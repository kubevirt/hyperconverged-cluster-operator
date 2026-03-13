package v1beta1

import (
	"slices"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	hcofg "github.com/kubevirt/hyperconverged-cluster-operator/api/v1/featuregates"
)

func TestV1Beta1(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "v1beta1 Suite")
}

var _ = Describe("api/v1beta1", func() {
	Context("Conversion", func() {
		It("should allow v1beta1 => v1 conversion", func() {
			v1beta1hco := getV1Beta1HC()

			Expect(v1beta1hco.ConvertTo(&hcov1.HyperConverged{})).To(Succeed())
		})

		It("should allow v1 => v1beta1 conversion", func() {
			v1hco := getV1HC()

			Expect((&HyperConverged{}).ConvertFrom(v1hco)).To(Succeed())
		})
	})

	Context("Feature Gates conversion", func() {
		Context("v1beta1 to v1", func() {
			It("should enable alpha feature gate in v1 when set to true in v1beta1", func() {
				in := &HyperConvergedFeatureGates{
					DownwardMetrics: ptr.To(true),
				}
				out := &hcofg.HyperConvergedFeatureGates{}

				convert_v1beta1_FeatureGates_To_v1(in, out)

				idx := slices.IndexFunc(*out, func(fg hcofg.FeatureGate) bool {
					return fg.Name == "downwardMetrics"
				})
				Expect(idx).ToNot(Equal(-1))
				Expect(*(*out)[idx].State).To(Equal(hcofg.Enabled))
			})

			It("should not add alpha feature gate to v1 when set to false in v1beta1", func() {
				in := &HyperConvergedFeatureGates{
					DownwardMetrics: ptr.To(false),
				}
				out := &hcofg.HyperConvergedFeatureGates{}

				convert_v1beta1_FeatureGates_To_v1(in, out)

				Expect(*out).To(BeEmpty())
			})

			It("should not add alpha feature gate to v1 when nil in v1beta1", func() {
				in := &HyperConvergedFeatureGates{}
				out := &hcofg.HyperConvergedFeatureGates{}

				convert_v1beta1_FeatureGates_To_v1(in, out)

				Expect(*out).To(BeEmpty())
			})

			It("should disable beta feature gate in v1 when set to false in v1beta1", func() {
				in := &HyperConvergedFeatureGates{
					DecentralizedLiveMigration: ptr.To(false),
				}
				out := &hcofg.HyperConvergedFeatureGates{}

				convert_v1beta1_FeatureGates_To_v1(in, out)

				idx := slices.IndexFunc(*out, func(fg hcofg.FeatureGate) bool {
					return fg.Name == "decentralizedLiveMigration"
				})
				Expect(idx).ToNot(Equal(-1))
				Expect(*(*out)[idx].State).To(Equal(hcofg.Disabled))
			})

			It("should not add beta feature gate to v1 when set to true in v1beta1", func() {
				in := &HyperConvergedFeatureGates{
					DecentralizedLiveMigration: ptr.To(true),
				}
				out := &hcofg.HyperConvergedFeatureGates{}

				convert_v1beta1_FeatureGates_To_v1(in, out)

				Expect(*out).To(BeEmpty())
			})

			It("should not add beta feature gate to v1 when nil in v1beta1", func() {
				in := &HyperConvergedFeatureGates{
					DecentralizedLiveMigration: nil,
				}
				out := &hcofg.HyperConvergedFeatureGates{}

				convert_v1beta1_FeatureGates_To_v1(in, out)

				Expect(*out).To(BeEmpty())
			})

			It("should ignore deprecated feature gates", func() {
				in := &HyperConvergedFeatureGates{
					WithHostPassthroughCPU:      ptr.To(true),
					EnableCommonBootImageImport: ptr.To(true),
					DeployTektonTaskResources:   ptr.To(true),
				}
				out := &hcofg.HyperConvergedFeatureGates{}

				convert_v1beta1_FeatureGates_To_v1(in, out)

				Expect(*out).To(BeEmpty())
			})

			It("should convert multiple feature gates at once", func() {
				in := &HyperConvergedFeatureGates{
					DownwardMetrics:            ptr.To(true),
					AlignCPUs:                  ptr.To(true),
					DecentralizedLiveMigration: ptr.To(false),
					VideoConfig:                ptr.To(false),
					ObjectGraph:                ptr.To(false), // alpha default, should not appear
				}
				out := &hcofg.HyperConvergedFeatureGates{}

				convert_v1beta1_FeatureGates_To_v1(in, out)

				Expect(*out).To(HaveLen(4))
			})
		})

		Context("v1 to v1beta1", func() {
			It("should set alpha feature gate to true when enabled in v1", func() {
				in := hcofg.HyperConvergedFeatureGates{}
				in.Enable("downwardMetrics")
				out := &HyperConvergedFeatureGates{}

				convert_v1_FeatureGates_To_v1beta1(in, out)

				Expect(out.DownwardMetrics).ToNot(BeNil())
				Expect(*out.DownwardMetrics).To(BeTrue())
			})

			It("should set alpha feature gate to false when not present in v1", func() {
				in := hcofg.HyperConvergedFeatureGates{}
				out := &HyperConvergedFeatureGates{}

				convert_v1_FeatureGates_To_v1beta1(in, out)

				Expect(out.DownwardMetrics).ToNot(BeNil())
				Expect(*out.DownwardMetrics).To(BeFalse())
			})

			It("should set beta feature gate to true when not explicitly disabled in v1", func() {
				in := hcofg.HyperConvergedFeatureGates{}
				out := &HyperConvergedFeatureGates{}

				convert_v1_FeatureGates_To_v1beta1(in, out)

				Expect(out.DecentralizedLiveMigration).ToNot(BeNil())
				Expect(*out.DecentralizedLiveMigration).To(BeTrue())
			})

			It("should set beta feature gate to false when disabled in v1", func() {
				in := hcofg.HyperConvergedFeatureGates{}
				in.Disable("decentralizedLiveMigration")
				out := &HyperConvergedFeatureGates{}

				convert_v1_FeatureGates_To_v1beta1(in, out)

				Expect(out.DecentralizedLiveMigration).ToNot(BeNil())
				Expect(*out.DecentralizedLiveMigration).To(BeFalse())
			})

			It("should not set deprecated feature gates", func() {
				in := hcofg.HyperConvergedFeatureGates{}
				out := &HyperConvergedFeatureGates{}

				convert_v1_FeatureGates_To_v1beta1(in, out)

				Expect(out.WithHostPassthroughCPU).To(BeNil())
				Expect(out.EnableCommonBootImageImport).To(BeNil())
				Expect(out.DeployTektonTaskResources).To(BeNil())
				Expect(out.NonRoot).To(BeNil())
			})
		})

		Context("round-trip", func() {
			It("should preserve alpha feature gate enabled through round-trip", func() {
				original := &HyperConvergedFeatureGates{
					DownwardMetrics: ptr.To(true),
					AlignCPUs:       ptr.To(true),
				}

				v1fgs := &hcofg.HyperConvergedFeatureGates{}
				convert_v1beta1_FeatureGates_To_v1(original, v1fgs)

				result := &HyperConvergedFeatureGates{}
				convert_v1_FeatureGates_To_v1beta1(*v1fgs, result)

				Expect(*result.DownwardMetrics).To(BeTrue())
				Expect(*result.AlignCPUs).To(BeTrue())
			})

			It("should preserve beta feature gate disabled through round-trip", func() {
				original := &HyperConvergedFeatureGates{
					DecentralizedLiveMigration: ptr.To(false),
					VideoConfig:                ptr.To(false),
				}

				v1fgs := &hcofg.HyperConvergedFeatureGates{}
				convert_v1beta1_FeatureGates_To_v1(original, v1fgs)

				result := &HyperConvergedFeatureGates{}
				convert_v1_FeatureGates_To_v1beta1(*v1fgs, result)

				Expect(*result.DecentralizedLiveMigration).To(BeFalse())
				Expect(*result.VideoConfig).To(BeFalse())
			})

			It("should preserve defaults through round-trip", func() {
				original := &HyperConvergedFeatureGates{}

				v1fgs := &hcofg.HyperConvergedFeatureGates{}
				convert_v1beta1_FeatureGates_To_v1(original, v1fgs)

				result := &HyperConvergedFeatureGates{}
				convert_v1_FeatureGates_To_v1beta1(*v1fgs, result)

				// alpha defaults stay false
				Expect(*result.DownwardMetrics).To(BeFalse())
				Expect(*result.AlignCPUs).To(BeFalse())
				// beta defaults stay true
				Expect(*result.DecentralizedLiveMigration).To(BeTrue())
				Expect(*result.VideoConfig).To(BeTrue())
			})
		})
	})
})

func getV1HC() *hcov1.HyperConverged {
	GinkgoHelper()

	defaultScheme := runtime.NewScheme()
	Expect(hcov1.AddToScheme(defaultScheme)).To(Succeed())
	Expect(hcov1.RegisterDefaults(defaultScheme)).To(Succeed())
	defaultHco := &hcov1.HyperConverged{
		TypeMeta: metav1.TypeMeta{
			APIVersion: hcov1.APIVersion,
			Kind:       "HyperConverged",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubevirt-hyperconverged",
			Namespace: "kubevirt-hyperconverged",
		}}
	defaultScheme.Default(defaultHco)
	return defaultHco
}

func getV1Beta1HC() *HyperConverged {
	GinkgoHelper()

	defaultScheme := runtime.NewScheme()
	Expect(AddToScheme(defaultScheme)).To(Succeed())
	Expect(RegisterDefaults(defaultScheme)).To(Succeed())
	defaultHco := &HyperConverged{
		TypeMeta: metav1.TypeMeta{
			APIVersion: APIVersion,
			Kind:       "HyperConverged",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubevirt-hyperconverged",
			Namespace: "kubevirt-hyperconverged",
		}}
	defaultScheme.Default(defaultHco)
	return defaultHco
}
