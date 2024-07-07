package observability_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/observability"
)

var _ = Describe("Observability Controller", func() {
	Context("When reconciling a resource", func() {
		ctx := context.Background()

		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			k8sClient := commontestutils.InitClient([]client.Object{})
			controllerReconciler := &observability.Reconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{})
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
