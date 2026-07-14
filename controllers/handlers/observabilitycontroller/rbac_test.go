package observabilitycontroller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

var _ = Describe("Observability Controller ClusterRole", func() {
	Context("newClusterRole", func() {
		It("should have all default values", func() {
			cr := newClusterRole()
			Expect(cr.Name).To(Equal(clusterRoleName))
			Expect(cr.Labels).To(HaveKeyWithValue(hcoutil.AppLabel, hcoutil.HyperConvergedName))
			Expect(cr.Labels).To(HaveKeyWithValue(hcoutil.AppLabelComponent, string(hcoutil.AppComponentObservability)))
			Expect(cr.Rules).To(HaveLen(9))

			Expect(cr.Rules[0].APIGroups).To(ConsistOf("kubevirt.io"))
			Expect(cr.Rules[0].Resources).To(ConsistOf("virtualmachines", "virtualmachineinstances", "virtualmachineinstancemigrations", "kubevirts"))
			Expect(cr.Rules[0].Verbs).To(ConsistOf("get", "list", "watch"))

			Expect(cr.Rules[1].APIGroups).To(ConsistOf("instancetype.kubevirt.io"))
			Expect(cr.Rules[1].Resources).To(ConsistOf("virtualmachineinstancetypes", "virtualmachineclusterinstancetypes", "virtualmachinepreferences", "virtualmachineclusterpreferences"))
			Expect(cr.Rules[1].Verbs).To(ConsistOf("get", "list", "watch"))

			Expect(cr.Rules[2].APIGroups).To(ConsistOf(""))
			Expect(cr.Rules[2].Resources).To(ConsistOf("pods", "persistentvolumeclaims"))
			Expect(cr.Rules[2].Verbs).To(ConsistOf("get", "list", "watch"))

			Expect(cr.Rules[3].APIGroups).To(ConsistOf(""))
			Expect(cr.Rules[3].Resources).To(ConsistOf("configmaps"))
			Expect(cr.Rules[3].Verbs).To(ConsistOf("get"))

			Expect(cr.Rules[4].APIGroups).To(ConsistOf(""))
			Expect(cr.Rules[4].Resources).To(ConsistOf("services", "secrets"))
			Expect(cr.Rules[4].Verbs).To(ConsistOf("get", "list", "watch", "create", "update", "delete"))

			Expect(cr.Rules[5].APIGroups).To(ConsistOf("apps"))
			Expect(cr.Rules[5].Resources).To(ConsistOf("controllerrevisions"))
			Expect(cr.Rules[5].Verbs).To(ConsistOf("get", "list", "watch"))

			Expect(cr.Rules[6].APIGroups).To(ConsistOf("monitoring.coreos.com"))
			Expect(cr.Rules[6].Resources).To(ConsistOf("prometheusrules", "servicemonitors"))
			Expect(cr.Rules[6].Verbs).To(ConsistOf("get", "list", "watch", "create", "update", "delete"))

			Expect(cr.Rules[7].APIGroups).To(ConsistOf("authentication.k8s.io"))
			Expect(cr.Rules[7].Resources).To(ConsistOf("tokenreviews"))
			Expect(cr.Rules[7].Verbs).To(ConsistOf("create"))

			Expect(cr.Rules[8].APIGroups).To(ConsistOf("authorization.k8s.io"))
			Expect(cr.Rules[8].Resources).To(ConsistOf("subjectaccessreviews"))
			Expect(cr.Rules[8].Verbs).To(ConsistOf("create"))
		})
	})

	Context("ClusterRole spec drift", func() {
		It("should restore RBAC rules if they are modified", func() {
			hco := commontestutils.NewHco()
			hco.Spec.FeatureGates.Enable(featureGateName)
			req := commontestutils.NewReq(hco)

			expected := newClusterRole()
			cr := newClusterRole()
			cr.Rules[0].Verbs = []string{"get"}
			cr.Rules = append(cr.Rules, rbacv1.PolicyRule{
				APIGroups: []string{""},
				Resources: []string{"nodes"},
				Verbs:     []string{"get", "list", "delete"},
			})
			cl := commontestutils.InitClient([]client.Object{hco, cr})

			handler := NewClusterRoleHandler(cl, commontestutils.GetScheme())
			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Updated).To(BeTrue())

			foundCR := &rbacv1.ClusterRole{}
			Expect(cl.Get(context.Background(), client.ObjectKey{Name: clusterRoleName}, foundCR)).To(Succeed())
			Expect(foundCR.Rules).To(Equal(expected.Rules))
		})
	})
})

var _ = Describe("Observability Controller ClusterRoleBinding", func() {
	Context("newClusterRoleBinding", func() {
		It("should have all default values", func() {
			crb := newClusterRoleBinding()
			Expect(crb.Name).To(Equal(clusterRoleBindingName))
			Expect(crb.Labels).To(HaveKeyWithValue(hcoutil.AppLabel, hcoutil.HyperConvergedName))
			Expect(crb.Labels).To(HaveKeyWithValue(hcoutil.AppLabelComponent, string(hcoutil.AppComponentObservability)))
			Expect(crb.RoleRef.Name).To(Equal(clusterRoleName))
			Expect(crb.RoleRef.Kind).To(Equal("ClusterRole"))
			Expect(crb.Subjects).To(HaveLen(1))
			Expect(crb.Subjects[0].Name).To(Equal(serviceAccountName))
		})
	})

	Context("ClusterRoleBinding spec drift", func() {
		It("should restore subjects and role ref if they are modified", func() {
			hco := commontestutils.NewHco()
			hco.Spec.FeatureGates.Enable(featureGateName)
			req := commontestutils.NewReq(hco)

			expected := newClusterRoleBinding()
			crb := newClusterRoleBinding()
			crb.Subjects[0].Name = "wrong-sa"
			crb.Subjects = append(crb.Subjects, rbacv1.Subject{
				Kind:      "ServiceAccount",
				Name:      "extra-sa",
				Namespace: "default",
			})
			cl := commontestutils.InitClient([]client.Object{hco, crb})

			handler := NewClusterRoleBindingHandler(cl, commontestutils.GetScheme())
			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Updated).To(BeTrue())

			foundCRB := &rbacv1.ClusterRoleBinding{}
			Expect(cl.Get(context.Background(), client.ObjectKey{Name: clusterRoleBindingName}, foundCRB)).To(Succeed())
			Expect(foundCRB.RoleRef).To(Equal(expected.RoleRef))
			Expect(foundCRB.Subjects).To(Equal(expected.Subjects))
		})
	})
})
