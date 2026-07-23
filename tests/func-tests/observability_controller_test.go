package tests_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	tests "github.com/kubevirt/hyperconverged-cluster-operator/tests/func-tests"
)

const (
	observabilityControllerName       = "virt-observability-controller"
	observabilityControllerCRBName    = "virt-observability-controller-rolebinding"
	observabilityControllerFGName     = "deployObservabilityController"
)

var _ = Describe("Observability Controller", Label("observability-controller"), Serial, Ordered, func() {
	tests.FlagParse()

	var cli client.Client

	BeforeAll(func(ctx context.Context) {
		cli = tests.GetControllerRuntimeClient()
	})

	AfterAll(func(ctx context.Context) {
		tests.RestoreDefaultFeatureGates(ctx, cli)
	})

	When("deployObservabilityController feature gate is enabled", func() {
		It("should deploy observability controller resources", func(ctx context.Context) {
			By("enabling the deployObservabilityController feature gate")
			Expect(tests.EnableFG(ctx, cli, observabilityControllerFGName)).To(Succeed())

			By("checking the ServiceAccount is created")
			Eventually(func(ctx context.Context) error {
				sa := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      observabilityControllerName,
						Namespace: tests.InstallNamespace,
					},
				}
				return cli.Get(ctx, client.ObjectKeyFromObject(sa), sa)
			}).WithTimeout(2 * time.Minute).WithPolling(time.Second).WithContext(ctx).Should(Succeed())

			By("checking the ClusterRole is created")
			Eventually(func(ctx context.Context) error {
				cr := &rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{
						Name: observabilityControllerName,
					},
				}
				return cli.Get(ctx, client.ObjectKeyFromObject(cr), cr)
			}).WithTimeout(time.Minute).WithPolling(time.Second).WithContext(ctx).Should(Succeed())

			By("checking the ClusterRoleBinding is created")
			Eventually(func(ctx context.Context) error {
				crb := &rbacv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name: observabilityControllerCRBName,
					},
				}
				return cli.Get(ctx, client.ObjectKeyFromObject(crb), crb)
			}).WithTimeout(time.Minute).WithPolling(time.Second).WithContext(ctx).Should(Succeed())

			By("checking the Deployment is created")
			Eventually(func(ctx context.Context) error {
				dep := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      observabilityControllerName,
						Namespace: tests.InstallNamespace,
					},
				}
				return cli.Get(ctx, client.ObjectKeyFromObject(dep), dep)
			}).WithTimeout(2 * time.Minute).WithPolling(time.Second).WithContext(ctx).Should(Succeed())
		})
	})

	When("deployObservabilityController feature gate is disabled", func() {
		It("should remove observability controller resources", func(ctx context.Context) {
			By("disabling the deployObservabilityController feature gate")
			tests.RestoreDefaultFeatureGates(ctx, cli)

			By("checking the Deployment is removed")
			Eventually(func(ctx context.Context) error {
				dep := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      observabilityControllerName,
						Namespace: tests.InstallNamespace,
					},
				}
				return cli.Get(ctx, client.ObjectKeyFromObject(dep), dep)
			}).WithTimeout(2 * time.Minute).WithPolling(time.Second).WithContext(ctx).Should(
				MatchError(ContainSubstring("not found")),
			)

			By("checking the ClusterRole is removed")
			Eventually(func(ctx context.Context) error {
				cr := &rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{
						Name: observabilityControllerName,
					},
				}
				return cli.Get(ctx, client.ObjectKeyFromObject(cr), cr)
			}).WithTimeout(time.Minute).WithPolling(time.Second).WithContext(ctx).Should(
				MatchError(ContainSubstring("not found")),
			)
		})
	})
})
