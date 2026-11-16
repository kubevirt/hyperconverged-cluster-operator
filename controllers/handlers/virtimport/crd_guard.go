package virtimport

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

// crdHandler deploys a virt-import CRD like a ConditionalHandler. When the feature gate is off,
// it does not delete a CRD that is already being deleted again, which would emit a removal event
// on every reconcile until the VirtualMachineImports are gone.
type crdHandler struct {
	*operands.ConditionalHandler
	cli  client.Client
	name string
}

func newCRDHandler(cli client.Client, scheme *runtime.Scheme, crd *apiextensionsv1.CustomResourceDefinition) *crdHandler {
	return &crdHandler{
		ConditionalHandler: operands.NewConditionalHandler(operands.NewCRDHandler(cli, scheme, crd), shouldDeploy, getObject(crd)),
		cli:                cli,
		name:               crd.Name,
	}
}

func (h *crdHandler) Ensure(req *common.HcoRequest) *operands.EnsureResult {
	if !shouldDeploy(req.Instance) {
		crd := &apiextensionsv1.CustomResourceDefinition{}
		err := h.cli.Get(req.Ctx, client.ObjectKey{Name: h.name}, crd)
		if err != nil && !apierrors.IsNotFound(err) {
			return &operands.EnsureResult{Err: err, Type: "CustomResourceDefinition", Name: h.name}
		}
		if err == nil && crd.DeletionTimestamp != nil {
			return &operands.EnsureResult{Type: "CustomResourceDefinition", Name: h.name, UpgradeDone: true}
		}
	}

	return h.ConditionalHandler.Ensure(req)
}

// crdGuardedHandler deploys its object like a ConditionalHandler. When the feature gate is off,
// it keeps the object until the virt-import CRDs are gone. This keeps the controller running
// while the CRDs terminate, so it can remove the finalizers of the VirtualMachineImports.
type crdGuardedHandler struct {
	*operands.ConditionalHandler
	cli      client.Client
	crdNames []string
	name     string
	kind     string
}

func newCRDGuardedHandler(cli client.Client, operand *operands.GenericOperand, obj client.Object, crdNames []string) *crdGuardedHandler {
	return &crdGuardedHandler{
		ConditionalHandler: operands.NewConditionalHandler(operand, shouldDeploy, getObject(obj)),
		cli:                cli,
		crdNames:           crdNames,
		name:               obj.GetName(),
		kind:               obj.GetObjectKind().GroupVersionKind().Kind,
	}
}

func (h *crdGuardedHandler) Ensure(req *common.HcoRequest) *operands.EnsureResult {
	if !shouldDeploy(req.Instance) {
		waiting, err := anyCRDBeingRemoved(req, h.cli, h.crdNames)
		if err != nil {
			return &operands.EnsureResult{Err: err, Type: h.kind, Name: h.name}
		}
		if waiting {
			req.Logger.V(1).Info("Waiting for the virt-import CRDs to be removed before removing the object", "kind", h.kind, "name", h.name)
			return &operands.EnsureResult{Type: h.kind, Name: h.name, UpgradeDone: true}
		}
	}

	return h.ConditionalHandler.Ensure(req)
}

// anyCRDBeingRemoved reports whether one of the CRDs still exists and is or will be removed by
// HCO. A CRD HCO does not manage is never removed, so there is nothing to wait for.
func anyCRDBeingRemoved(req *common.HcoRequest, cli client.Client, names []string) (bool, error) {
	for _, name := range names {
		crd := &apiextensionsv1.CustomResourceDefinition{}
		err := cli.Get(req.Ctx, client.ObjectKey{Name: name}, crd)
		if apierrors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return false, err
		}
		if crd.DeletionTimestamp != nil || crd.Labels[hcoutil.AppLabel] == hcoutil.HyperConvergedName {
			return true, nil
		}
	}
	return false, nil
}
