package common

const (
	ReconcileCompleted        = "ReconcileCompleted"
	ReconcileCompletedMessage = "Reconcile completed successfully"

	// JSONPatch annotation names
	JSONPatchKVAnnotationName   = "kubevirt.kubevirt.io/jsonpatch"
	JSONPatchCDIAnnotationName  = "containerizeddataimporter.kubevirt.io/jsonpatch"
	JSONPatchCNAOAnnotationName = "networkaddonsconfigs.kubevirt.io/jsonpatch"
	JSONPatchSSPAnnotationName  = "ssp.kubevirt.io/jsonpatch"
	// Tuning Policy annotation name
	TuningPolicyAnnotationName = "hco.kubevirt.io/tuningPolicy"
)
