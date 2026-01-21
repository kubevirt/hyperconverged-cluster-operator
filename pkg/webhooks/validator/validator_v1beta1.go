package validator

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-logr/logr"
	openshiftconfigv1 "github.com/openshift/api/config/v1"
	admissionv1 "k8s.io/api/admission/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/utils/strings/slices"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
)

type V1Beta1WebhookHandler struct {
	logger      logr.Logger
	cli         client.Client
	namespace   string
	isOpenshift bool
	decoder     admission.Decoder
	v1Handler   *WebhookHandler
}

func NewV1Beta1WebhookHandler(logger logr.Logger, cli client.Client, decoder admission.Decoder, namespace string, isOpenshift bool, hcoTLSSecurityProfile *openshiftconfigv1.TLSSecurityProfile) *V1Beta1WebhookHandler {
	hcoTLSConfigCache = hcoTLSSecurityProfile
	return &V1Beta1WebhookHandler{
		logger:      logger,
		cli:         cli,
		namespace:   namespace,
		isOpenshift: isOpenshift,
		decoder:     decoder,
		v1Handler:   NewWebhookHandler(logger, cli, decoder, namespace, isOpenshift, hcoTLSSecurityProfile),
	}
}

func (wh *V1Beta1WebhookHandler) Handle(ctx context.Context, req admission.Request) admission.Response {

	ctx = admission.NewContextWithRequest(ctx, req)
	logger := logr.FromContextOrDiscard(ctx)

	// Get the object in the request
	obj := &hcov1beta1.HyperConverged{}

	dryRun := req.DryRun != nil && *req.DryRun

	var err error
	switch req.Operation {
	case admissionv1.Create:
		if err = wh.decoder.Decode(req, obj); err != nil {
			logger.Error(err, decodeErrorMsg)
			return admission.Errored(http.StatusBadRequest, errDecode)
		}

		err = wh.ValidateCreate(ctx, dryRun, obj)
	case admissionv1.Update:
		oldObj := &hcov1beta1.HyperConverged{}
		if err = wh.decoder.DecodeRaw(req.Object, obj); err != nil {
			logger.Error(err, decodeErrorMsg)
			return admission.Errored(http.StatusBadRequest, errDecode)
		}
		if err = wh.decoder.DecodeRaw(req.OldObject, oldObj); err != nil {
			logger.Error(err, decodeErrorMsg)
			return admission.Errored(http.StatusBadRequest, errDecode)
		}

		err = wh.ValidateUpdate(ctx, dryRun, obj, oldObj)
	case admissionv1.Delete:
		// In reference to PR: https://github.com/kubernetes/kubernetes/pull/76346
		// OldObject contains the object being deleted
		if err = wh.decoder.DecodeRaw(req.OldObject, obj); err != nil {
			logger.Error(err, decodeErrorMsg)
			return admission.Errored(http.StatusBadRequest, errDecode)
		}

		err = wh.ValidateDelete(ctx, dryRun, obj)
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

func (wh *V1Beta1WebhookHandler) ValidateCreate(ctx context.Context, dryrun bool, hc *hcov1beta1.HyperConverged) error {
	wh.logger.Info("Validating create", "name", hc.Name, "namespace:", hc.Namespace)

	if err := wh.validateMediatedDeviceTypes(hc); err != nil {
		return err
	}

	if err := wh.validateFeatureGatesOnCreate(hc); err != nil {
		return err
	}

	return wh.v1Handler.ValidateCreate(ctx, dryrun, hc)
}

// ValidateUpdate is the ValidateUpdate webhook implementation. It calls all the resources in parallel, to dry-run the
// upgrade.
func (wh *V1Beta1WebhookHandler) ValidateUpdate(ctx context.Context, dryrun bool, requested *hcov1beta1.HyperConverged, exists *hcov1beta1.HyperConverged) error {
	wh.logger.Info("Validating update", "name", requested.Name)

	if err := wh.validateMediatedDeviceTypes(requested); err != nil {
		return err
	}

	if err := wh.validateFeatureGatesOnUpdate(requested, exists); err != nil {
		return err
	}

	return wh.v1Handler.ValidateUpdate(ctx, dryrun, requested, exists)
}

func (wh *V1Beta1WebhookHandler) ValidateDelete(ctx context.Context, dryrun bool, hc *hcov1beta1.HyperConverged) error {
	return wh.v1Handler.ValidateDelete(ctx, dryrun, hc)
}

func (wh *V1Beta1WebhookHandler) validateMediatedDeviceTypes(hc *hcov1beta1.HyperConverged) error {
	mdc := hc.Spec.MediatedDevicesConfiguration
	if mdc != nil {
		if len(mdc.MediatedDevicesTypes) > 0 && len(mdc.MediatedDeviceTypes) > 0 && !slices.Equal(mdc.MediatedDevicesTypes, mdc.MediatedDeviceTypes) { //nolint SA1019
			return fmt.Errorf("mediatedDevicesTypes is deprecated, please use mediatedDeviceTypes instead")
		}
		for _, nmdc := range mdc.NodeMediatedDeviceTypes {
			if len(nmdc.MediatedDevicesTypes) > 0 && len(nmdc.MediatedDeviceTypes) > 0 && !slices.Equal(nmdc.MediatedDevicesTypes, nmdc.MediatedDeviceTypes) { //nolint SA1019
				return fmt.Errorf("mediatedDevicesTypes is deprecated, please use mediatedDeviceTypes instead")
			}
		}
	}
	return nil
}

const (
	fgMovedWarning       = "spec.featureGates.%[1]s is deprecated and ignored. It will removed in a future version; use spec.%[1]s instead"
	fgDeprecationWarning = "spec.featureGates.%s is deprecated and ignored. It will be removed in a future version;"
)

func (wh *V1Beta1WebhookHandler) validateFeatureGatesOnCreate(hc *hcov1beta1.HyperConverged) error {
	warnings := wh.validateDeprecatedFeatureGates(hc)
	warnings = wh.validateOldFGOnCreate(warnings, hc)

	if len(warnings) > 0 {
		return newValidationWarning(warnings)
	}

	return nil
}

func (wh *V1Beta1WebhookHandler) validateFeatureGatesOnUpdate(requested, exists *hcov1beta1.HyperConverged) error {
	warnings := wh.validateDeprecatedFeatureGates(requested)
	warnings = wh.validateOldFGOnUpdate(warnings, requested, exists)

	if len(warnings) > 0 {
		return newValidationWarning(warnings)
	}

	return nil
}

func (wh *V1Beta1WebhookHandler) validateDeprecatedFeatureGates(hc *hcov1beta1.HyperConverged) []string {
	var warnings []string

	//nolint:staticcheck
	if hc.Spec.FeatureGates.WithHostPassthroughCPU != nil {
		warnings = append(warnings, fmt.Sprintf(fgDeprecationWarning, "withHostPassthroughCPU"))
	}

	//nolint:staticcheck
	if hc.Spec.FeatureGates.DeployTektonTaskResources != nil {
		warnings = append(warnings, fmt.Sprintf(fgDeprecationWarning, "deployTektonTaskResources"))
	}

	//nolint:staticcheck
	if hc.Spec.FeatureGates.NonRoot != nil {
		warnings = append(warnings, fmt.Sprintf(fgDeprecationWarning, "nonRoot"))
	}

	//nolint:staticcheck
	if hc.Spec.FeatureGates.EnableManagedTenantQuota != nil {
		warnings = append(warnings, fmt.Sprintf(fgDeprecationWarning, "enableManagedTenantQuota"))
	}

	return warnings
}

func (*V1Beta1WebhookHandler) validateOldFGOnCreate(warnings []string, hc *hcov1beta1.HyperConverged) []string {
	//nolint:staticcheck
	if hc.Spec.FeatureGates.EnableApplicationAwareQuota != nil {
		warnings = append(warnings, fmt.Sprintf(fgMovedWarning, "enableApplicationAwareQuota"))
	}

	//nolint:staticcheck
	if hc.Spec.FeatureGates.EnableCommonBootImageImport != nil {
		warnings = append(warnings, fmt.Sprintf(fgMovedWarning, "enableCommonBootImageImport"))
	}

	//nolint:staticcheck
	if hc.Spec.FeatureGates.DeployVMConsoleProxy != nil {
		warnings = append(warnings, fmt.Sprintf(fgMovedWarning, "deployVmConsoleProxy"))
	}

	return warnings
}

func (*V1Beta1WebhookHandler) validateOldFGOnUpdate(warnings []string, hc, prevHC *hcov1beta1.HyperConverged) []string {
	//nolint:staticcheck
	if oldFGChanged(hc.Spec.FeatureGates.EnableApplicationAwareQuota, prevHC.Spec.FeatureGates.EnableApplicationAwareQuota) {
		warnings = append(warnings, fmt.Sprintf(fgMovedWarning, "enableApplicationAwareQuota"))
	}

	//nolint:staticcheck
	if oldFGChanged(hc.Spec.FeatureGates.EnableCommonBootImageImport, prevHC.Spec.FeatureGates.EnableCommonBootImageImport) {
		warnings = append(warnings, fmt.Sprintf(fgMovedWarning, "enableCommonBootImageImport"))
	}

	//nolint:staticcheck
	if oldFGChanged(hc.Spec.FeatureGates.DeployVMConsoleProxy, prevHC.Spec.FeatureGates.DeployVMConsoleProxy) {
		warnings = append(warnings, fmt.Sprintf(fgMovedWarning, "deployVmConsoleProxy"))
	}

	return warnings
}

func oldFGChanged(newFG, prevFG *bool) bool {
	return newFG != nil && (prevFG == nil || *newFG != *prevFG)
}
