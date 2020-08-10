package common

import (
	"context"
	"github.com/go-logr/logr"
	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/pkg/apis/hco/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// hcoRequest - gather data for a specific request
type HcoRequest struct {
	reconcile.Request                                     // inheritance of operator request
	Logger                     logr.Logger                // request logger
	Conditions                 HcoConditions              // in-memory conditions
	Ctx                        context.Context            // context of this request, to be use for any other call
	Instance                   *hcov1beta1.HyperConverged // the current state of the CR, as read from K8s
	ComponentUpgradeInProgress bool                       // if in upgrade mode, accumulate the component upgrade status
	Dirty                      bool                       // is something was changed in the CR
	StatusDirty                bool                       // is something was changed in the CR's Status
	UpgradeMode                bool                       // this is a copy of ReconcileHyperConverged.upgradeMode
}
