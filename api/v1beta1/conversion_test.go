package v1beta1

import (
	"slices"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	openshiftconfigv1 "github.com/openshift/api/config/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

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

	Context("Storage conversion", func() {
		Context("v1 ==> v1beta1", func() {
			It("should convert VMStateStorageClass", func() {
				v1Storage := &hcov1.StorageConfig{
					VMStateStorageClass: ptr.To("my-storage-class"),
				}

				var v1beta1Spec HyperConvergedSpec
				convertStorageV1ToV1beta1(v1Storage, &v1beta1Spec)

				Expect(v1beta1Spec.VMStateStorageClass).To(HaveValue(Equal("my-storage-class")))
			})

			It("should convert ScratchSpaceStorageClass", func() {
				v1Storage := &hcov1.StorageConfig{
					ScratchSpaceStorageClass: ptr.To("scratch-class"),
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
					VMStateStorageClass:      ptr.To("vm-state-class"),
					ScratchSpaceStorageClass: ptr.To("scratch-class"),
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
					VMStateStorageClass: ptr.To("my-storage-class"),
				}

				v1Storage := convertStorageV1beta1ToV1(v1beta1Spec)

				Expect(v1Storage).ToNot(BeNil())
				Expect(v1Storage.VMStateStorageClass).To(HaveValue(Equal("my-storage-class")))
			})

			It("should convert ScratchSpaceStorageClass", func() {
				v1beta1Spec := HyperConvergedSpec{
					ScratchSpaceStorageClass: ptr.To("scratch-class"),
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
					VMStateStorageClass:      ptr.To("vm-state-class"),
					ScratchSpaceStorageClass: ptr.To("scratch-class"),
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
					VMStateStorageClass:      ptr.To("vm-state-class"),
					ScratchSpaceStorageClass: ptr.To("scratch-class"),
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
					VMStateStorageClass:      ptr.To("vm-state-class"),
					ScratchSpaceStorageClass: ptr.To("scratch-class"),
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
					KubeSecondaryDNSNameServerIP: ptr.To("192.168.1.1"),
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
						RangeStart: ptr.To("02:00:00:00:00:00"),
						RangeEnd:   ptr.To("02:FF:FF:FF:FF:FF"),
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
					KubeSecondaryDNSNameServerIP: ptr.To("10.0.0.1"),
					KubeMacPoolConfiguration: &hcov1.KubeMacPoolConfig{
						RangeStart: ptr.To("02:00:00:00:00:00"),
						RangeEnd:   ptr.To("02:FF:FF:FF:FF:FF"),
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
					KubeSecondaryDNSNameServerIP: ptr.To("192.168.1.1"),
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
						RangeStart: ptr.To("02:00:00:00:00:00"),
						RangeEnd:   ptr.To("02:FF:FF:FF:FF:FF"),
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
					KubeSecondaryDNSNameServerIP: ptr.To("10.0.0.1"),
					KubeMacPoolConfiguration: &hcov1.KubeMacPoolConfig{
						RangeStart: ptr.To("02:00:00:00:00:00"),
						RangeEnd:   ptr.To("02:FF:FF:FF:FF:FF"),
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
					KubeSecondaryDNSNameServerIP: ptr.To("10.0.0.1"),
					KubeMacPoolConfiguration: &hcov1.KubeMacPoolConfig{
						RangeStart: ptr.To("02:00:00:00:00:00"),
						RangeEnd:   ptr.To("02:FF:FF:FF:FF:FF"),
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
					KubeSecondaryDNSNameServerIP: ptr.To("10.0.0.1"),
					KubeMacPoolConfiguration: &hcov1.KubeMacPoolConfig{
						RangeStart: ptr.To("02:00:00:00:00:00"),
						RangeEnd:   ptr.To("02:FF:FF:FF:FF:FF"),
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
					CommonTemplatesNamespace: ptr.To("my-ns"),
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
					CommonBootImageNamespace: ptr.To("boot-ns"),
				}

				var v1beta1Spec HyperConvergedSpec
				convertWorkloadSourcesV1ToV1beta1(v1Config, &v1beta1Spec)

				Expect(v1beta1Spec.CommonBootImageNamespace).To(HaveValue(Equal("boot-ns")))
				Expect(v1beta1Spec.CommonTemplatesNamespace).To(BeNil())
			})

			It("should convert EnableCommonBootImageImport", func() {
				v1Config := hcov1.WorkloadSourcesConfig{
					EnableCommonBootImageImport: ptr.To(true),
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
						Enabled: ptr.To(true),
					},
				}

				var v1beta1Spec HyperConvergedSpec
				convertWorkloadSourcesV1ToV1beta1(v1Config, &v1beta1Spec)

				Expect(v1beta1Spec.CommonInstancetypesDeployment).ToNot(BeNil())
				Expect(v1beta1Spec.CommonInstancetypesDeployment.Enabled).To(HaveValue(BeTrue()))
			})

			It("should convert all fields together", func() {
				v1Config := hcov1.WorkloadSourcesConfig{
					CommonTemplatesNamespace:    ptr.To("templates-ns"),
					CommonBootImageNamespace:    ptr.To("boot-ns"),
					EnableCommonBootImageImport: ptr.To(false),
					DataImportCronTemplates: []hcov1.DataImportCronTemplate{
						{ObjectMeta: metav1.ObjectMeta{Name: "tmpl1"}},
					},
					InstancetypeConfig: &kubevirtv1.InstancetypeConfiguration{},
					CommonInstancetypesDeployment: &kubevirtv1.CommonInstancetypesDeployment{
						Enabled: ptr.To(true),
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
					CommonTemplatesNamespace: ptr.To("my-ns"),
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
					CommonBootImageNamespace: ptr.To("boot-ns"),
				}

				var v1Config hcov1.WorkloadSourcesConfig
				convertWorkloadSourcesV1beta1ToV1(v1beta1Spec, &v1Config)

				Expect(v1Config.CommonBootImageNamespace).To(HaveValue(Equal("boot-ns")))
				Expect(v1Config.CommonTemplatesNamespace).To(BeNil())
			})

			It("should convert EnableCommonBootImageImport", func() {
				v1beta1Spec := HyperConvergedSpec{
					EnableCommonBootImageImport: ptr.To(true),
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
						Enabled: ptr.To(true),
					},
				}

				var v1Config hcov1.WorkloadSourcesConfig
				convertWorkloadSourcesV1beta1ToV1(v1beta1Spec, &v1Config)

				Expect(v1Config.CommonInstancetypesDeployment).ToNot(BeNil())
				Expect(v1Config.CommonInstancetypesDeployment.Enabled).To(HaveValue(BeTrue()))
			})

			It("should convert all fields together", func() {
				v1beta1Spec := HyperConvergedSpec{
					CommonTemplatesNamespace:    ptr.To("templates-ns"),
					CommonBootImageNamespace:    ptr.To("boot-ns"),
					EnableCommonBootImageImport: ptr.To(false),
					DataImportCronTemplates: []hcov1.DataImportCronTemplate{
						{ObjectMeta: metav1.ObjectMeta{Name: "tmpl1"}},
					},
					InstancetypeConfig: &kubevirtv1.InstancetypeConfiguration{},
					CommonInstancetypesDeployment: &kubevirtv1.CommonInstancetypesDeployment{
						Enabled: ptr.To(true),
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
					CommonTemplatesNamespace:    ptr.To("templates-ns"),
					CommonBootImageNamespace:    ptr.To("boot-ns"),
					EnableCommonBootImageImport: ptr.To(true),
					DataImportCronTemplates: []hcov1.DataImportCronTemplate{
						{ObjectMeta: metav1.ObjectMeta{Name: "tmpl1"}},
					},
					InstancetypeConfig: &kubevirtv1.InstancetypeConfiguration{},
					CommonInstancetypesDeployment: &kubevirtv1.CommonInstancetypesDeployment{
						Enabled: ptr.To(true),
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
					CommonTemplatesNamespace:    ptr.To("templates-ns"),
					CommonBootImageNamespace:    ptr.To("boot-ns"),
					EnableCommonBootImageImport: ptr.To(true),
					DataImportCronTemplates: []hcov1.DataImportCronTemplate{
						{ObjectMeta: metav1.ObjectMeta{Name: "tmpl1"}},
					},
					InstancetypeConfig: &kubevirtv1.InstancetypeConfiguration{},
					CommonInstancetypesDeployment: &kubevirtv1.CommonInstancetypesDeployment{
						Enabled: ptr.To(true),
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
							Duration:    ptr.To(metav1.Duration{Duration: 48 * time.Hour}),
							RenewBefore: ptr.To(metav1.Duration{Duration: 24 * time.Hour}),
						},
						Server: hcov1.CertRotateConfigServer{
							Duration:    ptr.To(metav1.Duration{Duration: 24 * time.Hour}),
							RenewBefore: ptr.To(metav1.Duration{Duration: 12 * time.Hour}),
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
							Duration:    ptr.To(metav1.Duration{Duration: 48 * time.Hour}),
							RenewBefore: ptr.To(metav1.Duration{Duration: 24 * time.Hour}),
						},
						Server: hcov1.CertRotateConfigServer{
							Duration:    ptr.To(metav1.Duration{Duration: 24 * time.Hour}),
							RenewBefore: ptr.To(metav1.Duration{Duration: 12 * time.Hour}),
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
							Duration:    ptr.To(metav1.Duration{Duration: 48 * time.Hour}),
							RenewBefore: ptr.To(metav1.Duration{Duration: 24 * time.Hour}),
						},
						Server: hcov1.CertRotateConfigServer{
							Duration:    ptr.To(metav1.Duration{Duration: 24 * time.Hour}),
							RenewBefore: ptr.To(metav1.Duration{Duration: 12 * time.Hour}),
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
							Duration:    ptr.To(metav1.Duration{Duration: 48 * time.Hour}),
							RenewBefore: ptr.To(metav1.Duration{Duration: 24 * time.Hour}),
						},
						Server: hcov1.CertRotateConfigServer{
							Duration:    ptr.To(metav1.Duration{Duration: 24 * time.Hour}),
							RenewBefore: ptr.To(metav1.Duration{Duration: 12 * time.Hour}),
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
							Duration:    ptr.To(metav1.Duration{Duration: 48 * time.Hour}),
							RenewBefore: ptr.To(metav1.Duration{Duration: 24 * time.Hour}),
						},
						Server: hcov1.CertRotateConfigServer{
							Duration:    ptr.To(metav1.Duration{Duration: 24 * time.Hour}),
							RenewBefore: ptr.To(metav1.Duration{Duration: 12 * time.Hour}),
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
							Duration:    ptr.To(metav1.Duration{Duration: 48 * time.Hour}),
							RenewBefore: ptr.To(metav1.Duration{Duration: 24 * time.Hour}),
						},
						Server: hcov1.CertRotateConfigServer{
							Duration:    ptr.To(metav1.Duration{Duration: 24 * time.Hour}),
							RenewBefore: ptr.To(metav1.Duration{Duration: 12 * time.Hour}),
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
					DeployVMConsoleProxy: ptr.To(true),
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
						Enable: ptr.To(true),
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
						Enable: ptr.To(false),
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
						Enable: ptr.To(true),
					},
					DeployVMConsoleProxy: ptr.To(true),
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
					EnableApplicationAwareQuota: ptr.To(true),
				}

				var v1Config hcov1.DeploymentConfig
				Expect(convertDeploymentV1beta1ToV1(v1beta1Spec, &v1Config)).To(Succeed())

				Expect(v1Config.ApplicationAwareConfig).ToNot(BeNil())
				Expect(v1Config.ApplicationAwareConfig.Enable).To(HaveValue(BeTrue()))
			})

			It("should set Enable=true when EnableApplicationAwareQuota is true and ApplicationAwareConfig is nil", func() {
				v1beta1Spec := HyperConvergedSpec{
					EnableApplicationAwareQuota: ptr.To(true),
				}

				var v1Config hcov1.DeploymentConfig
				Expect(convertDeploymentV1beta1ToV1(v1beta1Spec, &v1Config)).To(Succeed())

				Expect(v1Config.ApplicationAwareConfig).ToNot(BeNil())
				Expect(v1Config.ApplicationAwareConfig.Enable).To(HaveValue(BeTrue()))
			})

			It("should set Enable=false when EnableApplicationAwareQuota is false", func() {
				v1beta1Spec := HyperConvergedSpec{
					EnableApplicationAwareQuota: ptr.To(false),
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
					EnableApplicationAwareQuota: ptr.To(true),
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
					EnableApplicationAwareQuota: ptr.To(true),
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
						Enable: ptr.To(true),
						AllowApplicationAwareClusterResourceQuota: true,
					},
					DeployVMConsoleProxy: ptr.To(true),
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
