package v1beta1

import (
	"fmt"
	"slices"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	openshiftconfigv1 "github.com/openshift/api/config/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	kubevirtv1 "kubevirt.io/api/core/v1"
	cdiv1beta1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	sdkapi "kubevirt.io/controller-lifecycle-operator-sdk/api"

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

	Context("NodePlacements conversion", func() {
		var (
			infraAffinity = &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "infra-node",
										Operator: corev1.NodeSelectorOpExists,
									},
								},
							},
						},
					},
				},
				PodAntiAffinity: &corev1.PodAntiAffinity{
					PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
						{
							Weight: 100,
							PodAffinityTerm: corev1.PodAffinityTerm{
								TopologyKey: "kubernetes.io/hostname",
							},
						},
					},
				},
			}

			workloadAffinity = &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "workload-node",
										Operator: corev1.NodeSelectorOpExists,
									},
								},
							},
						},
					},
				},
			}
		)

		Context("v1beta1 to v1", func() {
			It("should convert both infra and workload node placements", func() {
				v1beta1Spec := HyperConvergedSpec{
					Infra: HyperConvergedConfig{
						NodePlacement: &sdkapi.NodePlacement{
							NodeSelector: map[string]string{"infra-key": "infra-val"},
						},
					},
					Workloads: HyperConvergedConfig{
						NodePlacement: &sdkapi.NodePlacement{
							NodeSelector: map[string]string{"workload-key": "workload-val"},
						},
					},
				}
				v1Spec := &hcov1.HyperConvergedSpec{}

				convertNodePlacementsV1beta1ToV1(v1beta1Spec, v1Spec)

				nodePlacements := v1Spec.Deployment.NodePlacements

				Expect(nodePlacements).ToNot(BeNil())
				Expect(nodePlacements.Infra).ToNot(BeNil())
				Expect(nodePlacements.Infra.NodeSelector).To(Equal(map[string]string{"infra-key": "infra-val"}))
				Expect(nodePlacements.Workload).ToNot(BeNil())
				Expect(nodePlacements.Workload.NodeSelector).To(Equal(map[string]string{"workload-key": "workload-val"}))
			})

			It("should convert only infra node placement when workload is nil", func() {
				v1beta1Spec := HyperConvergedSpec{
					Infra: HyperConvergedConfig{
						NodePlacement: &sdkapi.NodePlacement{
							NodeSelector: map[string]string{"infra-key": "infra-val"},
						},
					},
				}
				v1Spec := &hcov1.HyperConvergedSpec{}

				convertNodePlacementsV1beta1ToV1(v1beta1Spec, v1Spec)

				Expect(v1Spec.Deployment.NodePlacements).ToNot(BeNil())
				Expect(v1Spec.Deployment.NodePlacements.Infra).ToNot(BeNil())
				Expect(v1Spec.Deployment.NodePlacements.Workload).To(BeNil())
			})

			It("should convert only workload node placement when infra is nil", func() {
				v1beta1Spec := HyperConvergedSpec{
					Workloads: HyperConvergedConfig{
						NodePlacement: &sdkapi.NodePlacement{
							NodeSelector: map[string]string{"workload-key": "workload-val"},
						},
					},
				}
				v1Spec := &hcov1.HyperConvergedSpec{}

				convertNodePlacementsV1beta1ToV1(v1beta1Spec, v1Spec)

				Expect(v1Spec.Deployment.NodePlacements).ToNot(BeNil())
				Expect(v1Spec.Deployment.NodePlacements.Infra).To(BeNil())
				Expect(v1Spec.Deployment.NodePlacements.Workload).ToNot(BeNil())
			})

			It("should not set NodePlacements when both are nil", func() {
				v1beta1Spec := HyperConvergedSpec{}
				v1Spec := &hcov1.HyperConvergedSpec{}

				convertNodePlacementsV1beta1ToV1(v1beta1Spec, v1Spec)

				Expect(v1Spec.Deployment.NodePlacements).To(BeNil())
			})

			It("should convert affinity and anti-affinity", func() {
				v1beta1Spec := HyperConvergedSpec{
					Infra: HyperConvergedConfig{
						NodePlacement: &sdkapi.NodePlacement{
							Affinity: infraAffinity,
						},
					},
					Workloads: HyperConvergedConfig{
						NodePlacement: &sdkapi.NodePlacement{
							Affinity: workloadAffinity,
						},
					},
				}
				v1Spec := &hcov1.HyperConvergedSpec{}

				convertNodePlacementsV1beta1ToV1(v1beta1Spec, v1Spec)
				Expect(v1Spec.Deployment.NodePlacements).ToNot(BeNil())
				Expect(v1Spec.Deployment.NodePlacements.Infra).ToNot(BeNil())
				Expect(v1Spec.Deployment.NodePlacements.Infra.Affinity).To(Equal(infraAffinity))

				Expect(v1Spec.Deployment.NodePlacements.Workload).ToNot(BeNil())
				Expect(v1Spec.Deployment.NodePlacements.Workload.Affinity).To(Equal(workloadAffinity))
			})
		})

		Context("v1 to v1beta1", func() {
			It("should convert both infra and workload node placements", func() {
				v1Spec := hcov1.HyperConvergedSpec{
					Deployment: hcov1.DeploymentConfig{
						NodePlacements: &hcov1.NodePlacements{
							Infra: &sdkapi.NodePlacement{
								NodeSelector: map[string]string{"infra-key": "infra-val"},
							},
							Workload: &sdkapi.NodePlacement{
								NodeSelector: map[string]string{"workload-key": "workload-val"},
							},
						},
					},
				}
				v1beta1Spec := &HyperConvergedSpec{}

				convertNodePlacementsV1ToV1beta1(v1Spec, v1beta1Spec)

				Expect(v1beta1Spec.Infra.NodePlacement).ToNot(BeNil())
				Expect(v1beta1Spec.Infra.NodePlacement.NodeSelector).To(Equal(map[string]string{"infra-key": "infra-val"}))
				Expect(v1beta1Spec.Workloads.NodePlacement).ToNot(BeNil())
				Expect(v1beta1Spec.Workloads.NodePlacement.NodeSelector).To(Equal(map[string]string{"workload-key": "workload-val"}))
			})

			It("should convert only infra when workload is nil", func() {
				v1Spec := hcov1.HyperConvergedSpec{
					Deployment: hcov1.DeploymentConfig{
						NodePlacements: &hcov1.NodePlacements{
							Infra: &sdkapi.NodePlacement{
								NodeSelector: map[string]string{"infra-key": "infra-val"},
							},
						},
					},
				}
				v1beta1Spec := &HyperConvergedSpec{}

				convertNodePlacementsV1ToV1beta1(v1Spec, v1beta1Spec)

				Expect(v1beta1Spec.Infra.NodePlacement).ToNot(BeNil())
				Expect(v1beta1Spec.Workloads.NodePlacement).To(BeNil())
			})

			It("should not modify v1beta1 when NodePlacements is nil", func() {
				v1Spec := hcov1.HyperConvergedSpec{}
				v1beta1Spec := &HyperConvergedSpec{}

				convertNodePlacementsV1ToV1beta1(v1Spec, v1beta1Spec)

				Expect(v1beta1Spec.Infra.NodePlacement).To(BeNil())
				Expect(v1beta1Spec.Workloads.NodePlacement).To(BeNil())
			})

			It("should convert affinity and anti-affinity", func() {
				v1Spec := hcov1.HyperConvergedSpec{
					Deployment: hcov1.DeploymentConfig{
						NodePlacements: &hcov1.NodePlacements{
							Infra: &sdkapi.NodePlacement{
								Affinity: infraAffinity,
							},
							Workload: &sdkapi.NodePlacement{
								Affinity: workloadAffinity,
							},
						},
					},
				}
				v1beta1Spec := &HyperConvergedSpec{}

				convertNodePlacementsV1ToV1beta1(v1Spec, v1beta1Spec)

				Expect(v1beta1Spec.Infra.NodePlacement).ToNot(BeNil())
				Expect(v1beta1Spec.Infra.NodePlacement.Affinity).To(Equal(infraAffinity))
				Expect(v1beta1Spec.Workloads.NodePlacement).ToNot(BeNil())
				Expect(v1beta1Spec.Workloads.NodePlacement.Affinity).To(Equal(workloadAffinity))
			})
		})

		Context("round-trip", func() {
			It("should preserve node placements through v1beta1 => v1 => v1beta1", func() {
				original := HyperConvergedSpec{
					Infra: HyperConvergedConfig{
						NodePlacement: &sdkapi.NodePlacement{
							NodeSelector: map[string]string{"infra-key": "infra-val"},
						},
					},
					Workloads: HyperConvergedConfig{
						NodePlacement: &sdkapi.NodePlacement{
							NodeSelector: map[string]string{"workload-key": "workload-val"},
						},
					},
				}

				v1Spec := &hcov1.HyperConvergedSpec{}
				convertNodePlacementsV1beta1ToV1(original, v1Spec)

				result := &HyperConvergedSpec{}
				convertNodePlacementsV1ToV1beta1(*v1Spec, result)

				Expect(result.Infra.NodePlacement.NodeSelector).To(Equal(original.Infra.NodePlacement.NodeSelector))
				Expect(result.Workloads.NodePlacement.NodeSelector).To(Equal(original.Workloads.NodePlacement.NodeSelector))
			})

			It("should preserve node placements through v1 => v1beta1 => v1", func() {
				original := hcov1.HyperConvergedSpec{
					Deployment: hcov1.DeploymentConfig{
						NodePlacements: &hcov1.NodePlacements{
							Infra: &sdkapi.NodePlacement{
								NodeSelector: map[string]string{"infra-key": "infra-val"},
							},
							Workload: &sdkapi.NodePlacement{
								NodeSelector: map[string]string{"workload-key": "workload-val"},
							},
						},
					},
				}

				v1beta1Spec := &HyperConvergedSpec{}
				convertNodePlacementsV1ToV1beta1(original, v1beta1Spec)

				result := &hcov1.HyperConvergedSpec{}
				convertNodePlacementsV1beta1ToV1(*v1beta1Spec, result)

				Expect(result.Deployment.NodePlacements.Infra.NodeSelector).To(Equal(original.Deployment.NodePlacements.Infra.NodeSelector))
				Expect(result.Deployment.NodePlacements.Workload.NodeSelector).To(Equal(original.Deployment.NodePlacements.Workload.NodeSelector))
			})
		})
	})

	Context("Feature Gates conversion", func() {
		Context("v1beta1 to v1", func() {
			It("should enable alpha feature gate in v1 when set to true in v1beta1", func() {
				in := &HyperConvergedFeatureGates{
					DownwardMetrics: new(true),
				}
				out := &hcofg.HyperConvergedFeatureGates{}

				convert_v1beta1_FeatureGates_To_v1(in, out)

				idx := slices.IndexFunc(*out, func(fg hcofg.FeatureGate) bool {
					return fg.Name == "downwardMetrics"
				})
				Expect(idx).ToNot(Equal(-1))
				Expect((*out)[idx].State).To(HaveValue(Equal(hcofg.Enabled)))
			})

			It("should not add alpha feature gate to v1 when set to false in v1beta1", func() {
				in := &HyperConvergedFeatureGates{
					DownwardMetrics: new(false),
				}
				out := &hcofg.HyperConvergedFeatureGates{}

				convert_v1beta1_FeatureGates_To_v1(in, out)

				Expect(out).To(HaveValue(BeEmpty()))
			})

			It("should not add alpha feature gate to v1 when nil in v1beta1", func() {
				in := &HyperConvergedFeatureGates{}
				out := &hcofg.HyperConvergedFeatureGates{}

				convert_v1beta1_FeatureGates_To_v1(in, out)

				Expect(out).To(HaveValue(BeEmpty()))
			})

			It("should disable beta feature gate in v1 when set to false in v1beta1", func() {
				in := &HyperConvergedFeatureGates{
					DecentralizedLiveMigration: new(false),
				}
				out := &hcofg.HyperConvergedFeatureGates{}

				convert_v1beta1_FeatureGates_To_v1(in, out)

				idx := slices.IndexFunc(*out, func(fg hcofg.FeatureGate) bool {
					return fg.Name == "decentralizedLiveMigration"
				})
				Expect(idx).ToNot(Equal(-1))
				Expect((*out)[idx].State).To(HaveValue(Equal(hcofg.Disabled)))
			})

			It("should not add beta feature gate to v1 when set to true in v1beta1", func() {
				in := &HyperConvergedFeatureGates{
					DecentralizedLiveMigration: new(true),
				}
				out := &hcofg.HyperConvergedFeatureGates{}

				convert_v1beta1_FeatureGates_To_v1(in, out)

				Expect(out).To(HaveValue(BeEmpty()))
			})

			It("should not add beta feature gate to v1 when nil in v1beta1", func() {
				in := &HyperConvergedFeatureGates{
					DecentralizedLiveMigration: nil,
				}
				out := &hcofg.HyperConvergedFeatureGates{}

				convert_v1beta1_FeatureGates_To_v1(in, out)

				Expect(out).To(HaveValue(BeEmpty()))
			})

			It("should ignore deprecated feature gates", func() {
				in := &HyperConvergedFeatureGates{
					WithHostPassthroughCPU:      new(true),
					EnableCommonBootImageImport: new(true),
					DeployTektonTaskResources:   new(true),
				}
				out := &hcofg.HyperConvergedFeatureGates{}

				convert_v1beta1_FeatureGates_To_v1(in, out)

				Expect(out).To(HaveValue(BeEmpty()))
			})

			It("should convert multiple feature gates at once", func() {
				in := &HyperConvergedFeatureGates{
					DownwardMetrics:            new(true),
					AlignCPUs:                  new(true),
					DecentralizedLiveMigration: new(false),
					DeclarativeHotplugVolumes:  new(false),
					ObjectGraph:                new(false), // alpha default, should not appear
				}
				out := &hcofg.HyperConvergedFeatureGates{}

				convert_v1beta1_FeatureGates_To_v1(in, out)

				Expect(out).To(HaveValue(HaveLen(4)))
			})
		})

		Context("v1 to v1beta1", func() {
			It("should set alpha feature gate to true when enabled in v1", func() {
				in := hcofg.HyperConvergedFeatureGates{}
				in.Enable("downwardMetrics")
				out := &HyperConvergedFeatureGates{}

				convert_v1_FeatureGates_To_v1beta1(in, out)

				Expect(out.DownwardMetrics).ToNot(BeNil())
				Expect(out.DownwardMetrics).To(HaveValue(BeTrue()))
			})

			It("should set alpha feature gate to false when not present in v1", func() {
				in := hcofg.HyperConvergedFeatureGates{}
				out := &HyperConvergedFeatureGates{}

				convert_v1_FeatureGates_To_v1beta1(in, out)

				Expect(out.DownwardMetrics).ToNot(BeNil())
				Expect(out.DownwardMetrics).To(HaveValue(BeFalse()))
			})

			It("should set beta feature gate to true when not explicitly disabled in v1", func() {
				in := hcofg.HyperConvergedFeatureGates{}
				out := &HyperConvergedFeatureGates{}

				convert_v1_FeatureGates_To_v1beta1(in, out)

				Expect(out.DecentralizedLiveMigration).ToNot(BeNil())
				Expect(out.DecentralizedLiveMigration).To(HaveValue(BeTrue()))
			})

			It("should set beta feature gate to false when disabled in v1", func() {
				in := hcofg.HyperConvergedFeatureGates{}
				in.Disable("decentralizedLiveMigration")
				out := &HyperConvergedFeatureGates{}

				convert_v1_FeatureGates_To_v1beta1(in, out)

				Expect(out.DecentralizedLiveMigration).ToNot(BeNil())
				Expect(out.DecentralizedLiveMigration).To(HaveValue(BeFalse()))
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
					DownwardMetrics: new(true),
					AlignCPUs:       new(true),
				}

				v1fgs := &hcofg.HyperConvergedFeatureGates{}
				convert_v1beta1_FeatureGates_To_v1(original, v1fgs)

				result := &HyperConvergedFeatureGates{}
				convert_v1_FeatureGates_To_v1beta1(*v1fgs, result)

				Expect(result.DownwardMetrics).To(HaveValue(BeTrue()))
				Expect(result.AlignCPUs).To(HaveValue(BeTrue()))
			})

			It("should preserve beta feature gate disabled through round-trip", func() {
				original := &HyperConvergedFeatureGates{
					DecentralizedLiveMigration: new(false),
					DeclarativeHotplugVolumes:  new(false),
				}

				v1fgs := &hcofg.HyperConvergedFeatureGates{}
				convert_v1beta1_FeatureGates_To_v1(original, v1fgs)

				result := &HyperConvergedFeatureGates{}
				convert_v1_FeatureGates_To_v1beta1(*v1fgs, result)

				Expect(result.DecentralizedLiveMigration).To(HaveValue(BeFalse()))
				Expect(result.DeclarativeHotplugVolumes).To(HaveValue(BeFalse()))
			})

			It("should preserve defaults through round-trip", func() {
				original := &HyperConvergedFeatureGates{}

				v1fgs := &hcofg.HyperConvergedFeatureGates{}
				convert_v1beta1_FeatureGates_To_v1(original, v1fgs)

				result := &HyperConvergedFeatureGates{}
				convert_v1_FeatureGates_To_v1beta1(*v1fgs, result)

				// alpha defaults stay false
				Expect(result.DownwardMetrics).To(HaveValue(BeFalse()))
				Expect(result.AlignCPUs).To(HaveValue(BeFalse()))
				// beta defaults stay true
				Expect(result.DecentralizedLiveMigration).To(HaveValue(BeTrue()))
				Expect(result.DeclarativeHotplugVolumes).To(HaveValue(BeTrue()))
			})
		})
	})

	Context("MDev enabled conversion", func() {
		Context("v1beta1 to v1", func() {
			It("should convert disableMDevConfiguration=true to enabled=false when enabled is unset", func() {
				v1beta1Spec := HyperConvergedSpec{
					FeatureGates: HyperConvergedFeatureGates{
						DisableMDevConfiguration: new(true),
					},
				}
				v1Spec := hcov1.HyperConvergedSpec{Virtualization: hcov1.VirtualizationConfig{}}

				convertMDevEnabledV1beta1ToV1(v1beta1Spec, &v1Spec)

				Expect(v1Spec.Virtualization.MediatedDevicesConfiguration).ToNot(BeNil())
				Expect(v1Spec.Virtualization.MediatedDevicesConfiguration.Enabled).To(HaveValue(BeFalse()))
			})

			It("should convert disableMDevConfiguration=false to enabled=true when enabled is unset", func() {
				v1beta1Spec := HyperConvergedSpec{
					FeatureGates: HyperConvergedFeatureGates{
						DisableMDevConfiguration: new(false),
					},
					MediatedDevicesConfiguration: &MediatedDevicesConfiguration{
						MediatedDeviceTypes: []string{"nvidia-222"},
					},
				}
				v1Spec := hcov1.HyperConvergedSpec{
					Virtualization: hcov1.VirtualizationConfig{
						MediatedDevicesConfiguration: &hcov1.MediatedDevicesConfiguration{
							MediatedDeviceTypes: []string{"nvidia-222"},
						},
					},
				}

				convertMDevEnabledV1beta1ToV1(v1beta1Spec, &v1Spec)

				Expect(v1Spec.Virtualization.MediatedDevicesConfiguration.Enabled).To(HaveValue(BeTrue()))
			})

			It("should override enabled=true when the FG is true", func() {
				v1beta1Spec := HyperConvergedSpec{
					FeatureGates: HyperConvergedFeatureGates{
						DisableMDevConfiguration: new(true),
					},
				}
				v1VSpec := hcov1.HyperConvergedSpec{
					Virtualization: hcov1.VirtualizationConfig{
						MediatedDevicesConfiguration: &hcov1.MediatedDevicesConfiguration{
							Enabled:             new(true),
							MediatedDeviceTypes: []string{"nvidia-222"},
						},
					},
				}

				convertMDevEnabledV1beta1ToV1(v1beta1Spec, &v1VSpec)

				Expect(v1VSpec.Virtualization.MediatedDevicesConfiguration.Enabled).To(HaveValue(BeFalse()))
			})

			It("should override enabled=false when the FG is false", func() {
				v1beta1Spec := HyperConvergedSpec{
					FeatureGates: HyperConvergedFeatureGates{
						DisableMDevConfiguration: new(false),
					},
				}
				v1Spec := hcov1.HyperConvergedSpec{
					Virtualization: hcov1.VirtualizationConfig{
						MediatedDevicesConfiguration: &hcov1.MediatedDevicesConfiguration{
							Enabled:             new(false),
							MediatedDeviceTypes: []string{"nvidia-222"},
						},
					},
				}

				convertMDevEnabledV1beta1ToV1(v1beta1Spec, &v1Spec)

				Expect(v1Spec.Virtualization.MediatedDevicesConfiguration.Enabled).To(HaveValue(BeTrue()))
			})

			It("should not override enabled=false when the FG is not set", func() {
				v1beta1Spec := HyperConvergedSpec{
					FeatureGates: HyperConvergedFeatureGates{},
				}
				v1Spec := hcov1.HyperConvergedSpec{
					Virtualization: hcov1.VirtualizationConfig{
						MediatedDevicesConfiguration: &hcov1.MediatedDevicesConfiguration{
							Enabled:             new(true),
							MediatedDeviceTypes: []string{"nvidia-222"},
						},
					},
				}

				convertMDevEnabledV1beta1ToV1(v1beta1Spec, &v1Spec)

				Expect(v1Spec.Virtualization.MediatedDevicesConfiguration.Enabled).To(HaveValue(BeTrue()))
			})

			It("should do nothing when disableMDevConfiguration is unset", func() {
				v1beta1Spec := HyperConvergedSpec{
					MediatedDevicesConfiguration: &MediatedDevicesConfiguration{
						MediatedDeviceTypes: []string{"nvidia-222"},
					},
				}
				v1Spec := hcov1.HyperConvergedSpec{
					Virtualization: hcov1.VirtualizationConfig{
						MediatedDevicesConfiguration: &hcov1.MediatedDevicesConfiguration{
							MediatedDeviceTypes: []string{"nvidia-222"},
						},
					},
				}

				convertMDevEnabledV1beta1ToV1(v1beta1Spec, &v1Spec)

				Expect(v1Spec.Virtualization.MediatedDevicesConfiguration.Enabled).To(BeNil())
			})
		})

		Context("v1 to v1beta1", func() {
			It("should convert enabled=false to v1beta1 disableMDevConfiguration FG = true", func() {
				v1VirtConfig := hcov1.VirtualizationConfig{
					MediatedDevicesConfiguration: &hcov1.MediatedDevicesConfiguration{
						Enabled:             new(false),
						MediatedDeviceTypes: []string{"nvidia-222"},
					},
				}
				v1beta1Spec := HyperConvergedSpec{}

				convertMDevEnabledV1ToV1beta1(v1VirtConfig, &v1beta1Spec)

				Expect(v1beta1Spec.FeatureGates.DisableMDevConfiguration).To(HaveValue(BeTrue()))
			})

			It("should convert enabled=true to the v1beta1 disableMDevConfiguration FG", func() {
				v1VirtConfig := hcov1.VirtualizationConfig{
					MediatedDevicesConfiguration: &hcov1.MediatedDevicesConfiguration{
						Enabled:             new(true),
						MediatedDeviceTypes: []string{"nvidia-222"},
					},
				}
				v1beta1Spec := HyperConvergedSpec{}

				convertMDevEnabledV1ToV1beta1(v1VirtConfig, &v1beta1Spec)

				Expect(v1beta1Spec.FeatureGates.DisableMDevConfiguration).To(HaveValue(BeFalse()))
			})

			It("should not convert to the v1beta1 disableMDevConfiguration FG when enabled is unset", func() {
				v1VirtConfig := hcov1.VirtualizationConfig{
					MediatedDevicesConfiguration: &hcov1.MediatedDevicesConfiguration{
						MediatedDeviceTypes: []string{"nvidia-222"},
					},
				}
				v1beta1Spec := HyperConvergedSpec{}

				convertMDevEnabledV1ToV1beta1(v1VirtConfig, &v1beta1Spec)

				Expect(v1beta1Spec.FeatureGates.DisableMDevConfiguration).To(BeNil())
			})

			It("should do nothing when mediatedDevicesConfiguration is nil", func() {
				v1VirtConfig := hcov1.VirtualizationConfig{}
				v1beta1Spec := HyperConvergedSpec{}

				convertMDevEnabledV1ToV1beta1(v1VirtConfig, &v1beta1Spec)

				Expect(v1beta1Spec.FeatureGates.DisableMDevConfiguration).To(BeNil())
			})
		})

		Context("ConvertTo()", func() {
			DescribeTable("should convert v1beta1 disableMDevConfiguration to v1 enabled, if not set", func(v1beta1FG bool, matcher gomegatypes.GomegaMatcher) {
				src := &HyperConverged{
					Spec: HyperConvergedSpec{
						FeatureGates: HyperConvergedFeatureGates{
							DisableMDevConfiguration: new(v1beta1FG),
						},
					},
				}
				dst := &hcov1.HyperConverged{}

				Expect(src.ConvertTo(dst)).To(Succeed())

				Expect(dst.Spec.Virtualization.MediatedDevicesConfiguration).ToNot(BeNil())
				Expect(dst.Spec.Virtualization.MediatedDevicesConfiguration.Enabled).To(HaveValue(matcher))
			},
				Entry("when the v1beta1 FG is true", true, BeFalse()),
				Entry("when the v1beta1 FG is false", false, BeTrue()),
			)

			DescribeTableSubtree("should convert v1beta1 disableMDevConfiguration to v1 enabled, if set", func(v1Field, v1beta1FG bool) {
				annotation := fmt.Sprintf(`{"mdevConfigEnable": %t}`, v1Field)

				It("just make sure the annotation works", func() {
					src := &HyperConverged{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{
								v1OnlyFieldAnnotation: annotation,
							},
						},
					}
					dst := &hcov1.HyperConverged{}

					Expect(src.ConvertTo(dst)).To(Succeed())

					Expect(dst.Spec.Virtualization.MediatedDevicesConfiguration).ToNot(BeNil())
					Expect(dst.Spec.Virtualization.MediatedDevicesConfiguration.Enabled).To(HaveValue(Equal(v1Field)))
				})

				It("should modify the v1 field", func() {
					src := &HyperConverged{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{
								v1OnlyFieldAnnotation: annotation,
							},
						},
						Spec: HyperConvergedSpec{
							FeatureGates: HyperConvergedFeatureGates{
								DisableMDevConfiguration: new(v1beta1FG),
							},
						},
					}
					dst := &hcov1.HyperConverged{}

					Expect(src.ConvertTo(dst)).To(Succeed())

					Expect(dst.Spec.Virtualization.MediatedDevicesConfiguration).ToNot(BeNil())
					Expect(dst.Spec.Virtualization.MediatedDevicesConfiguration.Enabled).To(HaveValue(Equal(!v1beta1FG)))
				})
			},
				Entry("when the v1 field is true and the v1beta1 FG is true", true, true),
				Entry("when the v1 field is false and the v1beta1 FG is true", false, true),
				Entry("when the v1 field is true and the v1beta1 FG is false", true, false),
				Entry("when the v1 field is false and the v1beta1 FG is false", false, false),
			)

			DescribeTableSubtree("should convert v1beta1 disableMDevConfiguration to v1 FG, if FG set", func(v1FG, v1beta1FG bool) {
				annotation := fmt.Sprintf(`{"disableMDevConfigurationFG": %t}`, v1FG)

				It("just make sure the annotation works", func() {
					src := &HyperConverged{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{
								v1OnlyFieldAnnotation: annotation,
							},
						},
					}
					dst := &hcov1.HyperConverged{}

					Expect(src.ConvertTo(dst)).To(Succeed())

					enabled, found := dst.Spec.FeatureGates.IsExplicitlyEnabled(DisableMDevConfigurationFG)
					Expect(found).To(BeTrue())
					Expect(enabled).To(Equal(v1FG))
				})

				It("should modify the v1 FG", func() {
					src := &HyperConverged{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{
								v1OnlyFieldAnnotation: annotation,
							},
						},
						Spec: HyperConvergedSpec{
							FeatureGates: HyperConvergedFeatureGates{
								DisableMDevConfiguration: new(v1beta1FG),
							},
						},
					}
					dst := &hcov1.HyperConverged{}

					Expect(src.ConvertTo(dst)).To(Succeed())

					Expect(dst.Spec.Virtualization.MediatedDevicesConfiguration).ToNot(BeNil())
					Expect(dst.Spec.Virtualization.MediatedDevicesConfiguration.Enabled).To(HaveValue(Equal(!v1beta1FG)))

					enabled, found := dst.Spec.FeatureGates.IsExplicitlyEnabled(DisableMDevConfigurationFG)
					Expect(found).To(BeTrue())
					Expect(enabled).To(Equal(v1beta1FG))
				})
			},
				Entry("when the v1 field is true and the v1beta1 FG is true", true, true),
				Entry("when the v1 field is false and the v1beta1 FG is true", false, true),
				Entry("when the v1 field is true and the v1beta1 FG is false", true, false),
				Entry("when the v1 field is false and the v1beta1 FG is false", false, false),
			)

			DescribeTableSubtree("should convert v1beta1 FG to v1 FG, if set, and override the v1 enabled field", func(v1Enabled, v1FG, v1beta1FG bool) {
				annotation := fmt.Sprintf(`{"mdevConfigEnable": %t, "disableMDevConfigurationFG": %t}`, v1Enabled, v1FG)

				It("just make sure the annotation works", func() {
					src := &HyperConverged{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{
								v1OnlyFieldAnnotation: annotation,
							},
						},
					}
					dst := &hcov1.HyperConverged{}

					Expect(src.ConvertTo(dst)).To(Succeed())

					Expect(dst.Spec.Virtualization.MediatedDevicesConfiguration).ToNot(BeNil())
					Expect(dst.Spec.Virtualization.MediatedDevicesConfiguration.Enabled).To(HaveValue(Equal(v1Enabled)))

					enabled, found := dst.Spec.FeatureGates.IsExplicitlyEnabled(DisableMDevConfigurationFG)
					Expect(found).To(BeTrue())
					Expect(enabled).To(Equal(v1FG))
				})

				It("should modify the v1 FG and the v1 Enabled field", func() {
					src := &HyperConverged{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{
								v1OnlyFieldAnnotation: annotation,
							},
						},
						Spec: HyperConvergedSpec{
							FeatureGates: HyperConvergedFeatureGates{
								DisableMDevConfiguration: new(v1beta1FG),
							},
						},
					}
					dst := &hcov1.HyperConverged{}

					Expect(src.ConvertTo(dst)).To(Succeed())

					Expect(dst.Spec.Virtualization.MediatedDevicesConfiguration).ToNot(BeNil())
					Expect(dst.Spec.Virtualization.MediatedDevicesConfiguration.Enabled).To(HaveValue(Equal(!v1beta1FG)))

					enabled, found := dst.Spec.FeatureGates.IsExplicitlyEnabled(DisableMDevConfigurationFG)
					Expect(found).To(BeTrue())
					Expect(enabled).To(Equal(v1beta1FG))
				})
			},
				Entry("when the v1 field is true, v1 enabled is true, and the v1beta1 FG is true", true, true, true),
				Entry("when the v1 field is false, v1 enabled is true, and the v1beta1 FG is true", false, true, true),
				Entry("when the v1 field is true, v1 enabled is false, and the v1beta1 FG is true", true, false, true),
				Entry("when the v1 field is false, v1 enabled is false, and the v1beta1 FG is true", false, false, true),
				Entry("when the v1 field is true, v1 enabled is true, and the v1beta1 FG is false", true, true, false),
				Entry("when the v1 field is false, v1 enabled is true, and the v1beta1 FG is false", false, true, false),
				Entry("when the v1 field is true, v1 enabled is false, and the v1beta1 FG is false", true, false, false),
				Entry("when the v1 field is false, v1 enabled is false, and the v1beta1 FG is false", false, false, false),
			)

			DescribeTableSubtree("should convert v1beta1 FG to v1 enabled, if FG set", func(v1FG bool, v1beta1FG bool) {
				annotation := fmt.Sprintf(`{"disableMDevConfigurationFG": %t}`, v1FG)

				It("just make sure the annotation works", func() {
					src := &HyperConverged{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{
								v1OnlyFieldAnnotation: annotation,
							},
						},
					}
					dst := &hcov1.HyperConverged{}

					Expect(src.ConvertTo(dst)).To(Succeed())

					enabled, found := dst.Spec.FeatureGates.IsExplicitlyEnabled(DisableMDevConfigurationFG)
					Expect(found).To(BeTrue())
					Expect(enabled).To(Equal(v1FG))
				})

				It("should modify the v1 FG", func() {
					src := &HyperConverged{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{
								v1OnlyFieldAnnotation: annotation,
							},
						},
						Spec: HyperConvergedSpec{
							FeatureGates: HyperConvergedFeatureGates{
								DisableMDevConfiguration: new(v1beta1FG),
							},
						},
					}
					dst := &hcov1.HyperConverged{}

					Expect(src.ConvertTo(dst)).To(Succeed())

					Expect(dst.Spec.Virtualization.MediatedDevicesConfiguration).ToNot(BeNil())
					Expect(dst.Spec.Virtualization.MediatedDevicesConfiguration.Enabled).To(HaveValue(Equal(!v1beta1FG)))

					enabled, found := dst.Spec.FeatureGates.IsExplicitlyEnabled(DisableMDevConfigurationFG)
					Expect(found).To(BeTrue())
					Expect(enabled).To(Equal(v1beta1FG))
				})
			},
				Entry("when the v1 field is true and the v1beta1 FG is true", true, true),
				Entry("when the v1 field is false and the v1beta1 FG is true", false, true),
				Entry("when the v1 field is true and the v1beta1 FG is false", true, false),
				Entry("when the v1 field is false and the v1beta1 FG is false", false, false),
			)
		})

		Context("ConvertFrom()", func() {
			It("should convert enabled=false to disableMDevConfiguration", func() {
				src := &hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						Virtualization: hcov1.VirtualizationConfig{
							MediatedDevicesConfiguration: &hcov1.MediatedDevicesConfiguration{
								Enabled: new(false),
							},
						},
					},
				}
				dst := &HyperConverged{}

				Expect(dst.ConvertFrom(src)).To(Succeed())

				Expect(dst.Spec.FeatureGates.DisableMDevConfiguration).To(HaveValue(BeTrue()))
			})

			It("should convert enabled=true", func() {
				src := &hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						Virtualization: hcov1.VirtualizationConfig{
							MediatedDevicesConfiguration: &hcov1.MediatedDevicesConfiguration{
								Enabled: new(true),
							},
						},
					},
				}
				dst := &HyperConverged{}

				Expect(dst.ConvertFrom(src)).To(Succeed())

				Expect(dst.Spec.FeatureGates.DisableMDevConfiguration).To(HaveValue(BeFalse()))
			})

			It("should not convert enabled=nil", func() {
				src := &hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						Virtualization: hcov1.VirtualizationConfig{
							MediatedDevicesConfiguration: &hcov1.MediatedDevicesConfiguration{},
						},
					},
				}
				dst := &HyperConverged{}

				Expect(dst.ConvertFrom(src)).To(Succeed())

				Expect(dst.Spec.FeatureGates.DisableMDevConfiguration).To(BeNil())
			})

			It("should ignore v1 FG", func() {
				// this test should not really happen, but it proves that the v1 FG does nothing in conversion
				src := &hcov1.HyperConverged{
					Spec: hcov1.HyperConvergedSpec{
						FeatureGates: hcofg.HyperConvergedFeatureGates{
							{Name: DisableMDevConfigurationFG, State: new(hcofg.Disabled)},
						},
						Virtualization: hcov1.VirtualizationConfig{
							MediatedDevicesConfiguration: &hcov1.MediatedDevicesConfiguration{
								Enabled: new(false),
							},
						},
					},
				}
				dst := &HyperConverged{}

				Expect(dst.ConvertFrom(src)).To(Succeed())

				Expect(dst.Spec.FeatureGates.DisableMDevConfiguration).To(HaveValue(BeTrue()))
			})
		})

		It("should preserve enabled through round-trip", func() {
			original := &HyperConverged{
				Spec: HyperConvergedSpec{
					FeatureGates: HyperConvergedFeatureGates{
						DisableMDevConfiguration: new(true),
					},
					MediatedDevicesConfiguration: &MediatedDevicesConfiguration{
						MediatedDeviceTypes: []string{"nvidia-222"},
					},
				},
			}

			v1hco := &hcov1.HyperConverged{}
			Expect(original.ConvertTo(v1hco)).To(Succeed())

			result := &HyperConverged{}
			Expect(result.ConvertFrom(v1hco)).To(Succeed())

			Expect(result.Spec.FeatureGates.DisableMDevConfiguration).To(HaveValue(BeTrue()))
			Expect(result.Spec.MediatedDevicesConfiguration).ToNot(BeNil())
			Expect(result.Spec.MediatedDevicesConfiguration.MediatedDeviceTypes).To(Equal([]string{"nvidia-222"}))
		})
	})

	Context("virtualization", func() {
		Context("v1 ==> v1beta1", func() {
			It("should convert tuningPolicy", func() {
				v1VirtConfig := hcov1.VirtualizationConfig{
					TuningPolicy: hcov1.HyperConvergedAnnotationTuningPolicy,
				}

				var v1beta1Spec = HyperConvergedSpec{}
				Expect(convertVirtualizationV1ToV1beta1(v1VirtConfig, &v1beta1Spec)).To(Succeed())

				Expect(v1beta1Spec.TuningPolicy).To(Equal(hcov1.HyperConvergedAnnotationTuningPolicy))
			})

			It("should convert LiveMigrationConfig", func() {
				v1VirtConfig := hcov1.VirtualizationConfig{
					LiveMigrationConfig: hcov1.LiveMigrationConfigurations{
						ParallelMigrationsPerCluster:      new(uint32(10)),
						ParallelOutboundMigrationsPerNode: new(uint32(4)),
						BandwidthPerMigration:             new("1Gi"),
						CompletionTimeoutPerGiB:           new(int64(300)),
						ProgressTimeout:                   new(int64(200)),
						AllowAutoConverge:                 new(true),
						AllowPostCopy:                     new(false),
					},
				}

				var v1beta1Spec = HyperConvergedSpec{}
				Expect(convertVirtualizationV1ToV1beta1(v1VirtConfig, &v1beta1Spec)).To(Succeed())

				Expect(v1beta1Spec.LiveMigrationConfig.ParallelMigrationsPerCluster).To(HaveValue(Equal(uint32(10))))
				Expect(v1beta1Spec.LiveMigrationConfig.ParallelOutboundMigrationsPerNode).To(HaveValue(Equal(uint32(4))))
				Expect(v1beta1Spec.LiveMigrationConfig.BandwidthPerMigration).To(HaveValue(Equal("1Gi")))
				Expect(v1beta1Spec.LiveMigrationConfig.CompletionTimeoutPerGiB).To(HaveValue(Equal(int64(300))))
				Expect(v1beta1Spec.LiveMigrationConfig.ProgressTimeout).To(HaveValue(Equal(int64(200))))
				Expect(v1beta1Spec.LiveMigrationConfig.AllowAutoConverge).To(HaveValue(BeTrue()))
				Expect(v1beta1Spec.LiveMigrationConfig.AllowPostCopy).To(HaveValue(BeFalse()))
			})

			It("should convert PermittedHostDevices", func() {
				v1VirtConfig := hcov1.VirtualizationConfig{
					PermittedHostDevices: &hcov1.PermittedHostDevices{
						PciHostDevices: []hcov1.PciHostDevice{
							{
								PCIDeviceSelector:        "selector",
								ResourceName:             "resourceName",
								ExternalResourceProvider: true,
								Disabled:                 false,
							},
						},
					},
				}

				var v1beta1Spec = HyperConvergedSpec{}
				Expect(convertVirtualizationV1ToV1beta1(v1VirtConfig, &v1beta1Spec)).To(Succeed())

				Expect(v1beta1Spec.PermittedHostDevices).ToNot(BeNil())
				Expect(v1beta1Spec.PermittedHostDevices.PciHostDevices).To(HaveLen(1))

			})

			It("should convert MediatedDevicesConfiguration", func() {
				v1VirtConfig := hcov1.VirtualizationConfig{
					MediatedDevicesConfiguration: &hcov1.MediatedDevicesConfiguration{
						Enabled:             new(false),
						MediatedDeviceTypes: []string{"aaa", "bbb", "ccc"},
						NodeMediatedDeviceTypes: []hcov1.NodeMediatedDeviceTypesConfig{
							{
								NodeSelector: map[string]string{
									"ddd": "444",
									"eee": "555",
								},
								MediatedDeviceTypes: []string{"fff", "ggg"},
							},
						},
					},
				}

				var v1beta1Spec = HyperConvergedSpec{}
				Expect(convertVirtualizationV1ToV1beta1(v1VirtConfig, &v1beta1Spec)).To(Succeed())

				Expect(v1beta1Spec.MediatedDevicesConfiguration).ToNot(BeNil())
				Expect(v1beta1Spec.MediatedDevicesConfiguration.MediatedDeviceTypes).To(Equal([]string{"aaa", "bbb", "ccc"}))
				Expect(v1beta1Spec.MediatedDevicesConfiguration.NodeMediatedDeviceTypes).To(HaveLen(1))
				Expect(v1beta1Spec.MediatedDevicesConfiguration.NodeMediatedDeviceTypes[0].MediatedDeviceTypes).To(Equal([]string{"fff", "ggg"}))
				Expect(v1beta1Spec.MediatedDevicesConfiguration.NodeMediatedDeviceTypes[0].NodeSelector).To(HaveLen(2))
				Expect(v1beta1Spec.MediatedDevicesConfiguration.NodeMediatedDeviceTypes[0].NodeSelector).To(HaveKeyWithValue("ddd", "444"))
				Expect(v1beta1Spec.MediatedDevicesConfiguration.NodeMediatedDeviceTypes[0].NodeSelector).To(HaveKeyWithValue("eee", "555"))
			})

			It("should convert WorkloadUpdateStrategy", func() {
				v1VirtConfig := hcov1.VirtualizationConfig{
					WorkloadUpdateStrategy: hcov1.HyperConvergedWorkloadUpdateStrategy{
						WorkloadUpdateMethods: []string{"LiveMigrate", "Evict"},
						BatchEvictionSize:     new(5),
						BatchEvictionInterval: new(metav1.Duration{Duration: 30000000000}),
					},
				}

				var v1beta1Spec = HyperConvergedSpec{}
				Expect(convertVirtualizationV1ToV1beta1(v1VirtConfig, &v1beta1Spec)).To(Succeed())

				Expect(v1beta1Spec.WorkloadUpdateStrategy.WorkloadUpdateMethods).To(Equal([]string{"LiveMigrate", "Evict"}))
				Expect(v1beta1Spec.WorkloadUpdateStrategy.BatchEvictionSize).To(HaveValue(Equal(5)))
				Expect(v1beta1Spec.WorkloadUpdateStrategy.BatchEvictionInterval).To(HaveValue(Equal(metav1.Duration{Duration: 30000000000})))
			})

			It("should convert ObsoleteCPUs", func() {
				v1VirtConfig := hcov1.VirtualizationConfig{
					ObsoleteCPUModels: []string{"model1", "model2"},
				}

				var v1beta1Spec = HyperConvergedSpec{}
				Expect(convertVirtualizationV1ToV1beta1(v1VirtConfig, &v1beta1Spec)).To(Succeed())

				Expect(v1beta1Spec.ObsoleteCPUs).ToNot(BeNil())
				Expect(v1beta1Spec.ObsoleteCPUs.CPUModels).To(Equal([]string{"model1", "model2"}))
			})

			It("should not convert ObsoleteCPUs when nil", func() {
				v1VirtConfig := hcov1.VirtualizationConfig{}

				var v1beta1Spec = HyperConvergedSpec{}
				Expect(convertVirtualizationV1ToV1beta1(v1VirtConfig, &v1beta1Spec)).To(Succeed())

				Expect(v1beta1Spec.ObsoleteCPUs).To(BeNil())
			})

			It("should not convert ObsoleteCPUs when CPUModels is empty", func() {
				v1VirtConfig := hcov1.VirtualizationConfig{
					ObsoleteCPUModels: []string{},
				}

				var v1beta1Spec = HyperConvergedSpec{}
				Expect(convertVirtualizationV1ToV1beta1(v1VirtConfig, &v1beta1Spec)).To(Succeed())

				Expect(v1beta1Spec.ObsoleteCPUs).To(BeNil())
			})

			It("should convert EvictionStrategy", func() {
				v1VirtConfig := hcov1.VirtualizationConfig{
					EvictionStrategy: new(kubevirtv1.EvictionStrategyLiveMigrate),
				}

				var v1beta1Spec = HyperConvergedSpec{}
				Expect(convertVirtualizationV1ToV1beta1(v1VirtConfig, &v1beta1Spec)).To(Succeed())

				Expect(v1beta1Spec.EvictionStrategy).To(HaveValue(Equal(kubevirtv1.EvictionStrategyLiveMigrate)))
			})

			It("should not convert EvictionStrategy when nil", func() {
				v1VirtConfig := hcov1.VirtualizationConfig{}

				var v1beta1Spec = HyperConvergedSpec{}
				Expect(convertVirtualizationV1ToV1beta1(v1VirtConfig, &v1beta1Spec)).To(Succeed())

				Expect(v1beta1Spec.EvictionStrategy).To(BeNil())
			})

			It("should convert VirtualMachineOptions", func() {
				v1VirtConfig := hcov1.VirtualizationConfig{
					VirtualMachineOptions: &hcov1.VirtualMachineOptions{
						DisableFreePageReporting: new(true),
						DisableSerialConsoleLog:  new(false),
						DefaultCPUModel:          new("Haswell"),
						DefaultRuntimeClass:      new("my-runtime-class"),
					},
				}

				var v1beta1Spec = HyperConvergedSpec{}
				Expect(convertVirtualizationV1ToV1beta1(v1VirtConfig, &v1beta1Spec)).To(Succeed())

				Expect(v1beta1Spec.VirtualMachineOptions.DisableFreePageReporting).To(HaveValue(BeTrue()))
				Expect(v1beta1Spec.VirtualMachineOptions.DisableSerialConsoleLog).To(HaveValue(BeFalse()))
				Expect(v1beta1Spec.DefaultCPUModel).To(HaveValue(Equal("Haswell")))
				Expect(v1beta1Spec.DefaultRuntimeClass).To(HaveValue(Equal("my-runtime-class")))
			})

			It("should not convert VirtualMachineOptions when nil", func() {
				v1VirtConfig := hcov1.VirtualizationConfig{}

				var v1beta1Spec = HyperConvergedSpec{}
				Expect(convertVirtualizationV1ToV1beta1(v1VirtConfig, &v1beta1Spec)).To(Succeed())

				Expect(v1beta1Spec.VirtualMachineOptions).To(BeNil())
				Expect(v1beta1Spec.DefaultCPUModel).To(BeNil())
				Expect(v1beta1Spec.DefaultRuntimeClass).To(BeNil())
			})

			It("should convert HigherWorkloadDensity", func() {
				v1VirtConfig := hcov1.VirtualizationConfig{
					HigherWorkloadDensity: &hcov1.HigherWorkloadDensityConfiguration{
						MemoryOvercommitPercentage: 150,
					},
				}

				var v1beta1Spec = HyperConvergedSpec{}
				Expect(convertVirtualizationV1ToV1beta1(v1VirtConfig, &v1beta1Spec)).To(Succeed())

				Expect(v1beta1Spec.HigherWorkloadDensity).ToNot(BeNil())
				Expect(v1beta1Spec.HigherWorkloadDensity.MemoryOvercommitPercentage).To(Equal(150))
			})

			It("should not convert HigherWorkloadDensity when nil", func() {
				v1VirtConfig := hcov1.VirtualizationConfig{}

				var v1beta1Spec = HyperConvergedSpec{}
				Expect(convertVirtualizationV1ToV1beta1(v1VirtConfig, &v1beta1Spec)).To(Succeed())

				Expect(v1beta1Spec.HigherWorkloadDensity).To(BeNil())
			})

			It("should convert LiveUpdateConfiguration", func() {
				v1VirtConfig := hcov1.VirtualizationConfig{
					LiveUpdateConfiguration: &kubevirtv1.LiveUpdateConfiguration{
						MaxHotplugRatio: uint32(4),
					},
				}

				var v1beta1Spec = HyperConvergedSpec{}
				Expect(convertVirtualizationV1ToV1beta1(v1VirtConfig, &v1beta1Spec)).To(Succeed())

				Expect(v1beta1Spec.LiveUpdateConfiguration).ToNot(BeNil())
				Expect(v1beta1Spec.LiveUpdateConfiguration.MaxHotplugRatio).To(Equal(uint32(4)))
			})

			It("should not convert LiveUpdateConfiguration when nil", func() {
				v1VirtConfig := hcov1.VirtualizationConfig{}

				var v1beta1Spec = HyperConvergedSpec{}
				Expect(convertVirtualizationV1ToV1beta1(v1VirtConfig, &v1beta1Spec)).To(Succeed())

				Expect(v1beta1Spec.LiveUpdateConfiguration).To(BeNil())
			})

			It("should convert KSMConfiguration", func() {
				v1VirtConfig := hcov1.VirtualizationConfig{
					KSMConfiguration: &kubevirtv1.KSMConfiguration{
						NodeLabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"ksm": "true"},
						},
					},
				}

				var v1beta1Spec = HyperConvergedSpec{}
				Expect(convertVirtualizationV1ToV1beta1(v1VirtConfig, &v1beta1Spec)).To(Succeed())

				Expect(v1beta1Spec.KSMConfiguration).ToNot(BeNil())
				Expect(v1beta1Spec.KSMConfiguration.NodeLabelSelector).ToNot(BeNil())
				Expect(v1beta1Spec.KSMConfiguration.NodeLabelSelector.MatchLabels).To(HaveKeyWithValue("ksm", "true"))
			})

			It("should not convert KSMConfiguration when nil", func() {
				v1VirtConfig := hcov1.VirtualizationConfig{}

				var v1beta1Spec = HyperConvergedSpec{}
				Expect(convertVirtualizationV1ToV1beta1(v1VirtConfig, &v1beta1Spec)).To(Succeed())

				Expect(v1beta1Spec.KSMConfiguration).To(BeNil())
			})

			It("should convert ChangedBlockTrackingLabelSelectors", func() {
				v1VirtConfig := hcov1.VirtualizationConfig{
					ChangedBlockTrackingLabelSelectors: &kubevirtv1.ChangedBlockTrackingSelectors{
						NamespaceLabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"cbt": "true"},
						},
						VirtualMachineLabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"cbt": "true"},
						},
					},
				}

				var v1beta1Spec = HyperConvergedSpec{}
				Expect(convertVirtualizationV1ToV1beta1(v1VirtConfig, &v1beta1Spec)).To(Succeed())

				Expect(v1beta1Spec.ChangedBlockTrackingLabelSelectors).ToNot(BeNil())
				Expect(v1beta1Spec.ChangedBlockTrackingLabelSelectors.NamespaceLabelSelector).ToNot(BeNil())
				Expect(v1beta1Spec.ChangedBlockTrackingLabelSelectors.NamespaceLabelSelector.MatchLabels).To(HaveKeyWithValue("cbt", "true"))
				Expect(v1beta1Spec.ChangedBlockTrackingLabelSelectors.VirtualMachineLabelSelector).ToNot(BeNil())
				Expect(v1beta1Spec.ChangedBlockTrackingLabelSelectors.VirtualMachineLabelSelector.MatchLabels).To(HaveKeyWithValue("cbt", "true"))
			})

			It("should not convert ChangedBlockTrackingLabelSelectors when nil", func() {
				v1VirtConfig := hcov1.VirtualizationConfig{}

				var v1beta1Spec = HyperConvergedSpec{}
				Expect(convertVirtualizationV1ToV1beta1(v1VirtConfig, &v1beta1Spec)).To(Succeed())

				Expect(v1beta1Spec.ChangedBlockTrackingLabelSelectors).To(BeNil())
			})

			It("should convert Hypervisors", func() {
				v1VirtConfig := hcov1.VirtualizationConfig{
					Hypervisors: []kubevirtv1.HypervisorConfiguration{
						{Name: "kvm"},
					},
				}

				var v1beta1Spec = HyperConvergedSpec{}
				Expect(convertVirtualizationV1ToV1beta1(v1VirtConfig, &v1beta1Spec)).To(Succeed())

				Expect(v1beta1Spec.Hypervisors).To(HaveLen(1))
				Expect(v1beta1Spec.Hypervisors[0].Name).To(Equal("kvm"))
			})

			It("should not convert Hypervisors when empty", func() {
				v1VirtConfig := hcov1.VirtualizationConfig{}

				var v1beta1Spec = HyperConvergedSpec{}
				Expect(convertVirtualizationV1ToV1beta1(v1VirtConfig, &v1beta1Spec)).To(Succeed())

				Expect(v1beta1Spec.Hypervisors).To(BeEmpty())
			})

			It("should convert RoleAggregationStrategy", func() {
				strategy := kubevirtv1.RoleAggregationStrategyManual
				v1VirtConfig := hcov1.VirtualizationConfig{
					RoleAggregationStrategy: &strategy,
				}

				var v1beta1Spec = HyperConvergedSpec{}
				Expect(convertVirtualizationV1ToV1beta1(v1VirtConfig, &v1beta1Spec)).To(Succeed())

				Expect(v1beta1Spec.RoleAggregationStrategy).To(HaveValue(Equal(kubevirtv1.RoleAggregationStrategyManual)))
			})

			It("should not convert RoleAggregationStrategy when nil", func() {
				v1VirtConfig := hcov1.VirtualizationConfig{}

				var v1beta1Spec = HyperConvergedSpec{}
				Expect(convertVirtualizationV1ToV1beta1(v1VirtConfig, &v1beta1Spec)).To(Succeed())

				Expect(v1beta1Spec.RoleAggregationStrategy).To(BeNil())
			})

			It("should convert VmiCPUAllocationRatio and AutoCPULimitNamespaceLabelSelector", func() {
				v1VirtConfig := hcov1.VirtualizationConfig{
					VmiCPUAllocationRatio: new(10),
					AutoCPULimitNamespaceLabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"cpu-limit": "true"},
					},
				}

				var v1beta1Spec = HyperConvergedSpec{}
				Expect(convertVirtualizationV1ToV1beta1(v1VirtConfig, &v1beta1Spec)).To(Succeed())

				Expect(v1beta1Spec.ResourceRequirements).ToNot(BeNil())
				Expect(v1beta1Spec.ResourceRequirements.VmiCPUAllocationRatio).To(HaveValue(Equal(10)))
				Expect(v1beta1Spec.ResourceRequirements.AutoCPULimitNamespaceLabelSelector).ToNot(BeNil())
				Expect(v1beta1Spec.ResourceRequirements.AutoCPULimitNamespaceLabelSelector.MatchLabels).To(HaveKeyWithValue("cpu-limit", "true"))
			})

			It("should convert only VmiCPUAllocationRatio when AutoCPULimitNamespaceLabelSelector is nil", func() {
				v1VirtConfig := hcov1.VirtualizationConfig{
					VmiCPUAllocationRatio: new(5),
				}

				var v1beta1Spec = HyperConvergedSpec{}
				Expect(convertVirtualizationV1ToV1beta1(v1VirtConfig, &v1beta1Spec)).To(Succeed())

				Expect(v1beta1Spec.ResourceRequirements).ToNot(BeNil())
				Expect(v1beta1Spec.ResourceRequirements.VmiCPUAllocationRatio).To(HaveValue(Equal(5)))
				Expect(v1beta1Spec.ResourceRequirements.AutoCPULimitNamespaceLabelSelector).To(BeNil())
			})

			It("should convert only AutoCPULimitNamespaceLabelSelector when VmiCPUAllocationRatio is nil", func() {
				v1VirtConfig := hcov1.VirtualizationConfig{
					AutoCPULimitNamespaceLabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"cpu-limit": "true"},
					},
				}

				var v1beta1Spec = HyperConvergedSpec{}
				Expect(convertVirtualizationV1ToV1beta1(v1VirtConfig, &v1beta1Spec)).To(Succeed())

				Expect(v1beta1Spec.ResourceRequirements).ToNot(BeNil())
				Expect(v1beta1Spec.ResourceRequirements.VmiCPUAllocationRatio).To(BeNil())
				Expect(v1beta1Spec.ResourceRequirements.AutoCPULimitNamespaceLabelSelector).ToNot(BeNil())
			})

			It("should not set ResourceRequirements when both VmiCPUAllocationRatio and AutoCPULimitNamespaceLabelSelector are nil", func() {
				v1VirtConfig := hcov1.VirtualizationConfig{}

				var v1beta1Spec = HyperConvergedSpec{}
				Expect(convertVirtualizationV1ToV1beta1(v1VirtConfig, &v1beta1Spec)).To(Succeed())

				Expect(v1beta1Spec.ResourceRequirements).To(BeNil())
			})
		})

		Context("v1beta1 ==> v1", func() {
			It("should convert TuningPolicy if it's 'annotation'", func() {
				v1beta1Spec := HyperConvergedSpec{
					TuningPolicy: hcov1.HyperConvergedAnnotationTuningPolicy,
				}

				var v1VirtConfig hcov1.VirtualizationConfig
				Expect(convertVirtualizationV1beta1ToV1(v1beta1Spec, &v1VirtConfig)).To(Succeed())

				Expect(v1VirtConfig.TuningPolicy).To(Equal(hcov1.HyperConvergedAnnotationTuningPolicy))
			})

			It("should not convert tuningPolicy if it's 'highBurst'", func() {
				v1beta1Spec := HyperConvergedSpec{
					TuningPolicy: HyperConvergedHighBurstProfile, //nolint SA1019
				}

				var v1VirtConfig hcov1.VirtualizationConfig
				Expect(convertVirtualizationV1beta1ToV1(v1beta1Spec, &v1VirtConfig)).To(Succeed())

				Expect(v1VirtConfig.TuningPolicy).To(BeEmpty())
			})

			It("should convert LiveMigrationConfig", func() {
				v1beta1Spec := HyperConvergedSpec{
					LiveMigrationConfig: hcov1.LiveMigrationConfigurations{
						ParallelMigrationsPerCluster:      new(uint32(10)),
						ParallelOutboundMigrationsPerNode: new(uint32(4)),
						BandwidthPerMigration:             new("1Gi"),
						CompletionTimeoutPerGiB:           new(int64(300)),
						ProgressTimeout:                   new(int64(200)),
						AllowAutoConverge:                 new(true),
						AllowPostCopy:                     new(false),
					},
				}

				var v1VirtConfig hcov1.VirtualizationConfig
				Expect(convertVirtualizationV1beta1ToV1(v1beta1Spec, &v1VirtConfig)).To(Succeed())

				Expect(v1VirtConfig.LiveMigrationConfig.ParallelMigrationsPerCluster).To(HaveValue(Equal(uint32(10))))
				Expect(v1VirtConfig.LiveMigrationConfig.ParallelOutboundMigrationsPerNode).To(HaveValue(Equal(uint32(4))))
				Expect(v1VirtConfig.LiveMigrationConfig.BandwidthPerMigration).To(HaveValue(Equal("1Gi")))
				Expect(v1VirtConfig.LiveMigrationConfig.CompletionTimeoutPerGiB).To(HaveValue(Equal(int64(300))))
				Expect(v1VirtConfig.LiveMigrationConfig.ProgressTimeout).To(HaveValue(Equal(int64(200))))
				Expect(v1VirtConfig.LiveMigrationConfig.AllowAutoConverge).To(HaveValue(BeTrue()))
				Expect(v1VirtConfig.LiveMigrationConfig.AllowPostCopy).To(HaveValue(BeFalse()))
			})

			It("should convert PermittedHostDevices", func() {
				v1beta1Spec := HyperConvergedSpec{
					PermittedHostDevices: &hcov1.PermittedHostDevices{
						PciHostDevices: []hcov1.PciHostDevice{
							{
								PCIDeviceSelector:        "selector",
								ResourceName:             "resourceName",
								ExternalResourceProvider: true,
								Disabled:                 false,
							},
						},
					},
				}

				var v1VirtConfig hcov1.VirtualizationConfig
				Expect(convertVirtualizationV1beta1ToV1(v1beta1Spec, &v1VirtConfig)).To(Succeed())

				Expect(v1VirtConfig.PermittedHostDevices).ToNot(BeNil())
				Expect(v1VirtConfig.PermittedHostDevices.PciHostDevices).To(HaveLen(1))
				Expect(v1VirtConfig.PermittedHostDevices.PciHostDevices[0].PCIDeviceSelector).To(Equal("selector"))
				Expect(v1VirtConfig.PermittedHostDevices.PciHostDevices[0].ResourceName).To(Equal("resourceName"))
				Expect(v1VirtConfig.PermittedHostDevices.PciHostDevices[0].ExternalResourceProvider).To(BeTrue())
			})

			It("should not convert PermittedHostDevices when nil", func() {
				v1beta1Spec := HyperConvergedSpec{}

				var v1VirtConfig hcov1.VirtualizationConfig
				Expect(convertVirtualizationV1beta1ToV1(v1beta1Spec, &v1VirtConfig)).To(Succeed())

				Expect(v1VirtConfig.PermittedHostDevices).To(BeNil())
			})

			It("should convert MediatedDevicesConfiguration", func() {
				v1beta1Spec := HyperConvergedSpec{
					MediatedDevicesConfiguration: &MediatedDevicesConfiguration{
						MediatedDeviceTypes: []string{"aaa", "bbb", "ccc"},
						NodeMediatedDeviceTypes: []NodeMediatedDeviceTypesConfig{
							{
								NodeSelector: map[string]string{
									"ddd": "444",
									"eee": "555",
								},
								MediatedDeviceTypes: []string{"fff", "ggg"},
							},
						},
					},
				}

				var v1VirtConfig hcov1.VirtualizationConfig
				Expect(convertVirtualizationV1beta1ToV1(v1beta1Spec, &v1VirtConfig)).To(Succeed())

				Expect(v1VirtConfig.MediatedDevicesConfiguration).ToNot(BeNil())
				Expect(v1VirtConfig.MediatedDevicesConfiguration.MediatedDeviceTypes).To(Equal([]string{"aaa", "bbb", "ccc"}))
				Expect(v1VirtConfig.MediatedDevicesConfiguration.NodeMediatedDeviceTypes).To(HaveLen(1))
				Expect(v1VirtConfig.MediatedDevicesConfiguration.NodeMediatedDeviceTypes[0].MediatedDeviceTypes).To(Equal([]string{"fff", "ggg"}))
				Expect(v1VirtConfig.MediatedDevicesConfiguration.NodeMediatedDeviceTypes[0].NodeSelector).To(HaveKeyWithValue("ddd", "444"))
				Expect(v1VirtConfig.MediatedDevicesConfiguration.NodeMediatedDeviceTypes[0].NodeSelector).To(HaveKeyWithValue("eee", "555"))
			})

			It("should not convert MediatedDevicesConfiguration when nil", func() {
				v1beta1Spec := HyperConvergedSpec{}

				var v1VirtConfig hcov1.VirtualizationConfig
				Expect(convertVirtualizationV1beta1ToV1(v1beta1Spec, &v1VirtConfig)).To(Succeed())

				Expect(v1VirtConfig.MediatedDevicesConfiguration).To(BeNil())
			})

			It("should convert WorkloadUpdateStrategy", func() {
				v1beta1Spec := HyperConvergedSpec{
					WorkloadUpdateStrategy: hcov1.HyperConvergedWorkloadUpdateStrategy{
						WorkloadUpdateMethods: []string{"LiveMigrate", "Evict"},
						BatchEvictionSize:     new(5),
						BatchEvictionInterval: new(metav1.Duration{Duration: 30000000000}),
					},
				}

				var v1VirtConfig hcov1.VirtualizationConfig
				Expect(convertVirtualizationV1beta1ToV1(v1beta1Spec, &v1VirtConfig)).To(Succeed())

				Expect(v1VirtConfig.WorkloadUpdateStrategy.WorkloadUpdateMethods).To(Equal([]string{"LiveMigrate", "Evict"}))
				Expect(v1VirtConfig.WorkloadUpdateStrategy.BatchEvictionSize).To(HaveValue(Equal(5)))
				Expect(v1VirtConfig.WorkloadUpdateStrategy.BatchEvictionInterval).To(HaveValue(Equal(metav1.Duration{Duration: 30000000000})))
			})

			It("should convert ObsoleteCPUs", func() {
				v1beta1Spec := HyperConvergedSpec{
					ObsoleteCPUs: &HyperConvergedObsoleteCPUs{
						CPUModels: []string{"model1", "model2"},
					},
				}

				var v1VirtConfig hcov1.VirtualizationConfig
				Expect(convertVirtualizationV1beta1ToV1(v1beta1Spec, &v1VirtConfig)).To(Succeed())

				Expect(v1VirtConfig.ObsoleteCPUModels).To(Equal([]string{"model1", "model2"}))
			})

			It("should not convert ObsoleteCPUs when nil", func() {
				v1beta1Spec := HyperConvergedSpec{}

				var v1VirtConfig hcov1.VirtualizationConfig
				Expect(convertVirtualizationV1beta1ToV1(v1beta1Spec, &v1VirtConfig)).To(Succeed())

				Expect(v1VirtConfig.ObsoleteCPUModels).To(BeEmpty())
			})

			It("should convert EvictionStrategy", func() {
				v1beta1Spec := HyperConvergedSpec{
					EvictionStrategy: new(kubevirtv1.EvictionStrategyLiveMigrate),
				}

				var v1VirtConfig hcov1.VirtualizationConfig
				Expect(convertVirtualizationV1beta1ToV1(v1beta1Spec, &v1VirtConfig)).To(Succeed())

				Expect(v1VirtConfig.EvictionStrategy).To(HaveValue(Equal(kubevirtv1.EvictionStrategyLiveMigrate)))
			})

			It("should not convert EvictionStrategy when nil", func() {
				v1beta1Spec := HyperConvergedSpec{}

				var v1VirtConfig hcov1.VirtualizationConfig
				Expect(convertVirtualizationV1beta1ToV1(v1beta1Spec, &v1VirtConfig)).To(Succeed())

				Expect(v1VirtConfig.EvictionStrategy).To(BeNil())
			})

			It("should convert VirtualMachineOptions", func() {
				v1beta1Spec := HyperConvergedSpec{
					VirtualMachineOptions: &VirtualMachineOptions{
						DisableFreePageReporting: new(true),
						DisableSerialConsoleLog:  new(false),
					},
					DefaultCPUModel:     new("Haswell"),
					DefaultRuntimeClass: new("my-runtime-class"),
				}

				var v1VirtConfig hcov1.VirtualizationConfig
				Expect(convertVirtualizationV1beta1ToV1(v1beta1Spec, &v1VirtConfig)).To(Succeed())

				Expect(v1VirtConfig.VirtualMachineOptions).ToNot(BeNil())
				Expect(v1VirtConfig.VirtualMachineOptions.DisableFreePageReporting).To(HaveValue(BeTrue()))
				Expect(v1VirtConfig.VirtualMachineOptions.DisableSerialConsoleLog).To(HaveValue(BeFalse()))
				Expect(v1VirtConfig.VirtualMachineOptions.DefaultCPUModel).To(HaveValue(Equal("Haswell")))
				Expect(v1VirtConfig.VirtualMachineOptions.DefaultRuntimeClass).To(HaveValue(Equal("my-runtime-class")))
			})

			It("should not convert VirtualMachineOptions when nil", func() {
				v1beta1Spec := HyperConvergedSpec{}

				var v1VirtConfig hcov1.VirtualizationConfig
				Expect(convertVirtualizationV1beta1ToV1(v1beta1Spec, &v1VirtConfig)).To(Succeed())

				Expect(v1VirtConfig.VirtualMachineOptions).To(BeNil())
			})

			It("should convert HigherWorkloadDensity", func() {
				v1beta1Spec := HyperConvergedSpec{
					HigherWorkloadDensity: &hcov1.HigherWorkloadDensityConfiguration{
						MemoryOvercommitPercentage: 150,
					},
				}

				var v1VirtConfig hcov1.VirtualizationConfig
				Expect(convertVirtualizationV1beta1ToV1(v1beta1Spec, &v1VirtConfig)).To(Succeed())

				Expect(v1VirtConfig.HigherWorkloadDensity).ToNot(BeNil())
				Expect(v1VirtConfig.HigherWorkloadDensity.MemoryOvercommitPercentage).To(Equal(150))
			})

			It("should not convert HigherWorkloadDensity when nil", func() {
				v1beta1Spec := HyperConvergedSpec{}

				var v1VirtConfig hcov1.VirtualizationConfig
				Expect(convertVirtualizationV1beta1ToV1(v1beta1Spec, &v1VirtConfig)).To(Succeed())

				Expect(v1VirtConfig.HigherWorkloadDensity).To(BeNil())
			})

			It("should convert LiveUpdateConfiguration", func() {
				v1beta1Spec := HyperConvergedSpec{
					LiveUpdateConfiguration: &kubevirtv1.LiveUpdateConfiguration{
						MaxHotplugRatio: uint32(4),
					},
				}

				var v1VirtConfig hcov1.VirtualizationConfig
				Expect(convertVirtualizationV1beta1ToV1(v1beta1Spec, &v1VirtConfig)).To(Succeed())

				Expect(v1VirtConfig.LiveUpdateConfiguration).ToNot(BeNil())
				Expect(v1VirtConfig.LiveUpdateConfiguration.MaxHotplugRatio).To(Equal(uint32(4)))
			})

			It("should not convert LiveUpdateConfiguration when nil", func() {
				v1beta1Spec := HyperConvergedSpec{}

				var v1VirtConfig hcov1.VirtualizationConfig
				Expect(convertVirtualizationV1beta1ToV1(v1beta1Spec, &v1VirtConfig)).To(Succeed())

				Expect(v1VirtConfig.LiveUpdateConfiguration).To(BeNil())
			})

			It("should convert KSMConfiguration", func() {
				v1beta1Spec := HyperConvergedSpec{
					KSMConfiguration: &kubevirtv1.KSMConfiguration{
						NodeLabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"ksm": "true"},
						},
					},
				}

				var v1VirtConfig hcov1.VirtualizationConfig
				Expect(convertVirtualizationV1beta1ToV1(v1beta1Spec, &v1VirtConfig)).To(Succeed())

				Expect(v1VirtConfig.KSMConfiguration).ToNot(BeNil())
				Expect(v1VirtConfig.KSMConfiguration.NodeLabelSelector).ToNot(BeNil())
				Expect(v1VirtConfig.KSMConfiguration.NodeLabelSelector.MatchLabels).To(HaveKeyWithValue("ksm", "true"))
			})

			It("should not convert KSMConfiguration when nil", func() {
				v1beta1Spec := HyperConvergedSpec{}

				var v1VirtConfig hcov1.VirtualizationConfig
				Expect(convertVirtualizationV1beta1ToV1(v1beta1Spec, &v1VirtConfig)).To(Succeed())

				Expect(v1VirtConfig.KSMConfiguration).To(BeNil())
			})

			It("should convert ChangedBlockTrackingLabelSelectors", func() {
				v1beta1Spec := HyperConvergedSpec{
					ChangedBlockTrackingLabelSelectors: &kubevirtv1.ChangedBlockTrackingSelectors{
						NamespaceLabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"cbt": "true"},
						},
						VirtualMachineLabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"cbt": "true"},
						},
					},
				}

				var v1VirtConfig hcov1.VirtualizationConfig
				Expect(convertVirtualizationV1beta1ToV1(v1beta1Spec, &v1VirtConfig)).To(Succeed())

				Expect(v1VirtConfig.ChangedBlockTrackingLabelSelectors).ToNot(BeNil())
				Expect(v1VirtConfig.ChangedBlockTrackingLabelSelectors.NamespaceLabelSelector).ToNot(BeNil())
				Expect(v1VirtConfig.ChangedBlockTrackingLabelSelectors.NamespaceLabelSelector.MatchLabels).To(HaveKeyWithValue("cbt", "true"))
				Expect(v1VirtConfig.ChangedBlockTrackingLabelSelectors.VirtualMachineLabelSelector).ToNot(BeNil())
				Expect(v1VirtConfig.ChangedBlockTrackingLabelSelectors.VirtualMachineLabelSelector.MatchLabels).To(HaveKeyWithValue("cbt", "true"))
			})

			It("should not convert ChangedBlockTrackingLabelSelectors when nil", func() {
				v1beta1Spec := HyperConvergedSpec{}

				var v1VirtConfig hcov1.VirtualizationConfig
				Expect(convertVirtualizationV1beta1ToV1(v1beta1Spec, &v1VirtConfig)).To(Succeed())

				Expect(v1VirtConfig.ChangedBlockTrackingLabelSelectors).To(BeNil())
			})

			It("should convert Hypervisors", func() {
				v1beta1Spec := HyperConvergedSpec{
					Hypervisors: []kubevirtv1.HypervisorConfiguration{
						{Name: "kvm"},
					},
				}

				var v1VirtConfig hcov1.VirtualizationConfig
				Expect(convertVirtualizationV1beta1ToV1(v1beta1Spec, &v1VirtConfig)).To(Succeed())

				Expect(v1VirtConfig.Hypervisors).To(HaveLen(1))
				Expect(v1VirtConfig.Hypervisors[0].Name).To(Equal("kvm"))
			})

			It("should not convert Hypervisors when empty", func() {
				v1beta1Spec := HyperConvergedSpec{}

				var v1VirtConfig hcov1.VirtualizationConfig
				Expect(convertVirtualizationV1beta1ToV1(v1beta1Spec, &v1VirtConfig)).To(Succeed())

				Expect(v1VirtConfig.Hypervisors).To(BeEmpty())
			})

			It("should convert RoleAggregationStrategy", func() {
				strategy := kubevirtv1.RoleAggregationStrategyManual
				v1beta1Spec := HyperConvergedSpec{
					RoleAggregationStrategy: &strategy,
				}

				var v1VirtConfig hcov1.VirtualizationConfig
				Expect(convertVirtualizationV1beta1ToV1(v1beta1Spec, &v1VirtConfig)).To(Succeed())

				Expect(v1VirtConfig.RoleAggregationStrategy).To(HaveValue(Equal(kubevirtv1.RoleAggregationStrategyManual)))
			})

			It("should not convert RoleAggregationStrategy when nil", func() {
				v1beta1Spec := HyperConvergedSpec{}

				var v1VirtConfig hcov1.VirtualizationConfig
				Expect(convertVirtualizationV1beta1ToV1(v1beta1Spec, &v1VirtConfig)).To(Succeed())

				Expect(v1VirtConfig.RoleAggregationStrategy).To(BeNil())
			})

			It("should convert ResourceRequirements with VmiCPUAllocationRatio and AutoCPULimitNamespaceLabelSelector", func() {
				v1beta1Spec := HyperConvergedSpec{
					ResourceRequirements: &OperandResourceRequirements{
						VmiCPUAllocationRatio: new(10),
						AutoCPULimitNamespaceLabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"cpu-limit": "true"},
						},
					},
				}

				var v1VirtConfig hcov1.VirtualizationConfig
				Expect(convertVirtualizationV1beta1ToV1(v1beta1Spec, &v1VirtConfig)).To(Succeed())

				Expect(v1VirtConfig.VmiCPUAllocationRatio).To(HaveValue(Equal(10)))
				Expect(v1VirtConfig.AutoCPULimitNamespaceLabelSelector).ToNot(BeNil())
				Expect(v1VirtConfig.AutoCPULimitNamespaceLabelSelector.MatchLabels).To(HaveKeyWithValue("cpu-limit", "true"))
			})

			It("should convert ResourceRequirements with only VmiCPUAllocationRatio", func() {
				v1beta1Spec := HyperConvergedSpec{
					ResourceRequirements: &OperandResourceRequirements{
						VmiCPUAllocationRatio: new(5),
					},
				}

				var v1VirtConfig hcov1.VirtualizationConfig
				Expect(convertVirtualizationV1beta1ToV1(v1beta1Spec, &v1VirtConfig)).To(Succeed())

				Expect(v1VirtConfig.VmiCPUAllocationRatio).To(HaveValue(Equal(5)))
				Expect(v1VirtConfig.AutoCPULimitNamespaceLabelSelector).To(BeNil())
			})

			It("should convert ResourceRequirements with only AutoCPULimitNamespaceLabelSelector", func() {
				v1beta1Spec := HyperConvergedSpec{
					ResourceRequirements: &OperandResourceRequirements{
						AutoCPULimitNamespaceLabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"cpu-limit": "true"},
						},
					},
				}

				var v1VirtConfig hcov1.VirtualizationConfig
				Expect(convertVirtualizationV1beta1ToV1(v1beta1Spec, &v1VirtConfig)).To(Succeed())

				Expect(v1VirtConfig.VmiCPUAllocationRatio).To(BeNil())
				Expect(v1VirtConfig.AutoCPULimitNamespaceLabelSelector).ToNot(BeNil())
			})

			It("should not convert VmiCPUAllocationRatio and AutoCPULimitNamespaceLabelSelector when ResourceRequirements is nil", func() {
				v1beta1Spec := HyperConvergedSpec{}

				var v1VirtConfig hcov1.VirtualizationConfig
				Expect(convertVirtualizationV1beta1ToV1(v1beta1Spec, &v1VirtConfig)).To(Succeed())

				Expect(v1VirtConfig.VmiCPUAllocationRatio).To(BeNil())
				Expect(v1VirtConfig.AutoCPULimitNamespaceLabelSelector).To(BeNil())
			})
		})
	})

	Context("Storage conversion", func() {
		Context("v1 ==> v1beta1", func() {
			It("should convert VMStateStorageClass", func() {
				v1Storage := &hcov1.StorageConfig{
					VMStateStorageClass: new("my-storage-class"),
				}

				var v1beta1Spec HyperConvergedSpec
				convertStorageV1ToV1beta1(v1Storage, &v1beta1Spec)

				Expect(v1beta1Spec.VMStateStorageClass).To(HaveValue(Equal("my-storage-class")))
			})

			It("should convert ScratchSpaceStorageClass", func() {
				v1Storage := &hcov1.StorageConfig{
					ScratchSpaceStorageClass: new("scratch-class"),
				}

				var v1beta1Spec HyperConvergedSpec
				convertStorageV1ToV1beta1(v1Storage, &v1beta1Spec)

				Expect(v1beta1Spec.ScratchSpaceStorageClass).To(HaveValue(Equal("scratch-class")))
			})

			It("should convert StorageImport with InsecureRegistries", func() {
				v1Storage := &hcov1.StorageConfig{
					StorageImport: &hcov1.StorageImportConfig{
						InsecureRegistries: []string{"registry1.example.com", "registry2.example.com"},
					},
				}

				var v1beta1Spec HyperConvergedSpec
				convertStorageV1ToV1beta1(v1Storage, &v1beta1Spec)

				Expect(v1beta1Spec.StorageImport).ToNot(BeNil())
				Expect(v1beta1Spec.StorageImport.InsecureRegistries).To(Equal([]string{"registry1.example.com", "registry2.example.com"}))
			})

			It("should not convert StorageImport when nil", func() {
				v1Storage := &hcov1.StorageConfig{}

				var v1beta1Spec HyperConvergedSpec
				convertStorageV1ToV1beta1(v1Storage, &v1beta1Spec)

				Expect(v1beta1Spec.StorageImport).To(BeNil())
			})

			It("should convert StorageImport when InsecureRegistries is empty", func() {
				v1Storage := &hcov1.StorageConfig{
					StorageImport: &hcov1.StorageImportConfig{
						InsecureRegistries: []string{},
					},
				}

				var v1beta1Spec HyperConvergedSpec
				convertStorageV1ToV1beta1(v1Storage, &v1beta1Spec)

				Expect(v1beta1Spec.StorageImport).ToNot(BeNil())
				Expect(v1beta1Spec.StorageImport.InsecureRegistries).To(BeEmpty())
			})

			It("should convert FilesystemOverhead", func() {
				v1Storage := &hcov1.StorageConfig{
					FilesystemOverhead: &cdiv1beta1.FilesystemOverhead{
						Global: cdiv1beta1.Percent("0.5"),
						StorageClass: map[string]cdiv1beta1.Percent{
							"class-1": cdiv1beta1.Percent("0.3"),
							"class-2": cdiv1beta1.Percent("0.2"),
						},
					},
				}

				var v1beta1Spec HyperConvergedSpec
				convertStorageV1ToV1beta1(v1Storage, &v1beta1Spec)

				Expect(v1beta1Spec.FilesystemOverhead).ToNot(BeNil())
				Expect(v1beta1Spec.FilesystemOverhead.Global).To(Equal(cdiv1beta1.Percent("0.5")))
				Expect(v1beta1Spec.FilesystemOverhead.StorageClass).To(HaveKeyWithValue("class-1", cdiv1beta1.Percent("0.3")))
				Expect(v1beta1Spec.FilesystemOverhead.StorageClass).To(HaveKeyWithValue("class-2", cdiv1beta1.Percent("0.2")))
			})

			It("should not convert FilesystemOverhead when nil", func() {
				v1Storage := &hcov1.StorageConfig{}

				var v1beta1Spec HyperConvergedSpec
				convertStorageV1ToV1beta1(v1Storage, &v1beta1Spec)

				Expect(v1beta1Spec.FilesystemOverhead).To(BeNil())
			})

			It("should convert StorageWorkloads", func() {
				v1Storage := &hcov1.StorageConfig{
					WorkloadResourceRequirements: &corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("100m"),
						},
					},
				}

				var v1beta1Spec HyperConvergedSpec
				convertStorageV1ToV1beta1(v1Storage, &v1beta1Spec)

				Expect(v1beta1Spec.ResourceRequirements).ToNot(BeNil())
				Expect(v1beta1Spec.ResourceRequirements.StorageWorkloads).ToNot(BeNil())
				Expect(v1beta1Spec.ResourceRequirements.StorageWorkloads.Limits).To(HaveLen(1))
				Expect(v1beta1Spec.ResourceRequirements.StorageWorkloads.Limits).To(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("100m")))
			})

			It("should convert all fields together", func() {
				v1Storage := &hcov1.StorageConfig{
					VMStateStorageClass:      new("vm-state-class"),
					ScratchSpaceStorageClass: new("scratch-class"),
					StorageImport: &hcov1.StorageImportConfig{
						InsecureRegistries: []string{"registry.example.com"},
					},
					FilesystemOverhead: &cdiv1beta1.FilesystemOverhead{
						Global: "0.5",
						StorageClass: map[string]cdiv1beta1.Percent{
							"class-1": "0.3",
							"class-2": "0.2",
						},
					},
					WorkloadResourceRequirements: &corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("100m"),
						},
					},
				}

				var v1beta1Spec HyperConvergedSpec
				convertStorageV1ToV1beta1(v1Storage, &v1beta1Spec)

				Expect(v1beta1Spec.VMStateStorageClass).To(HaveValue(Equal("vm-state-class")))
				Expect(v1beta1Spec.ScratchSpaceStorageClass).To(HaveValue(Equal("scratch-class")))

				Expect(v1beta1Spec.StorageImport).ToNot(BeNil())
				Expect(v1beta1Spec.StorageImport.InsecureRegistries).To(Equal([]string{"registry.example.com"}))

				Expect(v1beta1Spec.FilesystemOverhead).ToNot(BeNil())
				Expect(v1beta1Spec.FilesystemOverhead.Global).To(Equal(cdiv1beta1.Percent("0.5")))
				Expect(v1beta1Spec.FilesystemOverhead.StorageClass).To(HaveKeyWithValue("class-1", cdiv1beta1.Percent("0.3")))
				Expect(v1beta1Spec.FilesystemOverhead.StorageClass).To(HaveKeyWithValue("class-2", cdiv1beta1.Percent("0.2")))

				Expect(v1beta1Spec.ResourceRequirements).ToNot(BeNil())
				Expect(v1beta1Spec.ResourceRequirements.StorageWorkloads).ToNot(BeNil())
				Expect(v1beta1Spec.ResourceRequirements.StorageWorkloads.Limits).To(HaveLen(1))
				Expect(v1beta1Spec.ResourceRequirements.StorageWorkloads.Limits).To(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("100m")))
			})
		})

		Context("v1beta1 ==> v1", func() {
			It("should convert VMStateStorageClass", func() {
				v1beta1Spec := HyperConvergedSpec{
					VMStateStorageClass: new("my-storage-class"),
				}

				v1Storage := convertStorageV1beta1ToV1(v1beta1Spec)

				Expect(v1Storage).ToNot(BeNil())
				Expect(v1Storage.VMStateStorageClass).To(HaveValue(Equal("my-storage-class")))
			})

			It("should convert ScratchSpaceStorageClass", func() {
				v1beta1Spec := HyperConvergedSpec{
					ScratchSpaceStorageClass: new("scratch-class"),
				}

				v1Storage := convertStorageV1beta1ToV1(v1beta1Spec)

				Expect(v1Storage).ToNot(BeNil())
				Expect(v1Storage.ScratchSpaceStorageClass).To(HaveValue(Equal("scratch-class")))
			})

			It("should convert StorageImport with InsecureRegistries", func() {
				v1beta1Spec := HyperConvergedSpec{
					StorageImport: &hcov1.StorageImportConfig{
						InsecureRegistries: []string{"registry1.example.com", "registry2.example.com"},
					},
				}

				v1Storage := convertStorageV1beta1ToV1(v1beta1Spec)

				Expect(v1Storage).ToNot(BeNil())
				Expect(v1Storage.StorageImport).ToNot(BeNil())
				Expect(v1Storage.StorageImport.InsecureRegistries).To(Equal([]string{"registry1.example.com", "registry2.example.com"}))
			})

			It("should return nil when all storage fields are empty", func() {
				v1beta1Spec := HyperConvergedSpec{}

				v1Storage := convertStorageV1beta1ToV1(v1beta1Spec)

				Expect(v1Storage).To(BeNil())
			})

			It("should return nil when StorageImport has empty InsecureRegistries", func() {
				v1beta1Spec := HyperConvergedSpec{
					StorageImport: &hcov1.StorageImportConfig{
						InsecureRegistries: []string{},
					},
				}

				v1Storage := convertStorageV1beta1ToV1(v1beta1Spec)

				Expect(v1Storage).To(BeNil())
			})

			It("should convert FilesystemOverhead", func() {
				v1beta1Spec := HyperConvergedSpec{
					FilesystemOverhead: &cdiv1beta1.FilesystemOverhead{
						Global: "0.5",
						StorageClass: map[string]cdiv1beta1.Percent{
							"class-1": "0.3",
							"class-2": "0.2",
						},
					},
				}

				v1Storage := convertStorageV1beta1ToV1(v1beta1Spec)

				Expect(v1Storage.FilesystemOverhead).ToNot(BeNil())
				Expect(v1Storage.FilesystemOverhead.Global).To(Equal(cdiv1beta1.Percent("0.5")))
				Expect(v1Storage.FilesystemOverhead.StorageClass).To(HaveKeyWithValue("class-1", cdiv1beta1.Percent("0.3")))
				Expect(v1Storage.FilesystemOverhead.StorageClass).To(HaveKeyWithValue("class-2", cdiv1beta1.Percent("0.2")))
			})

			It("should convert StorageWorkloads", func() {
				v1beta1Spec := HyperConvergedSpec{
					ResourceRequirements: &OperandResourceRequirements{
						StorageWorkloads: &corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								corev1.ResourceCPU: resource.MustParse("100m"),
							},
						},
					},
				}

				v1Storage := convertStorageV1beta1ToV1(v1beta1Spec)

				Expect(v1Storage).ToNot(BeNil())
				Expect(v1Storage.WorkloadResourceRequirements).ToNot(BeNil())
				Expect(v1Storage.WorkloadResourceRequirements.Limits).To(HaveLen(1))
				Expect(v1Storage.WorkloadResourceRequirements.Limits).To(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("100m")))
			})

			It("should convert all fields together", func() {
				v1beta1Spec := HyperConvergedSpec{
					VMStateStorageClass:      new("vm-state-class"),
					ScratchSpaceStorageClass: new("scratch-class"),
					StorageImport: &hcov1.StorageImportConfig{
						InsecureRegistries: []string{"registry.example.com"},
					},
					FilesystemOverhead: &cdiv1beta1.FilesystemOverhead{
						Global: "0.5",
						StorageClass: map[string]cdiv1beta1.Percent{
							"class-1": "0.3",
							"class-2": "0.2",
						},
					},
					ResourceRequirements: &OperandResourceRequirements{
						StorageWorkloads: &corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								corev1.ResourceCPU: resource.MustParse("100m"),
							},
						},
					},
				}

				v1Storage := convertStorageV1beta1ToV1(v1beta1Spec)

				Expect(v1Storage).ToNot(BeNil())

				Expect(v1Storage.VMStateStorageClass).To(HaveValue(Equal("vm-state-class")))

				Expect(v1Storage.ScratchSpaceStorageClass).To(HaveValue(Equal("scratch-class")))

				Expect(v1Storage.StorageImport).ToNot(BeNil())
				Expect(v1Storage.StorageImport.InsecureRegistries).To(Equal([]string{"registry.example.com"}))

				Expect(v1Storage.FilesystemOverhead).ToNot(BeNil())
				Expect(v1Storage.FilesystemOverhead.Global).To(Equal(cdiv1beta1.Percent("0.5")))
				Expect(v1Storage.FilesystemOverhead.StorageClass).To(HaveKeyWithValue("class-1", cdiv1beta1.Percent("0.3")))
				Expect(v1Storage.FilesystemOverhead.StorageClass).To(HaveKeyWithValue("class-2", cdiv1beta1.Percent("0.2")))

				Expect(v1Storage.WorkloadResourceRequirements).ToNot(BeNil())
				Expect(v1Storage.WorkloadResourceRequirements.Limits).To(HaveLen(1))
				Expect(v1Storage.WorkloadResourceRequirements.Limits).To(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("100m")))
			})
		})

		Context("round-trip", func() {
			It("should preserve storage config through v1beta1 => v1 => v1beta1", func() {
				original := HyperConvergedSpec{
					VMStateStorageClass:      new("vm-state-class"),
					ScratchSpaceStorageClass: new("scratch-class"),
					StorageImport: &hcov1.StorageImportConfig{
						InsecureRegistries: []string{"registry1.example.com", "registry2.example.com"},
					},
				}

				v1Storage := convertStorageV1beta1ToV1(original)

				var result HyperConvergedSpec
				convertStorageV1ToV1beta1(v1Storage, &result)

				Expect(result.VMStateStorageClass).To(HaveValue(Equal("vm-state-class")))
				Expect(result.ScratchSpaceStorageClass).To(HaveValue(Equal("scratch-class")))
				Expect(result.StorageImport).ToNot(BeNil())
				Expect(result.StorageImport.InsecureRegistries).To(Equal([]string{"registry1.example.com", "registry2.example.com"}))
			})

			It("should preserve storage config through v1 => v1beta1 => v1", func() {
				original := &hcov1.StorageConfig{
					VMStateStorageClass:      new("vm-state-class"),
					ScratchSpaceStorageClass: new("scratch-class"),
					StorageImport: &hcov1.StorageImportConfig{
						InsecureRegistries: []string{"registry.example.com"},
					},
				}

				var v1beta1Spec HyperConvergedSpec
				convertStorageV1ToV1beta1(original, &v1beta1Spec)

				result := convertStorageV1beta1ToV1(v1beta1Spec)

				Expect(result).ToNot(BeNil())
				Expect(result.VMStateStorageClass).To(HaveValue(Equal("vm-state-class")))
				Expect(result.ScratchSpaceStorageClass).To(HaveValue(Equal("scratch-class")))
				Expect(result.StorageImport).ToNot(BeNil())
				Expect(result.StorageImport.InsecureRegistries).To(Equal([]string{"registry.example.com"}))
			})
		})
	})

	Context("Networking conversion", func() {
		Context("v1 ==> v1beta1", func() {
			It("should convert KubeSecondaryDNSNameServerIP", func() {
				v1Networking := &hcov1.NetworkingConfig{
					KubeSecondaryDNSNameServerIP: new("192.168.1.1"),
				}

				var v1beta1Spec HyperConvergedSpec
				convertNetworkingV1ToV1beta1(v1Networking, &v1beta1Spec)

				Expect(v1beta1Spec.KubeSecondaryDNSNameServerIP).To(HaveValue(Equal("192.168.1.1")))
				Expect(v1beta1Spec.KubeMacPoolConfiguration).To(BeNil())
				Expect(v1beta1Spec.NetworkBinding).To(BeNil())
			})

			It("should convert KubeMacPoolConfiguration", func() {
				v1Networking := &hcov1.NetworkingConfig{
					KubeMacPoolConfiguration: &hcov1.KubeMacPoolConfig{
						RangeStart: new("02:00:00:00:00:00"),
						RangeEnd:   new("02:FF:FF:FF:FF:FF"),
					},
				}

				var v1beta1Spec HyperConvergedSpec
				convertNetworkingV1ToV1beta1(v1Networking, &v1beta1Spec)

				Expect(v1beta1Spec.KubeMacPoolConfiguration).ToNot(BeNil())
				Expect(v1beta1Spec.KubeMacPoolConfiguration.RangeStart).To(HaveValue(Equal("02:00:00:00:00:00")))
				Expect(v1beta1Spec.KubeMacPoolConfiguration.RangeEnd).To(HaveValue(Equal("02:FF:FF:FF:FF:FF")))
				Expect(v1beta1Spec.KubeSecondaryDNSNameServerIP).To(BeNil())
				Expect(v1beta1Spec.NetworkBinding).To(BeNil())
			})

			It("should convert NetworkBinding", func() {
				v1Networking := &hcov1.NetworkingConfig{
					NetworkBinding: map[string]kubevirtv1.InterfaceBindingPlugin{
						"test-binding": {
							SidecarImage: "test-image:latest",
						},
					},
				}

				var v1beta1Spec HyperConvergedSpec
				convertNetworkingV1ToV1beta1(v1Networking, &v1beta1Spec)

				Expect(v1beta1Spec.NetworkBinding).To(HaveLen(1))
				Expect(v1beta1Spec.NetworkBinding).To(HaveKeyWithValue("test-binding", kubevirtv1.InterfaceBindingPlugin{
					SidecarImage: "test-image:latest",
				}))
				Expect(v1beta1Spec.KubeSecondaryDNSNameServerIP).To(BeNil())
				Expect(v1beta1Spec.KubeMacPoolConfiguration).To(BeNil())
			})

			It("should not convert when nil", func() {
				var v1beta1Spec HyperConvergedSpec
				convertNetworkingV1ToV1beta1(nil, &v1beta1Spec)

				Expect(v1beta1Spec.KubeSecondaryDNSNameServerIP).To(BeNil())
				Expect(v1beta1Spec.KubeMacPoolConfiguration).To(BeNil())
				Expect(v1beta1Spec.NetworkBinding).To(BeNil())
			})

			It("should convert all fields together", func() {
				v1Networking := &hcov1.NetworkingConfig{
					KubeSecondaryDNSNameServerIP: new("10.0.0.1"),
					KubeMacPoolConfiguration: &hcov1.KubeMacPoolConfig{
						RangeStart: new("02:00:00:00:00:00"),
						RangeEnd:   new("02:FF:FF:FF:FF:FF"),
					},
					NetworkBinding: map[string]kubevirtv1.InterfaceBindingPlugin{
						"binding1": {SidecarImage: "image1:v1"},
						"binding2": {NetworkAttachmentDefinition: "ns/nad"},
					},
				}

				var v1beta1Spec HyperConvergedSpec
				convertNetworkingV1ToV1beta1(v1Networking, &v1beta1Spec)

				Expect(v1beta1Spec.KubeSecondaryDNSNameServerIP).To(HaveValue(Equal("10.0.0.1")))
				Expect(v1beta1Spec.KubeMacPoolConfiguration.RangeStart).To(HaveValue(Equal("02:00:00:00:00:00")))
				Expect(v1beta1Spec.KubeMacPoolConfiguration.RangeEnd).To(HaveValue(Equal("02:FF:FF:FF:FF:FF")))
				Expect(v1beta1Spec.NetworkBinding).To(HaveLen(2))
				Expect(v1beta1Spec.NetworkBinding).To(HaveKeyWithValue("binding1", kubevirtv1.InterfaceBindingPlugin{SidecarImage: "image1:v1"}))
				Expect(v1beta1Spec.NetworkBinding).To(HaveKeyWithValue("binding2", kubevirtv1.InterfaceBindingPlugin{NetworkAttachmentDefinition: "ns/nad"}))
			})
		})

		Context("v1beta1 ==> v1", func() {
			It("should convert KubeSecondaryDNSNameServerIP", func() {
				v1beta1Spec := HyperConvergedSpec{
					KubeSecondaryDNSNameServerIP: new("192.168.1.1"),
				}

				result := convertNetworkingV1beta1ToV1(v1beta1Spec)

				Expect(result).ToNot(BeNil())
				Expect(result.KubeSecondaryDNSNameServerIP).To(HaveValue(Equal("192.168.1.1")))
				Expect(result.KubeMacPoolConfiguration).To(BeNil())
				Expect(result.NetworkBinding).To(BeNil())
			})

			It("should convert KubeMacPoolConfiguration", func() {
				v1beta1Spec := HyperConvergedSpec{
					KubeMacPoolConfiguration: &hcov1.KubeMacPoolConfig{
						RangeStart: new("02:00:00:00:00:00"),
						RangeEnd:   new("02:FF:FF:FF:FF:FF"),
					},
				}

				result := convertNetworkingV1beta1ToV1(v1beta1Spec)

				Expect(result).ToNot(BeNil())
				Expect(result.KubeMacPoolConfiguration).ToNot(BeNil())
				Expect(result.KubeMacPoolConfiguration.RangeStart).To(HaveValue(Equal("02:00:00:00:00:00")))
				Expect(result.KubeMacPoolConfiguration.RangeEnd).To(HaveValue(Equal("02:FF:FF:FF:FF:FF")))
				Expect(result.KubeSecondaryDNSNameServerIP).To(BeNil())
				Expect(result.NetworkBinding).To(BeNil())
			})

			It("should convert NetworkBinding", func() {
				v1beta1Spec := HyperConvergedSpec{
					NetworkBinding: map[string]kubevirtv1.InterfaceBindingPlugin{
						"test-binding": {
							SidecarImage: "test-image:latest",
						},
					},
				}

				result := convertNetworkingV1beta1ToV1(v1beta1Spec)

				Expect(result).ToNot(BeNil())
				Expect(result.NetworkBinding).To(HaveLen(1))
				Expect(result.NetworkBinding).To(HaveKeyWithValue("test-binding", kubevirtv1.InterfaceBindingPlugin{
					SidecarImage: "test-image:latest",
				}))
				Expect(result.KubeSecondaryDNSNameServerIP).To(BeNil())
				Expect(result.KubeMacPoolConfiguration).To(BeNil())
			})

			It("should return nil when all fields are nil", func() {
				v1beta1Spec := HyperConvergedSpec{}

				result := convertNetworkingV1beta1ToV1(v1beta1Spec)

				Expect(result).To(BeNil())
			})

			It("should convert all fields together", func() {
				v1beta1Spec := HyperConvergedSpec{
					KubeSecondaryDNSNameServerIP: new("10.0.0.1"),
					KubeMacPoolConfiguration: &hcov1.KubeMacPoolConfig{
						RangeStart: new("02:00:00:00:00:00"),
						RangeEnd:   new("02:FF:FF:FF:FF:FF"),
					},
					NetworkBinding: map[string]kubevirtv1.InterfaceBindingPlugin{
						"binding1": {SidecarImage: "image1:v1"},
						"binding2": {NetworkAttachmentDefinition: "ns/nad"},
					},
				}

				result := convertNetworkingV1beta1ToV1(v1beta1Spec)

				Expect(result).ToNot(BeNil())
				Expect(result.KubeSecondaryDNSNameServerIP).To(HaveValue(Equal("10.0.0.1")))
				Expect(result.KubeMacPoolConfiguration.RangeStart).To(HaveValue(Equal("02:00:00:00:00:00")))
				Expect(result.KubeMacPoolConfiguration.RangeEnd).To(HaveValue(Equal("02:FF:FF:FF:FF:FF")))
				Expect(result.NetworkBinding).To(HaveLen(2))
				Expect(result.NetworkBinding).To(HaveKeyWithValue("binding1", kubevirtv1.InterfaceBindingPlugin{SidecarImage: "image1:v1"}))
				Expect(result.NetworkBinding).To(HaveKeyWithValue("binding2", kubevirtv1.InterfaceBindingPlugin{NetworkAttachmentDefinition: "ns/nad"}))
			})
		})

		Context("round-trip", func() {
			It("should preserve networking config through v1beta1 => v1 => v1beta1", func() {
				original := HyperConvergedSpec{
					KubeSecondaryDNSNameServerIP: new("10.0.0.1"),
					KubeMacPoolConfiguration: &hcov1.KubeMacPoolConfig{
						RangeStart: new("02:00:00:00:00:00"),
						RangeEnd:   new("02:FF:FF:FF:FF:FF"),
					},
					NetworkBinding: map[string]kubevirtv1.InterfaceBindingPlugin{
						"binding1": {SidecarImage: "image1:v1"},
						"binding2": {NetworkAttachmentDefinition: "ns/nad"},
					},
				}

				v1Networking := convertNetworkingV1beta1ToV1(original)

				var result HyperConvergedSpec
				convertNetworkingV1ToV1beta1(v1Networking, &result)

				Expect(result.KubeSecondaryDNSNameServerIP).To(Equal(original.KubeSecondaryDNSNameServerIP))
				Expect(result.KubeMacPoolConfiguration.RangeStart).To(Equal(original.KubeMacPoolConfiguration.RangeStart))
				Expect(result.KubeMacPoolConfiguration.RangeEnd).To(Equal(original.KubeMacPoolConfiguration.RangeEnd))
				Expect(result.NetworkBinding).To(Equal(original.NetworkBinding))
			})

			It("should preserve networking config through v1 => v1beta1 => v1", func() {
				original := &hcov1.NetworkingConfig{
					KubeSecondaryDNSNameServerIP: new("10.0.0.1"),
					KubeMacPoolConfiguration: &hcov1.KubeMacPoolConfig{
						RangeStart: new("02:00:00:00:00:00"),
						RangeEnd:   new("02:FF:FF:FF:FF:FF"),
					},
					NetworkBinding: map[string]kubevirtv1.InterfaceBindingPlugin{
						"binding1": {SidecarImage: "image1:v1"},
						"binding2": {NetworkAttachmentDefinition: "ns/nad"},
					},
				}

				var v1beta1Spec HyperConvergedSpec
				convertNetworkingV1ToV1beta1(original, &v1beta1Spec)

				result := convertNetworkingV1beta1ToV1(v1beta1Spec)

				Expect(result).ToNot(BeNil())
				Expect(result.KubeSecondaryDNSNameServerIP).To(Equal(original.KubeSecondaryDNSNameServerIP))
				Expect(result.KubeMacPoolConfiguration.RangeStart).To(Equal(original.KubeMacPoolConfiguration.RangeStart))
				Expect(result.KubeMacPoolConfiguration.RangeEnd).To(Equal(original.KubeMacPoolConfiguration.RangeEnd))
				Expect(result.NetworkBinding).To(Equal(original.NetworkBinding))
			})

			It("should preserve nil networking through round-trip", func() {
				original := HyperConvergedSpec{}

				v1Networking := convertNetworkingV1beta1ToV1(original)
				Expect(v1Networking).To(BeNil())

				var result HyperConvergedSpec
				convertNetworkingV1ToV1beta1(v1Networking, &result)

				Expect(result.KubeSecondaryDNSNameServerIP).To(BeNil())
				Expect(result.KubeMacPoolConfiguration).To(BeNil())
				Expect(result.NetworkBinding).To(BeNil())
			})
		})
	})

	Context("WorkloadSources conversion", func() {
		Context("v1 ==> v1beta1", func() {
			It("should convert CommonTemplatesNamespace", func() {
				v1Config := hcov1.WorkloadSourcesConfig{
					CommonTemplatesNamespace: new("my-ns"),
				}

				var v1beta1Spec HyperConvergedSpec
				convertWorkloadSourcesV1ToV1beta1(v1Config, &v1beta1Spec)

				Expect(v1beta1Spec.CommonTemplatesNamespace).To(HaveValue(Equal("my-ns")))
				Expect(v1beta1Spec.CommonBootImageNamespace).To(BeNil())
				Expect(v1beta1Spec.EnableCommonBootImageImport).To(BeNil())
				Expect(v1beta1Spec.DataImportCronTemplates).To(BeEmpty())
				Expect(v1beta1Spec.InstancetypeConfig).To(BeNil())
				Expect(v1beta1Spec.CommonInstancetypesDeployment).To(BeNil())
			})

			It("should convert CommonBootImageNamespace", func() {
				v1Config := hcov1.WorkloadSourcesConfig{
					CommonBootImageNamespace: new("boot-ns"),
				}

				var v1beta1Spec HyperConvergedSpec
				convertWorkloadSourcesV1ToV1beta1(v1Config, &v1beta1Spec)

				Expect(v1beta1Spec.CommonBootImageNamespace).To(HaveValue(Equal("boot-ns")))
				Expect(v1beta1Spec.CommonTemplatesNamespace).To(BeNil())
			})

			It("should convert EnableCommonBootImageImport", func() {
				v1Config := hcov1.WorkloadSourcesConfig{
					EnableCommonBootImageImport: new(true),
				}

				var v1beta1Spec HyperConvergedSpec
				convertWorkloadSourcesV1ToV1beta1(v1Config, &v1beta1Spec)

				Expect(v1beta1Spec.EnableCommonBootImageImport).To(HaveValue(BeTrue()))
			})

			It("should convert DataImportCronTemplates", func() {
				v1Config := hcov1.WorkloadSourcesConfig{
					DataImportCronTemplates: []hcov1.DataImportCronTemplate{
						{ObjectMeta: metav1.ObjectMeta{Name: "template1"}},
						{ObjectMeta: metav1.ObjectMeta{Name: "template2"}},
					},
				}

				var v1beta1Spec HyperConvergedSpec
				convertWorkloadSourcesV1ToV1beta1(v1Config, &v1beta1Spec)

				Expect(v1beta1Spec.DataImportCronTemplates).To(HaveLen(2))
				Expect(v1beta1Spec.DataImportCronTemplates[0].Name).To(Equal("template1"))
				Expect(v1beta1Spec.DataImportCronTemplates[1].Name).To(Equal("template2"))
			})

			It("should not convert DataImportCronTemplates when empty", func() {
				v1Config := hcov1.WorkloadSourcesConfig{}

				var v1beta1Spec HyperConvergedSpec
				convertWorkloadSourcesV1ToV1beta1(v1Config, &v1beta1Spec)

				Expect(v1beta1Spec.DataImportCronTemplates).To(BeEmpty())
			})

			It("should convert InstancetypeConfig", func() {
				v1Config := hcov1.WorkloadSourcesConfig{
					InstancetypeConfig: &kubevirtv1.InstancetypeConfiguration{},
				}

				var v1beta1Spec HyperConvergedSpec
				convertWorkloadSourcesV1ToV1beta1(v1Config, &v1beta1Spec)

				Expect(v1beta1Spec.InstancetypeConfig).ToNot(BeNil())
			})

			It("should convert CommonInstancetypesDeployment", func() {
				v1Config := hcov1.WorkloadSourcesConfig{
					CommonInstancetypesDeployment: &kubevirtv1.CommonInstancetypesDeployment{
						Enabled: new(true),
					},
				}

				var v1beta1Spec HyperConvergedSpec
				convertWorkloadSourcesV1ToV1beta1(v1Config, &v1beta1Spec)

				Expect(v1beta1Spec.CommonInstancetypesDeployment).ToNot(BeNil())
				Expect(v1beta1Spec.CommonInstancetypesDeployment.Enabled).To(HaveValue(BeTrue()))
			})

			It("should convert all fields together", func() {
				v1Config := hcov1.WorkloadSourcesConfig{
					CommonTemplatesNamespace:    new("templates-ns"),
					CommonBootImageNamespace:    new("boot-ns"),
					EnableCommonBootImageImport: new(false),
					DataImportCronTemplates: []hcov1.DataImportCronTemplate{
						{ObjectMeta: metav1.ObjectMeta{Name: "tmpl1"}},
					},
					InstancetypeConfig: &kubevirtv1.InstancetypeConfiguration{},
					CommonInstancetypesDeployment: &kubevirtv1.CommonInstancetypesDeployment{
						Enabled: new(true),
					},
				}

				var v1beta1Spec HyperConvergedSpec
				convertWorkloadSourcesV1ToV1beta1(v1Config, &v1beta1Spec)

				Expect(v1beta1Spec.CommonTemplatesNamespace).To(HaveValue(Equal("templates-ns")))
				Expect(v1beta1Spec.CommonBootImageNamespace).To(HaveValue(Equal("boot-ns")))
				Expect(v1beta1Spec.EnableCommonBootImageImport).To(HaveValue(BeFalse()))
				Expect(v1beta1Spec.DataImportCronTemplates).To(HaveLen(1))
				Expect(v1beta1Spec.DataImportCronTemplates[0].Name).To(Equal("tmpl1"))
				Expect(v1beta1Spec.InstancetypeConfig).ToNot(BeNil())
				Expect(v1beta1Spec.CommonInstancetypesDeployment).ToNot(BeNil())
			})
		})

		Context("v1beta1 ==> v1", func() {
			It("should convert CommonTemplatesNamespace", func() {
				v1beta1Spec := HyperConvergedSpec{
					CommonTemplatesNamespace: new("my-ns"),
				}

				var v1Config hcov1.WorkloadSourcesConfig
				convertWorkloadSourcesV1beta1ToV1(v1beta1Spec, &v1Config)

				Expect(v1Config.CommonTemplatesNamespace).To(HaveValue(Equal("my-ns")))
				Expect(v1Config.CommonBootImageNamespace).To(BeNil())
				Expect(v1Config.EnableCommonBootImageImport).To(BeNil())
				Expect(v1Config.DataImportCronTemplates).To(BeEmpty())
				Expect(v1Config.InstancetypeConfig).To(BeNil())
				Expect(v1Config.CommonInstancetypesDeployment).To(BeNil())
			})

			It("should convert CommonBootImageNamespace", func() {
				v1beta1Spec := HyperConvergedSpec{
					CommonBootImageNamespace: new("boot-ns"),
				}

				var v1Config hcov1.WorkloadSourcesConfig
				convertWorkloadSourcesV1beta1ToV1(v1beta1Spec, &v1Config)

				Expect(v1Config.CommonBootImageNamespace).To(HaveValue(Equal("boot-ns")))
				Expect(v1Config.CommonTemplatesNamespace).To(BeNil())
			})

			It("should convert EnableCommonBootImageImport", func() {
				v1beta1Spec := HyperConvergedSpec{
					EnableCommonBootImageImport: new(true),
				}

				var v1Config hcov1.WorkloadSourcesConfig
				convertWorkloadSourcesV1beta1ToV1(v1beta1Spec, &v1Config)

				Expect(v1Config.EnableCommonBootImageImport).To(HaveValue(BeTrue()))
			})

			It("should convert DataImportCronTemplates", func() {
				v1beta1Spec := HyperConvergedSpec{
					DataImportCronTemplates: []hcov1.DataImportCronTemplate{
						{ObjectMeta: metav1.ObjectMeta{Name: "template1"}},
						{ObjectMeta: metav1.ObjectMeta{Name: "template2"}},
					},
				}

				var v1Config hcov1.WorkloadSourcesConfig
				convertWorkloadSourcesV1beta1ToV1(v1beta1Spec, &v1Config)

				Expect(v1Config.DataImportCronTemplates).To(HaveLen(2))
				Expect(v1Config.DataImportCronTemplates[0].Name).To(Equal("template1"))
				Expect(v1Config.DataImportCronTemplates[1].Name).To(Equal("template2"))
			})

			It("should not convert DataImportCronTemplates when empty", func() {
				v1beta1Spec := HyperConvergedSpec{}

				var v1Config hcov1.WorkloadSourcesConfig
				convertWorkloadSourcesV1beta1ToV1(v1beta1Spec, &v1Config)

				Expect(v1Config.DataImportCronTemplates).To(BeEmpty())
			})

			It("should convert InstancetypeConfig", func() {
				v1beta1Spec := HyperConvergedSpec{
					InstancetypeConfig: &kubevirtv1.InstancetypeConfiguration{},
				}

				var v1Config hcov1.WorkloadSourcesConfig
				convertWorkloadSourcesV1beta1ToV1(v1beta1Spec, &v1Config)

				Expect(v1Config.InstancetypeConfig).ToNot(BeNil())
			})

			It("should convert CommonInstancetypesDeployment", func() {
				v1beta1Spec := HyperConvergedSpec{
					CommonInstancetypesDeployment: &kubevirtv1.CommonInstancetypesDeployment{
						Enabled: new(true),
					},
				}

				var v1Config hcov1.WorkloadSourcesConfig
				convertWorkloadSourcesV1beta1ToV1(v1beta1Spec, &v1Config)

				Expect(v1Config.CommonInstancetypesDeployment).ToNot(BeNil())
				Expect(v1Config.CommonInstancetypesDeployment.Enabled).To(HaveValue(BeTrue()))
			})

			It("should convert all fields together", func() {
				v1beta1Spec := HyperConvergedSpec{
					CommonTemplatesNamespace:    new("templates-ns"),
					CommonBootImageNamespace:    new("boot-ns"),
					EnableCommonBootImageImport: new(false),
					DataImportCronTemplates: []hcov1.DataImportCronTemplate{
						{ObjectMeta: metav1.ObjectMeta{Name: "tmpl1"}},
					},
					InstancetypeConfig: &kubevirtv1.InstancetypeConfiguration{},
					CommonInstancetypesDeployment: &kubevirtv1.CommonInstancetypesDeployment{
						Enabled: new(true),
					},
				}

				var v1Config hcov1.WorkloadSourcesConfig
				convertWorkloadSourcesV1beta1ToV1(v1beta1Spec, &v1Config)

				Expect(v1Config.CommonTemplatesNamespace).To(HaveValue(Equal("templates-ns")))
				Expect(v1Config.CommonBootImageNamespace).To(HaveValue(Equal("boot-ns")))
				Expect(v1Config.EnableCommonBootImageImport).To(HaveValue(BeFalse()))
				Expect(v1Config.DataImportCronTemplates).To(HaveLen(1))
				Expect(v1Config.DataImportCronTemplates[0].Name).To(Equal("tmpl1"))
				Expect(v1Config.InstancetypeConfig).ToNot(BeNil())
				Expect(v1Config.CommonInstancetypesDeployment).ToNot(BeNil())
			})
		})

		Context("round-trip", func() {
			It("should preserve workload sources config through v1beta1 => v1 => v1beta1", func() {
				original := HyperConvergedSpec{
					CommonTemplatesNamespace:    new("templates-ns"),
					CommonBootImageNamespace:    new("boot-ns"),
					EnableCommonBootImageImport: new(true),
					DataImportCronTemplates: []hcov1.DataImportCronTemplate{
						{ObjectMeta: metav1.ObjectMeta{Name: "tmpl1"}},
					},
					InstancetypeConfig: &kubevirtv1.InstancetypeConfiguration{},
					CommonInstancetypesDeployment: &kubevirtv1.CommonInstancetypesDeployment{
						Enabled: new(true),
					},
				}

				var v1Config hcov1.WorkloadSourcesConfig
				convertWorkloadSourcesV1beta1ToV1(original, &v1Config)

				var result HyperConvergedSpec
				convertWorkloadSourcesV1ToV1beta1(v1Config, &result)

				Expect(result.CommonTemplatesNamespace).To(Equal(original.CommonTemplatesNamespace))
				Expect(result.CommonBootImageNamespace).To(Equal(original.CommonBootImageNamespace))
				Expect(result.EnableCommonBootImageImport).To(Equal(original.EnableCommonBootImageImport))
				Expect(result.DataImportCronTemplates).To(HaveLen(1))
				Expect(result.DataImportCronTemplates[0].Name).To(Equal("tmpl1"))
				Expect(result.InstancetypeConfig).ToNot(BeNil())
				Expect(result.CommonInstancetypesDeployment).ToNot(BeNil())
			})

			It("should preserve workload sources config through v1 => v1beta1 => v1", func() {
				original := hcov1.WorkloadSourcesConfig{
					CommonTemplatesNamespace:    new("templates-ns"),
					CommonBootImageNamespace:    new("boot-ns"),
					EnableCommonBootImageImport: new(true),
					DataImportCronTemplates: []hcov1.DataImportCronTemplate{
						{ObjectMeta: metav1.ObjectMeta{Name: "tmpl1"}},
					},
					InstancetypeConfig: &kubevirtv1.InstancetypeConfiguration{},
					CommonInstancetypesDeployment: &kubevirtv1.CommonInstancetypesDeployment{
						Enabled: new(true),
					},
				}

				var v1beta1Spec HyperConvergedSpec
				convertWorkloadSourcesV1ToV1beta1(original, &v1beta1Spec)

				var result hcov1.WorkloadSourcesConfig
				convertWorkloadSourcesV1beta1ToV1(v1beta1Spec, &result)

				Expect(result.CommonTemplatesNamespace).To(Equal(original.CommonTemplatesNamespace))
				Expect(result.CommonBootImageNamespace).To(Equal(original.CommonBootImageNamespace))
				Expect(result.EnableCommonBootImageImport).To(Equal(original.EnableCommonBootImageImport))
				Expect(result.DataImportCronTemplates).To(HaveLen(1))
				Expect(result.DataImportCronTemplates[0].Name).To(Equal("tmpl1"))
				Expect(result.InstancetypeConfig).ToNot(BeNil())
				Expect(result.CommonInstancetypesDeployment).ToNot(BeNil())
			})
		})
	})

	Context("Security conversion", func() {
		Context("v1 ==> v1beta1", func() {
			It("should convert CertConfig", func() {
				v1Security := hcov1.SecurityConfig{
					CertConfig: hcov1.HyperConvergedCertConfig{
						CA: hcov1.CertRotateConfigCA{
							Duration:    new(metav1.Duration{Duration: 48 * time.Hour}),
							RenewBefore: new(metav1.Duration{Duration: 24 * time.Hour}),
						},
						Server: hcov1.CertRotateConfigServer{
							Duration:    new(metav1.Duration{Duration: 24 * time.Hour}),
							RenewBefore: new(metav1.Duration{Duration: 12 * time.Hour}),
						},
					},
				}

				var v1beta1Spec HyperConvergedSpec
				convertSecurityV1ToV1beta1(v1Security, &v1beta1Spec)

				Expect(v1beta1Spec.CertConfig.CA.Duration).To(HaveValue(Equal(metav1.Duration{Duration: 48 * time.Hour})))
				Expect(v1beta1Spec.CertConfig.CA.RenewBefore).To(HaveValue(Equal(metav1.Duration{Duration: 24 * time.Hour})))
				Expect(v1beta1Spec.CertConfig.Server.Duration).To(HaveValue(Equal(metav1.Duration{Duration: 24 * time.Hour})))
				Expect(v1beta1Spec.CertConfig.Server.RenewBefore).To(HaveValue(Equal(metav1.Duration{Duration: 12 * time.Hour})))
			})

			It("should convert TLSSecurityProfile", func() {
				v1Security := hcov1.SecurityConfig{
					TLSSecurityProfile: &openshiftconfigv1.TLSSecurityProfile{
						Type:         openshiftconfigv1.TLSProfileIntermediateType,
						Intermediate: &openshiftconfigv1.IntermediateTLSProfile{},
					},
				}

				var v1beta1Spec HyperConvergedSpec
				convertSecurityV1ToV1beta1(v1Security, &v1beta1Spec)

				Expect(v1beta1Spec.TLSSecurityProfile).ToNot(BeNil())
				Expect(v1beta1Spec.TLSSecurityProfile.Type).To(Equal(openshiftconfigv1.TLSProfileIntermediateType))
			})

			It("should not convert TLSSecurityProfile when nil", func() {
				v1Security := hcov1.SecurityConfig{}

				var v1beta1Spec HyperConvergedSpec
				convertSecurityV1ToV1beta1(v1Security, &v1beta1Spec)

				Expect(v1beta1Spec.TLSSecurityProfile).To(BeNil())
			})

			It("should convert all fields together", func() {
				v1Security := hcov1.SecurityConfig{
					CertConfig: hcov1.HyperConvergedCertConfig{
						CA: hcov1.CertRotateConfigCA{
							Duration:    new(metav1.Duration{Duration: 48 * time.Hour}),
							RenewBefore: new(metav1.Duration{Duration: 24 * time.Hour}),
						},
						Server: hcov1.CertRotateConfigServer{
							Duration:    new(metav1.Duration{Duration: 24 * time.Hour}),
							RenewBefore: new(metav1.Duration{Duration: 12 * time.Hour}),
						},
					},
					TLSSecurityProfile: &openshiftconfigv1.TLSSecurityProfile{
						Type: openshiftconfigv1.TLSProfileOldType,
						Old:  &openshiftconfigv1.OldTLSProfile{},
					},
				}

				var v1beta1Spec HyperConvergedSpec
				convertSecurityV1ToV1beta1(v1Security, &v1beta1Spec)

				Expect(v1beta1Spec.CertConfig.CA.Duration).To(HaveValue(Equal(metav1.Duration{Duration: 48 * time.Hour})))
				Expect(v1beta1Spec.TLSSecurityProfile).ToNot(BeNil())
				Expect(v1beta1Spec.TLSSecurityProfile.Type).To(Equal(openshiftconfigv1.TLSProfileOldType))
			})
		})

		Context("v1beta1 ==> v1", func() {
			It("should convert CertConfig", func() {
				v1beta1Spec := HyperConvergedSpec{
					CertConfig: hcov1.HyperConvergedCertConfig{
						CA: hcov1.CertRotateConfigCA{
							Duration:    new(metav1.Duration{Duration: 48 * time.Hour}),
							RenewBefore: new(metav1.Duration{Duration: 24 * time.Hour}),
						},
						Server: hcov1.CertRotateConfigServer{
							Duration:    new(metav1.Duration{Duration: 24 * time.Hour}),
							RenewBefore: new(metav1.Duration{Duration: 12 * time.Hour}),
						},
					},
				}

				var v1Security hcov1.SecurityConfig
				convertSecurityV1beta1ToV1(v1beta1Spec, &v1Security)

				Expect(v1Security.CertConfig.CA.Duration).To(HaveValue(Equal(metav1.Duration{Duration: 48 * time.Hour})))
				Expect(v1Security.CertConfig.CA.RenewBefore).To(HaveValue(Equal(metav1.Duration{Duration: 24 * time.Hour})))
				Expect(v1Security.CertConfig.Server.Duration).To(HaveValue(Equal(metav1.Duration{Duration: 24 * time.Hour})))
				Expect(v1Security.CertConfig.Server.RenewBefore).To(HaveValue(Equal(metav1.Duration{Duration: 12 * time.Hour})))
			})

			It("should convert TLSSecurityProfile", func() {
				v1beta1Spec := HyperConvergedSpec{
					TLSSecurityProfile: &openshiftconfigv1.TLSSecurityProfile{
						Type:         openshiftconfigv1.TLSProfileIntermediateType,
						Intermediate: &openshiftconfigv1.IntermediateTLSProfile{},
					},
				}

				var v1Security hcov1.SecurityConfig
				convertSecurityV1beta1ToV1(v1beta1Spec, &v1Security)

				Expect(v1Security.TLSSecurityProfile).ToNot(BeNil())
				Expect(v1Security.TLSSecurityProfile.Type).To(Equal(openshiftconfigv1.TLSProfileIntermediateType))
			})

			It("should not convert TLSSecurityProfile when nil", func() {
				v1beta1Spec := HyperConvergedSpec{}

				var v1Security hcov1.SecurityConfig
				convertSecurityV1beta1ToV1(v1beta1Spec, &v1Security)

				Expect(v1Security.TLSSecurityProfile).To(BeNil())
			})

			It("should convert all fields together", func() {
				v1beta1Spec := HyperConvergedSpec{
					CertConfig: hcov1.HyperConvergedCertConfig{
						CA: hcov1.CertRotateConfigCA{
							Duration:    new(metav1.Duration{Duration: 48 * time.Hour}),
							RenewBefore: new(metav1.Duration{Duration: 24 * time.Hour}),
						},
						Server: hcov1.CertRotateConfigServer{
							Duration:    new(metav1.Duration{Duration: 24 * time.Hour}),
							RenewBefore: new(metav1.Duration{Duration: 12 * time.Hour}),
						},
					},
					TLSSecurityProfile: &openshiftconfigv1.TLSSecurityProfile{
						Type: openshiftconfigv1.TLSProfileOldType,
						Old:  &openshiftconfigv1.OldTLSProfile{},
					},
				}

				var v1Security hcov1.SecurityConfig
				convertSecurityV1beta1ToV1(v1beta1Spec, &v1Security)

				Expect(v1Security.CertConfig.CA.Duration).To(HaveValue(Equal(metav1.Duration{Duration: 48 * time.Hour})))
				Expect(v1Security.TLSSecurityProfile).ToNot(BeNil())
				Expect(v1Security.TLSSecurityProfile.Type).To(Equal(openshiftconfigv1.TLSProfileOldType))
			})
		})

		Context("round-trip", func() {
			It("should preserve security config through v1beta1 => v1 => v1beta1", func() {
				original := HyperConvergedSpec{
					CertConfig: hcov1.HyperConvergedCertConfig{
						CA: hcov1.CertRotateConfigCA{
							Duration:    new(metav1.Duration{Duration: 48 * time.Hour}),
							RenewBefore: new(metav1.Duration{Duration: 24 * time.Hour}),
						},
						Server: hcov1.CertRotateConfigServer{
							Duration:    new(metav1.Duration{Duration: 24 * time.Hour}),
							RenewBefore: new(metav1.Duration{Duration: 12 * time.Hour}),
						},
					},
					TLSSecurityProfile: &openshiftconfigv1.TLSSecurityProfile{
						Type:         openshiftconfigv1.TLSProfileIntermediateType,
						Intermediate: &openshiftconfigv1.IntermediateTLSProfile{},
					},
				}

				var v1Security hcov1.SecurityConfig
				convertSecurityV1beta1ToV1(original, &v1Security)

				var result HyperConvergedSpec
				convertSecurityV1ToV1beta1(v1Security, &result)

				Expect(result.CertConfig.CA.Duration).To(Equal(original.CertConfig.CA.Duration))
				Expect(result.CertConfig.CA.RenewBefore).To(Equal(original.CertConfig.CA.RenewBefore))
				Expect(result.CertConfig.Server.Duration).To(Equal(original.CertConfig.Server.Duration))
				Expect(result.CertConfig.Server.RenewBefore).To(Equal(original.CertConfig.Server.RenewBefore))
				Expect(result.TLSSecurityProfile).ToNot(BeNil())
				Expect(result.TLSSecurityProfile.Type).To(Equal(original.TLSSecurityProfile.Type))
			})

			It("should preserve security config through v1 => v1beta1 => v1", func() {
				original := hcov1.SecurityConfig{
					CertConfig: hcov1.HyperConvergedCertConfig{
						CA: hcov1.CertRotateConfigCA{
							Duration:    new(metav1.Duration{Duration: 48 * time.Hour}),
							RenewBefore: new(metav1.Duration{Duration: 24 * time.Hour}),
						},
						Server: hcov1.CertRotateConfigServer{
							Duration:    new(metav1.Duration{Duration: 24 * time.Hour}),
							RenewBefore: new(metav1.Duration{Duration: 12 * time.Hour}),
						},
					},
					TLSSecurityProfile: &openshiftconfigv1.TLSSecurityProfile{
						Type: openshiftconfigv1.TLSProfileOldType,
						Old:  &openshiftconfigv1.OldTLSProfile{},
					},
				}

				var v1beta1Spec HyperConvergedSpec
				convertSecurityV1ToV1beta1(original, &v1beta1Spec)

				var result hcov1.SecurityConfig
				convertSecurityV1beta1ToV1(v1beta1Spec, &result)

				Expect(result.CertConfig.CA.Duration).To(Equal(original.CertConfig.CA.Duration))
				Expect(result.CertConfig.CA.RenewBefore).To(Equal(original.CertConfig.CA.RenewBefore))
				Expect(result.CertConfig.Server.Duration).To(Equal(original.CertConfig.Server.Duration))
				Expect(result.CertConfig.Server.RenewBefore).To(Equal(original.CertConfig.Server.RenewBefore))
				Expect(result.TLSSecurityProfile).ToNot(BeNil())
				Expect(result.TLSSecurityProfile.Type).To(Equal(original.TLSSecurityProfile.Type))
			})
		})
	})

	Context("Deployment conversion", func() {
		Context("v1 ==> v1beta1", func() {
			It("should convert UninstallStrategy", func() {
				v1Config := hcov1.DeploymentConfig{
					UninstallStrategy: hcov1.HyperConvergedUninstallStrategyRemoveWorkloads,
				}

				var v1beta1Spec HyperConvergedSpec
				Expect(convertDeploymentV1ToV1beta1(v1Config, &v1beta1Spec)).To(Succeed())

				Expect(v1beta1Spec.UninstallStrategy).To(Equal(hcov1.HyperConvergedUninstallStrategyRemoveWorkloads))
			})

			It("should convert LogVerbosityConfig", func() {
				v1Config := hcov1.DeploymentConfig{
					LogVerbosityConfig: &hcov1.LogVerbosityConfiguration{
						Kubevirt: &kubevirtv1.LogVerbosity{
							VirtAPI:        3,
							VirtController: 2,
						},
					},
				}

				var v1beta1Spec HyperConvergedSpec
				Expect(convertDeploymentV1ToV1beta1(v1Config, &v1beta1Spec)).To(Succeed())

				Expect(v1beta1Spec.LogVerbosityConfig).ToNot(BeNil())
				Expect(v1beta1Spec.LogVerbosityConfig.Kubevirt).ToNot(BeNil())
				Expect(v1beta1Spec.LogVerbosityConfig.Kubevirt.VirtAPI).To(Equal(uint(3)))
				Expect(v1beta1Spec.LogVerbosityConfig.Kubevirt.VirtController).To(Equal(uint(2)))
			})

			It("should not convert LogVerbosityConfig when nil", func() {
				v1Config := hcov1.DeploymentConfig{}

				var v1beta1Spec HyperConvergedSpec
				Expect(convertDeploymentV1ToV1beta1(v1Config, &v1beta1Spec)).To(Succeed())

				Expect(v1beta1Spec.LogVerbosityConfig).To(BeNil())
			})

			It("should convert DeployVMConsoleProxy", func() {
				v1Config := hcov1.DeploymentConfig{
					DeployVMConsoleProxy: new(true),
				}

				var v1beta1Spec HyperConvergedSpec
				Expect(convertDeploymentV1ToV1beta1(v1Config, &v1beta1Spec)).To(Succeed())

				Expect(v1beta1Spec.DeployVMConsoleProxy).To(HaveValue(BeTrue()))
			})

			It("should not convert DeployVMConsoleProxy when nil", func() {
				v1Config := hcov1.DeploymentConfig{}

				var v1beta1Spec HyperConvergedSpec
				Expect(convertDeploymentV1ToV1beta1(v1Config, &v1beta1Spec)).To(Succeed())

				Expect(v1beta1Spec.DeployVMConsoleProxy).To(BeNil())
			})

			It("should convert ApplicationAwareConfig with Enable=true", func() {
				v1Config := hcov1.DeploymentConfig{
					ApplicationAwareConfig: &hcov1.ApplicationAwareConfigurations{
						Enable: new(true),
						AllowApplicationAwareClusterResourceQuota: true,
					},
				}

				var v1beta1Spec HyperConvergedSpec
				Expect(convertDeploymentV1ToV1beta1(v1Config, &v1beta1Spec)).To(Succeed())

				Expect(v1beta1Spec.ApplicationAwareConfig).ToNot(BeNil())
				Expect(v1beta1Spec.ApplicationAwareConfig.AllowApplicationAwareClusterResourceQuota).To(BeTrue())
				Expect(v1beta1Spec.EnableApplicationAwareQuota).To(HaveValue(BeTrue()))
			})

			It("should convert ApplicationAwareConfig with Enable=false", func() {
				v1Config := hcov1.DeploymentConfig{
					ApplicationAwareConfig: &hcov1.ApplicationAwareConfigurations{
						Enable: new(false),
					},
				}

				var v1beta1Spec HyperConvergedSpec
				Expect(convertDeploymentV1ToV1beta1(v1Config, &v1beta1Spec)).To(Succeed())

				Expect(v1beta1Spec.ApplicationAwareConfig).ToNot(BeNil())
				Expect(v1beta1Spec.EnableApplicationAwareQuota).To(HaveValue(BeFalse()))
			})

			It("should convert ApplicationAwareConfig with Enable=nil", func() {
				v1Config := hcov1.DeploymentConfig{
					ApplicationAwareConfig: &hcov1.ApplicationAwareConfigurations{},
				}

				var v1beta1Spec HyperConvergedSpec
				Expect(convertDeploymentV1ToV1beta1(v1Config, &v1beta1Spec)).To(Succeed())

				Expect(v1beta1Spec.ApplicationAwareConfig).ToNot(BeNil())
				Expect(v1beta1Spec.EnableApplicationAwareQuota).To(BeNil())
			})

			It("should not convert ApplicationAwareConfig when nil", func() {
				v1Config := hcov1.DeploymentConfig{}

				var v1beta1Spec HyperConvergedSpec
				Expect(convertDeploymentV1ToV1beta1(v1Config, &v1beta1Spec)).To(Succeed())

				Expect(v1beta1Spec.ApplicationAwareConfig).To(BeNil())
				Expect(v1beta1Spec.EnableApplicationAwareQuota).To(BeNil())
			})

			It("should convert all fields together", func() {
				v1Config := hcov1.DeploymentConfig{
					UninstallStrategy: hcov1.HyperConvergedUninstallStrategyRemoveWorkloads,
					LogVerbosityConfig: &hcov1.LogVerbosityConfiguration{
						Kubevirt: &kubevirtv1.LogVerbosity{
							VirtAPI: 5,
						},
					},
					ApplicationAwareConfig: &hcov1.ApplicationAwareConfigurations{
						Enable: new(true),
					},
					DeployVMConsoleProxy: new(true),
				}

				var v1beta1Spec HyperConvergedSpec
				Expect(convertDeploymentV1ToV1beta1(v1Config, &v1beta1Spec)).To(Succeed())

				Expect(v1beta1Spec.UninstallStrategy).To(Equal(hcov1.HyperConvergedUninstallStrategyRemoveWorkloads))
				Expect(v1beta1Spec.LogVerbosityConfig).ToNot(BeNil())
				Expect(v1beta1Spec.ApplicationAwareConfig).ToNot(BeNil())
				Expect(v1beta1Spec.EnableApplicationAwareQuota).To(HaveValue(BeTrue()))
				Expect(v1beta1Spec.DeployVMConsoleProxy).To(HaveValue(BeTrue()))
			})
		})

		Context("v1beta1 ==> v1", func() {
			It("should convert UninstallStrategy", func() {
				v1beta1Spec := HyperConvergedSpec{
					UninstallStrategy: hcov1.HyperConvergedUninstallStrategyRemoveWorkloads,
				}

				var v1Config hcov1.DeploymentConfig
				Expect(convertDeploymentV1beta1ToV1(v1beta1Spec, &v1Config)).To(Succeed())

				Expect(v1Config.UninstallStrategy).To(Equal(hcov1.HyperConvergedUninstallStrategyRemoveWorkloads))
			})

			It("should convert LogVerbosityConfig", func() {
				v1beta1Spec := HyperConvergedSpec{
					LogVerbosityConfig: &hcov1.LogVerbosityConfiguration{
						Kubevirt: &kubevirtv1.LogVerbosity{
							VirtAPI:        3,
							VirtController: 2,
						},
					},
				}

				var v1Config hcov1.DeploymentConfig
				Expect(convertDeploymentV1beta1ToV1(v1beta1Spec, &v1Config)).To(Succeed())

				Expect(v1Config.LogVerbosityConfig).ToNot(BeNil())
				Expect(v1Config.LogVerbosityConfig.Kubevirt).ToNot(BeNil())
				Expect(v1Config.LogVerbosityConfig.Kubevirt.VirtAPI).To(Equal(uint(3)))
				Expect(v1Config.LogVerbosityConfig.Kubevirt.VirtController).To(Equal(uint(2)))
			})

			It("should not convert LogVerbosityConfig when nil", func() {
				v1beta1Spec := HyperConvergedSpec{}

				var v1Config hcov1.DeploymentConfig
				Expect(convertDeploymentV1beta1ToV1(v1beta1Spec, &v1Config)).To(Succeed())

				Expect(v1Config.LogVerbosityConfig).To(BeNil())
			})

			It("should convert ApplicationAwareConfig", func() {
				v1beta1Spec := HyperConvergedSpec{
					ApplicationAwareConfig: &ApplicationAwareConfigurations{
						AllowApplicationAwareClusterResourceQuota: true,
					},
				}

				var v1Config hcov1.DeploymentConfig
				Expect(convertDeploymentV1beta1ToV1(v1beta1Spec, &v1Config)).To(Succeed())

				Expect(v1Config.ApplicationAwareConfig).ToNot(BeNil())
				Expect(v1Config.ApplicationAwareConfig.AllowApplicationAwareClusterResourceQuota).To(BeTrue())
			})

			It("should not convert ApplicationAwareConfig when nil", func() {
				v1beta1Spec := HyperConvergedSpec{}

				var v1Config hcov1.DeploymentConfig
				Expect(convertDeploymentV1beta1ToV1(v1beta1Spec, &v1Config)).To(Succeed())

				Expect(v1Config.ApplicationAwareConfig).To(BeNil())
			})

			It("should set Enable=true when EnableApplicationAwareQuota is true and ApplicationAwareConfig is set", func() {
				v1beta1Spec := HyperConvergedSpec{
					ApplicationAwareConfig:      &ApplicationAwareConfigurations{},
					EnableApplicationAwareQuota: new(true),
				}

				var v1Config hcov1.DeploymentConfig
				Expect(convertDeploymentV1beta1ToV1(v1beta1Spec, &v1Config)).To(Succeed())

				Expect(v1Config.ApplicationAwareConfig).ToNot(BeNil())
				Expect(v1Config.ApplicationAwareConfig.Enable).To(HaveValue(BeTrue()))
			})

			It("should set Enable=true when EnableApplicationAwareQuota is true and ApplicationAwareConfig is nil", func() {
				v1beta1Spec := HyperConvergedSpec{
					EnableApplicationAwareQuota: new(true),
				}

				var v1Config hcov1.DeploymentConfig
				Expect(convertDeploymentV1beta1ToV1(v1beta1Spec, &v1Config)).To(Succeed())

				Expect(v1Config.ApplicationAwareConfig).ToNot(BeNil())
				Expect(v1Config.ApplicationAwareConfig.Enable).To(HaveValue(BeTrue()))
			})

			It("should set Enable=false when EnableApplicationAwareQuota is false", func() {
				v1beta1Spec := HyperConvergedSpec{
					EnableApplicationAwareQuota: new(false),
				}

				var v1Config hcov1.DeploymentConfig
				Expect(convertDeploymentV1beta1ToV1(v1beta1Spec, &v1Config)).To(Succeed())

				Expect(v1Config.ApplicationAwareConfig).ToNot(BeNil())
				Expect(v1Config.ApplicationAwareConfig.Enable).To(HaveValue(BeFalse()))
			})

			It("should not set ApplicationAwareConfig when both ApplicationAwareConfig and EnableApplicationAwareQuota are nil", func() {
				v1beta1Spec := HyperConvergedSpec{}

				var v1Config hcov1.DeploymentConfig
				Expect(convertDeploymentV1beta1ToV1(v1beta1Spec, &v1Config)).To(Succeed())

				Expect(v1Config.ApplicationAwareConfig).To(BeNil())
			})

			It("should convert all fields together", func() {
				v1beta1Spec := HyperConvergedSpec{
					UninstallStrategy: hcov1.HyperConvergedUninstallStrategyRemoveWorkloads,
					LogVerbosityConfig: &hcov1.LogVerbosityConfiguration{
						Kubevirt: &kubevirtv1.LogVerbosity{
							VirtAPI: 5,
						},
					},
					ApplicationAwareConfig:      &ApplicationAwareConfigurations{},
					EnableApplicationAwareQuota: new(true),
				}

				var v1Config hcov1.DeploymentConfig
				Expect(convertDeploymentV1beta1ToV1(v1beta1Spec, &v1Config)).To(Succeed())

				Expect(v1Config.UninstallStrategy).To(Equal(hcov1.HyperConvergedUninstallStrategyRemoveWorkloads))
				Expect(v1Config.LogVerbosityConfig).ToNot(BeNil())
				Expect(v1Config.ApplicationAwareConfig).ToNot(BeNil())
				Expect(v1Config.ApplicationAwareConfig.Enable).To(HaveValue(BeTrue()))
			})
		})

		Context("round-trip", func() {
			It("should preserve deployment config through v1beta1 => v1 => v1beta1", func() {
				original := HyperConvergedSpec{
					UninstallStrategy: hcov1.HyperConvergedUninstallStrategyRemoveWorkloads,
					LogVerbosityConfig: &hcov1.LogVerbosityConfiguration{
						Kubevirt: &kubevirtv1.LogVerbosity{
							VirtAPI: 5,
						},
					},
					ApplicationAwareConfig:      &ApplicationAwareConfigurations{},
					EnableApplicationAwareQuota: new(true),
				}

				var v1Config hcov1.DeploymentConfig
				Expect(convertDeploymentV1beta1ToV1(original, &v1Config)).To(Succeed())

				var result HyperConvergedSpec
				Expect(convertDeploymentV1ToV1beta1(v1Config, &result)).To(Succeed())

				Expect(result.UninstallStrategy).To(Equal(original.UninstallStrategy))
				Expect(result.LogVerbosityConfig).ToNot(BeNil())
				Expect(result.LogVerbosityConfig.Kubevirt.VirtAPI).To(Equal(original.LogVerbosityConfig.Kubevirt.VirtAPI))
				Expect(result.ApplicationAwareConfig).ToNot(BeNil())
				Expect(result.EnableApplicationAwareQuota).To(HaveValue(BeTrue()))
			})

			It("should preserve deployment config through v1 => v1beta1 => v1", func() {
				original := hcov1.DeploymentConfig{
					UninstallStrategy: hcov1.HyperConvergedUninstallStrategyRemoveWorkloads,
					LogVerbosityConfig: &hcov1.LogVerbosityConfiguration{
						Kubevirt: &kubevirtv1.LogVerbosity{
							VirtAPI: 5,
						},
					},
					ApplicationAwareConfig: &hcov1.ApplicationAwareConfigurations{
						Enable: new(true),
						AllowApplicationAwareClusterResourceQuota: true,
					},
					DeployVMConsoleProxy: new(true),
				}

				var v1beta1Spec HyperConvergedSpec
				Expect(convertDeploymentV1ToV1beta1(original, &v1beta1Spec)).To(Succeed())

				var result hcov1.DeploymentConfig
				Expect(convertDeploymentV1beta1ToV1(v1beta1Spec, &result)).To(Succeed())

				Expect(result.UninstallStrategy).To(Equal(original.UninstallStrategy))
				Expect(result.LogVerbosityConfig).ToNot(BeNil())
				Expect(result.LogVerbosityConfig.Kubevirt.VirtAPI).To(Equal(original.LogVerbosityConfig.Kubevirt.VirtAPI))
				Expect(result.ApplicationAwareConfig).ToNot(BeNil())
				Expect(result.ApplicationAwareConfig.Enable).To(HaveValue(BeTrue()))
				Expect(result.ApplicationAwareConfig.AllowApplicationAwareClusterResourceQuota).To(BeTrue())
			})

			It("should preserve nil fields through round-trip", func() {
				original := HyperConvergedSpec{}

				var v1Config hcov1.DeploymentConfig
				Expect(convertDeploymentV1beta1ToV1(original, &v1Config)).To(Succeed())

				var result HyperConvergedSpec
				Expect(convertDeploymentV1ToV1beta1(v1Config, &result)).To(Succeed())

				Expect(result.LogVerbosityConfig).To(BeNil())
				Expect(result.ApplicationAwareConfig).To(BeNil())
				Expect(result.EnableApplicationAwareQuota).To(BeNil())
				Expect(result.DeployVMConsoleProxy).To(BeNil())
			})
		})
	})

	Context("v1 only fields", func() {
		DescribeTable("should round-trip, keeping the DeployNetworkResourcesInjector field", func(fieldValue bool, expectedAnnotation string) {
			v1HC := getV1HC()
			v1HC.Spec.Deployment.DeployNetworkResourcesInjector = new(fieldValue)
			v1beta1HC := &HyperConverged{}

			Expect(v1beta1HC.ConvertFrom(v1HC)).To(Succeed())
			Expect(v1beta1HC.Annotations).To(HaveKeyWithValue(v1OnlyFieldAnnotation, MatchJSON(expectedAnnotation)))

			roundTripHC := &hcov1.HyperConverged{}
			Expect(v1beta1HC.ConvertTo(roundTripHC)).To(Succeed())

			Expect(roundTripHC.Annotations).ToNot(HaveKey(v1OnlyFieldAnnotation))
			Expect(roundTripHC.Spec.Deployment.DeployNetworkResourcesInjector).To(HaveValue(Equal(fieldValue)))
		},
			Entry("when the field is false", false, `{"deployNetworkResourcesInjector": false}`),
			Entry("when the field is true", true, `{"deployNetworkResourcesInjector": true}`),
		)

		It("should round-trip when DeployNetworkResourcesInjector field is nil", func() {
			v1HC := getV1HC()
			v1HC.Spec.Deployment.DeployNetworkResourcesInjector = nil
			v1beta1HC := &HyperConverged{}

			Expect(v1beta1HC.ConvertFrom(v1HC)).To(Succeed())
			Expect(v1beta1HC.Annotations).ToNot(HaveKey(v1OnlyFieldAnnotation))

			roundTripHC := &hcov1.HyperConverged{}
			Expect(v1beta1HC.ConvertTo(roundTripHC)).To(Succeed())

			Expect(roundTripHC.Annotations).ToNot(HaveKey(v1OnlyFieldAnnotation))
			Expect(roundTripHC.Spec.Deployment.DeployNetworkResourcesInjector).To(BeNil())
		})

		DescribeTable("should round-trip, keeping the MDevConfig.Enable field", func(fieldValue bool, v1FG hcofg.HyperConvergedFeatureGates, expectedAnnotation string) {
			v1HC := getV1HC()
			v1HC.Spec.Deployment.DeployNetworkResourcesInjector = nil
			v1HC.Spec.Virtualization.MediatedDevicesConfiguration = &hcov1.MediatedDevicesConfiguration{Enabled: new(fieldValue)}
			v1HC.Spec.FeatureGates = v1FG

			_, originalExists := v1FG.IsExplicitlyEnabled(DisableMDevConfigurationFG)

			v1beta1HC := &HyperConverged{}
			Expect(v1beta1HC.ConvertFrom(v1HC)).To(Succeed())
			Expect(v1beta1HC.Annotations).To(HaveKeyWithValue(v1OnlyFieldAnnotation, MatchJSON(expectedAnnotation)))

			roundTripHC := &hcov1.HyperConverged{}
			Expect(v1beta1HC.ConvertTo(roundTripHC)).To(Succeed())

			Expect(roundTripHC.Annotations).ToNot(HaveKey(v1OnlyFieldAnnotation))
			Expect(roundTripHC.Spec.Virtualization.MediatedDevicesConfiguration).ToNot(BeNil())
			Expect(roundTripHC.Spec.Virtualization.MediatedDevicesConfiguration.Enabled).To(HaveValue(Equal(fieldValue)))

			enabled, exists := roundTripHC.Spec.FeatureGates.IsExplicitlyEnabled(DisableMDevConfigurationFG)
			Expect(exists).To(Equal(originalExists))
			Expect(enabled).To(Equal(originalExists && !fieldValue))
		},
			Entry("when the field is false", false, nil, `{"mdevConfigEnable": false}`),
			Entry("when the field is true", true, nil, `{"mdevConfigEnable": true}`),
			Entry("when the field is false, and FG is true (implicit)",
				false,
				hcofg.HyperConvergedFeatureGates{{Name: DisableMDevConfigurationFG}},
				`{"mdevConfigEnable": false, "disableMDevConfigurationFG": true}`,
			),
			Entry("when the field is false, and FG is true",
				false,
				hcofg.HyperConvergedFeatureGates{{Name: DisableMDevConfigurationFG, State: new(hcofg.Enabled)}},
				`{"mdevConfigEnable": false, "disableMDevConfigurationFG": true}`,
			),
			Entry("when the field is true, and FG is false",
				true,
				hcofg.HyperConvergedFeatureGates{{Name: DisableMDevConfigurationFG, State: new(hcofg.Disabled)}},
				`{"mdevConfigEnable": true, "disableMDevConfigurationFG": false}`,
			),
			Entry("when the field is true, and FG is true (implicit)",
				true,
				hcofg.HyperConvergedFeatureGates{{Name: DisableMDevConfigurationFG}},
				`{"mdevConfigEnable": true, "disableMDevConfigurationFG": true}`,
			),
			Entry("when the field is true, and FG is true",
				true,
				hcofg.HyperConvergedFeatureGates{{Name: DisableMDevConfigurationFG, State: new(hcofg.Enabled)}},
				`{"mdevConfigEnable": true, "disableMDevConfigurationFG": true}`,
			),
			Entry("when the field is false, and FG is false",
				false,
				hcofg.HyperConvergedFeatureGates{{Name: DisableMDevConfigurationFG, State: new(hcofg.Disabled)}},
				`{"mdevConfigEnable": false, "disableMDevConfigurationFG": false}`,
			),
		)

		It("should round-trip when MDevConfig.Enable field is nil", func() {
			v1HC := getV1HC()
			v1HC.Spec.Deployment.DeployNetworkResourcesInjector = nil
			v1HC.Spec.Virtualization.MediatedDevicesConfiguration = &hcov1.MediatedDevicesConfiguration{}
			v1beta1HC := &HyperConverged{}

			Expect(v1beta1HC.ConvertFrom(v1HC)).To(Succeed())
			Expect(v1beta1HC.Annotations).ToNot(HaveKey(v1OnlyFieldAnnotation))

			roundTripHC := &hcov1.HyperConverged{}
			Expect(v1beta1HC.ConvertTo(roundTripHC)).To(Succeed())

			Expect(roundTripHC.Annotations).ToNot(HaveKey(v1OnlyFieldAnnotation))
			Expect(roundTripHC.Spec.Virtualization.MediatedDevicesConfiguration).ToNot(BeNil())
			Expect(roundTripHC.Spec.Virtualization.MediatedDevicesConfiguration.Enabled).To(BeNil())
		})

		DescribeTable("should round-trip, keeping the disableMDevConfiguration feature gate", func(fgs hcofg.HyperConvergedFeatureGates, expectedAnnotation gomegatypes.GomegaMatcher, isEnabled, isFound bool) {
			v1HC := getV1HC()
			v1HC.Spec.FeatureGates = fgs
			v1HC.Spec.Deployment.DeployNetworkResourcesInjector = nil

			v1beta1HC := &HyperConverged{}
			Expect(v1beta1HC.ConvertFrom(v1HC)).To(Succeed())
			Expect(v1beta1HC.Annotations).To(expectedAnnotation)

			roundTripHC := &hcov1.HyperConverged{}
			Expect(v1beta1HC.ConvertTo(roundTripHC)).To(Succeed())

			Expect(roundTripHC.Annotations).ToNot(HaveKey(v1OnlyFieldAnnotation))
			enabled, found := roundTripHC.Spec.FeatureGates.IsExplicitlyEnabled(DisableMDevConfigurationFG)
			Expect(enabled).To(Equal(isEnabled))
			Expect(found).To(Equal(isFound))
		},
			Entry("when FG list is nil", nil, Not(HaveKey(v1OnlyFieldAnnotation)), false, false),
			Entry("when FG list is empty", hcofg.HyperConvergedFeatureGates{}, Not(HaveKey(v1OnlyFieldAnnotation)), false, false),
			Entry("when the disableMDevConfiguration FG is not set",
				hcofg.HyperConvergedFeatureGates{{Name: "somethingElse"}},
				Not(HaveKey(v1OnlyFieldAnnotation)),
				false,
				false,
			),
			Entry("when the disableMDevConfiguration FG is implicitly enabled",
				hcofg.HyperConvergedFeatureGates{{Name: DisableMDevConfigurationFG}}, // `{"deployNetworkResourcesInjector": false}`,
				HaveKeyWithValue(v1OnlyFieldAnnotation, MatchJSON(`{"disableMDevConfigurationFG": true}`)),
				true,
				true,
			),
			Entry("when the disableMDevConfiguration FG is explicitly enabled",
				hcofg.HyperConvergedFeatureGates{{Name: DisableMDevConfigurationFG, State: new(hcofg.Enabled)}},
				HaveKeyWithValue(v1OnlyFieldAnnotation, MatchJSON(`{"disableMDevConfigurationFG": true}`)),
				true,
				true,
			),
			Entry("when the disableMDevConfiguration FG is disabled",
				hcofg.HyperConvergedFeatureGates{{Name: DisableMDevConfigurationFG, State: new(hcofg.Disabled)}},
				HaveKeyWithValue(v1OnlyFieldAnnotation, MatchJSON(`{"disableMDevConfigurationFG": false}`)),
				false,
				true,
			),
		)

		// keep this up-to-date, and the last test case in this context
		It("should round-trip, keeping v1-only fields", func() {
			v1HC := getV1HC()
			// set the spec with non-default values
			v1HC.Spec.Deployment.DeployNetworkResourcesInjector = new(false)
			v1HC.Spec.Virtualization.MediatedDevicesConfiguration = &hcov1.MediatedDevicesConfiguration{
				Enabled: new(false),
			}
			v1HC.Spec.FeatureGates.Enable(DisableMDevConfigurationFG)
			v1beta1HC := &HyperConverged{}

			Expect(v1beta1HC.ConvertFrom(v1HC)).To(Succeed())
			const expectedJSONAnnotation = `{
	"deployNetworkResourcesInjector": false,
	"mdevConfigEnable": false,
	"disableMDevConfigurationFG": true
}`
			Expect(v1beta1HC.Annotations).To(HaveKeyWithValue(v1OnlyFieldAnnotation, MatchJSON(expectedJSONAnnotation)))

			roundTripHC := &hcov1.HyperConverged{}
			Expect(v1beta1HC.ConvertTo(roundTripHC)).To(Succeed())

			Expect(roundTripHC.Annotations).ToNot(HaveKey(v1OnlyFieldAnnotation))
			Expect(roundTripHC.Spec.Deployment.DeployNetworkResourcesInjector).To(HaveValue(BeFalse()))
			Expect(roundTripHC.Spec.Virtualization.MediatedDevicesConfiguration).ToNot(BeNil())
			Expect(roundTripHC.Spec.Virtualization.MediatedDevicesConfiguration.Enabled).To(HaveValue(BeFalse()))
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

// Using a plain go test here instead of ginkgo, because DescribeTable does not support type parameters.
func TestSetPtr(t *testing.T) {
	type mytype string

	srcs := map[string]any{
		"test string":                 "testing",
		"test int":                    42,
		"test uint64":                 uint64(42),
		"test float64":                float64(42),
		"test boolean (true)":         true,
		"test boolean (false)":        false,
		"test custom comparable type": mytype("custom"),
	}

	for testName, value := range srcs {
		t.Run(testName, func(t *testing.T) {
			singleSetPtrTest(NewWithT(t), value)
		})
	}
}

func singleSetPtrTest[V comparable](g Gomega, value V) {
	var src *V
	g.Expect(setPtr(src)).To(BeNil())

	src = &value
	g.Expect(setPtr(src)).To(HaveValue(Equal(value)))
}
