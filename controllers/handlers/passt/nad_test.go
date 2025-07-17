package passt_test

import (
	"context"
	"maps"

	netattdefv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/handlers/passt"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

var _ = Describe("Passt NetworkAttachmentDefinition tests", func() {
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

	Context("test NewPasstBindingCNINetworkAttachmentDefinition", func() {
		It("should have all default fields", func() {
			nad := passt.NewPasstBindingCNINetworkAttachmentDefinition(hco)

			Expect(nad.Name).To(Equal("primary-udn-kubevirt-binding"))
			Expect(nad.Namespace).To(Equal("default"))

			Expect(nad.Labels).To(HaveKeyWithValue(hcoutil.AppLabel, hcoutil.HyperConvergedName))
			Expect(nad.Labels).To(HaveKeyWithValue(hcoutil.AppLabelComponent, string(hcoutil.AppComponentNetwork)))

			Expect(nad.Spec.Config).To(ContainSubstring(`"cniVersion": "1.0.0"`))
			Expect(nad.Spec.Config).To(ContainSubstring(`"name": "primary-udn-kubevirt-binding"`))
			Expect(nad.Spec.Config).To(ContainSubstring(`"type": "kubevirt-passt-binding"`))
		})
	})

	Context("NetworkAttachmentDefinition deployment", func() {
		It("should not create NetworkAttachmentDefinition if the annotation is not set", func() {
			cl = commontestutils.InitClient([]client.Object{hco})

			handler := passt.NewPasstNetworkAttachmentDefinitionHandler(cl, commontestutils.GetScheme())

			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Created).To(BeFalse())
			Expect(res.Updated).To(BeFalse())
			Expect(res.Deleted).To(BeFalse())

			foundNADs := &netattdefv1.NetworkAttachmentDefinitionList{}
			Expect(cl.List(context.Background(), foundNADs)).To(Succeed())
			Expect(foundNADs.Items).To(BeEmpty())
		})

		It("should delete NetworkAttachmentDefinition if the deployPasstNetworkBinding annotation is false", func() {
			hco.Annotations[passt.DeployPasstNetworkBindingAnnotation] = "false"
			nad := passt.NewPasstBindingCNINetworkAttachmentDefinition(hco)
			cl = commontestutils.InitClient([]client.Object{hco, nad})

			handler := passt.NewPasstNetworkAttachmentDefinitionHandler(cl, commontestutils.GetScheme())

			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Name).To(Equal(nad.Name))
			Expect(res.Created).To(BeFalse())
			Expect(res.Updated).To(BeFalse())
			Expect(res.Deleted).To(BeTrue())

			foundNADs := &netattdefv1.NetworkAttachmentDefinitionList{}
			Expect(cl.List(context.Background(), foundNADs)).To(Succeed())
			Expect(foundNADs.Items).To(BeEmpty())
		})

		It("should create NetworkAttachmentDefinition if the deployPasstNetworkBinding annotation is true", func() {
			hco.Annotations[passt.DeployPasstNetworkBindingAnnotation] = "true"
			cl = commontestutils.InitClient([]client.Object{hco})

			handler := passt.NewPasstNetworkAttachmentDefinitionHandler(cl, commontestutils.GetScheme())

			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Name).To(Equal("primary-udn-kubevirt-binding"))
			Expect(res.Created).To(BeTrue())
			Expect(res.Updated).To(BeFalse())
			Expect(res.Deleted).To(BeFalse())

			foundNAD := &netattdefv1.NetworkAttachmentDefinition{}
			Expect(cl.Get(context.Background(), client.ObjectKey{Name: res.Name, Namespace: "default"}, foundNAD)).To(Succeed())

			Expect(foundNAD.Name).To(Equal("primary-udn-kubevirt-binding"))
			Expect(foundNAD.Namespace).To(Equal("default"))

			Expect(foundNAD.Spec.Config).To(ContainSubstring(`"type": "kubevirt-passt-binding"`))
		})
	})

	Context("NetworkAttachmentDefinition update", func() {
		It("should update NetworkAttachmentDefinition fields if not matched to the requirements", func() {
			hco.Annotations[passt.DeployPasstNetworkBindingAnnotation] = "true"

			nad := passt.NewPasstBindingCNINetworkAttachmentDefinition(hco)
			nad.Spec.Config = `{"cniVersion": "0.3.1", "name": "wrong-name", "type": "wrong-type"}`

			cl = commontestutils.InitClient([]client.Object{hco, nad})

			handler := passt.NewPasstNetworkAttachmentDefinitionHandler(cl, commontestutils.GetScheme())

			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Created).To(BeFalse())
			Expect(res.Updated).To(BeTrue())
			Expect(res.Deleted).To(BeFalse())

			foundNAD := &netattdefv1.NetworkAttachmentDefinition{}
			Expect(cl.Get(context.Background(), client.ObjectKey{Name: res.Name, Namespace: "default"}, foundNAD)).To(Succeed())

			Expect(foundNAD.Spec.Config).To(ContainSubstring(`"cniVersion": "1.0.0"`))
			Expect(foundNAD.Spec.Config).To(ContainSubstring(`"name": "primary-udn-kubevirt-binding"`))
			Expect(foundNAD.Spec.Config).To(ContainSubstring(`"type": "kubevirt-passt-binding"`))
		})

		It("should reconcile managed labels to default without touching user added ones", func() {
			const userLabelKey = "userLabelKey"
			const userLabelValue = "userLabelValue"
			hco.Annotations[passt.DeployPasstNetworkBindingAnnotation] = "true"

			outdatedResource := passt.NewPasstBindingCNINetworkAttachmentDefinition(hco)
			expectedLabels := maps.Clone(outdatedResource.Labels)

			for k, v := range expectedLabels {
				outdatedResource.Labels[k] = "wrong_" + v
			}
			outdatedResource.Labels[userLabelKey] = userLabelValue

			cl = commontestutils.InitClient([]client.Object{hco, outdatedResource})
			handler := passt.NewPasstNetworkAttachmentDefinitionHandler(cl, commontestutils.GetScheme())

			res := handler.Ensure(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Updated).To(BeTrue())
			Expect(res.Err).ToNot(HaveOccurred())

			foundResource := &netattdefv1.NetworkAttachmentDefinition{}
			Expect(
				cl.Get(context.TODO(),
					client.ObjectKey{Name: outdatedResource.Name, Namespace: outdatedResource.Namespace},
					foundResource),
			).ToNot(HaveOccurred())

			for k, v := range expectedLabels {
				Expect(foundResource.Labels).To(HaveKeyWithValue(k, v))
			}
			Expect(foundResource.Labels).To(HaveKeyWithValue(userLabelKey, userLabelValue))
		})

		It("should reconcile managed labels to default on label deletion without touching user added ones", func() {
			const userLabelKey = "userLabelKey"
			const userLabelValue = "userLabelValue"
			hco.Annotations[passt.DeployPasstNetworkBindingAnnotation] = "true"

			outdatedResource := passt.NewPasstBindingCNINetworkAttachmentDefinition(hco)
			expectedLabels := maps.Clone(outdatedResource.Labels)

			outdatedResource.Labels[userLabelKey] = userLabelValue
			delete(outdatedResource.Labels, hcoutil.AppLabelVersion)

			cl = commontestutils.InitClient([]client.Object{hco, outdatedResource})
			handler := passt.NewPasstNetworkAttachmentDefinitionHandler(cl, commontestutils.GetScheme())

			res := handler.Ensure(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Updated).To(BeTrue())
			Expect(res.Err).ToNot(HaveOccurred())

			foundResource := &netattdefv1.NetworkAttachmentDefinition{}
			Expect(
				cl.Get(context.TODO(),
					client.ObjectKey{Name: outdatedResource.Name, Namespace: outdatedResource.Namespace},
					foundResource),
			).ToNot(HaveOccurred())

			for k, v := range expectedLabels {
				Expect(foundResource.Labels).To(HaveKeyWithValue(k, v))
			}
			Expect(foundResource.Labels).To(HaveKeyWithValue(userLabelKey, userLabelValue))
		})
	})
})
