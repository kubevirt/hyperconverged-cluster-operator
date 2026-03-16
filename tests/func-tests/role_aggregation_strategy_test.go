package tests_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubevirtcorev1 "kubevirt.io/api/core/v1"

	tests "github.com/kubevirt/hyperconverged-cluster-operator/tests/func-tests"
)

var _ = Describe("RoleAggregationStrategy", Serial, Label("RoleAggregationStrategy"), func() {
	tests.FlagParse()
	var (
		cli client.Client
	)

	BeforeEach(func(ctx context.Context) {
		cli = tests.GetControllerRuntimeClient()
		tests.BeforeEach(ctx)
	})

	AfterEach(func(ctx context.Context) {
		rmPatch := []byte(`[{"op": "remove", "path": "/spec/roleAggregationStrategy"}]`)
		_ = tests.PatchHCO(ctx, cli, rmPatch)
	})

	It("should propagate Manual to KubeVirt CR and add OptOutRoleAggregation feature gate", func(ctx context.Context) {
		hc := tests.GetHCO(ctx, cli)
		hc.Spec.RoleAggregationStrategy = ptr.To(kubevirtcorev1.RoleAggregationStrategyManual)
		tests.UpdateHCORetry(ctx, cli, hc)

		Eventually(func(g Gomega, ctx context.Context) {
			kv := getKubeVirt(ctx, g, cli)
			g.Expect(kv.Spec.Configuration.RoleAggregationStrategy).To(HaveValue(Equal(kubevirtcorev1.RoleAggregationStrategyManual)))
			g.Expect(kv.Spec.Configuration.DeveloperConfiguration).ToNot(BeNil())
			g.Expect(kv.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(ContainElement("OptOutRoleAggregation"))
		}).WithTimeout(time.Minute).WithPolling(10 * time.Second).WithContext(ctx).Should(Succeed())
	})

	It("should keep OptOutRoleAggregation FG when changing from Manual to AggregateToDefault", func(ctx context.Context) {
		By("Setting RoleAggregationStrategy to Manual")
		hc := tests.GetHCO(ctx, cli)
		hc.Spec.RoleAggregationStrategy = ptr.To(kubevirtcorev1.RoleAggregationStrategyManual)
		tests.UpdateHCORetry(ctx, cli, hc)

		Eventually(func(g Gomega, ctx context.Context) {
			kv := getKubeVirt(ctx, g, cli)
			g.Expect(kv.Spec.Configuration.RoleAggregationStrategy).To(HaveValue(Equal(kubevirtcorev1.RoleAggregationStrategyManual)))
		}).WithTimeout(time.Minute).WithPolling(10 * time.Second).WithContext(ctx).Should(Succeed())

		By("Changing RoleAggregationStrategy to AggregateToDefault")
		hc = tests.GetHCO(ctx, cli)
		hc.Spec.RoleAggregationStrategy = ptr.To(kubevirtcorev1.RoleAggregationStrategyAggregateToDefault)
		tests.UpdateHCORetry(ctx, cli, hc)

		Eventually(func(g Gomega, ctx context.Context) {
			kv := getKubeVirt(ctx, g, cli)
			g.Expect(kv.Spec.Configuration.RoleAggregationStrategy).To(HaveValue(Equal(kubevirtcorev1.RoleAggregationStrategyAggregateToDefault)))
			g.Expect(kv.Spec.Configuration.DeveloperConfiguration).ToNot(BeNil())
			g.Expect(kv.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(ContainElement("OptOutRoleAggregation"))
		}).WithTimeout(time.Minute).WithPolling(10 * time.Second).WithContext(ctx).Should(Succeed())
	})

	It("should clear RoleAggregationStrategy and remove OptOutRoleAggregation FG when removed from HCO", func(ctx context.Context) {
		By("Setting RoleAggregationStrategy to Manual")
		hc := tests.GetHCO(ctx, cli)
		hc.Spec.RoleAggregationStrategy = ptr.To(kubevirtcorev1.RoleAggregationStrategyManual)
		tests.UpdateHCORetry(ctx, cli, hc)

		Eventually(func(g Gomega, ctx context.Context) {
			kv := getKubeVirt(ctx, g, cli)
			g.Expect(kv.Spec.Configuration.RoleAggregationStrategy).To(HaveValue(Equal(kubevirtcorev1.RoleAggregationStrategyManual)))
		}).WithTimeout(time.Minute).WithPolling(10 * time.Second).WithContext(ctx).Should(Succeed())

		By("Removing RoleAggregationStrategy from HCO")
		rmPatch := []byte(`[{"op": "remove", "path": "/spec/roleAggregationStrategy"}]`)
		Expect(tests.PatchHCO(ctx, cli, rmPatch)).To(Succeed())

		Eventually(func(g Gomega, ctx context.Context) {
			kv := getKubeVirt(ctx, g, cli)
			g.Expect(kv.Spec.Configuration.RoleAggregationStrategy).To(BeNil())
			g.Expect(kv.Spec.Configuration.DeveloperConfiguration).ToNot(BeNil())
			g.Expect(kv.Spec.Configuration.DeveloperConfiguration.FeatureGates).ToNot(ContainElement("OptOutRoleAggregation"))
		}).WithTimeout(time.Minute).WithPolling(10 * time.Second).WithContext(ctx).Should(Succeed())
	})
})

func getKubeVirt(ctx context.Context, g Gomega, cli client.Client) *kubevirtcorev1.KubeVirt {
	kv := &kubevirtcorev1.KubeVirt{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubevirt-kubevirt-hyperconverged",
			Namespace: tests.InstallNamespace,
		},
	}
	g.Expect(cli.Get(ctx, client.ObjectKeyFromObject(kv), kv)).To(Succeed())
	return kv
}
