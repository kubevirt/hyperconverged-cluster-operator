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

	guestDeviceDriversRoleName        = "guest-device-drivers-role"
	guestDeviceDriversRoleBindingName = "guest-device-drivers-rolebinding"

	deployObsCntrlFGName     = "deployObservabilityController"
	observeVMStatFeatureGate = "observeVMStat"
)

func shouldDeploy(hc *hcov1.HyperConverged) bool {
	return hc.Spec.FeatureGates.IsEnabled(deployObsCntrlFGName)
}
