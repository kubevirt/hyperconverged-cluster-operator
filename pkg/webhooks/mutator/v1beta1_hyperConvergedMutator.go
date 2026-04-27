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

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
)

const (
	mutatorV1Beta1Name = "hyperConverged v1beta1 mutator"

	v1beta1DICTPathTemplate = "/spec/dataImportCronTemplates/%d"
)

var (
	_ admission.Handler = &HyperConvergedV1Beta1Mutator{}
)

// HyperConvergedV1Beta1Mutator mutates v1beta1 HyperConverged requests
type HyperConvergedV1Beta1Mutator struct {
	decoder admission.Decoder
	cli     client.Client
}

func NewHyperConvergedV1Beta1Mutator(cli client.Client, decoder admission.Decoder) *HyperConvergedV1Beta1Mutator {
	return &HyperConvergedV1Beta1Mutator{
		cli:     cli,
		decoder: decoder,
	}
}

func (hcm *HyperConvergedV1Beta1Mutator) Handle(ctx context.Context, req admission.Request) admission.Response {
	log := logr.FromContextOrDiscard(ctx).WithName(mutatorV1Beta1Name)
	log.Info("reaching HyperConvergedV1Beta1Mutator.Handle")

	if req.Operation == admissionv1.Update || req.Operation == admissionv1.Create {
		return hcm.mutateHyperConverged(req, log)
	}

	// ignoring other operations
	return admission.Allowed(ignoreOperationMessage)
}

func (hcm *HyperConvergedV1Beta1Mutator) mutateHyperConverged(req admission.Request, logger logr.Logger) admission.Response {
	hc := &hcov1beta1.HyperConverged{}
	err := hcm.decoder.Decode(req, hc)
	if err != nil {
		logger.Error(err, "failed to read the HyperConverged custom resource")
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("failed to parse the HyperConverged"))
	}

	var oldHC *hcov1beta1.HyperConverged
	if req.Operation == admissionv1.Update && req.OldObject.Raw != nil {
		oldHC = &hcov1beta1.HyperConverged{}
		if err := hcm.decoder.DecodeRaw(req.OldObject, oldHC); err != nil {
			logger.Error(err, "failed to read the old HyperConverged custom resource")
			return admission.Errored(http.StatusBadRequest, fmt.Errorf("failed to parse the old HyperConverged"))
		}
	}

	var patches []jsonpatch.JsonPatchOperation
	switch req.Operation {
	case admissionv1.Create:
		patches = getV1beta1MutatePatchesOnCreate(hc)
	case admissionv1.Update:
		patches = getV1beta1MutatePatchesOnUpdate(hc, oldHC)
	default:
		patches = nil
	}

	if req.Operation == admissionv1.Create {
		patches = getV1beta1MutatePatchesOnCreateKSM(hc, patches)
	}

	if len(patches) > 0 {
		return admission.Patched("mutated", patches...)
	}

	return admission.Allowed("")
}

// getV1beta1MutatePatchesOnCreate builds patches for Create without needing the previous object.
func getV1beta1MutatePatchesOnCreate(hc *hcov1beta1.HyperConverged) []jsonpatch.JsonPatchOperation {
	patches := getDICTPatches(hc.Spec.DataImportCronTemplates, v1beta1DICTPathTemplate)
	patches = mutateV1beta1EvictionStrategy(hc, patches)
	patches = mutateMDevFeatureGateAndEnabledOnCreate(hc, patches)
	patches = mutateV1beta1MediatedDeviceTypes(hc, patches)

	return patches
}

// getV1beta1MutatePatchesOnUpdate builds patches for Update using the previous object when present.
func getV1beta1MutatePatchesOnUpdate(hc *hcov1beta1.HyperConverged, oldHC *hcov1beta1.HyperConverged) []jsonpatch.JsonPatchOperation {
	patches := getDICTPatches(hc.Spec.DataImportCronTemplates, v1beta1DICTPathTemplate)
	patches = mutateV1beta1EvictionStrategy(hc, patches)
	patches = mutateMDevFeatureGateAndEnabledOnUpdate(hc, oldHC, patches)
	patches = mutateV1beta1MediatedDeviceTypes(hc, patches)

	return patches
}

func getV1beta1MutatePatchesOnCreateKSM(hc *hcov1beta1.HyperConverged, patches []jsonpatch.JsonPatchOperation) []jsonpatch.JsonPatchOperation {
	if hc.Spec.KSMConfiguration == nil {
		patches = append(patches, jsonpatch.JsonPatchOperation{
			Operation: "add",
			Path:      "/spec/ksmConfiguration",
			Value:     kubevirtcorev1.KSMConfiguration{},
		})
	}

	return patches
}

