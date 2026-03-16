package validator

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/go-logr/logr"
	openshiftconfigv1 "github.com/openshift/api/config/v1"
	"github.com/samber/lo"
	xsync "golang.org/x/sync/errgroup"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/component-helpers/scheduling/corev1/nodeaffinity"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	hcov1fg "github.com/kubevirt/hyperconverged-cluster-operator/api/v1/featuregates"
	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/handlers"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/featuregatedetails"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/featuregates"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/nodeinfo"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/tlssecprofile"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
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

		return wh.validateCreate(logger, dryRun, v1obj)

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

func (wh *WebhookHandler) validateCreate(logger logr.Logger, dryrun bool, hc *hcov1.HyperConverged) admission.Response {
	logger.Info("Validating create", "name", hc.Name, "namespace:", hc.Namespace)

	if err := wh.validateCreateHyperConverged(hc); err != nil {
		return errToResponse(err)
	}

	v1beta1HC := &hcov1beta1.HyperConverged{}
	err := v1beta1HC.ConvertFrom(hc)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	err = wh.validateCreateComponents(v1beta1HC)
	if err != nil {
		return errToResponse(err)
	}

	if !dryrun {
		tlssecprofile.SetHyperConvergedTLSSecurityProfile(hc.Spec.TLSSecurityProfile)
	}

	return admission.Allowed("")
}

func (wh *WebhookHandler) validateUpdate(ctx context.Context, logger logr.Logger, dryrun bool, requested *hcov1.HyperConverged, exists *hcov1.HyperConverged) admission.Response {
	logger.Info("Validating update", "name", requested.Name)

	// If no change is detected in the spec nor the annotations - nothing to validate
	if reflect.DeepEqual(exists.Spec, requested.Spec) &&
		reflect.DeepEqual(exists.Annotations, requested.Annotations) {
		return admission.Allowed("")
	}

	if err := wh.validateUpdateHyperConverged(requested, exists); err != nil {
		return errToResponse(err)
	}

	v1beta1Req := &hcov1beta1.HyperConverged{}
	err := v1beta1Req.ConvertFrom(requested)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	v1beta1Old := &hcov1beta1.HyperConverged{}
	err = v1beta1Old.ConvertFrom(exists)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	err = wh.dryRunUpdateComponents(ctx, logger, v1beta1Req, requested)
	if err != nil {
		return errToResponse(err)
	}

	if !dryrun {
		tlssecprofile.SetHyperConvergedTLSSecurityProfile(requested.Spec.TLSSecurityProfile)
	}

	return admission.Allowed("")
}

func (wh *WebhookHandler) validateHyperConverged(hc *hcov1.HyperConverged) error {
	if err := wh.validateCertConfig(hc); err != nil {
		return err
	}

	if err := wh.validateDataImportCronTemplates(hc); err != nil {
		return err
	}

	if err := wh.validateTLSSecurityProfiles(hc); err != nil {
		return err
	}

	if err := wh.validateTuningPolicy(hc); err != nil {
		return err
	}

	if err := wh.validateAffinity(hc); err != nil {
		return err
	}

	return nil
}

func (wh *WebhookHandler) validateCreateHyperConverged(hc *hcov1.HyperConverged) error {
	if err := wh.validateHyperConverged(hc); err != nil {
		return err
	}

	if err := wh.validateFeatureGatesOnCreate(hc); err != nil {
		return err
	}

	return nil
}

func (wh *WebhookHandler) validateUpdateHyperConverged(hc, oldHC *hcov1.HyperConverged) error {
	if err := wh.validateHyperConverged(hc); err != nil {
		return err
	}

	if err := wh.validateFeatureGatesOnUpdate(hc, oldHC); err != nil {
		return err
	}

	return nil
}

func (wh *WebhookHandler) validateCreateComponents(v1beta1HC *hcov1beta1.HyperConverged) error {
	if _, err := handlers.NewKubeVirt(v1beta1HC); err != nil {
		return err
	}

	if _, err := handlers.NewCDI(v1beta1HC); err != nil {
		return err
	}

	if _, err := handlers.NewNetworkAddons(v1beta1HC); err != nil {
		return err
	}

	if _, _, err := handlers.NewSSP(v1beta1HC); err != nil {
		return err
	}

	return nil
}

