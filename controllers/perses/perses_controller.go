package perses

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-logr/logr"
	persesv1alpha1 "github.com/rhobs/perses-operator/api/v1alpha1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

const (
	dashboardReqType   = "dashboard"
	dashboardReqSufix  = "-" + dashboardReqType
	datasourceReqType  = "datasource"
	datasourceReqSufix = "-" + datasourceReqType
	startupReqType     = "startup"
	unknownReqType     = "unknown"
)

var (
	persesLog   = logf.Log.WithName("controller_observability_perses")
	randomSufix = "-" + string(uuid.NewUUID())
)

// test seam for availability checks
var checkPersesAvailable = hcoutil.IsPersesAvailable

type PersesReconciler struct {
	client.Client

	namespace string
	events    chan event.GenericEvent
	owner     metav1.OwnerReference

	cachedDashboards map[string]persesv1alpha1.PersesDashboard
	cachedDatasource *persesv1alpha1.PersesDatasource
}

func (r *PersesReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	if !hcoutil.IsPersesAvailable(ctx, r.Client) {
		return reconcile.Result{}, nil
	}

	reqLog := logr.FromContextOrDiscard(ctx)
	reqType, reqName := resolveRequest(req)

	persesLog.Info("Reconciling Perses", "Request.Namespace", req.Namespace, "Request.Name", reqName)

	var err error
	switch reqType {
	case dashboardReqType:
		err = r.reconcileDashboard(ctx, reqName, req.Namespace, reqLog)
	case datasourceReqType:
		err = r.reconcileDataSource(ctx, reqName, req.Namespace, reqLog)
	case startupReqType:
		err = r.reconcileAll(ctx, reqLog)
	default:
		reqLog.Info("unknow request; ignoring.", "Request.Namespace", req.Namespace, "Request.Name", req.Name, reqType, "type")
		return reconcile.Result{}, nil
	}

	if err != nil {
		reqLog.Error(err, "failed to reconcile Perses", "Request.Namespace", req.Namespace, "Request.Name", req.Name, "requestType", reqType)
	}
	return reconcile.Result{}, err
}

func SetupPersesWithManager(mgr manager.Manager, ownerRef metav1.OwnerReference) error {
	persesLog.Info("Setting up Perses controller")

	// Skip registration cleanly when Perses CRDs are not installed (e.g., unit/CI envs)
	if !checkPersesAvailable(context.Background(), mgr.GetClient()) {
		persesLog.Info("Perses CRDs not found; skipping Perses controller registration")
		return nil
	}

	namespace := hcoutil.GetOperatorNamespaceFromEnv()
	dashboards, err := initDashboards(namespace, persesLog)
	if err != nil {
		return err
	}
	datasource, err := initDatasource(namespace)
	if err != nil {
		return err
	}

	r := newPersesReconciler(mgr, namespace, ownerRef, dashboards, datasource)

	c, err := controller.New("perses controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	if err = c.Watch(
		source.Kind[*persesv1alpha1.PersesDashboard](mgr.GetCache(), &persesv1alpha1.PersesDashboard{},
			handler.TypedEnqueueRequestsFromMapFunc[*persesv1alpha1.PersesDashboard, reconcile.Request](func(ctx context.Context, dashboard *persesv1alpha1.PersesDashboard) []reconcile.Request {
				return []reconcile.Request{
					{
						NamespacedName: types.NamespacedName{
							Namespace: dashboard.Namespace,
							Name:      fmt.Sprintf("%s%s%s", dashboard.Name, dashboardReqSufix, randomSufix),
						},
					},
				}
			}),
			dashboardPredicate,
		),
	); err != nil {
		return err
	}

	if err = c.Watch(
		source.Kind[*persesv1alpha1.PersesDatasource](mgr.GetCache(), &persesv1alpha1.PersesDatasource{},
			handler.TypedEnqueueRequestsFromMapFunc[*persesv1alpha1.PersesDatasource, reconcile.Request](func(ctx context.Context, datasource *persesv1alpha1.PersesDatasource) []reconcile.Request {
				return []reconcile.Request{
					{
						NamespacedName: types.NamespacedName{
							Namespace: datasource.Namespace,
							Name:      fmt.Sprintf("%s%s%s", datasource.Name, datasourceReqSufix, randomSufix),
						},
					},
				}
			}),
			datasourcePredicate,
		),
	); err != nil {
		return err
	}

	if err = c.Watch(
		source.Channel(r.events, handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, _ client.Object) []reconcile.Request {
			return []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{
						Name: fmt.Sprintf("%s%s", startupReqType, randomSufix),
					},
				},
			}
		})),
	); err != nil {
		return err
	}

	r.forceFirstRequest()
	return nil
}

