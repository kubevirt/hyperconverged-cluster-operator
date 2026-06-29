package validator

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"slices"
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

	networkaddonsv1 "github.com/kubevirt/cluster-network-addons-operator/pkg/apis/networkaddonsoperator/v1"
	kubevirtcorev1 "kubevirt.io/api/core/v1"
	cdiv1beta1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	sspv1beta3 "kubevirt.io/ssp-operator/api/v1beta3"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	hcov1fg "github.com/kubevirt/hyperconverged-cluster-operator/api/v1/featuregates"
	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/handlers"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/featuregatedetails"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/featuregates"
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
	logger      logr.Logger
	cli         client.Client
	namespace   string
	isOpenshift bool
	decoder     admission.Decoder
}

func NewWebhookHandler(logger logr.Logger, cli client.Client, decoder admission.Decoder, namespace string, isOpenshift bool) *WebhookHandler {
	return &WebhookHandler{
		logger:      logger.WithName(validatorV1Name),
		cli:         cli,
		namespace:   namespace,
		isOpenshift: isOpenshift,
		decoder:     decoder,
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
	obj := &hcov1.HyperConverged{}

	dryRun := req.DryRun != nil && *req.DryRun

	switch req.Operation {
	case admissionv1.Create:
		if err = wh.decoder.Decode(req, obj); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}

		return wh.validateCreate(logger, dryRun, obj)

	case admissionv1.Update:
		if err = wh.decoder.DecodeRaw(req.Object, obj); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}

		oldObj := &hcov1.HyperConverged{}
		if err = wh.decoder.DecodeRaw(req.OldObject, oldObj); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}

		return wh.validateUpdate(ctx, logger, dryRun, obj, oldObj)

	case admissionv1.Delete:
		if err = wh.decoder.DecodeRaw(req.OldObject, obj); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}

		return wh.validateDelete(ctx, logger, dryRun, obj)

	default:
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unknown operation request %q", req.Operation))
	}
}

