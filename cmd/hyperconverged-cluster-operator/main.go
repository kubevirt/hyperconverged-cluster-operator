package main

import (
	"context"
	"fmt"
	"os"

	"sigs.k8s.io/controller-runtime/pkg/metrics/server"

	openshiftconfigv1 "github.com/openshift/api/config/v1"
	consolev1 "github.com/openshift/api/console/v1"
	imagev1 "github.com/openshift/api/image/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	openshiftroutev1 "github.com/openshift/api/route/v1"
	csvv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	operatorsapiv2 "github.com/operator-framework/api/pkg/operators/v2"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"

	networkaddonsv1 "github.com/kubevirt/cluster-network-addons-operator/pkg/apis/networkaddonsoperator/v1"
	"github.com/kubevirt/hyperconverged-cluster-operator/api"
	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/cmd/cmdcommon"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/hyperconverged"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
	kubevirtcorev1 "kubevirt.io/api/core/v1"
	cdiv1beta1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	mtqv1alpha1 "kubevirt.io/managed-tenant-quota/staging/src/kubevirt.io/managed-tenant-quota-api/pkg/apis/core/v1alpha1"
	sspv1beta2 "kubevirt.io/ssp-operator/api/v1beta2"
)

// Change below variables to serve metrics on different host or port.
var (
	logger               = logf.Log.WithName("hyperconverged-operator-cmd")
	cmdHelper            = cmdcommon.NewHelper(logger, "operator")
	resourcesSchemeFuncs = []func(*apiruntime.Scheme) error{
		api.AddToScheme,
		schedulingv1.AddToScheme,
		corev1.AddToScheme,
		appsv1.AddToScheme,
		rbacv1.AddToScheme,
		cdiv1beta1.AddToScheme,
		networkaddonsv1.AddToScheme,
		sspv1beta2.AddToScheme,
		csvv1alpha1.AddToScheme,
		admissionregistrationv1.AddToScheme,
		consolev1.Install,
		consolev1.Install,
		operatorv1.Install,
		openshiftconfigv1.Install,
		openshiftroutev1.Install,
		monitoringv1.AddToScheme,
		apiextensionsv1.AddToScheme,
		kubevirtcorev1.AddToScheme,
		coordinationv1.AddToScheme,
		operatorsapiv2.AddToScheme,
		imagev1.Install,
		mtqv1alpha1.AddToScheme,
	}
)

