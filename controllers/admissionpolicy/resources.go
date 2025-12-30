package admissionpolicy

import (
	"fmt"
	"sync"

	admissionv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

const (
	policyName        = "hyperconverged-namespace-policy"
	policyBindingName = policyName + "-binding"
)

var (
	requiredPolicy  *admissionv1.ValidatingAdmissionPolicy
	requiredBinding *admissionv1.ValidatingAdmissionPolicyBinding
	owner           *metav1.OwnerReference

	policyOnce  = &sync.Once{}
	bindingOnce = &sync.Once{}
	ownerOnce   = &sync.Once{}

	policyPredicate = predicate.NewTypedPredicateFuncs[*admissionv1.ValidatingAdmissionPolicy](func(policy *admissionv1.ValidatingAdmissionPolicy) bool {
		return policy.Name == policyName && policy.DeletionTimestamp == nil
	})

	bindingPredicate = predicate.NewTypedPredicateFuncs[*admissionv1.ValidatingAdmissionPolicyBinding](func(binding *admissionv1.ValidatingAdmissionPolicyBinding) bool {
		return binding.Name == policyBindingName && binding.DeletionTimestamp == nil
	})
)

func getRequiredPolicy() *admissionv1.ValidatingAdmissionPolicy {
	policyOnce.Do(func() {
		owner = getDeploymentReference()

		namespace := hcoutil.GetOperatorNamespaceFromEnv()
		requiredPolicy = &admissionv1.ValidatingAdmissionPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:   policyName,
				Labels: hcoutil.GetLabels(hcov1beta1.HyperConvergedName, hcoutil.AppComponentDeployment),
			},
			Spec: admissionv1.ValidatingAdmissionPolicySpec{
				FailurePolicy: ptr.To(admissionv1.Fail),
				MatchConstraints: &admissionv1.MatchResources{
					MatchPolicy:       ptr.To(admissionv1.Equivalent),
					NamespaceSelector: &metav1.LabelSelector{},
					ObjectSelector:    &metav1.LabelSelector{},
					ResourceRules: []admissionv1.NamedRuleWithOperations{
						{
							RuleWithOperations: admissionv1.RuleWithOperations{
								Rule: admissionv1.Rule{
									APIGroups:   []string{hcoutil.APIVersionGroup},
									APIVersions: []string{hcoutil.APIVersionBeta},
									Resources:   []string{"hyperconvergeds"},
									Scope:       ptr.To(admissionv1.NamespacedScope),
								},
								Operations: []admissionv1.OperationType{admissionv1.Create},
							},
						},
					},
				},
				Validations: []admissionv1.Validation{
					{
						Expression: fmt.Sprintf(`request.namespace == '%s'`, namespace),
						Message:    fmt.Sprintf(`HyperConverged CR can only be created in the '%s' namespace.`, namespace),
					},
				},
			},
		}

		if owner != nil {
			requiredPolicy.OwnerReferences = []metav1.OwnerReference{*owner}
		}
	})

	return requiredPolicy.DeepCopy()
}

func getRequiredBinding() *admissionv1.ValidatingAdmissionPolicyBinding {
	bindingOnce.Do(func() {
		owner = getDeploymentReference()

		requiredBinding = &admissionv1.ValidatingAdmissionPolicyBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:   policyBindingName,
				Labels: hcoutil.GetLabels(hcov1beta1.HyperConvergedName, hcoutil.AppComponentDeployment),
			},
			Spec: admissionv1.ValidatingAdmissionPolicyBindingSpec{
				PolicyName:        policyName,
				ValidationActions: []admissionv1.ValidationAction{admissionv1.Deny},
			},
		}

		if owner != nil {
			requiredBinding.OwnerReferences = []metav1.OwnerReference{*owner}
		}
	})

	return requiredBinding.DeepCopy()
}

func getDeploymentReference() *metav1.OwnerReference {
	ownerOnce.Do(func() {
		deployment := hcoutil.GetClusterInfo().GetDeployment()

		if deployment != nil {
			owner = &metav1.OwnerReference{
				APIVersion:         appsv1.SchemeGroupVersion.String(),
				Kind:               "Deployment",
				Name:               deployment.GetName(),
				UID:                deployment.GetUID(),
				BlockOwnerDeletion: ptr.To(false),
				Controller:         ptr.To(false),
			}
		}

	})

	return owner
}
