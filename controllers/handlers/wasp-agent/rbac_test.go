package wasp_agent

import (
	"context"
	"maps"

	log "github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

var _ = Describe("Wasp agent Cluster Role", func() {
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

	Context("newWaspAgentClusterRole", func() {
		It("Should have all the default fields", func() {
			cr := newWaspAgentClusterRole(hco)
			Expect(cr.Name).To(Equal("wasp-cluster"))
			Expect(cr.Labels).To(HaveKeyWithValue(hcoutil.AppLabel, hcoutil.HyperConvergedName))
			Expect(cr.Labels).To(HaveKeyWithValue(hcoutil.AppLabelComponent, "wasp-agent"))
			Expect(cr.Rules).To(HaveLen(1))
			Expect(cr.Rules[0].APIGroups).To(HaveLen(1))
			Expect(cr.Rules[0].APIGroups[0]).To(Equal(""))
			Expect(cr.Rules[0].Resources).To(HaveLen(1))
			Expect(cr.Rules[0].Resources[0]).To(Equal("pods"))
			Expect(cr.Rules[0].Verbs).To(HaveLen(2))
			Expect(cr.Rules[0].Verbs).To(ContainElements("list", "watch"))
		})
	})

	Context("Cluster role deployment", func() {
		It("should not create if overcommit percent is less or equal to 100", func() {
			hco.Spec.HigherWorkloadDensity = &hcov1beta1.HigherWorkloadDensityConfiguration{
				MemoryOvercommitPercentage: 100,
			}
			cr := newWaspAgentClusterRole(hco)

			cl = commontestutils.InitClient([]client.Object{hco, cr})

			handler, err := NewWaspAgentClusterRoleHandler(log.New(nil), cl, commontestutils.GetScheme(), hco)
			Expect(err).ToNot(HaveOccurred())

			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Name).To(Equal(cr.Name))
			Expect(res.Created).To(BeFalse())
			Expect(res.Updated).To(BeFalse())
			Expect(res.Deleted).To(BeTrue())

			foundCRs := &rbacv1.ClusterRoleList{}
			Expect(cl.List(context.Background(), foundCRs)).To(Succeed())
			Expect(foundCRs.Items).To(BeEmpty())
		})
		It("should delete cluster role when percentage is set to 100 and below", func() {
			hco.Spec.HigherWorkloadDensity = &hcov1beta1.HigherWorkloadDensityConfiguration{
				MemoryOvercommitPercentage: 100,
			}
			cr := newWaspAgentClusterRole(hco)

			cl = commontestutils.InitClient([]client.Object{hco, cr})

			handler, err := NewWaspAgentClusterRoleHandler(log.New(nil), cl, commontestutils.GetScheme(), hco)
			Expect(err).ToNot(HaveOccurred())

			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Name).To(Equal(cr.Name))
			Expect(res.Created).To(BeFalse())
			Expect(res.Updated).To(BeFalse())
			Expect(res.Deleted).To(BeTrue())

			foundCRs := &rbacv1.ClusterRoleList{}
			Expect(cl.List(context.Background(), foundCRs)).To(Succeed())
			Expect(foundCRs.Items).To(BeEmpty())
		})
		It("should create cluster role when percentage is set to higher than 100", func() {
			hco.Spec.HigherWorkloadDensity = &hcov1beta1.HigherWorkloadDensityConfiguration{
				MemoryOvercommitPercentage: 150,
			}

			handler, err := NewWaspAgentClusterRoleHandler(log.New(nil), cl, commontestutils.GetScheme(), hco)
			Expect(err).ToNot(HaveOccurred())

			res := handler.Ensure(req)
			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Name).To(Equal("wasp-cluster"))
			Expect(res.Created).To(BeTrue())
			Expect(res.Updated).To(BeFalse())
			Expect(res.Deleted).To(BeFalse())

			foundCRs := &rbacv1.ClusterRoleList{}
			Expect(cl.List(context.Background(), foundCRs)).To(Succeed())
			Expect(foundCRs.Items).To(HaveLen(1))
			Expect(foundCRs.Items[0].Name).To(Equal("wasp-cluster"))
		})
	})
	Context("Wasp agent cluster role update", func() {
		It("should reconcile labels if they are missing while preserving user labels", func() {
			hco.Spec.HigherWorkloadDensity = &hcov1beta1.HigherWorkloadDensityConfiguration{
				MemoryOvercommitPercentage: 150,
			}
			cr := newWaspAgentClusterRole(hco)
			expectedLabels := maps.Clone(cr.Labels)
			delete(cr.Labels, "app.kubernetes.io/component")
			cr.Labels["user-added-label"] = "user-value"
			cl = commontestutils.InitClient([]client.Object{hco, cr})

			handler, err := NewWaspAgentClusterRoleHandler(log.New(nil), cl, commontestutils.GetScheme(), hco)
			Expect(err).ToNot(HaveOccurred())

			res := handler.Ensure(req)
			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Created).To(BeFalse())
			Expect(res.Updated).To(BeTrue())
			Expect(res.Deleted).To(BeFalse())

			foundCr := &rbacv1.ClusterRole{}
			Expect(cl.Get(context.Background(), client.ObjectKey{Name: "wasp-cluster"}, foundCr)).To(Succeed())

			for key, value := range expectedLabels {
				Expect(foundCr.Labels).To(HaveKeyWithValue(key, value))
			}
			Expect(foundCr.Labels).To(HaveKeyWithValue("user-added-label", "user-value"))
		})
	})
})

