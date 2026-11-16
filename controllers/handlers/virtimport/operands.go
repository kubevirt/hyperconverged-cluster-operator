package virtimport

import (
	appsv1 "k8s.io/api/apps/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/virtimport"
)

// NewOperands returns the handlers for all objects of the embedded virt-import bundle.
//
// The admission policies come first, so the CRD is never served without them. The CRD handlers
// follow: when the feature gate is on, the CRDs must exist before the controller starts, and when
// it is off, the CRDs are deleted first while the guarded handlers keep everything else, the
// policies included, until they are gone.
func NewOperands(cli client.Client, scheme *runtime.Scheme) ([]operands.Operand, error) {
	b, err := virtimport.GetBundle()
	if err != nil {
		return nil, err
	}
	crds := hcoCRDs(b)

	crdNames := make([]string, 0, len(crds))
	for _, crd := range crds {
		crdNames = append(crdNames, crd.Name)
	}

	result := make([]operands.Operand, 0)
	guarded := func(operand *operands.GenericOperand, obj client.Object) {
		result = append(result, newCRDGuardedHandler(cli, operand, obj, crdNames))
	}

	for _, policy := range b.ValidatingAdmissionPolicies {
		setMetadata(&policy.ObjectMeta, false)
		guarded(operands.NewValidatingAdmissionPolicyHandler(cli, scheme, policy), policy)
	}
	for _, binding := range b.ValidatingAdmissionPolicyBindings {
		setMetadata(&binding.ObjectMeta, false)
		guarded(operands.NewValidatingAdmissionPolicyBindingHandler(cli, scheme, binding), binding)
	}

	for _, crd := range crds {
		result = append(result, newCRDHandler(cli, scheme, crd))
	}

	for _, sa := range b.ServiceAccounts {
		setMetadata(&sa.ObjectMeta, true)
		guarded(operands.NewServiceAccountHandler(cli, scheme, sa.DeepCopy), sa)
	}
	for _, role := range b.ClusterRoles {
		setMetadata(&role.ObjectMeta, false)
		guarded(operands.NewClusterRoleHandler(cli, scheme, role), role)
	}
	for _, binding := range b.ClusterRoleBindings {
		setMetadata(&binding.ObjectMeta, false)
		setSubjectNamespaces(binding.Subjects)
		guarded(operands.NewClusterRoleBindingHandler(cli, scheme, binding), binding)
	}
	for _, role := range b.Roles {
		setMetadata(&role.ObjectMeta, true)
		guarded(operands.NewRoleHandler(cli, scheme, role), role)
	}
	for _, binding := range b.RoleBindings {
		setMetadata(&binding.ObjectMeta, true)
		setSubjectNamespaces(binding.Subjects)
		guarded(operands.NewRoleBindingHandler(cli, scheme, binding), binding)
	}

	np := newNetworkPolicy(b.Deployment)
	guarded(operands.NewNetworkPolicyHandler(cli, scheme, np), np)

	guarded(operands.NewDeploymentHandler(cli, scheme, func(hc *hcov1.HyperConverged) *appsv1.Deployment {
		return newDeployment(b.Deployment, hc)
	}, hcov1.ConditionVirtImportReady), newDeploymentWithNameOnly(b.Deployment))

	return result, nil
}

// CRDs returns the CRDs of the embedded bundle with the HCO labels.
func CRDs() ([]*apiextensionsv1.CustomResourceDefinition, error) {
	b, err := virtimport.GetBundle()
	if err != nil {
		return nil, err
	}
	return hcoCRDs(b), nil
}

func hcoCRDs(b *virtimport.Bundle) []*apiextensionsv1.CustomResourceDefinition {
	for _, crd := range b.CRDs {
		setMetadata(&crd.ObjectMeta, false)
	}
	return b.CRDs
}

// getObject returns a function that returns a copy of obj. ConditionalHandler uses it to delete
// the object. The bundle objects keep their type information, which the deletion needs.
func getObject(obj client.Object) operands.GetCRWithNameFunc {
	return func(_ *hcov1.HyperConverged) client.Object {
		return obj.DeepCopyObject().(client.Object)
	}
}
