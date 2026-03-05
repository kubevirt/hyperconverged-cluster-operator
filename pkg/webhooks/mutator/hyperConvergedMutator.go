package mutator

import (
	"context"
	"fmt"
	"net/http"

	"gomodules.xyz/jsonpatch/v2"
	admissionv1 "k8s.io/api/admission/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	kubevirtcorev1 "kubevirt.io/api/core/v1"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	goldenimages "github.com/kubevirt/hyperconverged-cluster-operator/controllers/handlers/golden-images"
)

var (
	hcMutatorLogger = logf.Log.WithName("hyperConverged mutator")

	_ admission.Handler = &NsMutator{}
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

func (hcm *HyperConvergedMutator) Handle(_ context.Context, req admission.Request) admission.Response {
	hcMutatorLogger.Info("reaching HyperConvergedMutator.Handle")

	if req.Operation == admissionv1.Update || req.Operation == admissionv1.Create {
		return hcm.mutateHyperConverged(req)
	}

	// ignoring other operations
	return admission.Allowed(ignoreOperationMessage)
}

const (
	annotationPathTemplate     = "/spec/dataImportCronTemplates/%d/metadata/annotations"
	dictAnnotationPathTemplate = annotationPathTemplate + "/cdi.kubevirt.io~1storage.bind.immediate.requested"
)

func (hcm *HyperConvergedMutator) mutateHyperConverged(req admission.Request) admission.Response {
	hc := &hcov1beta1.HyperConverged{}
	err := hcm.decoder.Decode(req, hc)
	if err != nil {
		hcMutatorLogger.Error(err, "failed to read the HyperConverged custom resource")
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("failed to parse the HyperConverged"))
	}

	var oldHC *hcov1beta1.HyperConverged
	if req.Operation == admissionv1.Update && req.OldObject.Raw != nil {
		oldHC = &hcov1beta1.HyperConverged{}
		if err := hcm.decoder.DecodeRaw(req.OldObject, oldHC); err != nil {
			hcMutatorLogger.Error(err, "failed to read the old HyperConverged custom resource")
			return admission.Errored(http.StatusBadRequest, fmt.Errorf("failed to parse the old HyperConverged"))
		}
	}

	patches := getMutatePatches(hc, oldHC, req.Operation)

	if req.Operation == admissionv1.Create && hc.Spec.KSMConfiguration == nil {
		patches = append(patches, jsonpatch.JsonPatchOperation{
			Operation: "add",
			Path:      "/spec/ksmConfiguration",
			Value:     kubevirtcorev1.KSMConfiguration{},
		})
	}

	if len(patches) > 0 {
		return admission.Patched("mutated", patches...)
	}

	return admission.Allowed("")
}

func getMutatePatches(hc *hcov1beta1.HyperConverged, oldHC *hcov1beta1.HyperConverged, operation admissionv1.Operation) []jsonpatch.JsonPatchOperation {
	var patches []jsonpatch.JsonPatchOperation
	for index, dict := range hc.Spec.DataImportCronTemplates {
		if dict.Annotations == nil {
			patches = append(patches, jsonpatch.JsonPatchOperation{
				Operation: "add",
				Path:      fmt.Sprintf(annotationPathTemplate, index),
				Value:     map[string]string{goldenimages.CDIImmediateBindAnnotation: "true"},
			})
		} else if _, annotationFound := dict.Annotations[goldenimages.CDIImmediateBindAnnotation]; !annotationFound {
			patches = append(patches, jsonpatch.JsonPatchOperation{
				Operation: "add",
				Path:      fmt.Sprintf(dictAnnotationPathTemplate, index),
				Value:     "true",
			})
		}
	}

	patches = mutateEvictionStrategy(hc, patches)

	patches = mutateMDevFeatureGateAndEnabled(hc, oldHC, operation, patches)

	patches = mutateMediatedDeviceTypes(hc, patches)

	return patches
}

