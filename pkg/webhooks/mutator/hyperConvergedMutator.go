package mutator

import (
	"context"
	"fmt"
	"net/http"
	"slices"

	"github.com/go-logr/logr"
	"gomodules.xyz/jsonpatch/v2"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	kubevirtcorev1 "kubevirt.io/api/core/v1"
	cdiv1beta1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	hcofeaturegates "github.com/kubevirt/hyperconverged-cluster-operator/api/v1/featuregates"
	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	goldenimages "github.com/kubevirt/hyperconverged-cluster-operator/controllers/handlers/golden-images"
)

const (
	mutatorV1Name = "hyperConverged v1 mutator"

	v1MediatedDevicesConfigurationPath = "/spec/virtualization/mediatedDevicesConfiguration"
	v1MediatedDevicesEnabledPath       = v1MediatedDevicesConfigurationPath + "/enabled"
)

var (
	_ admission.Handler = &HyperConvergedMutator{}
)

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

	var oldHC *hcov1.HyperConverged
	if req.Operation == admissionv1.Update && req.OldObject.Raw != nil {
		oldHC = &hcov1.HyperConverged{}
		if err := hcm.decoder.DecodeRaw(req.OldObject, oldHC); err != nil {
			logger.Error(err, "failed to read the old HyperConverged custom resource")
			return admission.Errored(http.StatusBadRequest, fmt.Errorf("failed to parse the old HyperConverged"))
		}
	}

	var patches []jsonpatch.JsonPatchOperation
	switch req.Operation {
	case admissionv1.Create:
		patches = getV1MutatePatchesOnCreate(hc)
	case admissionv1.Update:
		patches = getV1MutatePatchesOnUpdate(hc, oldHC)
	default:
		patches = nil
	}

	if len(patches) > 0 {
		return admission.Patched("mutated", patches...)
	}

	return admission.Allowed("")
}

// getV1MutatePatchesOnCreate builds patches for Create without needing the previous object.
func getV1MutatePatchesOnCreate(hc *hcov1.HyperConverged) []jsonpatch.JsonPatchOperation {
	patches := getDICTPatches(hc.Spec.WorkloadSources.DataImportCronTemplates, dictsPathTemplate)
	patches = mutateEvictionStrategy(hc, patches)
	patches = mutateTuningPolicy(hc, patches)
	patches = mutateV1MDevFeatureGateAndEnabledOnCreate(hc, patches)
	patches = getMutatePatchesOnCreate(hc, patches)

	return patches
}

