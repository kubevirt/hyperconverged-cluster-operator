package validator

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	openshiftconfigv1 "github.com/openshift/api/config/v1"
	"github.com/samber/lo"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/component-helpers/scheduling/corev1/nodeaffinity"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
)

const (
	updateDryRunTimeOut = time.Second * 3

	validatorV1Name = "hyperConverged v1 validator"
)

type ValidationWarning struct {
	warnings []string
}

func newValidationWarning(warnings []string) *ValidationWarning {
	return &ValidationWarning{
		warnings: warnings,
	}
}

func (v *ValidationWarning) Error() string {
	return ""
}

func (v *ValidationWarning) Warnings() []string {
	return v.warnings
}

type WebhookHandler struct {
	logger         logr.Logger
	cli            client.Client
	namespace      string
	isOpenshift    bool
	decoder        admission.Decoder
	v1beta1Handler *WebhookV1Beta1Handler
}

func NewWebhookHandler(logger logr.Logger, cli client.Client, decoder admission.Decoder, namespace string, isOpenshift bool, v1beta1Handler *WebhookV1Beta1Handler) *WebhookHandler {
	return &WebhookHandler{
		logger:         logger.WithName(validatorV1Name),
		cli:            cli,
		namespace:      namespace,
		isOpenshift:    isOpenshift,
		decoder:        decoder,
		v1beta1Handler: v1beta1Handler,
	}
}

func (wh *WebhookHandler) Handle(ctx context.Context, req admission.Request) admission.Response {
	ctx = admission.NewContextWithRequest(ctx, req)
	logger, err := logr.FromContext(ctx)
	if err != nil {
		logger = wh.logger
	} else {
		logger = logger.WithName(validatorV1Name)
	}

	// Get the object in the request
	v1obj := &hcov1.HyperConverged{}

	dryRun := req.DryRun != nil && *req.DryRun

	switch req.Operation {
	case admissionv1.Create:
		if err = wh.decoder.Decode(req, v1obj); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}

		obj := &hcov1beta1.HyperConverged{}
		err = obj.ConvertFrom(v1obj)
		if err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}

		err = wh.v1beta1Handler.ValidateCreate(ctx, logger, dryRun, obj)
	case admissionv1.Update:
		v1OldObj := &hcov1.HyperConverged{}
		if err = wh.decoder.DecodeRaw(req.Object, v1obj); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		if err = wh.decoder.DecodeRaw(req.OldObject, v1OldObj); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}

		obj := &hcov1beta1.HyperConverged{}
		err = obj.ConvertFrom(v1obj)
		if err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}

		oldObj := &hcov1beta1.HyperConverged{}
		err = oldObj.ConvertFrom(v1OldObj)
		if err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}

		err = wh.v1beta1Handler.ValidateUpdate(ctx, logger, dryRun, obj, oldObj)
	case admissionv1.Delete:
		if err = wh.decoder.DecodeRaw(req.OldObject, v1obj); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}

		obj := &hcov1beta1.HyperConverged{}
		err = obj.ConvertFrom(v1obj)
		if err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}

		err = wh.v1beta1Handler.ValidateDelete(ctx, logger, dryRun, obj)
	default:
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unknown operation request %q", req.Operation))
	}

	// Check the error message first.
	if err != nil {
		var apiStatus apierrors.APIStatus
		if errors.As(err, &apiStatus) {
			return validationResponseFromStatus(false, apiStatus.Status())
		}

		var vw *ValidationWarning
		if errors.As(err, &vw) {
			return admission.Allowed("").WithWarnings(vw.Warnings()...)
		}

		return admission.Denied(err.Error())
	}

	// Return allowed if everything succeeded.
	return admission.Allowed("")
}

func hasRequiredHTTP2Ciphers(ciphers []string) bool {
	var requiredHTTP2Ciphers = []string{
		"ECDHE-RSA-AES128-GCM-SHA256",
		"ECDHE-ECDSA-AES128-GCM-SHA256",
	}

	// lo.Some returns true if at least 1 element of a subset is contained into a collection
	return lo.Some[string](requiredHTTP2Ciphers, ciphers)
}

// validationResponseFromStatus returns a response for admitting a request with provided Status object.
func validationResponseFromStatus(allowed bool, status metav1.Status) admission.Response {
	resp := admission.Response{
		AdmissionResponse: admissionv1.AdmissionResponse{
			Allowed: allowed,
			Result:  &status,
		},
	}
	return resp
}

func isValidTLSProtocolVersion(pv openshiftconfigv1.TLSProtocolVersion) bool {
	switch pv {
	case
		openshiftconfigv1.VersionTLS10,
		openshiftconfigv1.VersionTLS11,
		openshiftconfigv1.VersionTLS12,
		openshiftconfigv1.VersionTLS13:
		return true
	}
	return false
}

func validateAffinity(affinity *corev1.Affinity) error {
	if affinity == nil || affinity.NodeAffinity == nil || affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil {
		return nil
	}

	_, err := nodeaffinity.NewNodeSelector(affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution)

	return err
}
