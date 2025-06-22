package nodes

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	operatorhandler "github.com/operator-framework/operator-lib/handler"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/nodeinfo"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

var (
	log               = logf.Log.WithName("controller_nodes")
	randomConstSuffix = uuid.New().String()

	hcoReq = reconcile.Request{
		NamespacedName: k8stypes.NamespacedName{
			Name:      "hyperconverged-req-" + randomConstSuffix,
			Namespace: hcoutil.GetOperatorNamespaceFromEnv(),
		},
	}
)

// RegisterReconciler creates a new Nodes Reconciler and registers it into manager.
func RegisterReconciler(mgr manager.Manager, nodeEvents chan<- event.GenericEvent) error {
	return add(mgr, newReconciler(mgr, nodeEvents))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager, nodeEvents chan<- event.GenericEvent) reconcile.Reconciler {
	r := &ReconcileNodeCounter{
		Client:     mgr.GetClient(),
		nodeEvents: nodeEvents,
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
		source.Kind[*corev1.Node](
			mgr.GetCache(), &corev1.Node{},
			&operatorhandler.InstrumentedEnqueueRequestForObject[*corev1.Node]{},
			nodeCountChangePredicate{},
		))
	if err != nil {
		return err
	}

	return c.Watch(
		source.Kind[*hcov1beta1.HyperConverged](
			mgr.GetCache(), &hcov1beta1.HyperConverged{},
			&handler.TypedEnqueueRequestForObject[*hcov1beta1.HyperConverged]{},
			hyperconvergedPredicate{},
		))
}

// ReconcileNodeCounter reconciles the nodes count
type ReconcileNodeCounter struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client.Client
	HyperConvergedQueue workqueue.TypedRateLimitingInterface[reconcile.Request]
	nodeEvents          chan<- event.GenericEvent
}

// Reconcile updates the nodes count on ClusterInfo singleton
func (r *ReconcileNodeCounter) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	logger, err := logr.FromContext(ctx)
	if err != nil {
		logger = log
	}
	if req == hcoReq {
		// This is a request triggered by a change in the HyperConverged CR
		logger.Info("Triggered by a HyperConverged CR change")
	} else {
		logger.Info("Triggered by a node change", "node name", req.Name)
	}

	logger.Info("Reading the latest HyperConverged CR")
	hc, err := r.readHyperConverged(ctx)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to read the HyperConverged CR; %v", err)
	}

	nodeInfoChanged, err := nodeinfo.HandleNodeChanges(ctx, r, hc, logger)
	if err != nil {
		return reconcile.Result{}, err
	}

	if hc == nil || !hc.DeletionTimestamp.IsZero() {
		return reconcile.Result{}, nil
	}

	if nodeInfoChanged {
		r.nodeEvents <- event.GenericEvent{}
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileNodeCounter) readHyperConverged(ctx context.Context) (*hcov1beta1.HyperConverged, error) {
	hc := &hcov1beta1.HyperConverged{}
	hcoKey := k8stypes.NamespacedName{
		Name:      hcoutil.HyperConvergedName,
		Namespace: hcoutil.GetOperatorNamespaceFromEnv(),
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
