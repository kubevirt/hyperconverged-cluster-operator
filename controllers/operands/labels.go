package operands

import (
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

func GetLabels(component hcoutil.AppComponent) map[string]string {
	return hcoutil.GetLabels(hcoutil.HyperConvergedName, component)
}
