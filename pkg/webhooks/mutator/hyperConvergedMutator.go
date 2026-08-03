package mutator

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"

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
	mutatorV1Name           = "hyperConverged v1 mutator"
	featureGatesPath        = "/spec/featureGates"
	singleFeatureGatePrefix = featureGatesPath + "/"
	featureGatePathTmpt     = singleFeatureGatePrefix + "%d"
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

	var warnings []string

	switch req.Operation {
	case admissionv1.Create:
		patches = getMutatePatchesOnCreate(hc, patches)

		for _, fieldAndFG := range fieldFGDetails {
			allowed, fieldWarnings, newPatches := mutateFieldAndFGOnCreate(hc, fieldAndFG, patches)
			if !allowed {
				return admission.Denied(fieldAndFG.contraventionError)
			}
			patches = newPatches
			warnings = append(warnings, fieldWarnings...)
		}

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

		for _, fieldAndFG := range fieldFGDetails {
			allowed, fieldWarnings, newPatches := mutateFieldAndFGOnUpdate(hc, oldHC, fieldAndFG, patches)
			if !allowed {
				return admission.Denied(fieldAndFG.contraventionError)
			}
			patches = newPatches
			warnings = append(warnings, fieldWarnings...)
		}
	}

	return createResponse(patches, warnings, len(hc.Spec.FeatureGates))
}

func createResponse(patches []jsonpatch.JsonPatchOperation, warnings []string, numFGs int) admission.Response {
	var response admission.Response

	if len(patches) > 0 {
		patches = finalizePatches(patches, numFGs)

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

func dropFeatureGate(fgName string, fgs hcov1fg.HyperConvergedFeatureGates, patches []jsonpatch.JsonPatchOperation) []jsonpatch.JsonPatchOperation {
	if len(fgs) == 0 {
		return patches
	}

	idx := fgs.Index(fgName)

	if idx < 0 {
		return patches
	}

	path := featureGatesPath
	if len(fgs) > 1 {
		path = fmt.Sprintf(featureGatePathTmpt, idx)
	}

	return append(patches, jsonpatch.JsonPatchOperation{
		Operation: "remove",
		Path:      path,
	})
}

func mutateFieldAndFGOnCreate(hc *hcov1.HyperConverged, fieldAndFG fieldFGDetailsType, patches []jsonpatch.JsonPatchOperation) (allowed bool, warning []string, newPatches []jsonpatch.JsonPatchOperation) {
	fgEnabled, fgExists := hc.Spec.FeatureGates.IsExplicitlyEnabled(fieldAndFG.fgName)
	if !fgExists {
		return true, nil, patches
	}

	enabled, found := fieldAndFG.getFieldValue(hc)
	if found {
		if (fgEnabled == enabled) != fieldAndFG.fgShouldEqualField {
			return false, nil, nil
		}
		return true, []string{fieldAndFG.deprecationWarning}, patches
	}

	val := fgEnabled == fieldAndFG.fgShouldEqualField
	return true, []string{fieldAndFG.deprecationWarning}, fieldAndFG.mutateField(hc.Spec, val, patches)
}

func mutateFieldAndFGOnUpdate(hc, oldHC *hcov1.HyperConverged, fieldAndFG fieldFGDetailsType, patches []jsonpatch.JsonPatchOperation) (allow bool, warningList []string, newPatches []jsonpatch.JsonPatchOperation) {
	newFGEnabled, newFGPresent := hc.Spec.FeatureGates.IsExplicitlyEnabled(fieldAndFG.fgName)
	if !newFGPresent { // if the FG is not set in the requested HC, we need to do nothing
		return true, nil, patches
	}

	oldFGEnabled, oldFGPresent := oldHC.Spec.FeatureGates.IsExplicitlyEnabled(fieldAndFG.fgName)
	fgChanged := !oldFGPresent || (oldFGEnabled != newFGEnabled) // we know newFG is Present

	oldEnabled, oldEnabledFound := fieldAndFG.getFieldValue(oldHC)
	newEnabled, newEnabledFound := fieldAndFG.getFieldValue(hc)

	enabledChanged := oldEnabled != newEnabled || oldEnabledFound != newEnabledFound

	fgChangesLogic := (newEnabled == newFGEnabled) != fieldAndFG.fgShouldEqualField

	if fgChanged {
		if enabledChanged {
			if fgChangesLogic {
				return false, nil, nil
			}
		} else if fgChangesLogic || !newEnabledFound {
			// set the enabled field
			enabled := !newEnabled
			if !newEnabledFound {
				enabled = newFGEnabled == fieldAndFG.fgShouldEqualField
			}

			patches = fieldAndFG.mutateField(hc.Spec, enabled, patches)
		}

		return true, []string{fieldAndFG.deprecationWarning}, patches
	}

	// from here, FG was not changed
	if enabledChanged {
		return true, nil, dropFeatureGate(fieldAndFG.fgName, hc.Spec.FeatureGates, patches)
	}

	// from here, enabled was not changed
	if !newEnabledFound {
		// set enabled = !FG
		return true, nil, fieldAndFG.mutateField(hc.Spec, newFGEnabled == fieldAndFG.fgShouldEqualField, patches)
	}

	if fgChangesLogic {
		return true, nil, dropFeatureGate(fieldAndFG.fgName, hc.Spec.FeatureGates, patches)
	}

	return true, nil, patches
}

func finalizePatches(patches []jsonpatch.JsonPatchOperation, numFGs int) []jsonpatch.JsonPatchOperation {
	sortedPatches := make([]jsonpatch.JsonPatchOperation, 0, len(patches))
	var fgPatches []jsonpatch.JsonPatchOperation

	for _, patch := range patches {
		if strings.HasPrefix(patch.Path, singleFeatureGatePrefix) {
			fgPatches = append(fgPatches, patch)
		} else {
			sortedPatches = append(sortedPatches, patch)
		}
	}

	if len(fgPatches) == 0 {
		return sortedPatches
	}

	if numFGs == len(fgPatches) {
		sortedPatches = append(sortedPatches, jsonpatch.JsonPatchOperation{
			Operation: "remove",
			Path:      featureGatesPath,
		})
	} else {

		slices.SortFunc(fgPatches, compareFGPatches)

		sortedPatches = append(sortedPatches, fgPatches...)
	}

	return sortedPatches
}

func compareFGPatches(a, b jsonpatch.JsonPatchOperation) int {
	var aIdx, bIdx int
	_, err := fmt.Sscanf(a.Path, featureGatePathTmpt, &aIdx)
	if err != nil {
		// should never happen. No code in this package produces such path
		return -1
	}
	_, err = fmt.Sscanf(b.Path, featureGatePathTmpt, &bIdx)
	if err != nil {
		// should never happen. No code in this package produces such path
		return -1
	}

	return bIdx - aIdx
}
