package operandhandler

import (
	"context"
	"fmt"
	"io/fs"
	"time"

	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/handlers"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/handlers/passt"
	waspagent "github.com/kubevirt/hyperconverged-cluster-operator/controllers/handlers/wasp-agent"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/monitoring/hyperconverged/metrics"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

const (
	reconcileFailed       = "ReconcileFailed"
	ErrCDIUninstall       = "ErrCDIUninstall"
	uninstallCDIErrorMsg  = "The uninstall request failed on CDI component: "
	ErrVirtUninstall      = "ErrVirtUninstall"
	uninstallVirtErrorMsg = "The uninstall request failed on virt component: "
	ErrHCOUninstall       = "ErrHCOUninstall"
	uninstallHCOErrorMsg  = "The uninstall request failed on dependent components, please check their logs."
	deleteTimeOut         = 30 * time.Second
)

var (
	logger = logf.Log.WithName("operandHandlerInit")
)

type OperandHandler struct {
	client   client.Client
	operands []operands.Operand
	// save for deletions
	objects      []client.Object
	eventEmitter hcoutil.EventEmitter
}

func NewOperandHandler(client client.Client, scheme *runtime.Scheme, ci hcoutil.ClusterInfo, eventEmitter hcoutil.EventEmitter) *OperandHandler {
	operandList := []operands.Operand{
		handlers.NewKvPriorityClassHandler(client, scheme),
		handlers.NewKubevirtHandler(client, scheme),
		handlers.NewCdiHandler(client, scheme),
		handlers.NewCnaHandler(client, scheme),
		handlers.NewAAQHandler(client, scheme),
		passt.NewPasstDaemonSetHandler(client, scheme),
		passt.NewPasstNetworkAttachmentDefinitionHandler(client, scheme),
	}

	if ci.IsOpenshift() {
		operandList = append(operandList, []operands.Operand{
			handlers.NewSspHandler(client, scheme),
			handlers.NewCliDownloadHandler(client, scheme),
			handlers.NewCliDownloadsRouteHandler(client, scheme),
			operands.NewServiceHandler(client, scheme, handlers.NewCliDownloadsService),
			passt.NewPasstServiceAccountHandler(client, scheme),
			passt.NewPasstSecurityContextConstraintsHandler(client, scheme),
			waspagent.NewWaspAgentServiceAccountHandler(client, scheme),
			waspagent.NewWaspAgentSCCHandler(client, scheme),
			waspagent.NewWaspAgentDaemonSetHandler(client, scheme),
		}...)
	}

	if ci.IsOpenshift() && ci.IsConsolePluginImageProvided() {
		operandList = append(operandList, handlers.NewConsoleHandler(client))
		operandList = append(operandList, operands.NewServiceHandler(client, scheme, handlers.NewKvUIPluginSvc))
		operandList = append(operandList, operands.NewServiceHandler(client, scheme, handlers.NewKvUIProxySvc))
	}

	if ci.IsManagedByOLM() {
		operandList = append(operandList, handlers.NewCsvHandler(client, ci))
	}

	return &OperandHandler{
		client:       client,
		operands:     operandList,
		eventEmitter: eventEmitter,
	}
}

// FirstUseInitiation is a lazy init function
// The k8s client is not available when calling to NewOperandHandler.
// Initial operations that need to read/write from the cluster can only be done when the client is already working.
func (h *OperandHandler) FirstUseInitiation(scheme *runtime.Scheme, ci hcoutil.ClusterInfo, hc *hcov1beta1.HyperConverged, pwdFS fs.FS) {
	h.objects = make([]client.Object, 0)
	if !ci.IsOpenshift() {
		return
	}

	for _, fn := range []operands.GetHandlers{
		handlers.GetQuickStartHandlers,
		handlers.GetDashboardHandlers,
		handlers.GetImageStreamHandlers,
	} {
		h.addOperands(scheme, hc, fn, pwdFS)
	}

	getHandlerFuncs := []operands.GetHandler{
		handlers.NewVirtioWinCmHandler,
		handlers.NewVirtioWinCmReaderRoleHandler,
		handlers.NewVirtioWinCmReaderRoleBindingHandler,
		waspagent.NewWaspAgentClusterRoleHandler,
		waspagent.NewWaspAgentClusterRoleBindingHandler,
	}

	if ci.IsConsolePluginImageProvided() {
		getHandlerFuncs = append(getHandlerFuncs,
			handlers.NewKvUIPluginDeploymentHandler,
			handlers.NewKvUIProxyDeploymentHandler,
			handlers.NewKvUINginxCMHandler,
			handlers.NewKvUIPluginCRHandler,
			handlers.NewKvUIUserSettingsCMHandler,
			handlers.NewKvUIFeaturesCMHandler,
			handlers.NewKvUIConfigReaderRoleHandler,
			handlers.NewKvUIConfigReaderRoleBindingHandler,
			handlers.NewKVConsolePluginNetworkPolicyHandler,
			handlers.NewKVAPIServerProxyNetworkPolicyHandler,
		)
	}

	for _, fn := range getHandlerFuncs {
		h.addOperand(scheme, hc, fn)
	}
}

func (h *OperandHandler) GetQuickStartNames() []string {
	return handlers.GetQuickStartNames()
}

func (h *OperandHandler) GetImageStreamNames() []string {
	return handlers.GetImageStreamNames()
}

func (h *OperandHandler) addOperandObject(handler operands.Operand, hc *hcov1beta1.HyperConverged) {
	var (
		obj client.Object
		err error
	)

	if gh, ok := handler.(operands.CRGetter); ok {
		obj, err = gh.GetFullCr(hc)
	} else {
		err = fmt.Errorf("unknown handler with type %T", handler)
	}

	if err != nil {
		logger.Error(err, "can't create object")
	} else {
		h.objects = append(h.objects, obj)
	}
}

func (h *OperandHandler) addOperands(scheme *runtime.Scheme, hc *hcov1beta1.HyperConverged, getHandlers operands.GetHandlers, dir fs.FS) {
	handlers, err := getHandlers(logger, h.client, scheme, hc, dir)
	if err != nil {
		logger.Error(err, "can't create handler")
	} else if len(handlers) > 0 {
		for _, handler := range handlers {
			h.addOperandObject(handler, hc)
		}
		h.operands = append(h.operands, handlers...)
	}
}

func (h *OperandHandler) addOperand(scheme *runtime.Scheme, hc *hcov1beta1.HyperConverged, getHandler operands.GetHandler) {
	handler, err := getHandler(logger, h.client, scheme, hc)
	if err != nil {
		logger.Error(err, "can't create handler")
		return
	}

	h.addOperandObject(handler, hc)

	h.operands = append(h.operands, handler)
}

func (h *OperandHandler) Ensure(req *common.HcoRequest) error {
	for _, handler := range h.operands {
		res := handler.Ensure(req)
		if res.Err != nil {
			req.Logger.Error(res.Err, "failed to Ensure an operand")

			req.ComponentUpgradeInProgress = false
			req.Conditions.SetStatusCondition(metav1.Condition{
				Type:               hcov1beta1.ConditionReconcileComplete,
				Status:             metav1.ConditionFalse,
				Reason:             reconcileFailed,
				Message:            fmt.Sprintf("Error while reconciling: %v", res.Err),
				ObservedGeneration: req.Instance.Generation,
			})
			return res.Err
		}

		if res.Created {
			h.eventEmitter.EmitEvent(req.Instance, corev1.EventTypeNormal, "Created", fmt.Sprintf("Created %s %s", res.Type, res.Name))
		} else if res.Updated {
			h.handleUpdatedOperand(req, res)
		} else if res.Deleted {
			h.eventEmitter.EmitEvent(req.Instance, corev1.EventTypeNormal, "Killing", fmt.Sprintf("Removed %s %s", res.Type, res.Name))
		}

		req.ComponentUpgradeInProgress = req.ComponentUpgradeInProgress && res.UpgradeDone
	}
	return nil

}

func (h *OperandHandler) handleUpdatedOperand(req *common.HcoRequest, res *operands.EnsureResult) {
	if !res.Overwritten {
		h.eventEmitter.EmitEvent(req.Instance, corev1.EventTypeNormal, "Updated", fmt.Sprintf("Updated %s %s", res.Type, res.Name))
	} else {
		h.eventEmitter.EmitEvent(req.Instance, corev1.EventTypeWarning, "Overwritten", fmt.Sprintf("Overwritten %s %s", res.Type, res.Name))
		if !req.UpgradeMode {
			metrics.IncOverwrittenModifications(res.Type, res.Name)
		}
	}
}

func (h *OperandHandler) EnsureDeleted(req *common.HcoRequest) error {

	tCtx, cancel := context.WithTimeout(req.Ctx, deleteTimeOut)
	defer cancel()

	resources := []client.Object{
		handlers.NewNetworkAddonsWithNameOnly(req.Instance),
		handlers.NewSSPWithNameOnly(req.Instance),
		handlers.NewConsoleCLIDownload(req.Instance),
		handlers.NewAAQWithNameOnly(req.Instance),
		passt.NewPasstBindingCNINetworkAttachmentDefinition(req.Instance),
		passt.NewPasstBindingCNISecurityContextConstraints(req.Instance),
		waspagent.NewWaspAgentSCCWithNameOnly(req.Instance),
	}

	resources = append(resources, h.objects...)

	err := h.deleteMultipleResources(tCtx, req, resources)
	if err != nil {
		return err
	}

	err = h.deleteSingleResource(tCtx, req, handlers.NewKubeVirtWithNameOnly(req.Instance), ErrVirtUninstall, uninstallVirtErrorMsg)
	if err != nil {
		return err
	}

	return h.deleteSingleResource(tCtx, req, handlers.NewCDIWithNameOnly(req.Instance), ErrCDIUninstall, uninstallCDIErrorMsg)
}

func (h *OperandHandler) deleteMultipleResources(tCtx context.Context, req *common.HcoRequest, resources []client.Object) error {
	eg, egCtx := errgroup.WithContext(tCtx)

	for _, res := range resources {
		func(o client.Object) {
			eg.Go(func() error {
				deleted, err := hcoutil.EnsureDeleted(egCtx, h.client, o, req.Instance.Name, req.Logger, false, true, true)
				if err != nil {
					req.Logger.Error(err, "Failed to manually delete objects")
					h.eventEmitter.EmitEvent(req.Instance, corev1.EventTypeWarning, ErrHCOUninstall, uninstallHCOErrorMsg)
					return err
				} else if deleted {
					h.eventEmitter.EmitEvent(req.Instance, corev1.EventTypeNormal, "Killing", fmt.Sprintf("Removed %s %s", o.GetObjectKind().GroupVersionKind().Kind, o.GetName()))
				}
				return nil
			})
		}(res)
	}

	return eg.Wait()
}

func (h *OperandHandler) deleteSingleResource(ctx context.Context, req *common.HcoRequest, resource client.Object, errT, errMsg string) error {
	deleted, err := hcoutil.EnsureDeleted(ctx, h.client, resource, req.Instance.Name, req.Logger, false, true, true)
	if err != nil {
		req.Logger.Error(err, "Failed to manually delete objects")

		h.eventEmitter.EmitEvent(req.Instance, corev1.EventTypeWarning, errT, errMsg+err.Error())
		return err
	} else if deleted {
		h.eventEmitter.EmitEvent(req.Instance, corev1.EventTypeNormal, "Killing", fmt.Sprintf("Removed %s %s", resource.GetObjectKind().GroupVersionKind().Kind, resource.GetName()))
	}
	return nil
}

func (h *OperandHandler) Reset() {
	for _, op := range h.operands {
		op.Reset()
	}
}
