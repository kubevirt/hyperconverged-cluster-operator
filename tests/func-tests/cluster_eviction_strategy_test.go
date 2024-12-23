package tests_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "kubevirt.io/api/core/v1"
	"kubevirt.io/client-go/kubecli"

	tests "github.com/kubevirt/hyperconverged-cluster-operator/tests/func-tests"
)

var (
	rmEvictionStrategyPatch = []byte(`[{"op": "remove", "path": "/spec/evictionStrategy"}]`)
)

var _ = Describe("Cluster level evictionStrategy default value", Label("evictionStrategy"), func() {
	tests.FlagParse()
	var cli kubecli.KubevirtClient
	ctx := context.TODO()

	var (
		initialEvictionStrategy *v1.EvictionStrategy
		singleWorkerCluster     bool
	)

	BeforeEach(func() {
		var err error
		cli, err = kubecli.GetKubevirtClient()
		Expect(cli).ToNot(BeNil())
		Expect(err).ToNot(HaveOccurred())

		singleWorkerCluster, err = isSingleWorkerCluster(cli)
		Expect(err).ToNot(HaveOccurred())

		tests.BeforeEach()
		hc := tests.GetHCO(ctx, cli)
		initialEvictionStrategy = hc.Spec.EvictionStrategy
	})

	AfterEach(func() {
		hc := tests.GetHCO(ctx, cli)
		hc.Spec.EvictionStrategy = initialEvictionStrategy
		_ = tests.UpdateHCORetry(ctx, cli, hc)
	})

	DescribeTable("test spec.evictionStrategy", func(ctx context.Context, clusterValidationFn func(bool), expectedValue v1.EvictionStrategy) {
		clusterValidationFn(singleWorkerCluster)

		Expect(tests.PatchHCO(ctx, cli, rmEvictionStrategyPatch)).To(Succeed())

		Eventually(func(g Gomega, ctx context.Context) {
			hc := tests.GetHCO(ctx, cli)
			g.Expect(hc).NotTo(BeNil())
			g.Expect(hc.Spec.EvictionStrategy).To(HaveValue(Equal(expectedValue)))
		}).WithContext(ctx).WithTimeout(5 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())
	},
		Entry(
			"Should set spec.evictionStrategy = None by default on single worker clusters",
			Label(tests.SingleNodeLabel),
			tests.FailIfHighAvailableCluster,
			v1.EvictionStrategyNone,
		),
		Entry(
			"Should set spec.evictionStrategy = LiveMigrate by default with multiple worker node",
			Label(tests.HighlyAvailableClusterLabel),
			tests.FailIfSingleNodeCluster,
			v1.EvictionStrategyLiveMigrate,
		),
	)

})

func isSingleWorkerCluster(cli kubecli.KubevirtClient) (bool, error) {
	workerNodes, err := cli.CoreV1().Nodes().List(context.TODO(), k8smetav1.ListOptions{LabelSelector: "node-role.kubernetes.io/worker"})
	if err != nil {
		return false, err
	}

	return len(workerNodes.Items) == 1, nil
}
