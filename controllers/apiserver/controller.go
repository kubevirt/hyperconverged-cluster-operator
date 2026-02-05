package apiserver

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/event"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	openshiftconfigv1 "github.com/openshift/api/config/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/tlssecprofile"
)

const operatorName = "apiServer-controller"

// ReconcileAPIServer reconciles APIServer to consume uptodate TLSSecurityProfile
type ReconcileAPIServer struct {
	client   client.Client
	notifier chan<- event.GenericEvent
}

var (
	logger = logf.Log.WithName(operatorName)
)

// Implement reconcile.Reconciler so the controller can reconcile objects
var _ reconcile.Reconciler = &ReconcileAPIServer{}

func (r *ReconcileAPIServer) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	logger := logr.FromContextOrDiscard(ctx).WithName("ReconcileAPIServer").WithValues("Request.Name", req.Name)
	logger.Info("Reconciling APIServer")

	modified, err := tlssecprofile.Refresh(ctx, r.client)

	if err != nil {
		return reconcile.Result{RequeueAfter: 60 * time.Second}, err
	}

	if modified {
		r.notifier <- event.GenericEvent{}
	}

	return reconcile.Result{}, nil
}

// RegisterReconciler creates a new HyperConverged Reconciler and registers it into manager.
func RegisterReconciler(mgr manager.Manager, notify chan<- event.GenericEvent) error {
	return add(mgr, newReconciler(mgr, notify))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager, notifier chan<- event.GenericEvent) reconcile.Reconciler {
	r := &ReconcileAPIServer{
		client:   mgr.GetClient(),
		notifier: notifier,
	}
	return r
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Setup a new controller to reconcile APIServer
	logger.Info("Setting up APIServer controller")
	c, err := controller.New(operatorName, mgr, controller.Options{
		Reconciler: r,
	})
	if err != nil {
		return err
	}

	// Watch APIServer and enqueue APIServer object key
	return c.Watch(source.Kind(mgr.GetCache(), client.Object(&openshiftconfigv1.APIServer{}), &handler.EnqueueRequestForObject{}))
}
