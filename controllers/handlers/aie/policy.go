package aie

import (
	"errors"
	"fmt"
	"reflect"

	admissionv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubevirtcorev1 "kubevirt.io/api/core/v1"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	hcofg "github.com/kubevirt/hyperconverged-cluster-operator/api/v1/featuregates"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

const (
	policyName  = "kubevirt-aie-launcher"
	bindingName = policyName + "-binding"

	// LauncherLabelValue identifies KubeVirt launcher Pods.
	LauncherLabelValue = "virt-launcher"
	// AlternativeLauncherAnnotation records the selected launcher image on a Pod.
	AlternativeLauncherAnnotation = "kubevirt.io/alternative-launcher-image"
)

// NewLauncherPolicyHandler reconciles launcher image selection while AIE is enabled.
func NewLauncherPolicyHandler(cli client.Client, scheme *runtime.Scheme, image string) operands.Operand {
	return operands.NewConditionalHandler(
		operands.NewGenericOperand(cli, scheme, "MutatingAdmissionPolicy", &launcherPolicyHooks{policy: newLauncherPolicy(image)}, false),
		launcherEnabled,
		func(_ *hcov1.HyperConverged) client.Object { return newLauncherPolicyWithNameOnly() },
	)
}

// NewLauncherBindingHandler activates the policy in all workload namespaces.
func NewLauncherBindingHandler(cli client.Client, scheme *runtime.Scheme) operands.Operand {
	return operands.NewConditionalHandler(
		operands.NewGenericOperand(cli, scheme, "MutatingAdmissionPolicyBinding", &launcherBindingHooks{binding: newLauncherBinding()}, false),
		launcherEnabled,
		func(_ *hcov1.HyperConverged) client.Object { return newLauncherBindingWithNameOnly() },
	)
}

func launcherEnabled(hc *hcov1.HyperConverged) bool {
	return hc.Spec.FeatureGates.IsEnabled(hcofg.NVIDIAAIE)
}

// LauncherResources returns the identities needed for cleanup without an image.
func LauncherResources() []client.Object {
	return []client.Object{newLauncherBindingWithNameOnly(), newLauncherPolicyWithNameOnly()}
}

func newLauncherPolicyWithNameOnly() *admissionv1.MutatingAdmissionPolicy {
	return &admissionv1.MutatingAdmissionPolicy{
		TypeMeta:   metav1.TypeMeta{APIVersion: admissionv1.SchemeGroupVersion.String(), Kind: "MutatingAdmissionPolicy"},
		ObjectMeta: metav1.ObjectMeta{Name: policyName, Labels: operands.GetLabels(hcoutil.AppComponentAIE)},
	}
}

func newLauncherBindingWithNameOnly() *admissionv1.MutatingAdmissionPolicyBinding {
	return &admissionv1.MutatingAdmissionPolicyBinding{
		TypeMeta:   metav1.TypeMeta{APIVersion: admissionv1.SchemeGroupVersion.String(), Kind: "MutatingAdmissionPolicyBinding"},
		ObjectMeta: metav1.ObjectMeta{Name: bindingName, Labels: operands.GetLabels(hcoutil.AppComponentAIE)},
	}
}

func newLauncherBinding() *admissionv1.MutatingAdmissionPolicyBinding {
	binding := newLauncherBindingWithNameOnly()
	binding.Spec = admissionv1.MutatingAdmissionPolicyBindingSpec{
		PolicyName: policyName,
		MatchResources: &admissionv1.MatchResources{
			MatchPolicy:       new(admissionv1.Equivalent),
			NamespaceSelector: &metav1.LabelSelector{},
			ObjectSelector:    &metav1.LabelSelector{},
		},
	}
	return binding
}

type launcherPolicyHooks struct {
	policy *admissionv1.MutatingAdmissionPolicy
}

func (h *launcherPolicyHooks) GetFullCr(_ *hcov1.HyperConverged) (client.Object, error) {
	return h.policy.DeepCopy(), nil
}

func newLauncherPolicy(image string) *admissionv1.MutatingAdmissionPolicy {
	policy := newLauncherPolicyWithNameOnly()
	policy.Spec = admissionv1.MutatingAdmissionPolicySpec{
		MatchConstraints: &admissionv1.MatchResources{
			MatchPolicy:       new(admissionv1.Equivalent),
			NamespaceSelector: &metav1.LabelSelector{},
			ObjectSelector:    &metav1.LabelSelector{MatchLabels: map[string]string{kubevirtcorev1.AppLabel: LauncherLabelValue}},
			ResourceRules: []admissionv1.NamedRuleWithOperations{{RuleWithOperations: admissionv1.RuleWithOperations{
				Operations: []admissionv1.OperationType{admissionv1.Create},
				Rule:       admissionv1.Rule{APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"pods"}, Scope: new(admissionv1.NamespacedScope)},
			}}},
		},
		MatchConditions: []admissionv1.MatchCondition{
			{Name: "vmi-owner", Expression: `has(object.metadata.ownerReferences) && object.metadata.ownerReferences.exists(r, r.apiVersion == "kubevirt.io/v1" && r.kind == "VirtualMachineInstance" && has(r.controller) && r.controller)`},
			{Name: "compute-container", Expression: `object.spec.containers.exists(c, c.name == "compute")`},
		},
		Mutations: []admissionv1.Mutation{{
			PatchType: admissionv1.PatchTypeApplyConfiguration,
			ApplyConfiguration: &admissionv1.ApplyConfiguration{Expression: fmt.Sprintf(`Object{
    metadata: Object.metadata{annotations: {%[1]q: %[2]q}},
    spec: Object.spec{
        containers: [Object.spec.containers{name: "compute", image: %[2]q}],
        initContainers: has(object.spec.initContainers) ?
            object.spec.initContainers.filter(c, c.name in ["container-disk-binary", "image-volume-binary", "guest-console-log"])
                .map(c, Object.spec.initContainers{name: c.name, image: %[2]q}) : []
    }
}`, AlternativeLauncherAnnotation, image)},
		}},
		FailurePolicy:      new(admissionv1.Fail),
		ReinvocationPolicy: admissionv1.IfNeededReinvocationPolicy,
	}
	return policy
}

