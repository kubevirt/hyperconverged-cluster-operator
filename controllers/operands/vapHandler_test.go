package operands

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

func newTestVAP() *admissionregistrationv1.ValidatingAdmissionPolicy {
	return &admissionregistrationv1.ValidatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "test-policy",
			Labels: GetLabels(hcoutil.AppComponentDeployment),
		},
		Spec: admissionregistrationv1.ValidatingAdmissionPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
					RuleWithOperations: admissionregistrationv1.RuleWithOperations{
						Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{"example.kubevirt.io"},
							APIVersions: []string{"v1"},
							Resources:   []string{"tests"},
						},
					},
				}},
			},
			Validations: []admissionregistrationv1.Validation{
				{Expression: "true"},
			},
		},
	}
}

func newTestVAPB() *admissionregistrationv1.ValidatingAdmissionPolicyBinding {
	return &admissionregistrationv1.ValidatingAdmissionPolicyBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "test-policy",
			Labels: GetLabels(hcoutil.AppComponentDeployment),
		},
		Spec: admissionregistrationv1.ValidatingAdmissionPolicyBindingSpec{
			PolicyName:        "test-policy",
			ValidationActions: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny},
		},
	}
}

var _ = Describe("ValidatingAdmissionPolicy Handlers", func() {
	var (
		hco *hcov1.HyperConverged
		req *common.HcoRequest
	)

	BeforeEach(func() {
		hco = commontestutils.NewHco()
		req = commontestutils.NewReq(hco)
	})

	Context("ValidatingAdmissionPolicy", func() {
		It("should create the policy", func() {
			cl := commontestutils.InitClient([]client.Object{hco})
			res := NewValidatingAdmissionPolicyHandler(cl, commontestutils.GetScheme(), newTestVAP()).Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Created).To(BeTrue())
		})

		It("should not update for fields the apiserver defaults", func() {
			existing := newTestVAP()
			existing.Spec.FailurePolicy = new(admissionregistrationv1.Fail)
			existing.Spec.MatchConstraints.MatchPolicy = new(admissionregistrationv1.Equivalent)
			existing.Spec.MatchConstraints.NamespaceSelector = &metav1.LabelSelector{}
			existing.Spec.MatchConstraints.ObjectSelector = &metav1.LabelSelector{}
			existing.Spec.MatchConstraints.ResourceRules[0].Scope = new(admissionregistrationv1.AllScopes)
			cl := commontestutils.InitClient([]client.Object{hco, existing})

			res := NewValidatingAdmissionPolicyHandler(cl, commontestutils.GetScheme(), newTestVAP()).Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Updated).To(BeFalse())
		})

		It("should update a modified spec", func() {
			existing := newTestVAP()
			existing.Spec.Validations[0].Expression = "false"
			cl := commontestutils.InitClient([]client.Object{hco, existing})

			req.HCOTriggered = false
			res := NewValidatingAdmissionPolicyHandler(cl, commontestutils.GetScheme(), newTestVAP()).Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Updated).To(BeTrue())
			Expect(res.Overwritten).To(BeTrue())

			found := &admissionregistrationv1.ValidatingAdmissionPolicy{}
			Expect(cl.Get(context.Background(), client.ObjectKeyFromObject(existing), found)).To(Succeed())
			Expect(found.Spec.Validations[0].Expression).To(Equal("true"))
		})
	})

	Context("ValidatingAdmissionPolicy external changes", func() {
		It("should revert added matchConditions", func() {
			existing := newTestVAP()
			existing.Spec.MatchConditions = []admissionregistrationv1.MatchCondition{{Name: "skip", Expression: "false"}}
			cl := commontestutils.InitClient([]client.Object{hco, existing})

			res := NewValidatingAdmissionPolicyHandler(cl, commontestutils.GetScheme(), newTestVAP()).Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Updated).To(BeTrue())

			found := &admissionregistrationv1.ValidatingAdmissionPolicy{}
			Expect(cl.Get(context.Background(), client.ObjectKeyFromObject(existing), found)).To(Succeed())
			Expect(found.Spec.MatchConditions).To(BeEmpty())
		})

		It("should revert an added validation", func() {
			existing := newTestVAP()
			existing.Spec.Validations = append(existing.Spec.Validations, admissionregistrationv1.Validation{Expression: "false"})
			cl := commontestutils.InitClient([]client.Object{hco, existing})

			res := NewValidatingAdmissionPolicyHandler(cl, commontestutils.GetScheme(), newTestVAP()).Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Updated).To(BeTrue())
		})
	})

	Context("ValidatingAdmissionPolicyBinding", func() {
		It("should create the binding", func() {
			cl := commontestutils.InitClient([]client.Object{hco})
			res := NewValidatingAdmissionPolicyBindingHandler(cl, commontestutils.GetScheme(), newTestVAPB()).Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Created).To(BeTrue())
		})

		It("should not update for fields the apiserver defaults", func() {
			required := newTestVAPB()
			required.Spec.MatchResources = &admissionregistrationv1.MatchResources{}
			existing := newTestVAPB()
			existing.Spec.MatchResources = &admissionregistrationv1.MatchResources{
				MatchPolicy:       new(admissionregistrationv1.Equivalent),
				NamespaceSelector: &metav1.LabelSelector{},
				ObjectSelector:    &metav1.LabelSelector{},
			}
			cl := commontestutils.InitClient([]client.Object{hco, existing})

			res := NewValidatingAdmissionPolicyBindingHandler(cl, commontestutils.GetScheme(), required).Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Updated).To(BeFalse())
		})

		It("should revert added matchResources", func() {
			existing := newTestVAPB()
			existing.Spec.MatchResources = &admissionregistrationv1.MatchResources{
				NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"skip": "true"}},
			}
			cl := commontestutils.InitClient([]client.Object{hco, existing})

			res := NewValidatingAdmissionPolicyBindingHandler(cl, commontestutils.GetScheme(), newTestVAPB()).Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Updated).To(BeTrue())

			found := &admissionregistrationv1.ValidatingAdmissionPolicyBinding{}
			Expect(cl.Get(context.Background(), client.ObjectKeyFromObject(existing), found)).To(Succeed())
			Expect(found.Spec.MatchResources).To(BeNil())
		})

		It("should update a modified spec", func() {
			existing := newTestVAPB()
			existing.Spec.PolicyName = "other"
			cl := commontestutils.InitClient([]client.Object{hco, existing})

			res := NewValidatingAdmissionPolicyBindingHandler(cl, commontestutils.GetScheme(), newTestVAPB()).Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Updated).To(BeTrue())

			found := &admissionregistrationv1.ValidatingAdmissionPolicyBinding{}
			Expect(cl.Get(context.Background(), client.ObjectKeyFromObject(existing), found)).To(Succeed())
			Expect(found.Spec.PolicyName).To(Equal("test-policy"))
		})
	})
})
