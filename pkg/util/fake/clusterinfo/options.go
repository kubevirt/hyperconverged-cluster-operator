package clusterinfo

import utilversion "k8s.io/apimachinery/pkg/util/version"

type MockOption func(*ClusterInfoMock)

func WithIsOpenshift(isOpenshift bool) MockOption {
	return func(mock *ClusterInfoMock) {
		mock.isOpenshift = isOpenshift
	}
}

func WithRunningLocally(runningLocally bool) MockOption {
	return func(mock *ClusterInfoMock) {
		mock.runningLocally = runningLocally
	}
}

func WithIsManagedByOLM(isManagedByOLMV0 bool) MockOption {
	return func(mock *ClusterInfoMock) {
		mock.isManagedByOLM = isManagedByOLMV0
	}
}

func WithKubernetesVersion(version *utilversion.Version) MockOption {
	return func(mock *ClusterInfoMock) {
		mock.kubernetesVersion = version
	}
}
