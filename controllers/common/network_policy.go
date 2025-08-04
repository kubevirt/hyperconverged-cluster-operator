package common

import (
	"os"

	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

var deployNetworkPolicy = os.Getenv(hcoutil.DeployNetworkPoliciesEnvV) == "true"

var ShouldDeployNetworkPolicy = func() bool {
	return deployNetworkPolicy
}
