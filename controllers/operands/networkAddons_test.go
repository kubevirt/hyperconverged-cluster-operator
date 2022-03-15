package operands

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	conditionsv1 "github.com/openshift/custom-resource-status/conditions/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/reference"

	networkaddonsshared "github.com/kubevirt/cluster-network-addons-operator/pkg/apis/networkaddonsoperator/shared"
	networkaddonsv1 "github.com/kubevirt/cluster-network-addons-operator/pkg/apis/networkaddonsoperator/v1"
	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commonTestUtils"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
	sdkapi "kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk/api"
)

var _ = Describe("CNA Operand", func() {

	Context("NetworkAddonsConfig", func() {
		var hco *hcov1beta1.HyperConverged
		var req *common.HcoRequest

		BeforeEach(func() {
			hco = commonTestUtils.NewHco()
			req = commonTestUtils.NewReq(hco)
		})

		It("should create if not present", func() {
			expectedResource, err := NewNetworkAddons(hco)
			Expect(err).ToNot(HaveOccurred())
			cl := commonTestUtils.InitClient([]runtime.Object{})
			handler := (*genericOperand)(newCnaHandler(cl, commonTestUtils.GetScheme()))
			res := handler.ensure(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			foundResource := &networkaddonsv1.NetworkAddonsConfig{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: expectedResource.Name, Namespace: expectedResource.Namespace},
					foundResource),
			).To(BeNil())
			Expect(foundResource.Name).To(Equal(expectedResource.Name))
			Expect(foundResource.Labels).Should(HaveKeyWithValue(hcoutil.AppLabel, commonTestUtils.Name))
			Expect(foundResource.Namespace).To(Equal(expectedResource.Namespace))
			Expect(foundResource.Spec.Multus).To(Equal(&networkaddonsshared.Multus{}))
			Expect(foundResource.Spec.LinuxBridge).To(Equal(&networkaddonsshared.LinuxBridge{}))
			Expect(foundResource.Spec.KubeMacPool).To(Equal(&networkaddonsshared.KubeMacPool{}))
		})

		It("should find if present", func() {
			expectedResource, err := NewNetworkAddons(hco)
			Expect(err).ToNot(HaveOccurred())
			expectedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", expectedResource.Namespace, expectedResource.Name)
			cl := commonTestUtils.InitClient([]runtime.Object{hco, expectedResource})
			handler := (*genericOperand)(newCnaHandler(cl, commonTestUtils.GetScheme()))
			res := handler.ensure(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			// Check HCO's status
			Expect(hco.Status.RelatedObjects).To(Not(BeNil()))
			objectRef, err := reference.GetReference(handler.Scheme, expectedResource)
			Expect(err).To(BeNil())
			// ObjectReference should have been added
			Expect(hco.Status.RelatedObjects).To(ContainElement(*objectRef))
			// Check conditions
			Expect(req.Conditions[hcov1beta1.ConditionAvailable]).To(commonTestUtils.RepresentCondition(metav1.Condition{
				Type:    hcov1beta1.ConditionAvailable,
				Status:  metav1.ConditionFalse,
				Reason:  "NetworkAddonsConfigConditions",
				Message: "NetworkAddonsConfig resource has no conditions",
			}))
			Expect(req.Conditions[hcov1beta1.ConditionProgressing]).To(commonTestUtils.RepresentCondition(metav1.Condition{
				Type:    hcov1beta1.ConditionProgressing,
				Status:  metav1.ConditionTrue,
				Reason:  "NetworkAddonsConfigConditions",
				Message: "NetworkAddonsConfig resource has no conditions",
			}))
			Expect(req.Conditions[hcov1beta1.ConditionUpgradeable]).To(commonTestUtils.RepresentCondition(metav1.Condition{
				Type:    hcov1beta1.ConditionUpgradeable,
				Status:  metav1.ConditionFalse,
				Reason:  "NetworkAddonsConfigConditions",
				Message: "NetworkAddonsConfig resource has no conditions",
			}))
		})

		It("should find reconcile to default", func() {
			existingResource, err := NewNetworkAddons(hco)
			Expect(err).ToNot(HaveOccurred())
			existingResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", existingResource.Namespace, existingResource.Name)
			existingResource.Spec.ImagePullPolicy = corev1.PullAlways // set non-default value

			cl := commonTestUtils.InitClient([]runtime.Object{hco, existingResource})
			handler := (*genericOperand)(newCnaHandler(cl, commonTestUtils.GetScheme()))
			res := handler.ensure(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Updated).To(BeTrue())
			Expect(res.Err).To(BeNil())

			foundResource := &networkaddonsv1.NetworkAddonsConfig{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
					foundResource),
			).To(BeNil())
			Expect(foundResource.Spec.ImagePullPolicy).To(BeEmpty())

			Expect(req.Conditions).To(BeEmpty())

			// ObjectReference should have been updated
			Expect(hco.Status.RelatedObjects).To(Not(BeNil()))
			objectRefOutdated, err := reference.GetReference(handler.Scheme, existingResource)
			Expect(err).To(BeNil())
			objectRefFound, err := reference.GetReference(handler.Scheme, foundResource)
			Expect(err).To(BeNil())
			Expect(hco.Status.RelatedObjects).To(Not(ContainElement(*objectRefOutdated)))
			Expect(hco.Status.RelatedObjects).To(ContainElement(*objectRefFound))

		})

		It("should add node placement if missing in CNAO", func() {
			existingResource, err := NewNetworkAddons(hco)
			Expect(err).ToNot(HaveOccurred())

			hco.Spec.Infra = hcov1beta1.HyperConvergedConfig{NodePlacement: commonTestUtils.NewNodePlacement()}
			hco.Spec.Workloads = hcov1beta1.HyperConvergedConfig{NodePlacement: commonTestUtils.NewNodePlacement()}

			cl := commonTestUtils.InitClient([]runtime.Object{hco, existingResource})
			handler := (*genericOperand)(newCnaHandler(cl, commonTestUtils.GetScheme()))
			res := handler.ensure(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Updated).To(BeTrue())
			Expect(res.Err).To(BeNil())

			foundResource := &networkaddonsv1.NetworkAddonsConfig{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
					foundResource),
			).To(BeNil())

			Expect(existingResource.Spec.PlacementConfiguration).To(BeNil())
			Expect(foundResource.Spec.PlacementConfiguration).ToNot(BeNil())
			placementConfig := foundResource.Spec.PlacementConfiguration
			Expect(placementConfig.Infra).ToNot(BeNil())
			Expect(placementConfig.Infra.NodeSelector["key1"]).Should(Equal("value1"))
			Expect(placementConfig.Infra.NodeSelector["key2"]).Should(Equal("value2"))

			Expect(placementConfig.Workloads).ToNot(BeNil())
			Expect(placementConfig.Workloads.Tolerations).Should(Equal(hco.Spec.Workloads.NodePlacement.Tolerations))

			Expect(req.Conditions).To(BeEmpty())
		})

		It("should remove node placement if missing in HCO CR", func() {

			hcoNodePlacement := commonTestUtils.NewHco()
			hcoNodePlacement.Spec.Infra = hcov1beta1.HyperConvergedConfig{NodePlacement: commonTestUtils.NewNodePlacement()}
			hcoNodePlacement.Spec.Workloads = hcov1beta1.HyperConvergedConfig{NodePlacement: commonTestUtils.NewNodePlacement()}
			existingResource, err := NewNetworkAddons(hcoNodePlacement)
			Expect(err).ToNot(HaveOccurred())

			cl := commonTestUtils.InitClient([]runtime.Object{hco, existingResource})
			handler := (*genericOperand)(newCnaHandler(cl, commonTestUtils.GetScheme()))
			res := handler.ensure(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Updated).To(BeTrue())
			Expect(res.Err).To(BeNil())

			foundResource := &networkaddonsv1.NetworkAddonsConfig{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
					foundResource),
			).To(BeNil())

			Expect(existingResource.Spec.PlacementConfiguration).ToNot(BeNil())
			Expect(foundResource.Spec.PlacementConfiguration).To(BeNil())

			Expect(req.Conditions).To(BeEmpty())
		})

		It("should modify node placement according to HCO CR", func() {

			hco.Spec.Infra = hcov1beta1.HyperConvergedConfig{NodePlacement: commonTestUtils.NewNodePlacement()}
			hco.Spec.Workloads = hcov1beta1.HyperConvergedConfig{NodePlacement: commonTestUtils.NewNodePlacement()}
			existingResource, err := NewNetworkAddons(hco)
			Expect(err).ToNot(HaveOccurred())

			// now, modify HCO's node placement
			seconds3 := int64(3)
			hco.Spec.Infra.NodePlacement.Tolerations = append(hco.Spec.Infra.NodePlacement.Tolerations, corev1.Toleration{
				Key: "key3", Operator: "operator3", Value: "value3", Effect: "effect3", TolerationSeconds: &seconds3,
			})

			hco.Spec.Workloads.NodePlacement.NodeSelector["key1"] = "something else"

			cl := commonTestUtils.InitClient([]runtime.Object{hco, existingResource})
			handler := (*genericOperand)(newCnaHandler(cl, commonTestUtils.GetScheme()))
			res := handler.ensure(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Updated).To(BeTrue())
			Expect(res.Err).To(BeNil())

			foundResource := &networkaddonsv1.NetworkAddonsConfig{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
					foundResource),
			).To(BeNil())

			Expect(existingResource.Spec.PlacementConfiguration).ToNot(BeNil())
			Expect(existingResource.Spec.PlacementConfiguration.Infra.Tolerations).To(HaveLen(2))
			Expect(existingResource.Spec.PlacementConfiguration.Workloads.NodeSelector["key1"]).Should(Equal("value1"))

			Expect(foundResource.Spec.PlacementConfiguration).ToNot(BeNil())
			Expect(foundResource.Spec.PlacementConfiguration.Infra.Tolerations).To(HaveLen(3))
			Expect(foundResource.Spec.PlacementConfiguration.Workloads.NodeSelector["key1"]).Should(Equal("something else"))

			Expect(req.Conditions).To(BeEmpty())
		})

		It("should overwrite node placement if directly set on CNAO CR", func() {
			hco.Spec.Infra = hcov1beta1.HyperConvergedConfig{NodePlacement: commonTestUtils.NewNodePlacement()}
			hco.Spec.Workloads = hcov1beta1.HyperConvergedConfig{NodePlacement: commonTestUtils.NewNodePlacement()}
			existingResource, err := NewNetworkAddons(hco)
			Expect(err).ToNot(HaveOccurred())

			// mock a reconciliation triggered by a change in CNAO CR
			req.HCOTriggered = false

			// now, modify CNAO node placement
			seconds3 := int64(3)
			existingResource.Spec.PlacementConfiguration.Infra.Tolerations = append(hco.Spec.Infra.NodePlacement.Tolerations, corev1.Toleration{
				Key: "key3", Operator: "operator3", Value: "value3", Effect: "effect3", TolerationSeconds: &seconds3,
			})
			existingResource.Spec.PlacementConfiguration.Workloads.Tolerations = append(hco.Spec.Workloads.NodePlacement.Tolerations, corev1.Toleration{
				Key: "key3", Operator: "operator3", Value: "value3", Effect: "effect3", TolerationSeconds: &seconds3,
			})

			existingResource.Spec.PlacementConfiguration.Infra.NodeSelector["key1"] = "BADvalue1"
			existingResource.Spec.PlacementConfiguration.Workloads.NodeSelector["key2"] = "BADvalue2"

			cl := commonTestUtils.InitClient([]runtime.Object{hco, existingResource})
			handler := (*genericOperand)(newCnaHandler(cl, commonTestUtils.GetScheme()))
			res := handler.ensure(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Updated).To(BeTrue())
			Expect(res.Overwritten).To(BeTrue())
			Expect(res.Err).To(BeNil())

			foundResource := &networkaddonsv1.NetworkAddonsConfig{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
					foundResource),
			).To(BeNil())

			Expect(existingResource.Spec.PlacementConfiguration.Infra.Tolerations).To(HaveLen(3))
			Expect(existingResource.Spec.PlacementConfiguration.Workloads.Tolerations).To(HaveLen(3))
			Expect(existingResource.Spec.PlacementConfiguration.Infra.NodeSelector["key1"]).Should(Equal("BADvalue1"))
			Expect(existingResource.Spec.PlacementConfiguration.Workloads.NodeSelector["key2"]).Should(Equal("BADvalue2"))

			Expect(foundResource.Spec.PlacementConfiguration.Infra.Tolerations).To(HaveLen(2))
			Expect(foundResource.Spec.PlacementConfiguration.Workloads.Tolerations).To(HaveLen(2))
			Expect(foundResource.Spec.PlacementConfiguration.Infra.NodeSelector["key1"]).Should(Equal("value1"))
			Expect(foundResource.Spec.PlacementConfiguration.Workloads.NodeSelector["key2"]).Should(Equal("value2"))

			Expect(req.Conditions).To(BeEmpty())
		})

		It("should add self signed configutation if missing in CNAO", func() {
			existingResource, err := NewNetworkAddons(hco)
			Expect(err).ToNot(HaveOccurred())

			hco.Spec.CertConfig = hcov1beta1.HyperConvergedCertConfig{
				CA: hcov1beta1.CertRotateConfigCA{
					Duration:    metav1.Duration{Duration: 24 * time.Hour},
					RenewBefore: metav1.Duration{Duration: 1 * time.Hour},
				},
				Server: hcov1beta1.CertRotateConfigServer{
					Duration:    metav1.Duration{Duration: 12 * time.Hour},
					RenewBefore: metav1.Duration{Duration: 30 * time.Minute},
				},
			}

			cl := commonTestUtils.InitClient([]runtime.Object{hco, existingResource})
			handler := (*genericOperand)(newCnaHandler(cl, commonTestUtils.GetScheme()))
			res := handler.ensure(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Updated).To(BeTrue())
			Expect(res.Err).To(BeNil())

			foundResource := &networkaddonsv1.NetworkAddonsConfig{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
					foundResource),
			).To(BeNil())

			Expect(foundResource.Spec.SelfSignConfiguration).ToNot(BeNil())
			selfSignedConfig := foundResource.Spec.SelfSignConfiguration
			Expect(selfSignedConfig.CARotateInterval).Should(Equal("24h0m0s"))
			Expect(selfSignedConfig.CAOverlapInterval).Should(Equal("1h0m0s"))
			Expect(selfSignedConfig.CertRotateInterval).Should(Equal("12h0m0s"))
			Expect(selfSignedConfig.CertOverlapInterval).Should(Equal("30m0s"))

			Expect(req.Conditions).To(BeEmpty())
		})

		It("should set self signed configutation to defaults if missing in HCO CR", func() {
			existingResource := NewNetworkAddonsWithNameOnly(hco)

			cl := commonTestUtils.InitClient([]runtime.Object{hco})
			handler := (*genericOperand)(newCnaHandler(cl, commonTestUtils.GetScheme()))
			res := handler.ensure(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Updated).To(BeFalse())
			Expect(res.Err).To(BeNil())

			foundResource := &networkaddonsv1.NetworkAddonsConfig{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
					foundResource),
			).To(BeNil())

			Expect(existingResource.Spec.SelfSignConfiguration).To(BeNil())

			Expect(foundResource.Spec.SelfSignConfiguration.CARotateInterval).ToNot(BeNil())
			selfSignedConfig := foundResource.Spec.SelfSignConfiguration
			Expect(selfSignedConfig.CARotateInterval).Should(Equal("48h0m0s"))
			Expect(selfSignedConfig.CAOverlapInterval).Should(Equal("24h0m0s"))
			Expect(selfSignedConfig.CertRotateInterval).Should(Equal("24h0m0s"))
			Expect(selfSignedConfig.CertOverlapInterval).Should(Equal("12h0m0s"))

			Expect(req.Conditions).To(BeEmpty())
		})

		It("should modify self signed configutation according to HCO CR", func() {

			hco.Spec.CertConfig = hcov1beta1.HyperConvergedCertConfig{
				CA: hcov1beta1.CertRotateConfigCA{
					Duration:    metav1.Duration{Duration: 24 * time.Hour},
					RenewBefore: metav1.Duration{Duration: 1 * time.Hour},
				},
				Server: hcov1beta1.CertRotateConfigServer{
					Duration:    metav1.Duration{Duration: 12 * time.Hour},
					RenewBefore: metav1.Duration{Duration: 30 * time.Minute},
				},
			}
			existingResource, err := NewNetworkAddons(hco)
			Expect(err).ToNot(HaveOccurred())

			By("Modify HCO's cert configuration")
			hco.Spec.CertConfig.CA.Duration.Duration *= 2
			hco.Spec.CertConfig.CA.RenewBefore.Duration *= 2
			hco.Spec.CertConfig.Server.Duration.Duration *= 2
			hco.Spec.CertConfig.Server.RenewBefore.Duration *= 2

			cl := commonTestUtils.InitClient([]runtime.Object{hco, existingResource})
			handler := (*genericOperand)(newCnaHandler(cl, commonTestUtils.GetScheme()))
			res := handler.ensure(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Updated).To(BeTrue())
			Expect(res.Err).To(BeNil())

			foundResource := &networkaddonsv1.NetworkAddonsConfig{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
					foundResource),
			).To(BeNil())

			Expect(existingResource.Spec.SelfSignConfiguration).ToNot(BeNil())
			existingSelfSignedConfig := existingResource.Spec.SelfSignConfiguration
			Expect(existingSelfSignedConfig.CARotateInterval).Should(Equal("24h0m0s"))
			Expect(existingSelfSignedConfig.CAOverlapInterval).Should(Equal("1h0m0s"))
			Expect(existingSelfSignedConfig.CertRotateInterval).Should(Equal("12h0m0s"))
			Expect(existingSelfSignedConfig.CertOverlapInterval).Should(Equal("30m0s"))

			Expect(foundResource.Spec.SelfSignConfiguration).ToNot(BeNil())
			foundSelfSignedConfig := foundResource.Spec.SelfSignConfiguration
			Expect(foundSelfSignedConfig.CARotateInterval).Should(Equal("48h0m0s"))
			Expect(foundSelfSignedConfig.CAOverlapInterval).Should(Equal("2h0m0s"))
			Expect(foundSelfSignedConfig.CertRotateInterval).Should(Equal("24h0m0s"))
			Expect(foundSelfSignedConfig.CertOverlapInterval).Should(Equal("1h0m0s"))

			Expect(req.Conditions).To(BeEmpty())
		})

		It("should overwrite self signed configutation if directly set on CNAO CR", func() {

			hco.Spec.CertConfig = hcov1beta1.HyperConvergedCertConfig{
				CA: hcov1beta1.CertRotateConfigCA{
					Duration:    metav1.Duration{Duration: 24 * time.Hour},
					RenewBefore: metav1.Duration{Duration: 1 * time.Hour},
				},
				Server: hcov1beta1.CertRotateConfigServer{
					Duration:    metav1.Duration{Duration: 12 * time.Hour},
					RenewBefore: metav1.Duration{Duration: 30 * time.Minute},
				},
			}
			existingResource, err := NewNetworkAddons(hco)
			Expect(err).ToNot(HaveOccurred())

			By("Mock a reconciliation triggered by a change in CNAO CR")
			req.HCOTriggered = false

			By("Modify CNAO's cert configuration")
			existingResource.Spec.SelfSignConfiguration.CARotateInterval = "48h0m0s"
			existingResource.Spec.SelfSignConfiguration.CAOverlapInterval = "2h0m0s"
			existingResource.Spec.SelfSignConfiguration.CertRotateInterval = "24h0m0s"
			existingResource.Spec.SelfSignConfiguration.CertOverlapInterval = "1h0m0s"

			cl := commonTestUtils.InitClient([]runtime.Object{hco, existingResource})
			handler := (*genericOperand)(newCnaHandler(cl, commonTestUtils.GetScheme()))
			res := handler.ensure(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Updated).To(BeTrue())
			Expect(res.Overwritten).To(BeTrue())
			Expect(res.Err).To(BeNil())

			foundResource := &networkaddonsv1.NetworkAddonsConfig{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
					foundResource),
			).To(BeNil())

			Expect(existingResource.Spec.SelfSignConfiguration).ToNot(BeNil())
			existingSelfSignedConfig := existingResource.Spec.SelfSignConfiguration
			Expect(existingSelfSignedConfig.CARotateInterval).Should(Equal("48h0m0s"))
			Expect(existingSelfSignedConfig.CAOverlapInterval).Should(Equal("2h0m0s"))
			Expect(existingSelfSignedConfig.CertRotateInterval).Should(Equal("24h0m0s"))
			Expect(existingSelfSignedConfig.CertOverlapInterval).Should(Equal("1h0m0s"))

			Expect(foundResource.Spec.SelfSignConfiguration).ToNot(BeNil())
			foundSelfSignedConfig := foundResource.Spec.SelfSignConfiguration
			Expect(foundSelfSignedConfig.CARotateInterval).Should(Equal("24h0m0s"))
			Expect(foundSelfSignedConfig.CAOverlapInterval).Should(Equal("1h0m0s"))
			Expect(foundSelfSignedConfig.CertRotateInterval).Should(Equal("12h0m0s"))
			Expect(foundSelfSignedConfig.CertOverlapInterval).Should(Equal("30m0s"))

			Expect(req.Conditions).To(BeEmpty())
		})

		type ovsAnnotationParams struct {
			annotationExists  bool
			annotationValue   string
			ovsDeployExpected bool
		}
		table.DescribeTable("when reconciling ovs-cni", func(o ovsAnnotationParams) {
			hcoOVSConfig := commonTestUtils.NewHco()
			hcoOVSConfig.Annotations = map[string]string{}

			if o.annotationExists {
				hcoOVSConfig.Annotations["deployOVS"] = o.annotationValue
			}

			existingResource, err := NewNetworkAddons(hcoOVSConfig)
			Expect(err).ToNot(HaveOccurred())

			cl := commonTestUtils.InitClient([]runtime.Object{hco, existingResource})
			handler := (*genericOperand)(newCnaHandler(cl, commonTestUtils.GetScheme()))
			res := handler.ensure(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			foundResource := &networkaddonsv1.NetworkAddonsConfig{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
					foundResource),
			).To(BeNil())

			if o.ovsDeployExpected {
				Expect(existingResource.Spec.Ovs).ToNot(BeNil(), "Ovs spec should be added")
			} else {
				Expect(existingResource.Spec.Ovs).To(BeNil(), "Ovs spec should not be added")
			}
		},
			table.Entry("should have ovs if deployOVS annotation is set to true", ovsAnnotationParams{
				annotationExists:  true,
				annotationValue:   "true",
				ovsDeployExpected: true,
			}),
			table.Entry("should not have ovs if deployOVS annotation is not set to true", ovsAnnotationParams{
				annotationExists:  true,
				annotationValue:   "false",
				ovsDeployExpected: false,
			}),
			table.Entry("should not have ovs if deployOVS annotation does not exist", ovsAnnotationParams{
				annotationExists:  false,
				annotationValue:   "",
				ovsDeployExpected: false,
			}),
		)

		It("should handle conditions", func() {
			expectedResource, err := NewNetworkAddons(hco)
			Expect(err).ToNot(HaveOccurred())
			expectedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", expectedResource.Namespace, expectedResource.Name)
			expectedResource.Status.Conditions = []conditionsv1.Condition{
				{
					Type:    conditionsv1.ConditionAvailable,
					Status:  corev1.ConditionFalse,
					Reason:  "Foo",
					Message: "Bar",
				},
				{
					Type:    conditionsv1.ConditionProgressing,
					Status:  corev1.ConditionTrue,
					Reason:  "Foo",
					Message: "Bar",
				},
				{
					Type:    conditionsv1.ConditionDegraded,
					Status:  corev1.ConditionTrue,
					Reason:  "Foo",
					Message: "Bar",
				},
			}
			cl := commonTestUtils.InitClient([]runtime.Object{hco, expectedResource})
			handler := (*genericOperand)(newCnaHandler(cl, commonTestUtils.GetScheme()))
			res := handler.ensure(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			// Check HCO's status
			Expect(hco.Status.RelatedObjects).To(Not(BeNil()))
			objectRef, err := reference.GetReference(handler.Scheme, expectedResource)
			Expect(err).To(BeNil())
			// ObjectReference should have been added
			Expect(hco.Status.RelatedObjects).To(ContainElement(*objectRef))
			// Check conditions
			Expect(req.Conditions[hcov1beta1.ConditionAvailable]).To(commonTestUtils.RepresentCondition(metav1.Condition{
				Type:    hcov1beta1.ConditionAvailable,
				Status:  metav1.ConditionFalse,
				Reason:  "NetworkAddonsConfigNotAvailable",
				Message: "NetworkAddonsConfig is not available: Bar",
			}))
			Expect(req.Conditions[hcov1beta1.ConditionProgressing]).To(commonTestUtils.RepresentCondition(metav1.Condition{
				Type:    hcov1beta1.ConditionProgressing,
				Status:  metav1.ConditionTrue,
				Reason:  "NetworkAddonsConfigProgressing",
				Message: "NetworkAddonsConfig is progressing: Bar",
			}))
			Expect(req.Conditions[hcov1beta1.ConditionUpgradeable]).To(commonTestUtils.RepresentCondition(metav1.Condition{
				Type:    hcov1beta1.ConditionUpgradeable,
				Status:  metav1.ConditionFalse,
				Reason:  "NetworkAddonsConfigProgressing",
				Message: "NetworkAddonsConfig is progressing: Bar",
			}))
			Expect(req.Conditions[hcov1beta1.ConditionDegraded]).To(commonTestUtils.RepresentCondition(metav1.Condition{
				Type:    hcov1beta1.ConditionDegraded,
				Status:  metav1.ConditionTrue,
				Reason:  "NetworkAddonsConfigDegraded",
				Message: "NetworkAddonsConfig is degraded: Bar",
			}))
		})

		Context("jsonpath Annotation", func() {
			It("Should create CNA object with changes from the annotation", func() {
				hco.Annotations = map[string]string{common.JSONPatchCNAOAnnotationName: `[
					{
						"op": "add",
						"path": "/spec/kubeMacPool",
						"value": {"rangeStart": "1.1.1.1.1.1", "rangeEnd": "5.5.5.5.5.5" }
					},
					{
						"op": "add",
						"path": "/spec/imagePullPolicy",
						"value": "Always"
					}
				]`}

				cna, err := NewNetworkAddons(hco)
				Expect(err).ToNot(HaveOccurred())
				Expect(cna).ToNot(BeNil())
				Expect(cna.Spec.KubeMacPool.RangeStart).Should(Equal("1.1.1.1.1.1"))
				Expect(cna.Spec.KubeMacPool.RangeEnd).Should(Equal("5.5.5.5.5.5"))
				Expect(cna.Spec.ImagePullPolicy).To(BeEquivalentTo("Always"))
			})

			It("Should fail to create CNA object with wrong jsonPatch", func() {
				hco.Annotations = map[string]string{common.JSONPatchCNAOAnnotationName: `[
					{
						"op": "notExists",
						"path": "/spec/kubeMacPool",
						"value": {"rangeStart": "1.1.1.1.1.1", "rangeEnd": "5.5.5.5.5.5" }
					}
				]`}

				_, err := NewNetworkAddons(hco)
				Expect(err).To(HaveOccurred())
			})

			It("Ensure func should create CNA object with changes from the annotation", func() {
				hco.Annotations = map[string]string{common.JSONPatchCNAOAnnotationName: `[
					{
						"op": "add",
						"path": "/spec/kubeMacPool",
						"value": {"rangeStart": "1.1.1.1.1.1", "rangeEnd": "5.5.5.5.5.5" }
					},
					{
						"op": "add",
						"path": "/spec/imagePullPolicy",
						"value": "Always"
					}
				]`}

				expectedResource := NewNetworkAddonsWithNameOnly(hco)
				cl := commonTestUtils.InitClient([]runtime.Object{})
				handler := (*genericOperand)(newCnaHandler(cl, commonTestUtils.GetScheme()))
				res := handler.ensure(req)
				Expect(res.UpgradeDone).To(BeFalse())
				Expect(res.Err).To(BeNil())

				cna := &networkaddonsv1.NetworkAddonsConfig{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: expectedResource.Name, Namespace: expectedResource.Namespace},
						cna),
				).ToNot(HaveOccurred())

				Expect(cna).ToNot(BeNil())
				Expect(cna.Spec.KubeMacPool.RangeStart).Should(Equal("1.1.1.1.1.1"))
				Expect(cna.Spec.KubeMacPool.RangeEnd).Should(Equal("5.5.5.5.5.5"))
				Expect(cna.Spec.ImagePullPolicy).To(BeEquivalentTo("Always"))
			})

			It("Ensure func should fail to create CNA object with wrong jsonPatch", func() {
				hco.Annotations = map[string]string{common.JSONPatchCNAOAnnotationName: `[
					{
						"op": "notExists",
						"path": "/spec/kubeMacPool",
						"value": {"rangeStart": "1.1.1.1.1.1", "rangeEnd": "5.5.5.5.5.5" }
					}
				]`}

				expectedResource := NewNetworkAddonsWithNameOnly(hco)
				cl := commonTestUtils.InitClient([]runtime.Object{})
				handler := (*genericOperand)(newCnaHandler(cl, commonTestUtils.GetScheme()))
				res := handler.ensure(req)
				Expect(res.Err).To(HaveOccurred())

				cna := &networkaddonsv1.NetworkAddonsConfig{}

				err := cl.Get(context.TODO(),
					types.NamespacedName{Name: expectedResource.Name, Namespace: expectedResource.Namespace},
					cna)

				Expect(err).To(HaveOccurred())
				Expect(errors.IsNotFound(err)).To(BeTrue())
			})

			It("Ensure func should update CNA object with changes from the annotation", func() {
				existsCna, err := NewNetworkAddons(hco)
				Expect(err).ToNot(HaveOccurred())

				hco.Annotations = map[string]string{common.JSONPatchCNAOAnnotationName: `[
					{
						"op": "add",
						"path": "/spec/kubeMacPool",
						"value": {"rangeStart": "1.1.1.1.1.1", "rangeEnd": "5.5.5.5.5.5" }
					},
					{
						"op": "add",
						"path": "/spec/imagePullPolicy",
						"value": "Always"
					}
				]`}

				cl := commonTestUtils.InitClient([]runtime.Object{hco, existsCna})

				handler := (*genericOperand)(newCnaHandler(cl, commonTestUtils.GetScheme()))
				res := handler.ensure(req)
				Expect(res.Err).ToNot(HaveOccurred())
				Expect(res.Updated).To(BeTrue())
				Expect(res.UpgradeDone).To(BeFalse())

				cna := &networkaddonsv1.NetworkAddonsConfig{}

				expectedResource := NewNetworkAddonsWithNameOnly(hco)
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: expectedResource.Name, Namespace: expectedResource.Namespace},
						cna),
				).ToNot(HaveOccurred())

				Expect(cna.Spec.KubeMacPool.RangeStart).Should(Equal("1.1.1.1.1.1"))
				Expect(cna.Spec.KubeMacPool.RangeEnd).Should(Equal("5.5.5.5.5.5"))
				Expect(cna.Spec.ImagePullPolicy).To(BeEquivalentTo("Always"))
			})

			It("Ensure func should fail to update CNA object with wrong jsonPatch", func() {
				existsCna, err := NewNetworkAddons(hco)
				Expect(err).ToNot(HaveOccurred())

				hco.Annotations = map[string]string{common.JSONPatchCNAOAnnotationName: `[
					{
						"op": "notExists",
						"path": "/spec/kubeMacPool",
						"value": {"rangeStart": "1.1.1.1.1.1", "rangeEnd": "5.5.5.5.5.5" }
					}
				]`}

				cl := commonTestUtils.InitClient([]runtime.Object{hco, existsCna})

				handler := (*genericOperand)(newCnaHandler(cl, commonTestUtils.GetScheme()))
				res := handler.ensure(req)
				Expect(res.Err).To(HaveOccurred())

				cna := &networkaddonsv1.NetworkAddonsConfig{}

				expectedResource := NewNetworkAddonsWithNameOnly(hco)
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: expectedResource.Name, Namespace: expectedResource.Namespace},
						cna),
				).ToNot(HaveOccurred())

				Expect(cna.Spec.KubeMacPool.RangeStart).To(BeEmpty())
				Expect(cna.Spec.KubeMacPool.RangeEnd).To(BeEmpty())
				Expect(cna.Spec.ImagePullPolicy).To(BeEmpty())
			})
		})

		Context("Cache", func() {
			cl := commonTestUtils.InitClient([]runtime.Object{})
			handler := newCnaHandler(cl, commonTestUtils.GetScheme())

			It("should start with empty cache", func() {
				Expect(handler.hooks.(*cnaHooks).cache).To(BeNil())
			})

			It("should update the cache when reading full CR", func() {
				cr, err := handler.hooks.getFullCr(hco)
				Expect(err).ToNot(HaveOccurred())
				Expect(cr).ToNot(BeNil())
				Expect(handler.hooks.(*cnaHooks).cache).ToNot(BeNil())

				By("compare pointers to make sure cache is working", func() {
					Expect(handler.hooks.(*cnaHooks).cache == cr).Should(BeTrue())

					crII, err := handler.hooks.getFullCr(hco)
					Expect(err).ToNot(HaveOccurred())
					Expect(crII).ToNot(BeNil())
					Expect(cr == crII).Should(BeTrue())
				})
			})

			It("should remove the cache on reset", func() {
				handler.hooks.(*cnaHooks).reset()
				Expect(handler.hooks.(*cnaHooks).cache).To(BeNil())
			})

			It("check that reset actually cause creating of a new cached instance", func() {
				crI, err := handler.hooks.getFullCr(hco)
				Expect(err).ToNot(HaveOccurred())
				Expect(crI).ToNot(BeNil())
				Expect(handler.hooks.(*cnaHooks).cache).ToNot(BeNil())

				handler.hooks.(*cnaHooks).reset()
				Expect(handler.hooks.(*cnaHooks).cache).To(BeNil())

				crII, err := handler.hooks.getFullCr(hco)
				Expect(err).ToNot(HaveOccurred())
				Expect(crII).ToNot(BeNil())
				Expect(handler.hooks.(*cnaHooks).cache).ToNot(BeNil())

				Expect(crI == crII).To(BeFalse())
				Expect(handler.hooks.(*cnaHooks).cache == crI).To(BeFalse())
				Expect(handler.hooks.(*cnaHooks).cache == crII).To(BeTrue())
			})

		})

	})

	Context("hcoConfig2CnaoPlacement", func() {
		seconds1, seconds2 := int64(1), int64(2)
		tolr1 := corev1.Toleration{
			Key: "key1", Operator: "operator1", Value: "value1", Effect: "effect1", TolerationSeconds: &seconds1,
		}
		tolr2 := corev1.Toleration{
			Key: "key2", Operator: "operator2", Value: "value2", Effect: "effect2", TolerationSeconds: &seconds2,
		}

		It("Should return nil if HCO's input is empty", func() {
			Expect(hcoConfig2CnaoPlacement(&sdkapi.NodePlacement{})).To(BeNil())
		})

		It("Should return only NodeSelector", func() {
			hcoConf := &sdkapi.NodePlacement{
				NodeSelector: map[string]string{
					"key1": "value1",
					"key2": "value2",
				},
			}
			cnaoPlacement := hcoConfig2CnaoPlacement(hcoConf)
			Expect(cnaoPlacement).ToNot(BeNil())

			Expect(cnaoPlacement.NodeSelector).ToNot(BeNil())
			Expect(cnaoPlacement.Tolerations).To(BeNil())
			Expect(cnaoPlacement.Affinity.NodeAffinity).To(BeNil())
			Expect(cnaoPlacement.Affinity.PodAffinity).To(BeNil())
			Expect(cnaoPlacement.Affinity.PodAntiAffinity).To(BeNil())

			Expect(cnaoPlacement.NodeSelector["key1"]).Should(Equal("value1"))
			Expect(cnaoPlacement.NodeSelector["key2"]).Should(Equal("value2"))
		})

		It("Should return only Tolerations", func() {
			hcoConf := &sdkapi.NodePlacement{
				Tolerations: []corev1.Toleration{tolr1, tolr2},
			}
			cnaoPlacement := hcoConfig2CnaoPlacement(hcoConf)
			Expect(cnaoPlacement).ToNot(BeNil())

			Expect(cnaoPlacement.NodeSelector).To(BeNil())
			Expect(cnaoPlacement.Tolerations).ToNot(BeNil())
			Expect(cnaoPlacement.Affinity.NodeAffinity).To(BeNil())
			Expect(cnaoPlacement.Affinity.PodAffinity).To(BeNil())
			Expect(cnaoPlacement.Affinity.PodAntiAffinity).To(BeNil())

			Expect(cnaoPlacement.Tolerations[0]).Should(Equal(tolr1))
			Expect(cnaoPlacement.Tolerations[1]).Should(Equal(tolr2))
		})

		It("Should return only Affinity", func() {
			affinity := &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{Key: "key1", Operator: "operator1", Values: []string{"value11, value12"}},
									{Key: "key2", Operator: "operator2", Values: []string{"value21, value22"}},
								},
								MatchFields: []corev1.NodeSelectorRequirement{
									{Key: "key1", Operator: "operator1", Values: []string{"value11, value12"}},
									{Key: "key2", Operator: "operator2", Values: []string{"value21, value22"}},
								},
							},
						},
					},
				},
			}
			hcoConf := &sdkapi.NodePlacement{
				Affinity: affinity,
			}
			cnaoPlacement := hcoConfig2CnaoPlacement(hcoConf)
			Expect(cnaoPlacement).ToNot(BeNil())

			Expect(cnaoPlacement.NodeSelector).To(BeNil())
			Expect(cnaoPlacement.Tolerations).To(BeNil())
			Expect(cnaoPlacement.Affinity.NodeAffinity).ToNot(BeNil())
			Expect(cnaoPlacement.Affinity.PodAffinity).To(BeNil())
			Expect(cnaoPlacement.Affinity.PodAntiAffinity).To(BeNil())

			Expect(cnaoPlacement.Affinity.NodeAffinity).Should(Equal(affinity.NodeAffinity))
		})

		It("Should return the whole object", func() {
			hcoConf := &sdkapi.NodePlacement{

				NodeSelector: map[string]string{
					"key1": "value1",
					"key2": "value2",
				},
				Affinity: &corev1.Affinity{
					NodeAffinity: &corev1.NodeAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
							NodeSelectorTerms: []corev1.NodeSelectorTerm{
								{
									MatchExpressions: []corev1.NodeSelectorRequirement{
										{Key: "key1", Operator: "operator1", Values: []string{"value11, value12"}},
										{Key: "key2", Operator: "operator2", Values: []string{"value21, value22"}},
									},
									MatchFields: []corev1.NodeSelectorRequirement{
										{Key: "key1", Operator: "operator1", Values: []string{"value11, value12"}},
										{Key: "key2", Operator: "operator2", Values: []string{"value21, value22"}},
									},
								},
							},
						},
					},
				},
				Tolerations: []corev1.Toleration{tolr1, tolr2},
			}

			cnaoPlacement := hcoConfig2CnaoPlacement(hcoConf)
			Expect(cnaoPlacement).ToNot(BeNil())

			Expect(cnaoPlacement.NodeSelector).ToNot(BeNil())
			Expect(cnaoPlacement.Tolerations).ToNot(BeNil())
			Expect(cnaoPlacement.Affinity.NodeAffinity).ToNot(BeNil())

			Expect(cnaoPlacement.NodeSelector["key1"]).Should(Equal("value1"))
			Expect(cnaoPlacement.NodeSelector["key2"]).Should(Equal("value2"))

			Expect(cnaoPlacement.Tolerations[0]).Should(Equal(tolr1))
			Expect(cnaoPlacement.Tolerations[1]).Should(Equal(tolr2))

			Expect(cnaoPlacement.Affinity.NodeAffinity).Should(Equal(hcoConf.Affinity.NodeAffinity))
		})
	})
})