func (wh *WebhookHandler) dryRunUpdateComponents(ctx context.Context, logger logr.Logger, v1beta1Req *hcov1beta1.HyperConverged, requested *hcov1.HyperConverged) error {
	kv, cdi, cna, err := wh.v1beta1Handler.getOperands(ctx, v1beta1Req)
	if err != nil {
		return err
	}

	toCtx, cancel := context.WithTimeout(ctx, updateDryRunTimeOut)
	defer cancel()

	eg, egCtx := xsync.WithContext(toCtx)
	opts := &client.UpdateOptions{DryRun: []string{metav1.DryRunAll}}

	resources := []client.Object{
		kv,
		cdi,
		cna,
	}

	if wh.isOpenshift {
		origGetControlPlaneArchitectures := nodeinfo.GetControlPlaneArchitectures
		origGetWorkloadsArchitectures := nodeinfo.GetWorkloadsArchitectures
		defer func() {
			nodeinfo.GetControlPlaneArchitectures = origGetControlPlaneArchitectures
			nodeinfo.GetWorkloadsArchitectures = origGetWorkloadsArchitectures
		}()

		nodeinfo.GetControlPlaneArchitectures = func() []string {
			return requested.Status.NodeInfo.ControlPlaneArchitectures
		}
		nodeinfo.GetWorkloadsArchitectures = func() []string {
			return requested.Status.NodeInfo.WorkloadsArchitectures
		}

		ssp, _, err := handlers.NewSSP(v1beta1Req)
		if err != nil {
			return err
		}
		resources = append(resources, ssp)
	}

	for _, obj := range resources {
		func(o client.Object) {
			eg.Go(func() error {
				return wh.v1beta1Handler.updateOperatorCr(egCtx, logger, v1beta1Req, o, opts)
			})
		}(obj)
	}

	return eg.Wait()
}

func (wh *WebhookHandler) validateCertConfig(hc *hcov1.HyperConverged) error {
	minimalDuration := metav1.Duration{Duration: 10 * time.Minute}

	ccValues := make(map[string]time.Duration)
	ccValues["spec.certConfig.ca.duration"] = hc.Spec.CertConfig.CA.Duration.Duration
	ccValues["spec.certConfig.ca.renewBefore"] = hc.Spec.CertConfig.CA.RenewBefore.Duration
	ccValues["spec.certConfig.server.duration"] = hc.Spec.CertConfig.Server.Duration.Duration
	ccValues["spec.certConfig.server.renewBefore"] = hc.Spec.CertConfig.Server.RenewBefore.Duration

	for key, value := range ccValues {
		if value < minimalDuration.Duration {
			return fmt.Errorf("%v: value is too small", key)
		}
	}

	if hc.Spec.CertConfig.CA.Duration.Duration < hc.Spec.CertConfig.CA.RenewBefore.Duration {
		return errors.New("spec.certConfig.ca: duration is smaller than renewBefore")
	}

	if hc.Spec.CertConfig.Server.Duration.Duration < hc.Spec.CertConfig.Server.RenewBefore.Duration {
		return errors.New("spec.certConfig.server: duration is smaller than renewBefore")
	}

	if hc.Spec.CertConfig.CA.Duration.Duration < hc.Spec.CertConfig.Server.Duration.Duration {
		return errors.New("spec.certConfig: ca.duration is smaller than server.duration")
	}

	return nil
}

func (wh *WebhookHandler) validateDataImportCronTemplates(hc *hcov1.HyperConverged) error {

	for _, dict := range hc.Spec.DataImportCronTemplates {
		val, ok := dict.Annotations[hcoutil.DataImportCronEnabledAnnotation]
		val = strings.ToLower(val)
		if ok && val != "false" && val != "true" {
			return fmt.Errorf(`the %s annotation of a dataImportCronTemplate must be either "true" or "false"`, hcoutil.DataImportCronEnabledAnnotation)
		}

		enabled := !ok || val == "true"

		if enabled && dict.Spec == nil {
			return fmt.Errorf("dataImportCronTemplate spec is empty for an enabled DataImportCronTemplate")
		}
	}

	return nil
}

