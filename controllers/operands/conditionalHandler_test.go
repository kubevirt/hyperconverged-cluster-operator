package operands

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

var _ = Describe("Conditional Handler", func() {
	It("should delete the object once and not again while it is terminating", func() {
		hco := commontestutils.NewHco()
		req := commontestutils.NewReq(hco)

		dep := NewExpectedDeployment(hco)
		dep.Namespace = commontestutils.Namespace
		dep.Labels = map[string]string{hcoutil.AppLabel: hcoutil.HyperConvergedName}
		dep.Finalizers = []string{"test.kubevirt.io/finalizer"}
		cl := commontestutils.InitClient([]client.Object{hco, dep})

		handler := NewConditionalHandler(
			NewDeploymentHandler(cl, commontestutils.GetScheme(), NewExpectedDeployment, ""),
			func(_ *hcov1.HyperConverged) bool { return false },
			func(_ *hcov1.HyperConverged) client.Object { return dep.DeepCopy() },
		)

		res := handler.Ensure(req)
		Expect(res.Err).ToNot(HaveOccurred())
		Expect(res.Deleted).To(BeTrue())

		found := &appsv1.Deployment{}
		Expect(cl.Get(context.Background(), client.ObjectKeyFromObject(dep), found)).To(Succeed())
		Expect(found.DeletionTimestamp).ToNot(BeNil())

		res = handler.Ensure(req)
		Expect(res.Err).ToNot(HaveOccurred())
		Expect(res.Deleted).To(BeFalse())
	})
})
