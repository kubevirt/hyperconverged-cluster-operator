package handlers

import (
	"context"
	"maps"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	openshiftconfigv1 "github.com/openshift/api/config/v1"
	conditionsv1 "github.com/openshift/custom-resource-status/conditions/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/reference"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cdiv1beta1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

var _ = Describe("CDI Operand", func() {
	Context("CDI", func() {
		var (
			hco *hcov1beta1.HyperConverged
			req *common.HcoRequest
		)

		defaultFeatureGates := []string{honorWaitForFirstConsumerGate, dataVolumeClaimAdoptionGate, webhookPvcRenderingGate}

		BeforeEach(func() {
			hco = commontestutils.NewHco()
			req = commontestutils.NewReq(hco)
		})

		It("should create if not present", func() {
			expectedResource := NewCDIWithNameOnly(hco)
			cl := commontestutils.InitClient([]client.Object{})
			handler := NewCdiHandler(cl, commontestutils.GetScheme())
			res := handler.Ensure(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).ToNot(HaveOccurred())

			foundResource := &cdiv1beta1.CDI{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: expectedResource.Name, Namespace: expectedResource.Namespace},
					foundResource),
			).To(Succeed())
			Expect(foundResource.Name).To(Equal(expectedResource.Name))
			Expect(foundResource.Labels).To(HaveKeyWithValue(hcoutil.AppLabel, commontestutils.Name))
			Expect(foundResource.Namespace).To(Equal(expectedResource.Namespace))
			Expect(foundResource.Annotations).To(Equal(map[string]string{cdiConfigAuthorityAnnotation: ""}))
		})

		It("should find if present", func() {
			expectedResource, err := NewCDI(hco)
			Expect(err).ToNot(HaveOccurred())
			cl := commontestutils.InitClient([]client.Object{hco, expectedResource})
			handler := NewCdiHandler(cl, commontestutils.GetScheme())
			res := handler.Ensure(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).ToNot(HaveOccurred())

			// Check HCO's status
			Expect(hco.Status.RelatedObjects).To(Not(BeNil()))
			objectRef, err := reference.GetReference(handler.Scheme, expectedResource)
			Expect(err).ToNot(HaveOccurred())
			// ObjectReference should have been added
			Expect(hco.Status.RelatedObjects).To(ContainElement(*objectRef))
			// Check conditions
			Expect(req.Conditions[hcov1beta1.ConditionAvailable]).To(commontestutils.RepresentCondition(metav1.Condition{
				Type:    hcov1beta1.ConditionAvailable,
				Status:  metav1.ConditionFalse,
				Reason:  "CDIConditions",
				Message: "CDI resource has no conditions",
			}))
			Expect(req.Conditions[hcov1beta1.ConditionProgressing]).To(commontestutils.RepresentCondition(metav1.Condition{
				Type:    hcov1beta1.ConditionProgressing,
				Status:  metav1.ConditionTrue,
				Reason:  "CDIConditions",
				Message: "CDI resource has no conditions",
			}))
			Expect(req.Conditions[hcov1beta1.ConditionUpgradeable]).To(commontestutils.RepresentCondition(metav1.Condition{
				Type:    hcov1beta1.ConditionUpgradeable,
				Status:  metav1.ConditionFalse,
				Reason:  "CDIConditions",
				Message: "CDI resource has no conditions",
			}))
		})

		It("should reconcile managed labels to default without touching user added ones", func() {
			const userLabelKey = "userLabelKey"
			const userLabelValue = "userLabelValue"
			outdatedResource, err := NewCDI(hco)
			Expect(err).ToNot(HaveOccurred())
			expectedLabels := maps.Clone(outdatedResource.Labels)
			for k, v := range expectedLabels {
				outdatedResource.Labels[k] = "wrong_" + v
			}
			outdatedResource.Labels[userLabelKey] = userLabelValue

			cl := commontestutils.InitClient([]client.Object{hco, outdatedResource})
			handler := NewCdiHandler(cl, commontestutils.GetScheme())

			res := handler.Ensure(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Updated).To(BeTrue())
			Expect(res.Err).ToNot(HaveOccurred())

			foundResource := &cdiv1beta1.CDI{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: outdatedResource.Name, Namespace: outdatedResource.Namespace},
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
			outdatedResource, err := NewCDI(hco)
			Expect(err).ToNot(HaveOccurred())
			expectedLabels := maps.Clone(outdatedResource.Labels)
			outdatedResource.Labels[userLabelKey] = userLabelValue
			delete(outdatedResource.Labels, hcoutil.AppLabelVersion)

			cl := commontestutils.InitClient([]client.Object{hco, outdatedResource})
			handler := NewCdiHandler(cl, commontestutils.GetScheme())

			res := handler.Ensure(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Updated).To(BeTrue())
			Expect(res.Err).ToNot(HaveOccurred())

			foundResource := &cdiv1beta1.CDI{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: outdatedResource.Name, Namespace: outdatedResource.Namespace},
					foundResource),
			).ToNot(HaveOccurred())

			for k, v := range expectedLabels {
				Expect(foundResource.Labels).To(HaveKeyWithValue(k, v))
			}
			Expect(foundResource.Labels).To(HaveKeyWithValue(userLabelKey, userLabelValue))
		})

		It("should set default UninstallStrategy if missing", func() {
			expectedResource, err := NewCDI(hco)
			Expect(err).ToNot(HaveOccurred())
			missingUSResource, err := NewCDI(hco)
			Expect(err).ToNot(HaveOccurred())
			missingUSResource.Spec.UninstallStrategy = nil

			cl := commontestutils.InitClient([]client.Object{hco, missingUSResource})
			handler := NewCdiHandler(cl, commontestutils.GetScheme())
			res := handler.Ensure(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Updated).To(BeTrue())
			Expect(res.Overwritten).To(BeFalse())
			Expect(res.Err).ToNot(HaveOccurred())

			foundResource := &cdiv1beta1.CDI{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: expectedResource.Name, Namespace: expectedResource.Namespace},
					foundResource),
			).ToNot(HaveOccurred())

			Expect(*foundResource.Spec.UninstallStrategy).To(Equal(cdiv1beta1.CDIUninstallStrategyBlockUninstallIfWorkloadsExist))
		})

		Context("Test node placement", func() {
			It("should add node placement if missing in CDI", func() {
				existingResource, err := NewCDI(hco)
				Expect(err).ToNot(HaveOccurred())

				hco.Spec.Infra = hcov1beta1.HyperConvergedConfig{NodePlacement: commontestutils.NewNodePlacement()}
				hco.Spec.Workloads = hcov1beta1.HyperConvergedConfig{NodePlacement: commontestutils.NewNodePlacement()}

				cl := commontestutils.InitClient([]client.Object{hco, existingResource})
				handler := NewCdiHandler(cl, commontestutils.GetScheme())
				res := handler.Ensure(req)
				Expect(res.UpgradeDone).To(BeFalse())
				Expect(res.Updated).To(BeTrue())
				Expect(res.Overwritten).To(BeFalse())
				Expect(res.Err).ToNot(HaveOccurred())

				foundResource := &cdiv1beta1.CDI{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
						foundResource),
				).ToNot(HaveOccurred())

				Expect(existingResource.Spec.Infra.Affinity).To(BeNil())
				Expect(existingResource.Spec.Infra.Tolerations).To(BeEmpty())
				Expect(existingResource.Spec.Infra.NodeSelector).To(BeNil())
				Expect(existingResource.Spec.Workloads.Affinity).To(BeNil())
				Expect(existingResource.Spec.Workloads.Tolerations).To(BeEmpty())
				Expect(existingResource.Spec.Workloads.NodeSelector).To(BeNil())

				Expect(foundResource.Spec.Infra.Affinity).ToNot(BeNil())
				Expect(foundResource.Spec.Infra.NodeSelector["key1"]).To(Equal("value1"))
				Expect(foundResource.Spec.Infra.NodeSelector["key2"]).To(Equal("value2"))

				Expect(foundResource.Spec.Workloads).ToNot(BeNil())
				Expect(foundResource.Spec.Workloads.Tolerations).To(Equal(hco.Spec.Workloads.NodePlacement.Tolerations))

				Expect(req.Conditions).To(BeEmpty())
			})

			It("should remove node placement if missing in HCO CR", func() {

				hcoNodePlacement := commontestutils.NewHco()
				hcoNodePlacement.Spec.Infra = hcov1beta1.HyperConvergedConfig{NodePlacement: commontestutils.NewNodePlacement()}
				hcoNodePlacement.Spec.Workloads = hcov1beta1.HyperConvergedConfig{NodePlacement: commontestutils.NewNodePlacement()}
				existingResource, err := NewCDI(hcoNodePlacement)
				Expect(err).ToNot(HaveOccurred())

				cl := commontestutils.InitClient([]client.Object{hco, existingResource})
				handler := NewCdiHandler(cl, commontestutils.GetScheme())
				res := handler.Ensure(req)
				Expect(res.UpgradeDone).To(BeFalse())
				Expect(res.Updated).To(BeTrue())
				Expect(res.Overwritten).To(BeFalse())
				Expect(res.Err).ToNot(HaveOccurred())

				foundResource := &cdiv1beta1.CDI{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
						foundResource),
				).ToNot(HaveOccurred())

				Expect(existingResource.Spec.Infra.Affinity).ToNot(BeNil())
				Expect(existingResource.Spec.Infra.Tolerations).ToNot(BeEmpty())
				Expect(existingResource.Spec.Infra.NodeSelector).ToNot(BeNil())
				Expect(existingResource.Spec.Workloads.Affinity).ToNot(BeNil())
				Expect(existingResource.Spec.Workloads.Tolerations).ToNot(BeEmpty())
				Expect(existingResource.Spec.Workloads.NodeSelector).ToNot(BeNil())

				Expect(foundResource.Spec.Infra.Affinity).To(BeNil())
				Expect(foundResource.Spec.Infra.Tolerations).To(BeEmpty())
				Expect(foundResource.Spec.Infra.NodeSelector).To(BeNil())
				Expect(foundResource.Spec.Workloads.Affinity).To(BeNil())
				Expect(foundResource.Spec.Workloads.Tolerations).To(BeEmpty())
				Expect(foundResource.Spec.Workloads.NodeSelector).To(BeNil())

				Expect(req.Conditions).To(BeEmpty())
			})

			It("should modify node placement according to HCO CR", func() {
				hco.Spec.Infra = hcov1beta1.HyperConvergedConfig{NodePlacement: commontestutils.NewNodePlacement()}
				hco.Spec.Workloads = hcov1beta1.HyperConvergedConfig{NodePlacement: commontestutils.NewNodePlacement()}
				existingResource, err := NewCDI(hco)
				Expect(err).ToNot(HaveOccurred())

				// now, modify HCO's node placement
				hco.Spec.Infra.NodePlacement.Tolerations = append(hco.Spec.Infra.NodePlacement.Tolerations, corev1.Toleration{
					Key: "key3", Operator: "operator3", Value: "value3", Effect: "effect3", TolerationSeconds: ptr.To[int64](3),
				})

				hco.Spec.Workloads.NodePlacement.NodeSelector["key1"] = "something else"

				cl := commontestutils.InitClient([]client.Object{hco, existingResource})
				handler := NewCdiHandler(cl, commontestutils.GetScheme())
				res := handler.Ensure(req)
				Expect(res.UpgradeDone).To(BeFalse())
				Expect(res.Updated).To(BeTrue())
				Expect(res.Overwritten).To(BeFalse())
				Expect(res.Err).ToNot(HaveOccurred())

				foundResource := &cdiv1beta1.CDI{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
						foundResource),
				).ToNot(HaveOccurred())

				Expect(existingResource.Spec.Infra.Tolerations).To(HaveLen(2))
				Expect(existingResource.Spec.Workloads.NodeSelector["key1"]).To(Equal("value1"))

				Expect(foundResource.Spec.Infra.Tolerations).To(HaveLen(3))
				Expect(foundResource.Spec.Workloads.NodeSelector["key1"]).To(Equal("something else"))

				Expect(req.Conditions).To(BeEmpty())
			})

			It("should overwrite node placement if directly set on CDI CR", func() {
				hco.Spec.Infra = hcov1beta1.HyperConvergedConfig{NodePlacement: commontestutils.NewNodePlacement()}
				hco.Spec.Workloads = hcov1beta1.HyperConvergedConfig{NodePlacement: commontestutils.NewNodePlacement()}
				existingResource, err := NewCDI(hco)
				Expect(err).ToNot(HaveOccurred())

				// mock a reconciliation triggered by a change in CDI CR
				req.HCOTriggered = false

				// now, modify CDI's node placement
				existingResource.Spec.Infra.Tolerations = append(hco.Spec.Infra.NodePlacement.Tolerations, corev1.Toleration{
					Key: "key3", Operator: "operator3", Value: "value3", Effect: "effect3", TolerationSeconds: ptr.To[int64](3),
				})
				existingResource.Spec.Workloads.Tolerations = append(hco.Spec.Workloads.NodePlacement.Tolerations, corev1.Toleration{
					Key: "key3", Operator: "operator3", Value: "value3", Effect: "effect3", TolerationSeconds: ptr.To[int64](3),
				})

				existingResource.Spec.Infra.NodeSelector["key1"] = "BADvalue1"
				existingResource.Spec.Workloads.NodeSelector["key2"] = "BADvalue2"

				cl := commontestutils.InitClient([]client.Object{hco, existingResource})
				handler := NewCdiHandler(cl, commontestutils.GetScheme())
				res := handler.Ensure(req)
				Expect(res.UpgradeDone).To(BeFalse())
				Expect(res.Updated).To(BeTrue())
				Expect(res.Overwritten).To(BeTrue())
				Expect(res.Err).ToNot(HaveOccurred())

				foundResource := &cdiv1beta1.CDI{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
						foundResource),
				).ToNot(HaveOccurred())

				Expect(existingResource.Spec.Infra.Tolerations).To(HaveLen(3))
				Expect(existingResource.Spec.Workloads.Tolerations).To(HaveLen(3))
				Expect(existingResource.Spec.Infra.NodeSelector["key1"]).To(Equal("BADvalue1"))
				Expect(existingResource.Spec.Workloads.NodeSelector["key2"]).To(Equal("BADvalue2"))

				Expect(foundResource.Spec.Infra.Tolerations).To(HaveLen(2))
				Expect(foundResource.Spec.Workloads.Tolerations).To(HaveLen(2))
				Expect(foundResource.Spec.Infra.NodeSelector["key1"]).To(Equal("value1"))
				Expect(foundResource.Spec.Workloads.NodeSelector["key2"]).To(Equal("value2"))

				Expect(req.Conditions).To(BeEmpty())
			})
		})

		Context("Test Resource Requirements", func() {
			It("should add Resource Requirements if missing in CDI", func() {
				existingResource, err := NewCDI(hco)
				Expect(err).ToNot(HaveOccurred())

				hco.Spec.ResourceRequirements = &hcov1beta1.OperandResourceRequirements{
					StorageWorkloads: &corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("500m"),
							corev1.ResourceMemory: resource.MustParse("2Gi"),
						},
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("250m"),
							corev1.ResourceMemory: resource.MustParse("1Gi"),
						},
					},
				}

				cl := commontestutils.InitClient([]client.Object{hco, existingResource})
				handler := NewCdiHandler(cl, commontestutils.GetScheme())
				res := handler.Ensure(req)
				Expect(res.UpgradeDone).To(BeFalse())
				Expect(res.Updated).To(BeTrue())
				Expect(res.Overwritten).To(BeFalse())
				Expect(res.Err).ToNot(HaveOccurred())

				foundResource := &cdiv1beta1.CDI{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
						foundResource),
				).ToNot(HaveOccurred())

				Expect(foundResource.Spec.Config).ToNot(BeNil())
				Expect(foundResource.Spec.Config.PodResourceRequirements).ToNot(BeNil())
				Expect(foundResource.Spec.Config.PodResourceRequirements.Limits[corev1.ResourceCPU]).To(Equal(resource.MustParse("500m")))
				Expect(foundResource.Spec.Config.PodResourceRequirements.Limits[corev1.ResourceMemory]).To(Equal(resource.MustParse("2Gi")))
				Expect(foundResource.Spec.Config.PodResourceRequirements.Requests[corev1.ResourceCPU]).To(Equal(resource.MustParse("250m")))
				Expect(foundResource.Spec.Config.PodResourceRequirements.Requests[corev1.ResourceMemory]).To(Equal(resource.MustParse("1Gi")))
			})

			It("should remove Resource Requirements if missing in HCO CR", func() {

				hcoResourceRequirements := commontestutils.NewHco()
				hcoResourceRequirements.Spec.ResourceRequirements = &hcov1beta1.OperandResourceRequirements{
					StorageWorkloads: &corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("500m"),
							corev1.ResourceMemory: resource.MustParse("2Gi"),
						},
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("250m"),
							corev1.ResourceMemory: resource.MustParse("1Gi"),
						},
					},
				}

				existingResource, err := NewCDI(hcoResourceRequirements)
				Expect(err).ToNot(HaveOccurred())

				Expect(existingResource.Spec.Config).ToNot(BeNil())
				Expect(existingResource.Spec.Config.PodResourceRequirements).ToNot(BeNil())
				Expect(existingResource.Spec.Config.PodResourceRequirements.Limits[corev1.ResourceCPU]).To(Equal(resource.MustParse("500m")))
				Expect(existingResource.Spec.Config.PodResourceRequirements.Limits[corev1.ResourceMemory]).To(Equal(resource.MustParse("2Gi")))
				Expect(existingResource.Spec.Config.PodResourceRequirements.Requests[corev1.ResourceCPU]).To(Equal(resource.MustParse("250m")))
				Expect(existingResource.Spec.Config.PodResourceRequirements.Requests[corev1.ResourceMemory]).To(Equal(resource.MustParse("1Gi")))

				cl := commontestutils.InitClient([]client.Object{hco, existingResource})
				handler := NewCdiHandler(cl, commontestutils.GetScheme())
				res := handler.Ensure(req)
				Expect(res.UpgradeDone).To(BeFalse())
				Expect(res.Updated).To(BeTrue())
				Expect(res.Overwritten).To(BeFalse())
				Expect(res.Err).ToNot(HaveOccurred())

				foundResource := &cdiv1beta1.CDI{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
						foundResource),
				).ToNot(HaveOccurred())

				Expect(foundResource.Spec.Config).ToNot(BeNil())
				Expect(foundResource.Spec.Config.PodResourceRequirements).To(BeNil())
			})

			It("should modify Resource Requirements according to HCO CR", func() {
				hco.Spec.ResourceRequirements = &hcov1beta1.OperandResourceRequirements{
					StorageWorkloads: &corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("500m"),
							corev1.ResourceMemory: resource.MustParse("2Gi"),
						},
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("250m"),
							corev1.ResourceMemory: resource.MustParse("1Gi"),
						},
					},
				}
				existingResource, err := NewCDI(hco)
				Expect(err).ToNot(HaveOccurred())

				hco.Spec.ResourceRequirements.StorageWorkloads.Limits[corev1.ResourceCPU] = resource.MustParse("1024m")
				hco.Spec.ResourceRequirements.StorageWorkloads.Limits[corev1.ResourceMemory] = resource.MustParse("4Gi")
				hco.Spec.ResourceRequirements.StorageWorkloads.Requests[corev1.ResourceCPU] = resource.MustParse("500m")
				hco.Spec.ResourceRequirements.StorageWorkloads.Requests[corev1.ResourceMemory] = resource.MustParse("2Gi")

				cl := commontestutils.InitClient([]client.Object{hco, existingResource})
				handler := NewCdiHandler(cl, commontestutils.GetScheme())
				res := handler.Ensure(req)
				Expect(res.UpgradeDone).To(BeFalse())
				Expect(res.Updated).To(BeTrue())
				Expect(res.Overwritten).To(BeFalse())
				Expect(res.Err).ToNot(HaveOccurred())

				foundResource := &cdiv1beta1.CDI{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
						foundResource),
				).ToNot(HaveOccurred())

				Expect(foundResource.Spec.Config.PodResourceRequirements.Limits).To(HaveLen(2))
				Expect(foundResource.Spec.Config.PodResourceRequirements.Limits[corev1.ResourceCPU]).To(Equal(resource.MustParse("1024m")))
				Expect(foundResource.Spec.Config.PodResourceRequirements.Limits[corev1.ResourceMemory]).To(Equal(resource.MustParse("4Gi")))
				Expect(foundResource.Spec.Config.PodResourceRequirements.Requests).To(HaveLen(2))
				Expect(foundResource.Spec.Config.PodResourceRequirements.Requests[corev1.ResourceCPU]).To(Equal(resource.MustParse("500m")))
				Expect(foundResource.Spec.Config.PodResourceRequirements.Requests[corev1.ResourceMemory]).To(Equal(resource.MustParse("2Gi")))
			})
		})

		Context("Test FilesystemOverhead", func() {

			hcoFilesystemOverheadValue := cdiv1beta1.FilesystemOverhead{
				Global:       "0.123",
				StorageClass: map[string]cdiv1beta1.Percent{"someStorageClass": cdiv1beta1.Percent("0.321")},
			}
			cdiFilesystemOverheadValue := cdiv1beta1.FilesystemOverhead{
				Global:       "0.234",
				StorageClass: map[string]cdiv1beta1.Percent{"someStorageClass": cdiv1beta1.Percent("0.432")},
			}

			It("should add FilesystemOverhead if missing in CDI", func() {
				existingResource, err := NewCDI(hco)
				Expect(err).ToNot(HaveOccurred())
				hco.Spec.FilesystemOverhead = &hcoFilesystemOverheadValue

				cl := commontestutils.InitClient([]client.Object{hco, existingResource})
				handler := NewCdiHandler(cl, commontestutils.GetScheme())
				res := handler.Ensure(req)
				Expect(res.UpgradeDone).To(BeFalse())
				Expect(res.Updated).To(BeTrue())
				Expect(res.Overwritten).To(BeFalse())
				Expect(res.Err).ToNot(HaveOccurred())

				foundCdi := &cdiv1beta1.CDI{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
						foundCdi),
				).ToNot(HaveOccurred())

				Expect(foundCdi.Spec.Config).ToNot(BeNil())
				Expect(foundCdi.Spec.Config.FilesystemOverhead).ToNot(BeNil())
				Expect(*foundCdi.Spec.Config.FilesystemOverhead).To(Equal(hcoFilesystemOverheadValue))
			})

			It("should remove FilesystemOverhead if missing in HCO CR", func() {
				hcoResourceRequirements := commontestutils.NewHco()

				existingCdi, err := NewCDI(hcoResourceRequirements)
				Expect(err).ToNot(HaveOccurred())
				existingCdi.Spec.Config.FilesystemOverhead = &cdiFilesystemOverheadValue

				Expect(existingCdi.Spec.Config).ToNot(BeNil())
				Expect(existingCdi.Spec.Config.FilesystemOverhead).ToNot(BeNil())
				Expect(*existingCdi.Spec.Config.FilesystemOverhead).To(Equal(cdiFilesystemOverheadValue))

				cl := commontestutils.InitClient([]client.Object{hco, existingCdi})
				handler := NewCdiHandler(cl, commontestutils.GetScheme())
				res := handler.Ensure(req)
				Expect(res.UpgradeDone).To(BeFalse())
				Expect(res.Updated).To(BeTrue())
				Expect(res.Overwritten).To(BeFalse())
				Expect(res.Err).ToNot(HaveOccurred())

				foundCDI := &cdiv1beta1.CDI{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: existingCdi.Name, Namespace: existingCdi.Namespace},
						foundCDI),
				).ToNot(HaveOccurred())

				Expect(foundCDI.Spec.Config).ToNot(BeNil())
				Expect(foundCDI.Spec.Config.FilesystemOverhead).To(BeNil())
			})

			It("should modify FilesystemOverhead according to HCO CR", func() {
				hco.Spec.FilesystemOverhead = &cdiFilesystemOverheadValue
				existingCDI, err := NewCDI(hco)
				Expect(err).ToNot(HaveOccurred())

				Expect(existingCDI.Spec.Config).ToNot(BeNil())
				Expect(*existingCDI.Spec.Config.FilesystemOverhead).To(Equal(cdiFilesystemOverheadValue))

				hco.Spec.FilesystemOverhead = &hcoFilesystemOverheadValue

				cl := commontestutils.InitClient([]client.Object{hco, existingCDI})
				handler := NewCdiHandler(cl, commontestutils.GetScheme())
				res := handler.Ensure(req)
				Expect(res.UpgradeDone).To(BeFalse())
				Expect(res.Updated).To(BeTrue())
				Expect(res.Overwritten).To(BeFalse())
				Expect(res.Err).ToNot(HaveOccurred())

				foundCDI := &cdiv1beta1.CDI{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: existingCDI.Name, Namespace: existingCDI.Namespace},
						foundCDI),
				).ToNot(HaveOccurred())

				Expect(foundCDI.Spec.Config.FilesystemOverhead).ToNot(BeNil())
				Expect(*foundCDI.Spec.Config.FilesystemOverhead).To(Equal(hcoFilesystemOverheadValue))
			})
		})

		Context("Log verbosity", func() {

			It("Should be defined for CDI CR if defined in HCO CR", func() {
				hco.Spec.LogVerbosityConfig = &hcov1beta1.LogVerbosityConfiguration{CDI: ptr.To[int32](4)}
				cdi, err := NewCDI(hco)

				Expect(err).ToNot(HaveOccurred())
				Expect(cdi).ToNot(BeNil())
				Expect(cdi.Spec.Config.LogVerbosity).To(HaveValue(Equal(int32(4))))
			})

			DescribeTable("Should not be defined for CDI CR if not defined in HCO CR", func(logConfig *hcov1beta1.LogVerbosityConfiguration) {
				hco.Spec.LogVerbosityConfig = logConfig
				cdi, err := NewCDI(hco)

				Expect(err).ToNot(HaveOccurred())
				Expect(cdi).ToNot(BeNil())
				Expect(cdi.Spec.Config.LogVerbosity).To(BeNil())
			},
				Entry("nil LogVerbosityConfiguration", nil),
				Entry("nil CDI logs", &hcov1beta1.LogVerbosityConfiguration{CDI: nil}),
			)
		})

		Context("Test ScratchSpaceStorageClass", func() {

			const (
				hcoScratchSpaceStorageClassValue = "hcoScratchSpaceStorageClassValue"
				cdiScratchSpaceStorageClassValue = "cdiScratchSpaceStorageClassValue"
			)

			It("should add ScratchSpaceStorageClass if missing in CDI", func() {
				existingResource, err := NewCDI(hco)
				Expect(err).ToNot(HaveOccurred())
				hco.Spec.ScratchSpaceStorageClass = ptr.To(hcoScratchSpaceStorageClassValue)

				cl := commontestutils.InitClient([]client.Object{hco, existingResource})
				handler := NewCdiHandler(cl, commontestutils.GetScheme())
				res := handler.Ensure(req)
				Expect(res.UpgradeDone).To(BeFalse())
				Expect(res.Updated).To(BeTrue())
				Expect(res.Overwritten).To(BeFalse())
				Expect(res.Err).ToNot(HaveOccurred())

				foundCdi := &cdiv1beta1.CDI{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
						foundCdi),
				).ToNot(HaveOccurred())

				Expect(foundCdi.Spec.Config).ToNot(BeNil())
				Expect(foundCdi.Spec.Config.ScratchSpaceStorageClass).To(HaveValue(Equal(hcoScratchSpaceStorageClassValue)))
			})

			It("should remove ScratchSpaceStorageClass if missing in HCO CR", func() {
				hcoResourceRequirements := commontestutils.NewHco()

				existingCdi, err := NewCDI(hcoResourceRequirements)
				Expect(err).ToNot(HaveOccurred())
				existingCdi.Spec.Config.ScratchSpaceStorageClass = ptr.To(cdiScratchSpaceStorageClassValue)

				Expect(existingCdi.Spec.Config).ToNot(BeNil())
				Expect(existingCdi.Spec.Config.ScratchSpaceStorageClass).To(HaveValue(Equal(cdiScratchSpaceStorageClassValue)))

				cl := commontestutils.InitClient([]client.Object{hco, existingCdi})
				handler := NewCdiHandler(cl, commontestutils.GetScheme())
				res := handler.Ensure(req)
				Expect(res.UpgradeDone).To(BeFalse())
				Expect(res.Updated).To(BeTrue())
				Expect(res.Overwritten).To(BeFalse())
				Expect(res.Err).ToNot(HaveOccurred())

				foundCDI := &cdiv1beta1.CDI{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: existingCdi.Name, Namespace: existingCdi.Namespace},
						foundCDI),
				).ToNot(HaveOccurred())

				Expect(foundCDI.Spec.Config).ToNot(BeNil())
				Expect(foundCDI.Spec.Config.ScratchSpaceStorageClass).To(BeNil())
			})

			It("should modify ScratchSpaceStorageClass according to HCO CR", func() {
				hco.Spec.ScratchSpaceStorageClass = ptr.To(cdiScratchSpaceStorageClassValue)
				existingCDI, err := NewCDI(hco)
				Expect(err).ToNot(HaveOccurred())

				Expect(existingCDI.Spec.Config).ToNot(BeNil())
				Expect(*existingCDI.Spec.Config.ScratchSpaceStorageClass).To(Equal(cdiScratchSpaceStorageClassValue))

				hco.Spec.ScratchSpaceStorageClass = ptr.To(hcoScratchSpaceStorageClassValue)

				cl := commontestutils.InitClient([]client.Object{hco, existingCDI})
				handler := NewCdiHandler(cl, commontestutils.GetScheme())
				res := handler.Ensure(req)
				Expect(res.UpgradeDone).To(BeFalse())
				Expect(res.Updated).To(BeTrue())
				Expect(res.Overwritten).To(BeFalse())
				Expect(res.Err).ToNot(HaveOccurred())

				foundCDI := &cdiv1beta1.CDI{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: existingCDI.Name, Namespace: existingCDI.Namespace},
						foundCDI),
				).ToNot(HaveOccurred())

				Expect(foundCDI.Spec.Config.ScratchSpaceStorageClass).To(HaveValue(Equal(hcoScratchSpaceStorageClassValue)))
			})
		})

		Context("Test StorageImport", func() {

			It("should add InsecureRegistries if exists in HC and missing in CDI", func() {
				existingResource, err := NewCDI(hco)
				Expect(err).ToNot(HaveOccurred())
				hco.Spec.StorageImport = &hcov1beta1.StorageImportConfig{
					InsecureRegistries: []string{"first:5000", "second:5000", "third:5000"},
				}

				cl := commontestutils.InitClient([]client.Object{hco, existingResource})
				handler := NewCdiHandler(cl, commontestutils.GetScheme())
				res := handler.Ensure(req)
				Expect(res.UpgradeDone).To(BeFalse())
				Expect(res.Updated).To(BeTrue())
				Expect(res.Overwritten).To(BeFalse())
				Expect(res.Err).ToNot(HaveOccurred())

				foundCdi := &cdiv1beta1.CDI{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
						foundCdi),
				).ToNot(HaveOccurred())

				Expect(foundCdi.Spec.Config).ToNot(BeNil())
				Expect(foundCdi.Spec.Config.InsecureRegistries).ToNot(BeEmpty())
				Expect(foundCdi.Spec.Config.InsecureRegistries).To(HaveLen(3))
				Expect(foundCdi.Spec.Config.InsecureRegistries).To(ContainElements("first:5000", "second:5000", "third:5000"))
			})

			It("should remove InsecureRegistries if missing in HCO CR", func() {
				existingCdi, err := NewCDI(hco)
				Expect(err).ToNot(HaveOccurred())
				existingCdi.Spec.Config.InsecureRegistries = []string{"first:5000", "second:5000", "third:5000"}

				cl := commontestutils.InitClient([]client.Object{hco, existingCdi})
				handler := NewCdiHandler(cl, commontestutils.GetScheme())
				res := handler.Ensure(req)
				Expect(res.UpgradeDone).To(BeFalse())
				Expect(res.Updated).To(BeTrue())
				Expect(res.Overwritten).To(BeFalse())
				Expect(res.Err).ToNot(HaveOccurred())

				foundCDI := &cdiv1beta1.CDI{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: existingCdi.Name, Namespace: existingCdi.Namespace},
						foundCDI),
				).ToNot(HaveOccurred())

				Expect(foundCDI.Spec.Config).ToNot(BeNil())
				Expect(foundCDI.Spec.Config.InsecureRegistries).To(BeNil())
			})

			It("should modify InsecureRegistries according to HCO CR", func() {
				existingCDI, err := NewCDI(hco)
				Expect(err).ToNot(HaveOccurred())
				existingCDI.Spec.Config.InsecureRegistries = []string{"first:5000", "second:5000", "third:5000"}

				hco.Spec.StorageImport = &hcov1beta1.StorageImportConfig{
					InsecureRegistries: []string{"other1:5000", "other2:5000"},
				}

				cl := commontestutils.InitClient([]client.Object{hco, existingCDI})
				handler := NewCdiHandler(cl, commontestutils.GetScheme())
				res := handler.Ensure(req)
				Expect(res.UpgradeDone).To(BeFalse())
				Expect(res.Updated).To(BeTrue())
				Expect(res.Overwritten).To(BeFalse())
				Expect(res.Err).ToNot(HaveOccurred())

				foundCDI := &cdiv1beta1.CDI{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: existingCDI.Name, Namespace: existingCDI.Namespace},
						foundCDI),
				).ToNot(HaveOccurred())

				Expect(foundCDI.Spec.Config.InsecureRegistries).To(HaveLen(2))
				Expect(foundCDI.Spec.Config.InsecureRegistries).To(ContainElements("other1:5000", "other2:5000"))
			})
		})

		Context("Test UninstallStrategy", func() {

			It("should set BlockUninstallIfWorkloadsExist if missing HCO CR", func() {
				existingResource, err := NewCDI(hco)
				Expect(err).ToNot(HaveOccurred())
				hco.Spec.UninstallStrategy = ""

				cl := commontestutils.InitClient([]client.Object{hco, existingResource})
				handler := NewCdiHandler(cl, commontestutils.GetScheme())
				res := handler.Ensure(req)
				Expect(res.Err).ToNot(HaveOccurred())

				foundCdi := &cdiv1beta1.CDI{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
						foundCdi),
				).ToNot(HaveOccurred())

				Expect(foundCdi.Spec.UninstallStrategy).ToNot(BeNil())
				Expect(*foundCdi.Spec.UninstallStrategy).To(Equal(cdiv1beta1.CDIUninstallStrategyBlockUninstallIfWorkloadsExist))
			})

			It("should set BlockUninstallIfWorkloadsExist if set on HCO CR", func() {
				existingResource, err := NewCDI(hco)
				Expect(err).ToNot(HaveOccurred())
				uninstallStrategy := hcov1beta1.HyperConvergedUninstallStrategyBlockUninstallIfWorkloadsExist
				hco.Spec.UninstallStrategy = uninstallStrategy

				cl := commontestutils.InitClient([]client.Object{hco, existingResource})
				handler := NewCdiHandler(cl, commontestutils.GetScheme())
				res := handler.Ensure(req)
				Expect(res.Err).ToNot(HaveOccurred())

				foundCdi := &cdiv1beta1.CDI{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
						foundCdi),
				).ToNot(HaveOccurred())

				Expect(foundCdi.Spec.UninstallStrategy).ToNot(BeNil())
				Expect(*foundCdi.Spec.UninstallStrategy).To(Equal(cdiv1beta1.CDIUninstallStrategyBlockUninstallIfWorkloadsExist))
			})

			It("should set BlockUninstallIfRemoveWorkloads if set on HCO CR", func() {
				existingResource, err := NewCDI(hco)
				Expect(err).ToNot(HaveOccurred())
				uninstallStrategy := hcov1beta1.HyperConvergedUninstallStrategyRemoveWorkloads
				hco.Spec.UninstallStrategy = uninstallStrategy

				cl := commontestutils.InitClient([]client.Object{hco, existingResource})
				handler := NewCdiHandler(cl, commontestutils.GetScheme())
				res := handler.Ensure(req)
				Expect(res.Err).ToNot(HaveOccurred())

				foundCdi := &cdiv1beta1.CDI{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
						foundCdi),
				).ToNot(HaveOccurred())

				Expect(foundCdi.Spec.UninstallStrategy).ToNot(BeNil())
				Expect(*foundCdi.Spec.UninstallStrategy).To(Equal(cdiv1beta1.CDIUninstallStrategyRemoveWorkloads))
			})

		})

		It("should override CDI config field", func() {
			expectedResource, err := NewCDI(hco)
			Expect(err).ToNot(HaveOccurred())

			// mock a reconciliation triggered by a change in CDI CR
			req.HCOTriggered = false

			// modify a cfg
			expectedResource.Spec.Config = &cdiv1beta1.CDIConfigSpec{
				UploadProxyURLOverride:   ptr.To("proxyOverride"),
				ScratchSpaceStorageClass: ptr.To("aa"),
				PodResourceRequirements:  &corev1.ResourceRequirements{},
				FeatureGates:             []string{"SomeFeatureGate"},
				FilesystemOverhead:       &cdiv1beta1.FilesystemOverhead{Global: "5"},
			}

			cl := commontestutils.InitClient([]client.Object{hco, expectedResource})
			handler := NewCdiHandler(cl, commontestutils.GetScheme())
			res := handler.Ensure(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Updated).To(BeTrue())
			Expect(res.Overwritten).To(BeTrue())
			Expect(res.Err).ToNot(HaveOccurred())

			foundResource := &cdiv1beta1.CDI{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: expectedResource.Name, Namespace: expectedResource.Namespace},
					foundResource),
			).ToNot(HaveOccurred())
			Expect(foundResource.Spec.Config).ToNot(BeNil())
			// contains all that was found
			Expect(foundResource.Spec.Config.UploadProxyURLOverride).To(BeNil())
			Expect(foundResource.Spec.Config.ScratchSpaceStorageClass).To(BeNil())
			Expect(foundResource.Spec.Config.PodResourceRequirements).To(BeNil())
			Expect(foundResource.Spec.Config.FilesystemOverhead).To(BeNil())
			Expect(foundResource.Spec.Config.FeatureGates).To(HaveLen(len(defaultFeatureGates)))
			Expect(foundResource.Spec.Config.FeatureGates).To(ContainElements(defaultFeatureGates))
			Expect(*foundResource.Spec.UninstallStrategy).To(Equal(cdiv1beta1.CDIUninstallStrategyBlockUninstallIfWorkloadsExist))
		})

		It("should add HonorWaitForFirstConsumer, DataVolumeClaimAdoption and WebhookPvcRendering feature gates if Spec.Config if empty", func() {
			expectedResource, err := NewCDI(hco)
			Expect(err).ToNot(HaveOccurred())
			expectedResource.Spec.Config = nil

			// mock a reconciliation triggered by a change in CDI CR
			req.HCOTriggered = false

			cl := commontestutils.InitClient([]client.Object{hco, expectedResource})
			handler := NewCdiHandler(cl, commontestutils.GetScheme())
			res := handler.Ensure(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Updated).To(BeTrue())
			Expect(res.Overwritten).To(BeTrue())
			Expect(res.Err).ToNot(HaveOccurred())

			foundResource := &cdiv1beta1.CDI{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: expectedResource.Name, Namespace: expectedResource.Namespace},
					foundResource),
			).ToNot(HaveOccurred())
			Expect(foundResource.Spec.Config).ToNot(BeNil())
			Expect(foundResource.Spec.Config.FeatureGates).To(HaveLen(len(defaultFeatureGates)))
			Expect(foundResource.Spec.Config.FeatureGates).To(ContainElements(defaultFeatureGates))
			Expect(*foundResource.Spec.UninstallStrategy).To(Equal(cdiv1beta1.CDIUninstallStrategyBlockUninstallIfWorkloadsExist))
		})

		It("should add cert configuration if missing in CDI", func() {
			existingResource := NewCDIWithNameOnly(hco)

			cl := commontestutils.InitClient([]client.Object{hco, existingResource})
			handler := NewCdiHandler(cl, commontestutils.GetScheme())
			res := handler.Ensure(req)

			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Updated).To(BeTrue())
			Expect(res.Err).ToNot(HaveOccurred())

			foundResource := &cdiv1beta1.CDI{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
					foundResource),
			).ToNot(HaveOccurred())

			Expect(existingResource.Spec.CertConfig).To(BeNil())

			Expect(foundResource.Spec.CertConfig).ToNot(BeNil())
			Expect(foundResource.Spec.CertConfig.CA.Duration.Duration.String()).To(Equal("48h0m0s"))
			Expect(foundResource.Spec.CertConfig.CA.RenewBefore.Duration.String()).To(Equal("24h0m0s"))
			Expect(foundResource.Spec.CertConfig.Server.Duration.Duration.String()).To(Equal("24h0m0s"))
			Expect(foundResource.Spec.CertConfig.Server.RenewBefore.Duration.String()).To(Equal("12h0m0s"))

			Expect(req.Conditions).To(BeEmpty())
		})

		It("should set cert config to defaults if missing in HCO CR", func() {
			existingResource := NewCDIWithNameOnly(hco)

			cl := commontestutils.InitClient([]client.Object{hco})
			handler := NewCdiHandler(cl, commontestutils.GetScheme())
			res := handler.Ensure(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Updated).To(BeFalse())
			Expect(res.Err).ToNot(HaveOccurred())

			foundResource := &cdiv1beta1.CDI{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
					foundResource),
			).ToNot(HaveOccurred())

			Expect(existingResource.Spec.CertConfig).To(BeNil())

			Expect(foundResource.Spec.CertConfig).ToNot(BeNil())
			Expect(foundResource.Spec.CertConfig.CA.Duration.Duration.String()).To(Equal("48h0m0s"))
			Expect(foundResource.Spec.CertConfig.CA.RenewBefore.Duration.String()).To(Equal("24h0m0s"))
			Expect(foundResource.Spec.CertConfig.Server.Duration.Duration.String()).To(Equal("24h0m0s"))
			Expect(foundResource.Spec.CertConfig.Server.RenewBefore.Duration.String()).To(Equal("12h0m0s"))

			Expect(req.Conditions).To(BeEmpty())
		})

		It("should modify cert configuration according to HCO CR", func() {
			hcoCertConfig := commontestutils.NewHco()

			existingResource, err := NewCDI(hcoCertConfig)
			Expect(err).ToNot(HaveOccurred())

			hco.Spec.CertConfig = hcov1beta1.HyperConvergedCertConfig{
				CA: hcov1beta1.CertRotateConfigCA{
					Duration:    &metav1.Duration{Duration: 5 * time.Hour},
					RenewBefore: &metav1.Duration{Duration: 6 * time.Hour},
				},
				Server: hcov1beta1.CertRotateConfigServer{
					Duration:    &metav1.Duration{Duration: 7 * time.Hour},
					RenewBefore: &metav1.Duration{Duration: 8 * time.Hour},
				},
			}

			cl := commontestutils.InitClient([]client.Object{hco, existingResource})
			handler := NewCdiHandler(cl, commontestutils.GetScheme())
			res := handler.Ensure(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Updated).To(BeTrue())
			Expect(res.Err).ToNot(HaveOccurred())

			foundResource := &cdiv1beta1.CDI{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
					foundResource),
			).ToNot(HaveOccurred())

			Expect(existingResource.Spec.CertConfig).ToNot(BeNil())
			Expect(existingResource.Spec.CertConfig.CA.Duration.Duration.String()).To(Equal("48h0m0s"))
			Expect(existingResource.Spec.CertConfig.CA.RenewBefore.Duration.String()).To(Equal("24h0m0s"))
			Expect(existingResource.Spec.CertConfig.Server.Duration.Duration.String()).To(Equal("24h0m0s"))
			Expect(existingResource.Spec.CertConfig.Server.RenewBefore.Duration.String()).To(Equal("12h0m0s"))

			Expect(foundResource.Spec.CertConfig).ToNot(BeNil())
			Expect(foundResource.Spec.CertConfig.CA.Duration.Duration.String()).To(Equal("5h0m0s"))
			Expect(foundResource.Spec.CertConfig.CA.RenewBefore.Duration.String()).To(Equal("6h0m0s"))
			Expect(foundResource.Spec.CertConfig.Server.Duration.Duration.String()).To(Equal("7h0m0s"))
			Expect(foundResource.Spec.CertConfig.Server.RenewBefore.Duration.String()).To(Equal("8h0m0s"))
			Expect(req.Conditions).To(BeEmpty())

			// ObjectReference should have been updated
			Expect(hco.Status.RelatedObjects).To(Not(BeNil()))
			objectRefOutdated, err := reference.GetReference(handler.Scheme, existingResource)
			Expect(err).ToNot(HaveOccurred())
			objectRefFound, err := reference.GetReference(handler.Scheme, foundResource)
			Expect(err).ToNot(HaveOccurred())
			Expect(hco.Status.RelatedObjects).To(Not(ContainElement(*objectRefOutdated)))
			Expect(hco.Status.RelatedObjects).To(ContainElement(*objectRefFound))
		})

		It("should handle conditions", func() {
			expectedResource, err := NewCDI(hco)
			Expect(err).ToNot(HaveOccurred())
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
			cl := commontestutils.InitClient([]client.Object{hco, expectedResource})
			handler := NewCdiHandler(cl, commontestutils.GetScheme())
			res := handler.Ensure(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).ToNot(HaveOccurred())

			// Check HCO's status
			Expect(hco.Status.RelatedObjects).To(Not(BeNil()))
			objectRef, err := reference.GetReference(handler.Scheme, expectedResource)
			Expect(err).ToNot(HaveOccurred())
			// ObjectReference should have been added
			Expect(hco.Status.RelatedObjects).To(ContainElement(*objectRef))
			// Check conditions
			Expect(req.Conditions[hcov1beta1.ConditionAvailable]).To(commontestutils.RepresentCondition(metav1.Condition{
				Type:    hcov1beta1.ConditionAvailable,
				Status:  metav1.ConditionFalse,
				Reason:  "CDINotAvailable",
				Message: "CDI is not available: Bar",
			}))
			Expect(req.Conditions[hcov1beta1.ConditionProgressing]).To(commontestutils.RepresentCondition(metav1.Condition{
				Type:    hcov1beta1.ConditionProgressing,
				Status:  metav1.ConditionTrue,
				Reason:  "CDIProgressing",
				Message: "CDI is progressing: Bar",
			}))
			Expect(req.Conditions[hcov1beta1.ConditionUpgradeable]).To(commontestutils.RepresentCondition(metav1.Condition{
				Type:    hcov1beta1.ConditionUpgradeable,
				Status:  metav1.ConditionFalse,
				Reason:  "CDIProgressing",
				Message: "CDI is progressing: Bar",
			}))
			Expect(req.Conditions[hcov1beta1.ConditionDegraded]).To(commontestutils.RepresentCondition(metav1.Condition{
				Type:    hcov1beta1.ConditionDegraded,
				Status:  metav1.ConditionTrue,
				Reason:  "CDIDegraded",
				Message: "CDI is degraded: Bar",
			}))
		})

		Context("Jsonpatch Annotation", func() {
			It("Should create CDI object with changes from the annotation", func() {
				hco.Annotations = map[string]string{common.JSONPatchCDIAnnotationName: `[
					{
						"op": "add",
						"path": "/spec/config/featureGates/-",
						"value": "fg1"
					},
					{
						"op": "add",
						"path": "/spec/config/filesystemOverhead",
						"value": {"global": "50", "storageClass": {"AAA": "75", "BBB": "25"}}
					}
				]`}

				cdi, err := NewCDI(hco)
				Expect(err).ToNot(HaveOccurred())
				Expect(cdi.Spec.Config.FeatureGates).To(HaveLen(len(defaultFeatureGates) + 1))
				Expect(cdi.Spec.Config.FeatureGates).To(ContainElement("fg1"))
				Expect(cdi.Spec.Config.FilesystemOverhead).ToNot(BeNil())
				Expect(cdi.Spec.Config.FilesystemOverhead.Global).To(BeEquivalentTo("50"))
				Expect(cdi.Spec.Config.FilesystemOverhead.StorageClass).To(HaveLen(2))
				Expect(cdi.Spec.Config.FilesystemOverhead.StorageClass["AAA"]).To(BeEquivalentTo("75"))
				Expect(cdi.Spec.Config.FilesystemOverhead.StorageClass["BBB"]).To(BeEquivalentTo("25"))
			})

			It("Should fail to create CDI object with wrong jsonPatch", func() {
				hco.Annotations = map[string]string{common.JSONPatchCDIAnnotationName: `[
					{
						"op": "notExists",
						"path": "/spec/config/featureGates/-",
						"value": "fg1"
					}
				]`}

				_, err := NewCDI(hco)
				Expect(err).To(HaveOccurred())
			})

			It("Ensure func should create CDI object with changes from the annotation", func() {
				hco.Annotations = map[string]string{common.JSONPatchCDIAnnotationName: `[
					{
						"op": "add",
						"path": "/spec/config/featureGates/-",
						"value": "fg1"
					},
					{
						"op": "add",
						"path": "/spec/config/filesystemOverhead",
						"value": {"global": "50", "storageClass": {"AAA": "75", "BBB": "25"}}
					}
				]`}

				expectedResource := NewCDIWithNameOnly(hco)
				cl := commontestutils.InitClient([]client.Object{})
				handler := NewCdiHandler(cl, commontestutils.GetScheme())
				res := handler.Ensure(req)
				Expect(res.UpgradeDone).To(BeFalse())
				Expect(res.Err).ToNot(HaveOccurred())

				cdi := &cdiv1beta1.CDI{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: expectedResource.Name, Namespace: expectedResource.Namespace},
						cdi),
				).ToNot(HaveOccurred())

				Expect(cdi.Spec.Config.FeatureGates).To(HaveLen(len(defaultFeatureGates) + 1))
				Expect(cdi.Spec.Config.FeatureGates).To(ContainElement("fg1"))
				Expect(cdi.Spec.Config.FilesystemOverhead).ToNot(BeNil())
				Expect(cdi.Spec.Config.FilesystemOverhead.Global).To(BeEquivalentTo("50"))
				Expect(cdi.Spec.Config.FilesystemOverhead.StorageClass).To(HaveLen(2))
				Expect(cdi.Spec.Config.FilesystemOverhead.StorageClass["AAA"]).To(BeEquivalentTo("75"))
				Expect(cdi.Spec.Config.FilesystemOverhead.StorageClass["BBB"]).To(BeEquivalentTo("25"))
			})

			It("Ensure func should fail to create CDI object with wrong jsonPatch", func() {
				hco.Annotations = map[string]string{common.JSONPatchCDIAnnotationName: `[
					{
						"op": "notExists",
						"path": "/spec/config/featureGates/-",
						"value": "fg1"
					}
				]`}

				expectedResource := NewCDIWithNameOnly(hco)
				cl := commontestutils.InitClient([]client.Object{})
				handler := NewCdiHandler(cl, commontestutils.GetScheme())
				res := handler.Ensure(req)
				Expect(res.Err).To(HaveOccurred())

				cdi := &cdiv1beta1.CDI{}

				Expect(cl.Get(context.TODO(),
					types.NamespacedName{Name: expectedResource.Name, Namespace: expectedResource.Namespace},
					cdi,
				)).To(MatchError(errors.IsNotFound, "not found error"))
			})

			It("Ensure func should update CDI object with changes from the annotation", func() {
				existsCdi, err := NewCDI(hco)
				Expect(err).ToNot(HaveOccurred())

				hco.Annotations = map[string]string{common.JSONPatchCDIAnnotationName: `[
					{
						"op": "add",
						"path": "/spec/cloneStrategyOverride",
						"value": "copy"
					},
					{
						"op": "add",
						"path": "/spec/ImagePullPolicy",
						"value": "Always"
					}
				]`}

				cl := commontestutils.InitClient([]client.Object{hco, existsCdi})

				handler := NewCdiHandler(cl, commontestutils.GetScheme())
				res := handler.Ensure(req)
				Expect(res.Err).ToNot(HaveOccurred())
				Expect(res.Updated).To(BeTrue())
				Expect(res.UpgradeDone).To(BeFalse())

				cdi := &cdiv1beta1.CDI{}

				expectedResource := NewCDIWithNameOnly(hco)
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: expectedResource.Name, Namespace: expectedResource.Namespace},
						cdi),
				).ToNot(HaveOccurred())

				Expect(cdi.Spec.ImagePullPolicy).To(BeEquivalentTo("Always"))
				Expect(cdi.Spec.CloneStrategyOverride).ToNot(BeNil())
				Expect(*cdi.Spec.CloneStrategyOverride).To(BeEquivalentTo("copy"))
			})

			It("Ensure func should fail to update CDI object with wrong jsonPatch", func() {
				existsCdi, err := NewCDI(hco)
				Expect(err).ToNot(HaveOccurred())

				hco.Annotations = map[string]string{common.JSONPatchCDIAnnotationName: `[
					{
						"op": "notExistsOp",
						"path": "/spec/cloneStrategyOverride",
						"value": "copy"
					},
					{
						"op": "add",
						"path": "/spec/ImagePullPolicy",
						"value": "Always"
					}
				]`}

				cl := commontestutils.InitClient([]client.Object{hco, existsCdi})

				handler := NewCdiHandler(cl, commontestutils.GetScheme())
				res := handler.Ensure(req)
				Expect(res.Err).To(HaveOccurred())

				cdi := &cdiv1beta1.CDI{}

				expectedResource := NewCDIWithNameOnly(hco)
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: expectedResource.Name, Namespace: expectedResource.Namespace},
						cdi),
				).ToNot(HaveOccurred())

				Expect(cdi.Spec.ImagePullPolicy).To(BeEmpty())
				Expect(cdi.Spec.CloneStrategyOverride).To(BeNil())

			})
		})

		Context("Cache", func() {
			It("should create new cache if it empty", func() {
				hook := &cdiHooks{
					Client: commontestutils.InitClient([]client.Object{}),
					Scheme: commontestutils.GetScheme(),
				}

				Expect(hook.cache).To(BeNil())

				firstCallResult, err := hook.GetFullCr(hco)
				Expect(err).ToNot(HaveOccurred())
				Expect(firstCallResult).ToNot(BeNil())
				Expect(hook.cache).To(BeIdenticalTo(firstCallResult))

				secondCallResult, err := hook.GetFullCr(hco)
				Expect(err).ToNot(HaveOccurred())
				Expect(secondCallResult).ToNot(BeNil())
				Expect(hook.cache).To(BeIdenticalTo(secondCallResult))
				Expect(firstCallResult).To(BeIdenticalTo(secondCallResult))

				hook.Reset()
				Expect(hook.cache).To(BeNil())

				thirdCallResult, err := hook.GetFullCr(hco)
				Expect(err).ToNot(HaveOccurred())
				Expect(thirdCallResult).ToNot(BeNil())
				Expect(hook.cache).To(BeIdenticalTo(thirdCallResult))
				Expect(thirdCallResult).ToNot(BeIdenticalTo(firstCallResult))
				Expect(thirdCallResult).ToNot(BeIdenticalTo(secondCallResult))
			})
		})

		Context("TLSSecurityProfile", func() {

			intermediateTLSSecurityProfile := &openshiftconfigv1.TLSSecurityProfile{
				Type:         openshiftconfigv1.TLSProfileIntermediateType,
				Intermediate: &openshiftconfigv1.IntermediateTLSProfile{},
			}
			modernTLSSecurityProfile := &openshiftconfigv1.TLSSecurityProfile{
				Type:   openshiftconfigv1.TLSProfileModernType,
				Modern: &openshiftconfigv1.ModernTLSProfile{},
			}

			It("should modify TLSSecurityProfile on CDI CR according to ApiServer or HCO CR", func() {
				existingResource, err := NewCDI(hco)
				Expect(err).ToNot(HaveOccurred())
				Expect(existingResource.Spec.Config.TLSSecurityProfile).To(Equal(openshift2CdiSecProfile(intermediateTLSSecurityProfile)))

				// now, modify HCO's TLSSecurityProfile
				hco.Spec.TLSSecurityProfile = modernTLSSecurityProfile

				cl := commontestutils.InitClient([]client.Object{hco, existingResource})
				handler := NewCdiHandler(cl, commontestutils.GetScheme())
				res := handler.Ensure(req)
				Expect(res.UpgradeDone).To(BeFalse())
				Expect(res.Updated).To(BeTrue())
				Expect(res.Err).ToNot(HaveOccurred())

				foundResource := &cdiv1beta1.CDI{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
						foundResource),
				).ToNot(HaveOccurred())

				Expect(foundResource.Spec.Config.TLSSecurityProfile).To(Equal(openshift2CdiSecProfile(modernTLSSecurityProfile)))

				Expect(req.Conditions).To(BeEmpty())
			})

			It("should overwrite TLSSecurityProfile if directly set on CDI CR", func() {
				hco.Spec.TLSSecurityProfile = intermediateTLSSecurityProfile
				existingResource, err := NewCDI(hco)
				Expect(err).ToNot(HaveOccurred())

				// mock a reconciliation triggered by a change in CDI CR
				req.HCOTriggered = false

				// now, modify CDI node placement
				existingResource.Spec.Config.TLSSecurityProfile = openshift2CdiSecProfile(modernTLSSecurityProfile)

				cl := commontestutils.InitClient([]client.Object{hco, existingResource})
				handler := NewCdiHandler(cl, commontestutils.GetScheme())
				res := handler.Ensure(req)
				Expect(res.UpgradeDone).To(BeFalse())
				Expect(res.Updated).To(BeTrue())
				Expect(res.Overwritten).To(BeTrue())
				Expect(res.Err).ToNot(HaveOccurred())

				foundResource := &cdiv1beta1.CDI{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
						foundResource),
				).ToNot(HaveOccurred())

				Expect(foundResource.Spec.Config.TLSSecurityProfile).To(Equal(openshift2CdiSecProfile(hco.Spec.TLSSecurityProfile)))
				Expect(foundResource.Spec.Config.TLSSecurityProfile).ToNot(Equal(existingResource.Spec.Config.TLSSecurityProfile))

				Expect(req.Conditions).To(BeEmpty())
			})
		})

		It("should reformat quantity field to add the quantity type, if missing", func() {
			hco.Spec.ResourceRequirements = &hcov1beta1.OperandResourceRequirements{
				StorageWorkloads: &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("1.5"),
					},
				},
			}
			cdi, err := NewCDI(hco)
			Expect(err).ToNot(HaveOccurred())
			Expect(cdi).ToNot(BeNil())
			Expect(cdi.Spec.Config).ToNot(BeNil())
			Expect(cdi.Spec.Config.PodResourceRequirements).ToNot(BeNil())
			Expect(cdi.Spec.Config.PodResourceRequirements.Requests).ToNot(BeEmpty())
			Expect(cdi.Spec.Config.PodResourceRequirements.Requests[corev1.ResourceCPU]).ToNot(Equal(resource.MustParse("1.5")))
			Expect(cdi.Spec.Config.PodResourceRequirements.Requests[corev1.ResourceCPU]).To(Equal(resource.MustParse("1500m")))
		})
	})

})
