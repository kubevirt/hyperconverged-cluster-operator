package validator

import (
	"fmt"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	hcofg "github.com/kubevirt/hyperconverged-cluster-operator/api/v1/featuregates"
)

func validateNVIDIAAIE(hc, previous *hcov1.HyperConverged, imageAvailable bool) error {
	// Existing enabled installations may still update unrelated configuration or
	// disable AIE after an upgrade to a bundle that does not supply the image.
	if !imageAvailable && hc.Spec.FeatureGates.IsEnabled(hcofg.NVIDIAAIE) && (previous == nil || !previous.Spec.FeatureGates.IsEnabled(hcofg.NVIDIAAIE)) {
		return fmt.Errorf("cannot enable %s: this installation does not provide an AIE launcher image", hcofg.NVIDIAAIE)
	}
	return nil
}
