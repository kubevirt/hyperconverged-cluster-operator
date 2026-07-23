package mutator

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-logr/logr"
	"gomodules.xyz/jsonpatch/v2"
	admissionv1 "k8s.io/api/admission/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	kubevirtcorev1 "kubevirt.io/api/core/v1"
	cdiv1beta1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	hcov1fg "github.com/kubevirt/hyperconverged-cluster-operator/api/v1/featuregates"
	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	goldenimages "github.com/kubevirt/hyperconverged-cluster-operator/controllers/handlers/golden-images"
)

const (
	mutatorV1Name = "hyperConverged v1 mutator"

	v1HyperConvergedMdevConfigPath = "/spec/virtualization/mediatedDevicesConfiguration"
	v1MDevEnabledPath              = v1HyperConvergedMdevConfigPath + "/enabled"
	disableMDevConfigurationFGName = hcov1beta1.DisableMDevConfigurationFG

	fgDeprecationMsg = "feature gate %s is deprecated; please use the spec.virtualization.mediatedDevicesConfiguration.enabled field instead"

	mdevErrorMessage = "the deprecated disableMDevConfiguration feature gate, and spec.virtualization.mediatedDevicesConfiguration.enabled field contradict each other; disableMDevConfiguration must not be set, or equal !enabled"

	v1HyperConvergedStoragePath  = "/spec/storage"
	v1HyperConvergedPRConfigPath = v1HyperConvergedStoragePath + "/persistentReservationConfiguration"
	v1PRConfigEnabledPath        = v1HyperConvergedPRConfigPath + "/enabled"
	persistentReservationFGName  = "persistentReservation"

	prFGDeprecationMsg = "feature gate %s is deprecated; please use the spec.storage.persistentReservationConfiguration.enabled field instead"

	prErrorMessage = "the deprecated persistentReservation feature gate, and spec.storage.persistentReservationConfiguration.enabled field contradict each other; persistentReservation must not be set, or equal enabled"
)

var (
	_ admission.Handler = &HyperConvergedMutator{}
)

var mdevWarning = fmt.Sprintf(fgDeprecationMsg, disableMDevConfigurationFGName)
var prWarning = fmt.Sprintf(prFGDeprecationMsg, persistentReservationFGName)

// HyperConvergedMutator mutates HyperConverged requests
type HyperConvergedMutator struct {
	decoder admission.Decoder
	cli     client.Client
}

func NewHyperConvergedMutator(cli client.Client, decoder admission.Decoder) *HyperConvergedMutator {
	return &HyperConvergedMutator{
		cli:     cli,
		decoder: decoder,
	}
}

func (hcm *HyperConvergedMutator) Handle(ctx context.Context, req admission.Request) admission.Response {
	log := logr.FromContextOrDiscard(ctx).WithName(mutatorV1Name)
	log.Info("reaching HyperConvergedMutator.Handle")

	if req.Operation == admissionv1.Update || req.Operation == admissionv1.Create {
		return hcm.mutateHyperConverged(req, log)
	}

	// ignoring other operations
	return admission.Allowed(ignoreOperationMessage)
}

const (
	dictsPathTemplate           = "/spec/workloadSources/dataImportCronTemplates/%d"
	dictAnnotationPath          = "/metadata/annotations"
	dictImmediateAnnotationPath = "/cdi.kubevirt.io~1storage.bind.immediate.requested"
	retentionPolicyPath         = "/spec/retentionPolicy"
	importsToKeepPath           = "/spec/importsToKeep"
)

