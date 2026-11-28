package operands

import (
	"errors"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

func NewValidatingAdmissionPolicyHandler(cli client.Client, scheme *runtime.Scheme, required *admissionregistrationv1.ValidatingAdmissionPolicy) *GenericOperand {
	required = required.DeepCopy()
	setValidatingAdmissionPolicyDefaults(&required.Spec)
	return NewGenericOperand(cli, scheme, "ValidatingAdmissionPolicy", &vapHooks{required: required}, false)
}

type vapHooks struct {
	required *admissionregistrationv1.ValidatingAdmissionPolicy
}

func (h *vapHooks) GetFullCr(_ *hcov1.HyperConverged) (client.Object, error) {
	return h.required.DeepCopy(), nil
}

func (h *vapHooks) GetEmptyCr() client.Object {
	return &admissionregistrationv1.ValidatingAdmissionPolicy{}
}

func (h *vapHooks) UpdateCR(req *common.HcoRequest, cli client.Client, exists runtime.Object, required runtime.Object) (bool, bool, error) {
	policy, ok1 := required.(*admissionregistrationv1.ValidatingAdmissionPolicy)
	found, ok2 := exists.(*admissionregistrationv1.ValidatingAdmissionPolicy)
	if !ok1 || !ok2 {
		return false, false, errors.New("can't convert to ValidatingAdmissionPolicy")
	}

	if hcoutil.CompareLabels(policy, found) && equality.Semantic.DeepEqual(policy.Spec, found.Spec) {
		return false, false, nil
	}

	logUpdate(req, "ValidatingAdmissionPolicy", found.Name)
	hcoutil.MergeLabels(&policy.ObjectMeta, &found.ObjectMeta)
	policy.Spec.DeepCopyInto(&found.Spec)

	if err := cli.Update(req.Ctx, found); err != nil {
		return false, false, err
	}
	return true, !req.HCOTriggered, nil
}

func NewValidatingAdmissionPolicyBindingHandler(cli client.Client, scheme *runtime.Scheme, required *admissionregistrationv1.ValidatingAdmissionPolicyBinding) *GenericOperand {
	required = required.DeepCopy()
	setValidatingAdmissionPolicyBindingDefaults(&required.Spec)
	return NewGenericOperand(cli, scheme, "ValidatingAdmissionPolicyBinding", &vapbHooks{required: required}, false)
}

type vapbHooks struct {
	required *admissionregistrationv1.ValidatingAdmissionPolicyBinding
}

func (h *vapbHooks) GetFullCr(_ *hcov1.HyperConverged) (client.Object, error) {
	return h.required.DeepCopy(), nil
}

func (h *vapbHooks) GetEmptyCr() client.Object {
	return &admissionregistrationv1.ValidatingAdmissionPolicyBinding{}
}

func (h *vapbHooks) UpdateCR(req *common.HcoRequest, cli client.Client, exists runtime.Object, required runtime.Object) (bool, bool, error) {
	binding, ok1 := required.(*admissionregistrationv1.ValidatingAdmissionPolicyBinding)
	found, ok2 := exists.(*admissionregistrationv1.ValidatingAdmissionPolicyBinding)
	if !ok1 || !ok2 {
		return false, false, errors.New("can't convert to ValidatingAdmissionPolicyBinding")
	}

	if hcoutil.CompareLabels(binding, found) && equality.Semantic.DeepEqual(binding.Spec, found.Spec) {
		return false, false, nil
	}

	logUpdate(req, "ValidatingAdmissionPolicyBinding", found.Name)
	hcoutil.MergeLabels(&binding.ObjectMeta, &found.ObjectMeta)
	binding.Spec.DeepCopyInto(&found.Spec)

	if err := cli.Update(req.Ctx, found); err != nil {
		return false, false, err
	}
	return true, !req.HCOTriggered, nil
}

// The apiserver defaults below are the ones of k8s.io/kubernetes/pkg/apis/admissionregistration/v1,
// which k8s.io/api does not ship. Filling them in on the required object lets the handlers compare
// with DeepEqual, so removed fields and external changes are reverted too.

func setValidatingAdmissionPolicyDefaults(spec *admissionregistrationv1.ValidatingAdmissionPolicySpec) {
	if spec.FailurePolicy == nil {
		spec.FailurePolicy = new(admissionregistrationv1.Fail)
	}
	if spec.MatchConstraints != nil {
		setMatchResourcesDefaults(spec.MatchConstraints)
	}
}

func setValidatingAdmissionPolicyBindingDefaults(spec *admissionregistrationv1.ValidatingAdmissionPolicyBindingSpec) {
	if spec.MatchResources != nil {
		setMatchResourcesDefaults(spec.MatchResources)
	}
}

func setMatchResourcesDefaults(m *admissionregistrationv1.MatchResources) {
	if m.MatchPolicy == nil {
		m.MatchPolicy = new(admissionregistrationv1.Equivalent)
	}
	if m.NamespaceSelector == nil {
		m.NamespaceSelector = &metav1.LabelSelector{}
	}
	if m.ObjectSelector == nil {
		m.ObjectSelector = &metav1.LabelSelector{}
	}
	for i := range m.ResourceRules {
		setRuleDefaults(&m.ResourceRules[i].Rule)
	}
	for i := range m.ExcludeResourceRules {
		setRuleDefaults(&m.ExcludeResourceRules[i].Rule)
	}
}

func setRuleDefaults(r *admissionregistrationv1.Rule) {
	if r.Scope == nil {
		r.Scope = new(admissionregistrationv1.AllScopes)
	}
}
