package v1beta1

import (
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/conversion"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
)

// Implement the conversion.Convertible interface, to be used in the conversion webhook.

func (src *HyperConverged) ConvertTo(dstRaw conversion.Hub) error { //revive:disable:receiver-naming
	dst := dstRaw.(*hcov1.HyperConverged)

	if err := Convert_v1beta1_HyperConverged_To_v1_HyperConverged(src, dst, nil); err != nil {
		return fmt.Errorf("failed to convert HyperConverged from v1beta1 to v1; %w", err)
	}

	// TODO: Add manual/custom conversion logic here

	return nil
}

func (dst *HyperConverged) ConvertFrom(srcRaw conversion.Hub) error { //revive:disable:receiver-naming
	src := srcRaw.(*hcov1.HyperConverged)

	if err := Convert_v1_HyperConverged_To_v1beta1_HyperConverged(src, dst, nil); err != nil {
		return fmt.Errorf("failed to convert HyperConverged from v1 to v1beta1; %w", err)
	}

	// TODO: Add manual/custom conversion logic here

	return nil
}