func (*launcherPolicyHooks) GetEmptyCr() client.Object { return &admissionv1.MutatingAdmissionPolicy{} }

func (*launcherPolicyHooks) UpdateCR(req *common.HcoRequest, cli client.Client, exists runtime.Object, required runtime.Object) (bool, bool, error) {
	policy, ok := required.(*admissionv1.MutatingAdmissionPolicy)
	found, foundOK := exists.(*admissionv1.MutatingAdmissionPolicy)
	if !ok || !foundOK {
		return false, false, errors.New("can't convert to MutatingAdmissionPolicy")
	}
	return updateLauncherObject(req, cli, found, policy, reflect.DeepEqual(policy.Spec, found.Spec), func() {
		policy.Spec.DeepCopyInto(&found.Spec)
	})
}

type launcherBindingHooks struct {
	binding *admissionv1.MutatingAdmissionPolicyBinding
}

func (h *launcherBindingHooks) GetFullCr(_ *hcov1.HyperConverged) (client.Object, error) {
	return h.binding.DeepCopy(), nil
}
func (*launcherBindingHooks) GetEmptyCr() client.Object {
	return &admissionv1.MutatingAdmissionPolicyBinding{}
}

func (*launcherBindingHooks) UpdateCR(req *common.HcoRequest, cli client.Client, exists runtime.Object, required runtime.Object) (bool, bool, error) {
	binding, ok := required.(*admissionv1.MutatingAdmissionPolicyBinding)
	found, foundOK := exists.(*admissionv1.MutatingAdmissionPolicyBinding)
	if !ok || !foundOK {
		return false, false, errors.New("can't convert to MutatingAdmissionPolicyBinding")
	}
	return updateLauncherObject(req, cli, found, binding, reflect.DeepEqual(binding.Spec, found.Spec), func() {
		binding.Spec.DeepCopyInto(&found.Spec)
	})
}

// updateLauncherObject preserves unrelated metadata while reconciling the operand spec and managed labels.
func updateLauncherObject(req *common.HcoRequest, cli client.Client, found, required client.Object, specEqual bool, copySpec func()) (bool, bool, error) {
	if specEqual && hcoutil.CompareLabels(required, found) {
		return false, false, nil
	}
	copySpec()
	metadata := metav1.ObjectMeta{Labels: found.GetLabels()}
	hcoutil.MergeLabels(&metav1.ObjectMeta{Labels: required.GetLabels()}, &metadata)
	found.SetLabels(metadata.Labels)
	if err := cli.Update(req.Ctx, found); err != nil {
		return false, false, err
	}
	return true, !req.HCOTriggered, nil
}
