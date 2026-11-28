package virtimport

import (
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/virtimport"
)

// NewOperands returns the handlers for the objects of the embedded virt-import bundle, which follow
// the feature gate. The CRD is shipped statically. The admission policies come first and go only
// once the controller is gone, so users never get access to the API without the policy.
func NewOperands(cli client.Client, scheme *runtime.Scheme) ([]operands.Operand, error) {
	b, err := virtimport.GetBundle()
	if err != nil {
		return nil, err
	}

	var result []operands.Operand
	gated := func(operand *operands.GenericOperand, obj client.Object) {
		result = append(result, operands.NewConditionalHandler(operand, shouldDeploy, getObject(obj)))
	}

	controller := client.ObjectKey{Name: b.Deployment.Name, Namespace: hcoutil.GetOperatorNamespaceFromEnv()}
	policy := func(operand *operands.GenericOperand, obj client.Object) {
		result = append(result, &policyHandler{
			ConditionalHandler: operands.NewConditionalHandler(operand, shouldDeploy, getObject(obj)),
			cli:                cli,
			controller:         controller,
			kind:               obj.GetObjectKind().GroupVersionKind().Kind,
			name:               obj.GetName(),
		})
	}
	for _, vap := range b.ValidatingAdmissionPolicies {
		setMetadata(&vap.ObjectMeta, false)
		policy(operands.NewValidatingAdmissionPolicyHandler(cli, scheme, vap), vap)
	}
	for _, binding := range b.ValidatingAdmissionPolicyBindings {
		setMetadata(&binding.ObjectMeta, false)
		policy(operands.NewValidatingAdmissionPolicyBindingHandler(cli, scheme, binding), binding)
	}

	for _, sa := range b.ServiceAccounts {
		setMetadata(&sa.ObjectMeta, true)
		gated(operands.NewServiceAccountHandler(cli, scheme, sa.DeepCopy), sa)
	}
	for _, role := range b.ClusterRoles {
		setMetadata(&role.ObjectMeta, false)
		gated(operands.NewClusterRoleHandler(cli, scheme, role), role)
	}
	for _, binding := range b.ClusterRoleBindings {
		setMetadata(&binding.ObjectMeta, false)
		setSubjectNamespaces(binding.Subjects)
		gated(operands.NewClusterRoleBindingHandler(cli, scheme, binding), binding)
	}
	for _, role := range b.Roles {
		setMetadata(&role.ObjectMeta, true)
		gated(operands.NewRoleHandler(cli, scheme, role), role)
	}
	for _, binding := range b.RoleBindings {
		setMetadata(&binding.ObjectMeta, true)
		setSubjectNamespaces(binding.Subjects)
		gated(operands.NewRoleBindingHandler(cli, scheme, binding), binding)
	}

	np := newNetworkPolicy(b.Deployment)
	gated(operands.NewNetworkPolicyHandler(cli, scheme, np), np)
	for _, policy := range b.NetworkPolicies {
		setMetadata(&policy.ObjectMeta, true)
		gated(operands.NewNetworkPolicyHandler(cli, scheme, policy), policy)
	}

	svc := newMetricsService(b)
	gated(operands.NewServiceHandler(cli, scheme, svc), svc)
	ci := hcoutil.GetClusterInfo()
	// On OpenShift Prometheus authenticates with its client certificate, so it needs no token.
	if !ci.IsOpenshift() {
		for _, secret := range b.Secrets {
			setMetadata(&secret.ObjectMeta, true)
			gated(operands.NewSecretHandler(cli, scheme, secret), secret)
		}
		cert := newCertificate(b)
		gated(operands.NewCertificateHandler(cli, scheme, cert), cert)
	}
	if ci.IsMonitoringAvailable() {
		sm := newServiceMonitor(b)
		gated(operands.NewServiceMonitorHandler(cli, scheme, sm), sm)
	}

	deployment := b.Deployment.DeepCopy()
	setMetadata(&deployment.ObjectMeta, true)
	gated(operands.NewDeploymentHandler(cli, scheme, func(hc *hcov1.HyperConverged) *appsv1.Deployment {
		return newDeployment(b.Deployment, hc)
	}, hcov1.ConditionVirtImportReady), deployment)

	return result, nil
}

// getObject returns a function that returns a copy of obj. ConditionalHandler uses it to delete
// the object. The bundle objects keep their type information, which the deletion needs.
func getObject(obj client.Object) operands.GetCRWithNameFunc {
	return func(_ *hcov1.HyperConverged) client.Object {
		return obj.DeepCopyObject().(client.Object)
	}
}

// policyHandler keeps an admission policy while the controller still runs with the feature gate
// off. The policies are removed in a later reconcile, after the controller and the user roles.
type policyHandler struct {
	*operands.ConditionalHandler
	cli        client.Client
	controller client.ObjectKey
	kind, name string
}

func (h *policyHandler) Ensure(req *common.HcoRequest) *operands.EnsureResult {
	if !shouldDeploy(req.Instance) {
		err := h.cli.Get(req.Ctx, h.controller, &appsv1.Deployment{})
		if err == nil {
			return &operands.EnsureResult{Type: h.kind, Name: h.name, UpgradeDone: true}
		}
		if !apierrors.IsNotFound(err) {
			return &operands.EnsureResult{Err: err, Type: h.kind, Name: h.name}
		}
	}
	return h.ConditionalHandler.Ensure(req)
}
