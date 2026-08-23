package nodes

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	operatorhandler "github.com/operator-framework/operator-lib/handler"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/fields"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	ctrlcache "sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
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
			Namespace: os.Getenv(hcoutil.OperatorNamespaceEnv),
		},
	}

	placementReq = reconcile.Request{
		NamespacedName: k8stypes.NamespacedName{
			Name:      "operator-placement-req-" + randomConstSuffix,
			Namespace: os.Getenv(hcoutil.OperatorNamespaceEnv),
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

	// Label nodes after the cache starts on OpenShift: HCP workers, and classic OCP
	// nodes selected by operator Deployment node placement.
	clusterInfo := hcoutil.GetClusterInfo()
	if clusterInfo.IsOpenshift() {
		if err := mgr.Add(&startupNodeLabeler{reconciler: reconciler}); err != nil {
			return fmt.Errorf("failed to add startup node labeler: %w", err)
		}
	}

	watchOperatorPlacement := clusterInfo.IsOpenshift() && !clusterInfo.IsHyperShiftManaged()
	return add(mgr, reconciler, watchOperatorPlacement)
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager, nodeEvents chan<- event.GenericEvent) *ReconcileNodeCounter {
	clusterInfo := hcoutil.GetClusterInfo()

	isOpenshift := clusterInfo.IsOpenshift()
	isHyperShift := clusterInfo.IsHyperShiftManaged()
	shouldLabelHCPNodes := isOpenshift && isHyperShift
	shouldLabelClassicPlacementNodes := isOpenshift && !isHyperShift

	log.Info("Initializing nodes controller",
		"isOpenshift", isOpenshift,
		"isHyperShiftManaged", isHyperShift,
		"shouldLabelHCPNodes", shouldLabelHCPNodes,
		"shouldLabelClassicPlacementNodes", shouldLabelClassicPlacementNodes,
	)

	r := &ReconcileNodeCounter{
		Client:     mgr.GetClient(),
		nodeEvents: nodeEvents,
	}

	if shouldLabelHCPNodes {
		r.HandleHyperShiftNodeLabeling = HandleHyperShiftNodeLabeling
	} else {
		r.HandleHyperShiftNodeLabeling = staleNodeLabeling
	}

	if shouldLabelClassicPlacementNodes {
		// hco-operator is not in the restricted Deployment cache
		// (app=kubevirt-hyperconverged). Read nodeSelector via the API.
		reader := mgr.GetAPIReader()
		r.HandleClassicOperatorPlacementLabeling = func(ctx context.Context, cli client.Client, nodeName string, logger logr.Logger) error {
			return labelClassicOperatorPlacement(ctx, cli, reader, nodeName, logger)
		}
	} else {
		r.HandleClassicOperatorPlacementLabeling = staleNodeLabeling
	}

	return r
}

// staleNodeLabeling is a no-op used when a labeling path is not needed.
func staleNodeLabeling(_ context.Context, _ client.Client, _ string, _ logr.Logger) error {
	return nil
}

// staleHyperShiftNodeLabeling is kept for existing tests.
func staleHyperShiftNodeLabeling(ctx context.Context, cli client.Client, nodeName string, logger logr.Logger) error {
	return staleNodeLabeling(ctx, cli, nodeName, logger)
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler, watchOperatorPlacement bool) error {
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

	if err := c.Watch(
		source.Kind[*hcov1.HyperConverged](
			mgr.GetCache(), &hcov1.HyperConverged{},
			&handler.TypedEnqueueRequestForObject[*hcov1.HyperConverged]{},
			hyperconvergedPredicate{},
		)); err != nil {
		return err
	}

	if !watchOperatorPlacement {
		return nil
	}

	return watchHCOOperatorDeployment(mgr, c)
}

// watchHCOOperatorDeployment watches OLM's hco-operator Deployment.
// The main cache only lists Deployments labeled app=kubevirt-hyperconverged,
// and that label is not present on the OLM-managed operator Deployment.
// Do not watch Subscription: that CRD is OLM v0-only and is missing on
// plain Kubernetes (and OLM v1).
func watchHCOOperatorDeployment(mgr manager.Manager, c controller.Controller) error {
	ns := hcoutil.GetOperatorNamespaceFromEnv()
	operatorCache, err := ctrlcache.New(mgr.GetConfig(), ctrlcache.Options{
		Scheme: mgr.GetScheme(),
		Mapper: mgr.GetRESTMapper(),
		DefaultNamespaces: map[string]ctrlcache.Config{
			ns: {},
		},
		ByObject: map[client.Object]ctrlcache.ByObject{
			&appsv1.Deployment{}: {
				Field: fields.OneTermEqualSelector("metadata.name", hcoutil.OperatorName),
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create cache for HCO operator Deployment: %w", err)
	}
	if err := mgr.Add(operatorCache); err != nil {
		return fmt.Errorf("failed to add HCO operator Deployment cache: %w", err)
	}

	return c.Watch(
		source.Kind[*appsv1.Deployment](
			operatorCache, &appsv1.Deployment{},
			handler.TypedEnqueueRequestsFromMapFunc(func(_ context.Context, _ *appsv1.Deployment) []reconcile.Request {
				return []reconcile.Request{placementReq}
			}),
			operatorDeploymentPredicate{},
		))
}

// ReconcileNodeCounter reconciles the nodes count
type ReconcileNodeCounter struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client.Client
	HyperConvergedQueue                    workqueue.TypedRateLimitingInterface[reconcile.Request]
	nodeEvents                             chan<- event.GenericEvent
	HandleHyperShiftNodeLabeling           func(ctx context.Context, cli client.Client, nodeName string, logger logr.Logger) error
	HandleClassicOperatorPlacementLabeling func(ctx context.Context, cli client.Client, nodeName string, logger logr.Logger) error
}

// Reconcile updates the nodes count on ClusterInfo singleton
func (r *ReconcileNodeCounter) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	logger, err := logr.FromContext(ctx)
	if err != nil {
		logger = log
	}
	switch req {
	case hcoReq:
		logger.Info("Triggered by a HyperConverged CR change")
	case placementReq:
		logger.Info("Triggered by operator placement change")
	default:
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

	if req == placementReq {
		if err := r.labelAllNodes(ctx, logger); err != nil {
			logger.Error(err, "Failed to label nodes for operator placement")
			return reconcile.Result{}, err
		}
	} else if req != hcoReq {
		if err := r.applyNodeLabeling(ctx, req.Name, logger); err != nil {
			logger.Error(err, "Failed to handle node labeling")
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

func (r *ReconcileNodeCounter) applyNodeLabeling(ctx context.Context, nodeName string, logger logr.Logger) error {
	if r.HandleHyperShiftNodeLabeling != nil {
		if err := r.HandleHyperShiftNodeLabeling(ctx, r.Client, nodeName, logger); err != nil {
			return err
		}
	}
	if r.HandleClassicOperatorPlacementLabeling != nil {
		if err := r.HandleClassicOperatorPlacementLabeling(ctx, r.Client, nodeName, logger); err != nil {
			return err
		}
	}
	return nil
}

func (r *ReconcileNodeCounter) readHyperConverged(ctx context.Context) (*hcov1.HyperConverged, error) {
	hc := &hcov1.HyperConverged{}
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

// labelAllNodesAtStartup labels nodes after the manager cache is ready.
func (r *ReconcileNodeCounter) labelAllNodesAtStartup(ctx context.Context) error {
	log.Info("Labeling nodes at startup")
	return r.labelAllNodes(ctx, log)
}

func (r *ReconcileNodeCounter) labelAllNodes(ctx context.Context, logger logr.Logger) error {
	nodesList := &corev1.NodeList{}
	if err := r.List(ctx, nodesList); err != nil {
		return fmt.Errorf("failed to list nodes for labeling: %w", err)
	}

	var errs []error
	for i := range nodesList.Items {
		node := &nodesList.Items[i]
		if err := r.applyNodeLabeling(ctx, node.Name, logger); err != nil {
			logger.Error(err, "Failed to label node", "node", node.Name)
			errs = append(errs, fmt.Errorf("node %s: %w", node.Name, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to label %d node(s): %v", len(errs), errs)
	}

	logger.Info("Completed labeling nodes", "totalNodes", len(nodesList.Items))
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

// labelNode applies the HyperShift label on a single node.
// It adds node-role.kubevirt.io/control-plane and removes the old
// node-role.kubernetes.io/control-plane label if it was previously set by HCO (upgrade path).
func labelNode(ctx context.Context, cli client.Client, node *corev1.Node, logger logr.Logger) error {
	if !isWorkerNode(node) {
		return nil
	}

	patch := client.MergeFrom(node.DeepCopy())
	needsPatch := false

	// Add the new kubevirt-specific control-plane label if not already present
	if node.Labels[nodeinfo.LabelNodeRoleKubevirtControlPlane] != hypershiftLabelValue {
		logger.Info("Adding kubevirt control-plane label to worker node",
			"node", node.Name,
			"label", nodeinfo.LabelNodeRoleKubevirtControlPlane,
			"labelValue", hypershiftLabelValue,
		)
		node.Labels[nodeinfo.LabelNodeRoleKubevirtControlPlane] = hypershiftLabelValue
		needsPatch = true
	}

	// Upgrade path: remove the old kubernetes control-plane label if it was set by HCO
	if node.Labels[nodeinfo.LabelNodeRoleControlPlane] == hypershiftLabelValue {
		logger.Info("Removing old control-plane label from worker node (upgrade)",
			"node", node.Name,
			"label", nodeinfo.LabelNodeRoleControlPlane,
		)
		delete(node.Labels, nodeinfo.LabelNodeRoleControlPlane)
		needsPatch = true
	}

	if !needsPatch {
		return nil
	}

	if err := cli.Patch(ctx, node, patch); err != nil {
		return fmt.Errorf("failed to patch node %s: %w", node.Name, err)
	}

	logger.Info("Successfully patched node labels", "node", node.Name)
	return nil
}

// isWorkerNode checks if a node has the worker role label.
// A node is considered a worker if it has the worker label and does not have
// the standard Kubernetes control-plane label (ignoring the kubevirt-specific one,
// which HCO itself sets on HCP clusters).
func isWorkerNode(node *corev1.Node) bool {
	_, hasWorkerLabel := node.Labels[nodeinfo.LabelNodeRoleWorker]
	cpVal, hasControlPlaneLabel := node.Labels[nodeinfo.LabelNodeRoleControlPlane]

	if hasControlPlaneLabel && cpVal == hypershiftLabelValue {
		// This is a label HCO set previously; treat the node as a worker for upgrade purposes
		return hasWorkerLabel
	}

	return hasWorkerLabel && !hasControlPlaneLabel
}
