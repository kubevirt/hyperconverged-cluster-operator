package cmdcommon

import (
	"context"
	"errors"
	"testing"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/ownresources"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
	fakeclusterinfo "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util/fake/clusterinfo"
)

// The webhook and the operator exit when ClusterInitializations fails, so
// the pod restarts. A failure it swallowed left the binary running without
// what it needs: the conversion webhook's CA controller was skipped, and the
// HyperConverged CRD's conversions failed with an unknown-authority error.

type failingClusterInfo struct {
	fakeclusterinfo.ClusterInfoMock
}

func (failingClusterInfo) Init(context.Context, client.Client, logr.Logger) error {
	return errors.New("context deadline exceeded")
}

func stub(t *testing.T, ci hcoutil.ClusterInfo, initErr error) (ownInitCalled *bool) {
	t.Helper()
	origCI, origInit := hcoutil.GetClusterInfo, ownresources.Init
	t.Cleanup(func() { hcoutil.GetClusterInfo, ownresources.Init = origCI, origInit })
	hcoutil.GetClusterInfo = func() hcoutil.ClusterInfo { return ci }
	called := false
	ownresources.Init = func(context.Context, client.Reader, *runtime.Scheme, logr.Logger) error {
		called = true
		return initErr
	}
	return &called
}

func TestClusterInitializationsFailsWhenTheClusterCantBeRead(t *testing.T) {
	stub(t, failingClusterInfo{}, nil)
	if err := ClusterInitializations(context.Background(), fake.NewClientBuilder().Build(), runtime.NewScheme(), logr.Discard()); err == nil {
		t.Fatal("got nil; a cluster-info failure must stop the binary")
	}
}

func TestClusterInitializationsFailsWhenItsOwnResourcesCantBeRead(t *testing.T) {
	called := stub(t, fakeclusterinfo.New(), errors.New("can't get deployment"))
	err := ClusterInitializations(context.Background(), fake.NewClientBuilder().Build(), runtime.NewScheme(), logr.Discard())
	if !*called {
		t.Fatal("own resources were never read")
	}
	if err == nil {
		t.Fatal("got nil; an own-resources failure must stop the binary")
	}
}

func TestClusterInitializationsSucceeds(t *testing.T) {
	stub(t, fakeclusterinfo.New(), nil)
	if err := ClusterInitializations(context.Background(), fake.NewClientBuilder().Build(), runtime.NewScheme(), logr.Discard()); err != nil {
		t.Fatalf("got %v", err)
	}
}
