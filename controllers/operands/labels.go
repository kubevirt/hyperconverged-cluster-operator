package operands

import (
	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

func GetLabels(hc *hcov1beta1.HyperConverged, component hcoutil.AppComponent) map[string]string {
	hcoName := hcoutil.HyperConvergedName

	if hc.Name != "" {
		hcoName = hc.Name
	}

	return hcoutil.GetLabels(hcoName, component)
}

func GetStaticLabels(component hcoutil.AppComponent) map[string]string {
	return hcoutil.GetLabels(hcoutil.HyperConvergedName, component)
}
