package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/go-logr/logr"
	openshiftconfigv1 "github.com/openshift/api/config/v1"
	csvv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/util/retry"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	networkaddonsv1 "github.com/kubevirt/cluster-network-addons-operator/pkg/apis/networkaddonsoperator/v1"
	kubevirtcorev1 "kubevirt.io/api/core/v1"
	cdiv1beta1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	sspv1beta3 "kubevirt.io/ssp-operator/api/v1beta3"

	"github.com/kubevirt/hyperconverged-cluster-operator/api"
	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/cmd/cmdcommon"
	whapiservercontrollers "github.com/kubevirt/hyperconverged-cluster-operator/controllers/webhooks/apiserver-controller"
	bearertokencontroller "github.com/kubevirt/hyperconverged-cluster-operator/controllers/webhooks/bearer-token-controller"
	conversionwebhookcontroller "github.com/kubevirt/hyperconverged-cluster-operator/controllers/webhooks/conversion-webhook-controller"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/authorization"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/ownresources"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/tlssecprofile"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/webhooks"
)

// Change below variables to serve metrics on different host or port.
var (
	logger               = logf.Log.WithName("hyperconverged-webhook-cmd")
	cmdHelper            = cmdcommon.NewHelper(logger, "webhook")
	resourcesSchemeFuncs = []func(*apiruntime.Scheme) error{
		api.AddToScheme,
		corev1.AddToScheme,
		appsv1.AddToScheme,
		cdiv1beta1.AddToScheme,
		networkaddonsv1.AddToScheme,
		sspv1beta3.AddToScheme,
		admissionregistrationv1.AddToScheme,
		openshiftconfigv1.Install,
		kubevirtcorev1.AddToScheme,
		openshiftconfigv1.Install,
		csvv1alpha1.AddToScheme,
		apiextensionsv1.AddToScheme,
		monitoringv1.AddToScheme,
	}
)

