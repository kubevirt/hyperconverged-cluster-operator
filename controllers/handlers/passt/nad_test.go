package passt_test

import (
	"context"

	"github.com/go-logr/logr"
	netattdefv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	openshiftconfigv1 "github.com/openshift/api/config/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/handlers/passt"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

var _ = Describe("Passt NetworkAttachmentDefinition tests", func() {
	var (
		hco    *hcov1beta1.HyperConverged
		req    *common.HcoRequest
		logger = zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)).WithName("test")
	)

	BeforeEach(func() {
		hco = commontestutils.NewHco()
		hco.Annotations = make(map[string]string)
		req = commontestutils.NewReq(hco)
	})

	Context("test NewPasstBindingCNINetworkAttachmentDefinition", func() {
		It("should have all default fields", func() {
			nad := passt.NewPasstBindingCNINetworkAttachmentDefinition(hco, hco.Namespace)

			Expect(nad.Name).To(Equal("primary-udn-kubevirt-binding"))
			Expect(nad.Namespace).To(Equal("default"))

			Expect(nad.Labels).To(HaveKeyWithValue(hcoutil.AppLabel, hcoutil.HyperConvergedName))
			Expect(nad.Labels).To(HaveKeyWithValue(hcoutil.AppLabelComponent, string(hcoutil.AppComponentNetwork)))

			Expect(nad.Spec.Config).To(ContainSubstring(`"cniVersion": "1.0.0"`))
			Expect(nad.Spec.Config).To(ContainSubstring(`"name": "primary-udn-kubevirt-binding"`))
			Expect(nad.Spec.Config).To(ContainSubstring(`"type": "kubevirt-passt-binding"`))
		})

		It("should use default namespace on OpenShift when not in openshift-cnv namespace", func() {
			setupOpenShiftEnvironment(hco, logger)

			nad := passt.NewPasstBindingCNINetworkAttachmentDefinition(hco, hco.Namespace)
			Expect(nad.Namespace).To(Equal("default"))
		})

		It("should use openshift-cnv namespace on OpenShift when HCO is in openshift-cnv", func() {
			hco.Namespace = "openshift-cnv"
			setupOpenShiftEnvironment(hco, logger)

			nad := passt.NewPasstBindingCNINetworkAttachmentDefinition(hco, hco.Namespace)
			Expect(nad.Namespace).To(Equal("openshift-cnv"))
		})
	})

	Context("NetworkAttachmentDefinition deployment", func() {
		It("should not create NetworkAttachmentDefinition if the annotation is not set", func() {
			cl := setupKubernetesEnvironment(hco, logger)
			handler, err := passt.NewPasstNetworkAttachmentDefinitionHandler(logger, cl, commontestutils.GetScheme(), hco)
			Expect(err).ToNot(HaveOccurred())

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
			nad := passt.NewPasstBindingCNINetworkAttachmentDefinition(hco, hco.Namespace)
			cl := commontestutils.InitClient([]client.Object{hco, nad})
			Expect(hcoutil.GetClusterInfo().Init(context.TODO(), cl, logger)).To(Succeed())

			handler, err := passt.NewPasstNetworkAttachmentDefinitionHandler(logger, cl, commontestutils.GetScheme(), hco)
			Expect(err).ToNot(HaveOccurred())

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
			cl := setupKubernetesEnvironment(hco, logger)

			handler, err := passt.NewPasstNetworkAttachmentDefinitionHandler(logger, cl, commontestutils.GetScheme(), hco)
			Expect(err).ToNot(HaveOccurred())

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
})

// setupOpenShiftEnvironment creates a minimal OpenShift environment with all required objects
func setupOpenShiftEnvironment(hco *hcov1beta1.HyperConverged, logger logr.Logger) {
	clusterVersion := &openshiftconfigv1.ClusterVersion{
		ObjectMeta: metav1.ObjectMeta{Name: "version"},
		Spec:       openshiftconfigv1.ClusterVersionSpec{ClusterID: "clusterId"},
	}
	infrastructure := &openshiftconfigv1.Infrastructure{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
		Status: openshiftconfigv1.InfrastructureStatus{
			PlatformStatus: &openshiftconfigv1.PlatformStatus{Type: "mocked"},
		},
	}
	network := &openshiftconfigv1.Network{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
		Status: openshiftconfigv1.NetworkStatus{
			ClusterNetwork: []openshiftconfigv1.ClusterNetworkEntry{{CIDR: "10.128.0.0/14"}},
		},
	}
	dns := &openshiftconfigv1.DNS{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
		Spec:       openshiftconfigv1.DNSSpec{BaseDomain: "test.domain"},
	}
	apiServer := &openshiftconfigv1.APIServer{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
	}

	cl := commontestutils.InitClient([]client.Object{hco, clusterVersion, infrastructure, network, dns, apiServer})
	Expect(hcoutil.GetClusterInfo().Init(context.TODO(), cl, logger)).To(Succeed())
}

// setupKubernetesEnvironment creates a basic Kubernetes environment
func setupKubernetesEnvironment(hco *hcov1beta1.HyperConverged, logger logr.Logger) client.Client {
	cl := commontestutils.InitClient([]client.Object{hco})
	Expect(hcoutil.GetClusterInfo().Init(context.TODO(), cl, logger)).To(Succeed())
	return cl
}
