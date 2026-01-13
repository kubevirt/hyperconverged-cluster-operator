package v1beta1

import (
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
)

//nolint:revive
func (src *HyperConverged) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*hcov1.HyperConverged)

	if err := Convert_v1beta1_HyperConverged_To_v1_HyperConverged(src, dst, nil); err != nil {
		return err
	}

	// TODO: add custom conversion

	return nil
}

//nolint:revive
func (dst *HyperConverged) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*hcov1.HyperConverged)

	if err := Convert_v1_HyperConverged_To_v1beta1_HyperConverged(src, dst, nil); err != nil {
		return err
	}

	// TODO: add custom conversion

	return nil
}
