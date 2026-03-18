package v1beta1

import (
	"slices"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	kubevirtv1 "kubevirt.io/api/core/v1"
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

				Expect(v1Spec.NodePlacements).ToNot(BeNil())
				Expect(v1Spec.NodePlacements.Infra).ToNot(BeNil())
				Expect(v1Spec.NodePlacements.Infra.NodeSelector).To(Equal(map[string]string{"infra-key": "infra-val"}))
				Expect(v1Spec.NodePlacements.Workload).ToNot(BeNil())
				Expect(v1Spec.NodePlacements.Workload.NodeSelector).To(Equal(map[string]string{"workload-key": "workload-val"}))
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

				Expect(v1Spec.NodePlacements).ToNot(BeNil())
				Expect(v1Spec.NodePlacements.Infra).ToNot(BeNil())
				Expect(v1Spec.NodePlacements.Workload).To(BeNil())
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

				Expect(v1Spec.NodePlacements).ToNot(BeNil())
				Expect(v1Spec.NodePlacements.Infra).To(BeNil())
				Expect(v1Spec.NodePlacements.Workload).ToNot(BeNil())
			})

			It("should not set NodePlacements when both are nil", func() {
				v1beta1Spec := HyperConvergedSpec{}
				v1Spec := &hcov1.HyperConvergedSpec{}

				convertNodePlacementsV1beta1ToV1(v1beta1Spec, v1Spec)

				Expect(v1Spec.NodePlacements).To(BeNil())
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
				Expect(v1Spec.NodePlacements).ToNot(BeNil())
				Expect(v1Spec.NodePlacements.Infra).ToNot(BeNil())
				Expect(v1Spec.NodePlacements.Infra.Affinity).To(Equal(infraAffinity))

				Expect(v1Spec.NodePlacements.Workload).ToNot(BeNil())
				Expect(v1Spec.NodePlacements.Workload.Affinity).To(Equal(workloadAffinity))
			})
		})

		Context("v1 to v1beta1", func() {
			It("should convert both infra and workload node placements", func() {
				v1Spec := hcov1.HyperConvergedSpec{
					NodePlacements: &hcov1.NodePlacements{
						Infra: &sdkapi.NodePlacement{
							NodeSelector: map[string]string{"infra-key": "infra-val"},
						},
						Workload: &sdkapi.NodePlacement{
							NodeSelector: map[string]string{"workload-key": "workload-val"},
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
					NodePlacements: &hcov1.NodePlacements{
						Infra: &sdkapi.NodePlacement{
							NodeSelector: map[string]string{"infra-key": "infra-val"},
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
					NodePlacements: &hcov1.NodePlacements{
						Infra: &sdkapi.NodePlacement{
							Affinity: infraAffinity,
						},
						Workload: &sdkapi.NodePlacement{
							Affinity: workloadAffinity,
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
					NodePlacements: &hcov1.NodePlacements{
						Infra: &sdkapi.NodePlacement{
							NodeSelector: map[string]string{"infra-key": "infra-val"},
						},
						Workload: &sdkapi.NodePlacement{
							NodeSelector: map[string]string{"workload-key": "workload-val"},
						},
					},
				}

				v1beta1Spec := &HyperConvergedSpec{}
				convertNodePlacementsV1ToV1beta1(original, v1beta1Spec)

				result := &hcov1.HyperConvergedSpec{}
				convertNodePlacementsV1beta1ToV1(*v1beta1Spec, result)

				Expect(result.NodePlacements.Infra.NodeSelector).To(Equal(original.NodePlacements.Infra.NodeSelector))
				Expect(result.NodePlacements.Workload.NodeSelector).To(Equal(original.NodePlacements.Workload.NodeSelector))
			})
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

	Context("virtualization", func() {
		Context("v1 ==> v1beta2", func() {
			It("should convert LiveMigrationConfig", func() {
				v1VirtConfig := hcov1.VirtualizationConfig{
					LiveMigrationConfig: hcov1.LiveMigrationConfigurations{
						ParallelMigrationsPerCluster:      ptr.To(uint32(10)),
						ParallelOutboundMigrationsPerNode: ptr.To(uint32(4)),
						BandwidthPerMigration:             ptr.To("1Gi"),
						CompletionTimeoutPerGiB:           ptr.To(int64(300)),
						ProgressTimeout:                   ptr.To(int64(200)),
						AllowAutoConverge:                 ptr.To(true),
						AllowPostCopy:                     ptr.To(false),
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
						BatchEvictionSize:     ptr.To(5),
						BatchEvictionInterval: ptr.To(metav1.Duration{Duration: 30000000000}),
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
					EvictionStrategy: ptr.To(kubevirtv1.EvictionStrategyLiveMigrate),
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
						DisableFreePageReporting: ptr.To(true),
						DisableSerialConsoleLog:  ptr.To(false),
						DefaultCPUModel:          ptr.To("Haswell"),
						DefaultRuntimeClass:      ptr.To("my-runtime-class"),
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
					VmiCPUAllocationRatio: ptr.To(10),
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
					VmiCPUAllocationRatio: ptr.To(5),
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
			It("should convert LiveMigrationConfig", func() {
				v1beta1Spec := HyperConvergedSpec{
					LiveMigrationConfig: hcov1.LiveMigrationConfigurations{
						ParallelMigrationsPerCluster:      ptr.To(uint32(10)),
						ParallelOutboundMigrationsPerNode: ptr.To(uint32(4)),
						BandwidthPerMigration:             ptr.To("1Gi"),
						CompletionTimeoutPerGiB:           ptr.To(int64(300)),
						ProgressTimeout:                   ptr.To(int64(200)),
						AllowAutoConverge:                 ptr.To(true),
						AllowPostCopy:                     ptr.To(false),
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
						BatchEvictionSize:     ptr.To(5),
						BatchEvictionInterval: ptr.To(metav1.Duration{Duration: 30000000000}),
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
					EvictionStrategy: ptr.To(kubevirtv1.EvictionStrategyLiveMigrate),
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
						DisableFreePageReporting: ptr.To(true),
						DisableSerialConsoleLog:  ptr.To(false),
					},
					DefaultCPUModel:     ptr.To("Haswell"),
					DefaultRuntimeClass: ptr.To("my-runtime-class"),
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
						VmiCPUAllocationRatio: ptr.To(10),
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
						VmiCPUAllocationRatio: ptr.To(5),
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
