package mutator

import (
	"gomodules.xyz/jsonpatch/v2"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	goldenimages "github.com/kubevirt/hyperconverged-cluster-operator/controllers/handlers/golden-images"
)

const (
	fgDeprecationMsgStart  = "feature gate "
	fgDeprecationMsgMiddle = " is deprecated; please use the "
	fgDeprecationMsgEnd    = " field instead"

	fgErrorMsgStart           = "the deprecated "
	fgErrorMsgMiddle          = " feature gate, and "
	fgErrorMsgEndCommon       = " field contradict each other; the feature gate must not be set, or should be "
	fgErrorMsgEnd             = fgErrorMsgEndCommon + "equal to the field"
	fgErrorMsgEndReverseLogic = fgErrorMsgEndCommon + "equal to the negative value of the field"

	v1HyperConvergedMdevConfigPath = "/spec/virtualization/mediatedDevicesConfiguration"
	v1MDevEnabledPath              = v1HyperConvergedMdevConfigPath + "/enabled"
	v1MDevEnabledField             = "spec.virtualization.mediatedDevicesConfiguration.enabled"
	disableMDevConfigurationFGName = hcov1beta1.DisableMDevConfigurationFG
	mdevDeprecationMsg             = fgDeprecationMsgStart + disableMDevConfigurationFGName + fgDeprecationMsgMiddle + v1MDevEnabledField + fgDeprecationMsgEnd
	mdevErrorMessage               = fgErrorMsgStart + disableMDevConfigurationFGName + fgErrorMsgMiddle + v1MDevEnabledField + fgErrorMsgEndReverseLogic

	v1HyperConvergedStoragePath  = "/spec/storage"
	v1HyperConvergedPRConfigPath = v1HyperConvergedStoragePath + "/persistentReservationConfiguration"
	v1PRConfigEnabledPath        = v1HyperConvergedPRConfigPath + "/enabled"
	v1PRConfigEnabledField       = "spec.storage.persistentReservationConfiguration.enabled"
	persistentReservationFGName  = "persistentReservation"
	prFGDeprecationMsg           = fgDeprecationMsgStart + persistentReservationFGName + fgDeprecationMsgMiddle + v1PRConfigEnabledField + fgDeprecationMsgEnd
	prErrorMessage               = fgErrorMsgStart + persistentReservationFGName + fgErrorMsgMiddle + v1PRConfigEnabledField + fgErrorMsgEnd

	v1MultiArchEnabledPath    = "/spec/workloadSources/enableMultiArchBootImageImport"
	v1MultiArchEnabledField   = "spec.workloadSources.enableMultiArchBootImageImport"
	multiArchFGName           = goldenimages.EnableMultiArchFeatureGate
	multiArchFGDeprecationMsg = fgDeprecationMsgStart + multiArchFGName + fgDeprecationMsgMiddle + v1MultiArchEnabledField + fgDeprecationMsgEnd
	multiArchErrorMessage     = fgErrorMsgStart + multiArchFGName + fgErrorMsgMiddle + v1MultiArchEnabledField + fgErrorMsgEnd
)

type getFieldValueFunc func(*hcov1.HyperConverged) (enabled, found bool)
type mutateFieldFunc func(hc hcov1.HyperConvergedSpec, fieldVal bool, patches []jsonpatch.JsonPatchOperation) []jsonpatch.JsonPatchOperation

type fieldFGDetailsType struct {
	getFieldValue      getFieldValueFunc
	mutateField        mutateFieldFunc
	fgName             string
	deprecationWarning string
	contraventionError string
	fgShouldEqualField bool
}

var fieldFGDetails = []fieldFGDetailsType{
	{
		getFieldValue:      hcV1MDevEnabledValue,
		mutateField:        mutateMdevEnabled,
		fgName:             disableMDevConfigurationFGName,
		deprecationWarning: mdevDeprecationMsg,
		contraventionError: mdevErrorMessage,
		fgShouldEqualField: false,
	},
	{
		getFieldValue:      hcV1PREnabledValue,
		mutateField:        mutatePREnabled,
		fgName:             persistentReservationFGName,
		deprecationWarning: prFGDeprecationMsg,
		contraventionError: prErrorMessage,
		fgShouldEqualField: true,
	},
	{
		getFieldValue:      hcV1MultiArchEnabledValue,
		mutateField:        mutateMultiArchEnabled,
		fgName:             multiArchFGName,
		deprecationWarning: multiArchFGDeprecationMsg,
		contraventionError: multiArchErrorMessage,
		fgShouldEqualField: true,
	},
}

func hcV1MDevEnabledValue(hc *hcov1.HyperConverged) (enabled bool, found bool) {
	mdc := hc.Spec.Virtualization.MediatedDevicesConfiguration
	if mdc == nil || mdc.Enabled == nil {
		return true, false
	}

	return *mdc.Enabled, true
}

func mutateMdevEnabled(
	spec hcov1.HyperConvergedSpec,
	fieldVal bool,
	patches []jsonpatch.JsonPatchOperation,
) []jsonpatch.JsonPatchOperation {

	mdevConfig := spec.Virtualization.MediatedDevicesConfiguration
	if mdevConfig != nil {
		return append(patches, jsonpatch.JsonPatchOperation{
			Operation: "add",
			Path:      v1MDevEnabledPath,
			Value:     fieldVal,
		})
	}

	return append(patches, jsonpatch.JsonPatchOperation{
		Operation: "add",
		Path:      v1HyperConvergedMdevConfigPath,
		Value:     map[string]any{"enabled": fieldVal},
	})
}

func hcV1PREnabledValue(hc *hcov1.HyperConverged) (enabled bool, found bool) {
	if hc.Spec.Storage == nil ||
		hc.Spec.Storage.PersistentReservationConfiguration == nil ||
		hc.Spec.Storage.PersistentReservationConfiguration.Enabled == nil {
		return false, false
	}

	return *hc.Spec.Storage.PersistentReservationConfiguration.Enabled, true
}

func mutatePREnabled(hc hcov1.HyperConvergedSpec, fieldVal bool, patches []jsonpatch.JsonPatchOperation) []jsonpatch.JsonPatchOperation {
	storage := hc.Storage
	if storage != nil && storage.PersistentReservationConfiguration != nil {
		return append(patches, jsonpatch.JsonPatchOperation{
			Operation: "add",
			Path:      v1PRConfigEnabledPath,
			Value:     fieldVal,
		})
	}

	if storage != nil {
		return append(patches, jsonpatch.JsonPatchOperation{
			Operation: "add",
			Path:      v1HyperConvergedPRConfigPath,
			Value:     map[string]any{"enabled": fieldVal},
		})
	}

	return append(patches, jsonpatch.JsonPatchOperation{
		Operation: "add",
		Path:      v1HyperConvergedStoragePath,
		Value:     map[string]any{"persistentReservationConfiguration": map[string]any{"enabled": fieldVal}},
	})
}

func hcV1MultiArchEnabledValue(hc *hcov1.HyperConverged) (enabled bool, found bool) {
	if hc.Spec.WorkloadSources.EnableMultiArchBootImageImport == nil {
		return false, false
	}

	return *hc.Spec.WorkloadSources.EnableMultiArchBootImageImport, true
}

func mutateMultiArchEnabled(spec hcov1.HyperConvergedSpec, fieldVal bool, patches []jsonpatch.JsonPatchOperation) []jsonpatch.JsonPatchOperation {
	return append(patches, jsonpatch.JsonPatchOperation{
		Operation: "add",
		Path:      v1MultiArchEnabledPath,
		Value:     fieldVal,
	})
}
