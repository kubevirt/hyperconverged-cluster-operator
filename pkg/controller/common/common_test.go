package common

import (
	"context"
	"k8s.io/apimachinery/pkg/types"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"testing"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/pkg/apis/hco/v1beta1"
	conditionsv1 "github.com/openshift/custom-resource-status/conditions/v1"
	corev1 "k8s.io/api/core/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestControllerCommon(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller Common Suite")
}

var (
	_ = Describe("HCO Conditions Tests", func() {
		Context("Test SetStatusCondition", func() {
			conds := NewHcoConditions()
			Expect(conds.IsEmpty()).To(BeTrue())

			It("Should create new condition", func() {
				conds.SetStatusCondition(conditionsv1.Condition{
					Type:    hcov1beta1.ConditionReconcileComplete,
					Status:  corev1.ConditionFalse,
					Reason:  "reason",
					Message: "a message",
				})

				Expect(conds.IsEmpty()).To(BeFalse())
				Expect(conds).To(HaveLen(1))

				Expect(conds[hcov1beta1.ConditionReconcileComplete]).ToNot(BeNil())
				Expect(conds[hcov1beta1.ConditionReconcileComplete].Type).Should(Equal(hcov1beta1.ConditionReconcileComplete))
				Expect(conds[hcov1beta1.ConditionReconcileComplete].Status).Should(Equal(corev1.ConditionFalse))
				Expect(conds[hcov1beta1.ConditionReconcileComplete].Reason).Should(Equal("reason"))
				Expect(conds[hcov1beta1.ConditionReconcileComplete].Message).Should(Equal("a message"))
			})

			It("Should update a condition if already exists", func() {
				conds.SetStatusCondition(conditionsv1.Condition{
					Type:    hcov1beta1.ConditionReconcileComplete,
					Status:  corev1.ConditionTrue,
					Reason:  "reason2",
					Message: "another message",
				})

				Expect(conds.IsEmpty()).To(BeFalse())
				Expect(conds).To(HaveLen(1))

				Expect(conds[hcov1beta1.ConditionReconcileComplete]).ToNot(BeNil())
				Expect(conds[hcov1beta1.ConditionReconcileComplete].Type).Should(Equal(hcov1beta1.ConditionReconcileComplete))
				Expect(conds[hcov1beta1.ConditionReconcileComplete].Status).Should(Equal(corev1.ConditionTrue))
				Expect(conds[hcov1beta1.ConditionReconcileComplete].Reason).Should(Equal("reason2"))
				Expect(conds[hcov1beta1.ConditionReconcileComplete].Message).Should(Equal("another message"))
			})
		})

		Context("Test SetStatusConditionIfUnset", func() {
			conds := NewHcoConditions()
			Expect(conds.IsEmpty()).To(BeTrue())

			It("Should not update the condition", func() {
				By("Set initial condition")
				conds.SetStatusConditionIfUnset(conditionsv1.Condition{
					Type:    hcov1beta1.ConditionReconcileComplete,
					Status:  corev1.ConditionFalse,
					Reason:  "reason",
					Message: "a message",
				})

				Expect(conds.IsEmpty()).To(BeFalse())
				Expect(conds).To(HaveLen(1))

				Expect(conds[hcov1beta1.ConditionReconcileComplete]).ToNot(BeNil())
				Expect(conds[hcov1beta1.ConditionReconcileComplete].Type).Should(Equal(hcov1beta1.ConditionReconcileComplete))
				Expect(conds[hcov1beta1.ConditionReconcileComplete].Status).Should(Equal(corev1.ConditionFalse))
				Expect(conds[hcov1beta1.ConditionReconcileComplete].Reason).Should(Equal("reason"))
				Expect(conds[hcov1beta1.ConditionReconcileComplete].Message).Should(Equal("a message"))

				By("The condition should not be changed by this call")
				conds.SetStatusConditionIfUnset(conditionsv1.Condition{
					Type:    hcov1beta1.ConditionReconcileComplete,
					Status:  corev1.ConditionTrue,
					Reason:  "reason2",
					Message: "another message",
				})

				Expect(conds.IsEmpty()).To(BeFalse())
				Expect(conds).To(HaveLen(1))

				By("Make sure the values are the same as before and were not changed")
				Expect(conds[hcov1beta1.ConditionReconcileComplete]).ToNot(BeNil())
				Expect(conds[hcov1beta1.ConditionReconcileComplete].Type).Should(Equal(hcov1beta1.ConditionReconcileComplete))
				Expect(conds[hcov1beta1.ConditionReconcileComplete].Status).Should(Equal(corev1.ConditionFalse))
				Expect(conds[hcov1beta1.ConditionReconcileComplete].Reason).Should(Equal("reason"))
				Expect(conds[hcov1beta1.ConditionReconcileComplete].Message).Should(Equal("a message"))
			})
		})

		Context("Test HasCondition", func() {
			conds := NewHcoConditions()
			Expect(conds.IsEmpty()).To(BeTrue())

			It("Should not contain the condition", func() {
				Expect(conds.HasCondition(hcov1beta1.ConditionReconcileComplete)).To(BeFalse())
				conds.SetStatusConditionIfUnset(conditionsv1.Condition{
					Type:    hcov1beta1.ConditionReconcileComplete,
					Status:  corev1.ConditionFalse,
					Reason:  "reason",
					Message: "a message",
				})

				Expect(conds.HasCondition(hcov1beta1.ConditionReconcileComplete)).To(BeTrue())
				Expect(conds.HasCondition(conditionsv1.ConditionAvailable)).To(BeFalse())
			})
		})
	})

	_ = Describe("Test HcoRequest", func() {
		It("should set all the fields", func() {
			ctx := context.TODO()
			req := NewHcoRequest(
				ctx,
				reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      "name",
						Namespace: "namespace",
					},
				},
				logf.Log,
				false,
				true,
			)

			Expect(req.Name).Should(Equal("name"))
			Expect(req.Namespace).Should(Equal("namespace"))
			Expect(req.Ctx).Should(Equal(ctx))
			Expect(req.Conditions).ToNot(BeNil())
			Expect(req.Conditions).To(BeEmpty())
			Expect(req.UpgradeMode).To(BeFalse())
			Expect(req.ComponentUpgradeInProgress).To(BeFalse())
			Expect(req.Dirty).To(BeFalse())
			Expect(req.StatusDirty).To(BeFalse())
		})

		It("should set set upgrade mode to true", func() {
			ctx := context.TODO()
			req := NewHcoRequest(
				ctx,
				reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      "name",
						Namespace: "namespace",
					},
				},
				logf.Log,
				false,
				true,
			)

			Expect(req.ComponentUpgradeInProgress).To(BeFalse())
			Expect(req.Dirty).To(BeFalse())

			req.SetUpgradeMode(true)
			Expect(req.UpgradeMode).To(BeTrue())
			Expect(req.ComponentUpgradeInProgress).To(BeTrue())

			req.SetUpgradeMode(false)
			Expect(req.UpgradeMode).To(BeFalse())
			Expect(req.ComponentUpgradeInProgress).To(BeFalse())
		})
	})
)
