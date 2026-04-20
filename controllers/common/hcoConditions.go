package common

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
)

var (
	HcoConditionTypes = []string{
		hcov1.ConditionReconcileComplete,
		hcov1.ConditionAvailable,
		hcov1.ConditionProgressing,
		hcov1.ConditionDegraded,
		hcov1.ConditionUpgradeable,
	}
)

type HcoConditions map[string]metav1.Condition

func NewHcoConditions() HcoConditions {
	return HcoConditions{}
}

func (hc HcoConditions) SetStatusCondition(newCondition metav1.Condition) {
	existingCondition, exists := hc[newCondition.Type]

	if !exists {
		hc[newCondition.Type] = newCondition
		return
	}

	if existingCondition.Status != newCondition.Status {
		existingCondition.Status = newCondition.Status
	}

	existingCondition.Reason = newCondition.Reason
	existingCondition.Message = newCondition.Message
	hc[newCondition.Type] = existingCondition
}

func (hc HcoConditions) SetStatusConditionIfUnset(newCondition metav1.Condition) {
	if !hc.HasCondition(newCondition.Type) {
		hc.SetStatusCondition(newCondition)
	}
}

func (hc HcoConditions) IsEmpty() bool {
	return len(hc) == 0
}

func (hc HcoConditions) HasCondition(conditionType string) bool {
	_, exists := hc[conditionType]

	return exists
}

func (hc HcoConditions) GetCondition(conditionType string) (metav1.Condition, bool) {
	cond, found := hc[conditionType]
	return cond, found
}

func (hc HcoConditions) IsStatusConditionTrue(conditionType string) bool {
	cond, found := hc[conditionType]
	return found && cond.Status == metav1.ConditionTrue
}
