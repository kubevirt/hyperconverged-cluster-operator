package admissionpolicy

import (
	"context"
	"fmt"
	"sync"

	admissionv1 "k8s.io/api/admissionregistration/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

	policyOnce  = &sync.Once{}
	bindingOnce = &sync.Once{}

	policyPredicate = predicate.NewTypedPredicateFuncs[*admissionv1.ValidatingAdmissionPolicy](func(policy *admissionv1.ValidatingAdmissionPolicy) bool {
		return policy.Name == policyName
	})

	bindingPredicate = predicate.NewTypedPredicateFuncs[*admissionv1.ValidatingAdmissionPolicyBinding](func(binding *admissionv1.ValidatingAdmissionPolicyBinding) bool {
		return binding.Name == policyBindingName
	})
)

func getRequiredPolicy(owner *metav1.OwnerReference) *admissionv1.ValidatingAdmissionPolicy {
	policyOnce.Do(func() {
		namespace := hcoutil.GetOperatorNamespaceFromEnv()
		requiredPolicy = &admissionv1.ValidatingAdmissionPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:            policyName,
				Labels:          hcoutil.GetLabels(hcov1beta1.HyperConvergedName, hcoutil.AppComponentDeployment),
				OwnerReferences: []metav1.OwnerReference{*owner},
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
									APIGroups:   []string{hcov1beta1.APIVersionGroup},
									APIVersions: []string{hcov1beta1.APIVersionBeta},
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
	})

	return requiredPolicy.DeepCopy()
}

func getRequiredBinding(owner *metav1.OwnerReference) *admissionv1.ValidatingAdmissionPolicyBinding {
	bindingOnce.Do(func() {
		requiredBinding = &admissionv1.ValidatingAdmissionPolicyBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:            policyBindingName,
				Labels:          hcoutil.GetLabels(hcov1beta1.HyperConvergedName, hcoutil.AppComponentDeployment),
				OwnerReferences: []metav1.OwnerReference{*owner},
			},
			Spec: admissionv1.ValidatingAdmissionPolicyBindingSpec{
				PolicyName:        policyName,
				ValidationActions: []admissionv1.ValidationAction{admissionv1.Deny},
			},
		}
	})

	return requiredBinding.DeepCopy()
}

const crdName = "hyperconvergeds.hco.kubevirt.io"

func getCRDRef(ctx context.Context, cl client.Reader) (*metav1.OwnerReference, error) {
	crd := &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: crdName,
		},
	}

	err := cl.Get(ctx, client.ObjectKeyFromObject(crd), crd)
	if err != nil {
		return nil, err
	}
	gvk := schema.GroupVersionKind{
		Group:   apiextensionsv1.SchemeGroupVersion.Group,
		Version: apiextensionsv1.SchemeGroupVersion.Version,
		Kind:    "CustomResourceDefinition",
	}
	return metav1.NewControllerRef(crd, gvk), nil
}
