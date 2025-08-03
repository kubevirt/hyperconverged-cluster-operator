package handlers

import (
	"context"
	"fmt"
	"maps"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	openshiftconfigv1 "github.com/openshift/api/config/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/reference"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cdiv1beta1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	sspv1beta3 "kubevirt.io/ssp-operator/api/v1beta3"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	goldenimages "github.com/kubevirt/hyperconverged-cluster-operator/controllers/handlers/golden-images"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/nodeinfo"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

var _ = Describe("SSP Operands", func() {

	BeforeEach(func() {
		origGetWorkloadsArchFunc := nodeinfo.GetWorkloadsArchitectures
		origGetControlPlaneArchFunc := nodeinfo.GetControlPlaneArchitectures

		DeferCleanup(func() {
			nodeinfo.GetWorkloadsArchitectures = origGetWorkloadsArchFunc
			nodeinfo.GetControlPlaneArchitectures = origGetControlPlaneArchFunc
		})
	})

	Context("SSP", func() {
		var (
			hco *hcov1beta1.HyperConverged
			req *common.HcoRequest
		)

		BeforeEach(func() {
			hco = commontestutils.NewHco()
			req = commontestutils.NewReq(hco)
		})

		It("should create if not present", func() {
			expectedResource, _, err := NewSSP(hco)
			Expect(err).ToNot(HaveOccurred())
			cl := commontestutils.InitClient([]client.Object{})
			handler := NewSspHandler(cl, commontestutils.GetScheme())
			res := handler.Ensure(req)
			Expect(res.Created).To(BeTrue())
			Expect(res.Updated).To(BeFalse())
			Expect(res.Overwritten).To(BeFalse())
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).ToNot(HaveOccurred())

			foundResource := &sspv1beta3.SSP{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: expectedResource.Name, Namespace: expectedResource.Namespace},
					foundResource),
			).ToNot(HaveOccurred())
			Expect(foundResource.Name).To(Equal(expectedResource.Name))
			Expect(foundResource.Labels).To(HaveKeyWithValue(hcoutil.AppLabel, commontestutils.Name))
			Expect(foundResource.Namespace).To(Equal(expectedResource.Namespace))
		})

		It("should find if present", func() {
			expectedResource, _, err := NewSSP(hco)
			Expect(err).ToNot(HaveOccurred())
			cl := commontestutils.InitClient([]client.Object{hco, expectedResource})
			handler := NewSspHandler(cl, commontestutils.GetScheme())
			res := handler.Ensure(req)
			Expect(res.Created).To(BeFalse())
			Expect(res.Updated).To(BeFalse())
			Expect(res.Overwritten).To(BeFalse())
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).ToNot(HaveOccurred())

			// Check HCO's status
			Expect(hco.Status.RelatedObjects).To(Not(BeNil()))
			objectRef, err := reference.GetReference(handler.Scheme, expectedResource)
			Expect(err).ToNot(HaveOccurred())
			// ObjectReference should have been added
			Expect(hco.Status.RelatedObjects).To(ContainElement(*objectRef))
		})

		It("should reconcile to default", func() {
			const cTNamespace = "nonDefault"
			hco.Spec.CommonTemplatesNamespace = ptr.To(cTNamespace)
			expectedResource, _, err := NewSSP(hco)
			Expect(err).ToNot(HaveOccurred())
			existingResource := expectedResource.DeepCopy()

			existingResource.Spec.TemplateValidator.Replicas = ptr.To(defaultTemplateValidatorReplicas * 2) // non-default value

			req.HCOTriggered = false // mock a reconciliation triggered by a change in NewKubeVirtCommonTemplateBundle CR

			cl := commontestutils.InitClient([]client.Object{hco, existingResource})
			handler := NewSspHandler(cl, commontestutils.GetScheme())
			res := handler.Ensure(req)
			Expect(res.Created).To(BeFalse())
			Expect(res.Updated).To(BeTrue())
			Expect(res.Overwritten).To(BeTrue())
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).ToNot(HaveOccurred())

			foundResource := &sspv1beta3.SSP{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
					foundResource),
			).ToNot(HaveOccurred())
			Expect(foundResource.Spec).To(Equal(expectedResource.Spec))
			Expect(foundResource.Spec.CommonTemplates.Namespace).To(Equal(cTNamespace), "common-templates namespace should equal")

			// ObjectReference should have been updated
			Expect(hco.Status.RelatedObjects).To(Not(BeNil()))
			objectRefOutdated, err := reference.GetReference(handler.Scheme, existingResource)
			Expect(err).ToNot(HaveOccurred())
			objectRefFound, err := reference.GetReference(handler.Scheme, foundResource)
			Expect(err).ToNot(HaveOccurred())
			Expect(hco.Status.RelatedObjects).To(Not(ContainElement(*objectRefOutdated)))
			Expect(hco.Status.RelatedObjects).To(ContainElement(*objectRefFound))
		})

		It("should reconcile managed labels to default without touching user added ones", func() {
			const userLabelKey = "userLabelKey"
			const userLabelValue = "userLabelValue"
			outdatedResource, _, err := NewSSP(hco)
			Expect(err).ToNot(HaveOccurred())
			expectedLabels := maps.Clone(outdatedResource.Labels)
			for k, v := range expectedLabels {
				outdatedResource.Labels[k] = "wrong_" + v
			}
			outdatedResource.Labels[userLabelKey] = userLabelValue

			cl := commontestutils.InitClient([]client.Object{hco, outdatedResource})
			handler := NewSspHandler(cl, commontestutils.GetScheme())

			res := handler.Ensure(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Updated).To(BeTrue())
			Expect(res.Err).ToNot(HaveOccurred())

			foundResource := &sspv1beta3.SSP{}
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
			outdatedResource, _, err := NewSSP(hco)
			Expect(err).ToNot(HaveOccurred())
			expectedLabels := maps.Clone(outdatedResource.Labels)
			outdatedResource.Labels[userLabelKey] = userLabelValue
			delete(outdatedResource.Labels, hcoutil.AppLabelVersion)

			cl := commontestutils.InitClient([]client.Object{hco, outdatedResource})
			handler := NewSspHandler(cl, commontestutils.GetScheme())

			res := handler.Ensure(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Updated).To(BeTrue())
			Expect(res.Err).ToNot(HaveOccurred())

			foundResource := &sspv1beta3.SSP{}
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

		It("should create ssp with deployVmConsoleProxy feature gate enabled", func() {
			hco.Spec.DeployVMConsoleProxy = ptr.To(true)

			expectedResource, _, err := NewSSP(hco)
			Expect(err).ToNot(HaveOccurred())

			Expect(expectedResource.Spec.TokenGenerationService).ToNot(BeNil())
			Expect(expectedResource.Spec.TokenGenerationService.Enabled).To(BeTrue())
		})

		DescribeTable("should copy the HC's EnableMultiArchBootImageImport feature gate, to SSP's EnableMultipleArchitectures field", func(hcFG *bool, matcher gomegatypes.GomegaMatcher) {
			hco.Spec.FeatureGates.EnableMultiArchBootImageImport = hcFG

			ssp, _, err := NewSSP(hco)
			Expect(err).ToNot(HaveOccurred())
			Expect(ssp.Spec.EnableMultipleArchitectures).To(matcher)
		},
			Entry("when HC's EnableMultiArchBootImageImport is nil", nil, BeNil()),
			Entry("when HC's EnableMultiArchBootImageImport is false", ptr.To(false), HaveValue(BeFalse())),
			Entry("when HC's EnableMultiArchBootImageImport is true", ptr.To(true), HaveValue(BeTrue())),
		)

		Context("SSP's Cluster filed", func() {
			It("should set Cluster field to the HC's Cluster field", func() {
				nodeinfo.GetControlPlaneArchitectures = func() []string {
					return []string{"cparch1", "cparch2"}
				}

				nodeinfo.GetWorkloadsArchitectures = func() []string {
					return []string{"wlarch1", "wlarch2", "wlarch3"}
				}

				ssp, _, err := NewSSP(hco)
				Expect(err).ToNot(HaveOccurred())
				Expect(ssp.Spec.Cluster).ToNot(BeNil())
				Expect(ssp.Spec.Cluster.ControlPlaneArchitectures).To(HaveLen(2))
				Expect(ssp.Spec.Cluster.ControlPlaneArchitectures).To(ConsistOf("cparch1", "cparch2"))
				Expect(ssp.Spec.Cluster.WorkloadArchitectures).To(HaveLen(3))
				Expect(ssp.Spec.Cluster.WorkloadArchitectures).To(ConsistOf("wlarch1", "wlarch2", "wlarch3"))

			})

			It("should not set Cluster.ControlPlaneArchitectures field if there are no cp nodes", func() {
				nodeinfo.GetControlPlaneArchitectures = func() []string {
					return nil
				}

				nodeinfo.GetWorkloadsArchitectures = func() []string {
					return []string{"wlarch1", "wlarch2", "wlarch3"}
				}

				ssp, _, err := NewSSP(hco)
				Expect(err).ToNot(HaveOccurred())
				Expect(ssp.Spec.Cluster).ToNot(BeNil())
				Expect(ssp.Spec.Cluster.ControlPlaneArchitectures).To(BeNil())
				Expect(ssp.Spec.Cluster.WorkloadArchitectures).To(HaveLen(3))
				Expect(ssp.Spec.Cluster.WorkloadArchitectures).To(ConsistOf("wlarch1", "wlarch2", "wlarch3"))
			})

			It("should not set Cluster.WorkloadArchitectures field if there are no wl nodes", func() {
				nodeinfo.GetControlPlaneArchitectures = func() []string {
					return []string{"cparch1", "cparch2"}
				}

				nodeinfo.GetWorkloadsArchitectures = func() []string {
					return nil
				}

				ssp, _, err := NewSSP(hco)
				Expect(err).ToNot(HaveOccurred())
				Expect(ssp.Spec.Cluster).ToNot(BeNil())
				Expect(ssp.Spec.Cluster.ControlPlaneArchitectures).To(HaveLen(2))
				Expect(ssp.Spec.Cluster.ControlPlaneArchitectures).To(ConsistOf("cparch1", "cparch2"))
				Expect(ssp.Spec.Cluster.WorkloadArchitectures).To(BeNil())
			})

			It("should not set Cluster field if there are no nodes", func() { // should never happen, but just in case
				nodeinfo.GetControlPlaneArchitectures = func() []string {
					return nil
				}

				nodeinfo.GetWorkloadsArchitectures = func() []string {
					return nil
				}

				ssp, _, err := NewSSP(hco)
				Expect(err).ToNot(HaveOccurred())
				Expect(ssp.Spec.Cluster).To(BeNil())
			})
		})

		Context("Node placement", func() {

			It("should add node placement if missing", func() {
				existingResource, _, err := NewSSP(hco)
				Expect(err).ToNot(HaveOccurred())

				hco.Spec.Workloads.NodePlacement = commontestutils.NewNodePlacement()
				hco.Spec.Infra.NodePlacement = commontestutils.NewOtherNodePlacement()

				cl := commontestutils.InitClient([]client.Object{hco, existingResource})
				handler := NewSspHandler(cl, commontestutils.GetScheme())
				res := handler.Ensure(req)
				Expect(res.Created).To(BeFalse())
				Expect(res.Updated).To(BeTrue())
				Expect(res.Overwritten).To(BeFalse())
				Expect(res.UpgradeDone).To(BeFalse())
				Expect(res.Err).ToNot(HaveOccurred())

				foundResource := &sspv1beta3.SSP{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
						foundResource),
				).ToNot(HaveOccurred())

				Expect(existingResource.Spec.TemplateValidator.Placement).To(BeNil())
				// TODO: replace BeEquivalentTo with BeEqual once SSP will consume kubevirt.io/controller-lifecycle-operator-sdk/api v0.2.4
				Expect(*foundResource.Spec.TemplateValidator.Placement).To(BeEquivalentTo(*hco.Spec.Infra.NodePlacement))
				Expect(req.Conditions).To(BeEmpty())
			})

			It("should remove node placement if missing in HCO CR", func() {

				hcoNodePlacement := commontestutils.NewHco()
				hcoNodePlacement.Spec.Workloads.NodePlacement = commontestutils.NewNodePlacement()
				hcoNodePlacement.Spec.Infra.NodePlacement = commontestutils.NewOtherNodePlacement()
				existingResource, _, err := NewSSP(hcoNodePlacement)
				Expect(err).ToNot(HaveOccurred())

				cl := commontestutils.InitClient([]client.Object{hco, existingResource})
				handler := NewSspHandler(cl, commontestutils.GetScheme())
				res := handler.Ensure(req)
				Expect(res.Created).To(BeFalse())
				Expect(res.Updated).To(BeTrue())
				Expect(res.Overwritten).To(BeFalse())
				Expect(res.UpgradeDone).To(BeFalse())
				Expect(res.Err).ToNot(HaveOccurred())

				foundResource := &sspv1beta3.SSP{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
						foundResource),
				).ToNot(HaveOccurred())

				Expect(existingResource.Spec.TemplateValidator.Placement).ToNot(BeNil())
				Expect(foundResource.Spec.TemplateValidator.Placement).To(BeNil())
				Expect(req.Conditions).To(BeEmpty())
			})

			It("should modify node placement according to HCO CR", func() {

				hco.Spec.Workloads.NodePlacement = commontestutils.NewNodePlacement()
				hco.Spec.Infra.NodePlacement = commontestutils.NewOtherNodePlacement()
				existingResource, _, err := NewSSP(hco)
				Expect(err).ToNot(HaveOccurred())

				// now, modify HCO's node placement
				hco.Spec.Workloads.NodePlacement.Tolerations = append(hco.Spec.Workloads.NodePlacement.Tolerations, corev1.Toleration{
					Key: "key12", Operator: "operator12", Value: "value12", Effect: "effect12", TolerationSeconds: ptr.To[int64](12),
				})
				hco.Spec.Workloads.NodePlacement.NodeSelector["key1"] = "something else"

				hco.Spec.Infra.NodePlacement.Tolerations = append(hco.Spec.Infra.NodePlacement.Tolerations, corev1.Toleration{
					Key: "key34", Operator: "operator34", Value: "value34", Effect: "effect34", TolerationSeconds: ptr.To[int64](34),
				})
				hco.Spec.Infra.NodePlacement.NodeSelector["key3"] = "something entirely else"

				cl := commontestutils.InitClient([]client.Object{hco, existingResource})
				handler := NewSspHandler(cl, commontestutils.GetScheme())
				res := handler.Ensure(req)
				Expect(res.Created).To(BeFalse())
				Expect(res.Updated).To(BeTrue())
				Expect(res.Overwritten).To(BeFalse())
				Expect(res.UpgradeDone).To(BeFalse())
				Expect(res.Err).ToNot(HaveOccurred())

				foundResource := &sspv1beta3.SSP{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
						foundResource),
				).ToNot(HaveOccurred())

				Expect(existingResource.Spec.TemplateValidator.Placement.Affinity.NodeAffinity).ToNot(BeNil())
				Expect(existingResource.Spec.TemplateValidator.Placement.Tolerations).To(HaveLen(2))
				Expect(existingResource.Spec.TemplateValidator.Placement.NodeSelector).To(HaveKeyWithValue("key3", "value3"))

				Expect(foundResource.Spec.TemplateValidator.Placement.Affinity.NodeAffinity).ToNot(BeNil())
				Expect(foundResource.Spec.TemplateValidator.Placement.Tolerations).To(HaveLen(3))
				Expect(foundResource.Spec.TemplateValidator.Placement.NodeSelector).To(HaveKeyWithValue("key3", "something entirely else"))

				Expect(req.Conditions).To(BeEmpty())
			})

			It("should overwrite node placement if directly set on SSP CR", func() {
				hco.Spec.Workloads = hcov1beta1.HyperConvergedConfig{NodePlacement: commontestutils.NewNodePlacement()}
				hco.Spec.Infra = hcov1beta1.HyperConvergedConfig{NodePlacement: commontestutils.NewOtherNodePlacement()}
				existingResource, _, err := NewSSP(hco)
				Expect(err).ToNot(HaveOccurred())

				// mock a reconciliation triggered by a change in NewKubeVirtNodeLabellerBundle CR
				req.HCOTriggered = false

				// and modify TemplateValidator node placement
				existingResource.Spec.TemplateValidator.Placement.Tolerations = append(hco.Spec.Infra.NodePlacement.Tolerations, corev1.Toleration{
					Key: "key34", Operator: "operator34", Value: "value34", Effect: "effect34", TolerationSeconds: ptr.To(int64(34)),
				})
				existingResource.Spec.TemplateValidator.Placement.NodeSelector["key3"] = "BADvalue3"

				cl := commontestutils.InitClient([]client.Object{hco, existingResource})
				handler := NewSspHandler(cl, commontestutils.GetScheme())
				res := handler.Ensure(req)
				Expect(res.UpgradeDone).To(BeFalse())
				Expect(res.Updated).To(BeTrue())
				Expect(res.Overwritten).To(BeTrue())
				Expect(res.Err).ToNot(HaveOccurred())

				foundResource := &sspv1beta3.SSP{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
						foundResource),
				).ToNot(HaveOccurred())

				Expect(existingResource.Spec.TemplateValidator.Placement.Tolerations).To(HaveLen(3))
				Expect(existingResource.Spec.TemplateValidator.Placement.NodeSelector).To(HaveKeyWithValue("key3", "BADvalue3"))

				Expect(foundResource.Spec.TemplateValidator.Placement.Tolerations).To(HaveLen(2))
				Expect(foundResource.Spec.TemplateValidator.Placement.NodeSelector).To(HaveKeyWithValue("key3", "value3"))

				Expect(req.Conditions).To(BeEmpty())
			})
		})

		Context("jsonpath Annotation", func() {
			It("Should create SSP object with changes from the annotation", func() {
				hco.Annotations = map[string]string{common.JSONPatchSSPAnnotationName: `[
					{
						"op": "replace",
						"path": "/spec/templateValidator/replicas",
						"value": 5
					}
				]`}

				ssp, _, err := NewSSP(hco)
				Expect(err).ToNot(HaveOccurred())
				Expect(ssp).ToNot(BeNil())
				Expect(ssp.Spec.TemplateValidator.Replicas).ToNot(BeNil())
				Expect(*ssp.Spec.TemplateValidator.Replicas).To(Equal(int32(5)))
			})

			It("Should fail to create SSP object with wrong jsonPatch", func() {
				hco.Annotations = map[string]string{common.JSONPatchSSPAnnotationName: `[
					{
						"op": "notExists",
						"path": "/spec/templateValidator/replicas",
						"value": 5
					}
				]`}

				_, _, err := NewSSP(hco)
				Expect(err).To(HaveOccurred())
			})

			It("Ensure func should create SSP object with changes from the annotation", func() {
				hco.Annotations = map[string]string{common.JSONPatchSSPAnnotationName: `[
					{
						"op": "replace",
						"path": "/spec/templateValidator/replicas",
						"value": 5
					}
				]`}

				expectedResource := NewSSPWithNameOnly(hco)
				cl := commontestutils.InitClient([]client.Object{})
				handler := NewSspHandler(cl, commontestutils.GetScheme())
				res := handler.Ensure(req)
				Expect(res.UpgradeDone).To(BeFalse())
				Expect(res.Err).ToNot(HaveOccurred())

				ssp := &sspv1beta3.SSP{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: expectedResource.Name, Namespace: expectedResource.Namespace},
						ssp),
				).To(Succeed())

				Expect(ssp).ToNot(BeNil())
				Expect(ssp.Spec.TemplateValidator.Replicas).ToNot(BeNil())
				Expect(*ssp.Spec.TemplateValidator.Replicas).To(Equal(int32(5)))
			})

			It("Ensure func should fail to create SSP object with wrong jsonPatch", func() {
				hco.Annotations = map[string]string{common.JSONPatchSSPAnnotationName: `[
					{
						"op": "notExists",
						"path": "/spec/templateValidator/replicas",
						"value": 5
					}
				]`}

				expectedResource := NewSSPWithNameOnly(hco)
				cl := commontestutils.InitClient([]client.Object{})
				handler := NewSspHandler(cl, commontestutils.GetScheme())
				res := handler.Ensure(req)
				Expect(res.Err).To(HaveOccurred())

				ssp := &sspv1beta3.SSP{}

				Expect(cl.Get(context.TODO(),
					types.NamespacedName{Name: expectedResource.Name, Namespace: expectedResource.Namespace},
					ssp,
				)).To(MatchError(errors.IsNotFound, "not found error"))
			})

			It("Ensure func should update SSP object with changes from the annotation", func() {
				existsSsp, _, err := NewSSP(hco)
				Expect(err).ToNot(HaveOccurred())

				hco.Annotations = map[string]string{common.JSONPatchSSPAnnotationName: `[
					{
						"op": "replace",
						"path": "/spec/templateValidator/replicas",
						"value": 5
					}
				]`}

				cl := commontestutils.InitClient([]client.Object{hco, existsSsp})

				handler := NewSspHandler(cl, commontestutils.GetScheme())
				res := handler.Ensure(req)
				Expect(res.Err).ToNot(HaveOccurred())
				Expect(res.Updated).To(BeTrue())
				Expect(res.UpgradeDone).To(BeFalse())

				ssp := &sspv1beta3.SSP{}

				expectedResource := NewSSPWithNameOnly(hco)
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: expectedResource.Name, Namespace: expectedResource.Namespace},
						ssp),
				).To(Succeed())

				Expect(ssp.Spec.TemplateValidator.Replicas).ToNot(BeNil())
				Expect(*ssp.Spec.TemplateValidator.Replicas).To(Equal(int32(5)))
			})

			It("Ensure func should fail to update SSP object with wrong jsonPatch", func() {
				existsSsp, _, err := NewSSP(hco)
				Expect(err).ToNot(HaveOccurred())

				hco.Annotations = map[string]string{common.JSONPatchSSPAnnotationName: `[
					{
						"op": "notExists",
						"path": "/spec/templateValidator/replicas",
						"value": 5
					}
				]`}

				cl := commontestutils.InitClient([]client.Object{hco, existsSsp})

				handler := NewSspHandler(cl, commontestutils.GetScheme())
				res := handler.Ensure(req)
				Expect(res.Err).To(HaveOccurred())

				ssp := &sspv1beta3.SSP{}

				expectedResource := NewSSPWithNameOnly(hco)
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: expectedResource.Name, Namespace: expectedResource.Namespace},
						ssp),
				).To(Succeed())

				Expect(ssp.Spec.TemplateValidator.Replicas).ToNot(BeNil())
				Expect(*ssp.Spec.TemplateValidator.Replicas).To(Equal(defaultTemplateValidatorReplicas))
			})
		})

		Context("Cache", func() {

			It("should create new cache if it empty", func() {
				hook := &sspHooks{}

				Expect(hook.cache).To(BeNil())

				origFunc := goldenimages.GetDataImportCronTemplates
				goldenimages.GetDataImportCronTemplates = func(_ *hcov1beta1.HyperConverged) ([]hcov1beta1.DataImportCronTemplateStatus, error) {
					return []hcov1beta1.DataImportCronTemplateStatus{makeDICT(1)}, nil
				}
				DeferCleanup(func() {
					goldenimages.GetDataImportCronTemplates = origFunc
				})

				hco.Spec.EnableCommonBootImageImport = ptr.To(true)

				firstCR, err := hook.GetFullCr(hco)
				Expect(err).ToNot(HaveOccurred())
				Expect(firstCR).ToNot(BeNil())
				Expect(hook.cache).To(BeIdenticalTo(firstCR))

				secondCR, err := hook.GetFullCr(hco)
				Expect(err).ToNot(HaveOccurred())
				Expect(secondCR).ToNot(BeNil())
				Expect(hook.cache).To(BeIdenticalTo(secondCR))
				Expect(firstCR).To(BeIdenticalTo(secondCR))

				hook.Reset()
				Expect(hook.cache).To(BeNil())

				thirdCR, err := hook.GetFullCr(hco)
				Expect(err).ToNot(HaveOccurred())
				Expect(thirdCR).ToNot(BeNil())
				Expect(hook.cache).To(BeIdenticalTo(thirdCR))
				Expect(thirdCR).ToNot(BeIdenticalTo(firstCR))
				Expect(thirdCR).ToNot(BeIdenticalTo(secondCR))
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

			It("should modify TLSSecurityProfile on SSP CR according to ApiServer or HCO CR", func() {
				existingResource, _, err := NewSSP(hco)
				Expect(err).ToNot(HaveOccurred())
				Expect(existingResource.Spec.TLSSecurityProfile).To(Equal(intermediateTLSSecurityProfile))

				// now, modify HCO's TLSSecurityProfile
				hco.Spec.TLSSecurityProfile = modernTLSSecurityProfile

				cl := commontestutils.InitClient([]client.Object{hco, existingResource})
				handler := NewSspHandler(cl, commontestutils.GetScheme())
				res := handler.Ensure(req)
				Expect(res.UpgradeDone).To(BeFalse())
				Expect(res.Updated).To(BeTrue())
				Expect(res.Err).ToNot(HaveOccurred())

				foundResource := &sspv1beta3.SSP{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
						foundResource),
				).ToNot(HaveOccurred())

				Expect(foundResource.Spec.TLSSecurityProfile).To(Equal(modernTLSSecurityProfile))

				Expect(req.Conditions).To(BeEmpty())
			})

			It("should overwrite TLSSecurityProfile if directly set on SSP CR", func() {
				hco.Spec.TLSSecurityProfile = intermediateTLSSecurityProfile
				existingResource, _, err := NewSSP(hco)
				Expect(err).ToNot(HaveOccurred())

				// mock a reconciliation triggered by a change in CDI CR
				req.HCOTriggered = false

				// now, modify SSP TLSSecurityProfile
				existingResource.Spec.TLSSecurityProfile = modernTLSSecurityProfile

				cl := commontestutils.InitClient([]client.Object{hco, existingResource})
				handler := NewSspHandler(cl, commontestutils.GetScheme())
				res := handler.Ensure(req)
				Expect(res.UpgradeDone).To(BeFalse())
				Expect(res.Updated).To(BeTrue())
				Expect(res.Overwritten).To(BeTrue())
				Expect(res.Err).ToNot(HaveOccurred())

				foundResource := &sspv1beta3.SSP{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
						foundResource),
				).ToNot(HaveOccurred())

				Expect(foundResource.Spec.TLSSecurityProfile).To(Equal(hco.Spec.TLSSecurityProfile))
				Expect(foundResource.Spec.TLSSecurityProfile).ToNot(Equal(existingResource.Spec.TLSSecurityProfile))

				Expect(req.Conditions).To(BeEmpty())
			})
		})
	})
})

func makeDICT(num int) hcov1beta1.DataImportCronTemplateStatus {
	name := fmt.Sprintf("image%d", num)

	dict := hcov1beta1.DataImportCronTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			//Annotations: make(map[string]string),
		},
		Spec: &cdiv1beta1.DataImportCronSpec{
			Schedule: fmt.Sprintf("%d */12 * * *", num),
			Template: cdiv1beta1.DataVolume{
				Spec: cdiv1beta1.DataVolumeSpec{
					Source: &cdiv1beta1.DataVolumeSource{
						Registry: &cdiv1beta1.DataVolumeSourceRegistry{URL: ptr.To(fmt.Sprintf("docker://someregistry/%s", name))},
					},
				},
			},
			ManagedDataSource: name,
		},
	}

	return hcov1beta1.DataImportCronTemplateStatus{
		DataImportCronTemplate: *dict.DeepCopy(),
		Status: hcov1beta1.DataImportCronStatus{
			CommonTemplate: true,
			Modified:       false,
		},
	}
}