func (hcm *HyperConvergedMutator) mutateHyperConverged(req admission.Request, logger logr.Logger) admission.Response {
	hc := &hcov1.HyperConverged{}
	err := hcm.decoder.Decode(req, hc)
	if err != nil {
		logger.Error(err, "failed to read the HyperConverged custom resource")
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("failed to parse the HyperConverged"))
	}

	patches := getDICTPatches(hc.Spec.WorkloadSources.DataImportCronTemplates, dictsPathTemplate)
	patches = mutateEvictionStrategy(hc, patches)
	patches = mutateTuningPolicy(hc, patches)

	var warnings []string

	switch req.Operation {
	case admissionv1.Create:
		patches = getMutatePatchesOnCreate(hc, patches)

		allowed, mdevWarnings, newPatches := hcMutateV1MDevFGAndEnabledOnCreate(hc, patches)
		if !allowed {
			return admission.Denied(mdevErrorMessage)
		}
		patches = newPatches
		warnings = append(warnings, mdevWarnings...)

		allowed, prWarnings, newPatches := hcMutateV1PRFGAndEnabledOnCreate(hc, patches)
		if !allowed {
			return admission.Denied(prErrorMessage)
		}
		patches = newPatches
		warnings = append(warnings, prWarnings...)

	case admissionv1.Update:
		var oldHC *hcov1.HyperConverged
		if len(req.OldObject.Raw) == 0 {
			return admission.Errored(http.StatusBadRequest, fmt.Errorf("missing old object"))
		}

		oldHC = &hcov1.HyperConverged{}
		if err = hcm.decoder.DecodeRaw(req.OldObject, oldHC); err != nil {
			logger.Error(err, "failed to read the old HyperConverged custom resource")
			return admission.Errored(http.StatusBadRequest, fmt.Errorf("failed to parse the old HyperConverged"))
		}

		allowed, mdevWarnings, newPatches := hcMutateV1MDevFGAndEnabledOnUpdate(hc, oldHC, patches)
		if !allowed {
			return admission.Denied(mdevErrorMessage)
		}
		patches = newPatches
		warnings = append(warnings, mdevWarnings...)

		allowed, prWarnings, newPatches := hcMutateV1PRFGAndEnabledOnUpdate(hc, oldHC, patches)
		if !allowed {
			return admission.Denied(prErrorMessage)
		}
		patches = newPatches
		warnings = append(warnings, prWarnings...)
	}

	return createResponse(patches, warnings)
}

func createResponse(patches []jsonpatch.JsonPatchOperation, warnings []string) admission.Response {
	var response admission.Response

	if len(patches) > 0 {
		response = admission.Patched("mutated", patches...)
	} else {
		response = admission.Allowed("")
	}

	if len(warnings) > 0 {
		response = response.WithWarnings(warnings...)
	}

	return response
}

func getDICTPatches(dicts []hcov1.DataImportCronTemplate, patchTemplate string) []jsonpatch.JsonPatchOperation {
	var patches []jsonpatch.JsonPatchOperation
	for index, dict := range dicts {
		if dict.Annotations == nil {
			patches = append(patches, jsonpatch.JsonPatchOperation{
				Operation: "add",
				Path:      fmt.Sprintf(patchTemplate+dictAnnotationPath, index),
				Value:     map[string]string{goldenimages.CDIImmediateBindAnnotation: "true"},
			})
		} else if _, annotationFound := dict.Annotations[goldenimages.CDIImmediateBindAnnotation]; !annotationFound {
			patches = append(patches, jsonpatch.JsonPatchOperation{
				Operation: "add",
				Path:      fmt.Sprintf(patchTemplate+dictAnnotationPath+dictImmediateAnnotationPath, index),
				Value:     "true",
			})
		}

		if dict.Spec != nil {
			if dict.Spec.RetentionPolicy == nil {
				patches = append(patches, jsonpatch.JsonPatchOperation{
					Operation: "add",
					Path:      fmt.Sprintf(patchTemplate+retentionPolicyPath, index),
					Value:     cdiv1beta1.DataImportCronRetainNone,
				})
			}

			if dict.Spec.ImportsToKeep == nil {
				patches = append(patches, jsonpatch.JsonPatchOperation{
					Operation: "add",
					Path:      fmt.Sprintf(patchTemplate+importsToKeepPath, index),
					Value:     1,
				})
			}
		}
	}

	return patches
}

func getMutatePatchesOnCreate(hc *hcov1.HyperConverged, patches []jsonpatch.JsonPatchOperation) []jsonpatch.JsonPatchOperation {
	if hc.Spec.Virtualization.KSMConfiguration == nil {
		patches = append(patches, jsonpatch.JsonPatchOperation{
			Operation: "add",
			Path:      "/spec/virtualization/ksmConfiguration",
			Value:     kubevirtcorev1.KSMConfiguration{},
		})
	}

	return patches
}

