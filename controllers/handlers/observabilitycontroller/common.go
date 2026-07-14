package observabilitycontroller

import (
	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
)

const (
	controllerName         = "virt-observability-controller"
	clusterRoleName        = controllerName
	clusterRoleBindingName = controllerName + "-rolebinding"
	serviceAccountName     = controllerName
	deploymentName         = controllerName

	featureGateName = "deployObservabilityController"
)

func shouldDeploy(hc *hcov1.HyperConverged) bool {
	return hc.Spec.FeatureGates.IsEnabled(featureGateName)
}