// getV1MutatePatchesOnUpdate builds patches for Update using the previous object when present.
func getV1MutatePatchesOnUpdate(hc *hcov1.HyperConverged, oldHC *hcov1.HyperConverged) []jsonpatch.JsonPatchOperation {
	patches := getDICTPatches(hc.Spec.WorkloadSources.DataImportCronTemplates, dictsPathTemplate)
	patches = mutateEvictionStrategy(hc, patches)
	patches = mutateTuningPolicy(hc, patches)
	patches = mutateV1MDevFeatureGateAndEnabledOnUpdate(hc, oldHC, patches)

	return patches
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

// mutateV1MDevFeatureGateAndEnabledOnCreate keeps migration behavior on create when the deprecated feature gate is set.
func mutateV1MDevFeatureGateAndEnabledOnCreate(hc *hcov1.HyperConverged, patches []jsonpatch.JsonPatchOperation) []jsonpatch.JsonPatchOperation {
	fg := v1DeprecatedDisableMDevFGPtr(hc.Spec.FeatureGates)
	if fg == nil {
		return patches
	}

	if !*fg {
		return patches
	}

	if hc.Spec.Virtualization.MediatedDevicesConfiguration == nil {
		return append(patches, jsonpatch.JsonPatchOperation{
			Operation: "add",
			Path:      v1MediatedDevicesConfigurationPath,
			Value:     map[string]any{"enabled": false},
		})
	}

	if hc.Spec.Virtualization.MediatedDevicesConfiguration.Enabled == nil {
		return append(patches, jsonpatch.JsonPatchOperation{
			Operation: "add",
			Path:      v1MediatedDevicesEnabledPath,
			Value:     false,
		})
	}

	return patches
}

// mutateV1MDevFeatureGateAndEnabledOnUpdate keeps spec.featureGates (disableMDevConfiguration) and
// spec.virtualization.mediatedDevicesConfiguration.enabled in sync. When the legacy gate is absent, it is ignored.
func mutateV1MDevFeatureGateAndEnabledOnUpdate(hc *hcov1.HyperConverged, oldHC *hcov1.HyperConverged, patches []jsonpatch.JsonPatchOperation) []jsonpatch.JsonPatchOperation {
	fg := v1DeprecatedDisableMDevFGPtr(hc.Spec.FeatureGates)
	if fg == nil {
		return patches
	}

	if oldHC == nil {
		return patches
	}

	oldFG := v1DeprecatedDisableMDevFGPtr(oldHC.Spec.FeatureGates)
	oldEnabled := v1MDevEnabledValue(oldHC)
	newEnabled := v1MDevEnabledValue(hc)
	fgChanged := oldFG == nil || *oldFG != *fg
	enabledChanged := oldEnabled != newEnabled

	if enabledChanged && !fgChanged {
		patches = appendV1SyncDisableMDevFGPatchesForEnabledOnlyChange(patches, hc.Spec.FeatureGates, newEnabled, fg)
	} else if fgChanged && !enabledChanged {
		patches = appendV1SyncMDevEnabledPatchesForFGOnlyChange(hc, patches, fg)
	} else if !fgChanged && !enabledChanged {
		if *fg && hc.Spec.Virtualization.MediatedDevicesConfiguration == nil {
			patches = append(patches, jsonpatch.JsonPatchOperation{
				Operation: "add",
				Path:      v1MediatedDevicesConfigurationPath,
				Value:     map[string]any{"enabled": false},
			})
		} else if *fg && hc.Spec.Virtualization.MediatedDevicesConfiguration != nil && hc.Spec.Virtualization.MediatedDevicesConfiguration.Enabled == nil {
			patches = append(patches, jsonpatch.JsonPatchOperation{
				Operation: "add",
				Path:      v1MediatedDevicesEnabledPath,
				Value:     false,
			})
		}
	}

	return patches
}

func appendV1SyncDisableMDevFGPatchesForEnabledOnlyChange(
	patches []jsonpatch.JsonPatchOperation,
	fgs hcofeaturegates.HyperConvergedFeatureGates,
	newEnabled bool,
	fg *bool,
) []jsonpatch.JsonPatchOperation {
	if newEnabled {
		if fg != nil && *fg {
			idx := v1DeprecatedDisableMDevFGIndex(fgs)
			if idx < 0 {
				return patches
			}

			return append(patches, jsonpatch.JsonPatchOperation{
				Operation: "remove",
				Path:      fmt.Sprintf("/spec/featureGates/%d", idx),
			})
		}

		return patches
	}

	if fg == nil || !*fg {
		idx := v1DeprecatedDisableMDevFGIndex(fgs)
		if idx < 0 {
			return append(patches, jsonpatch.JsonPatchOperation{
				Operation: "add",
				Path:      "/spec/featureGates/-",
				Value: map[string]any{
					"name": hcofeaturegates.DeprecatedDisableMDevConfigurationFeatureGateName,
				},
			})
		}

		return append(patches, jsonpatch.JsonPatchOperation{
			Operation: "replace",
			Path:      fmt.Sprintf("/spec/featureGates/%d", idx),
			Value: map[string]any{
				"name": hcofeaturegates.DeprecatedDisableMDevConfigurationFeatureGateName,
			},
		})
	}

	return patches
}

func appendV1SyncMDevEnabledPatchesForFGOnlyChange(
	hc *hcov1.HyperConverged,
	patches []jsonpatch.JsonPatchOperation,
	fg *bool,
) []jsonpatch.JsonPatchOperation {
	if *fg {
		if hc.Spec.Virtualization.MediatedDevicesConfiguration == nil {
			return append(patches, jsonpatch.JsonPatchOperation{
				Operation: "add",
				Path:      v1MediatedDevicesConfigurationPath,
				Value:     map[string]any{"enabled": false},
			})
		}

		if hc.Spec.Virtualization.MediatedDevicesConfiguration.Enabled == nil {
			return append(patches, jsonpatch.JsonPatchOperation{
				Operation: "add",
				Path:      v1MediatedDevicesEnabledPath,
				Value:     false,
			})
		}

		if hc.Spec.Virtualization.MediatedDevicesConfiguration.Enabled != nil && *hc.Spec.Virtualization.MediatedDevicesConfiguration.Enabled {
			return patches
		}

		return append(patches, jsonpatch.JsonPatchOperation{
			Operation: "replace",
			Path:      v1MediatedDevicesEnabledPath,
			Value:     false,
		})
	}

	if hc.Spec.Virtualization.MediatedDevicesConfiguration != nil && hc.Spec.Virtualization.MediatedDevicesConfiguration.Enabled != nil {
		return append(patches, jsonpatch.JsonPatchOperation{
			Operation: "remove",
			Path:      v1MediatedDevicesEnabledPath,
		})
	}

	return patches
}

func v1MDevEnabledValue(hc *hcov1.HyperConverged) bool {
	if hc.Spec.Virtualization.MediatedDevicesConfiguration == nil || hc.Spec.Virtualization.MediatedDevicesConfiguration.Enabled == nil {
		return true
	}

	return *hc.Spec.Virtualization.MediatedDevicesConfiguration.Enabled
}

func v1DeprecatedDisableMDevFGIndex(fgs hcofeaturegates.HyperConvergedFeatureGates) int {
	return slices.IndexFunc(fgs, func(fg hcofeaturegates.FeatureGate) bool {
		return fg.Name == hcofeaturegates.DeprecatedDisableMDevConfigurationFeatureGateName
	})
}

func v1DeprecatedDisableMDevFGPtr(fgs hcofeaturegates.HyperConvergedFeatureGates) *bool {
	idx := v1DeprecatedDisableMDevFGIndex(fgs)
	if idx < 0 {
		return nil
	}

	state := ptr.Deref(fgs[idx].State, hcofeaturegates.Enabled)

	return ptr.To(state == hcofeaturegates.Enabled)
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
