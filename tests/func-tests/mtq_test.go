package tests_test

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	conditionsv1 "github.com/openshift/custom-resource-status/conditions/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"kubevirt.io/client-go/kubecli"
	"kubevirt.io/kubevirt/tests/flags"
	mtqv1alpha1 "kubevirt.io/managed-tenant-quota/staging/src/kubevirt.io/managed-tenant-quota-api/pkg/apis/core/v1alpha1"

	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
	tests "github.com/kubevirt/hyperconverged-cluster-operator/tests/func-tests"
)

const (
	setMTQFGPatchTemplate = `[{"op": "replace", "path": "/spec/featureGates/enableManagedTenantQuota", "value": %t}]`
)

var _ = Describe("Test MTQ", Label("MTQ"), Serial, Ordered, func() {
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

		ctx = context.Background()

		disableMTQFeatureGate(ctx, cli)
	})

	AfterAll(func() {
		disableMTQFeatureGate(ctx, cli)
	})

	When("set the EnableManagedTenantQuota FG", func() {
		It("should create the MTQ CR and all the pods", func() {
			enableMTQFeatureGate(ctx, cli)

			By("check the MTQ CR")
			Eventually(func(g Gomega) bool {
				mtq := getMTQ(ctx, cli, g)
				g.Expect(mtq.Status.Conditions).ShouldNot(BeEmpty())
				return conditionsv1.IsStatusConditionTrue(mtq.Status.Conditions, conditionsv1.ConditionAvailable)
			}).WithTimeout(5 * time.Minute).WithPolling(time.Second).ShouldNot(BeTrue())

			By("check MTQ pods")
			Eventually(func(g Gomega) []corev1.Pod {
				pods, err := cli.CoreV1().Pods(flags.KubeVirtInstallNamespace).List(ctx, metav1.ListOptions{LabelSelector: "app.kubernetes.io/component=multi-tenant"})
				g.Expect(err).ShouldNot(HaveOccurred())
				return pods.Items
			}).WithTimeout(5 * time.Minute).
				WithPolling(time.Second).
				Should(HaveLen(3))
		})
	})
})

func getMTQ(ctx context.Context, cli kubecli.KubevirtClient, g Gomega) *mtqv1alpha1.MTQ {
	mtq := &mtqv1alpha1.MTQ{}

	unstMTQ, err := getMTQResource(ctx, cli)
	g.ExpectWithOffset(1, err).ToNot(HaveOccurred())
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(unstMTQ.Object, mtq)
	g.ExpectWithOffset(1, err).ToNot(HaveOccurred())

	return mtq
}

func getMTQResource(ctx context.Context, client kubecli.KubevirtClient) (*unstructured.Unstructured, error) {
	mtqGVR := schema.GroupVersionResource{Group: mtqv1alpha1.SchemeGroupVersion.Group, Version: mtqv1alpha1.SchemeGroupVersion.Version, Resource: "mtqs"}

	return client.DynamicClient().Resource(mtqGVR).Get(ctx, "mtq-"+hcoutil.HyperConvergedName, metav1.GetOptions{})
}

func enableMTQFeatureGate(ctx context.Context, cli kubecli.KubevirtClient) {
	By("enable the MTQ FG")
	setMTQFeatureGate(ctx, cli, true)
}

func disableMTQFeatureGate(ctx context.Context, cli kubecli.KubevirtClient) {
	By("disable the MTQ FG")
	setMTQFeatureGate(ctx, cli, false)

	By("make sure the MTQ CR was removed")
	Eventually(func(g Gomega) bool {
		_, err := getMTQResource(ctx, cli)
		g.Expect(err).To(HaveOccurred())
		return errors.IsNotFound(err)
	}).WithTimeout(5 * time.Minute).
		WithPolling(100 * time.Millisecond).
		WithOffset(1).
		Should(BeTrue())
}

func setMTQFeatureGate(ctx context.Context, cli kubecli.KubevirtClient, fgState bool) {
	patch := []byte(fmt.Sprintf(setMTQFGPatchTemplate, fgState))
	Eventually(tests.PatchHCO).
		WithArguments(ctx, cli, patch).
		WithTimeout(10 * time.Second).
		WithPolling(100 * time.Millisecond).
		WithOffset(2).
		Should(Succeed())
}
