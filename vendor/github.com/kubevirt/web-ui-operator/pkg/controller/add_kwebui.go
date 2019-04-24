package controller

import (
	"kubevirt.io/web-ui-operator/pkg/controller/kwebui"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, kwebui.Add)
}