var _ = Describe("Wasp agent Cluster Role Binding", func() {
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

	Context("newWaspAgentClusterRoleBinding", func() {
		It("Should have all the default fields", func() {
			crb := newWaspAgentClusterRoleBinding(hco)
			Expect(crb.Name).To(Equal("wasp-cluster"))
			Expect(crb.Labels).To(HaveKeyWithValue(hcoutil.AppLabel, hcoutil.HyperConvergedName))
			Expect(crb.Labels).To(HaveKeyWithValue(hcoutil.AppLabelComponent, "wasp-agent"))
			Expect(crb.RoleRef.Name).To(Equal("wasp-cluster"))
			Expect(crb.Subjects).To(HaveLen(1))
			Expect(crb.Subjects[0].Name).To(Equal("wasp"))
			Expect(crb.Subjects[0].Namespace).To(Equal(hco.Namespace))
		})
	})
	Context("Cluster role binding deployment", func() {
		It("should not create if overcommit percent is less or equal to 100", func() {
			hco.Spec.HigherWorkloadDensity = &hcov1beta1.HigherWorkloadDensityConfiguration{
				MemoryOvercommitPercentage: 100,
			}
			crb := newWaspAgentClusterRoleBinding(hco)

			cl = commontestutils.InitClient([]client.Object{hco, crb})

			handler, err := NewWaspAgentClusterRoleBindingHandler(log.New(nil), cl, commontestutils.GetScheme(), hco)
			Expect(err).ToNot(HaveOccurred())

			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Name).To(Equal(crb.Name))
			Expect(res.Created).To(BeFalse())
			Expect(res.Updated).To(BeFalse())
			Expect(res.Deleted).To(BeTrue())

			foundCRBs := &rbacv1.ClusterRoleBindingList{}
			Expect(cl.List(context.Background(), foundCRBs)).To(Succeed())
			Expect(foundCRBs.Items).To(BeEmpty())
		})
		It("should delete cluster role binding when percentage is set to 100 and below", func() {
			hco.Spec.HigherWorkloadDensity = &hcov1beta1.HigherWorkloadDensityConfiguration{
				MemoryOvercommitPercentage: 100,
			}
			crb := newWaspAgentClusterRoleBinding(hco)

			cl = commontestutils.InitClient([]client.Object{hco, crb})

			handler, err := NewWaspAgentClusterRoleBindingHandler(log.New(nil), cl, commontestutils.GetScheme(), hco)
			Expect(err).ToNot(HaveOccurred())

			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Name).To(Equal(crb.Name))
			Expect(res.Created).To(BeFalse())
			Expect(res.Updated).To(BeFalse())
			Expect(res.Deleted).To(BeTrue())

			foundCRBs := &rbacv1.ClusterRoleBindingList{}
			Expect(cl.List(context.Background(), foundCRBs)).To(Succeed())
			Expect(foundCRBs.Items).To(BeEmpty())
		})
		It("should create cluster role binding when percentage is set to higher than 100", func() {
			hco.Spec.HigherWorkloadDensity = &hcov1beta1.HigherWorkloadDensityConfiguration{
				MemoryOvercommitPercentage: 150,
			}

			handler, err := NewWaspAgentClusterRoleBindingHandler(log.New(nil), cl, commontestutils.GetScheme(), hco)
			Expect(err).ToNot(HaveOccurred())

			res := handler.Ensure(req)
			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Name).To(Equal("wasp-cluster"))
			Expect(res.Created).To(BeTrue())
			Expect(res.Updated).To(BeFalse())
			Expect(res.Deleted).To(BeFalse())

			foundCRBs := &rbacv1.ClusterRoleBindingList{}
			Expect(cl.List(context.Background(), foundCRBs)).To(Succeed())
			Expect(foundCRBs.Items).To(HaveLen(1))
			Expect(foundCRBs.Items[0].Name).To(Equal("wasp-cluster"))
		})
	})
	Context("Wasp agent cluster role binding update", func() {
		It("should reconcile labels if they are missing while preserving user labels", func() {
			hco.Spec.HigherWorkloadDensity = &hcov1beta1.HigherWorkloadDensityConfiguration{
				MemoryOvercommitPercentage: 150,
			}
			crb := newWaspAgentClusterRoleBinding(hco)
			expectedLabels := maps.Clone(crb.Labels)
			delete(crb.Labels, "app.kubernetes.io/component")
			crb.Labels["user-added-label"] = "user-value"
			cl = commontestutils.InitClient([]client.Object{hco, crb})

			handler, err := NewWaspAgentClusterRoleBindingHandler(log.New(nil), cl, commontestutils.GetScheme(), hco)
			Expect(err).ToNot(HaveOccurred())

			res := handler.Ensure(req)
			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Created).To(BeFalse())
			Expect(res.Updated).To(BeTrue())
			Expect(res.Deleted).To(BeFalse())

			foundCrb := &rbacv1.ClusterRoleBinding{}
			Expect(cl.Get(context.Background(), client.ObjectKey{Name: "wasp-cluster"}, foundCrb)).To(Succeed())

			for key, value := range expectedLabels {
				Expect(foundCrb.Labels).To(HaveKeyWithValue(key, value))
			}
			Expect(foundCrb.Labels).To(HaveKeyWithValue("user-added-label", "user-value"))
		})
	})
})
