package mutator

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-logr/logr"
	admissionv1 "k8s.io/api/admission/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
)

const (
	mutatorV1Beta1Name = "hyperConverged v1beta1 mutator"
)

var (
	_ admission.Handler = &NsMutator{}
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
	log.Info("reaching HyperConvergedMutator.Handle")

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
	if req.Operation == admissionv1.Create {
		patches = getMutatePatchesOnCreate(v1hc, patches)
	}

	if len(patches) > 0 {
		return admission.Patched("mutated", patches...)
	}

	return admission.Allowed("")
}
