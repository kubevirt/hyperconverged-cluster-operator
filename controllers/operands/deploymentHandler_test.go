package operands

import (
	"context"
	"fmt"
	"maps"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
)

const testReadyCondition = "VirtConditionTestDeploymentReady"

var _ = Describe("Deployment Handler", func() {
	Context("update or recreate the Deployment as required", func() {
		var hco *hcov1.HyperConverged
		var req *common.HcoRequest
		var expectedDeployment *appsv1.Deployment

		BeforeEach(func() {
			hco = commontestutils.NewHco()
			req = commontestutils.NewReq(hco)

			expectedDeployment = NewExpectedDeployment(hco)
		})

		It("should recreate the Deployment as LabelSelector has changed", func() {
			modifiedDeployment := &appsv1.Deployment{}
			expectedDeployment.DeepCopyInto(modifiedDeployment)
			// modify the LabelSelector
			modifiedDeployment.Spec.Selector = &metav1.LabelSelector{
				MatchLabels: map[string]string{"key2": "value2"},
			}
			modifiedDeployment.UID = "oldObjectUID"

			// let's initialize the fake client with a modified object
			cl := commontestutils.InitClient([]client.Object{modifiedDeployment})

			foundResource := &appsv1.Deployment{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Namespace: modifiedDeployment.GetNamespace(), Name: modifiedDeployment.GetName()},
					foundResource),
			).ToNot(HaveOccurred())
			Expect(foundResource.GetUID()).To(Equal(types.UID("oldObjectUID")))

			// let's Ensure the handler properly reconcile it back to the expected state
			handler := NewDeploymentHandler(cl, commontestutils.GetScheme(), NewExpectedDeployment)
			res := handler.Ensure(req)
			Expect(res.Updated).To(BeTrue())
			Expect(res.Err).ToNot(HaveOccurred())

			foundResource = &appsv1.Deployment{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Namespace: modifiedDeployment.GetNamespace(), Name: modifiedDeployment.GetName()},
					foundResource),
			).ToNot(HaveOccurred())

			Expect(foundResource.Spec.Selector).To(Equal(expectedDeployment.Spec.Selector))
			// let's check the object UID to Ensure that the object get really deleted and recreated
			Expect(foundResource.ObjectMeta.UID).ToNot(Equal(modifiedDeployment.UID))
		})

		It("should only update, not recreate, the Deployment since LabelSelector hasn't changed", func() {
			modifiedDeployment := &appsv1.Deployment{}
			expectedDeployment.DeepCopyInto(modifiedDeployment)
			// modify only the labels
			gotLabels := modifiedDeployment.GetLabels()
			gotLabels["key1"] = "wrongValue1"
			gotLabels["key2"] = "newValue2"
			modifiedDeployment.SetLabels(gotLabels)
			modifiedDeployment.UID = "oldObjectUID"

			// let's initialize the fake client with a modified object
			cl := commontestutils.InitClient([]client.Object{modifiedDeployment})
			foundResource := &appsv1.Deployment{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Namespace: modifiedDeployment.GetNamespace(), Name: modifiedDeployment.GetName()},
					foundResource),
			).ToNot(HaveOccurred())
			Expect(foundResource.GetUID()).To(Equal(types.UID("oldObjectUID")))

			// let's Ensure the handler properly reconcile it back to the expected state
			handler := NewDeploymentHandler(cl, commontestutils.GetScheme(), NewExpectedDeployment)
			res := handler.Ensure(req)
			Expect(res.Updated).To(BeTrue())
			Expect(res.Err).ToNot(HaveOccurred())

			foundResource = &appsv1.Deployment{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Namespace: modifiedDeployment.GetNamespace(), Name: modifiedDeployment.GetName()},
					foundResource),
			).ToNot(HaveOccurred())

			Expect(foundResource.Spec.Selector).To(Equal(expectedDeployment.Spec.Selector))
			// let's check the object UID to Ensure that the object get updated and not deleted and recreated
			Expect(foundResource.GetUID()).To(Equal(types.UID("oldObjectUID")))
		})

		It("should reconcile managed labels to default without touching user added ones", func() {
			const userLabelKey = "userLabelKey"
			const userLabelValue = "userLabelValue"
			outdatedResource := NewExpectedDeployment(hco)
			expectedLabels := maps.Clone(outdatedResource.Labels)

			for k, v := range expectedLabels {
				outdatedResource.Labels[k] = "wrong_" + v
			}
			outdatedResource.Labels[userLabelKey] = userLabelValue

			cl := commontestutils.InitClient([]client.Object{hco, outdatedResource})
			handler := NewDeploymentHandler(cl, commontestutils.GetScheme(), NewExpectedDeployment)

			res := handler.Ensure(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Updated).To(BeTrue())
			Expect(res.Err).ToNot(HaveOccurred())

			foundResource := &appsv1.Deployment{}
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
			outdatedResource := NewExpectedDeployment(hco)
			expectedLabels := maps.Clone(outdatedResource.Labels)

			removed := false
			for k := range outdatedResource.Labels {
				if !removed {
					delete(outdatedResource.Labels, k)
					removed = true
				}
			}
			outdatedResource.Labels[userLabelKey] = userLabelValue

			cl := commontestutils.InitClient([]client.Object{hco, outdatedResource})
			handler := NewDeploymentHandler(cl, commontestutils.GetScheme(), NewExpectedDeployment)

			res := handler.Ensure(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Updated).To(BeTrue())
			Expect(res.Err).ToNot(HaveOccurred())

			foundResource := &appsv1.Deployment{}
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

	})

	Context("Deployment readiness condition", func() {
		var (
			hco *hcov1.HyperConverged
			req *common.HcoRequest
		)

		BeforeEach(func() {
			hco = commontestutils.NewHco()
			req = commontestutils.NewReq(hco)
		})

		It("should set condition to True when deployment is ready", func() {
			dep := NewExpectedDeployment(hco)
			replicas := *dep.Spec.Replicas
			dep.Status.ReadyReplicas = replicas
			dep.Status.Replicas = replicas
			cl := commontestutils.InitClient([]client.Object{hco, dep})

			handler := NewDeploymentHandlerWithReadyCondition(cl, commontestutils.GetScheme(), NewExpectedDeployment, testReadyCondition)
			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(meta.IsStatusConditionTrue(req.Instance.Status.Conditions, testReadyCondition)).To(BeTrue())
			cond := meta.FindStatusCondition(req.Instance.Status.Conditions, testReadyCondition)
			Expect(cond.Reason).To(Equal("DeploymentReady"))
			Expect(req.StatusDirty).To(BeTrue())
		})

		It("should set condition to False when deployment is not ready", func() {
			dep := NewExpectedDeployment(hco)
			dep.Status.ReadyReplicas = 0
			dep.Status.Replicas = *dep.Spec.Replicas
			cl := commontestutils.InitClient([]client.Object{hco, dep})

			handler := NewDeploymentHandlerWithReadyCondition(cl, commontestutils.GetScheme(), NewExpectedDeployment, testReadyCondition)
			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			cond := meta.FindStatusCondition(req.Instance.Status.Conditions, testReadyCondition)
			Expect(cond).ToNot(BeNil())
			Expect(cond.Status).To(Equal(metav1.ConditionFalse))
			Expect(cond.Reason).To(Equal("DeploymentNotReady"))
			Expect(cond.Message).To(Equal(fmt.Sprintf("0/%d replicas ready", *dep.Spec.Replicas)))
		})

		It("should set condition to False when deployment is not found", func() {
			cl := commontestutils.InitClient([]client.Object{hco})
			handler := &deploymentReadyConditionHandler{
				inner:               &stubOperand{result: &EnsureResult{}},
				client:              cl,
				conditionType:       testReadyCondition,
				deploymentGenerator: NewExpectedDeployment,
			}

			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			cond := meta.FindStatusCondition(req.Instance.Status.Conditions, testReadyCondition)
			Expect(cond).ToNot(BeNil())
			Expect(cond.Status).To(Equal(metav1.ConditionFalse))
			Expect(cond.Reason).To(Equal("DeploymentNotFound"))
			Expect(cond.Message).To(Equal("modifiedDeployment deployment not found"))
			Expect(req.StatusDirty).To(BeTrue())
		})

		It("should remove condition when shouldDeploy is false", func() {
			meta.SetStatusCondition(&hco.Status.Conditions, metav1.Condition{
				Type:   testReadyCondition,
				Status: metav1.ConditionTrue,
				Reason: "DeploymentReady",
			})
			req = commontestutils.NewReq(hco)
			cl := commontestutils.InitClient([]client.Object{hco})

			handler := NewConditionalDeploymentHandlerWithReadyCondition(
				cl,
				commontestutils.GetScheme(),
				NewExpectedDeployment,
				func(_ *hcov1.HyperConverged) bool { return false },
				func(hc *hcov1.HyperConverged) client.Object { return NewExpectedDeployment(hc) },
				testReadyCondition,
			)
			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			cond := meta.FindStatusCondition(req.Instance.Status.Conditions, testReadyCondition)
			Expect(cond).To(BeNil())
			Expect(req.StatusDirty).To(BeTrue())
		})
	})

})

func NewExpectedDeployment(_ *hcov1.HyperConverged) *appsv1.Deployment {
	replicas := int32(1)
	return &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   "modifiedDeployment",
			Labels: map[string]string{"key1": "value1"},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"key1": "value1"},
			},
		},
	}
}

type stubOperand struct {
	result *EnsureResult
}

func (s *stubOperand) Ensure(_ *common.HcoRequest) *EnsureResult {
	if s.result != nil {
		return s.result
	}
	return &EnsureResult{}
}

func (s *stubOperand) Reset() {}
