package wasp_agent

import (
	"context"
	"maps"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

var _ = Describe("Wasp Agent Service Account", func() {
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

	Context("newWaspAgentServiceAccount", func() {
		It("should have all default values", func() {
			sa := newWaspAgentServiceAccount(hco)
			Expect(sa.Name).To(Equal("wasp"))
			Expect(sa.Namespace).To(BeEquivalentTo(hco.Namespace))
			Expect(sa.Labels).To(HaveKeyWithValue(hcoutil.AppLabel, hcoutil.HyperConvergedName))
			Expect(sa.Labels).To(HaveKeyWithValue(hcoutil.AppLabelComponent, "wasp-agent"))
		})
	})

	Context("Wasp agent service account deployment", func() {
		It("should not create if overcommit percent is less or equal to 100", func() {
			hco.Spec.HigherWorkloadDensity = &hcov1beta1.HigherWorkloadDensityConfiguration{
				MemoryOvercommitPercentage: 100,
			}
			cl = commontestutils.InitClient([]client.Object{hco})

			handler := NewWaspAgentServiceAccountHandler(cl, commontestutils.GetScheme())

			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Created).To(BeFalse())
			Expect(res.Updated).To(BeFalse())
			Expect(res.Deleted).To(BeFalse())

			foundSAs := &corev1.ServiceAccountList{}
			Expect(cl.List(context.Background(), foundSAs)).To(Succeed())
			Expect(foundSAs.Items).To(BeEmpty())

		})

		It("should delete service account when percentage is set to 100 and below", func() {
			hco.Spec.HigherWorkloadDensity = &hcov1beta1.HigherWorkloadDensityConfiguration{
				MemoryOvercommitPercentage: 100,
			}
			sa := newWaspAgentServiceAccount(hco)

			cl = commontestutils.InitClient([]client.Object{hco, sa})

			handler := NewWaspAgentServiceAccountHandler(cl, commontestutils.GetScheme())

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

		It("should create service account when percentage is set to higher than 100", func() {
			hco.Spec.HigherWorkloadDensity = &hcov1beta1.HigherWorkloadDensityConfiguration{
				MemoryOvercommitPercentage: 150,
			}

			handler := NewWaspAgentServiceAccountHandler(cl, commontestutils.GetScheme())

			res := handler.Ensure(req)
			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Name).To(Equal("wasp"))
			Expect(res.Created).To(BeTrue())
			Expect(res.Updated).To(BeFalse())
			Expect(res.Deleted).To(BeFalse())

			foundSAs := &corev1.ServiceAccountList{}
			Expect(cl.List(context.Background(), foundSAs)).To(Succeed())
			Expect(foundSAs.Items).To(HaveLen(1))
			Expect(foundSAs.Items[0].Name).To(Equal("wasp"))
		})
	})
	Context("Wasp agent service account update", func() {
		It("should reconcile labels if they are missing while preserving user labels", func() {
			hco.Spec.HigherWorkloadDensity = &hcov1beta1.HigherWorkloadDensityConfiguration{
				MemoryOvercommitPercentage: 150,
			}
			sa := newWaspAgentServiceAccount(hco)
			expectedLabels := maps.Clone(sa.Labels)
			delete(sa.Labels, "app.kubernetes.io/component")
			sa.Labels["user-added-label"] = "user-value"
			cl = commontestutils.InitClient([]client.Object{hco, sa})
			handler := NewWaspAgentServiceAccountHandler(cl, commontestutils.GetScheme())

			res := handler.Ensure(req)
			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Created).To(BeFalse())
			Expect(res.Updated).To(BeTrue())
			Expect(res.Deleted).To(BeFalse())

			foundDs := &corev1.ServiceAccount{}
			Expect(cl.Get(context.Background(), client.ObjectKey{Name: "wasp", Namespace: hco.Namespace}, foundDs)).To(Succeed())

			for key, value := range expectedLabels {
				Expect(foundDs.Labels).To(HaveKeyWithValue(key, value))
			}
			Expect(foundDs.Labels).To(HaveKeyWithValue("user-added-label", "user-value"))
		})
	})
})
