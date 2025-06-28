package nodes

import (
	"context"
	"maps"

	operatorhandler "github.com/operator-framework/operator-lib/handler"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/nodeinfo"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

var (
	log = logf.Log.WithName("controller_nodes")
)

// RegisterReconciler creates a new Nodes Reconciler and registers it into manager.
func RegisterReconciler(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	r := &ReconcileNodeCounter{
		Client: mgr.GetClient(),
	}

	return r
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("nodes-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to the cluster's nodes
	err = c.Watch(
		source.Kind(
			mgr.GetCache(), client.Object(&corev1.Node{}),
			&operatorhandler.InstrumentedEnqueueRequestForObject[client.Object]{},
			nodeCountChangePredicate{},
		))
	if err != nil {
		return err
	}

	return nil
}

// Custom predicate to detect changes in node count
type nodeCountChangePredicate struct {
	predicate.Funcs
}

func (nodeCountChangePredicate) Update(e event.UpdateEvent) bool {
	return !maps.Equal(e.ObjectOld.GetLabels(), e.ObjectNew.GetLabels())
}

func (nodeCountChangePredicate) Create(_ event.CreateEvent) bool {
	// node is added
	return true
}

func (nodeCountChangePredicate) Delete(_ event.DeleteEvent) bool {
	// node is removed
	return true
}

func (nodeCountChangePredicate) Generic(_ event.GenericEvent) bool {
	return false
}

// ReconcileNodeCounter reconciles the nodes count
type ReconcileNodeCounter struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client.Client
	HyperConvergedQueue workqueue.TypedRateLimitingInterface[reconcile.Request]
}

// Reconcile updates the nodes count on ClusterInfo singleton
func (r *ReconcileNodeCounter) Reconcile(ctx context.Context, _ reconcile.Request) (reconcile.Result, error) {
	log.Info("Triggered by a node count change")

	hc, err := r.readHyperConverged(ctx)
	if err != nil {
		return reconcile.Result{}, err
	}

	_, err = nodeinfo.HandleNodeChanges(ctx, r, hc, log)
	if err != nil {
		return reconcile.Result{}, err
	}

	if hc == nil || !hc.DeletionTimestamp.IsZero() {
		return reconcile.Result{}, nil
	}

	if infraHighlyAvailable := nodeinfo.IsInfrastructureHighlyAvailable(); hc.Status.InfrastructureHighlyAvailable == nil ||
		*hc.Status.InfrastructureHighlyAvailable != infraHighlyAvailable {

		hc.Status.InfrastructureHighlyAvailable = ptr.To(infraHighlyAvailable)
		err = r.Status().Update(ctx, hc)
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileNodeCounter) readHyperConverged(ctx context.Context) (*hcov1beta1.HyperConverged, error) {
	hc := &hcov1beta1.HyperConverged{}
	namespace := hcoutil.GetOperatorNamespaceFromEnv()
	hcoKey := types.NamespacedName{
		Name:      hcoutil.HyperConvergedName,
		Namespace: namespace,
	}
	err := r.Get(ctx, hcoKey, hc)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	return hc, nil
}
