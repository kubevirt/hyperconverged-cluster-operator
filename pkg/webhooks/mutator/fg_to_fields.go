package mutator

import (
	"gomodules.xyz/jsonpatch/v2"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
)

const (
	v1HyperConvergedMdevConfigPath = "/spec/virtualization/mediatedDevicesConfiguration"
	v1MDevEnabledPath              = v1HyperConvergedMdevConfigPath + "/enabled"
	v1MDevEnabledFiled             = "spec.virtualization.mediatedDevicesConfiguration.enabled"
	disableMDevConfigurationFGName = hcov1beta1.DisableMDevConfigurationFG
	fgDeprecationMsg               = "feature gate " + disableMDevConfigurationFGName + " is deprecated; please use the " + v1MDevEnabledFiled + " field instead"
	mdevErrorMessage               = "the deprecated " + disableMDevConfigurationFGName + " feature gate, and " + v1MDevEnabledFiled + " field contradict each other; the feature gate must not be set, or equal !enabled"

	v1HyperConvergedStoragePath  = "/spec/storage"
	v1HyperConvergedPRConfigPath = v1HyperConvergedStoragePath + "/persistentReservationConfiguration"
	v1PRConfigEnabledPath        = v1HyperConvergedPRConfigPath + "/enabled"
	v1PRConfigEnabledField       = "spec.storage.persistentReservationConfiguration.enabled"
	persistentReservationFGName  = "persistentReservation"
	prFGDeprecationMsg           = "feature gate " + persistentReservationFGName + " is deprecated; please use the " + v1PRConfigEnabledField + " field instead"
	prErrorMessage               = "the deprecated " + persistentReservationFGName + " feature gate, and " + v1PRConfigEnabledField + " field contradict each other; feature gate must not be set, or equal enabled"
)

type getFieldValueFunc func(*hcov1.HyperConverged) (enabled, found bool)
type mutateFiledFunc func(hc hcov1.HyperConvergedSpec, fieldVal bool, patches []jsonpatch.JsonPatchOperation) []jsonpatch.JsonPatchOperation

type fieldFGDetailsType struct {
	getFieldValue      getFieldValueFunc
	mutateFiled        mutateFiledFunc
	fgName             string
	deprecationWarning string
	contraventionError string
	fgShouldEqualField bool
}

var fieldFGDetails = []fieldFGDetailsType{
	{
		getFieldValue:      hcV1MDevEnabledValue,
		mutateFiled:        mutateMdevEnabled,
		fgName:             disableMDevConfigurationFGName,
		deprecationWarning: fgDeprecationMsg,
		contraventionError: mdevErrorMessage,
		fgShouldEqualField: false,
	},
	{
		getFieldValue:      hcV1PREnabledValue,
		mutateFiled:        mutatePREnabled,
		fgName:             persistentReservationFGName,
		deprecationWarning: prFGDeprecationMsg,
		contraventionError: prErrorMessage,
		fgShouldEqualField: true,
	},
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

func hcV1MDevEnabledValue(hc *hcov1.HyperConverged) (enabled bool, found bool) {
	mdc := hc.Spec.Virtualization.MediatedDevicesConfiguration
	if mdc == nil || mdc.Enabled == nil {
		return true, false
	}

	return *mdc.Enabled, true
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
