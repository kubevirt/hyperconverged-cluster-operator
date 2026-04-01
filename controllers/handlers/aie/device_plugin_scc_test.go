package aie

import (
	"context"
	"fmt"
	"maps"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	securityv1 "github.com/openshift/api/security/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

var _ = Describe("IOMMUFD Device Plugin SecurityContextConstraints", func() {
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

	Context("newIOMMUFDDevicePluginSCC", func() {
		It("should have all default fields", func() {
			scc := newIOMMUFDDevicePluginSCC(hco)
			Expect(scc.Name).To(Equal("iommufd-device-plugin"))
			Expect(scc.Labels).To(HaveKeyWithValue(hcoutil.AppLabel, hcoutil.HyperConvergedName))
			Expect(scc.Labels).To(HaveKeyWithValue(hcoutil.AppLabelComponent, string(hcoutil.AppComponentIOMMUFDDevicePlugin)))

			Expect(scc.AllowPrivilegedContainer).To(BeTrue())
			Expect(scc.AllowHostDirVolumePlugin).To(BeTrue())
			Expect(scc.AllowHostIPC).To(BeFalse())
			Expect(scc.AllowHostNetwork).To(BeFalse())
			Expect(scc.AllowHostPID).To(BeFalse())
			Expect(scc.AllowHostPorts).To(BeFalse())
			Expect(scc.ReadOnlyRootFilesystem).To(BeFalse())

			Expect(scc.RunAsUser.Type).To(Equal(securityv1.RunAsUserStrategyRunAsAny))
			Expect(scc.SupplementalGroups.Type).To(Equal(securityv1.SupplementalGroupsStrategyRunAsAny))
			Expect(scc.SELinuxContext.Type).To(Equal(securityv1.SELinuxStrategyRunAsAny))

			expectedUser := fmt.Sprintf("system:serviceaccount:%s:iommufd-device-plugin", hco.Namespace)
			Expect(scc.Users).To(ContainElement(expectedUser))

			Expect(scc.Volumes).To(ContainElement(securityv1.FSTypeHostPath))
		})
	})

	Context("SCC deployment", func() {
		It("should not create if deploy-aie-webhook annotation is absent", func() {
			delete(hco.Annotations, DeployAIEAnnotation)
			cl = commontestutils.InitClient([]client.Object{hco})

			handler, err := NewIOMMUFDDevicePluginSCCHandler(logf.Log, cl, commontestutils.GetScheme(), hco)
			Expect(err).ToNot(HaveOccurred())

			res := handler.Ensure(req)
			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Created).To(BeFalse())
			Expect(res.Updated).To(BeFalse())
			Expect(res.Deleted).To(BeFalse())

			foundSCCs := &securityv1.SecurityContextConstraintsList{}
			Expect(cl.List(context.Background(), foundSCCs)).To(Succeed())
			Expect(foundSCCs.Items).To(BeEmpty())
		})

		It("should delete SCC when deploy-aie-webhook annotation is removed", func() {
			delete(hco.Annotations, DeployAIEAnnotation)
			scc := newIOMMUFDDevicePluginSCC(hco)
			cl = commontestutils.InitClient([]client.Object{hco, scc})

			handler, err := NewIOMMUFDDevicePluginSCCHandler(logf.Log, cl, commontestutils.GetScheme(), hco)
			Expect(err).ToNot(HaveOccurred())

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

		It("should create SCC when deploy-aie-webhook annotation is true", func() {
			hco.Annotations[DeployAIEAnnotation] = "true"
			cl = commontestutils.InitClient([]client.Object{hco})

			handler, err := NewIOMMUFDDevicePluginSCCHandler(logf.Log, cl, commontestutils.GetScheme(), hco)
			Expect(err).ToNot(HaveOccurred())

			res := handler.Ensure(req)
			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Name).To(Equal("iommufd-device-plugin"))
			Expect(res.Created).To(BeTrue())
			Expect(res.Updated).To(BeFalse())
			Expect(res.Deleted).To(BeFalse())

			foundSCCs := &securityv1.SecurityContextConstraintsList{}
			Expect(cl.List(context.Background(), foundSCCs)).To(Succeed())
			Expect(foundSCCs.Items).To(HaveLen(1))
			Expect(foundSCCs.Items[0].Name).To(Equal("iommufd-device-plugin"))
		})
	})

	Context("SCC update", func() {
		It("should update SCC fields if not matched to the requirements", func() {
			hco.Annotations[DeployAIEAnnotation] = "true"
			scc := newIOMMUFDDevicePluginSCC(hco)
			scc.AllowPrivilegedContainer = false
			scc.Users = []string{"wrong-user"}

			cl = commontestutils.InitClient([]client.Object{hco, scc})

			handler, err := NewIOMMUFDDevicePluginSCCHandler(logf.Log, cl, commontestutils.GetScheme(), hco)
			Expect(err).ToNot(HaveOccurred())

			res := handler.Ensure(req)
			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Created).To(BeFalse())
			Expect(res.Updated).To(BeTrue())
			Expect(res.Deleted).To(BeFalse())

			foundSCC := &securityv1.SecurityContextConstraints{}
			Expect(cl.Get(context.Background(), client.ObjectKey{Name: "iommufd-device-plugin"}, foundSCC)).To(Succeed())
			Expect(foundSCC.AllowPrivilegedContainer).To(BeTrue())
			Expect(foundSCC.Users).To(ContainElement(fmt.Sprintf("system:serviceaccount:%s:iommufd-device-plugin", hco.Namespace)))
		})

		It("should reconcile labels if they are missing while preserving user labels", func() {
			hco.Annotations[DeployAIEAnnotation] = "true"
			scc := newIOMMUFDDevicePluginSCC(hco)
			expectedLabels := maps.Clone(scc.Labels)
			delete(scc.Labels, "app.kubernetes.io/component")
			scc.Labels["user-added-label"] = "user-value"
			cl = commontestutils.InitClient([]client.Object{hco, scc})

			handler, err := NewIOMMUFDDevicePluginSCCHandler(logf.Log, cl, commontestutils.GetScheme(), hco)
			Expect(err).ToNot(HaveOccurred())

			res := handler.Ensure(req)
			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Created).To(BeFalse())
			Expect(res.Updated).To(BeTrue())
			Expect(res.Deleted).To(BeFalse())

			foundSCC := &securityv1.SecurityContextConstraints{}
			Expect(cl.Get(context.Background(), client.ObjectKey{Name: "iommufd-device-plugin"}, foundSCC)).To(Succeed())

			for key, value := range expectedLabels {
				Expect(foundSCC.Labels).To(HaveKeyWithValue(key, value))
			}
			Expect(foundSCC.Labels).To(HaveKeyWithValue("user-added-label", "user-value"))
		})
	})
})