func (wh *WebhookHandler) validateTLSSecurityProfiles(hc *hcov1.HyperConverged) error {
	tlsSP := hc.Spec.TLSSecurityProfile

	if tlsSP == nil {
		return nil
	}

	if tlsSP.Custom == nil {
		if tlsSP.Type == openshiftconfigv1.TLSProfileCustomType {
			return fmt.Errorf("missing required field spec.tlsSecurityProfile.custom when type is Custom")
		}
		return nil
	}

	if !isValidTLSProtocolVersion(tlsSP.Custom.MinTLSVersion) {
		return fmt.Errorf("invalid value for spec.tlsSecurityProfile.custom.minTLSVersion: %q", tlsSP.Custom.MinTLSVersion)
	}

	if tlsSP.Custom.MinTLSVersion < openshiftconfigv1.VersionTLS13 && !hasRequiredHTTP2Ciphers(tlsSP.Custom.Ciphers) {
		return fmt.Errorf("http2: TLSConfig.CipherSuites is missing an HTTP/2-required AES_128_GCM_SHA256 cipher (need at least one of ECDHE-RSA-AES128-GCM-SHA256 or ECDHE-ECDSA-AES128-GCM-SHA256)")
	} else if tlsSP.Custom.MinTLSVersion == openshiftconfigv1.VersionTLS13 && len(tlsSP.Custom.Ciphers) > 0 {
		return fmt.Errorf("custom ciphers cannot be selected when minTLSVersion is VersionTLS13")
	}

	return nil
}

func (wh *WebhookHandler) validateTuningPolicy(hc *hcov1.HyperConverged) error {
	if hc.Spec.TuningPolicy == hcov1.HyperConvergedHighBurstProfile { //nolint SA1019
		return newValidationWarning([]string{"spec.tuningPolicy: the highBurst profile is deprecated as of v1.16.0 and will be removed in a future release"})
	}
	return nil
}

func (wh *WebhookHandler) validateFeatureGatesOnCreate(hc *hcov1.HyperConverged) error {
	fgMap := v1FGsToMap(hc.Spec.FeatureGates)

	warnings := wh.validateDeprecatedFeatureGates(fgMap, nil)

	if len(warnings) > 0 {
		return newValidationWarning(warnings)
	}

	return nil
}

func (wh *WebhookHandler) validateFeatureGatesOnUpdate(requested, exists *hcov1.HyperConverged) error {
	reqFGMap := v1FGsToMap(requested.Spec.FeatureGates)
	oldFGMap := v1FGsToMap(exists.Spec.FeatureGates)

	warnings := wh.validateDeprecatedFeatureGates(reqFGMap, oldFGMap)

	if len(warnings) > 0 {
		return newValidationWarning(warnings)
	}

	return nil
}

func (wh *WebhookHandler) validateAffinity(hc *hcov1.HyperConverged) error {
	if hc.Spec.NodePlacements == nil {
		return nil
	}

	if hc.Spec.NodePlacements.Workload != nil {
		if err := validateAffinity(hc.Spec.NodePlacements.Workload.Affinity); err != nil {
			return fmt.Errorf("invalid workloads node placement affinity: %v", err.Error())
		}
	}

	if hc.Spec.NodePlacements.Infra != nil {
		if err := validateAffinity(hc.Spec.NodePlacements.Infra.Affinity); err != nil {
			return fmt.Errorf("invalid infra node placement affinity: %v", err.Error())
		}
	}

	return nil
}

const (
	fgv1Unknown            = "the %s featureGate is unknown and ignored."
	fgv1MovedWarning       = "the %s featureGate is deprecated and ignored. use %s field instead"
	fgv1DeprecationWarning = "the %s featureGate deprecated and ignored."
)

var movedFGs = map[string]string{
	"enableApplicationAwareQuota": "spec.enableApplicationAwareQuota",
	"enableCommonBootImageImport": "spec.enableCommonBootImageImport",
	"deployVmConsoleProxy":        "spec.deployVmConsoleProxy",
}

func (wh *WebhookHandler) validateDeprecatedFeatureGates(fgMap, oldFgMap map[string]bool) []string {
	var warnings []string

	for fgName, enabled := range fgMap {
		phase, exists := featuregatedetails.GetFeatureGatePhase(fgName)
		if !exists {
			warnings = append(warnings, fmt.Sprintf(fgv1Unknown, fgName))
			continue
		}

		if phase != featuregates.PhaseDeprecated {
			continue
		}

		if newFiled, ok := movedFGs[fgName]; ok {
			if oldEnabled, oldExists := oldFgMap[fgName]; !oldExists || enabled != oldEnabled {
				warnings = append(warnings, fmt.Sprintf(fgv1MovedWarning, fgName, newFiled))
			}
		} else {
			warnings = append(warnings, fmt.Sprintf(fgv1DeprecationWarning, fgName))
		}
	}

	return warnings
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

func errToResponse(err error) admission.Response {
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

func v1FGsToMap(fgs hcov1fg.HyperConvergedFeatureGates) map[string]bool {
	m := map[string]bool{}
	for _, fg := range fgs {
		m[fg.Name] = ptr.Deref(fg.State, hcov1fg.Enabled) == hcov1fg.Enabled
	}

	return m
}
