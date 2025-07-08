package passt_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	securityv1 "github.com/openshift/api/security/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/handlers/passt"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

var _ = Describe("Passt SecurityContextConstraints tests", func() {
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

	Context("test NewPasstBindingCNISecurityContextConstraints", func() {
		It("should have all default fields", func() {
			scc := passt.NewPasstBindingCNISecurityContextConstraints(hco)

			Expect(scc.Name).To(Equal("passt-binding-cni"))
			Expect(scc.Labels).To(HaveKeyWithValue(hcoutil.AppLabel, hcoutil.HyperConvergedName))
			Expect(scc.Labels).To(HaveKeyWithValue(hcoutil.AppLabelComponent, string(hcoutil.AppComponentNetwork)))

			Expect(scc.AllowPrivilegedContainer).To(BeTrue())
			Expect(scc.AllowHostDirVolumePlugin).To(BeTrue())
			Expect(scc.AllowHostIPC).To(BeFalse())
			Expect(scc.AllowHostNetwork).To(BeFalse())
			Expect(scc.AllowHostPID).To(BeFalse())
			Expect(scc.AllowHostPorts).To(BeFalse())
			Expect(scc.ReadOnlyRootFilesystem).To(BeFalse())

			Expect(scc.RunAsUser.Type).To(Equal(securityv1.RunAsUserStrategyRunAsAny))
			Expect(scc.SELinuxContext.Type).To(Equal(securityv1.SELinuxStrategyRunAsAny))

			expectedUser := "system:serviceaccount:" + hco.Namespace + ":passt-binding-cni"
			Expect(scc.Users).To(ContainElement(expectedUser))

			Expect(scc.Volumes).To(ContainElement(securityv1.FSTypeAll))
		})
	})

	Context("SecurityContextConstraints deployment", func() {
		It("should not create SecurityContextConstraints if the annotation is not set", func() {
			cl = commontestutils.InitClient([]client.Object{hco})

			handler := passt.NewPasstSecurityContextConstraintsHandler(cl, commontestutils.GetScheme())

			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Created).To(BeFalse())
			Expect(res.Updated).To(BeFalse())
			Expect(res.Deleted).To(BeFalse())

			foundSCCs := &securityv1.SecurityContextConstraintsList{}
			Expect(cl.List(context.Background(), foundSCCs)).To(Succeed())
			Expect(foundSCCs.Items).To(BeEmpty())
		})

		It("should delete SecurityContextConstraints if the deployPasstNetworkBinding annotation is false", func() {
			hco.Annotations[passt.DeployPasstNetworkBindingAnnotation] = "false"
			scc := passt.NewPasstBindingCNISecurityContextConstraints(hco)
			cl = commontestutils.InitClient([]client.Object{hco, scc})

			handler := passt.NewPasstSecurityContextConstraintsHandler(cl, commontestutils.GetScheme())

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

		It("should create SecurityContextConstraints if the deployPasstNetworkBinding annotation is true", func() {
			hco.Annotations[passt.DeployPasstNetworkBindingAnnotation] = "true"
			cl = commontestutils.InitClient([]client.Object{hco})

			handler := passt.NewPasstSecurityContextConstraintsHandler(cl, commontestutils.GetScheme())

			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Name).To(Equal("passt-binding-cni"))
			Expect(res.Created).To(BeTrue())
			Expect(res.Updated).To(BeFalse())
			Expect(res.Deleted).To(BeFalse())

			foundSCC := &securityv1.SecurityContextConstraints{}
			Expect(cl.Get(context.Background(), client.ObjectKey{Name: res.Name}, foundSCC)).To(Succeed())

			Expect(foundSCC.Name).To(Equal("passt-binding-cni"))
			Expect(foundSCC.AllowPrivilegedContainer).To(BeTrue())
		})
	})
})
