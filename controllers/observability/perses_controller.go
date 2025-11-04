package observability

import (
	"context"
	"fmt"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	persesv1alpha1 "github.com/rhobs/perses-operator/api/v1alpha1"

	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

var (
	persesLog = logf.Log.WithName("controller_observability_perses")
)

// PersesReconciler handles Perses dashboards, datasources and token secret.
type PersesReconciler struct {
	client.Client

	namespace string
	events    chan event.GenericEvent
	owner     metav1.OwnerReference

	// Cached, parsed Perses assets loaded from the image (read once per process)
	assetsOnce        sync.Once
	cachedDashboards  []map[string]any
	datasourcesOnce   sync.Once
	cachedDatasources []map[string]any
}

func (r *PersesReconciler) Reconcile(ctx context.Context, _ reconcile.Request) (reconcile.Result, error) {
	persesLog.Info("Reconciling Perses Observability")

	var errors []error

	// Refresh managed dashboard list from assets and env on each run (cheap and safe)
	r.updateManagedDashboards()

	if err := r.ReconcilePersesResources(ctx); err != nil {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		err := fmt.Errorf("perses reconciliation failed: %v", errors)
		persesLog.Error(err, "Perses reconciliation failed")
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

// SetupPersesWithManager registers the Perses controller with the manager.
func SetupPersesWithManager(mgr manager.Manager, ownerRef metav1.OwnerReference) error {
	persesLog.Info("Setting up Perses controller")

	namespace := util.GetOperatorNamespaceFromEnv()

	// Register Perses types so typed listing works (guard nil scheme in tests)
	if sch := mgr.GetScheme(); sch != nil {
		_ = persesv1alpha1.AddToScheme(sch)
	}

	r := NewPersesReconciler(mgr, namespace, ownerRef)
	r.startEventLoop()

	c, err := controller.New("observability-perses", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch dashboards with predicate on managed list
	if err = c.Watch(
		source.Kind[*persesv1alpha1.PersesDashboard](mgr.GetCache(), &persesv1alpha1.PersesDashboard{},
			&handler.TypedEnqueueRequestForObject[*persesv1alpha1.PersesDashboard]{},
			dashboardPredicate,
		),
	); err != nil {
		return err
	}

	// Watch the single datasource we manage
	if err = c.Watch(
		source.Kind[*persesv1alpha1.PersesDatasource](mgr.GetCache(), &persesv1alpha1.PersesDatasource{},
			&handler.TypedEnqueueRequestForObject[*persesv1alpha1.PersesDatasource]{},
			datasourcePredicate,
		),
	); err != nil {
		return err
	}

	// Trigger startup reconcile
	if err = c.Watch(source.Channel(r.events, &handler.EnqueueRequestForObject{})); err != nil {
		return err
	}
	return nil
}

func NewPersesReconciler(mgr manager.Manager, namespace string, ownerRef metav1.OwnerReference) *PersesReconciler {
	return &PersesReconciler{
		Client:    mgr.GetClient(),
		namespace: namespace,
		events:    make(chan event.GenericEvent, 1),
		owner:     ownerRef,
	}
}

func (r *PersesReconciler) startEventLoop() {
	ticker := time.NewTicker(periodicity)

	go func() {
		r.events <- event.GenericEvent{
			Object: &metav1.PartialObjectMetadata{},
		}
		for range ticker.C {
			r.events <- event.GenericEvent{
				Object: &metav1.PartialObjectMetadata{},
			}
		}
	}()
}

// updateManagedDashboards rebuilds the managed dashboard allowlist from embedded assets and env.
func (r *PersesReconciler) updateManagedDashboards() {
	// Start from the default allowlist we control
	names := make([]string, 0, len(defaultManagedDashboardNames)+4)
	names = append(names, defaultManagedDashboardNames...)
	setManagedDashboards(names)
}
