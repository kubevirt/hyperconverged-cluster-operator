package tests_test

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
	tests "github.com/kubevirt/hyperconverged-cluster-operator/tests/func-tests"
	"kubevirt.io/client-go/kubecli"
	"kubevirt.io/kubevirt/tests/flags"
)

const (
	setWaspFGPatchTemplate = `[{"op": "replace", "path": "/spec/featureGates/enableHigherDensityWithSwap", "value": %t}]`
)

var _ = Describe("wasp-agent", Label("wasp"), Serial, Ordered, func() {
	tests.FlagParse()
	var (
		cli kubecli.KubevirtClient
		ctx context.Context
	)

	BeforeEach(func() {
		var err error

		cli, err = kubecli.GetKubevirtClient()
		Expect(cli).ToNot(BeNil())
		Expect(err).ToNot(HaveOccurred())
		tests.SkipIfNotOpenShift(cli, "wasp-agent")

		ctx = context.Background()
		hc := tests.GetHCO(ctx, cli)
		if hc.Annotations == nil {
			hc.Annotations = make(map[string]string)
		}
		hc.Annotations["wasp.hyperconverged.io/dry-run"] = "true"
		tests.UpdateHCORetry(ctx, cli, hc)
	})

	AfterAll(func() {
		disableWaspFeatureGate(ctx, cli)
		hc := tests.GetHCO(ctx, cli)

		delete(hc.Annotations, "wasp.hyperconverged.io/dry-run")
		tests.UpdateHCORetry(ctx, cli, hc)
	})

	Context("wasp-agent deployment", func() {
		It("should deploy all wasp operands when enableHigherDensityWithSwap is on", func() {
			By("enable enableHigherDensityWithSwap feature-gate")
			enableWaspFeatureGate(ctx, cli)

			By("check for wasp-agent deamonset creation")
			Eventually(func(g Gomega) {
				waspDaemonset, err := cli.AppsV1().DaemonSets(flags.KubeVirtInstallNamespace).Get(ctx, string(hcoutil.AppComponentWasp), metav1.GetOptions{})
				g.Expect(err).ShouldNot(HaveOccurred())
				g.Expect(waspDaemonset.Status.DesiredNumberScheduled).To(Equal(waspDaemonset.Status.NumberReady))
			}).WithTimeout(1 * time.Minute).
				WithPolling(100 * time.Millisecond).
				Should(Succeed())

			By("check for wasp-agent ClusterRole creation")
			Eventually(func(g Gomega) {
				_, err := cli.RbacV1().ClusterRoles().Get(ctx, string(hcoutil.AppComponentWasp), metav1.GetOptions{})
				g.Expect(err).ShouldNot(HaveOccurred())
			}).WithTimeout(1 * time.Minute).
				WithPolling(100 * time.Millisecond).
				Should(Succeed())

			By("check for wasp-agent ClusterRoleBinding creation")
			Eventually(func(g Gomega) {
				_, err := cli.RbacV1().ClusterRoleBindings().Get(ctx, string(hcoutil.AppComponentWasp), metav1.GetOptions{})
				g.Expect(err).ShouldNot(HaveOccurred())
			}).WithTimeout(1 * time.Minute).
				WithPolling(100 * time.Millisecond).
				Should(Succeed())
		})
		It("should remove all wasp operands when enableHigherDensityWithSwap is off", func() {
			By("disable enableHigherDensityWithSwap feature-gate")
			disableWaspFeatureGate(ctx, cli)

			By("check for wasp-agent deamonset removal")
			Eventually(func() error {
				_, err := cli.AppsV1().DaemonSets(flags.KubeVirtInstallNamespace).Get(ctx, string(hcoutil.AppComponentWasp), metav1.GetOptions{})
				return err
			}).WithTimeout(1 * time.Minute).
				WithPolling(100 * time.Millisecond).
				Should(MatchError(apierrors.IsNotFound, "not found error"))

			By("check for wasp-agent ClusterRole removal")
			Eventually(func() error {
				_, err := cli.RbacV1().ClusterRoles().Get(ctx, string(hcoutil.AppComponentWasp), metav1.GetOptions{})
				return err
			}).WithTimeout(1 * time.Minute).
				WithPolling(100 * time.Millisecond).
				Should(MatchError(apierrors.IsNotFound, "not found error"))

			By("check for wasp-agent ClusterRoleBinding removal")
			Eventually(func() error {
				_, err := cli.RbacV1().ClusterRoleBindings().Get(ctx, string(hcoutil.AppComponentWasp), metav1.GetOptions{})
				return err
			}).WithTimeout(1 * time.Minute).
				WithPolling(100 * time.Millisecond).
				Should(MatchError(apierrors.IsNotFound, "not found error"))
		})
	})
})

func enableWaspFeatureGate(ctx context.Context, cli kubecli.KubevirtClient) {
	setWaspFeatureGate(ctx, cli, true)
}

func disableWaspFeatureGate(ctx context.Context, cli kubecli.KubevirtClient) {
	setWaspFeatureGate(ctx, cli, false)
}

func setWaspFeatureGate(ctx context.Context, cli kubecli.KubevirtClient, fgState bool) {
	patch := []byte(fmt.Sprintf(setWaspFGPatchTemplate, fgState))
	Eventually(tests.PatchHCO).
		WithArguments(ctx, cli, patch).
		WithTimeout(10 * time.Second).
		WithPolling(100 * time.Millisecond).
		WithOffset(2).
		Should(Succeed())
}
