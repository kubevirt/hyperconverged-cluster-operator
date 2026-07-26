package common

import (
	"k8s.io/utils/ptr"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

// ShouldDeployNetworkResourcesInjector checks if network-resources-injector should be deployed
func ShouldDeployNetworkResourcesInjector(hc *hcov1.HyperConverged) bool {
	return ptr.Deref(hc.Spec.Deployment.DeployNetworkResourcesInjector, true)
}

const (
	ReconcileCompleted        = "ReconcileCompleted"
	ReconcileCompletedMessage = "Reconcile completed successfully"

	// JSONPatch annotation names
	JSONPatchKVAnnotationName   = "kubevirt.kubevirt.io/jsonpatch"
	JSONPatchCDIAnnotationName  = "containerizeddataimporter.kubevirt.io/jsonpatch"
	JSONPatchCNAOAnnotationName = "networkaddonsconfigs.kubevirt.io/jsonpatch"
	JSONPatchSSPAnnotationName  = "ssp.kubevirt.io/jsonpatch"
	// Tuning Policy annotation name
	TuningPolicyAnnotationName = util.HCOAnnotationPrefix + "tuningPolicy"
)
