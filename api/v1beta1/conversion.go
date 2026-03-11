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

	convert_v1beta1_FeatureGates_To_v1(&src.Spec.FeatureGates, &dst.Spec.FeatureGates)

	convertNodePlacementsV1beta1ToV1(src.Spec, &dst.Spec)

	return nil
}

func (dst *HyperConverged) ConvertFrom(srcRaw conversion.Hub) error { //revive:disable:receiver-naming
	src := srcRaw.(*hcov1.HyperConverged)

	if err := Convert_v1_HyperConverged_To_v1beta1_HyperConverged(src, dst, nil); err != nil {
		return fmt.Errorf("failed to convert HyperConverged from v1 to v1beta1; %w", err)
	}

	convert_v1_FeatureGates_To_v1beta1(src.Spec.FeatureGates, &dst.Spec.FeatureGates)

	convertNodePlacementsV1ToV1beta1(src.Spec, &dst.Spec)

	return nil
}

func convertNodePlacementsV1ToV1beta1(v1Spec hcov1.HyperConvergedSpec, v1beta1Spec *HyperConvergedSpec) {
	if v1Spec.NodePlacements == nil {
		return
	}

	if v1Spec.NodePlacements.Infra != nil {
		v1beta1Spec.Infra.NodePlacement = v1Spec.NodePlacements.Infra.DeepCopy()
	}

	if v1Spec.NodePlacements.Workload != nil {
		v1beta1Spec.Workloads.NodePlacement = v1Spec.NodePlacements.Workload.DeepCopy()
	}
}

func convertNodePlacementsV1beta1ToV1(v1beta1Spec HyperConvergedSpec, v1Spec *hcov1.HyperConvergedSpec) {
	if v1beta1Spec.Infra.NodePlacement != nil {
		v1Spec.NodePlacements = &hcov1.NodePlacements{
			Infra: v1beta1Spec.Infra.NodePlacement.DeepCopy(),
		}
	}

	if v1beta1Spec.Workloads.NodePlacement != nil {
		if v1Spec.NodePlacements == nil {
			v1Spec.NodePlacements = &hcov1.NodePlacements{}
		}

		v1Spec.NodePlacements.Workload = v1beta1Spec.Workloads.NodePlacement.DeepCopy()
	}
}
