package controller

import (
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// AddToManagerFuncs is a list of functions to add all Controllers to the Manager
var AddToManagerFuncs []func(manager.Manager, hcoutil.ClusterInfo, hcoutil.Condition) error

// AddToManager adds all Controllers to the Manager
func AddToManager(m manager.Manager, ci hcoutil.ClusterInfo, upgradableCond hcoutil.Condition) error {
	for _, f := range AddToManagerFuncs {
		if err := f(m, ci, upgradableCond); err != nil {
			return err
		}
	}
	return nil
}
