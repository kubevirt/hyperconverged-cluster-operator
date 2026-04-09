package operands

import (
	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

func GetLabels(component hcoutil.AppComponent) map[string]string {
	return hcoutil.GetLabels(hcoutil.HyperConvergedName, component)
}

// GetLabelsDeprecated is the old form, that requires the HyperConverged CR. This is not really needed, as the CR name
// is known. and can be changed
//
// Deprecated: use GetLabels instead
func GetLabelsDeprecated(hc *hcov1beta1.HyperConverged, component hcoutil.AppComponent) map[string]string {
	hcoName := hcoutil.HyperConvergedName

	if hc.Name != "" {
		hcoName = hc.Name
	}

	return hcoutil.GetLabels(hcoName, component)
}
