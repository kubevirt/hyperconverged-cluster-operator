package tests_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubevirtcorev1 "kubevirt.io/api/core/v1"

	tests "github.com/kubevirt/hyperconverged-cluster-operator/tests/func-tests"
)

var _ = Describe("Hypervisors configuration", Label("Hypervisors"), func() {
	tests.FlagParse()
	var (
		cli                client.Client
		initialHypervisors []kubevirtcorev1.HypervisorConfiguration
	)

	BeforeEach(func(ctx context.Context) {
		cli = tests.GetControllerRuntimeClient()

		tests.BeforeEach(ctx)
		hc := tests.GetHCO(ctx, cli)
		initialHypervisors = hc.Spec.Hypervisors
	})

	AfterEach(func(ctx context.Context) {
		hc := tests.GetHCO(ctx, cli)
		hc.Spec.Hypervisors = initialHypervisors
		_ = tests.UpdateHCORetry(ctx, cli, hc)
	})

	getKubeVirt := func(ctx context.Context, g Gomega, cli client.Client) *kubevirtcorev1.KubeVirt {
		kv := &kubevirtcorev1.KubeVirt{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kubevirt-kubevirt-hyperconverged",
				Namespace: tests.InstallNamespace,
			},
		}
		g.Expect(cli.Get(ctx, client.ObjectKeyFromObject(kv), kv)).To(Succeed())
		return kv
	}

	It("should propagate hypervisors to KubeVirt CR and add ConfigurableHypervisor feature gate", func(ctx context.Context) {
		hc := tests.GetHCO(ctx, cli)
		hc.Spec.Hypervisors = []kubevirtcorev1.HypervisorConfiguration{
			{Name: kubevirtcorev1.KvmHypervisorName},
		}
		_ = tests.UpdateHCORetry(ctx, cli, hc)

		Eventually(func(g Gomega, ctx context.Context) {
			kv := getKubeVirt(ctx, g, cli)
			g.Expect(kv.Spec.Configuration.Hypervisors).To(HaveLen(1))
			g.Expect(kv.Spec.Configuration.Hypervisors[0].Name).To(Equal(kubevirtcorev1.KvmHypervisorName))
			g.Expect(kv.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(ContainElement("ConfigurableHypervisor"))
		}).WithTimeout(time.Minute).
			WithPolling(10 * time.Second).
			WithContext(ctx).
			Should(Succeed())
	})

	It("should remove hypervisors from KubeVirt CR and remove ConfigurableHypervisor FG when cleared from HCO", func(ctx context.Context) {
		hc := tests.GetHCO(ctx, cli)
		hc.Spec.Hypervisors = []kubevirtcorev1.HypervisorConfiguration{
			{Name: kubevirtcorev1.KvmHypervisorName},
		}
		_ = tests.UpdateHCORetry(ctx, cli, hc)

		Eventually(func(g Gomega, ctx context.Context) {
			kv := getKubeVirt(ctx, g, cli)
			g.Expect(kv.Spec.Configuration.Hypervisors).To(HaveLen(1))
		}).WithTimeout(time.Minute).
			WithPolling(10 * time.Second).
			WithContext(ctx).
			Should(Succeed())

		hc = tests.GetHCO(ctx, cli)
		hc.Spec.Hypervisors = nil
		_ = tests.UpdateHCORetry(ctx, cli, hc)

		Eventually(func(g Gomega, ctx context.Context) {
			kv := getKubeVirt(ctx, g, cli)
			g.Expect(kv.Spec.Configuration.Hypervisors).To(BeEmpty())
			g.Expect(kv.Spec.Configuration.DeveloperConfiguration.FeatureGates).ToNot(ContainElement("ConfigurableHypervisor"))
		}).WithTimeout(time.Minute).
			WithPolling(10 * time.Second).
			WithContext(ctx).
			Should(Succeed())
	})
})
