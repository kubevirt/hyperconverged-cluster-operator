package tests_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	tests "github.com/kubevirt/hyperconverged-cluster-operator/tests/func-tests"
)

const (
	netResInjectorDeploymentName = "cnv-network-resources-injector"
)

var _ = Describe("Test Network Resources Injector", Label("NetResInjector"), Serial, Ordered, func() {
	tests.FlagParse()

	var cli client.Client

	BeforeAll(func(ctx context.Context) {
		cli = tests.GetControllerRuntimeClient()
	})

	AfterAll(func(ctx context.Context) {
		restoreNetResInjectorToDefault(ctx, cli)
	})

	Context("when deployNetworkResourcesInjector is true (default)", func() {
		It("should deploy the network resources injector", func(ctx context.Context) {
			By("verifying the deployment exists and is ready")
			Eventually(func(g Gomega, ctx context.Context) {
				dep := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      netResInjectorDeploymentName,
						Namespace: tests.InstallNamespace,
					},
				}
				g.Expect(cli.Get(ctx, client.ObjectKeyFromObject(dep), dep)).To(Succeed())
				g.Expect(dep.Status.ReadyReplicas).To(Equal(*dep.Spec.Replicas))
			}).WithTimeout(5 * time.Minute).WithPolling(time.Second).WithContext(ctx).Should(Succeed())

			By("verifying the deployment has control plane node affinity")
			dep := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      netResInjectorDeploymentName,
					Namespace: tests.InstallNamespace,
				},
			}
			Expect(cli.Get(ctx, client.ObjectKeyFromObject(dep), dep)).To(Succeed())

			affinity := dep.Spec.Template.Spec.Affinity
			Expect(affinity).NotTo(BeNil(), "deployment should have affinity set")
			Expect(affinity.NodeAffinity).NotTo(BeNil(), "deployment should have node affinity")
			Expect(affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution).NotTo(BeEmpty())

			// Verify it prefers control plane nodes
			foundControlPlaneSelector := false
			for _, preferred := range affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution {
				for _, expr := range preferred.Preference.MatchExpressions {
					if expr.Key == "node-role.kubernetes.io/control-plane" {
						foundControlPlaneSelector = true
						break
					}
				}
			}
			Expect(foundControlPlaneSelector).To(BeTrue(), "should prefer control plane nodes")
		})
	})

	Context("when deployNetworkResourcesInjector is false", func() {
		BeforeAll(func(ctx context.Context) {
			disableNetResInjector(ctx, cli)
		})

		It("should not deploy the network resources injector", func(ctx context.Context) {
			validateNetResInjectorDeleted(ctx, cli)
		})

		It("should recreate the deployment when set back to true", func(ctx context.Context) {
			enableNetResInjector(ctx, cli)

			By("verifying the deployment is recreated and ready")
			Eventually(func(g Gomega, ctx context.Context) {
				dep := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      netResInjectorDeploymentName,
						Namespace: tests.InstallNamespace,
					},
				}
				g.Expect(cli.Get(ctx, client.ObjectKeyFromObject(dep), dep)).To(Succeed())
				g.Expect(dep.Status.ReadyReplicas).To(Equal(*dep.Spec.Replicas))
			}).WithTimeout(5 * time.Minute).WithPolling(time.Second).WithContext(ctx).Should(Succeed())
		})
	})
})

func getNetResInjectorDeploymentErr(ctx context.Context, cli client.Client) error {
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      netResInjectorDeploymentName,
			Namespace: tests.InstallNamespace,
		},
	}
	return cli.Get(ctx, client.ObjectKeyFromObject(dep), dep)
}

func validateNetResInjectorDeleted(ctx context.Context, cli client.Client) {
	GinkgoHelper()
	Eventually(func(ctx context.Context) error {
		return getNetResInjectorDeploymentErr(ctx, cli)
	}).WithTimeout(2 * time.Minute).
		WithPolling(time.Second).
		WithContext(ctx).
		Should(MatchError(k8serrors.IsNotFound, "should be not-found error"))
}

func enableNetResInjector(ctx context.Context, cli client.Client) {
	GinkgoHelper()
	By("setting deployNetworkResourcesInjector to true")
	patch := []byte(`[{"op": "replace", "path": "/spec/deployment/deployNetworkResourcesInjector", "value": true}]`)
	tests.PatchHCO(ctx, cli, patch)
}

func disableNetResInjector(ctx context.Context, cli client.Client) {
	GinkgoHelper()
	By("setting deployNetworkResourcesInjector to false")
	patch := []byte(`[{"op": "add", "path": "/spec/deployment/deployNetworkResourcesInjector", "value": false}]`)
	tests.PatchHCO(ctx, cli, patch)
}

func restoreNetResInjectorToDefault(ctx context.Context, cli client.Client) {
	GinkgoHelper()
	By("restoring deployNetworkResourcesInjector to default")
	patch := []byte(`[{"op": "remove", "path": "/spec/deployment/deployNetworkResourcesInjector"}]`)
	tests.PatchHCO(ctx, cli, patch)

	// Wait for deployment to be recreated
	Eventually(func(g Gomega, ctx context.Context) {
		dep := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      netResInjectorDeploymentName,
				Namespace: tests.InstallNamespace,
			},
		}
		g.Expect(cli.Get(ctx, client.ObjectKeyFromObject(dep), dep)).To(Succeed())
		g.Expect(dep.Status.ReadyReplicas).To(Equal(*dep.Spec.Replicas))
	}).WithTimeout(5 * time.Minute).WithPolling(time.Second).WithContext(ctx).Should(Succeed())
}