// mutateMDevFeatureGateAndEnabledOnCreate keeps migration behavior on create when the deprecated FG is set.
func mutateMDevFeatureGateAndEnabledOnCreate(hc *hcov1beta1.HyperConverged, patches []jsonpatch.JsonPatchOperation) []jsonpatch.JsonPatchOperation {
	//nolint:staticcheck // Read deprecated field to migrate CRs.
	fg := hc.Spec.FeatureGates.DisableMDevConfiguration
	if fg == nil {
		return patches
	}

	if !*fg {
		return patches
	}

	if hc.Spec.MediatedDevicesConfiguration == nil {
		return append(patches, jsonpatch.JsonPatchOperation{
			Operation: "add",
			Path:      "/spec/mediatedDevicesConfiguration",
			Value:     map[string]any{"enabled": false},
		})
	}

	if hc.Spec.MediatedDevicesConfiguration.Enabled == nil {
		return append(patches, jsonpatch.JsonPatchOperation{
			Operation: "add",
			Path:      "/spec/mediatedDevicesConfiguration/enabled",
			Value:     false,
		})
	}

	return patches
}

// mutateMDevFeatureGateAndEnabledOnUpdate keeps spec.featureGates.disableMDevConfiguration and
// spec.mediatedDevicesConfiguration.enabled in sync. When FG is nil, it is ignored.
func mutateMDevFeatureGateAndEnabledOnUpdate(hc *hcov1beta1.HyperConverged, oldHC *hcov1beta1.HyperConverged, patches []jsonpatch.JsonPatchOperation) []jsonpatch.JsonPatchOperation {
	//nolint:staticcheck // Read deprecated field to migrate CRs and keep in sync.
	fg := hc.Spec.FeatureGates.DisableMDevConfiguration
	if fg == nil {
		return patches
	}

	if oldHC == nil {
		return patches
	}

	//nolint:staticcheck // Read deprecated field for comparison.
	oldFG := oldHC.Spec.FeatureGates.DisableMDevConfiguration
	oldEnabled := mDevEnabledValue(oldHC)
	newEnabled := mDevEnabledValue(hc)
	// fg is non-nil here (early return above), so only compare against oldFG.
	fgChanged := oldFG == nil || *oldFG != *fg
	enabledChanged := oldEnabled != newEnabled

	if enabledChanged && !fgChanged {
		patches = appendSyncDisableMDevFGPatchesForEnabledOnlyChange(patches, newEnabled, fg)
	} else if fgChanged && !enabledChanged {
		patches = appendSyncMDevEnabledPatchesForFGOnlyChange(hc, patches, fg)
	} else if !fgChanged && !enabledChanged {
		if *fg && hc.Spec.MediatedDevicesConfiguration == nil {
			patches = append(patches, jsonpatch.JsonPatchOperation{
				Operation: "add",
				Path:      "/spec/mediatedDevicesConfiguration",
				Value:     map[string]any{"enabled": false},
			})
		} else if *fg && hc.Spec.MediatedDevicesConfiguration != nil && hc.Spec.MediatedDevicesConfiguration.Enabled == nil {
			patches = append(patches, jsonpatch.JsonPatchOperation{
				Operation: "add",
				Path:      "/spec/mediatedDevicesConfiguration/enabled",
				Value:     false,
			})
		}
	}

	return patches
}

func appendSyncDisableMDevFGPatchesForEnabledOnlyChange(
	patches []jsonpatch.JsonPatchOperation,
	newEnabled bool,
	fg *bool,
) []jsonpatch.JsonPatchOperation {
	if newEnabled {
		// Prefer dropping the deprecated key instead of forcing it to false (review feedback).
		if fg != nil && *fg {
			patches = append(patches, jsonpatch.JsonPatchOperation{
				Operation: "remove",
				Path:      "/spec/featureGates/disableMDevConfiguration",
			})
		}

		return patches
	}

	// newEnabled == false => disableMDevConfiguration should be true.
	if fg == nil || !*fg {
		if fg == nil {
			return append(patches, jsonpatch.JsonPatchOperation{
				Operation: "add",
				Path:      "/spec/featureGates/disableMDevConfiguration",
				Value:     true,
			})
		}

		return append(patches, jsonpatch.JsonPatchOperation{
			Operation: "replace",
			Path:      "/spec/featureGates/disableMDevConfiguration",
			Value:     true,
		})
	}

	return patches
}