// mutateMDevFeatureGateAndEnabled keeps spec.featureGates.disableMDevConfiguration and
// spec.mediatedDevicesConfiguration.enabled in sync. When FG is nil, it is ignored.
func mutateMDevFeatureGateAndEnabled(hc *hcov1beta1.HyperConverged, oldHC *hcov1beta1.HyperConverged, operation admissionv1.Operation, patches []jsonpatch.JsonPatchOperation) []jsonpatch.JsonPatchOperation {
	//nolint:staticcheck // Read deprecated field to migrate CRs and keep in sync.
	fg := hc.Spec.FeatureGates.DisableMDevConfiguration
	if fg == nil {
		return patches
	}

	if operation == admissionv1.Create {
		// Create: when FG is true and enabled is missing, set enabled = false (upgrade/migration behavior).
		if *fg && hc.Spec.MediatedDevicesConfiguration != nil && hc.Spec.MediatedDevicesConfiguration.Enabled == nil {
			patches = append(patches, jsonpatch.JsonPatchOperation{
				Operation: "add",
				Path:      "/spec/mediatedDevicesConfiguration/enabled",
				Value:     false,
			})
		}
		return patches
	}

	// Update: sync FG and enabled when only one of them changed.
	if oldHC == nil {
		return patches
	}
	//nolint:staticcheck // Read deprecated field for comparison.
	oldFG := oldHC.Spec.FeatureGates.DisableMDevConfiguration
	oldEnabled := mDevEnabledValue(oldHC)
	newEnabled := mDevEnabledValue(hc)
	fgChanged := (oldFG == nil) != (fg == nil) || (oldFG != nil && fg != nil && *oldFG != *fg)
	enabledChanged := oldEnabled != newEnabled

	if enabledChanged && !fgChanged {
		// Only enabled changed: set FG = !enabled.
		patches = append(patches, jsonpatch.JsonPatchOperation{
			Operation: "replace",
			Path:      "/spec/featureGates/disableMDevConfiguration",
			Value:     !newEnabled,
		})
	} else if fgChanged && !enabledChanged {
		// Only FG changed: set enabled to match (FG true => enabled false; FG false => enabled nil).
		if *fg {
			if hc.Spec.MediatedDevicesConfiguration == nil {
				patches = append(patches, jsonpatch.JsonPatchOperation{
					Operation: "add",
					Path:      "/spec/mediatedDevicesConfiguration",
					Value:     map[string]interface{}{"enabled": false},
				})
			} else if hc.Spec.MediatedDevicesConfiguration.Enabled == nil {
				patches = append(patches, jsonpatch.JsonPatchOperation{
					Operation: "add",
					Path:      "/spec/mediatedDevicesConfiguration/enabled",
					Value:     false,
				})
			} else {
				patches = append(patches, jsonpatch.JsonPatchOperation{
					Operation: "replace",
					Path:      "/spec/mediatedDevicesConfiguration/enabled",
					Value:     false,
				})
			}
		} else {
			// FG false => set enabled to null so default (true) applies.
			if hc.Spec.MediatedDevicesConfiguration != nil && hc.Spec.MediatedDevicesConfiguration.Enabled != nil {
				patches = append(patches, jsonpatch.JsonPatchOperation{
					Operation: "replace",
					Path:      "/spec/mediatedDevicesConfiguration/enabled",
					Value:     nil,
				})
			}
		}
	} else if !fgChanged && !enabledChanged {
		// Neither changed but FG is true and enabled is missing (e.g. upgrade): set enabled = false.
		if *fg && hc.Spec.MediatedDevicesConfiguration != nil && hc.Spec.MediatedDevicesConfiguration.Enabled == nil {
			patches = append(patches, jsonpatch.JsonPatchOperation{
				Operation: "add",
				Path:      "/spec/mediatedDevicesConfiguration/enabled",
				Value:     false,
			})
		}
	}
	return patches
}

func mDevEnabledValue(hc *hcov1beta1.HyperConverged) bool {
	if hc.Spec.MediatedDevicesConfiguration == nil || hc.Spec.MediatedDevicesConfiguration.Enabled == nil {
		return true
	}
	return *hc.Spec.MediatedDevicesConfiguration.Enabled
}

func mutateMediatedDeviceTypes(hc *hcov1beta1.HyperConverged, patches []jsonpatch.JsonPatchOperation) []jsonpatch.JsonPatchOperation {
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

func mutateEvictionStrategy(hc *hcov1beta1.HyperConverged, patches []jsonpatch.JsonPatchOperation) []jsonpatch.JsonPatchOperation {
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
