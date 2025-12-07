package ownresources

import (
	internal "github.com/kubevirt/hyperconverged-cluster-operator/pkg/internal/ownresources"
)

var (
	// GetPod returns the running pod, or nil if not exists
	GetPod = internal.GetPod

	// GetDeploymentRef returns the ObjectReference, pointing to the deployment that controls the running
	// pod, or nil if not exists
	GetDeploymentRef = internal.GetDeploymentRef

	// GetCSVRef returns the object reference of the CSV that defines the application, or nil if not exists
	GetCSVRef = internal.GetCSVRef

	// Init collect own references at startup
	Init = internal.Init
)