func mutateEvictionStrategy(hc *hcov1.HyperConverged, patches []jsonpatch.JsonPatchOperation) []jsonpatch.JsonPatchOperation {
	if hc.Status.InfrastructureHighlyAvailable == nil || hc.Spec.Virtualization.EvictionStrategy != nil { // New HyperConverged CR
		return patches
	}

	var value = kubevirtcorev1.EvictionStrategyNone
	if *hc.Status.InfrastructureHighlyAvailable {
		value = kubevirtcorev1.EvictionStrategyLiveMigrate
	}

	patches = append(patches, jsonpatch.JsonPatchOperation{
		Operation: "replace",
		Path:      "/spec/virtualization/evictionStrategy",
		Value:     value,
	})

	return patches
}

// the "highBurst" tuningPolicy is not supported in v1. If set, drop it and make KubeVirt use its
// default values, that are now equal to the v1beta1 highBurst policy.
func mutateTuningPolicy(hc *hcov1.HyperConverged, patches []jsonpatch.JsonPatchOperation) []jsonpatch.JsonPatchOperation {
	if hc.Spec.Virtualization.TuningPolicy != hcov1beta1.HyperConvergedHighBurstProfile { //nolint SA1019
		return patches
	}

	patches = append(patches, jsonpatch.JsonPatchOperation{
		Operation: "remove",
		Path:      "/spec/virtualization/tuningPolicy",
	})

	return patches
}

func hcV1MDevEnabledValue(hc *hcov1.HyperConverged) (enabled bool, found bool) {
	mdc := hc.Spec.Virtualization.MediatedDevicesConfiguration
	if mdc == nil || mdc.Enabled == nil {
		return true, false
	}

	return *mdc.Enabled, true
}

func dropFeatureGate(fgName string, fgs hcov1fg.HyperConvergedFeatureGates, patches []jsonpatch.JsonPatchOperation) []jsonpatch.JsonPatchOperation {
	if len(fgs) == 0 {
		return patches
	}

	idx := fgs.Index(fgName)

	if idx < 0 {
		return patches
	}

	path := "/spec/featureGates"
	if len(fgs) > 1 {
		path = fmt.Sprintf(path+"/%d", idx)
	}

	return append(patches, jsonpatch.JsonPatchOperation{
		Operation: "remove",
		Path:      path,
	})
}

func hcMutateV1MDevFGAndEnabledOnCreate(hc *hcov1.HyperConverged, patches []jsonpatch.JsonPatchOperation) (allowed bool, warning []string, newPatches []jsonpatch.JsonPatchOperation) {
	fgEnabled, fgExists := hc.Spec.FeatureGates.IsExplicitlyEnabled(disableMDevConfigurationFGName)
	if !fgExists {
		return true, nil, patches
	}

	mdc := hc.Spec.Virtualization.MediatedDevicesConfiguration
	if mdc != nil && mdc.Enabled != nil {
		if fgEnabled == *mdc.Enabled {
			//nolint:staticcheck
			// this is a bug in the staticcheck linter. fmt.Errorf may be used with no parameters
			return false, nil, nil
		}
		return true, []string{mdevWarning}, patches
	}

	return true, []string{mdevWarning}, mutateMdevEnabled(mdc, !fgEnabled, patches)
}

