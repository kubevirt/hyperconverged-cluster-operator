package netresinjector

import (
	"context"
	"maps"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

var _ = Describe("Network Resources Injector ServiceAccount", func() {
	var (
		hco *hcov1.HyperConverged
		req *common.HcoRequest
		cl  client.Client
	)

	BeforeEach(func() {
		hco = commontestutils.NewHco()
		req = commontestutils.NewReq(hco)
	})

	Context("newServiceAccount", func() {
		It("should have all default values", func() {
			sa := newServiceAccount()
			Expect(sa.Name).To(Equal(serviceAccountName))
			Expect(sa.Namespace).To(Equal(hco.Namespace))
			Expect(sa.Labels).To(HaveKeyWithValue(hcoutil.AppLabel, hcoutil.HyperConvergedName))
			Expect(sa.Labels).To(HaveKeyWithValue(hcoutil.AppLabelComponent, string(hcoutil.AppComponentNetResInjector)))
		})
	})

	Context("ServiceAccount handler", func() {
		It("should create ServiceAccount if it does not exist", func() {
			cl = commontestutils.InitClient([]client.Object{hco})

			handler := NewServiceAccountHandler(cl, commontestutils.GetScheme())
			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Created).To(BeTrue())

			foundSAs := &corev1.ServiceAccountList{}
			Expect(cl.List(context.Background(), foundSAs)).To(Succeed())
			Expect(foundSAs.Items).To(HaveLen(1))
			Expect(foundSAs.Items[0].Name).To(Equal(serviceAccountName))
		})
	})

	Context("ServiceAccount update", func() {
		It("should reconcile labels if they are missing while preserving user labels", func() {
			sa := newServiceAccount()
			expectedLabels := maps.Clone(sa.Labels)
			delete(sa.Labels, hcoutil.AppLabelComponent)
			sa.Labels["user-added-label"] = "user-value"
			cl = commontestutils.InitClient([]client.Object{hco, sa})

			handler := NewServiceAccountHandler(cl, commontestutils.GetScheme())
			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Updated).To(BeTrue())

			foundSA := &corev1.ServiceAccount{}
			Expect(cl.Get(context.Background(), client.ObjectKey{Name: serviceAccountName, Namespace: hco.Namespace}, foundSA)).To(Succeed())

			for key, value := range expectedLabels {
				Expect(foundSA.Labels).To(HaveKeyWithValue(key, value))
			}
			Expect(foundSA.Labels).To(HaveKeyWithValue("user-added-label", "user-value"))
		})
	})
})
