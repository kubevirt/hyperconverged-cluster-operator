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

	// holds the CA bundle the controller verifies virt-handler against
	kubevirtCAConfigMapName = "kubevirt-ca"

	deployObsCntrlFGName     = "deployObservabilityController"
	observeVMStatFeatureGate = "observeVMStat"
)

func shouldDeploy(hc *hcov1.HyperConverged) bool {
	return hc.Spec.FeatureGates.IsEnabled(deployObsCntrlFGName)
}