func appendSyncMDevEnabledPatchesForFGOnlyChange(
	hc *hcov1beta1.HyperConverged,
	patches []jsonpatch.JsonPatchOperation,
	fg *bool,
) []jsonpatch.JsonPatchOperation {
	if *fg {
		if hc.Spec.MediatedDevicesConfiguration == nil {
			return append(patches, jsonpatch.JsonPatchOperation{
				Operation: "add",
				Path:      "/spec/mediatedDevicesConfiguration",
				Value:     map[string]any{"enabled": false},
			})
		}

		if hc.Spec.MediatedDevicesConfiguration.Enabled == nil {
			return append(patches, jsonpatch.JsonPatchOperation{
				Operation: "add",
				Path:      "/spec/mediatedDevicesConfiguration/enabled",
				Value:     false,
			})
		}

		// Explicit enabled=true wins over the legacy feature gate (align with kubevirt handler semantics).
		if hc.Spec.MediatedDevicesConfiguration.Enabled != nil && *hc.Spec.MediatedDevicesConfiguration.Enabled {
			return patches
		}

		return append(patches, jsonpatch.JsonPatchOperation{
			Operation: "replace",
			Path:      "/spec/mediatedDevicesConfiguration/enabled",
			Value:     false,
		})
	}

	// FG false => remove enabled so defaults apply (review feedback: prefer remove over replace+nil).
	if hc.Spec.MediatedDevicesConfiguration != nil && hc.Spec.MediatedDevicesConfiguration.Enabled != nil {
		return append(patches, jsonpatch.JsonPatchOperation{
			Operation: "remove",
			Path:      "/spec/mediatedDevicesConfiguration/enabled",
		})
	}

	return patches
}

func mDevEnabledValue(hc *hcov1beta1.HyperConverged) bool {
	if hc.Spec.MediatedDevicesConfiguration == nil || hc.Spec.MediatedDevicesConfiguration.Enabled == nil {
		return true
	}

	return *hc.Spec.MediatedDevicesConfiguration.Enabled
}

func mutateV1beta1MediatedDeviceTypes(hc *hcov1beta1.HyperConverged, patches []jsonpatch.JsonPatchOperation) []jsonpatch.JsonPatchOperation {
	if hc.Spec.MediatedDevicesConfiguration == nil {
		return patches
	}

	if len(hc.Spec.MediatedDevicesConfiguration.MediatedDevicesTypes) > 0 && len(hc.Spec.MediatedDevicesConfiguration.MediatedDeviceTypes) == 0 { //nolint SA1019
		patches = append(patches, jsonpatch.JsonPatchOperation{
			Operation: "add",
			Path:      "/spec/mediatedDevicesConfiguration/mediatedDeviceTypes",
			Value:     hc.Spec.MediatedDevicesConfiguration.MediatedDevicesTypes, //nolint SA1019
		})
	}

	for i, hcoNodeMdevTypeConf := range hc.Spec.MediatedDevicesConfiguration.NodeMediatedDeviceTypes {
		if len(hcoNodeMdevTypeConf.MediatedDevicesTypes) > 0 && len(hcoNodeMdevTypeConf.MediatedDeviceTypes) == 0 { //nolint SA1019
			patches = append(patches, jsonpatch.JsonPatchOperation{
				Operation: "add",
				Path:      fmt.Sprintf("/spec/mediatedDevicesConfiguration/nodeMediatedDeviceTypes/%d/mediatedDeviceTypes", i),
				Value:     hcoNodeMdevTypeConf.MediatedDevicesTypes, //nolint SA1019
			})
		}
	}

	return patches
}

func mutateV1beta1EvictionStrategy(hc *hcov1beta1.HyperConverged, patches []jsonpatch.JsonPatchOperation) []jsonpatch.JsonPatchOperation {
	if hc.Status.InfrastructureHighlyAvailable == nil || hc.Spec.EvictionStrategy != nil { // New HyperConverged CR
		return patches
	}

	var value = kubevirtcorev1.EvictionStrategyNone
	if *hc.Status.InfrastructureHighlyAvailable {
		value = kubevirtcorev1.EvictionStrategyLiveMigrate
	}

	patches = append(patches, jsonpatch.JsonPatchOperation{
		Operation: "replace",
		Path:      "/spec/evictionStrategy",
		Value:     value,
	})

	return patches
}
