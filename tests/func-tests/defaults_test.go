package tests_test

import (
	"context"
	"fmt"
	"reflect"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	tests "github.com/kubevirt/hyperconverged-cluster-operator/tests/func-tests"
)

const (
	removePathPatchTmplt = `[{"op": "remove", "path": %q}]`
)

var _ = Describe("Check Default values", Label("defaults"), Serial, func() {
	var cli client.Client

	BeforeEach(func(ctx context.Context) {
		cli = tests.GetControllerRuntimeClient()

		tests.RestoreDefaults(ctx, cli)
	})

	Context("certConfig defaults", func() {
		defaultCertConfig := hcov1.HyperConvergedCertConfig{
			CA: hcov1.CertRotateConfigCA{
				Duration:    &metav1.Duration{Duration: time.Hour * 48},
				RenewBefore: &metav1.Duration{Duration: time.Hour * 24},
			},
			Server: hcov1.CertRotateConfigServer{
				Duration:    &metav1.Duration{Duration: time.Hour * 24},
				RenewBefore: &metav1.Duration{Duration: time.Hour * 12},
			},
		}

		DescribeTable("Check that certConfig defaults are behaving as expected", func(ctx context.Context, path string) {
			patch := fmt.Appendf(nil, removePathPatchTmplt, path)
			Eventually(func(ctx context.Context) error {
				return tests.PatchHCO(ctx, cli, patch)
			}).WithTimeout(2 * time.Second).WithPolling(500 * time.Millisecond).WithContext(ctx).Should(Succeed())

			Eventually(func(g Gomega, ctx context.Context) {
				hc, err := tests.GetHCO(ctx, cli)
				g.Expect(err).NotTo(HaveOccurred())

				g.Expect(reflect.DeepEqual(hc.Spec.Security.CertConfig, defaultCertConfig)).To(BeTrue(), "certConfig should be equal to default")
			}).WithTimeout(2 * time.Second).WithPolling(100 * time.Millisecond).WithContext(ctx).Should(Succeed())
		},
			Entry("when removing /spec/security/certConfig/ca/duration", "/spec/security/certConfig/ca/duration"),
			Entry("when removing /spec/security/certConfig/ca/renewBefore", "/spec/security/certConfig/ca/renewBefore"),
			Entry("when removing /spec/security/certConfig/ca", "/spec/security/certConfig/ca"),
			Entry("when removing /spec/security/certConfig/server/duration", "/spec/security/certConfig/server/duration"),
			Entry("when removing /spec/security/certConfig/server/renewBefore", "/spec/security/certConfig/server/renewBefore"),
			Entry("when removing /spec/security/certConfig/server", "/spec/security/certConfig/server"),
			Entry("when removing /spec/security/certConfig", "/spec/security/certConfig"),
			Entry("when removing /spec/security", "/spec/security"),
			Entry("when removing /spec", "/spec"),
		)
	})

	Context("feature gate defaults", func() {
		defaultFeatureGates := map[string]gomegatypes.GomegaMatcher{
			"downwardMetrics":                BeFalseBecause("the downwardMetrics feature gate should be disabled by default"),
			"deployKubeSecondaryDNS":         BeFalseBecause("the deployKubeSecondaryDNS feature gate should be disabled by default"),
			"disableMDevConfiguration":       BeFalseBecause("the disableMDevConfiguration feature gate should be disabled by default"),
			"persistentReservation":          BeFalseBecause("the persistentReservation feature gate should be disabled by default"),
			"alignCPUs":                      BeFalseBecause("the alignCPUs feature gate should be disabled by default"),
			"enableMultiArchBootImageImport": BeFalseBecause("the enableMultiArchBootImageImport feature gate should be disabled by default"),
			"decentralizedLiveMigration":     BeTrueBecause("the decentralizedLiveMigration feature gate should be enabled by default"),
			"declarativeHotplugVolumes":      BeTrueBecause("the declarativeHotplugVolumes feature gate should be enabled by default"),
			"videoConfig":                    BeTrueBecause("the videoConfig feature gate should be enabled by default"),
			"objectGraph":                    BeFalseBecause("the objectGraph feature gate should be disabled by default"),
			"incrementalBackup":              BeFalseBecause("the incrementalBackup feature gate should be disabled by default"),
			"containerPathVolumes":           BeFalseBecause("the containerPathVolumes feature gate should be disabled by default"),
		}

		It("Check that featureGates defaults are behaving as expected", func(ctx context.Context) {
			patch := fmt.Appendf(nil, removePathPatchTmplt, "/spec/featureGates")
			Eventually(func(g Gomega, ctx context.Context) error {
				hc, err := tests.GetHCO(ctx, cli)
				g.Expect(err).NotTo(HaveOccurred())

				if len(hc.Spec.FeatureGates) == 0 {
					return nil
				}
				return tests.PatchHCO(ctx, cli, patch)
			}).WithTimeout(2 * time.Second).WithPolling(500 * time.Millisecond).WithContext(ctx).Should(Succeed())

			Eventually(func(g Gomega, ctx context.Context) {
				hc, err := tests.GetHCO(ctx, cli)
				g.Expect(err).NotTo(HaveOccurred())

				for fgName, matcher := range defaultFeatureGates {
					g.Expect(hc.Spec.FeatureGates.IsEnabled(fgName)).To(matcher)
				}
			}).WithTimeout(2 * time.Second).WithPolling(100 * time.Millisecond).WithContext(ctx).Should(Succeed())
		})
	})

	Context("liveMigrationConfig defaults", func() {
		defaultLiveMigrationConfig := hcov1.LiveMigrationConfigurations{
			AllowAutoConverge:                 new(false),
			AllowPostCopy:                     new(false),
			CompletionTimeoutPerGiB:           new(int64(150)),
			ParallelMigrationsPerCluster:      new(uint32(5)),
			ParallelOutboundMigrationsPerNode: new(uint32(2)),
			ProgressTimeout:                   new(int64(150)),
		}

		DescribeTable("Check that liveMigrationConfig defaults are behaving as expected", func(ctx context.Context, path string) {
			patch := fmt.Appendf(nil, removePathPatchTmplt, path)
			Eventually(func(ctx context.Context) error {
				return tests.PatchHCO(ctx, cli, patch)
			}).WithTimeout(2 * time.Second).WithPolling(500 * time.Millisecond).WithContext(ctx).Should(Succeed())

			Eventually(func(g Gomega, ctx context.Context) {
				hc, err := tests.GetHCO(ctx, cli)
				g.Expect(err).NotTo(HaveOccurred())

				g.Expect(reflect.DeepEqual(hc.Spec.Virtualization.LiveMigrationConfig, defaultLiveMigrationConfig)).To(BeTrue(), "liveMigrationConfig should be equal to default")
			}).WithTimeout(2 * time.Second).WithPolling(100 * time.Millisecond).WithContext(ctx).Should(Succeed())
		},
			Entry("when removing /spec/virtualization/liveMigrationConfig/allowAutoConverge", "/spec/virtualization/liveMigrationConfig/allowAutoConverge"),
			Entry("when removing /spec/virtualization/liveMigrationConfig/allowPostCopy", "/spec/virtualization/liveMigrationConfig/allowPostCopy"),
			Entry("when removing /spec/virtualization/liveMigrationConfig/completionTimeoutPerGiB", "/spec/virtualization/liveMigrationConfig/completionTimeoutPerGiB"),
			Entry("when removing /spec/virtualization/liveMigrationConfig/parallelMigrationsPerCluster", "/spec/virtualization/liveMigrationConfig/parallelMigrationsPerCluster"),
			Entry("when removing /spec/virtualization/liveMigrationConfig/parallelOutboundMigrationsPerNode", "/spec/virtualization/liveMigrationConfig/parallelOutboundMigrationsPerNode"),
			Entry("when removing /spec/virtualization/liveMigrationConfig/progressTimeout", "/spec/virtualization/liveMigrationConfig/progressTimeout"),
			Entry("when removing /spec/virtualization/liveMigrationConfig", "/spec/virtualization/liveMigrationConfig"),
			Entry("when removing /spec/virtualization", "/spec/virtualization"),
			Entry("when removing /spec", "/spec"),
		)
	})

	Context("VmiCPUAllocationRatio defaults", func() {
		const defaultVmiCPUAllocationRatio = 10

		DescribeTable("Check that resourceRequirements defaults are behaving as expected", func(ctx context.Context, path string) {
			patch := fmt.Appendf(nil, removePathPatchTmplt, path)
			Eventually(func(ctx context.Context) error {
				return tests.PatchHCO(ctx, cli, patch)
			}).WithTimeout(20 * time.Second).WithPolling(500 * time.Millisecond).WithContext(ctx).Should(Succeed())

			Eventually(func(g Gomega, ctx context.Context) {
				hc, err := tests.GetHCO(ctx, cli)
				g.Expect(err).NotTo(HaveOccurred())

				g.Expect(hc.Spec.Virtualization.VmiCPUAllocationRatio).To(HaveValue(Equal(defaultVmiCPUAllocationRatio)), "VmiCPUAllocationRatio should be equal to default")
			}).WithTimeout(2 * time.Second).WithPolling(100 * time.Millisecond).WithContext(ctx).Should(Succeed())
		},
			Entry("when removing /spec/virtualization/vmiCPUAllocationRatio", "/spec/virtualization/vmiCPUAllocationRatio"),
			Entry("when removing /spec/virtualization", "/spec/virtualization"),
			Entry("when removing /spec", "/spec"),
		)
	})

	Context("workloadUpdateStrategy defaults", func() {
		defaultWorkloadUpdateStrategy := hcov1.HyperConvergedWorkloadUpdateStrategy{
			BatchEvictionInterval: &metav1.Duration{Duration: time.Minute},
			BatchEvictionSize:     new(10),
			WorkloadUpdateMethods: []string{"LiveMigrate"},
		}

		DescribeTable("Check that workloadUpdateStrategy defaults are behaving as expected", func(ctx context.Context, path string) {
			patch := fmt.Appendf(nil, removePathPatchTmplt, path)
			Eventually(func(ctx context.Context) error {
				return tests.PatchHCO(ctx, cli, patch)
			}).WithTimeout(20 * time.Second).WithPolling(500 * time.Millisecond).WithContext(ctx).Should(Succeed())

			Eventually(func(g Gomega, ctx context.Context) {
				hc, err := tests.GetHCO(ctx, cli)
				g.Expect(err).NotTo(HaveOccurred())

				g.Expect(reflect.DeepEqual(hc.Spec.Virtualization.WorkloadUpdateStrategy, defaultWorkloadUpdateStrategy)).To(BeTrue(), "workloadUpdateStrategy should be equal to default")
			}).WithTimeout(2 * time.Second).WithPolling(100 * time.Millisecond).WithContext(ctx).Should(Succeed())
		},
			Entry("when removing /spec/virtualization/workloadUpdateStrategy/batchEvictionInterval", "/spec/virtualization/workloadUpdateStrategy/batchEvictionInterval"),
			Entry("when removing /spec/virtualization/workloadUpdateStrategy/batchEvictionSize", "/spec/virtualization/workloadUpdateStrategy/batchEvictionSize"),
			Entry("when removing /spec/virtualization/workloadUpdateStrategy/workloadUpdateMethods", "/spec/virtualization/workloadUpdateStrategy/workloadUpdateMethods"),
			Entry("when removing /spec/virtualization/workloadUpdateStrategy", "/spec/virtualization/workloadUpdateStrategy"),
			Entry("when removing /spec/virtualization", "/spec/virtualization"),
			Entry("when removing /spec", "/spec"),
		)
	})

	Context("uninstallStrategy defaults", func() {
		const defaultUninstallStrategy = hcov1.HyperConvergedUninstallStrategyBlockUninstallIfWorkloadsExist

		DescribeTable("Check that uninstallStrategy default is behaving as expected", func(ctx context.Context, path string) {
			patch := fmt.Appendf(nil, removePathPatchTmplt, path)
			Eventually(func(ctx context.Context) error {
				return tests.PatchHCO(ctx, cli, patch)
			}).WithTimeout(2 * time.Second).WithPolling(500 * time.Millisecond).WithContext(ctx).Should(Succeed())

			Eventually(func(g Gomega, ctx context.Context) {
				hc, err := tests.GetHCO(ctx, cli)
				g.Expect(err).NotTo(HaveOccurred())

				g.Expect(hc.Spec.Deployment.UninstallStrategy).To(Equal(defaultUninstallStrategy), "uninstallStrategy should be equal to default")
			}).WithTimeout(2 * time.Second).WithPolling(100 * time.Millisecond).WithContext(ctx).Should(Succeed())
		},
			Entry("when removing /spec/deployment/uninstallStrategy", "/spec/deployment/uninstallStrategy"),
			Entry("when removing /spec/deployment", "/spec/deployment"),
			Entry("when removing /spec", "/spec"),
		)
	})

	Context("VirtualMachineOptions defaults", func() {
		defaultVirtualMachineOptions := &hcov1.VirtualMachineOptions{
			DisableFreePageReporting: new(false),
			DisableSerialConsoleLog:  new(false),
		}

		DescribeTable("Check that featureGates defaults are behaving as expected", func(ctx context.Context, path string) {
			patch := fmt.Appendf(nil, removePathPatchTmplt, path)
			Eventually(func(ctx context.Context) error {
				return tests.PatchHCO(ctx, cli, patch)
			}).WithTimeout(2 * time.Second).WithPolling(500 * time.Millisecond).WithContext(ctx).Should(Succeed())

			Eventually(func(g Gomega, ctx context.Context) {
				hc, err := tests.GetHCO(ctx, cli)
				g.Expect(err).NotTo(HaveOccurred())

				g.Expect(reflect.DeepEqual(hc.Spec.Virtualization.VirtualMachineOptions, defaultVirtualMachineOptions)).To(BeTrue(), "virtualMachineOptions should be equal to default")
			}).WithTimeout(2 * time.Second).WithPolling(100 * time.Millisecond).WithContext(ctx).Should(Succeed())
		},
			Entry("when removing /spec/virtualization/virtualMachineOptions/disableFreePageReporting", "/spec/virtualization/virtualMachineOptions/disableFreePageReporting"),
			Entry("when removing /spec/virtualization/virtualMachineOptions/disableSerialConsoleLog", "/spec/virtualization/virtualMachineOptions/disableSerialConsoleLog"),
			Entry("when removing /spec/virtualization/virtualMachineOptions", "/spec/virtualization/virtualMachineOptions"),
			Entry("when removing /spec/virtualization", "/spec/virtualization"),
			Entry("when removing /spec", "/spec"),
		)
	})

	Context("HigherWorkloadDensity defaults", func() {
		defaultHigherWorkloadDensity := &hcov1.HigherWorkloadDensityConfiguration{
			MemoryOvercommitPercentage: 100,
		}

		DescribeTable("Check that HigherWorkloadDensity defaults are behaving as expected", func(ctx context.Context, path string) {
			patch := fmt.Appendf(nil, removePathPatchTmplt, path)
			Eventually(func(ctx context.Context) error {
				return tests.PatchHCO(ctx, cli, patch)
			}).WithTimeout(2 * time.Second).WithPolling(500 * time.Millisecond).WithContext(ctx).Should(Succeed())

			Eventually(func(g Gomega, ctx context.Context) {
				hc, err := tests.GetHCO(ctx, cli)
				g.Expect(err).NotTo(HaveOccurred())

				g.Expect(reflect.DeepEqual(hc.Spec.Virtualization.HigherWorkloadDensity, defaultHigherWorkloadDensity)).To(BeTrue(), "HigherWorkloadDensity should be equal to default")
			}).WithTimeout(2 * time.Second).WithPolling(100 * time.Millisecond).WithContext(ctx).Should(Succeed())
		},
			Entry("when removing /spec/virtualization/higherWorkloadDensity/memoryOvercommitPercentage", "/spec/virtualization/higherWorkloadDensity/memoryOvercommitPercentage"),
			Entry("when removing /spec/virtualization/higherWorkloadDensity", "/spec/virtualization/higherWorkloadDensity"),
			Entry("when removing /spec/virtualization", "/spec/virtualization"),
			Entry("when removing /spec", "/spec"),
		)
	})
})
