package nodes

import (
	"context"
	"fmt"
	"time"

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

const (
	// HyperShift label value for worker nodes
	hypershiftLabelValue = "set-to-allow-kubevirt-deployment"
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

// startupNodeLabeler is a runnable that labels all nodes after the manager cache starts
type startupNodeLabeler struct {
	reconciler *ReconcileNodeCounter
}

// Start implements manager.Runnable
func (s *startupNodeLabeler) Start(ctx context.Context) error {
	log.Info("Starting node labeling after cache is ready")

	const (
		maxRetries    = 5
		retryInterval = 10 * time.Second
	)

	// Label all nodes now that the cache is ready
	err := s.reconciler.labelAllNodesAtStartup(ctx)
	if err != nil {
		log.Error(err, "Failed to label nodes at startup, will retry", "maxRetries", maxRetries, "retryInterval", retryInterval)

		// Retry up to maxRetries times
		for attempt := 1; attempt <= maxRetries && err != nil; attempt++ {
			select {
			case <-ctx.Done():
				log.Info("Context cancelled during retry, stopping node labeling")
				return nil
			case <-time.After(retryInterval):
				log.Info("Retrying node labeling at startup", "attempt", attempt, "maxRetries", maxRetries)
				err = s.reconciler.labelAllNodesAtStartup(ctx)
				if err == nil {
					log.Info("Successfully labeled nodes at startup after retry", "attempt", attempt)
				} else {
					log.Error(err, "Failed to label nodes at startup", "attempt", attempt, "maxRetries", maxRetries)
				}
			}
		}

		if err != nil {
			log.Error(err, "Failed to label nodes at startup after all retries, giving up", "maxRetries", maxRetries)
		}
	}

	// Keep running until context is cancelled
	<-ctx.Done()
	return nil
}

// NeedLeaderElection implements manager.LeaderElectionRunnable
func (s *startupNodeLabeler) NeedLeaderElection() bool {
	return true
}

// RegisterReconciler creates a new Nodes Reconciler and registers it into manager.
func RegisterReconciler(mgr manager.Manager, nodeEvents chan<- event.GenericEvent) error {
	reconciler := newReconciler(mgr, nodeEvents)

	// Add a runnable to label all nodes after the cache starts if we're in a HyperShift cluster
	clusterInfo := hcoutil.GetClusterInfo()
	if clusterInfo.IsOpenshift() && clusterInfo.IsHyperShiftManaged() {
		if err := mgr.Add(&startupNodeLabeler{reconciler: reconciler}); err != nil {
			return fmt.Errorf("failed to add startup node labeler: %w", err)
		}
	}

	return add(mgr, reconciler)
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager, nodeEvents chan<- event.GenericEvent) *ReconcileNodeCounter {
	clusterInfo := hcoutil.GetClusterInfo()

	// Evaluate once at initialization whether we should label nodes for HyperShift
	shouldLabelNodes := clusterInfo.IsOpenshift() && clusterInfo.IsHyperShiftManaged()

	log.Info("Initializing nodes controller",
		"isOpenshift", clusterInfo.IsOpenshift(),
		"isHyperShiftManaged", clusterInfo.IsHyperShiftManaged(),
		"shouldLabelNodes", shouldLabelNodes,
	)

	r := &ReconcileNodeCounter{
		Client:     mgr.GetClient(),
		nodeEvents: nodeEvents,
	}

	if shouldLabelNodes {
		r.HandleHyperShiftNodeLabeling = HandleHyperShiftNodeLabeling
	} else {
		r.HandleHyperShiftNodeLabeling = staleHyperShiftNodeLabeling
	}

	return r
}

// staleHyperShiftNodeLabeling is a no-op function used when HyperShift node labeling is not needed
func staleHyperShiftNodeLabeling(_ context.Context, _ client.Client, _ string, _ logr.Logger) error {
	return nil
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
	HyperConvergedQueue          workqueue.TypedRateLimitingInterface[reconcile.Request]
	nodeEvents                   chan<- event.GenericEvent
	HandleHyperShiftNodeLabeling func(ctx context.Context, cli client.Client, nodeName string, logger logr.Logger) error
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

	// Handle HyperShift node labeling for hosted control plane clusters
	// Only process if this is a node event (not HCO event)
	if req != hcoReq {
		if err := r.HandleHyperShiftNodeLabeling(ctx, r.Client, req.Name, logger); err != nil {
			logger.Error(err, "Failed to handle HyperShift node labeling")
			return reconcile.Result{}, err
		}
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

// labelAllNodesAtStartup labels all worker nodes at controller startup for HyperShift clusters
func (r *ReconcileNodeCounter) labelAllNodesAtStartup(ctx context.Context) error {
	log.Info("Labeling all worker nodes at startup for HyperShift")

	// Get all nodes
	nodesList := &corev1.NodeList{}
	if err := r.List(ctx, nodesList); err != nil {
		return fmt.Errorf("failed to list nodes for HyperShift labeling at startup: %w", err)
	}

	var errs []error
	for i := range nodesList.Items {
		node := &nodesList.Items[i]
		if err := labelNode(ctx, r.Client, node, log); err != nil {
			log.Error(err, "Failed to label node at startup", "node", node.Name)
			errs = append(errs, fmt.Errorf("node %s: %w", node.Name, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to label %d node(s): %v", len(errs), errs)
	}

	log.Info("Completed labeling nodes at startup", "totalNodes", len(nodesList.Items))
	return nil
}

// HandleHyperShiftNodeLabeling manages the control-plane label on a specific worker node for HyperShift managed clusters
func HandleHyperShiftNodeLabeling(ctx context.Context, cli client.Client, nodeName string, logger logr.Logger) error {
	// Get the specific node
	node := &corev1.Node{}
	if err := cli.Get(ctx, client.ObjectKey{Name: nodeName}, node); err != nil {
		if errors.IsNotFound(err) {
			// Node was deleted, nothing to do
			logger.V(1).Info("Node not found, skipping HyperShift labeling", "node", nodeName)
			return nil
		}
		return fmt.Errorf("failed to get node %s for HyperShift labeling: %w", nodeName, err)
	}

	return labelNode(ctx, cli, node, logger)
}

// labelNode applies the HyperShift label on a single node
func labelNode(ctx context.Context, cli client.Client, node *corev1.Node, logger logr.Logger) error {
	if !isWorkerNode(node) {
		return nil
	}

	logger.Info("Adding control-plane label to worker node",
		"node", node.Name,
		"labelValue", hypershiftLabelValue,
	)

	patch := client.MergeFrom(node.DeepCopy())
	node.Labels[nodeinfo.LabelNodeRoleControlPlane] = hypershiftLabelValue

	if err := cli.Patch(ctx, node, patch); err != nil {
		return fmt.Errorf("failed to patch node %s: %w", node.Name, err)
	}

	logger.Info("Successfully patched node labels", "node", node.Name)
	return nil
}

// isWorkerNode checks if a node has the worker role label
func isWorkerNode(node *corev1.Node) bool {
	// A node is a worker if it has the worker label and doesn't have control-plane label
	_, hasWorkerLabel := node.Labels[nodeinfo.LabelNodeRoleWorker]
	_, hasControlPlaneLabel := node.Labels[nodeinfo.LabelNodeRoleControlPlane]

	return hasWorkerLabel && !hasControlPlaneLabel
}
