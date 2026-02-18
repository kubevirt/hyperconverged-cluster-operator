package tests_test

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	tests "github.com/kubevirt/hyperconverged-cluster-operator/tests/func-tests"
)

const (
	aieWebhookName               = "kubevirt-aie-webhook"
	aieWebhookConfigMapName      = "kubevirt-aie-launcher-config"
	setAIEWebhookFGPatchTemplate = `[{"op": "replace", "path": "/spec/featureGates/deployAIEWebhook", "value": %t}]`
	setAIEWebhookConfigPatch     = `[
		{"op": "replace", "path": "/spec/featureGates/deployAIEWebhook", "value": true},
		{"op": "add", "path": "/spec/aieWebhookConfig", "value": {
			"rules": [{
				"name": "test-rule",
				"image": "quay.io/test/virt-launcher:latest",
				"selector": {
					"deviceNames": ["nvidia.com/TEST_GPU"]
				}
			}]
		}}
	]`
)

var _ = Describe("Test AIE Webhook", Label("AIEWebhook"), Serial, Ordered, func() {
	tests.FlagParse()

	var cli client.Client

	BeforeAll(func(ctx context.Context) {
		cli = tests.GetControllerRuntimeClient()

		Expect(admissionregistrationv1.AddToScheme(cli.Scheme())).To(Succeed())
	})

	AfterAll(func(ctx context.Context) {
		disableAIEWebhookFeatureGate(ctx, cli)

		validateAIEDeleted(ctx, cli, getAIEWebhookDeploymentErr)
		validateAIEDeleted(ctx, cli, getAIEWebhookServiceErr)
		validateAIEDeleted(ctx, cli, getAIEWebhookServiceAccountErr)
		validateAIEDeleted(ctx, cli, getAIEWebhookConfigMapErr)
		validateAIEDeleted(ctx, cli, getAIEWebhookClusterRoleErr)
		validateAIEDeleted(ctx, cli, getAIEWebhookClusterRoleBindingErr)
		validateAIEDeleted(ctx, cli, getAIEWebhookMutatingWebhookConfigurationErr)
	})

	When("deployAIEWebhook feature gate is enabled", func() {
		It("should deploy AIE webhook components", func(ctx context.Context) {
			enableAIEWebhookFeatureGate(ctx, cli)

			By("check the AIE webhook Deployment")
			Eventually(func(g Gomega, ctx context.Context) {
				dep := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      aieWebhookName,
						Namespace: tests.InstallNamespace,
					},
				}
				g.Expect(cli.Get(ctx, client.ObjectKeyFromObject(dep), dep)).To(Succeed())
				g.Expect(dep.Status.ReadyReplicas).To(Equal(*dep.Spec.Replicas))
			}).WithTimeout(5 * time.Minute).WithPolling(time.Second).WithContext(ctx).Should(Succeed())

			By("check the AIE webhook Service")
			Eventually(func(ctx context.Context) error {
				return getAIEWebhookServiceErr(ctx, cli)
			}).WithTimeout(1 * time.Minute).
				WithPolling(100 * time.Millisecond).
				WithContext(ctx).
				Should(Succeed())

			By("check the AIE webhook ServiceAccount")
			Eventually(func(ctx context.Context) error {
				return getAIEWebhookServiceAccountErr(ctx, cli)
			}).WithTimeout(1 * time.Minute).
				WithPolling(100 * time.Millisecond).
				WithContext(ctx).
				Should(Succeed())

			By("check the AIE webhook ConfigMap")
			Eventually(func(ctx context.Context) error {
				return getAIEWebhookConfigMapErr(ctx, cli)
			}).WithTimeout(1 * time.Minute).
				WithPolling(100 * time.Millisecond).
				WithContext(ctx).
				Should(Succeed())

			By("check the AIE webhook ClusterRole")
			Eventually(func(ctx context.Context) error {
				return getAIEWebhookClusterRoleErr(ctx, cli)
			}).WithTimeout(1 * time.Minute).
				WithPolling(100 * time.Millisecond).
				WithContext(ctx).
				Should(Succeed())

			By("check the AIE webhook ClusterRoleBinding")
			Eventually(func(ctx context.Context) error {
				return getAIEWebhookClusterRoleBindingErr(ctx, cli)
			}).WithTimeout(1 * time.Minute).
				WithPolling(100 * time.Millisecond).
				WithContext(ctx).
				Should(Succeed())

			By("check the AIE webhook MutatingWebhookConfiguration")
			Eventually(func(ctx context.Context) error {
				return getAIEWebhookMutatingWebhookConfigurationErr(ctx, cli)
			}).WithTimeout(1 * time.Minute).
				WithPolling(100 * time.Millisecond).
				WithContext(ctx).
				Should(Succeed())
		})

		It("should update the ConfigMap when aieWebhookConfig rules change", func(ctx context.Context) {
			By("setting AIE webhook config with a test rule")
			patchBytes := []byte(setAIEWebhookConfigPatch)
			Eventually(tests.PatchHCO).
				WithArguments(ctx, cli, patchBytes).
				WithTimeout(10 * time.Second).
				WithPolling(100 * time.Millisecond).
				WithContext(ctx).
				Should(Succeed())

			By("checking the ConfigMap contains the rule")
			Eventually(func(g Gomega, ctx context.Context) {
				cm := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      aieWebhookConfigMapName,
						Namespace: tests.InstallNamespace,
					},
				}
				g.Expect(cli.Get(ctx, client.ObjectKeyFromObject(cm), cm)).To(Succeed())
				g.Expect(cm.Data).To(HaveKey("config.yaml"))
				g.Expect(cm.Data["config.yaml"]).To(ContainSubstring("test-rule"))
				g.Expect(cm.Data["config.yaml"]).To(ContainSubstring("nvidia.com/TEST_GPU"))
			}).WithTimeout(2 * time.Minute).
				WithPolling(time.Second).
				WithContext(ctx).
				Should(Succeed())
		})

		It("should remove AIE webhook resources when feature gate is disabled", func(ctx context.Context) {
			enableAIEWebhookFeatureGate(ctx, cli)

			By("waiting for the Deployment to be created")
			Eventually(func(ctx context.Context) error {
				return getAIEWebhookDeploymentErr(ctx, cli)
			}).WithTimeout(2 * time.Minute).
				WithPolling(time.Second).
				WithContext(ctx).
				Should(Succeed())

			By("disabling the AIE webhook feature gate")
			disableAIEWebhookFeatureGate(ctx, cli)

			By("checking that all AIE webhook resources are removed")
			validateAIEDeleted(ctx, cli, getAIEWebhookDeploymentErr)
			validateAIEDeleted(ctx, cli, getAIEWebhookServiceErr)
			validateAIEDeleted(ctx, cli, getAIEWebhookServiceAccountErr)
			validateAIEDeleted(ctx, cli, getAIEWebhookConfigMapErr)
			validateAIEDeleted(ctx, cli, getAIEWebhookClusterRoleErr)
			validateAIEDeleted(ctx, cli, getAIEWebhookClusterRoleBindingErr)
			validateAIEDeleted(ctx, cli, getAIEWebhookMutatingWebhookConfigurationErr)
		})
	})
})

