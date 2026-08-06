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

	warnings, err := wh.validateCreateHyperConverged(hc)
	if err != nil {
		return errToResponse(err, warnings)
	}

	err = wh.validateCreateComponents(hc)
	if err != nil {
		return errToResponse(err, warnings)
	}

	if !dryrun {
		tlssecprofile.SetHyperConvergedTLSSecurityProfile(hc.Spec.Security.TLSSecurityProfile)
	}

	return errToResponse(nil, warnings)
}

func (wh *WebhookHandler) validateUpdate(ctx context.Context, logger logr.Logger, dryrun bool, requested *hcov1.HyperConverged, exists *hcov1.HyperConverged) admission.Response {
	logger.Info("Validating update", "name", requested.Name)

	// If no change is detected in the spec nor the annotations - nothing to validate
	if reflect.DeepEqual(exists.Spec, requested.Spec) &&
		reflect.DeepEqual(exists.Annotations, requested.Annotations) {
		return admission.Allowed("")
	}

	warnings, err := wh.validateUpdateHyperConverged(requested, exists)
	if err != nil {
		return errToResponse(err, warnings)
	}

	if err = checkOperands(ctx, wh.cli, logger, requested, wh.isOpenshift); err != nil {
		return errToResponse(err, warnings)
	}

	if !dryrun {
		tlssecprofile.SetHyperConvergedTLSSecurityProfile(requested.Spec.Security.TLSSecurityProfile)
	}

	return errToResponse(nil, warnings)
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

	return errToResponse(err, nil)
}

func (wh *WebhookHandler) validateHyperConverged(hc *hcov1.HyperConverged) ([]string, error) {
	var warnings []string
	if err := wh.validateCertConfig(hc); err != nil {
		return nil, err
	}

	if err := wh.validateDataImportCronTemplates(hc); err != nil {
		return nil, err
	}

	if err := wh.validateTLSSecurityProfiles(hc); err != nil {
		return nil, err
	}

	if err := wh.validateAffinity(hc); err != nil {
		return nil, err
	}

	if warn := wh.validateTuningPolicy(hc); len(warn) > 0 {
		warnings = append(warnings, warn...)
	}

	return warnings, nil
}

func (wh *WebhookHandler) validateCreateHyperConverged(hc *hcov1.HyperConverged) ([]string, error) {
	var warnings []string

	warn, err := wh.validateHyperConverged(hc)
	if len(warn) > 0 {
		warnings = append(warnings, warn...)
	}

	if warn = wh.validateFeatureGatesOnCreate(hc); len(warn) > 0 {
		warnings = append(warnings, warn...)
	}

	return warnings, err
}

func (wh *WebhookHandler) validateUpdateHyperConverged(hc, oldHC *hcov1.HyperConverged) ([]string, error) {
	var warnings []string

	warn, err := wh.validateHyperConverged(hc)
	if len(warn) > 0 {
		warnings = append(warnings, warn...)
	}

	if warn = wh.validateFeatureGatesOnUpdate(hc, oldHC); len(warn) > 0 {
		warnings = append(warnings, warn...)
	}

	return warnings, err
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

func (wh *WebhookHandler) validateTuningPolicy(hc *hcov1.HyperConverged) []string {
	if hc.Spec.Virtualization.TuningPolicy == hcov1beta1.HyperConvergedHighBurstProfile { //nolint SA1019
		return []string{"spec.virtualization.tuningPolicy: the highBurst profile is not supported and ignored"}
	}

	return nil
}

func (wh *WebhookHandler) validateFeatureGatesOnCreate(hc *hcov1.HyperConverged) []string {
	fgMap := v1FGsToMap(hc.Spec.FeatureGates)

	return wh.validateDeprecatedFeatureGates(fgMap, nil)
}

func (wh *WebhookHandler) validateFeatureGatesOnUpdate(requested, exists *hcov1.HyperConverged) []string {
	reqFGMap := v1FGsToMap(requested.Spec.FeatureGates)
	oldFGMap := v1FGsToMap(exists.Spec.FeatureGates)

	return wh.validateDeprecatedFeatureGates(reqFGMap, oldFGMap)
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
	fgv1DeprecationWarning = "the %s featureGate is deprecated and will be removed in a future release."
)

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
func validationResponseFromStatus(status metav1.Status, warnings []string) admission.Response {
	resp := admission.Response{
		AdmissionResponse: admissionv1.AdmissionResponse{
			Allowed: false,
			Result:  &status,
		},
	}

	if len(warnings) > 0 {
		resp = resp.WithWarnings(warnings...)
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

func errToResponse(err error, warnings []string) admission.Response {
	if err == nil {
		return withWarnings(admission.Allowed(""), warnings)
	}

	if apiStatus, ok := errors.AsType[*apierrors.StatusError](err); ok {
		return validationResponseFromStatus(apiStatus.Status(), warnings)
	}

	return withWarnings(admission.Denied(err.Error()), warnings)
}

func withWarnings(resp admission.Response, warnings []string) admission.Response {
	if len(warnings) > 0 {
		return resp.WithWarnings(warnings...)
	}

	return resp
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
