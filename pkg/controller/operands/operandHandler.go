package operands

import (
    "fmt"
    hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/pkg/apis/hco/v1beta1"
    "github.com/kubevirt/hyperconverged-cluster-operator/pkg/controller/common"
    hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
    conditionsv1 "github.com/openshift/custom-resource-status/conditions/v1"
    corev1 "k8s.io/api/core/v1"
    "k8s.io/apimachinery/pkg/runtime"
    "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
    reconcileFailed = "ReconcileFailed"
)

type OperandHandler struct {
    operands     []Operand
    eventEmitter hcoutil.EventEmitter
}

func NewOperandHandler(client client.Client, scheme *runtime.Scheme, isOpenshiftCluster bool, eventEmitter hcoutil.EventEmitter) *OperandHandler {
    operands := []Operand{
        &kvConfigHandler{Client: client, Scheme: scheme},
        &kvPriorityClassHandler{Client: client, Scheme: scheme},
        &kubevirtHandler{Client: client, Scheme: scheme},
        &cdiHandler{Client: client, Scheme: scheme},
        &cnaHandler{Client: client, Scheme: scheme},
        &vmImportHandler{Client: client, Scheme: scheme},
        &imsConfigHandler{Client: client, Scheme: scheme},
    }

    if isOpenshiftCluster {
        operands = append(operands, []Operand{
            newCommonTemplateBundleHandler(client, scheme),
            newNodeLabellerBundleHandler(client, scheme),
            newTemplateValidatorHandler(client, scheme),
            newMetricsAggregationHandler(client, scheme),
        }...)
    }

    return &OperandHandler{
        operands: operands,
        eventEmitter: eventEmitter,
    }
}

func (h OperandHandler) Ensure(req *common.HcoRequest) error {
    for _, handler := range h.operands {
        res := handler.Ensure(req)
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
            h.eventEmitter.EmitEvent(req.Instance, corev1.EventTypeNormal, "Updated", fmt.Sprintf("Updated %s %s", res.Type, res.Name))
        }

        req.ComponentUpgradeInProgress = req.ComponentUpgradeInProgress && res.UpgradeDone
    }
    return nil

}
