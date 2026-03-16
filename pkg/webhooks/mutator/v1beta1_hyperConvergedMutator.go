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

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
)

const (
	mutatorV1Beta1Name = "hyperConverged v1beta1 mutator"
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

	v1hc := &hcov1.HyperConverged{}
	err = hc.ConvertTo(v1hc)
	if err != nil {
		logger.Error(err, "failed to convert the HyperConverged custom resource")
		return admission.Errored(http.StatusInternalServerError, fmt.Errorf("failed to convert to HyperConverged v1"))
	}

	patches := getMutatePatches(v1hc)
	patches = mutateV1beta1EvictionStrategy(hc, patches)

	if hc.Spec.MediatedDevicesConfiguration != nil {
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
	}

	if req.Operation == admissionv1.Create {
		patches = getV1beta1MutatePatchesOnCreate(hc, patches)
	}

	if len(patches) > 0 {
		return admission.Patched("mutated", patches...)
	}

	return admission.Allowed("")
}

func getV1beta1MutatePatchesOnCreate(hc *hcov1beta1.HyperConverged, patches []jsonpatch.JsonPatchOperation) []jsonpatch.JsonPatchOperation {
	if hc.Spec.KSMConfiguration == nil {
		patches = append(patches, jsonpatch.JsonPatchOperation{
			Operation: "add",
			Path:      "/spec/ksmConfiguration",
			Value:     kubevirtcorev1.KSMConfiguration{},
		})
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