func (wh *WebhookHandler) validateCreate(logger logr.Logger, dryrun bool, hc *hcov1.HyperConverged) admission.Response {
	logger.Info("Validating create", "name", hc.Name, "namespace:", hc.Namespace)

	if err := wh.validateCreateHyperConverged(hc); err != nil {
		return errToResponse(err)
	}

	err := wh.validateCreateComponents(hc)
	if err != nil {
		return errToResponse(err)
	}

	if !dryrun {
		tlssecprofile.SetHyperConvergedTLSSecurityProfile(hc.Spec.Security.TLSSecurityProfile)
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

	if err := checkOperands(ctx, wh.cli, logger, requested, wh.isOpenshift); err != nil {
		return errToResponse(err)
	}

	if !dryrun {
		tlssecprofile.SetHyperConvergedTLSSecurityProfile(requested.Spec.Security.TLSSecurityProfile)
	}

	return admission.Allowed("")
}

func (wh *WebhookHandler) validateDelete(ctx context.Context, logger logr.Logger, dryrun bool, hc *hcov1.HyperConverged) admission.Response {
	logger.Info("Validating delete", "name", hc.Name, "namespace", hc.Namespace)

	var err error
	for _, obj := range []client.Object{
		handlers.NewKubeVirtWithNameOnly(),
		handlers.NewCDIWithNameOnly(),
	} {
		_, err = hcoutil.EnsureDeleted(ctx, wh.cli, obj, hc.Name, logger, true, false, true)
		if err != nil {
			logger.Error(err, "Delete validation failed", "GVK", obj.GetObjectKind().GroupVersionKind())
			break
		}
	}

	if err == nil && !dryrun {
		tlssecprofile.SetHyperConvergedTLSSecurityProfile(nil)
	}

	return errToResponse(err)
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

func (wh *WebhookHandler) validateCreateComponents(hc *hcov1.HyperConverged) error {
	if _, err := handlers.NewKubeVirt(hc); err != nil {
		return err
	}

	if _, err := handlers.NewCDI(hc); err != nil {
		return err
	}

	if _, err := handlers.NewNetworkAddons(hc); err != nil {
		return err
	}

	if _, _, err := handlers.NewSSP(hc, true); err != nil {
		return err
	}

	return nil
}

func (wh *WebhookHandler) validateCertConfig(hc *hcov1.HyperConverged) error {
	minimalDuration := metav1.Duration{Duration: 10 * time.Minute}

	ccValues := make(map[string]time.Duration)
	ccValues["spec.certConfig.ca.duration"] = hc.Spec.Security.CertConfig.CA.Duration.Duration
	ccValues["spec.certConfig.ca.renewBefore"] = hc.Spec.Security.CertConfig.CA.RenewBefore.Duration
	ccValues["spec.certConfig.server.duration"] = hc.Spec.Security.CertConfig.Server.Duration.Duration
	ccValues["spec.certConfig.server.renewBefore"] = hc.Spec.Security.CertConfig.Server.RenewBefore.Duration

	for key, value := range ccValues {
		if value < minimalDuration.Duration {
			return fmt.Errorf("%v: value is too small", key)
		}
	}

	if hc.Spec.Security.CertConfig.CA.Duration.Duration < hc.Spec.Security.CertConfig.CA.RenewBefore.Duration {
		return errors.New("spec.certConfig.ca: duration is smaller than renewBefore")
	}

	if hc.Spec.Security.CertConfig.Server.Duration.Duration < hc.Spec.Security.CertConfig.Server.RenewBefore.Duration {
		return errors.New("spec.certConfig.server: duration is smaller than renewBefore")
	}

	if hc.Spec.Security.CertConfig.CA.Duration.Duration < hc.Spec.Security.CertConfig.Server.Duration.Duration {
		return errors.New("spec.certConfig: ca.duration is smaller than server.duration")
	}

	return nil
}

func (wh *WebhookHandler) validateDataImportCronTemplates(hc *hcov1.HyperConverged) error {

	for _, dict := range hc.Spec.WorkloadSources.DataImportCronTemplates {
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
	tlsSP := hc.Spec.Security.TLSSecurityProfile

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
	if hc.Spec.Virtualization.TuningPolicy == hcov1beta1.HyperConvergedHighBurstProfile { //nolint SA1019
		return newValidationWarning([]string{"spec.virtualization.tuningPolicy: the highBurst profile is not supported and ignored"})
	}
	return nil
}

func (wh *WebhookHandler) validateFeatureGatesOnCreate(hc *hcov1.HyperConverged) error {
	if err := validateMDevFeatureGateVsEnabledOnCreate(hc); err != nil {
		return err
	}

	fgMap := v1FGsToMap(hc.Spec.FeatureGates)

	warnings := wh.validateDeprecatedFeatureGates(fgMap, nil)

	if len(warnings) > 0 {
		return newValidationWarning(warnings)
	}

	return nil
}

func (wh *WebhookHandler) validateFeatureGatesOnUpdate(requested, exists *hcov1.HyperConverged) error {
	if err := validateMDevFeatureGateVsEnabledOnUpdate(requested, exists); err != nil {
		return err
	}

	reqFGMap := v1FGsToMap(requested.Spec.FeatureGates)
	oldFGMap := v1FGsToMap(exists.Spec.FeatureGates)

	warnings := wh.validateDeprecatedFeatureGates(reqFGMap, oldFGMap)

	if len(warnings) > 0 {
		return newValidationWarning(warnings)
	}

	return nil
}

func (wh *WebhookHandler) validateAffinity(hc *hcov1.HyperConverged) error {
	if hc.Spec.Deployment.NodePlacements == nil {
		return nil
	}

	nodePlacements := hc.Spec.Deployment.NodePlacements

	if nodePlacements.Workload != nil {
		if err := validateAffinity(nodePlacements.Workload.Affinity); err != nil {
			return fmt.Errorf("invalid workloads node placement affinity: %v", err.Error())
		}
	}

	if nodePlacements.Infra != nil {
		if err := validateAffinity(nodePlacements.Infra.Affinity); err != nil {
			return fmt.Errorf("invalid infra node placement affinity: %v", err.Error())
		}
	}

	return nil
}

const (
	fgv1Unknown            = "the %s featureGate is unknown and ignored."
	fgv1DeprecationWarning = "the %s featureGate deprecated and will be removed in a future release."

	disableMDevConfigurationFGName = "disableMDevConfiguration"
)

func v1DisableMDevFGIndex(fgs hcov1fg.HyperConvergedFeatureGates) int {
	return slices.IndexFunc(fgs, func(fg hcov1fg.FeatureGate) bool {
		return fg.Name == disableMDevConfigurationFGName
	})
}

func v1MDevEnabledExplicit(hc *hcov1.HyperConverged) (*bool, bool) {
	mdc := hc.Spec.Virtualization.MediatedDevicesConfiguration
	if mdc == nil || mdc.Enabled == nil {
		return nil, false
	}

	return mdc.Enabled, true
}

func validateMDevFeatureGateVsEnabledOnCreate(hc *hcov1.HyperConverged) error {
	if v1DisableMDevFGIndex(hc.Spec.FeatureGates) == -1 {
		return nil
	}

	enabled, present := v1MDevEnabledExplicit(hc)
	if !present {
		return nil
	}

	fgDisable := hc.Spec.FeatureGates.IsEnabled(disableMDevConfigurationFGName)
	if fgDisable == *enabled {
		return fmt.Errorf("spec.featureGates.disableMDevConfiguration and spec.virtualization.mediatedDevicesConfiguration.enabled contradict each other; disableMDevConfiguration must equal !enabled")
	}

	return nil
}

func v1MDevEnabledChanged(oldHC, newHC *hcov1.HyperConverged) bool {
	oldEnabled, oldPresent := v1MDevEnabledExplicit(oldHC)
	newEnabled, newPresent := v1MDevEnabledExplicit(newHC)

	if oldPresent != newPresent {
		return true
	}

	if !oldPresent {
		return false
	}

	return *oldEnabled != *newEnabled
}

func v1DisableMDevFGChanged(oldHC, newHC *hcov1.HyperConverged) bool {
	oldIdx := v1DisableMDevFGIndex(oldHC.Spec.FeatureGates)
	newIdx := v1DisableMDevFGIndex(newHC.Spec.FeatureGates)
	oldFGPresent := oldIdx != -1
	newFGPresent := newIdx != -1

	if oldFGPresent != newFGPresent {
		return true
	}

	if !oldFGPresent {
		return false
	}

	oldFGEnabled := oldHC.Spec.FeatureGates.IsEnabled(disableMDevConfigurationFGName)
	newFGEnabled := newHC.Spec.FeatureGates.IsEnabled(disableMDevConfigurationFGName)

	return oldFGEnabled != newFGEnabled
}

func validateMDevFeatureGateVsEnabledOnUpdate(requested, exists *hcov1.HyperConverged) error {
	fgChanged := v1DisableMDevFGChanged(exists, requested)
	enabledChanged := v1MDevEnabledChanged(exists, requested)

	if !fgChanged || !enabledChanged {
		return nil
	}

	if v1DisableMDevFGIndex(requested.Spec.FeatureGates) == -1 {
		return nil
	}

	enabled, present := v1MDevEnabledExplicit(requested)
	if !present {
		return nil
	}

	fgDisable := requested.Spec.FeatureGates.IsEnabled(disableMDevConfigurationFGName)
	if fgDisable == *enabled {
		return fmt.Errorf("spec.featureGates.disableMDevConfiguration and spec.virtualization.mediatedDevicesConfiguration.enabled were both changed and contradict each other; disableMDevConfiguration must equal !enabled")
	}

	return nil
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

		if oldEnabled, oldExists := oldFgMap[fgName]; !oldExists || enabled != oldEnabled {
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
	if err == nil {
		// Return allowed if everything succeeded.
		return admission.Allowed("")
	}

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

func v1FGsToMap(fgs hcov1fg.HyperConvergedFeatureGates) map[string]bool {
	m := map[string]bool{}
	for _, fg := range fgs {
		m[fg.Name] = ptr.Deref(fg.State, hcov1fg.Enabled) == hcov1fg.Enabled
	}

	return m
}

func checkOperands(ctx context.Context, cli client.Client, logger logr.Logger, requested *hcov1.HyperConverged, isOpenshift bool) error {
	if requested.DeletionTimestamp != nil { // do not check other components when removing HCO
		return nil
	}

	resources, err := getOperands(ctx, cli, isOpenshift)
	if err != nil {
		return err
	}

	toCtx, cancel := context.WithTimeout(ctx, updateDryRunTimeOut)
	defer cancel()

	eg, egCtx := xsync.WithContext(toCtx)
	opts := &client.UpdateOptions{DryRun: []string{metav1.DryRunAll}}

	for _, obj := range resources {
		func(o client.Object) {
			eg.Go(func() error {
				return updateOperatorCr(egCtx, cli, logger, requested, o, opts)
			})
		}(obj)
	}

	return eg.Wait()
}

func getOperands(ctx context.Context, cli client.Client, isOpenshift bool) ([]client.Object, error) {
	kv := handlers.NewKubeVirtWithNameOnly()
	err := cli.Get(ctx, client.ObjectKeyFromObject(kv), kv)
	if err != nil {
		return nil, err
	}

	cdi := handlers.NewCDIWithNameOnly()
	err = cli.Get(ctx, client.ObjectKeyFromObject(cdi), cdi)
	if err != nil {
		return nil, err
	}

	cna := handlers.NewNetworkAddonsWithNameOnly()
	err = cli.Get(ctx, client.ObjectKeyFromObject(cna), cna)
	if err != nil {
		return nil, err
	}

	resources := make([]client.Object, 0, 4)
	resources = append(resources, kv, cdi, cna)

	if isOpenshift {
		ssp := handlers.NewSSPWithNameOnly()
		err = cli.Get(ctx, client.ObjectKeyFromObject(ssp), ssp)
		if err != nil {
			return nil, err
		}

		resources = append(resources, ssp)
	}

	return resources, nil
}

const dryRunMaxRetries = 3

func updateOperatorCr(ctx context.Context, cli client.Client, logger logr.Logger, hc *hcov1.HyperConverged, exists client.Object, opts *client.UpdateOptions) error {
	for attempt := range dryRunMaxRetries {
		if attempt > 0 {
			if err := cli.Get(ctx, client.ObjectKeyFromObject(exists), exists); err != nil {
				logger.Error(err, "failed to re-fetch object for dry-run retry", "kind", exists.GetObjectKind())
				return err
			}
		}

		if err := applyDesiredSpec(hc, exists); err != nil {
			return err
		}

		err := cli.Update(ctx, exists, opts)
		if err == nil {
			logger.Info("dry-run update the object passed", "kind", exists.GetObjectKind())
			return nil
		}

		if !apierrors.IsConflict(err) {
			logger.Error(err, "failed to dry-run update the object", "kind", exists.GetObjectKind())
			return err
		}

		logger.Info("dry-run update conflict, retrying", "kind", exists.GetObjectKind(), "attempt", attempt+1)
	}

	return fmt.Errorf("failed to dry-run update %v after %d retries due to persistent conflicts", exists.GetObjectKind(), dryRunMaxRetries)
}

func applyDesiredSpec(hc *hcov1.HyperConverged, exists client.Object) error {
	switch existing := exists.(type) {
	case *kubevirtcorev1.KubeVirt:
		required, err := handlers.NewKubeVirt(hc)
		if err != nil {
			return err
		}
		required.Spec.DeepCopyInto(&existing.Spec)

	case *cdiv1beta1.CDI:
		required, err := handlers.NewCDI(hc)
		if err != nil {
			return err
		}
		required.Spec.DeepCopyInto(&existing.Spec)

	case *networkaddonsv1.NetworkAddonsConfig:
		required, err := handlers.NewNetworkAddons(hc)
		if err != nil {
			return err
		}
		required.Spec.DeepCopyInto(&existing.Spec)

	case *sspv1beta3.SSP:
		required, _, err := handlers.NewSSP(hc, true)
		if err != nil {
			return err
		}
		required.Spec.DeepCopyInto(&existing.Spec)
	}

	return nil
}
