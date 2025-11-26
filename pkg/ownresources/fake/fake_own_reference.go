package fakeownreferences

import (
	csvv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"

	internal "github.com/kubevirt/hyperconverged-cluster-operator/pkg/internal/ownresources"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/ownresources"
)

func ResetOwnReference() {
	ownresources.GetPod = internal.GetPod
	ownresources.GetDeploymentRef = internal.GetDeploymentRef
	ownresources.GetCSV = internal.GetCSV
	ownresources.Init = internal.Init
}

func OLMV0OwnerReferenceMock() {
	ownresources.GetPod = fakeGetPod
	ownresources.GetDeploymentRef = GetFakeDeploymentRef
	ownresources.GetCSV = func() *csvv1alpha1.ClusterServiceVersion { return GetCSV() }
	ownresources.Init = fakeInit
}

func NoOLMOwnerReferenceMock() {
	ownresources.GetPod = fakeGetPod
	ownresources.GetDeploymentRef = GetFakeDeploymentRef
	ownresources.GetCSV = func() *csvv1alpha1.ClusterServiceVersion { return nil }
	ownresources.Init = fakeInit
}
