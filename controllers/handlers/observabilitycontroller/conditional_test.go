package observabilitycontroller

import (
	"context"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

type handlerTestEntry struct {
	newHandler func(client.Client, *runtime.Scheme) operands.Operand
	existing   func() client.Object
}


func setupImageEnv() {
	origImage, origImageSet := os.LookupEnv(hcoutil.ObservabilityControllerImageEnvV)
	Expect(os.Setenv(hcoutil.ObservabilityControllerImageEnvV, "quay.io/kubevirt/virt-observability-controller:test")).To(Succeed())
	DeferCleanup(func() {
		if origImageSet {
			Expect(os.Setenv(hcoutil.ObservabilityControllerImageEnvV, origImage)).To(Succeed())
		} else {
			Expect(os.Unsetenv(hcoutil.ObservabilityControllerImageEnvV)).To(Succeed())
		}
	})
}

var _ = Describe("Observability Controller Handlers", func() {
	var (
		hco *hcov1.HyperConverged
		req *common.HcoRequest
	)

	BeforeEach(func() {
		hco = commontestutils.NewHco()
		req = commontestutils.NewReq(hco)
		setupImageEnv()
	})

	DescribeTableSubtree("handler lifecycle",
		func(entry handlerTestEntry) {
			It("should create when deployObservabilityController feature gate is enabled", func() {
				hco.Spec.FeatureGates.Enable(featureGateName)
				cl := commontestutils.InitClient([]client.Object{hco})

				handler := entry.newHandler(cl, commontestutils.GetScheme())
				res := handler.Ensure(req)

				Expect(res.Err).ToNot(HaveOccurred())
				Expect(res.Created).To(BeTrue())

				cr := entry.existing()
				Expect(cl.Get(context.Background(), client.ObjectKeyFromObject(cr), cr)).To(Succeed())
			})

			It("should not create when deployObservabilityController feature gate is not set", func() {
				cl := commontestutils.InitClient([]client.Object{hco})

				handler := entry.newHandler(cl, commontestutils.GetScheme())
				res := handler.Ensure(req)

				Expect(res.Err).ToNot(HaveOccurred())
				Expect(res.Created).To(BeFalse())

				cr := entry.existing()
				err := cl.Get(context.Background(), client.ObjectKeyFromObject(cr), cr)
				Expect(err).To(MatchError(apierrors.IsNotFound, "not found error"))
			})

			It("should delete when deployObservabilityController feature gate is disabled", func() {
				hco.Spec.FeatureGates.Disable(featureGateName)
				cr := entry.existing()
				crKey := client.ObjectKeyFromObject(cr)
				cl := commontestutils.InitClient([]client.Object{hco, cr})

				Expect(cl.Get(context.Background(), crKey, entry.existing())).To(Succeed())

				handler := entry.newHandler(cl, commontestutils.GetScheme())
				res := handler.Ensure(req)

				Expect(res.Err).ToNot(HaveOccurred())
				Expect(res.Deleted).To(BeTrue())

				err := cl.Get(context.Background(), crKey, entry.existing())
				Expect(err).To(MatchError(apierrors.IsNotFound, "not found error"))
			})

		},
		Entry("ClusterRole", handlerTestEntry{
			newHandler: func(cl client.Client, s *runtime.Scheme) operands.Operand { return NewClusterRoleHandler(cl, s) },
			existing:   func() client.Object { return newClusterRole() },
		}),
		Entry("ClusterRoleBinding", handlerTestEntry{
			newHandler: func(cl client.Client, s *runtime.Scheme) operands.Operand { return NewClusterRoleBindingHandler(cl, s) },
			existing:   func() client.Object { return newClusterRoleBinding() },
		}),
		Entry("ServiceAccount", handlerTestEntry{
			newHandler: func(cl client.Client, s *runtime.Scheme) operands.Operand { return NewServiceAccountHandler(cl, s) },
			existing:   func() client.Object { return newServiceAccount() },
		}),
		Entry("Deployment", handlerTestEntry{
			newHandler: func(cl client.Client, s *runtime.Scheme) operands.Operand { return NewDeploymentHandler(cl, s) },
			existing:   func() client.Object { return newDeployment(commontestutils.NewHco()) },
		}),
	)
})

var _ = Describe("Handler resource verification", func() {
	BeforeEach(func() {
		setupImageEnv()
	})

	It("should create ClusterRole with correct rules", func() {
		hco := commontestutils.NewHco()
		hco.Spec.FeatureGates.Enable(featureGateName)
		req := commontestutils.NewReq(hco)
		cl := commontestutils.InitClient([]client.Object{hco})

		handler := NewClusterRoleHandler(cl, commontestutils.GetScheme())
		res := handler.Ensure(req)
		Expect(res.Err).ToNot(HaveOccurred())

		foundCR := newClusterRoleWithNameOnly()
		Expect(cl.Get(context.Background(), client.ObjectKeyFromObject(foundCR), foundCR)).To(Succeed())
		Expect(foundCR.Rules).To(HaveLen(9))
	})

	It("should create Deployment with correct replicas", func() {
		hco := commontestutils.NewHco()
		hco.Spec.FeatureGates.Enable(featureGateName)
		req := commontestutils.NewReq(hco)
		cl := commontestutils.InitClient([]client.Object{hco})

		handler := NewDeploymentHandler(cl, commontestutils.GetScheme())
		res := handler.Ensure(req)
		Expect(res.Err).ToNot(HaveOccurred())

		foundDep := NewDeploymentWithNameOnly()
		Expect(cl.Get(context.Background(), client.ObjectKeyFromObject(foundDep), foundDep)).To(Succeed())
		Expect(foundDep.Spec.Replicas).To(HaveValue(Equal(int32(1))))
	})

	It("should create ServiceAccount in correct namespace", func() {
		hco := commontestutils.NewHco()
		hco.Spec.FeatureGates.Enable(featureGateName)
		req := commontestutils.NewReq(hco)
		cl := commontestutils.InitClient([]client.Object{hco})

		handler := NewServiceAccountHandler(cl, commontestutils.GetScheme())
		res := handler.Ensure(req)
		Expect(res.Err).ToNot(HaveOccurred())

		foundSA := newServiceAccount()
		Expect(cl.Get(context.Background(), client.ObjectKeyFromObject(foundSA), foundSA)).To(Succeed())
	})
})
