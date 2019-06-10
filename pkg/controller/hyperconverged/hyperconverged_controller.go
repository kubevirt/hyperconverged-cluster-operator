package hyperconverged

import (
	"context"
	"fmt"
	"reflect"

	sspv1 "github.com/MarSik/kubevirt-ssp-operator/pkg/apis/kubevirt/v1"
	"github.com/go-logr/logr"
	networkaddons "github.com/kubevirt/cluster-network-addons-operator/pkg/apis/networkaddonsoperator/v1alpha1"
	hcov1alpha1 "github.com/kubevirt/hyperconverged-cluster-operator/pkg/apis/hco/v1alpha1"
	kwebuis "github.com/kubevirt/web-ui-operator/pkg/apis/kubevirt/v1alpha1"
	cdi "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
	kubevirt "kubevirt.io/kubevirt/pkg/api/v1"

	"encoding/json"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_hyperconverged")

const (
	// We cannot set owner reference of cluster-wide resources to namespaced HyperConverged object. Therefore,
	// use finalizers to manage the cleanup.
	FinalizerName = "hyperconvergeds.hco.kubevirt.io"

	// Foreground deletion finalizer is blocking removal of HyperConverged until explicitly dropped.
	// TODO: Research whether there is a better way.
	foregroundDeletionFinalizer = "foregroundDeletion"
)

// Add creates a new HyperConverged Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileHyperConverged{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("hyperconverged-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource HyperConverged
	err = c.Watch(&source.Kind{Type: &hcov1alpha1.HyperConverged{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch secondary resources
	for _, resource := range []runtime.Object{
		&kubevirt.KubeVirt{},
		&cdi.CDI{},
		&networkaddons.NetworkAddonsConfig{},
		&sspv1.KubevirtCommonTemplatesBundle{},
		&sspv1.KubevirtNodeLabellerBundle{},
		&sspv1.KubevirtTemplateValidator{},
		&kwebuis.KWebUI{},
	} {
		err = c.Watch(&source.Kind{Type: resource}, &handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &hcov1alpha1.HyperConverged{},
		})
		if err != nil {
			return err
		}
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileHyperConverged{}

// ReconcileHyperConverged reconciles a HyperConverged object
type ReconcileHyperConverged struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a HyperConverged object and makes changes based on the state read
// and what is in the HyperConverged.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileHyperConverged) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling HyperConverged operator")

	// Fetch the HyperConverged instance
	instance := &hcov1alpha1.HyperConverged{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	return r.reconcileUpdate(reqLogger, instance, request)
}

// ResultFromCRCreate holds the result and error from creating a CR
type ResultFromCRCreate struct {
	result reconcile.Result
	err    error
}

func (r *ReconcileHyperConverged) reconcileUpdate(logger logr.Logger, cr *hcov1alpha1.HyperConverged, request reconcile.Request) (reconcile.Result, error) {
	results := []ResultFromCRCreate{}

	resources := r.getAllResources(cr, request)
	for _, desiredRuntimeObj := range resources {
		desiredMetaObj := desiredRuntimeObj.(metav1.Object)

		// use reflection to create default instance of desiredRuntimeObj type
		typ := reflect.ValueOf(desiredRuntimeObj).Elem().Type()
		currentRuntimeObj := reflect.New(typ).Interface().(runtime.Object)

		key := client.ObjectKey{
			Namespace: desiredMetaObj.GetNamespace(),
			Name:      desiredMetaObj.GetName(),
		}
		err := r.client.Get(context.TODO(), key, currentRuntimeObj)

		if err != nil {
			if !errors.IsNotFound(err) {
				results = append(results, ResultFromCRCreate{result: reconcile.Result{}, err: err})
			}

			if err = controllerutil.SetControllerReference(cr, desiredMetaObj, r.scheme); err != nil {
				results = append(results, ResultFromCRCreate{result: reconcile.Result{}, err: err})
			}

			// TODO: common-templates and cdi fails the Get check above and appears to still be missing.
			// But in reality, when you try to Create it, the client reports back that
			// the resource already exists. Need to investigate why.
			// Before the refactor, the code didn't check if a resource already exists. It just
			// tried to create it, and will skip if the Create indicated that it already exists.
			if err = r.client.Create(context.TODO(), desiredRuntimeObj); err != nil {
				if err != nil && errors.IsAlreadyExists(err) {
					logger.Info("Skip reconcile: tried create but resource already exists", "key", key)
					results = append(results, ResultFromCRCreate{result: reconcile.Result{}, err: nil})
				} else if err != nil {
					results = append(results, ResultFromCRCreate{result: reconcile.Result{}, err: err})
				}
			} else {
				logger.Info("Resource created",
					"namespace", desiredMetaObj.GetNamespace(),
					"name", desiredMetaObj.GetName(),
					"type", fmt.Sprintf("%T", desiredMetaObj))
			}
		} else {
			logger.Info("Skip reconcile: resource already exists", "key", key)
			results = append(results, ResultFromCRCreate{result: reconcile.Result{}, err: nil})
		}
	}

	for _, r := range results {
		if r.err != nil {
			return r.result, r.err
		}
	}

	return reconcile.Result{}, nil
}

func contains(l []string, s string) bool {
	for _, elem := range l {
		if elem == s {
			return true
		}
	}
	return false
}

func drop(l []string, s string) []string {
	newL := []string{}
	for _, elem := range l {
		if elem != s {
			newL = append(newL, elem)
		}
	}
	return newL
}

// toUnstructured convers an arbitrary object (which MUST obey the
// k8s object conventions) to an Unstructured
func toUnstructured(obj interface{}) (*unstructured.Unstructured, error) {
	b, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	u := &unstructured.Unstructured{}
	if err := json.Unmarshal(b, u); err != nil {
		return nil, err
	}
	return u, nil
}
