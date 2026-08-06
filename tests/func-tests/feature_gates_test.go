package tests_test

import (
	"context"
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/featuregatedetails"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/featuregates"
	tests "github.com/kubevirt/hyperconverged-cluster-operator/tests/func-tests"
)

var _ = Describe("test feature gates", Label("feature-gates"), func() {
	var (
		cli client.Client
	)

	BeforeEach(func(ctx context.Context) {
		cli = tests.GetControllerRuntimeClient()

		tests.RestoreDefaultFeatureGates(ctx, cli)
		DeferCleanup(func(ctx context.Context) {
			tests.RestoreDefaultFeatureGates(ctx, cli)
		})
	})

	It("allow enabling a FG with KubeVirt casing", func(ctx context.Context) {
		// this is an example of a "proxy feature gate" - a FG that is set in the HyperConverged CR, in order to set a
		// FG in the KubeVirt CR. The HCO name starts with lower case, while KubeVirt's FG name starts with upper case
		// This test works only if the feature gate is in alpha phase. If it became deprecated, or graduated to beta or GA
		// it should be replaced with another FG.
		const (
			kvFG  = "DownwardMetrics"
			hcoFG = "downwardMetrics"
		)

		// make sure it's alpha. If not, choose another alpha proxy FG
		phase, exists := featuregatedetails.GetFeatureGatePhase(hcoFG)
		Expect(exists).To(BeTrue())
		Expect(phase).To(Equal(featuregates.PhaseAlpha))

		By("make sure the feature gate is not enabled in KubeVirt")
		Eventually(func(g Gomega, ctx context.Context) {
			kv := getKubeVirt(ctx, g, cli)
			g.Expect(kv.Spec.Configuration.DeveloperConfiguration).ToNot(BeNil())
			g.Expect(kv.Spec.Configuration.DeveloperConfiguration.FeatureGates).ToNot(ContainElement(kvFG))
		}).WithTimeout(time.Minute).WithPolling(10 * time.Second).WithContext(ctx).Should(Succeed())

		By("set the feature gate using KubeVirt casing")
		Expect(tests.EnableFG(ctx, cli, "DownwardMetrics")).To(Succeed())

		By("make sure the feature gate is enabled in KubeVirt")
		Eventually(func(g Gomega, ctx context.Context) {
			kv := getKubeVirt(ctx, g, cli)
			g.Expect(kv.Spec.Configuration.DeveloperConfiguration).ToNot(BeNil())
			g.Expect(kv.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(ContainElement(kvFG))
		}).WithTimeout(time.Minute).WithPolling(10 * time.Second).WithContext(ctx).Should(Succeed())
	})

	// assuming the spec.featureGate field is not empty. adding this function to the test context because it's
	// implemented for this test. To enable a feature gate in another context, use the tests.EnableFG() function.
	addFeatureGate := func(ctx context.Context, cli client.Client, fgName string) error {
		const (
			appendFGTemplate = `[{"op": "add", "path": "/spec/featureGates/-", "value": {"name": %q}}]`
		)

		return retry.RetryOnConflict(retry.DefaultRetry, func() error {
			hc, err := tests.GetHCO(ctx, cli)
			if err != nil {
				return err
			}

			patch := fmt.Appendf(nil, appendFGTemplate, fgName)
			return cli.Patch(ctx, hc, client.RawPatch(types.JSONPatchType, patch))
		})
	}

	It("should not allow enabling the same feature gate with different casing", func(ctx context.Context) {
		hcoAlphaFGs := featuregatedetails.ListAlphaFeatureGates()
		if len(hcoAlphaFGs) == 0 {
			Skip("no Alpha feature gates found")
		}

		fgName := hcoAlphaFGs[0]

		By(fmt.Sprintf("Add the %q feature gate to the HyperConverged CR", fgName))
		Expect(tests.EnableFG(ctx, cli, fgName)).To(Succeed())

		By("try adding the feature gate to the HyperConverged CR, with the same name")
		Expect(addFeatureGate(ctx, cli, strings.ToUpper(fgName))).To(MatchError(k8serrors.IsInvalid, "check if it's the invalid error"))

		By("try adding the feature gate to the HyperConverged CR, with all lower case")
		Expect(addFeatureGate(ctx, cli, strings.ToLower(fgName))).To(MatchError(k8serrors.IsInvalid, "check if it's the invalid error"))

		By("try adding the feature gate to the HyperConverged CR, with all lower case")
		Expect(addFeatureGate(ctx, cli, strings.ToLower(fgName))).To(MatchError(k8serrors.IsInvalid, "check if it's the invalid error"))

		By("try adding the feature gate to the HyperConverged CR, in KubeVirt format")
		nameInKVFormat := strings.ToUpper(fgName[:1]) + fgName[1:]
		Expect(addFeatureGate(ctx, cli, nameInKVFormat)).To(MatchError(k8serrors.IsInvalid, "check if it's the invalid error"))
	})
})
