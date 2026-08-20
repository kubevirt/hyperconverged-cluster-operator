package nodes

import (
	"maps"
	"os"
	"reflect"

	csvv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

// Custom predicate to detect changes in node count
type nodeCountChangePredicate predicate.TypedFuncs[*corev1.Node]

func (nodeCountChangePredicate) Update(e event.TypedUpdateEvent[*corev1.Node]) bool {
	return !maps.Equal(e.ObjectOld.GetLabels(), e.ObjectNew.GetLabels())
}

func (nodeCountChangePredicate) Create(_ event.TypedCreateEvent[*corev1.Node]) bool {
	// node is added
	return true
}

func (nodeCountChangePredicate) Delete(_ event.TypedDeleteEvent[*corev1.Node]) bool {
	// node is removed
	return true
}

func (nodeCountChangePredicate) Generic(_ event.TypedGenericEvent[*corev1.Node]) bool {
	return false
}

type hyperconvergedPredicate predicate.TypedFuncs[*hcov1.HyperConverged]

func (hyperconvergedPredicate) Create(_ event.TypedCreateEvent[*hcov1.HyperConverged]) bool {
	// HyperConverged CR is created, we want to reconcile
	return true
}

func (hyperconvergedPredicate) Update(e event.TypedUpdateEvent[*hcov1.HyperConverged]) bool {
	// HyperConverged CR is updated
	if e.ObjectNew.DeletionTimestamp != nil {
		// If the HyperConverged CR is being deleted, we do not want to reconcile
		return false
	}

	newNP := e.ObjectNew.Spec.Deployment.NodePlacements
	oldNP := e.ObjectOld.Spec.Deployment.NodePlacements
	newNPExists, oldNPExists := newNP != nil, oldNP != nil

	if newNPExists != oldNPExists {
		return (newNPExists && newNP.Workload != nil) || (oldNPExists && oldNP.Workload != nil)
	} else if newNPExists && oldNPExists {
		return !reflect.DeepEqual(newNP.Workload, oldNP.Workload)
	}

	return false
}

func (hyperconvergedPredicate) Delete(_ event.TypedDeleteEvent[*hcov1.HyperConverged]) bool {
	return true
}

func (hyperconvergedPredicate) Generic(_ event.TypedGenericEvent[*hcov1.HyperConverged]) bool {
	return false
}

func operatorNamespace() string {
	return os.Getenv(hcoutil.OperatorNamespaceEnv)
}

type virtOperatorDeploymentPredicate predicate.TypedFuncs[*appsv1.Deployment]

func (virtOperatorDeploymentPredicate) Create(e event.TypedCreateEvent[*appsv1.Deployment]) bool {
	return isVirtOperatorDeployment(e.Object)
}

func (virtOperatorDeploymentPredicate) Update(e event.TypedUpdateEvent[*appsv1.Deployment]) bool {
	if !isVirtOperatorDeployment(e.ObjectNew) {
		return false
	}
	return !maps.Equal(e.ObjectOld.Spec.Template.Spec.NodeSelector, e.ObjectNew.Spec.Template.Spec.NodeSelector)
}

func (virtOperatorDeploymentPredicate) Delete(e event.TypedDeleteEvent[*appsv1.Deployment]) bool {
	return isVirtOperatorDeployment(e.Object)
}

func (virtOperatorDeploymentPredicate) Generic(_ event.TypedGenericEvent[*appsv1.Deployment]) bool {
	return false
}

func isVirtOperatorDeployment(dep *appsv1.Deployment) bool {
	return dep != nil && dep.Name == virtOperatorDeploymentName && dep.Namespace == operatorNamespace()
}

type subscriptionConfigPredicate predicate.TypedFuncs[*csvv1alpha1.Subscription]

func (subscriptionConfigPredicate) Create(e event.TypedCreateEvent[*csvv1alpha1.Subscription]) bool {
	return e.Object != nil && e.Object.Namespace == operatorNamespace()
}

func (subscriptionConfigPredicate) Update(e event.TypedUpdateEvent[*csvv1alpha1.Subscription]) bool {
	if e.ObjectNew == nil || e.ObjectNew.Namespace != operatorNamespace() {
		return false
	}
	return !reflect.DeepEqual(subscriptionNodeSelector(e.ObjectOld), subscriptionNodeSelector(e.ObjectNew))
}

func (subscriptionConfigPredicate) Delete(e event.TypedDeleteEvent[*csvv1alpha1.Subscription]) bool {
	return e.Object != nil && e.Object.Namespace == operatorNamespace()
}

func (subscriptionConfigPredicate) Generic(_ event.TypedGenericEvent[*csvv1alpha1.Subscription]) bool {
	return false
}

func subscriptionNodeSelector(sub *csvv1alpha1.Subscription) map[string]string {
	if sub == nil || sub.Spec == nil || sub.Spec.Config == nil {
		return nil
	}
	return sub.Spec.Config.NodeSelector
}
