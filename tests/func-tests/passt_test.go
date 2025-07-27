package tests_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	netattdefv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"

	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/handlers/passt"
	tests "github.com/kubevirt/hyperconverged-cluster-operator/tests/func-tests"
)

const (
	passtCNIObjectName         = "passt-binding-cni"
	networkBindingNADName      = "primary-udn-kubevirt-binding"
	networkBindingNADNamespace = "default"
)

var _ = Describe("Test Passt Network Binding", Label("Passt"), Serial, Ordered, func() {
	tests.FlagParse()
	var (
		k8scli client.Client
	)

	BeforeEach(func(ctx context.Context) {
		k8scli = tests.GetControllerRuntimeClient()

		Expect(netattdefv1.AddToScheme(k8scli.Scheme())).To(Succeed())
	})

	AfterEach(func(ctx context.Context) {
		By("disabling the passt deployment annotation")
		unsetPasstDeploymentAnnotation(ctx, k8scli)
	})

	When("deployPasstNetworkBinding annotation is set to true", func() {
		It("should create the passt DaemonSet and NetworkAttachmentDefinition", func(ctx context.Context) {
			By("enabling the passt deployment annotation")
			setPasstDeploymentAnnotation(ctx, k8scli)

			By("checking that the passt DaemonSet is created and ready")
			Eventually(func(g Gomega, ctx context.Context) {
				ds := &appsv1.DaemonSet{}
				g.Expect(k8scli.Get(ctx, types.NamespacedName{
					Name:      passtCNIObjectName,
					Namespace: tests.InstallNamespace,
				}, ds)).To(Succeed())
				g.Expect(ds.Status.NumberReady).To(BeNumerically(">", 0))
				g.Expect(ds.Status.DesiredNumberScheduled).To(Equal(ds.Status.NumberReady))
			}).WithTimeout(2 * time.Minute).
				WithPolling(10 * time.Second).
				WithContext(ctx).
				Should(Succeed())

			By("checking that the passt NetworkAttachmentDefinition is created")
			Eventually(func(g Gomega, ctx context.Context) {
				nad := &netattdefv1.NetworkAttachmentDefinition{}
				g.Expect(k8scli.Get(ctx, types.NamespacedName{
					Name:      networkBindingNADName,
					Namespace: networkBindingNADNamespace,
				}, nad)).To(Succeed())
				g.Expect(nad.Spec.Config).ToNot(BeEmpty())
			}).WithTimeout(2 * time.Minute).
				WithPolling(5 * time.Second).
				WithContext(ctx).
				Should(Succeed())
		})

		It("should remove passt resources when annotation is disabled", func(ctx context.Context) {
			By("enabling the passt deployment annotation")
			setPasstDeploymentAnnotation(ctx, k8scli)

			By("waiting for resources to be created")
			Eventually(func(g Gomega, ctx context.Context) {
				ds := &appsv1.DaemonSet{}
				g.Expect(k8scli.Get(ctx, types.NamespacedName{
					Name:      passtCNIObjectName,
					Namespace: tests.InstallNamespace,
				}, ds)).To(Succeed())
			}).WithTimeout(2 * time.Minute).
				WithPolling(5 * time.Second).
				WithContext(ctx).
				Should(Succeed())

			By("disabling the passt deployment annotation")
			unsetPasstDeploymentAnnotation(ctx, k8scli)

			By("checking that the passt DaemonSet is removed")
			Eventually(func(g Gomega, ctx context.Context) {
				ds := &appsv1.DaemonSet{}
				err := k8scli.Get(ctx, types.NamespacedName{
					Name:      passtCNIObjectName,
					Namespace: tests.InstallNamespace,
				}, ds)
				g.Expect(err).To(MatchError(errors.IsNotFound, "error should be NotFound"))
			}).WithTimeout(2 * time.Minute).
				WithPolling(5 * time.Second).
				WithContext(ctx).
				Should(Succeed())

			By("checking that the passt NetworkAttachmentDefinition is removed")
			Eventually(func(g Gomega, ctx context.Context) {
				nad := &netattdefv1.NetworkAttachmentDefinition{}
				err := k8scli.Get(ctx, types.NamespacedName{
					Name:      networkBindingNADName,
					Namespace: networkBindingNADNamespace,
				}, nad)
				g.Expect(err).To(MatchError(errors.IsNotFound, "error should be NotFound"))
			}).WithTimeout(2 * time.Minute).
				WithPolling(5 * time.Second).
				WithContext(ctx).
				Should(Succeed())
		})
	})
})

func setPasstDeploymentAnnotation(ctx context.Context, cli client.Client) {
	patchBytes := []byte(`{
		"metadata": {
			"annotations": {
				"hco.kubevirt.io/deployPasstNetworkBinding": "true"
			}
		}
	}`)

	Eventually(func(g Gomega, ctx context.Context) {
		g.Expect(tests.PatchMergeHCO(ctx, cli, patchBytes)).To(Succeed())
	}).WithTimeout(30 * time.Second).
		WithPolling(time.Second).
		WithContext(ctx).
		Should(Succeed())

	Eventually(func(g Gomega, ctx context.Context) {
		hco := tests.GetHCO(ctx, cli)
		g.Expect(hco.Annotations).To(HaveKeyWithValue(passt.DeployPasstNetworkBindingAnnotation, "true"))
	}).WithTimeout(30 * time.Second).
		WithPolling(time.Second).
		WithContext(ctx).
		Should(Succeed())
}

func unsetPasstDeploymentAnnotation(ctx context.Context, cli client.Client) {
	patchBytes := []byte(`{
		"metadata": {
			"annotations": {
				"hco.kubevirt.io/deployPasstNetworkBinding": null
			}
		}
	}`)

	Eventually(func(g Gomega, ctx context.Context) {
		g.Expect(tests.PatchMergeHCO(ctx, cli, patchBytes)).To(Succeed())
	}).WithTimeout(30 * time.Second).
		WithPolling(time.Second).
		WithContext(ctx).
		Should(Succeed())

	Eventually(func(g Gomega, ctx context.Context) {
		hco := tests.GetHCO(ctx, cli)
		g.Expect(hco.Annotations).ToNot(HaveKey(passt.DeployPasstNetworkBindingAnnotation))
	}).WithTimeout(30 * time.Second).
		WithPolling(time.Second).
		WithContext(ctx).
		Should(Succeed())
}
