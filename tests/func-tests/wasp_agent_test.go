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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	wasp_agent "github.com/kubevirt/hyperconverged-cluster-operator/controllers/handlers/wasp-agent"
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

	workerMachineConfigPoolName = "worker"
	swapMachineConfig           = "90-worker-swap-online"
	machineConfigPoolAPIGroup   = "machineconfiguration.openshift.io"
	machineConfigPoolAPIVersion = "v1"
	machineConfigPoolKind       = "MachineConfigPool"
	machineConfigKind           = "MachineConfig"
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

		pauseWorkerMachineConfigPool(ctx, cli)
	})

	AfterAll(func(ctx context.Context) {
		removeAutopilotSwapAnnotation(ctx, cli)

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

		deleteSwapMachineConfig(ctx, cli)
		unpauseWorkerMachineConfigPool(ctx, cli)
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

	When("Autopilot swap annotation is set", Label(tests.DestructiveLabel), func() {
		It("should remove wasp-agent components when annotation is added", func(ctx context.Context) {
			setMemoryOvercommitPercentage(ctx, cli, overcommitPercent)

			By("first ensure wasp-agent components are deployed")
			Eventually(func(g Gomega, ctx context.Context) bool {
				ds, err := getWaspDS(ctx, cli)
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(ds.Status.DesiredNumberScheduled).ToNot(BeZero())
				return ds.Status.DesiredNumberScheduled == ds.Status.CurrentNumberScheduled
			}).WithTimeout(5 * time.Minute).WithPolling(time.Second).WithContext(ctx).Should(BeTrue())

			By("set the autopilot swap annotation")
			setAutopilotSwapAnnotation(ctx, cli)

			By("check the wasp-agent components are removed")
			validateDeleted(ctx, cli, func(ctx context.Context, cli client.Client) error {
				_, err := getWaspDS(ctx, cli)
				return err
			})
			validateDeleted(ctx, cli, getWaspSA)
			validateDeleted(ctx, cli, getWaspRole)
			validateDeleted(ctx, cli, getWaspRoleBinding)
			validateDeleted(ctx, cli, getWaspSCC)

		})

		It("should redeploy wasp-agent components when annotation is removed", func(ctx context.Context) {
			By("remove the autopilot swap annotation")
			removeAutopilotSwapAnnotation(ctx, cli)

			By("check the wasp-agent components are redeployed")
			Eventually(func(g Gomega, ctx context.Context) bool {
				ds, err := getWaspDS(ctx, cli)
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(ds.Status.DesiredNumberScheduled).ToNot(BeZero())
				return ds.Status.DesiredNumberScheduled == ds.Status.CurrentNumberScheduled
			}).WithTimeout(5 * time.Minute).WithPolling(time.Second).WithContext(ctx).Should(BeTrue())

			Eventually(func(ctx context.Context) error {
				return getWaspSA(ctx, cli)
			}).WithTimeout(1 * time.Minute).
				WithPolling(100 * time.Millisecond).
				WithContext(ctx).
				Should(Succeed())

			Eventually(func(ctx context.Context) error {
				return getWaspRole(ctx, cli)
			}).WithTimeout(1 * time.Minute).
				WithPolling(100 * time.Millisecond).
				WithContext(ctx).
				Should(Succeed())

			Eventually(func(ctx context.Context) error {
				return getWaspRoleBinding(ctx, cli)
			}).WithTimeout(1 * time.Minute).
				WithPolling(100 * time.Millisecond).
				WithContext(ctx).
				Should(Succeed())

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

func setAutopilotSwapAnnotation(ctx context.Context, cli client.Client) {
	GinkgoHelper()
	patchBytes := []byte(fmt.Sprintf(`{
		"metadata": {
			"annotations": {
				%q: %q
			}
		}
	}`, wasp_agent.AutopilotSwapAnnotation, wasp_agent.AutopilotSwapAnnotationValue))

	Eventually(func(g Gomega, ctx context.Context) {
		g.Expect(tests.PatchMergeHCO(ctx, cli, patchBytes)).To(Succeed())
	}).WithTimeout(30 * time.Second).
		WithPolling(time.Second).
		WithContext(ctx).
		Should(Succeed())

	Eventually(func(g Gomega, ctx context.Context) {
		hco := tests.GetHCO(ctx, cli)
		g.Expect(hco.Annotations).To(HaveKeyWithValue(wasp_agent.AutopilotSwapAnnotation, wasp_agent.AutopilotSwapAnnotationValue))
	}).WithTimeout(30 * time.Second).
		WithPolling(time.Second).
		WithContext(ctx).
		Should(Succeed())
}

func removeAutopilotSwapAnnotation(ctx context.Context, cli client.Client) {
	GinkgoHelper()
	patchBytes := []byte(fmt.Sprintf(`{
		"metadata": {
			"annotations": {
				%q: null
			}
		}
	}`, wasp_agent.AutopilotSwapAnnotation))

	Eventually(func(g Gomega, ctx context.Context) {
		g.Expect(tests.PatchMergeHCO(ctx, cli, patchBytes)).To(Succeed())
	}).WithTimeout(30 * time.Second).
		WithPolling(time.Second).
		WithContext(ctx).
		Should(Succeed())

	Eventually(func(g Gomega, ctx context.Context) {
		hco := tests.GetHCO(ctx, cli)
		g.Expect(hco.Annotations).ToNot(HaveKey(wasp_agent.AutopilotSwapAnnotation))
	}).WithTimeout(30 * time.Second).
		WithPolling(time.Second).
		WithContext(ctx).
		Should(Succeed())
}

func newWorkerMachineConfigPool() *unstructured.Unstructured {
	mcp := &unstructured.Unstructured{}
	mcp.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   machineConfigPoolAPIGroup,
		Version: machineConfigPoolAPIVersion,
		Kind:    machineConfigPoolKind,
	})
	mcp.SetName(workerMachineConfigPoolName)
	return mcp
}

func patchWorkerMachineConfigPoolPaused(ctx context.Context, cli client.Client, paused bool) {
	GinkgoHelper()
	mcp := newWorkerMachineConfigPool()
	patch := client.RawPatch(types.MergePatchType, []byte(fmt.Sprintf(`{"spec":{"paused":%t}}`, paused)))

	Eventually(func(g Gomega, ctx context.Context) {
		g.Expect(cli.Patch(ctx, mcp, patch)).To(Succeed())
	}).WithTimeout(30 * time.Second).
		WithPolling(time.Second).
		WithContext(ctx).
		Should(Succeed())

	Eventually(func(g Gomega, ctx context.Context) {
		current := newWorkerMachineConfigPool()
		g.Expect(cli.Get(ctx, client.ObjectKeyFromObject(current), current)).To(Succeed())
		val, found, err := unstructured.NestedBool(current.Object, "spec", "paused")
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(found).To(BeTrue(), "spec.paused field not found in MachineConfigPool")
		g.Expect(val).To(Equal(paused), "expected spec.paused to be %t", paused)
	}).WithTimeout(30 * time.Second).
		WithPolling(time.Second).
		WithContext(ctx).
		Should(Succeed())
}

func pauseWorkerMachineConfigPool(ctx context.Context, cli client.Client) {
	GinkgoHelper()
	patchWorkerMachineConfigPoolPaused(ctx, cli, true)
}

func unpauseWorkerMachineConfigPool(ctx context.Context, cli client.Client) {
	GinkgoHelper()
	patchWorkerMachineConfigPoolPaused(ctx, cli, false)
}

func deleteSwapMachineConfig(ctx context.Context, cli client.Client) {
	GinkgoHelper()
	mc := &unstructured.Unstructured{}
	mc.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   machineConfigPoolAPIGroup,
		Version: machineConfigPoolAPIVersion,
		Kind:    machineConfigKind,
	})
	mc.SetName(swapMachineConfig)

	err := cli.Get(ctx, client.ObjectKeyFromObject(mc), mc)
	if k8serrors.IsNotFound(err) {
		return
	}
	Expect(err).ToNot(HaveOccurred(), "unexpected error getting MachineConfig %q", swapMachineConfig)

	Eventually(func(ctx context.Context) error {
		err := cli.Delete(ctx, mc)
		if k8serrors.IsNotFound(err) {
			return nil
		}
		return err
	}).WithTimeout(1 * time.Minute).
		WithPolling(time.Second).
		WithContext(ctx).
		Should(Succeed())
}
