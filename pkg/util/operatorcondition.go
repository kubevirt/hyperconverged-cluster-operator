package util

import (
	"context"

	operatorframeworkv1 "github.com/operator-framework/api/pkg/operators/v1"
	"github.com/operator-framework/operator-lib/conditions"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// OperatorCondition wraps operator-lib's Condition to make it not crash,
// when running locally or in Kubernetes without OLM.
type OperatorCondition struct {
	cond        conditions.Condition
	clusterinfo ClusterInfo
}

const UpgradableCondition operatorframeworkv1.ConditionType = "Upgradable"

// We just need the Set method in our code.
// TODO: define interface next to consumer in hyperconverged_controller.go after refactoring controller registration.
type Condition interface {
	Set(ctx context.Context, status metav1.ConditionStatus, option ...conditions.Option) error
}

func NewOperatorCondition(
	clusterInfo ClusterInfo,
	c client.Client,
	condType operatorframeworkv1.ConditionType,
) (*OperatorCondition, error) {
	oc := &OperatorCondition{
		clusterinfo: clusterInfo,
	}
	if oc.clusterinfo.IsRunningLocally() {
		// Don't try to update OperatorCondition when we don't run in cluster,
		// because operator-lib can't discover the namespace we are running in.
		return oc, nil
	}
	if !oc.clusterinfo.IsManagedByOLM() {
		// We are not managed by OLM -> no OperatorCondition
		return oc, nil
	}

	cond, err := conditions.NewCondition(c, condType)
	if err != nil {
		return nil, err
	}
	oc.cond = cond
	return oc, nil
}

func (oc *OperatorCondition) Set(
	ctx context.Context, status metav1.ConditionStatus, option ...conditions.Option) error {
	if oc.cond == nil {
		// no op
		return nil
	}
	return oc.cond.Set(ctx, status, option...)
}
