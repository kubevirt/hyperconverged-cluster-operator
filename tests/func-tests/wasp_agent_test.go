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
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
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

		Expect(securityv1.Install(cli.Scheme())).To(Succeed())

		originalHco := tests.GetHCO(ctx, cli)
		if originalHco.Spec.HigherWorkloadDensity != nil {
			originalOvercommitPercent = originalHco.Spec.HigherWorkloadDensity.MemoryOvercommitPercentage
		}
	})

	AfterAll(func(ctx context.Context) {
		// Reset the memory overcommit percentage to the original value
		if originalOvercommitPercent != 0 {
			setMemoryOvercommitPercentage(ctx, cli, originalOvercommitPercent)
		}

		validateDeleted(ctx, cli, func(ctx context.Context, cli client.Client) error {
			_, err := getWaspDS(ctx, cli)
			return err
		})
		validateDeleted(ctx, cli, getWaspSA)
		validateDeleted(ctx, cli, getWaspRole)
		validateDeleted(ctx, cli, getWaspRoleBinding)
		validateDeleted(ctx, cli, getWaspSCC)
	})

	When("Higher density is set beyond 100 percent", func() {
		It("should deploy wasp-agent components", func(ctx context.Context) {
			setMemoryOvercommitPercentage(ctx, cli, overcommitPercent)

			By("check the wasp-agent Daemonset")
			Eventually(func(g Gomega, ctx context.Context) bool {
				ds, err := getWaspDS(ctx, cli)
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(ds.Status.DesiredNumberScheduled).ToNot(BeZero())
				return ds.Status.DesiredNumberScheduled == ds.Status.CurrentNumberScheduled
			}).WithTimeout(5 * time.Minute).WithPolling(time.Second).WithContext(ctx).Should(BeTrue())

			By("check the wasp-agent SA")
			Eventually(func(ctx context.Context) error {
				return getWaspSA(ctx, cli)
			}).WithTimeout(1 * time.Minute).
				WithPolling(100 * time.Millisecond).
				WithContext(ctx).
				Should(Succeed())

			By("check the wasp-agent cluster role")
			Eventually(func(ctx context.Context) error {
				return getWaspRole(ctx, cli)
			}).WithTimeout(1 * time.Minute).
				WithPolling(100 * time.Millisecond).
				WithContext(ctx).
				Should(Succeed())

			By("check the wasp-agent cluster role binding")
			Eventually(func(ctx context.Context) error {
				return getWaspRoleBinding(ctx, cli)
			}).WithTimeout(1 * time.Minute).
				WithPolling(100 * time.Millisecond).
				WithContext(ctx).
				Should(Succeed())

			By("check the wasp-agent SecurityContextConstraints")
			Eventually(func(ctx context.Context) error {
				return getWaspSCC(ctx, cli)
			}).WithTimeout(1 * time.Minute).
				WithPolling(100 * time.Millisecond).
				WithContext(ctx).
				Should(Succeed())
		})
	})
})

type tryGetResourceFn func(ctx context.Context, cli client.Client) error

func validateDeleted(ctx context.Context, cli client.Client, tryGetResource tryGetResourceFn) {
	GinkgoHelper()
	Eventually(func(ctx context.Context) error {
		return tryGetResource(ctx, cli)
	}).WithTimeout(2 * time.Minute).
		WithPolling(time.Second).
		WithContext(ctx).
		Should(MatchError(k8serrors.IsNotFound, "should be not-found error"))
}

// only check if can get or not
func getWaspSCC(ctx context.Context, cli client.Client) error {
	scc := &securityv1.SecurityContextConstraints{
		ObjectMeta: metav1.ObjectMeta{
			Name: sccName,
		},
	}
	return cli.Get(ctx, client.ObjectKeyFromObject(scc), scc)
}

func getWaspRoleBinding(ctx context.Context, cli client.Client) error {
	binding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterRoleBindingName,
		},
	}
	return cli.Get(ctx, client.ObjectKeyFromObject(binding), binding)
}

func getWaspRole(ctx context.Context, cli client.Client) error {
	role := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterRoleName,
		},
	}
	return cli.Get(ctx, client.ObjectKeyFromObject(role), role)
}

func getWaspSA(ctx context.Context, cli client.Client) error {
	sa := &v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      saName,
			Namespace: tests.InstallNamespace,
		},
	}
	return cli.Get(ctx, client.ObjectKeyFromObject(sa), sa)
}

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

func getWaspDS(ctx context.Context, cli client.Client) (*appsv1.DaemonSet, error) {
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dsName,
			Namespace: tests.InstallNamespace,
		},
	}

	err := cli.Get(ctx, client.ObjectKeyFromObject(ds), ds)
	return ds, err
}
