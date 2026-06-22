package netresinjector

import (
	"context"
	"maps"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

var _ = Describe("Network Resources Injector ClusterRole", func() {
	var (
		hco *hcov1.HyperConverged
		req *common.HcoRequest
		cl  client.Client
	)

	BeforeEach(func() {
		hco = commontestutils.NewHco()
		req = commontestutils.NewReq(hco)
	})

	Context("newClusterRole", func() {
		It("should have all default values", func() {
			cr := newClusterRole()
			Expect(cr.Name).To(Equal(clusterRoleName))
			Expect(cr.Labels).To(HaveKeyWithValue(hcoutil.AppLabel, hcoutil.HyperConvergedName))
			Expect(cr.Labels).To(HaveKeyWithValue(hcoutil.AppLabelComponent, string(hcoutil.AppComponentNetResInjector)))
			Expect(cr.Rules).To(HaveLen(2))
			Expect(cr.Rules[0].APIGroups).To(ContainElement("k8s.cni.cncf.io"))
			Expect(cr.Rules[0].Resources).To(ContainElement("network-attachment-definitions"))
			Expect(cr.Rules[0].Verbs).To(ContainElements("watch", "list", "get"))
			Expect(cr.Rules[1].APIGroups).To(ContainElement(""))
			Expect(cr.Rules[1].Resources).To(ContainElement("configmaps"))
			Expect(cr.Rules[1].Verbs).To(ContainElements("watch", "list", "get"))
		})
	})

	Context("ClusterRole handler", func() {
		It("should create ClusterRole if it does not exist", func() {
			cl = commontestutils.InitClient([]client.Object{hco})

			handler := NewClusterRoleHandler(cl, commontestutils.GetScheme())
			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Created).To(BeTrue())

			foundCRs := &rbacv1.ClusterRoleList{}
			Expect(cl.List(context.Background(), foundCRs)).To(Succeed())
			Expect(foundCRs.Items).To(HaveLen(1))
			Expect(foundCRs.Items[0].Name).To(Equal(clusterRoleName))
		})
	})

	Context("ClusterRole update", func() {
		It("should reconcile labels if they are missing while preserving user labels", func() {
			cr := newClusterRole()
			expectedLabels := maps.Clone(cr.Labels)
			delete(cr.Labels, hcoutil.AppLabelComponent)
			cr.Labels["user-added-label"] = "user-value"
			cl = commontestutils.InitClient([]client.Object{hco, cr})

			handler := NewClusterRoleHandler(cl, commontestutils.GetScheme())
			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Updated).To(BeTrue())

			foundCR := &rbacv1.ClusterRole{}
			Expect(cl.Get(context.Background(), client.ObjectKey{Name: clusterRoleName}, foundCR)).To(Succeed())

			for key, value := range expectedLabels {
				Expect(foundCR.Labels).To(HaveKeyWithValue(key, value))
			}
			Expect(foundCR.Labels).To(HaveKeyWithValue("user-added-label", "user-value"))
		})
	})
})

var _ = Describe("Network Resources Injector ClusterRoleBinding", func() {
	var (
		hco *hcov1.HyperConverged
		req *common.HcoRequest
		cl  client.Client
	)

	BeforeEach(func() {
		hco = commontestutils.NewHco()
		req = commontestutils.NewReq(hco)
	})

	Context("newClusterRoleBinding", func() {
		It("should have all default values", func() {
			crb := newClusterRoleBinding()
			Expect(crb.Name).To(Equal(clusterRoleName + "-role-binding"))
			Expect(crb.Labels).To(HaveKeyWithValue(hcoutil.AppLabel, hcoutil.HyperConvergedName))
			Expect(crb.Labels).To(HaveKeyWithValue(hcoutil.AppLabelComponent, string(hcoutil.AppComponentNetResInjector)))
			Expect(crb.RoleRef.Name).To(Equal(clusterRoleName))
			Expect(crb.RoleRef.Kind).To(Equal("ClusterRole"))
			Expect(crb.Subjects).To(HaveLen(1))
			Expect(crb.Subjects[0].Name).To(Equal(serviceAccountName))
			Expect(crb.Subjects[0].Namespace).To(Equal(hco.Namespace))
		})
	})

	Context("ClusterRoleBinding handler", func() {
		It("should create ClusterRoleBinding if it does not exist", func() {
			cl = commontestutils.InitClient([]client.Object{hco})

			handler := NewClusterRoleBindingHandler(cl, commontestutils.GetScheme())
			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Created).To(BeTrue())

			foundCRBs := &rbacv1.ClusterRoleBindingList{}
			Expect(cl.List(context.Background(), foundCRBs)).To(Succeed())
			Expect(foundCRBs.Items).To(HaveLen(1))
			Expect(foundCRBs.Items[0].Name).To(Equal(clusterRoleName + "-role-binding"))
		})
	})

	Context("ClusterRoleBinding update", func() {
		It("should reconcile labels if they are missing while preserving user labels", func() {
			crb := newClusterRoleBinding()
			expectedLabels := maps.Clone(crb.Labels)
			delete(crb.Labels, hcoutil.AppLabelComponent)
			crb.Labels["user-added-label"] = "user-value"
			cl = commontestutils.InitClient([]client.Object{hco, crb})

			handler := NewClusterRoleBindingHandler(cl, commontestutils.GetScheme())
			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Updated).To(BeTrue())

			foundCRB := &rbacv1.ClusterRoleBinding{}
			Expect(cl.Get(context.Background(), client.ObjectKey{Name: clusterRoleName + "-role-binding"}, foundCRB)).To(Succeed())

			for key, value := range expectedLabels {
				Expect(foundCRB.Labels).To(HaveKeyWithValue(key, value))
			}
			Expect(foundCRB.Labels).To(HaveKeyWithValue("user-added-label", "user-value"))
		})
	})
})
