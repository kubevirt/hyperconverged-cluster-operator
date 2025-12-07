package fakeownresources

import (
	corev1 "k8s.io/api/core/v1"

	internal "github.com/kubevirt/hyperconverged-cluster-operator/pkg/internal/ownresources"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/ownresources"
)

func ResetOwnResources() {
	ownresources.GetPod = internal.GetPod
	ownresources.GetDeploymentRef = internal.GetDeploymentRef
	ownresources.GetCSVRef = internal.GetCSVRef
	ownresources.Init = internal.Init
}

func OLMV0OwnResourcesMock() {
	ownresources.GetPod = fakeGetPod
	ownresources.GetDeploymentRef = GetFakeDeploymentRef
	ownresources.GetCSVRef = func() *corev1.ObjectReference { return GetCSVRef() }
	ownresources.Init = fakeInit
}

func NoOLMOwnerResourcesMock() {
	ownresources.GetPod = fakeGetPod
	ownresources.GetDeploymentRef = GetFakeDeploymentRef
	ownresources.GetCSVRef = func() *corev1.ObjectReference { return nil }
	ownresources.Init = fakeInit
}