func main() {
	cmdHelper.InitiateCommand()

	operatorNamespace, err := hcoutil.GetOperatorNamespaceFromEnv()
	cmdHelper.ExitOnError(err, "can't get operator expected namespace")

	// Get a config to talk to the apiserver
	cfg, err := config.GetConfig()
	cmdHelper.ExitOnError(err, "can't load configuration")

	// Setup Scheme for all resources
	scheme := apiruntime.NewScheme()
	cmdHelper.AddToScheme(scheme, resourcesSchemeFuncs)

	ci := hcoutil.GetClusterInfo()

	// apiclient.New() returns a client without cache.
	// cache is not initialized before mgr.Start()
	// we need this because we need to interact with OperatorCondition
	apiClient, err := client.New(cfg, client.Options{
		Scheme: scheme,
	})
	cmdHelper.ExitOnError(err, "Cannot create a new API client")

	// Detect OpenShift version
	ctx := context.TODO()
	err = ci.Init(ctx, apiClient, logger)
	cmdHelper.ExitOnError(err, "Cannot detect cluster type")

	needLeaderElection := !ci.IsRunningLocally()

	// Create a new Cmd to provide shared dependencies and start components
	mgr, err := manager.New(cfg, getManagerOptions(operatorNamespace, needLeaderElection, ci.IsMonitoringAvailable(), ci.IsOpenshift(), scheme))
	cmdHelper.ExitOnError(err, "can't initiate manager")

	// register pprof instrumentation if HCO_PPROF_ADDR is set
	cmdHelper.ExitOnError(cmdHelper.RegisterPPROFServer(mgr), "can't register pprof server")

	logger.Info("Registering Components.")

	eventEmitter := hcoutil.GetEventEmitter()
	eventEmitter.Init(ci.GetPod(), ci.GetCSV(), mgr.GetEventRecorderFor(hcoutil.HyperConvergedName))

	err = mgr.AddHealthzCheck("ping", healthz.Ping)
	cmdHelper.ExitOnError(err, "unable to add health check")

	readyCheck := hcoutil.GetHcoPing()

	err = mgr.AddReadyzCheck("ready", readyCheck)
	cmdHelper.ExitOnError(err, "unable to add ready check")

	// Force OperatorCondition Upgradeable to False
	//
	// We have to at least default the condition to False or
	// OLM will use the Readiness condition via our readiness probe instead:
	// https://olm.operatorframework.io/docs/advanced-tasks/communicating-operator-conditions-to-olm/#setting-defaults
	//
	// We want to force it to False to ensure that the final decision about whether
	// the operator can be upgraded stays within the hyperconverged controller.
	logger.Info("Setting OperatorCondition.")
	upgradeableCondition, err := hcoutil.NewOperatorCondition(ci, apiClient, operatorsapiv2.Upgradeable)
	cmdHelper.ExitOnError(err, "Cannot create the Upgradeable Operator Condition")

	err = wait.ExponentialBackoff(retry.DefaultRetry, func() (bool, error) {
		err := upgradeableCondition.Set(ctx, metav1.ConditionFalse, hcoutil.UpgradeableInitReason, hcoutil.UpgradeableInitMessage)
		if err != nil {
			logger.Error(err, "Cannot set the status of the Upgradeable Operator Condition; "+err.Error())
		}
		return err == nil, nil
	})
	cmdHelper.ExitOnError(err, "Failed to set the status of the Upgradeable Operator Condition")

	// re-create the condition, this time with the final client
	upgradeableCondition, err = hcoutil.NewOperatorCondition(ci, mgr.GetClient(), operatorsapiv2.Upgradeable)
	cmdHelper.ExitOnError(err, "Cannot create Upgradeable Operator Condition")

	// Create a new reconciler
	if err := hyperconverged.RegisterReconciler(mgr, ci, upgradeableCondition); err != nil {
		logger.Error(err, "failed to register the HyperConverged controller")
		eventEmitter.EmitEvent(nil, corev1.EventTypeWarning, "InitError", "Unable to register HyperConverged controller; "+err.Error())
		os.Exit(1)
	}

	err = createPriorityClass(ctx, mgr)
	cmdHelper.ExitOnError(err, "Failed creating PriorityClass")

	logger.Info("Starting the Cmd.")
	eventEmitter.EmitEvent(nil, corev1.EventTypeNormal, "Init", "Starting the HyperConverged Pod")

	// Start the Cmd
	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		logger.Error(err, "Manager exited non-zero")
		eventEmitter.EmitEvent(nil, corev1.EventTypeWarning, "UnexpectedError", "HyperConverged crashed; "+err.Error())
		os.Exit(1)
	}
}

