package operands

import (
	"context"
	"fmt"
	"sync"
	"time"

	consolev1 "github.com/openshift/api/console/v1"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/pkg/apis/hco/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/controller/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/metrics"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
	conditionsv1 "github.com/openshift/custom-resource-status/conditions/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubevirtv1 "kubevirt.io/client-go/api/v1"
	cdiv1beta1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
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
	operands []Operand
	// save for deletions
	quickStartObjects []*consolev1.ConsoleQuickStart
	eventEmitter      hcoutil.EventEmitter
}

func NewOperandHandler(client client.Client, scheme *runtime.Scheme, isOpenshiftCluster bool, eventEmitter hcoutil.EventEmitter) *OperandHandler {
	operands := []Operand{
		(*genericOperand)(newKvConfigHandler(client, scheme)),
		(*genericOperand)(newKvPriorityClassHandler(client, scheme)),
		(*genericOperand)(newKubevirtHandler(client, scheme)),
		(*genericOperand)(newCdiHandler(client, scheme)),
		(*genericOperand)(newStorageConfigHandler(client, scheme)),
		(*genericOperand)(newCnaHandler(client, scheme)),
		(*genericOperand)(newVmImportHandler(client, scheme)),
		(*genericOperand)(newImsConfigHandler(client, scheme)),
	}

	if isOpenshiftCluster {
		operands = append(operands, []Operand{
			newSspHandler(client, scheme),
			(*genericOperand)(newMetricsServiceHandler(client, scheme)),
			(*genericOperand)(newMetricsServiceMonitorHandler(client, scheme)),
			(*genericOperand)(newMonitoringPrometheusRuleHandler(client, scheme)),
		}...)
	}

	return &OperandHandler{
		client:       client,
		operands:     operands,
		eventEmitter: eventEmitter,
	}
}

// The k8s client is not available when calling to NewOperandHandler.
// Initial operations that need to read/write from the cluster can only be done when the client is already working.
func (h *OperandHandler) FirstUseInitiation(scheme *runtime.Scheme, isOpenshiftCluster bool, hc *hcov1beta1.HyperConverged) {
	if isOpenshiftCluster {
		qsHandlers, err := getQuickStartHandlers(logger, h.client, scheme, hc)
		if numQs := len(qsHandlers); numQs > 0 {
			h.quickStartObjects = make([]*consolev1.ConsoleQuickStart, numQs)
			for i, op := range qsHandlers {
				qs, err := op.(*genericOperand).hooks.getFullCr(hc)
				if err != nil {
					logger.Error(err, "can't create ConsoleQuickStarts object")
					continue
				}

				h.quickStartObjects[i] = qs.(*consolev1.ConsoleQuickStart)
			}
		}
		if err != nil {
			logger.Error(err, "can't create ConsoleQuickStarts objects")
		} else if len(qsHandlers) > 0 {
			h.operands = append(h.operands, qsHandlers...)
		}
	}
}

func (h OperandHandler) Ensure(req *common.HcoRequest) error {
	for _, handler := range h.operands {
		res := handler.ensure(req)
		if res.Err != nil {
			req.ComponentUpgradeInProgress = false
			req.Conditions.SetStatusCondition(conditionsv1.Condition{
				Type:    hcov1beta1.ConditionReconcileComplete,
				Status:  corev1.ConditionFalse,
				Reason:  reconcileFailed,
				Message: fmt.Sprintf("Error while reconciling: %v", res.Err),
			})
			return res.Err
		}

		if res.Created {
			h.eventEmitter.EmitEvent(req.Instance, corev1.EventTypeNormal, "Created", fmt.Sprintf("Created %s %s", res.Type, res.Name))
		} else if res.Updated {
			if !res.Overwritten {
				h.eventEmitter.EmitEvent(req.Instance, corev1.EventTypeNormal, "Updated", fmt.Sprintf("Updated %s %s", res.Type, res.Name))
			} else {
				h.eventEmitter.EmitEvent(req.Instance, corev1.EventTypeWarning, "Overwritten", fmt.Sprintf("Overwritten %s %s", res.Type, res.Name))
				metrics.HcoMetrics.IncOverwrittenModifications(res.Name)
			}
		}

		req.ComponentUpgradeInProgress = req.ComponentUpgradeInProgress && res.UpgradeDone
	}
	return nil

}

func (h OperandHandler) EnsureDeleted(req *common.HcoRequest) error {

	tCtx, cancel := context.WithTimeout(req.Ctx, deleteTimeOut)
	defer cancel()

	wg := sync.WaitGroup{}
	errorCh := make(chan error)
	done := make(chan bool)

	resources := []client.Object{
		NewKubeVirtWithNameOnly(req.Instance),
		NewCDIWithNameOnly(req.Instance),
		NewNetworkAddonsWithNameOnly(req.Instance),
		NewSSP(req.Instance),
		NewConsoleCLIDownload(req.Instance),
		NewVMImportForCR(req.Instance),
	}

	for _, qs := range h.quickStartObjects {
		resources = append(resources, qs)
	}

	wg.Add(len(resources))

	go func() {
		wg.Wait()
		close(done)
	}()

	for _, res := range resources {
		go func(o client.Object, wgr *sync.WaitGroup) {
			defer wgr.Done()
			err := hcoutil.EnsureDeleted(tCtx, h.client, o, req.Instance.Name, req.Logger, false, true)
			if err != nil {
				req.Logger.Error(err, "Failed to manually delete objects")
				errT := ErrHCOUninstall
				errMsg := uninstallHCOErrorMsg
				switch o.(type) {
				case *kubevirtv1.KubeVirt:
					errT = ErrVirtUninstall
					errMsg = uninstallVirtErrorMsg + err.Error()
				case *cdiv1beta1.CDI:
					errT = ErrCDIUninstall
					errMsg = uninstallCDIErrorMsg + err.Error()
				}

				h.eventEmitter.EmitEvent(req.Instance, corev1.EventTypeWarning, errT, errMsg)
				errorCh <- err
			} else {
				key := client.ObjectKeyFromObject(o)
				if err := h.client.Get(tCtx, key, o); err == nil {
					h.eventEmitter.EmitEvent(req.Instance, corev1.EventTypeNormal, "Killing", fmt.Sprintf("Removed %s %s", o.GetObjectKind().GroupVersionKind().Kind, key.Name))
				}
			}
		}(res, &wg)
	}

	select {
	case err := <-errorCh:
		return err
	case <-tCtx.Done():
		return tCtx.Err()
	case <-done:
		// just in case close(done) was selected while there is an error,
		// check the error channel again.
		if len(errorCh) != 0 {
			err := <-errorCh
			return err
		}

		return nil
	}

	return nil
}

func (h *OperandHandler) Reset() {
	for _, op := range h.operands {
		op.reset()
	}
}
