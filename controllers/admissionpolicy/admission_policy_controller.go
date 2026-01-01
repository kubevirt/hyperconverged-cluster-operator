package admissionpolicy

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	operatorhandler "github.com/operator-framework/operator-lib/handler"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	k8stypes "k8s.io/apimachinery/pkg/types"
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

const controllerName = "admission-policy-controller"

var (
	initLogger = logf.Log.WithName(controllerName)

	randomConstSuffix = uuid.New().String()

	startupReq = reconcile.Request{
		NamespacedName: k8stypes.NamespacedName{
			Name: "startup-req-" + randomConstSuffix,
		},
	}
)

// RegisterReconciler creates a new Nodes Reconciler and registers it into manager.
func RegisterReconciler(mgr manager.Manager) error {
	startupEvent := make(chan event.GenericEvent, 1)
	defer close(startupEvent)

	r := newReconciler(mgr, startupEvent)

	startupEvent <- event.GenericEvent{}

	return add(mgr, r)
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager, startupEvent <-chan event.GenericEvent) *ReconcileAdmissionPolicy {
	initLogger.Info("Initializing the admission policy controller")

	r := &ReconcileAdmissionPolicy{
		Client:       mgr.GetClient(),
		startupEvent: startupEvent,
	}

	return r
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r *ReconcileAdmissionPolicy) error {
	// Create a new controller
	c, err := controller.New(controllerName, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to the ValidatingAdmissionPolicy
	if err = c.Watch(
		source.Kind[*admissionv1.ValidatingAdmissionPolicy](
			mgr.GetCache(), &admissionv1.ValidatingAdmissionPolicy{},
			&operatorhandler.InstrumentedEnqueueRequestForObject[*admissionv1.ValidatingAdmissionPolicy]{},
			policyPredicate,
		),
	); err != nil {
		return err
	}

	// Watch for changes to the ValidatingAdmissionPolicyBinding
	if err = c.Watch(
		source.Kind[*admissionv1.ValidatingAdmissionPolicyBinding](
			mgr.GetCache(), &admissionv1.ValidatingAdmissionPolicyBinding{},
			&handler.TypedEnqueueRequestForObject[*admissionv1.ValidatingAdmissionPolicyBinding]{},
			bindingPredicate,
		),
	); err != nil {
		return err
	}

	return c.Watch(
		source.Channel(
			r.startupEvent,
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, a client.Object) []reconcile.Request {
				logr.FromContextOrDiscard(ctx).Info("first reconciliation of ValidatingAdmissionPolicy")
				return []reconcile.Request{startupReq}
			}),
		),
	)
}

// ReconcileAdmissionPolicy reconciles the ValidatingAdmissionPolicy and ValidatingAdmissionPolicyBinding
type ReconcileAdmissionPolicy struct {
	client.Client
	startupEvent <-chan event.GenericEvent
}

// Reconcile updates the ValidatingAdmissionPolicy and ValidatingAdmissionPolicyBinding
func (r *ReconcileAdmissionPolicy) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	logger, err := logr.FromContext(ctx)
	if err != nil {
		logger = initLogger.WithValues("Request.Name", req.Name)
	}

	logger.Info(fmt.Sprintf("Reconciling admission policy %s", req.Name))

	startup := startupReq == req
	var policyErr, bindingErr error

	if req.Name == policyName || startup {
		policyErr = r.reconcilePolicy(ctx, logger)
	}

	if req.Name == policyBindingName || startup {
		bindingErr = r.reconcileBinding(ctx, logger)
	}

	err = errors.Join(policyErr, bindingErr)
	if err != nil {
		logger.Error(err, "Reconciliation failed")
	}

	return reconcile.Result{}, err
}

func (r *ReconcileAdmissionPolicy) reconcilePolicy(ctx context.Context, logger logr.Logger) error {
	policy := getRequiredPolicy()
	key := client.ObjectKeyFromObject(policy)
	foundPolicy := &admissionv1.ValidatingAdmissionPolicy{}

	if err := r.Get(ctx, key, foundPolicy); err != nil {
		if k8serrors.IsNotFound(err) {
			logger.Info("ValidatingAdmissionPolicy does not exist; creating it", "name", policy.Name)
			return r.Create(ctx, policy.DeepCopy())
		}

		return err
	}

	changed := false
	if !reflect.DeepEqual(foundPolicy.Spec, policy.Spec) {
		policy.Spec.DeepCopyInto(&foundPolicy.Spec)
		changed = true
	}

	if !hcoutil.CompareLabels(policy, foundPolicy) {
		hcoutil.MergeLabels(&policy.ObjectMeta, &foundPolicy.ObjectMeta)
		changed = true
	}

	if changed {
		logger.Info("ValidatingAdmissionPolicy was modified; updating it", "name", policy.Name)
		return r.Update(ctx, foundPolicy)
	}

	return nil
}

func (r *ReconcileAdmissionPolicy) reconcileBinding(ctx context.Context, logger logr.Logger) error {
	binding := getRequiredBinding()

	key := client.ObjectKeyFromObject(binding)
	foundBinding := &admissionv1.ValidatingAdmissionPolicyBinding{}

	if err := r.Get(ctx, key, foundBinding); err != nil {
		if k8serrors.IsNotFound(err) {
			logger.Info("ValidatingAdmissionPolicyBinding does not exist; creating it", "name", binding.Name)
			return r.Create(ctx, binding.DeepCopy())
		}

		return err
	}

	changed := false
	if !reflect.DeepEqual(foundBinding.Spec, binding.Spec) {
		binding.Spec.DeepCopyInto(&foundBinding.Spec)
		changed = true
	}

	if !hcoutil.CompareLabels(binding, foundBinding) {
		hcoutil.MergeLabels(&binding.ObjectMeta, &foundBinding.ObjectMeta)
		changed = true
	}

	if changed {
		logger.Info("ValidatingAdmissionPolicyBinding was modified; updating it", "name", binding.Name)
		return r.Update(ctx, foundBinding)
	}

	return nil
}