// Restricts the cache's ListWatch to specific fields/labels per GVK at the specified object to control the memory impact
// this is used to completely overwrite the NewCache function so all the interesting objects should be explicitly listed here
func getCacheOption(operatorNamespace string, isMonitoringAvailable, isOpenshift bool) cache.Options {
	namespaceSelector := fields.Set{"metadata.namespace": operatorNamespace}.AsSelector()
	labelSelector := labels.Set{hcoutil.AppLabel: hcoutil.HyperConvergedName}.AsSelector()
	labelSelectorForNamespace := labels.Set{hcoutil.KubernetesMetadataName: operatorNamespace}.AsSelector()

	cacheOptions := cache.Options{
		ByObject: map[client.Object]cache.ByObject{
			&hcov1beta1.HyperConverged{}:           {},
			&kubevirtcorev1.KubeVirt{}:             {},
			&cdiv1beta1.CDI{}:                      {},
			&networkaddonsv1.NetworkAddonsConfig{}: {},
			&sspv1beta2.SSP{}:                      {},
			&mtqv1alpha1.MTQ{}:                     {},
			&schedulingv1.PriorityClass{}: {
				Label: labels.SelectorFromSet(labels.Set{hcoutil.AppLabel: hcoutil.HyperConvergedName}),
			},
			&corev1.ConfigMap{}: {
				Label: labelSelector,
			},
			&corev1.Service{}: {
				Field: namespaceSelector,
			},
			&corev1.Endpoints{}: {
				Field: namespaceSelector,
			},
			&rbacv1.Role{}: {
				Label: labelSelector,
				Field: namespaceSelector,
			},
			&rbacv1.RoleBinding{}: {
				Label: labelSelector,
				Field: namespaceSelector,
			},
			&corev1.Namespace{}: {
				Label: labelSelectorForNamespace,
			},
			&appsv1.Deployment{}: {
				Label: labelSelector,
				Field: namespaceSelector,
			},
			&apiextensionsv1.CustomResourceDefinition{}: {},
		},
	}

	cacheOptionsByOjectForMonitoring := map[client.Object]cache.ByObject{
		&monitoringv1.ServiceMonitor{}: {
			Label: labelSelector,
			Field: namespaceSelector,
		},
		&monitoringv1.PrometheusRule{}: {
			Label: labelSelector,
			Field: namespaceSelector,
		},
	}

	cacheOptionsByOjectForOpenshift := map[client.Object]cache.ByObject{
		&openshiftroutev1.Route{}: {
			Field: namespaceSelector,
		},
		&imagev1.ImageStream{}: {
			Label: labelSelector,
		},
		&openshiftconfigv1.APIServer{}: {},
		&consolev1.ConsoleCLIDownload{}: {
			Label: labelSelector,
		},
		&consolev1.ConsoleQuickStart{}: {
			Label: labelSelector,
		},
		&consolev1.ConsolePlugin{}: {
			Label: labelSelector,
		},
	}

	if isMonitoringAvailable {
		for k, v := range cacheOptionsByOjectForMonitoring {
			cacheOptions.ByObject[k] = v
		}
	}
	if isOpenshift {
		for k, v := range cacheOptionsByOjectForOpenshift {
			cacheOptions.ByObject[k] = v
		}
	}

	return cacheOptions

}

func getManagerOptions(operatorNamespace string, needLeaderElection, isMonitoringAvailable, isOpenshift bool, scheme *apiruntime.Scheme) manager.Options {
	return manager.Options{
		Metrics: server.Options{
			BindAddress: fmt.Sprintf("%s:%d", hcoutil.MetricsHost, hcoutil.MetricsPort),
		},
		HealthProbeBindAddress: fmt.Sprintf("%s:%d", hcoutil.HealthProbeHost, hcoutil.HealthProbePort),
		ReadinessEndpointName:  hcoutil.ReadinessEndpointName,
		LivenessEndpointName:   hcoutil.LivenessEndpointName,
		LeaderElection:         needLeaderElection,
		// We set ConfigMapsLeasesResourceLock already in release-1.5 to migrate from configmaps to leases.
		// Since we used "configmapsleases" for over two years, spanning five minor releases,
		// any actively maintained operators are very likely to have a released version that uses
		// "configmapsleases". Therefore, having only "leases" should be safe now.
		LeaderElectionResourceLock: resourcelock.LeasesResourceLock,
		LeaderElectionID:           "hyperconverged-cluster-operator-lock",
		Cache:                      getCacheOption(operatorNamespace, isMonitoringAvailable, isOpenshift),
		Scheme:                     scheme,
	}
}

// KubeVirtPriorityClass is needed by virt-operator but OLM is not able to
// create it so we have to create it ASAP.
// When the user deletes HCO CR virt-operator should continue running
// so we are never supposed to delete it: because the priority class
// is completely opaque to OLM it will remain as a leftover on the cluster
func createPriorityClass(ctx context.Context, mgr manager.Manager) error {
	pc := operands.NewKubeVirtPriorityClass(&hcov1beta1.HyperConverged{})

	err := mgr.GetAPIReader().Get(ctx, client.ObjectKeyFromObject(pc), pc)
	if err != nil && apierrors.IsNotFound(err) {
		logger.Info("Creating KubeVirt PriorityClass")
		return mgr.GetClient().Create(ctx, pc, &client.CreateOptions{})
	}

	return err
}