type aieGetResourceFn func(ctx context.Context, cli client.Client) error

func validateAIEDeleted(ctx context.Context, cli client.Client, tryGetResource aieGetResourceFn) {
	GinkgoHelper()
	Eventually(func(ctx context.Context) error {
		return tryGetResource(ctx, cli)
	}).WithTimeout(2 * time.Minute).
		WithPolling(time.Second).
		WithContext(ctx).
		Should(MatchError(k8serrors.IsNotFound, "should be not-found error"))
}

func enableAIEWebhookFeatureGate(ctx context.Context, cli client.Client) {
	GinkgoHelper()
	setAIEWebhookFeatureGate(ctx, cli, true)
}

func disableAIEWebhookFeatureGate(ctx context.Context, cli client.Client) {
	GinkgoHelper()
	setAIEWebhookFeatureGate(ctx, cli, false)
}

func setAIEWebhookFeatureGate(ctx context.Context, cli client.Client, fgState bool) {
	GinkgoHelper()
	patchBytes := []byte(fmt.Sprintf(setAIEWebhookFGPatchTemplate, fgState))

	Eventually(tests.PatchHCO).
		WithArguments(ctx, cli, patchBytes).
		WithTimeout(10 * time.Second).
		WithPolling(100 * time.Millisecond).
		WithContext(ctx).
		WithOffset(1).
		Should(Succeed())
}

func getAIEWebhookDeploymentErr(ctx context.Context, cli client.Client) error {
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      aieWebhookName,
			Namespace: tests.InstallNamespace,
		},
	}
	return cli.Get(ctx, client.ObjectKeyFromObject(dep), dep)
}

func getAIEWebhookServiceErr(ctx context.Context, cli client.Client) error {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      aieWebhookName,
			Namespace: tests.InstallNamespace,
		},
	}
	return cli.Get(ctx, client.ObjectKeyFromObject(svc), svc)
}

func getAIEWebhookServiceAccountErr(ctx context.Context, cli client.Client) error {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      aieWebhookName,
			Namespace: tests.InstallNamespace,
		},
	}
	return cli.Get(ctx, client.ObjectKeyFromObject(sa), sa)
}

func getAIEWebhookConfigMapErr(ctx context.Context, cli client.Client) error {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      aieWebhookConfigMapName,
			Namespace: tests.InstallNamespace,
		},
	}
	return cli.Get(ctx, client.ObjectKeyFromObject(cm), cm)
}

func getAIEWebhookClusterRoleErr(ctx context.Context, cli client.Client) error {
	cr := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: aieWebhookName,
		},
	}
	return cli.Get(ctx, client.ObjectKeyFromObject(cr), cr)
}

func getAIEWebhookClusterRoleBindingErr(ctx context.Context, cli client.Client) error {
	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: aieWebhookName,
		},
	}
	return cli.Get(ctx, client.ObjectKeyFromObject(crb), crb)
}

func getAIEWebhookMutatingWebhookConfigurationErr(ctx context.Context, cli client.Client) error {
	mwc := &admissionregistrationv1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: aieWebhookName,
		},
	}
	return cli.Get(ctx, client.ObjectKeyFromObject(mwc), mwc)
}