func newPersesReconciler(
	mgr manager.Manager,
	namespace string,
	ownerRef metav1.OwnerReference,
	dashboards map[string]persesv1alpha1.PersesDashboard,
	datasource *persesv1alpha1.PersesDatasource,
) *PersesReconciler {
	return &PersesReconciler{
		Client:           mgr.GetClient(),
		namespace:        namespace,
		events:           make(chan event.GenericEvent, 1),
		owner:            ownerRef,
		cachedDashboards: dashboards,
		cachedDatasource: datasource,
	}
}

func (r *PersesReconciler) ensureOwnerReference(obj client.Object) bool {
	// Avoid duplicate owner refs
	ors := obj.GetOwnerReferences()
	for i := range ors {
		if ors[i].UID == r.owner.UID {
			return false
		}
	}
	// Some client.Object implementations may not preserve APIVersion/Kind on OwnerReference
	// Ensure r.owner has the necessary fields (assumed to be pre-filled by ownresources.GetDeploymentRef()).
	ors = append(ors, r.owner)
	obj.SetOwnerReferences(ors)
	return true
}

// syncDashboard ensures the found dashboard matches the desired spec and metadata.
// It returns true if any change was applied to found (spec, ownerRef, labels).
func (r *PersesReconciler) syncDashboard(found *persesv1alpha1.PersesDashboard, desired *persesv1alpha1.PersesDashboard) (modified bool) {
	if !reflect.DeepEqual(found.Spec, desired.Spec) {
		desired.Spec.DeepCopyInto(&found.Spec)
		modified = true
	}
	if r.ensureOwnerReference(found) {
		modified = true
	}
	if !hcoutil.CompareLabels(desired, found) {
		hcoutil.MergeLabels(&desired.ObjectMeta, &found.ObjectMeta)
		modified = true
	}
	return modified
}

// syncDatasource ensures the found datasource matches the desired spec and metadata.
// It returns true if any change was applied to found (spec, ownerRef, labels).
func (r *PersesReconciler) syncDatasource(found *persesv1alpha1.PersesDatasource, desired *persesv1alpha1.PersesDatasource) (modified bool) {
	if !reflect.DeepEqual(found.Spec, desired.Spec) {
		desired.Spec.DeepCopyInto(&found.Spec)
		modified = true
	}
	if r.ensureOwnerReference(found) {
		modified = true
	}
	if !hcoutil.CompareLabels(desired, found) {
		hcoutil.MergeLabels(&desired.ObjectMeta, &found.ObjectMeta)
		modified = true
	}
	return modified
}

func (r *PersesReconciler) reconcileDashboard(ctx context.Context, name string, namespace string, logger logr.Logger) error {
	db, ok := r.cachedDashboards[name]
	if !ok || db.Namespace != namespace {
		logger.Info("Not a managed dashboard; ignoring", "namespace", namespace, "name", name)
		return nil
	}

	found := &persesv1alpha1.PersesDashboard{}
	err := r.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, found)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			obj := db.DeepCopy()
			_ = r.ensureOwnerReference(obj)
			return r.Create(ctx, obj)
		}
		return err
	}
	if r.syncDashboard(found, &db) {
		return r.Update(ctx, found)
	}
	return nil
}

func (r *PersesReconciler) reconcileDataSource(ctx context.Context, name string, namespace string, logger logr.Logger) error {
	if r.cachedDatasource.Name != name || r.cachedDatasource.Namespace != namespace {
		logger.Info("Not a managed dashboard; ignoring", "namespace", namespace, "name", name)
		return nil
	}

	found := &persesv1alpha1.PersesDatasource{}
	err := r.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, found)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			obj := r.cachedDatasource.DeepCopy()
			_ = r.ensureOwnerReference(obj)
			return r.Create(ctx, obj)
		}
		return err
	}
	if r.syncDatasource(found, r.cachedDatasource) {
		return r.Update(ctx, found)
	}
	return nil
}

func (r *PersesReconciler) reconcileAll(ctx context.Context, logger logr.Logger) error {
	var errs []error
	for name := range r.cachedDashboards {
		if err := r.reconcileDashboard(ctx, name, r.namespace, logger); err != nil {
			errs = append(errs, err)
		}
	}
	if err := r.reconcileDataSource(ctx, datasourceName, r.namespace, logger); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func (r *PersesReconciler) forceFirstRequest() {
	r.events <- event.GenericEvent{Object: &metav1.PartialObjectMetadata{}}
	close(r.events)
}

func resolveRequest(req reconcile.Request) (reqType, resourceName string) {
	if !strings.HasSuffix(req.Name, randomSufix) {
		return unknownReqType, ""
	}
	reqType = strings.TrimSuffix(req.Name, randomSufix)
	if reqType == startupReqType {
		return startupReqType, ""
	}
	if strings.HasSuffix(reqType, dashboardReqSufix) {
		return dashboardReqType, strings.TrimSuffix(reqType, dashboardReqSufix)
	}
	if strings.HasSuffix(reqType, datasourceReqSufix) {
		return datasourceReqType, strings.TrimSuffix(reqType, datasourceReqSufix)
	}
	return unknownReqType, ""
}
