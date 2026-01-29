package v1beta1

import (
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	"github.com/kubevirt/hyperconverged-cluster-operator/api/v1/featuregates"
)

//nolint:revive
func (src *HyperConverged) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*hcov1.HyperConverged)

	if err := Convert_v1beta1_HyperConverged_To_v1_HyperConverged(src, dst, nil); err != nil {
		return err
	}

	dst.Spec.FeatureGates = ConvertV1Beta1FGsToV1FGs(src.Spec.FeatureGates)

	return nil
}

//nolint:revive
func (dst *HyperConverged) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*hcov1.HyperConverged)

	if err := Convert_v1_HyperConverged_To_v1beta1_HyperConverged(src, dst, nil); err != nil {
		return err
	}

	ConvertV1FGsToV1Beta1FGs(src.Spec.FeatureGates, &dst.Spec.FeatureGates)

	return nil
}

func ConvertV1Beta1FGsToV1FGs(src HyperConvergedFeatureGates) featuregates.HyperConvergedFeatureGates {
	var v1FGs featuregates.HyperConvergedFeatureGates

	if ptr.Deref(src.DownwardMetrics, false) {
		v1FGs.Enable("downwardMetrics")
	}

	if ptr.Deref(src.DeployKubeSecondaryDNS, false) {
		v1FGs.Enable("deployKubeSecondaryDNS")
	}

	if ptr.Deref(src.DisableMDevConfiguration, false) {
		v1FGs.Enable("disableMDevConfiguration")
	}

	if ptr.Deref(src.PersistentReservation, false) {
		v1FGs.Enable("persistentReservation")
	}

	if ptr.Deref(src.AlignCPUs, false) {
		v1FGs.Enable("alignCPUs")
	}

	if ptr.Deref(src.EnableMultiArchBootImageImport, false) {
		v1FGs.Enable("enableMultiArchBootImageImport")
	}

	if ptr.Deref(src.DecentralizedLiveMigration, false) {
		v1FGs.Enable("decentralizedLiveMigration")
	}

	if ptr.Deref(src.DeclarativeHotplugVolumes, false) {
		v1FGs.Enable("declarativeHotplugVolumes")
	}

	if !ptr.Deref(src.VideoConfig, true) {
		v1FGs.Disable("videoConfig")
	}

	if ptr.Deref(src.ObjectGraph, false) {
		v1FGs.Enable("objectGraph")
	}

	return v1FGs
}

func ConvertV1FGsToV1Beta1FGs(src featuregates.HyperConvergedFeatureGates, dst *HyperConvergedFeatureGates) {
	dst.DownwardMetrics = ptr.To(src.IsEnabled("downwardMetrics"))
	dst.DeployKubeSecondaryDNS = ptr.To(src.IsEnabled("deployKubeSecondaryDNS"))
	dst.DisableMDevConfiguration = ptr.To(src.IsEnabled("disableMDevConfiguration"))
	dst.PersistentReservation = ptr.To(src.IsEnabled("persistentReservation"))
	dst.AlignCPUs = ptr.To(src.IsEnabled("alignCPUs"))
	dst.EnableMultiArchBootImageImport = ptr.To(src.IsEnabled("enableMultiArchBootImageImport"))
	dst.DecentralizedLiveMigration = ptr.To(src.IsEnabled("decentralizedLiveMigration"))
	dst.DeclarativeHotplugVolumes = ptr.To(src.IsEnabled("declarativeHotplugVolumes"))
	dst.VideoConfig = ptr.To(src.IsEnabled("videoConfig"))
	dst.ObjectGraph = ptr.To(src.IsEnabled("objectGraph"))
}
