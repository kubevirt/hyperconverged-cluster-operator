package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"path/filepath"

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
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
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
	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/cmd/cmdcommon"
	whapiservercontrollers "github.com/kubevirt/hyperconverged-cluster-operator/controllers/webhooks/apiserver-controller"
	bearertokencontroller "github.com/kubevirt/hyperconverged-cluster-operator/controllers/webhooks/bearer-token-controller"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/authorization"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/ownresources"
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
			TLSOpts:        []func(*tls.Config){cmdcommon.MutateTLSConfig},
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
			TLSOpts:  []func(*tls.Config){cmdcommon.MutateTLSConfig},
		}),
		Cache: getCacheOption(operatorNamespace, hcoutil.GetClusterInfo()),
	})
	cmdHelper.ExitOnError(err, "failed to create manager")

	// register pprof instrumentation if HCO_PPROF_ADDR is set
	cmdHelper.ExitOnError(cmdHelper.RegisterPPROFServer(mgr), "can't register pprof server")

	logger.Info("Registering Components.")

	ctx = signals.SetupSignalHandler()

	eventEmitter := hcoutil.GetEventEmitter()
	eventEmitter.Init(ownresources.GetPod(), ownresources.GetCSVRef(), mgr.GetEventRecorderFor(hcoutil.HyperConvergedName))

	err = mgr.AddHealthzCheck("ping", healthz.Ping)
	cmdHelper.ExitOnError(err, "unable to add health check")

	err = mgr.AddReadyzCheck("ready", healthz.Ping)
	cmdHelper.ExitOnError(err, "unable to add ready check")

	hcoCR := &hcov1beta1.HyperConverged{}
	hcoCR.Name = hcoutil.HyperConvergedName
	hcoCR.Namespace = operatorNamespace

	var hcoTLSSecurityProfile *openshiftconfigv1.TLSSecurityProfile
	err = apiClient.Get(ctx, client.ObjectKeyFromObject(hcoCR), hcoCR)
	if err != nil && !apierrors.IsNotFound(err) {
		cmdHelper.ExitOnError(err, "Cannot read existing HCO CR")
	} else {
		hcoTLSSecurityProfile = hcoCR.Spec.TLSSecurityProfile
	}

	logger.Info("Registering the APIServer reconciler")
	err = whapiservercontrollers.RegisterReconciler(mgr, ci)
	cmdHelper.ExitOnError(err, "Cannot register APIServer reconciler")

	logger.Info("Registering the Bearer Token reconciler")
	err = bearertokencontroller.RegisterReconciler(mgr, ci, eventEmitter)
	cmdHelper.ExitOnError(err, "Cannot register the Bearer Token reconciler")

	if err = webhooks.SetupWebhookWithManager(ctx, mgr, ci.IsOpenshift(), hcoTLSSecurityProfile); err != nil {
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
	if !ci.IsMonitoringAvailable() {
		return cache.Options{}
	}

	namespaceSelector := fields.Set{"metadata.namespace": operatorNamespace}.AsSelector()
	labelSelector := labels.Set{hcoutil.AppLabel: hcoutil.HCOWebhookName}.AsSelector()

	cacheOptions := cache.Options{
		ByObject: map[client.Object]cache.ByObject{
			&appsv1.Deployment{}: {
				Label: labels.Set{"name": hcoutil.HCOWebhookName}.AsSelector(),
				Field: namespaceSelector,
			},
			&corev1.Service{}: {
				Label: labelSelector,
				Field: namespaceSelector,
			},
			&corev1.Secret{}: {
				Label: labelSelector,
				Field: namespaceSelector,
			},
			&monitoringv1.ServiceMonitor{}: {
				Label: labelSelector,
				Field: namespaceSelector,
			},
		},
	}

	return cacheOptions
}
