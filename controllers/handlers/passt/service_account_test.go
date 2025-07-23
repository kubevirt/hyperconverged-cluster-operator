package passt_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/handlers/passt"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

var _ = Describe("Passt ServiceAccount tests", func() {
	var (
		hco *hcov1beta1.HyperConverged
		req *common.HcoRequest
		cl  client.Client
	)

	BeforeEach(func() {
		hco = commontestutils.NewHco()
		hco.Annotations = make(map[string]string)
		req = commontestutils.NewReq(hco)
	})

	Context("test NewPasstBindingCNISA", func() {
		It("should have all default fields", func() {
			sa := passt.NewPasstBindingCNISA(hco)

			Expect(sa.Name).To(Equal("passt-binding-cni"))
			Expect(sa.Namespace).To(Equal(hco.Namespace))
			Expect(sa.Labels).To(HaveKeyWithValue(hcoutil.AppLabel, hcoutil.HyperConvergedName))
			Expect(sa.Labels).To(HaveKeyWithValue(hcoutil.AppLabelComponent, string(hcoutil.AppComponentNetwork)))
		})
	})

	Context("ServiceAccount deployment", func() {
		It("should not create ServiceAccount if the annotation is not set", func() {
			cl = commontestutils.InitClient([]client.Object{hco})

			handler := passt.NewPasstServiceAccountHandler(cl, commontestutils.GetScheme())

			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Created).To(BeFalse())
			Expect(res.Updated).To(BeFalse())
			Expect(res.Deleted).To(BeFalse())

			foundSAs := &corev1.ServiceAccountList{}
			Expect(cl.List(context.Background(), foundSAs)).To(Succeed())
			Expect(foundSAs.Items).To(BeEmpty())
		})

		It("should delete ServiceAccount if the deployPasstNetworkBinding annotation is not set", func() {
			sa := passt.NewPasstBindingCNISA(hco)
			cl = commontestutils.InitClient([]client.Object{hco, sa})

			handler := passt.NewPasstServiceAccountHandler(cl, commontestutils.GetScheme())

			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Name).To(Equal(sa.Name))
			Expect(res.Created).To(BeFalse())
			Expect(res.Updated).To(BeFalse())
			Expect(res.Deleted).To(BeTrue())

			foundSAs := &corev1.ServiceAccountList{}
			Expect(cl.List(context.Background(), foundSAs)).To(Succeed())
			Expect(foundSAs.Items).To(BeEmpty())
		})

		It("should create ServiceAccount if the deployPasstNetworkBinding annotation is true", func() {
			hco.Annotations[passt.DeployPasstNetworkBindingAnnotation] = "true"
			cl = commontestutils.InitClient([]client.Object{hco})

			handler := passt.NewPasstServiceAccountHandler(cl, commontestutils.GetScheme())

			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Name).To(Equal("passt-binding-cni"))
			Expect(res.Created).To(BeTrue())
			Expect(res.Updated).To(BeFalse())
			Expect(res.Deleted).To(BeFalse())

			foundSA := &corev1.ServiceAccount{}
			Expect(cl.Get(context.Background(), client.ObjectKey{Name: res.Name, Namespace: hco.Namespace}, foundSA)).To(Succeed())

			Expect(foundSA.Name).To(Equal("passt-binding-cni"))
			Expect(foundSA.Namespace).To(Equal(hco.Namespace))
		})
	})
})
