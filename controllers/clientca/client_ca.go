// Package clientca defines a controller that watches changes in the general K8s client CA ConfigMap, and
// updates the metrics endpoint with the latest CA content.
package clientca

import (
	"context"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/tlssecprofile"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

const operatorName = "client-ca-controller"

type clientCAReconciler struct {
	client.Client
}

var (
	setupLogger = logf.Log.WithName(operatorName)
)

func (r *clientCAReconciler) Reconcile(ctx context.Context, _ ctrl.Request) (ctrl.Result, error) {
	logger := logr.FromContextOrDiscard(ctx)
	logger.Info("Reconciling Client CA")

	if err := tlssecprofile.RefreshClientCA(ctx, r, logger); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// RegisterReconciler creates a new HyperConverged Reconciler and registers it into manager.
func RegisterReconciler(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

func newReconciler(mgr manager.Manager) *clientCAReconciler {
	return &clientCAReconciler{
		Client: mgr.GetClient(),
	}
}

func add(mgr manager.Manager, r *clientCAReconciler) error {
	setupLogger.Info("Setting up Client CA controller")

	c, err := controller.New(operatorName, mgr, controller.Options{
		Reconciler: r,
	})
	if err != nil {
		return err
	}

	// Watch K8s client CA, to configure the TLS client for prometheus
	return c.Watch(source.Kind[*corev1.ConfigMap](
		mgr.GetCache(),
		new(corev1.ConfigMap),
		&handler.TypedEnqueueRequestForObject[*corev1.ConfigMap]{},
		cmPredicate,
	))
}

// clientCAConfigMapPredicate limits reconciles to the authoritative client-CA
// ConfigMap. The cache informer is already scoped to it via ByObject, but the
// predicate keeps intent explicit and guards against any wider event source.
var cmPredicate = predicate.NewTypedPredicateFuncs[*corev1.ConfigMap](func(cm *corev1.ConfigMap) bool {
	return cm.GetNamespace() == hcoutil.ClientCAConfigMapNamespace &&
		cm.GetName() == hcoutil.ClientCAConfigMapName
})
