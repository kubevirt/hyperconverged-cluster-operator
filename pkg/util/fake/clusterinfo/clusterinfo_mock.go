package clusterinfo

import (
	"context"

	"github.com/go-logr/logr"
	utilversion "k8s.io/apimachinery/pkg/util/version"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

type ClusterInfoMock struct {
	isOpenshift       bool
	runningLocally    bool
	isManagedByOLM    bool
	kubernetesVersion *utilversion.Version
}

// make sure ClusterInfoMock implements hcoutil.ClusterInfo
var _ hcoutil.ClusterInfo = &ClusterInfoMock{}

func New(opts ...MockOption) *ClusterInfoMock {
	mock := &ClusterInfoMock{
		kubernetesVersion: utilversion.MustParseSemantic("v1.36.0"),
	}

	for _, op := range opts {
		op(mock)
	}

	return mock
}

func NewGetClusterInfo(opts ...MockOption) func() hcoutil.ClusterInfo {
	ci := New(opts...)

	return func() hcoutil.ClusterInfo {
		return ci
	}
}

func (ClusterInfoMock) Init(_ context.Context, _ client.Client, _ logr.Logger, _ ...*rest.Config) error {
	return nil
}

func (c ClusterInfoMock) IsOpenshift() bool {
	return c.isOpenshift
}

func (c ClusterInfoMock) IsHyperShiftManaged() bool {
	return false
}

func (c ClusterInfoMock) IsRunningLocally() bool {
	return c.runningLocally
}

func (ClusterInfoMock) GetBaseDomain() string {
	return ""
}

func (c ClusterInfoMock) IsManagedByOLM() bool {
	return c.isManagedByOLM
}

func (ClusterInfoMock) IsConsolePluginImageProvided() bool {
	return true
}

func (ClusterInfoMock) IsMonitoringAvailable() bool {
	return true
}

func (ClusterInfoMock) IsDeschedulerAvailable() bool {
	return true
}

func (ClusterInfoMock) IsDeschedulerCRDDeployed(ctx context.Context, cl client.Client) bool {
	return true
}

func (ClusterInfoMock) IsSingleStackIPv6() bool {
	return true
}

func (c ClusterInfoMock) GetKubernetesVersion() *utilversion.Version {
	return c.kubernetesVersion
}
