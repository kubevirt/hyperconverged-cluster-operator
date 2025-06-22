package nodes

import (
	"context"
	"errors"
	"os"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/nodeinfo"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

// Mock TestRequest to simulate Reconcile() being called on an event for a watched resource
var (
	request = reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "nodes-controller",
			Namespace: commontestutils.Namespace,
		},
	}
)

var _ = Describe("NodesController", func() {
	Describe("Reconcile NodesController", func() {

		var nodeEvents chan event.GenericEvent
		BeforeEach(func() {
			nodeEvents = make(chan event.GenericEvent, 1)
			DeferCleanup(func() {
				close(nodeEvents)
			})
		})

		origHandleNodeChanges := nodeinfo.HandleNodeChanges
		AfterEach(func() {
			nodeinfo.HandleNodeChanges = origHandleNodeChanges

			_ = os.Setenv(hcoutil.OperatorNamespaceEnv, commontestutils.Namespace)
		})

		Context("Node Count Change", func() {
			It("Should send event if nodeInfo was changed", func() {
				nodeinfo.HandleNodeChanges = func(_ context.Context, _ client.Client, _ *hcov1beta1.HyperConverged, _ logr.Logger) (bool, error) {
					return true, nil
				}

				hco := commontestutils.NewHco()
				resources := []client.Object{hco}
				cl := commontestutils.InitClient(resources)

				r := &ReconcileNodeCounter{
					Client:     cl,
					nodeEvents: nodeEvents,
				}

				// Reconcile
				res, err := r.Reconcile(context.TODO(), request)
				Expect(err).ToNot(HaveOccurred())
				Expect(res.IsZero()).To(BeTrue())
				Expect(nodeEvents).To(Receive())
			})

			It("Should not send event if nodeInfo was changed, but there is no HC CR", func() {
				nodeinfo.HandleNodeChanges = func(_ context.Context, _ client.Client, _ *hcov1beta1.HyperConverged, _ logr.Logger) (bool, error) {
					return true, nil
				}

				cl := commontestutils.InitClient(nil)

				r := &ReconcileNodeCounter{
					Client:     cl,
					nodeEvents: nodeEvents,
				}

				// Reconcile
				res, err := r.Reconcile(context.TODO(), request)
				Expect(err).ToNot(HaveOccurred())
				Expect(res.IsZero()).To(BeTrue())
				Expect(nodeEvents).ToNot(Receive())
			})

			It("Should not send event if nodeInfo was not changed", func() {
				nodeinfo.HandleNodeChanges = func(_ context.Context, _ client.Client, _ *hcov1beta1.HyperConverged, _ logr.Logger) (bool, error) {
					return false, nil
				}

				hco := commontestutils.NewHco()
				resources := []client.Object{hco}
				cl := commontestutils.InitClient(resources)

				r := &ReconcileNodeCounter{
					Client:     cl,
					nodeEvents: nodeEvents,
				}

				// Reconcile
				res, err := r.Reconcile(context.TODO(), request)
				Expect(err).ToNot(HaveOccurred())
				Expect(res.IsZero()).To(BeTrue())
				Expect(nodeEvents).ToNot(Receive())
			})

			It("Should return error is failed to handle nodeInfo", func() {
				nodeinfo.HandleNodeChanges = func(_ context.Context, _ client.Client, _ *hcov1beta1.HyperConverged, _ logr.Logger) (bool, error) {
					return false, errors.New("fake error")
				}

				hco := commontestutils.NewHco()
				resources := []client.Object{hco}
				cl := commontestutils.InitClient(resources)

				r := &ReconcileNodeCounter{
					Client:     cl,
					nodeEvents: nodeEvents,
				}

				// Reconcile
				res, err := r.Reconcile(context.TODO(), request)
				Expect(err).To(HaveOccurred())
				Expect(res.IsZero()).To(BeTrue())
				Expect(nodeEvents).ToNot(Receive())
			})
		})

	})
})
