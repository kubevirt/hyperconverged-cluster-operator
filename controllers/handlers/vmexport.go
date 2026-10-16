package handlers

import (
	"errors"
	"fmt"
	"reflect"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

const (
	vmExportOpenDefaultPortsPolicyName  = "kubevirt-vmexport-open-default-ports"
	vmExportOpenDefaultPortsBindingName = vmExportOpenDefaultPortsPolicyName + "-binding"
	vmExportOpenDefaultPortsAnnotation  = "k8s.ovn.org/open-default-ports"
	vmExportOpenDefaultPortsValue       = `[{"protocol":"tcp","port":8443}]`
	primaryUserDefinedNetworkLabel      = "k8s.ovn.org/primary-user-defined-network"
)

func vmExportOpenDefaultPortsMatchCondition() string {
	return fmt.Sprintf(`!has(object.metadata.annotations) || !(%q in object.metadata.annotations)`, vmExportOpenDefaultPortsAnnotation)
}

func vmExportOpenDefaultPortsMutation() string {
	return fmt.Sprintf(`has(object.metadata.annotations) ?
  %s :
  %s`, vmExportOpenDefaultPortsExistingAnnotationsPatch(), vmExportOpenDefaultPortsMissingAnnotationsPatch())
}

func vmExportOpenDefaultPortsExistingAnnotationsPatch() string {
	return fmt.Sprintf(`[JSONPatch{
    op: "add",
    path: "/metadata/annotations/" + jsonpatch.escapeKey(%q),
    value: %q
  }]`, vmExportOpenDefaultPortsAnnotation, vmExportOpenDefaultPortsValue)
}

func vmExportOpenDefaultPortsMissingAnnotationsPatch() string {
	return fmt.Sprintf(`[JSONPatch{
    op: "add",
    path: "/metadata/annotations",
    value: {%q: %q}
  }]`, vmExportOpenDefaultPortsAnnotation, vmExportOpenDefaultPortsValue)
}

// NewVMExportOpenDefaultPortsPolicyHandler returns the HCO operand that
// installs the OpenShift MutatingAdmissionPolicy for VMExport resources.
func NewVMExportOpenDefaultPortsPolicyHandler(cli client.Client, scheme *runtime.Scheme) operands.Operand {
	return operands.NewGenericOperand(cli, scheme, "MutatingAdmissionPolicy", &vmExportOpenDefaultPortsPolicyHooks{}, false)
}

// NewVMExportOpenDefaultPortsBindingHandler returns the HCO operand that
// binds the VMExport policy to primary User Defined Network namespaces.
func NewVMExportOpenDefaultPortsBindingHandler(cli client.Client, scheme *runtime.Scheme) operands.Operand {
	return operands.NewGenericOperand(cli, scheme, "MutatingAdmissionPolicyBinding", &vmExportOpenDefaultPortsBindingHooks{}, false)
}

func newVMExportOpenDefaultPortsPolicy() *admissionregistrationv1.MutatingAdmissionPolicy {
	failurePolicy := admissionregistrationv1.Fail
	matchPolicy := admissionregistrationv1.Equivalent
	scope := admissionregistrationv1.NamespacedScope

	return &admissionregistrationv1.MutatingAdmissionPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: admissionregistrationv1.SchemeGroupVersion.String(),
			Kind:       "MutatingAdmissionPolicy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   vmExportOpenDefaultPortsPolicyName,
			Labels: operands.GetLabels(hcoutil.AppComponentDeployment),
		},
		Spec: admissionregistrationv1.MutatingAdmissionPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				MatchPolicy:       &matchPolicy,
				NamespaceSelector: &metav1.LabelSelector{},
				ObjectSelector:    &metav1.LabelSelector{},
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
					{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{"export.kubevirt.io"},
								APIVersions: []string{"v1", "v1beta1"},
								Resources:   []string{"virtualmachineexports"},
								Scope:       &scope,
							},
						},
					},
				},
			},
			MatchConditions: []admissionregistrationv1.MatchCondition{
				{
					Name:       "vmexport-open-default-ports-absent",
					Expression: vmExportOpenDefaultPortsMatchCondition(),
				},
			},
			Mutations: []admissionregistrationv1.Mutation{
				{
					PatchType: admissionregistrationv1.PatchTypeJSONPatch,
					JSONPatch: &admissionregistrationv1.JSONPatch{
						Expression: vmExportOpenDefaultPortsMutation(),
					},
				},
			},
			FailurePolicy:      &failurePolicy,
			ReinvocationPolicy: admissionregistrationv1.NeverReinvocationPolicy,
		},
	}
}

