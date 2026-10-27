package cmdcommon

import (
	"context"
	"errors"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/ownresources"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
	fakeclusterinfo "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util/fake/clusterinfo"
)

type failingClusterInfo struct {
	fakeclusterinfo.ClusterInfoMock
}

func (failingClusterInfo) Init(context.Context, client.Client, logr.Logger) error {
	return errors.New("context deadline exceeded")
}

// The webhook and the operator exit when ClusterInitializations fails, so
// the pod restarts. A failure it swallowed left the binary running without
// what it needs: the conversion webhook's CA controller was skipped, and the
// HyperConverged CRD's conversions failed with an unknown-authority error.
var _ = Describe("ClusterInitializations", func() {
	var (
		ownInitErr    error
		ownInitCalled bool
	)

	BeforeEach(func() {
		origGetClusterInfo, origOwnInit := hcoutil.GetClusterInfo, ownresources.Init
		DeferCleanup(func() {
			hcoutil.GetClusterInfo = origGetClusterInfo
			ownresources.Init = origOwnInit
		})

		hcoutil.GetClusterInfo = fakeclusterinfo.NewGetClusterInfo()
		ownInitErr, ownInitCalled = nil, false
		ownresources.Init = func(context.Context, client.Reader, *runtime.Scheme, logr.Logger) error {
			ownInitCalled = true
			return ownInitErr
		}
	})

	clusterInitializations := func(ctx context.Context) error {
		return ClusterInitializations(ctx, fake.NewClientBuilder().Build(), runtime.NewScheme(), GinkgoLogr)
	}

	It("should fail when the cluster can't be read", func(ctx context.Context) {
		hcoutil.GetClusterInfo = func() hcoutil.ClusterInfo { return failingClusterInfo{} }
		Expect(clusterInitializations(ctx)).To(MatchError("context deadline exceeded"))
	})

	It("should fail when its own resources can't be read", func(ctx context.Context) {
		ownInitErr = errors.New("can't get deployment")
		Expect(clusterInitializations(ctx)).To(MatchError("can't get deployment"))
	})

	It("should read its own resources and succeed", func(ctx context.Context) {
		Expect(clusterInitializations(ctx)).To(Succeed())
		Expect(ownInitCalled).To(BeTrue())
	})
})