func main() {

	cmdHelper.InitiateCommand()

	operatorNamespace := hcoutil.GetOperatorNamespaceFromEnv()

	// Get a config to talk to the apiserver
	cfg, err := config.GetConfig()
	if err != nil {
		logger.Error(err, "")
		os.Exit(1)
	}

	// Make sure the certificates are mounted, this should be handled by the OLM
	webhookCertDir := webhooks.GetWebhookCertDir()
	certs := []string{filepath.Join(webhookCertDir, hcoutil.WebhookCertName), filepath.Join(webhookCertDir, hcoutil.WebhookKeyName)}
	for _, fname := range certs {
		if _, err := os.Stat(fname); err != nil {
			logger.Error(err, "CSV certificates were not found, skipping webhook initialization")
			cmdHelper.ExitOnError(err, "CSV certificates were not found, skipping webhook initialization")
		}
	}

	// Setup Scheme for all resources
	scheme := apiruntime.NewScheme()
	cmdHelper.AddToScheme(scheme, resourcesSchemeFuncs)

	// apiclient.New() returns a client without cache.
	// cache is not initialized before mgr.Start()
	// we need this because we need to interact with OperatorCondition
	apiClient, err := client.New(cfg, client.Options{
		Scheme: scheme,
	})
	cmdHelper.ExitOnError(err, "Cannot create a new API client")

	ci := hcoutil.GetClusterInfo()
	ctx := context.Background()
	err = cmdcommon.ClusterInitializations(ctx, apiClient, scheme, logger)
	cmdHelper.ExitOnError(err, "Cannot detect cluster type")

	// Create a new Cmd to provide shared dependencies and start components
	mgr, err := manager.New(cfg, manager.Options{
		Metrics: server.Options{
			SecureServing:  true,
			CertDir:        webhooks.GetWebhookCertDir(),
			CertName:       hcoutil.WebhookCertName,
			KeyName:        hcoutil.WebhookKeyName,
			BindAddress:    fmt.Sprintf("%s:%d", hcoutil.MetricsHost, hcoutil.MetricsPort),
			FilterProvider: authorization.HttpWithBearerToken,
			TLSOpts:        []func(*tls.Config){tlssecprofile.MutateTLSConfig},
		},
		HealthProbeBindAddress:     fmt.Sprintf("%s:%d", hcoutil.HealthProbeHost, hcoutil.HealthProbePort),
		ReadinessEndpointName:      hcoutil.ReadinessEndpointName,
		LivenessEndpointName:       hcoutil.LivenessEndpointName,
		LeaderElection:             !ci.IsRunningLocally(),
		LeaderElectionResourceLock: resourcelock.LeasesResourceLock,
		LeaderElectionID:           "hyperconverged-cluster-webhook-lock",
		Scheme:                     scheme,

		WebhookServer: webhook.NewServer(webhook.Options{
			CertDir:  webhooks.GetWebhookCertDir(),
			CertName: hcoutil.WebhookCertName,
			KeyName:  hcoutil.WebhookKeyName,
			Port:     hcoutil.WebhookPort,
			TLSOpts:  []func(*tls.Config){tlssecprofile.MutateTLSConfig},
		}),
		Cache: getCacheOption(operatorNamespace, hcoutil.GetClusterInfo()),
	})
	cmdHelper.ExitOnError(err, "failed to create manager")

	err = mgr.Add(manager.RunnableFunc(func(ctx context.Context) error {
		err := ensureAPIv1(ctx, apiClient, operatorNamespace, logger)
		if err != nil {
			logger.Error(err, "Failed to store the HyperConverged CR in v1 format")
			return fmt.Errorf("failed to store the HyperConverged CR in v1 format; %w", err)
		}

		return cmdcommon.SetHyperConvergedTLSProfile(ctx, operatorNamespace, apiClient)
	}))
	cmdHelper.ExitOnError(err, "failed to add runnable to manager")

	// register pprof instrumentation if HCO_PPROF_ADDR is set
	cmdHelper.ExitOnError(cmdHelper.RegisterPPROFServer(mgr), "can't register pprof server")

	logger.Info("Registering Components.")

	ctx = signals.SetupSignalHandler()

	eventEmitter := hcoutil.GetEventEmitter()
	//nolint:staticcheck
	eventEmitter.Init(ownresources.GetPod(), ownresources.GetCSVRef(), mgr.GetEventRecorderFor(hcoutil.HyperConvergedName))

	err = mgr.AddHealthzCheck("ping", healthz.Ping)
	cmdHelper.ExitOnError(err, "unable to add health check")

	err = mgr.AddReadyzCheck("ready", healthz.Ping)
	cmdHelper.ExitOnError(err, "unable to add ready check")

	logger.Info("Registering the APIServer reconciler")
	if ci.IsOpenshift() {
		_, err = tlssecprofile.Refresh(ctx, apiClient)
		if err != nil {
			logger.Error(err, "Cannot refresh TLS profile from the APIServer CR")
		}

		err = whapiservercontrollers.RegisterReconciler(mgr)
		cmdHelper.ExitOnError(err, "Cannot register APIServer reconciler")
	}

	logger.Info("Registering the Bearer Token reconciler")
	err = bearertokencontroller.RegisterReconciler(mgr, ci, eventEmitter)
	cmdHelper.ExitOnError(err, "Cannot register the Bearer Token reconciler")

	logger.Info("Registering the Conversion Webhook reconciler")
	err = conversionwebhookcontroller.RegisterReconciler(mgr)
	cmdHelper.ExitOnError(err, "Cannot register the Conversion Webhook reconciler")

	if err = webhooks.SetupWebhookWithManager(mgr, ci.IsOpenshift()); err != nil {
		logger.Error(err, "unable to create webhook", "webhook", "HyperConverged")
		eventEmitter.EmitEvent(nil, corev1.EventTypeWarning, "InitError", "Unable to create webhook")
		os.Exit(1)
	}

	logger.Info("Starting the Cmd.")
	eventEmitter.EmitEvent(nil, corev1.EventTypeNormal, "Init", "Starting the HyperConverged webhook Pod")
	// Start the Cmd
	if err = mgr.Start(ctx); err != nil {
		logger.Error(err, "Manager exited non-zero")
		eventEmitter.EmitEvent(nil, corev1.EventTypeWarning, "UnexpectedError", "HyperConverged crashed; "+err.Error())
		os.Exit(1)
	}
}

func getCacheOption(operatorNamespace string, ci hcoutil.ClusterInfo) cache.Options {
	if !ci.IsMonitoringAvailable() && !ci.IsOpenshift() {
		return cache.Options{}
	}

	objMap := map[client.Object]cache.ByObject{}
	if ci.IsMonitoringAvailable() {
		namespaceSelector := fields.Set{"metadata.namespace": operatorNamespace}.AsSelector()
		labelSelector := labels.Set{hcoutil.AppLabel: hcoutil.HCOWebhookName}.AsSelector()

		objMap[&appsv1.Deployment{}] = cache.ByObject{
			Label: labels.Set{"name": hcoutil.HCOWebhookName}.AsSelector(),
			Field: namespaceSelector,
		}

		objMap[&corev1.Service{}] = cache.ByObject{
			Label: labelSelector,
			Field: namespaceSelector,
		}

		objMap[&corev1.Secret{}] = cache.ByObject{
			Field: namespaceSelector,
		}

		objMap[&monitoringv1.ServiceMonitor{}] = cache.ByObject{
			Label: labelSelector,
			Field: namespaceSelector,
		}

		objMap[&apiextensionsv1.CustomResourceDefinition{}] = cache.ByObject{
			SyncPeriod: ptr.To(time.Minute * 5),
			Field:      fields.Set{"metadata.name": hcoutil.HyperConvergedCRDName}.AsSelector(),
		}
	}

	if ci.IsOpenshift() {
		objMap[&openshiftconfigv1.APIServer{}] = cache.ByObject{
			SyncPeriod: ptr.To(5 * time.Minute),
		}
	}

	return cache.Options{
		ByObject: objMap,
	}
}

