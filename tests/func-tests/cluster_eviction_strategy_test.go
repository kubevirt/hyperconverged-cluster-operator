package tests_test

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kvv1 "kubevirt.io/api/core/v1"

	tests "github.com/kubevirt/hyperconverged-cluster-operator/tests/func-tests"
	"github.com/kubevirt/hyperconverged-cluster-operator/tests/func-tests/libnode"
)

var (
	rmEvictionStrategyPatch  = []byte(`[{"op": "remove", "path": "/spec/virtualization/evictionStrategy"}]`)
	setEvictionStrategyPatch = `[{"op": "replace", "path": "/spec/virtualization/evictionStrategy", "value": "%s"}]`
)

var _ = Describe("Cluster level evictionStrategy default value", Label("evictionStrategy"), func() {
	tests.FlagParse()
	var (
		cli client.Client

		initialEvictionStrategy *kvv1.EvictionStrategy
		singleWorkerCluster     bool
	)

	BeforeEach(func(ctx context.Context) {
		cli = tests.GetControllerRuntimeClient()

		var err error
		singleWorkerCluster, err = libnode.IsSingleWorkerCluster(ctx, cli)
		Expect(err).ToNot(HaveOccurred())

		tests.BeforeEach(ctx)
		hc, err := tests.GetHCO(ctx, cli)
		Expect(err).ToNot(HaveOccurred())

		initialEvictionStrategy = hc.Spec.Virtualization.EvictionStrategy
	})

	AfterEach(func(ctx context.Context) {
		patch := rmEvictionStrategyPatch
		if initialEvictionStrategy != nil {
			patch = fmt.Appendf(nil, setEvictionStrategyPatch, *initialEvictionStrategy)
		}
		tests.PatchHCO(ctx, cli, patch)
	})

	DescribeTable("test spec.virtualization.evictionStrategy", func(ctx context.Context, clusterValidationFn func(bool), expectedValue kvv1.EvictionStrategy) {
		clusterValidationFn(singleWorkerCluster)

		tests.PatchHCO(ctx, cli, rmEvictionStrategyPatch)

		Eventually(func(g Gomega, ctx context.Context) {
			hc, err := tests.GetHCO(ctx, cli)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(hc).NotTo(BeNil())
			g.Expect(hc.Spec.Virtualization.EvictionStrategy).To(HaveValue(Equal(expectedValue)))
		}).WithContext(ctx).WithTimeout(5 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())
	},
		Entry(
			"Should set spec.virtualization.evictionStrategy = None by default on single worker clusters",
			Label(tests.SingleNodeLabel),
			tests.FailIfHighAvailableCluster,
			kvv1.EvictionStrategyNone,
		),
		Entry(
			"Should set spec.virtualization.evictionStrategy = LiveMigrate by default with multiple worker node",
			Label(tests.HighlyAvailableClusterLabel),
			tests.FailIfSingleNodeCluster,
			kvv1.EvictionStrategyLiveMigrate,
		),
	)
})
