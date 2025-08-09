package tests_test

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	securityv1 "github.com/openshift/api/security/v1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	tests "github.com/kubevirt/hyperconverged-cluster-operator/tests/func-tests"
)

const (
	waspAgentComponentLabel     = "app.kubernetes.io/component=wasp-agent"
	dsName                      = "wasp-agent"
	saName                      = "wasp"
	sccName                     = "wasp"
	clusterRoleName             = "wasp-cluster"
	clusterRoleBindingName      = "wasp-cluster"
	setMemoryOvercommitTemplate = `[{"op": "replace", "path": "/spec/higherWorkloadDensity/memoryOvercommitPercentage", "value": %d}]`
	overcommitPercent           = 150
)

var _ = Describe("Test wasp-agent", Label(tests.OpenshiftLabel, "wasp-agent"), Serial, Ordered, func() {
	tests.FlagParse()

	var (
		cli                       client.Client
		originalOvercommitPercent int
	)

	BeforeAll(func(ctx context.Context) {
		cli = tests.GetControllerRuntimeClient()
		tests.FailIfNotOpenShift(ctx, cli, "wasp-agent")
		originalHco := tests.GetHCO(ctx, cli)
		if originalHco.Spec.HigherWorkloadDensity != nil {
			originalOvercommitPercent = originalHco.Spec.HigherWorkloadDensity.MemoryOvercommitPercentage
		}

	})

	AfterAll(func(ctx context.Context) {
		if originalOvercommitPercent != overcommitPercent {
			setMemoryOvercommitPercentage(ctx, cli, originalOvercommitPercent)
		}
	})

	BeforeEach(func() {
		Expect(securityv1.AddToScheme(cli.Scheme())).To(Succeed())
	})

	When("Higher density is set beyond 100 percent", func() {
		It("should deploy wasp-agent components", func(ctx context.Context) {
			setMemoryOvercommitPercentage(ctx, cli, overcommitPercent)

			By("check the wasp-agent Daemonset")
			Eventually(func(g Gomega, ctx context.Context) bool {
				ds, err := getDs(ctx, cli)
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(ds.Status.DesiredNumberScheduled).ToNot(BeZero())
				return ds.Status.DesiredNumberScheduled == ds.Status.CurrentNumberScheduled
			}).WithTimeout(5 * time.Minute).WithPolling(time.Second).WithContext(ctx).Should(BeTrue())

			By("check the wasp-agent SA")
			Eventually(func(g Gomega, ctx context.Context) {
				sa := &v1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      saName,
						Namespace: tests.InstallNamespace,
					},
				}
				g.Expect(cli.Get(ctx, client.ObjectKeyFromObject(sa), sa)).To(Succeed())
			}).WithTimeout(1 * time.Minute).
				WithPolling(100 * time.Millisecond).
				WithContext(ctx).
				Should(Succeed())

			By("check the wasp-agent cluster role")
			Eventually(func(g Gomega, ctx context.Context) {
				role := &rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{
						Name: clusterRoleName,
					},
				}
				g.Expect(cli.Get(ctx, client.ObjectKeyFromObject(role), role)).To(Succeed())
			}).WithTimeout(1 * time.Minute).
				WithPolling(100 * time.Millisecond).
				WithContext(ctx).
				Should(Succeed())

			By("check the wasp-agent cluster role binding")
			Eventually(func(g Gomega, ctx context.Context) {
				binding := &rbacv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name: clusterRoleBindingName,
					},
				}
				g.Expect(cli.Get(ctx, client.ObjectKeyFromObject(binding), binding)).To(Succeed())
			}).WithTimeout(1 * time.Minute).
				WithPolling(100 * time.Millisecond).
				WithContext(ctx).
				Should(Succeed())

			By("check the wasp-agent SecurityContextConstraints")
			Eventually(func(g Gomega, ctx context.Context) {
				scc := &securityv1.SecurityContextConstraints{
					ObjectMeta: metav1.ObjectMeta{
						Name: sccName,
					},
				}
				g.Expect(cli.Get(ctx, client.ObjectKeyFromObject(scc), scc)).To(Succeed())
			}).WithTimeout(1 * time.Minute).
				WithPolling(100 * time.Millisecond).
				WithContext(ctx).
				Should(Succeed())
		})
	})
})

func setMemoryOvercommitPercentage(ctx context.Context, cli client.Client, percentage int) {
	patchBytes := []byte(fmt.Sprintf(setMemoryOvercommitTemplate, percentage))

	Eventually(tests.PatchHCO).
		WithArguments(ctx, cli, patchBytes).
		WithTimeout(10 * time.Second).
		WithPolling(100 * time.Millisecond).
		WithContext(ctx).
		WithOffset(2).
		Should(Succeed())
}

func getDs(ctx context.Context, cli client.Client) (*appsv1.DaemonSet, error) {
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dsName,
			Namespace: tests.InstallNamespace,
		},
	}

	err := cli.Get(ctx, client.ObjectKeyFromObject(ds), ds)
	return ds, err
}
