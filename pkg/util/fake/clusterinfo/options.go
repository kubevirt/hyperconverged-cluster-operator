package clusterinfo

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

func WithMonitoringAvailable(monitoringAvailable bool) MockOption {
	return func(mock *ClusterInfoMock) {
		mock.monitoringAvailable = monitoringAvailable
	}
}
