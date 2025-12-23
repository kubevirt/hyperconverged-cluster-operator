package bearer_token_controller

// the bearer_token_controller package contains the ReconcileWHBearerToken struct
// which is used to reconcile bearer tokens in the context the HyperConverged cluster webhook.
// The webhook provides a bearer token secret, to be used by Prometheus to scrape metrics from the
// HyperConverged cluster webhook pod.
//
// The reconciler makes sure the bearer token secret is created and updated as needed, as well as a
// Service and a ServiceMonitor resources that points to the webhook pod and uses the bearer token
// secret for authentication.
import (
	"context"
	"time"

	"github.com/go-logr/logr"
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/alerts"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

const (
	requeueDurationForNextRequest = time.Minute * 5
	requeueDurationForError       = time.Millisecond * 100
)

var (
	log = logf.Log.WithName("bearer-token-controller")
)

// RegisterReconciler creates a new WH Bearer Token Reconciler and registers it into manager.
func RegisterReconciler(mgr manager.Manager, ci hcoutil.ClusterInfo, ee hcoutil.EventEmitter) error {
	if ci.IsMonitoringAvailable() {
		return add(mgr, newReconciler(mgr, ci, ee))
	}

	mgr.GetLogger().Info("The cluster does not have monitoring installed. Skipping the registration of the Bearer Token reconciler")
	return nil
}

type ReconcileWHBearerToken struct {
	metricReconciler *alerts.MonitoringReconciler
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager, ci hcoutil.ClusterInfo, ee hcoutil.EventEmitter) reconcile.Reconciler {
	metricsReconciler := alerts.CreateMonitoringReconciler(ci, mgr.GetClient(), ee, mgr.GetScheme(), false, getReconcilers)

	r := &ReconcileWHBearerToken{
		metricReconciler: metricsReconciler,
	}

	return r
}

// Add creates a new ReconcileWHBearerToken and adds it to the manager.
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("wb-bearer-token-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	srcs := []source.Source{
		source.Kind[*appsv1.Deployment](
			mgr.GetCache(),
			&appsv1.Deployment{},
			&handler.TypedEnqueueRequestForObject[*appsv1.Deployment]{},
			newPredicate[*appsv1.Deployment](), getDeployPredicate(),
		),
		source.Kind[*corev1.Service](
			mgr.GetCache(),
			&corev1.Service{},
			&handler.TypedEnqueueRequestForObject[*corev1.Service]{},
			newPredicate[*corev1.Service](), servicePredicate,
		),
		source.Kind[*corev1.Secret](
			mgr.GetCache(),
			&corev1.Secret{},
			&handler.TypedEnqueueRequestForObject[*corev1.Secret]{},
			newPredicate[*corev1.Secret](), secretPredicate,
		),
		source.Kind[*promv1.ServiceMonitor](
			mgr.GetCache(),
			&promv1.ServiceMonitor{},
			&handler.TypedEnqueueRequestForObject[*promv1.ServiceMonitor]{},
			newPredicate[*promv1.ServiceMonitor](), smPredicate,
		),
	}

	for _, src := range srcs {
		err = c.Watch(src)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *ReconcileWHBearerToken) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	reqLogger := getLogger(ctx, request)

	reqLogger.Info("Reconciling WH Bearer Token")
	req := common.NewHcoRequest(ctx, request, reqLogger, false, true)

	if err := r.metricReconciler.Reconcile(req, true); err != nil {
		return reconcile.Result{RequeueAfter: requeueDurationForError}, err
	}

	return reconcile.Result{RequeueAfter: requeueDurationForNextRequest}, nil // this will cause the reconciler to run every 5 minutes
}

func getReconcilers(_ hcoutil.ClusterInfo, namespace string, owner metav1.OwnerReference) []alerts.MetricReconciler {
	refresher := alerts.NewRefresher()

	reconcilers := []alerts.MetricReconciler{
		newWHMetricServiceReconciler(namespace, owner),
		newWHSecretReconciler(namespace, owner, refresher),
		newWHServiceMonitorReconciler(namespace, owner, refresher),
	}

	return reconcilers
}

func newPredicate[T client.Object]() predicate.TypedPredicate[T] {
	return predicate.Or[T](
		predicate.TypedGenerationChangedPredicate[T]{},
		predicate.TypedLabelChangedPredicate[T]{},
	)
}

func getLogger(ctx context.Context, request reconcile.Request) logr.Logger {
	l, err := logr.FromContext(ctx)
	if err != nil {
		return log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	}

	return l
}

var (
	servicePredicate = predicate.NewTypedPredicateFuncs[*corev1.Service](func(service *corev1.Service) bool {
		return service.Name == serviceName
	})

	secretPredicate = predicate.NewTypedPredicateFuncs[*corev1.Secret](func(secret *corev1.Secret) bool {
		return secret.Name == secretName
	})

	smPredicate = predicate.NewTypedPredicateFuncs[*promv1.ServiceMonitor](func(sm *promv1.ServiceMonitor) bool {
		return sm.Name == serviceName
	})
)

func getDeployPredicate() predicate.TypedPredicate[*appsv1.Deployment] {
	deployment := hcoutil.GetClusterInfo().GetDeployment()
	if deployment == nil {
		return nil
	}

	return predicate.NewTypedPredicateFuncs[*appsv1.Deployment](func(deploy *appsv1.Deployment) bool {
		return deploy.Name == deployment.Name
	})
}
