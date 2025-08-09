package wasp_agent

import (
	"context"
	"fmt"
	"maps"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	securityv1 "github.com/openshift/api/security/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

var _ = Describe("Wasp Agent SecurityContextConstraints", func() {
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

	Context("newWaspAgentSCC", func() {
		It("should have all default fields", func() {
			scc := newWaspAgentSCC(hco)
			Expect(scc.Name).To(Equal("wasp"))
			Expect(scc.Labels).To(HaveKeyWithValue(hcoutil.AppLabel, hcoutil.HyperConvergedName))
			Expect(scc.Labels).To(HaveKeyWithValue(hcoutil.AppLabelComponent, "wasp-agent"))

			Expect(scc.AllowPrivilegedContainer).To(BeTrue())
			Expect(scc.AllowHostDirVolumePlugin).To(BeTrue())
			Expect(scc.AllowHostIPC).To(BeTrue())
			Expect(scc.AllowHostNetwork).To(BeTrue())
			Expect(scc.AllowHostPID).To(BeTrue())
			Expect(scc.AllowHostPorts).To(BeTrue())
			Expect(scc.ReadOnlyRootFilesystem).To(BeFalse())
			Expect(scc.DefaultAddCapabilities).To(BeNil())

			Expect(scc.RunAsUser.Type).To(Equal(securityv1.RunAsUserStrategyRunAsAny))
			Expect(scc.SupplementalGroups.Type).To(Equal(securityv1.SupplementalGroupsStrategyRunAsAny))
			Expect(scc.SELinuxContext.Type).To(Equal(securityv1.SELinuxStrategyRunAsAny))

			expectedUser := "system:serviceaccount:" + hco.Namespace + ":wasp"
			Expect(scc.Users).To(ContainElement(expectedUser))

			Expect(scc.Volumes).To(ContainElement(securityv1.FSTypeAll))
		})
	})

	Context("SecurityContextConstraints deployment", func() {
		It("should not create if overcommit percent is less or equal to 100", func() {
			hco.Spec.HigherWorkloadDensity = &hcov1beta1.HigherWorkloadDensityConfiguration{
				MemoryOvercommitPercentage: 100,
			}
			cl = commontestutils.InitClient([]client.Object{hco})

			handler := NewWaspAgentSCCHandler(cl, commontestutils.GetScheme())

			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Created).To(BeFalse())
			Expect(res.Updated).To(BeFalse())
			Expect(res.Deleted).To(BeFalse())

			foundSCCs := &securityv1.SecurityContextConstraintsList{}
			Expect(cl.List(context.Background(), foundSCCs)).To(Succeed())
			Expect(foundSCCs.Items).To(BeEmpty())
		})

		It("should delete SecurityContextConstraints when percentage is set to 100 and below", func() {
			hco.Spec.HigherWorkloadDensity = &hcov1beta1.HigherWorkloadDensityConfiguration{
				MemoryOvercommitPercentage: 100,
			}
			scc := newWaspAgentSCC(hco)
			cl = commontestutils.InitClient([]client.Object{hco, scc})

			handler := NewWaspAgentSCCHandler(cl, commontestutils.GetScheme())

			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Name).To(Equal(scc.Name))
			Expect(res.Created).To(BeFalse())
			Expect(res.Updated).To(BeFalse())
			Expect(res.Deleted).To(BeTrue())

			foundSCCs := &securityv1.SecurityContextConstraintsList{}
			Expect(cl.List(context.Background(), foundSCCs)).To(Succeed())
			Expect(foundSCCs.Items).To(BeEmpty())
		})

		It("should create SecurityContextConstraints when percentage is set to higher than 100", func() {
			hco.Spec.HigherWorkloadDensity = &hcov1beta1.HigherWorkloadDensityConfiguration{
				MemoryOvercommitPercentage: 150,
			}
			cl = commontestutils.InitClient([]client.Object{hco})

			handler := NewWaspAgentSCCHandler(cl, commontestutils.GetScheme())

			res := handler.Ensure(req)
			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Name).To(Equal("wasp"))
			Expect(res.Created).To(BeTrue())
			Expect(res.Updated).To(BeFalse())
			Expect(res.Deleted).To(BeFalse())

			foundSCCs := &securityv1.SecurityContextConstraintsList{}
			Expect(cl.List(context.Background(), foundSCCs)).To(Succeed())
			Expect(foundSCCs.Items).To(HaveLen(1))
			Expect(foundSCCs.Items[0].Name).To(Equal("wasp"))
		})
	})

	Context("SecurityContextConstraints update", func() {
		It("should update SecurityContextConstraints fields if not matched to the requirements", func() {
			hco.Spec.HigherWorkloadDensity = &hcov1beta1.HigherWorkloadDensityConfiguration{
				MemoryOvercommitPercentage: 150,
			}
			scc := newWaspAgentSCC(hco)
			scc.AllowPrivilegedContainer = false
			scc.AllowHostNetwork = false
			scc.Users = []string{"wrong-user"}

			cl = commontestutils.InitClient([]client.Object{hco, scc})

			handler := NewWaspAgentSCCHandler(cl, commontestutils.GetScheme())

			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Created).To(BeFalse())
			Expect(res.Updated).To(BeTrue())
			Expect(res.Deleted).To(BeFalse())

			foundSCC := &securityv1.SecurityContextConstraints{}
			Expect(cl.Get(context.Background(), client.ObjectKey{Name: "wasp"}, foundSCC)).To(Succeed())
			Expect(foundSCC.AllowPrivilegedContainer).To(BeTrue())
			Expect(foundSCC.AllowHostNetwork).To(BeTrue())
			Expect(foundSCC.Users).To(ContainElement(fmt.Sprintf("system:serviceaccount:%s:wasp", hco.Namespace)))
		})

		It("should reconcile labels if they are missing while preserving user labels", func() {
			hco.Spec.HigherWorkloadDensity = &hcov1beta1.HigherWorkloadDensityConfiguration{
				MemoryOvercommitPercentage: 150,
			}
			scc := newWaspAgentSCC(hco)
			expectedLabels := maps.Clone(scc.Labels)
			delete(scc.Labels, "app.kubernetes.io/component")
			scc.Labels["user-added-label"] = "user-value"
			cl = commontestutils.InitClient([]client.Object{hco, scc})
			handler := NewWaspAgentSCCHandler(cl, commontestutils.GetScheme())

			res := handler.Ensure(req)
			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Created).To(BeFalse())
			Expect(res.Updated).To(BeTrue())
			Expect(res.Deleted).To(BeFalse())

			foundSCC := &securityv1.SecurityContextConstraints{}
			Expect(cl.Get(context.Background(), client.ObjectKey{Name: "wasp"}, foundSCC)).To(Succeed())

			for key, value := range expectedLabels {
				Expect(foundSCC.Labels).To(HaveKeyWithValue(key, value))
			}
			Expect(foundSCC.Labels).To(HaveKeyWithValue("user-added-label", "user-value"))
		})
	})
})
