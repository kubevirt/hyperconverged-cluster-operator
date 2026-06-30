package mutator

import (
	"context"
	"fmt"
	"net/http"
	"slices"

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
	disableMDevConfigurationFGName = "disableMDevConfiguration"
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

	patches := getDICTPatches(hc.Spec.WorkloadSources.DataImportCronTemplates, dictsPathTemplate)
	patches = mutateEvictionStrategy(hc, patches)
	patches = mutateTuningPolicy(hc, patches)

	if req.Operation == admissionv1.Create {
		patches = getMutatePatchesOnCreate(hc, patches)
		patches = hcMutateV1MDevFGAndEnabledOnCreate(hc, patches)
	} else {
		var oldHC *hcov1.HyperConverged
		if req.OldObject.Raw != nil {
			oldHC = &hcov1.HyperConverged{}
			if err := hcm.decoder.DecodeRaw(req.OldObject, oldHC); err != nil {
				logger.Error(err, "failed to read the old HyperConverged custom resource")
				return admission.Errored(http.StatusBadRequest, fmt.Errorf("failed to parse the old HyperConverged"))
			}
		}
		patches = hcMutateV1MDevFGAndEnabledOnUpdate(hc, oldHC, patches)
	}

	if len(patches) > 0 {
		return admission.Patched("mutated", patches...)
	}

	return admission.Allowed("")
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

func hcV1MDevEnabledValue(hc *hcov1.HyperConverged) bool {
	mdc := hc.Spec.Virtualization.MediatedDevicesConfiguration
	if mdc == nil || mdc.Enabled == nil {
		return true
	}

	return *mdc.Enabled
}

func hcPatchV1MDevEnabledFalseWhenUnset(hasConfig bool, patches []jsonpatch.JsonPatchOperation) []jsonpatch.JsonPatchOperation {
	if hasConfig {
		return append(patches, jsonpatch.JsonPatchOperation{
			Operation: "add",
			Path:      v1MDevEnabledPath,
			Value:     false,
		})
	}

	return append(patches, jsonpatch.JsonPatchOperation{
		Operation: "add",
		Path:      v1HyperConvergedMdevConfigPath,
		Value:     map[string]any{"enabled": false},
	})
}

func hcV1MDevConfigHasDeviceTypes(mdc *hcov1.MediatedDevicesConfiguration) bool {
	if mdc == nil {
		return false
	}

	if len(mdc.MediatedDeviceTypes) > 0 {
		return true
	}

	for _, nodeConfig := range mdc.NodeMediatedDeviceTypes {
		if len(nodeConfig.MediatedDeviceTypes) > 0 {
			return true
		}
	}

	return false
}

func hcPatchV1RemoveMDevEnabled(hc *hcov1.HyperConverged, patches []jsonpatch.JsonPatchOperation) []jsonpatch.JsonPatchOperation {
	mdc := hc.Spec.Virtualization.MediatedDevicesConfiguration
	if mdc == nil || mdc.Enabled == nil {
		return patches
	}

	if hcV1MDevConfigHasDeviceTypes(mdc) {
		return append(patches, jsonpatch.JsonPatchOperation{
			Operation: "remove",
			Path:      v1MDevEnabledPath,
		})
	}

	return append(patches, jsonpatch.JsonPatchOperation{
		Operation: "remove",
		Path:      v1HyperConvergedMdevConfigPath,
	})
}

func hcV1DisableMDevFGIndex(fgs hcov1fg.HyperConvergedFeatureGates) int {
	return slices.IndexFunc(fgs, func(fg hcov1fg.FeatureGate) bool {
		return fg.Name == disableMDevConfigurationFGName
	})
}

func hcPatchV1DisableMDevFG(fgs hcov1fg.HyperConvergedFeatureGates, enable bool, patches []jsonpatch.JsonPatchOperation) []jsonpatch.JsonPatchOperation {
	idx := hcV1DisableMDevFGIndex(fgs)
	if enable {
		if idx == -1 {
			return append(patches, jsonpatch.JsonPatchOperation{
				Operation: "add",
				Path:      "/spec/featureGates/-",
				Value:     hcov1fg.FeatureGate{Name: disableMDevConfigurationFGName},
			})
		}

		if !fgs.IsEnabled(disableMDevConfigurationFGName) {
			return append(patches, jsonpatch.JsonPatchOperation{
				Operation: "replace",
				Path:      fmt.Sprintf("/spec/featureGates/%d", idx),
				Value:     hcov1fg.FeatureGate{Name: disableMDevConfigurationFGName},
			})
		}

		return patches
	}

	if idx != -1 {
		return append(patches, jsonpatch.JsonPatchOperation{
			Operation: "remove",
			Path:      fmt.Sprintf("/spec/featureGates/%d", idx),
		})
	}

	return patches
}

func hcMutateV1MDevFGAndEnabledOnCreate(hc *hcov1.HyperConverged, patches []jsonpatch.JsonPatchOperation) []jsonpatch.JsonPatchOperation {
	if !hc.Spec.FeatureGates.IsEnabled(disableMDevConfigurationFGName) {
		return patches
	}

	mdc := hc.Spec.Virtualization.MediatedDevicesConfiguration
	if mdc != nil && mdc.Enabled != nil {
		return patches
	}

	return hcPatchV1MDevEnabledFalseWhenUnset(mdc != nil, patches)
}

func hcMutateV1HandleEnabledOnlyChanged(
	fgs hcov1fg.HyperConvergedFeatureGates,
	newEnabled bool,
	patches []jsonpatch.JsonPatchOperation,
) []jsonpatch.JsonPatchOperation {
	if !newEnabled {
		return hcPatchV1DisableMDevFG(fgs, true, patches)
	}

	return hcPatchV1DisableMDevFG(fgs, false, patches)
}

func hcMutateV1HandleFGOnlyChanged(
	hc *hcov1.HyperConverged,
	newFGEnabled bool,
	patches []jsonpatch.JsonPatchOperation,
) []jsonpatch.JsonPatchOperation {
	if newFGEnabled {
		mdc := hc.Spec.Virtualization.MediatedDevicesConfiguration
		if mdc != nil {
			return append(patches, jsonpatch.JsonPatchOperation{
				Operation: "add",
				Path:      v1MDevEnabledPath,
				Value:     false,
			})
		}

		return append(patches, jsonpatch.JsonPatchOperation{
			Operation: "add",
			Path:      v1HyperConvergedMdevConfigPath,
			Value:     map[string]any{"enabled": false},
		})
	}

	patches = hcPatchV1RemoveMDevEnabled(hc, patches)
	return hcPatchV1DisableMDevFG(hc.Spec.FeatureGates, false, patches)
}

func hcMutateV1HandleNormalization(hc *hcov1.HyperConverged, patches []jsonpatch.JsonPatchOperation) []jsonpatch.JsonPatchOperation {
	mdc := hc.Spec.Virtualization.MediatedDevicesConfiguration
	if mdc == nil || mdc.Enabled == nil {
		return hcPatchV1MDevEnabledFalseWhenUnset(mdc != nil, patches)
	}

	return patches
}

func hcMutateV1MDevFGAndEnabledOnUpdate(hc, oldHC *hcov1.HyperConverged, patches []jsonpatch.JsonPatchOperation) []jsonpatch.JsonPatchOperation {
	if oldHC == nil {
		return patches
	}

	oldIdx := hcV1DisableMDevFGIndex(oldHC.Spec.FeatureGates)
	newIdx := hcV1DisableMDevFGIndex(hc.Spec.FeatureGates)
	oldFGPresent := oldIdx != -1
	newFGPresent := newIdx != -1
	oldFGEnabled := oldHC.Spec.FeatureGates.IsEnabled(disableMDevConfigurationFGName)
	newFGEnabled := hc.Spec.FeatureGates.IsEnabled(disableMDevConfigurationFGName)

	oldEnabled := hcV1MDevEnabledValue(oldHC)
	newEnabled := hcV1MDevEnabledValue(hc)
	fgChanged := oldFGPresent != newFGPresent || (oldFGPresent && newFGPresent && oldFGEnabled != newFGEnabled)
	enabledChanged := oldEnabled != newEnabled

	if enabledChanged && !fgChanged {
		return hcMutateV1HandleEnabledOnlyChanged(hc.Spec.FeatureGates, newEnabled, patches)
	}

	if fgChanged && !enabledChanged {
		return hcMutateV1HandleFGOnlyChanged(hc, newFGEnabled, patches)
	}

	if !fgChanged && !enabledChanged && newFGEnabled {
		return hcMutateV1HandleNormalization(hc, patches)
	}

	return patches
}