func newVMExportOpenDefaultPortsBinding() *admissionregistrationv1.MutatingAdmissionPolicyBinding {
	matchPolicy := admissionregistrationv1.Equivalent

	return &admissionregistrationv1.MutatingAdmissionPolicyBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: admissionregistrationv1.SchemeGroupVersion.String(),
			Kind:       "MutatingAdmissionPolicyBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   vmExportOpenDefaultPortsBindingName,
			Labels: operands.GetLabels(hcoutil.AppComponentDeployment),
		},
		Spec: admissionregistrationv1.MutatingAdmissionPolicyBindingSpec{
			PolicyName: vmExportOpenDefaultPortsPolicyName,
			MatchResources: &admissionregistrationv1.MatchResources{
				MatchPolicy:    &matchPolicy,
				ObjectSelector: &metav1.LabelSelector{},
				NamespaceSelector: &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{
							Key:      primaryUserDefinedNetworkLabel,
							Operator: metav1.LabelSelectorOpExists,
						},
					},
				},
			},
		},
	}
}

type vmExportOpenDefaultPortsPolicyHooks struct{}

func (*vmExportOpenDefaultPortsPolicyHooks) GetFullCr(_ *hcov1.HyperConverged) (client.Object, error) {
	return newVMExportOpenDefaultPortsPolicy(), nil
}

func (*vmExportOpenDefaultPortsPolicyHooks) GetEmptyCr() client.Object {
	return &admissionregistrationv1.MutatingAdmissionPolicy{}
}

func (*vmExportOpenDefaultPortsPolicyHooks) UpdateCR(req *common.HcoRequest, cli client.Client, exists runtime.Object, required runtime.Object) (bool, bool, error) {
	policy, ok := required.(*admissionregistrationv1.MutatingAdmissionPolicy)
	if !ok {
		return false, false, errors.New("can't convert required object to MutatingAdmissionPolicy")
	}
	found, ok := exists.(*admissionregistrationv1.MutatingAdmissionPolicy)
	if !ok {
		return false, false, errors.New("can't convert existing object to MutatingAdmissionPolicy")
	}

	if reflect.DeepEqual(policy.Spec, found.Spec) && hcoutil.CompareLabels(policy, found) {
		return false, false, nil
	}

	policy.Spec.DeepCopyInto(&found.Spec)
	hcoutil.MergeLabels(&policy.ObjectMeta, &found.ObjectMeta)
	if err := cli.Update(req.Ctx, found); err != nil {
		return false, false, err
	}
	return true, !req.HCOTriggered, nil
}

type vmExportOpenDefaultPortsBindingHooks struct{}

func (*vmExportOpenDefaultPortsBindingHooks) GetFullCr(_ *hcov1.HyperConverged) (client.Object, error) {
	return newVMExportOpenDefaultPortsBinding(), nil
}

func (*vmExportOpenDefaultPortsBindingHooks) GetEmptyCr() client.Object {
	return &admissionregistrationv1.MutatingAdmissionPolicyBinding{}
}

func (*vmExportOpenDefaultPortsBindingHooks) UpdateCR(req *common.HcoRequest, cli client.Client, exists runtime.Object, required runtime.Object) (bool, bool, error) {
	binding, ok := required.(*admissionregistrationv1.MutatingAdmissionPolicyBinding)
	if !ok {
		return false, false, errors.New("can't convert required object to MutatingAdmissionPolicyBinding")
	}
	found, ok := exists.(*admissionregistrationv1.MutatingAdmissionPolicyBinding)
	if !ok {
		return false, false, errors.New("can't convert existing object to MutatingAdmissionPolicyBinding")
	}

	if reflect.DeepEqual(binding.Spec, found.Spec) && hcoutil.CompareLabels(binding, found) {
		return false, false, nil
	}

	binding.Spec.DeepCopyInto(&found.Spec)
	hcoutil.MergeLabels(&binding.ObjectMeta, &found.ObjectMeta)
	if err := cli.Update(req.Ctx, found); err != nil {
		return false, false, err
	}
	return true, !req.HCOTriggered, nil
}
