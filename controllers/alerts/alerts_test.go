package alerts

import (
	"context"
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commonTestUtils"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

var (
	logger = zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)).WithName("alerts-test")
)

func TestAlerts(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Alerts Suite")
}

var _ = Describe("test the alert package", func() {
	Context("Prometheus rule in openshift", func() {
		ci := commonTestUtils.ClusterInfoMock{}
		ee := commonTestUtils.NewEventEmitterMock()

		BeforeEach(func() {
			ee.Reset()
		})

		It("should create if not present", func() {
			cl := commonTestUtils.InitClient([]runtime.Object{})

			reconciler := NewAlertRuleReconciler(cl, ci, ee)
			Expect(reconciler).NotTo(BeNil())

			Expect(reconciler.Reconcile(context.Background(), logger)).To(Succeed())

			res := &monitoringv1.PrometheusRule{}
			Expect(cl.Get(context.Background(), types.NamespacedName{Namespace: commonTestUtils.Namespace, Name: ruleName}, res)).To(Succeed())

			Expect(res.Spec.Groups).To(HaveLen(1))
			Expect(res.Spec.Groups[0].Rules).To(HaveLen(4))
			Expect(res.Labels).Should(HaveKeyWithValue(hcoutil.AppLabel, commonTestUtils.Name))
			testOwnerReferences(res.OwnerReferences)

			expectedEvents := []commonTestUtils.MockEvent{
				{
					EventType: corev1.EventTypeNormal,
					Reason:    "Created",
					Msg:       "Created PrometheusRule " + ruleName,
				},
			}

			Expect(ee.CheckEvents(expectedEvents)).To(BeTrue())
		})

		It("should find if present", func() {
			ci := commonTestUtils.ClusterInfoMock{}
			existRule := newPrometheusRule(commonTestUtils.Namespace, ci.GetDeployment())
			cl := commonTestUtils.InitClient([]runtime.Object{existRule})

			reconciler := NewAlertRuleReconciler(cl, ci, ee)
			Expect(reconciler).NotTo(BeNil())

			Expect(reconciler.Reconcile(context.Background(), logger)).To(Succeed())

			res := &monitoringv1.PrometheusRule{}
			Expect(cl.Get(context.Background(), types.NamespacedName{Namespace: commonTestUtils.Namespace, Name: ruleName}, res)).To(Succeed())

			Expect(res.Spec.Groups).To(HaveLen(1))
			Expect(res.Spec.Groups[0].Rules).To(HaveLen(4))
			Expect(res.Labels).Should(HaveKeyWithValue(hcoutil.AppLabel, commonTestUtils.Name))
			testOwnerReferences(res.OwnerReferences)

			Expect(ee.CheckNoEventEmitted()).To(BeTrue())

		})

		It("should add the owner reference if missing", func() {
			ci := commonTestUtils.ClusterInfoMock{}
			existRule := newPrometheusRule(commonTestUtils.Namespace, ci.GetDeployment())
			existRule.OwnerReferences = nil

			cl := commonTestUtils.InitClient([]runtime.Object{existRule})

			reconciler := NewAlertRuleReconciler(cl, ci, ee)
			Expect(reconciler).NotTo(BeNil())

			Expect(reconciler.Reconcile(context.Background(), logger)).To(Succeed())

			res := &monitoringv1.PrometheusRule{}
			Expect(cl.Get(context.Background(), types.NamespacedName{Namespace: commonTestUtils.Namespace, Name: ruleName}, res)).To(Succeed())

			Expect(res.Spec.Groups).To(HaveLen(1))
			Expect(res.Spec.Groups[0].Rules).To(HaveLen(4))

			testOwnerReferences(res.OwnerReferences)

			expectedEvents := []commonTestUtils.MockEvent{
				{
					EventType: corev1.EventTypeNormal,
					Reason:    "Updated",
					Msg:       "Updated PrometheusRule " + ruleName,
				},
			}

			Expect(ee.CheckEvents(expectedEvents)).To(BeTrue())
		})

		It("should reconcile the rules if modified", func() {
			ci := commonTestUtils.ClusterInfoMock{}
			existRule := newPrometheusRule(commonTestUtils.Namespace, ci.GetDeployment())
			// remove the 2nd rule
			existRule.Spec.Groups[0].Rules = []monitoringv1.Rule{
				existRule.Spec.Groups[0].Rules[0],
				existRule.Spec.Groups[0].Rules[2],
				existRule.Spec.Groups[0].Rules[3],
			}
			// modify the first rule
			existRule.Spec.Groups[0].Rules[0].Alert = "modified alert"

			cl := commonTestUtils.InitClient([]runtime.Object{existRule})

			reconciler := NewAlertRuleReconciler(cl, ci, ee)
			Expect(reconciler).NotTo(BeNil())

			Expect(reconciler.Reconcile(context.Background(), logger)).To(Succeed())

			res := &monitoringv1.PrometheusRule{}
			Expect(cl.Get(context.Background(), types.NamespacedName{Namespace: commonTestUtils.Namespace, Name: ruleName}, res)).To(Succeed())

			Expect(res.Spec.Groups).To(HaveLen(1))
			Expect(res.Spec.Groups[0].Rules).To(HaveLen(4))
			Expect(res.Spec.Groups[0].Rules[0].Alert).Should(Equal(outOfBandUpdateAlert))

			testOwnerReferences(res.OwnerReferences)

			expectedEvents := []commonTestUtils.MockEvent{
				{
					EventType: corev1.EventTypeNormal,
					Reason:    "Updated",
					Msg:       "Updated PrometheusRule " + ruleName,
				},
			}

			Expect(ee.CheckEvents(expectedEvents)).To(BeTrue())
		})

		It("should fix the owner reference if pointing to another owner", func() {
			ci := commonTestUtils.ClusterInfoMock{}
			existRule := newPrometheusRule(commonTestUtils.Namespace, ci.GetDeployment())
			existRule.OwnerReferences = []metav1.OwnerReference{
				{
					APIVersion:         "wrongAPIVersion",
					Kind:               "wrongKind",
					Name:               "wrongName",
					Controller:         pointer.BoolPtr(true),
					BlockOwnerDeletion: pointer.BoolPtr(true),
					UID:                "0987654321",
				},
			}

			cl := commonTestUtils.InitClient([]runtime.Object{existRule})

			reconciler := NewAlertRuleReconciler(cl, ci, ee)
			Expect(reconciler).NotTo(BeNil())

			Expect(reconciler.Reconcile(context.Background(), logger)).To(Succeed())

			res := &monitoringv1.PrometheusRule{}
			Expect(cl.Get(context.Background(), types.NamespacedName{Namespace: commonTestUtils.Namespace, Name: ruleName}, res)).To(Succeed())

			Expect(res.Spec.Groups).To(HaveLen(1))
			Expect(res.Spec.Groups[0].Rules).To(HaveLen(4))

			testOwnerReferences(res.OwnerReferences)
		})

		It("should leave only a referenceOwner of the deployment", func() {
			ci := commonTestUtils.ClusterInfoMock{}
			existRule := newPrometheusRule(commonTestUtils.Namespace, ci.GetDeployment())
			ref := existRule.OwnerReferences[0]
			existRule.OwnerReferences = []metav1.OwnerReference{
				{
					APIVersion:         "wrongAPIVersion1",
					Kind:               "wrongKind1",
					Name:               "wrongName1",
					Controller:         pointer.BoolPtr(true),
					BlockOwnerDeletion: pointer.BoolPtr(true),
					UID:                "0987654321-1",
				},
				ref,
				{
					APIVersion:         "wrongAPIVersion3",
					Kind:               "wrongKind3",
					Name:               "wrongName3",
					Controller:         pointer.BoolPtr(true),
					BlockOwnerDeletion: pointer.BoolPtr(true),
					UID:                "0987654321-3",
				},
			}

			cl := commonTestUtils.InitClient([]runtime.Object{existRule})

			reconciler := NewAlertRuleReconciler(cl, ci, ee)
			Expect(reconciler).NotTo(BeNil())

			Expect(reconciler.Reconcile(context.Background(), logger)).To(Succeed())

			res := &monitoringv1.PrometheusRule{}
			Expect(cl.Get(context.Background(), types.NamespacedName{Namespace: commonTestUtils.Namespace, Name: ruleName}, res)).To(Succeed())

			Expect(res.Spec.Groups).To(HaveLen(1))
			Expect(res.Spec.Groups[0].Rules).To(HaveLen(4))

			testOwnerReferences(res.OwnerReferences)
		})

		Context("error cases", func() {
			fakeError := fmt.Errorf("unexpected error")

			It("should return error if failed to create the rule", func() {
				cl := commonTestUtils.InitClient([]runtime.Object{})
				cl.InitiateCreateErrors(func(obj client.Object) error {
					return fakeError
				})

				reconciler := NewAlertRuleReconciler(cl, ci, ee)
				Expect(reconciler).NotTo(BeNil())

				err := reconciler.Reconcile(context.Background(), logger)
				Expect(err).To(HaveOccurred())
				Expect(err).Should(Equal(fakeError))

				res := &monitoringv1.PrometheusRule{}
				err = cl.Get(context.Background(), types.NamespacedName{Namespace: commonTestUtils.Namespace, Name: ruleName}, res)
				Expect(err).To(HaveOccurred())
				Expect(errors.IsNotFound(err)).Should(BeTrue())
				expectedEvents := []commonTestUtils.MockEvent{
					{
						EventType: corev1.EventTypeWarning,
						Reason:    "UnexpectedError",
						Msg:       "failed to create the " + ruleName + " PrometheusRule",
					},
				}
				Expect(ee.CheckEvents(expectedEvents)).To(BeTrue())
			})

			It("should return error if failed to update the rule", func() {
				existRule := newPrometheusRule(commonTestUtils.Namespace, ci.GetDeployment())
				existRule.OwnerReferences = nil
				cl := commonTestUtils.InitClient([]runtime.Object{existRule})
				cl.InitiateUpdateErrors(func(obj client.Object) error {
					return fakeError
				})

				reconciler := NewAlertRuleReconciler(cl, ci, ee)
				Expect(reconciler).NotTo(BeNil())

				err := reconciler.Reconcile(context.Background(), logger)
				Expect(err).To(HaveOccurred())
				Expect(err).Should(Equal(fakeError))

				res := &monitoringv1.PrometheusRule{}
				Expect(cl.Get(context.Background(), types.NamespacedName{Namespace: commonTestUtils.Namespace, Name: ruleName}, res)).To(Succeed())

				Expect(res.OwnerReferences).To(BeNil())
				expectedEvents := []commonTestUtils.MockEvent{
					{
						EventType: corev1.EventTypeWarning,
						Reason:    "UnexpectedError",
						Msg:       "failed to update the " + ruleName + " PrometheusRule",
					},
				}
				Expect(ee.CheckEvents(expectedEvents)).To(BeTrue())
			})
		})

		Context("Prometheus rule in K8s", func() {
			It("should not return error even if it actually nil", func() {
				var reconciler *AlertRuleReconciler = nil

				Expect(reconciler.Reconcile(context.Background(), logger)).To(Succeed())

				By("Make sure that the Prometheus rule was not created")
				cl := commonTestUtils.InitClient([]runtime.Object{})
				res := &monitoringv1.PrometheusRule{}
				err := cl.Get(context.Background(), types.NamespacedName{Namespace: commonTestUtils.Namespace, Name: ruleName}, res)
				Expect(err).To(HaveOccurred())
				Expect(errors.IsNotFound(err)).To(BeTrue())
			})
		})
	})
})

func testOwnerReferences(refs []metav1.OwnerReference) {
	ExpectWithOffset(1, refs).To(HaveLen(1))
	ref := refs[0]
	ExpectWithOffset(1, ref.Name).Should(Equal(commonTestUtils.RSName))
	ExpectWithOffset(1, ref.APIVersion).Should(Equal("apps/v1"))
	ExpectWithOffset(1, ref.Kind).Should(Equal("Deployment"))
	ExpectWithOffset(1, ref.UID).Should(BeEquivalentTo("1234567890"))
	ExpectWithOffset(1, *ref.Controller).To(BeFalse())
	ExpectWithOffset(1, *ref.BlockOwnerDeletion).To(BeFalse())
}
