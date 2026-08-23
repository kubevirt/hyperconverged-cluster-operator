package nodes

import (
	"maps"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
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