// ensureAPIv1 makes sure the HyperConverged CR is stored in v1 API version format.
// This Based on the article in the Kubernetes documentation: https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definition-versioning/#upgrade-existing-objects-to-a-new-stored-version
// We are using option 2 from this article: re-write the HyperConverged CR in v1 API version format, then update the
// HyperConverged CRD status to only have v1 in its storedVersions field.
func ensureAPIv1(ctx context.Context, cli client.Client, namespace string, logger logr.Logger) error {
	hcCRD := &apiextensionsv1.CustomResourceDefinition{}

	logger.Info("Reading the HyperConverged CRD")
	err := cli.Get(ctx, client.ObjectKey{Name: hcoutil.HyperConvergedCRDName}, hcCRD)
	if err != nil {
		logger.Error(err, "Failed to read the HyperConverged CRD")
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("%s CRD not found", hcoutil.HyperConvergedCRDName)
		}
		return err
	}

	if !slices.Contains(hcCRD.Status.StoredVersions, hcov1.APIVersionV1) {
		return fmt.Errorf("unexpected stored API versions of %v in the %s CRD; missing %s", hcCRD.Status.StoredVersions, hcoutil.HyperConvergedCRDName, hcov1.APIVersionV1)
	}

	if len(hcCRD.Status.StoredVersions) == 1 {
		logger.Info("HyperConverged CRD is already up-to-date")
		return nil
	}

	if len(hcCRD.Status.StoredVersions) > 1 {
		if err = reStoreHCv1(ctx, cli, namespace, logger); err != nil {
			return err
		}

		const apiVersionPatch = `{"status":{"storedVersions":["v1"]}}`
		patchBytes := []byte(apiVersionPatch)

		attempt := 1
		return retry.RetryOnConflict(retry.DefaultRetry, func() error {
			defer func() {
				attempt++
			}()

			retryLogger := logger.WithValues("attempt", attempt)
			retryLogger.Info("Updating HyperConverged CRD to store only the v1 API version")
			err = cli.Status().Patch(ctx, hcCRD, client.RawPatch(types.StrategicMergePatchType, patchBytes))
			if err != nil {
				return fmt.Errorf("failed to patch HyperConverged CRD, to store only the v1 API version; %w", err)
			}

			retryLogger.Info("Successfully updated the HyperConverged CRD to store only the v1 API version")
			return nil
		})
	}

	return nil
}

func reStoreHCv1(ctx context.Context, cli client.Client, namespace string, logger logr.Logger) error {
	attempt := 1
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		defer func() {
			attempt++
		}()

		retryLogger := logger.WithValues("attempt", attempt)
		retryLogger.Info("reading the HyperConverged CR in API v1 format")
		hc, err := getHyperConvergedV1(ctx, cli, namespace, retryLogger)
		if err != nil {
			return err
		}

		if hc == nil {
			return nil
		}

		retryLogger.Info("re-storing the HyperConverged CR in API v1 format")
		err = cli.Update(ctx, hc, client.FieldValidation("Ignore"))
		if err != nil {
			return fmt.Errorf("failed to re-store the HyperConverged CR in API v1 format; %w", err)
		}

		retryLogger.Info("Successfully re-stored the HyperConverged CR in API v1 format")

		return nil
	})
}

func getHyperConvergedV1(ctx context.Context, cli client.Client, namespace string, retryLogger logr.Logger) (*hcov1.HyperConverged, error) {
	hc := &hcov1.HyperConverged{}
	hcKey := client.ObjectKey{Name: hcoutil.HyperConvergedName, Namespace: namespace}

	err := cli.Get(ctx, hcKey, hc)
	if err == nil {
		return hc, nil
	}

	if apierrors.IsNotFound(err) {
		retryLogger.Info("The HyperConverged CR does not exist, yet")
		return nil, nil
	}

	if _, isTypeErr := errors.AsType[*json.UnmarshalTypeError](err); !isTypeErr {
		return nil, fmt.Errorf("unknown error while fetching the HyperConverged CR as v1; %w", err)
	}

	retryLogger.Info("reading the HyperConverged CR in API v1 format failed. Trying v1beta1")
	hcv1beta1 := &hcov1beta1.HyperConverged{}
	err = cli.Get(ctx, hcKey, hcv1beta1)
	if err != nil {
		return nil, fmt.Errorf("failed to read the HyperConverged CR as v1beta1; %w", err)
	}

	hc = &hcov1.HyperConverged{}
	err = hcv1beta1.ConvertTo(hc)
	if err != nil {
		return nil, fmt.Errorf("failed to convert the HyperConverged CR from v1beta1 to v1; %w", err)
	}

	return hc, nil
}