func mutateMdevEnabled(
	mdevConfig *hcov1.MediatedDevicesConfiguration,
	fieldVal bool,
	patches []jsonpatch.JsonPatchOperation,
) []jsonpatch.JsonPatchOperation {

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

func hcMutateV1MDevFGAndEnabledOnUpdate(hc, oldHC *hcov1.HyperConverged, patches []jsonpatch.JsonPatchOperation) (allow bool, warningList []string, newPatches []jsonpatch.JsonPatchOperation) {
	newFGEnabled, newFGPresent := hc.Spec.FeatureGates.IsExplicitlyEnabled(disableMDevConfigurationFGName)
	if !newFGPresent { // if the FG is not set in the requested HC, we need to do nothing
		return true, nil, patches
	}

	oldFGEnabled, oldFGPresent := oldHC.Spec.FeatureGates.IsExplicitlyEnabled(disableMDevConfigurationFGName)
	fgChanged := !oldFGPresent || (oldFGEnabled != newFGEnabled) // we know newFG is Present

	oldEnabled, oldEnabledFound := hcV1MDevEnabledValue(oldHC)
	newEnabled, newEnabledFound := hcV1MDevEnabledValue(hc)

	enabledChanged := oldEnabled != newEnabled || oldEnabledFound != newEnabledFound

	if fgChanged {
		if enabledChanged {
			if newEnabled == newFGEnabled {
				return false, nil, nil
			}
		} else if newEnabled == newFGEnabled || !newEnabledFound {
			// set the enabled field
			enabled := !newEnabled
			if !newEnabledFound {
				enabled = !newFGEnabled
			}

			patches = mutateMdevEnabled(hc.Spec.Virtualization.MediatedDevicesConfiguration, enabled, patches)
		}

		return true, []string{mdevWarning}, patches
	}

	// from here, FG was not changed
	if enabledChanged {
		return true, nil, dropFeatureGate(disableMDevConfigurationFGName, hc.Spec.FeatureGates, patches)
	}

	// from here, enabled was not changed
	if !newEnabledFound {
		// set enabled = !FG
		return true, nil, mutateMdevEnabled(hc.Spec.Virtualization.MediatedDevicesConfiguration, !newFGEnabled, patches)
	}

	if newEnabled == newFGEnabled {
		return true, nil, dropFeatureGate(disableMDevConfigurationFGName, hc.Spec.FeatureGates, patches)
	}

	return true, nil, patches
}

func hcV1PREnabledValue(hc *hcov1.HyperConverged) (enabled bool, found bool) {
	if hc.Spec.Storage == nil ||
		hc.Spec.Storage.PersistentReservationConfiguration == nil ||
		hc.Spec.Storage.PersistentReservationConfiguration.Enabled == nil {
		return false, false
	}

	return *hc.Spec.Storage.PersistentReservationConfiguration.Enabled, true
}

func mutatePREnabled(storage *hcov1.StorageConfig, fieldVal bool, patches []jsonpatch.JsonPatchOperation) []jsonpatch.JsonPatchOperation {
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

func hcMutateV1PRFGAndEnabledOnCreate(hc *hcov1.HyperConverged, patches []jsonpatch.JsonPatchOperation) (allowed bool, warning []string, newPatches []jsonpatch.JsonPatchOperation) {
	fgEnabled, fgExists := hc.Spec.FeatureGates.IsExplicitlyEnabled(persistentReservationFGName)
	if !fgExists {
		return true, nil, patches
	}

	enabled, found := hcV1PREnabledValue(hc)
	if found {
		if fgEnabled != enabled {
			return false, nil, nil
		}
		return true, []string{prWarning}, patches
	}

	return true, []string{prWarning}, mutatePREnabled(hc.Spec.Storage, fgEnabled, patches)
}

func hcMutateV1PRFGAndEnabledOnUpdate(hc, oldHC *hcov1.HyperConverged, patches []jsonpatch.JsonPatchOperation) (allow bool, warningList []string, newPatches []jsonpatch.JsonPatchOperation) {
	newFGEnabled, newFGPresent := hc.Spec.FeatureGates.IsExplicitlyEnabled(persistentReservationFGName)
	if !newFGPresent {
		return true, nil, patches
	}

	oldFGEnabled, oldFGPresent := oldHC.Spec.FeatureGates.IsExplicitlyEnabled(persistentReservationFGName)
	fgChanged := !oldFGPresent || (oldFGEnabled != newFGEnabled)

	oldEnabled, oldEnabledFound := hcV1PREnabledValue(oldHC)
	newEnabled, newEnabledFound := hcV1PREnabledValue(hc)

	enabledChanged := oldEnabled != newEnabled || oldEnabledFound != newEnabledFound

	if fgChanged {
		if enabledChanged {
			if newEnabled != newFGEnabled {
				return false, nil, nil
			}
		} else if newEnabled != newFGEnabled || !newEnabledFound {
			patches = mutatePREnabled(hc.Spec.Storage, newFGEnabled, patches)
		}

		return true, []string{prWarning}, patches
	}

	// from here, FG was not changed
	if enabledChanged {
		return true, nil, dropFeatureGate(persistentReservationFGName, hc.Spec.FeatureGates, patches)
	}

	// from here, enabled was not changed
	if !newEnabledFound {
		return true, nil, mutatePREnabled(hc.Spec.Storage, newFGEnabled, patches)
	}

	if newEnabled != newFGEnabled {
		return true, nil, dropFeatureGate(persistentReservationFGName, hc.Spec.FeatureGates, patches)
	}

	return true, nil, patches
}
