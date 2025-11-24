package nodes

import (
	"context"
	"errors"
	"os"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
					Client:                       cl,
					nodeEvents:                   nodeEvents,
					HandleHyperShiftNodeLabeling: staleHyperShiftNodeLabeling,
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
					Client:                       cl,
					nodeEvents:                   nodeEvents,
					HandleHyperShiftNodeLabeling: staleHyperShiftNodeLabeling,
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
					Client:                       cl,
					nodeEvents:                   nodeEvents,
					HandleHyperShiftNodeLabeling: staleHyperShiftNodeLabeling,
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
					Client:                       cl,
					nodeEvents:                   nodeEvents,
					HandleHyperShiftNodeLabeling: staleHyperShiftNodeLabeling,
				}

				// Reconcile
				res, err := r.Reconcile(context.TODO(), request)
				Expect(err).To(HaveOccurred())
				Expect(res.IsZero()).To(BeTrue())
				Expect(nodeEvents).ToNot(Receive())
			})
		})

		Context("HyperShift Node Labeling", func() {
			It("Should label worker node when shouldLabelNodes is true", func() {
				workerNode := &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "worker-1",
						Labels: map[string]string{
							nodeinfo.LabelNodeRoleWorker: "",
						},
					},
				}

				hco := commontestutils.NewHco()
				resources := []client.Object{hco, workerNode}
				cl := commontestutils.InitClient(resources)

				r := &ReconcileNodeCounter{
					Client:                       cl,
					nodeEvents:                   nodeEvents,
					HandleHyperShiftNodeLabeling: HandleHyperShiftNodeLabeling,
				}

				nodeinfo.HandleNodeChanges = func(_ context.Context, _ client.Client, _ *hcov1beta1.HyperConverged, _ logr.Logger) (bool, error) {
					return false, nil
				}

				// Create a request for the worker node
				nodeRequest := reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name: "worker-1",
					},
				}

				// Reconcile
				res, err := r.Reconcile(context.TODO(), nodeRequest)
				Expect(err).ToNot(HaveOccurred())
				Expect(res.IsZero()).To(BeTrue())

				// Verify the node was labeled
				updatedNode := &corev1.Node{}
				err = cl.Get(context.TODO(), client.ObjectKey{Name: "worker-1"}, updatedNode)
				Expect(err).ToNot(HaveOccurred())
				Expect(updatedNode.Labels).To(HaveKey(nodeinfo.LabelNodeRoleControlPlane))
				Expect(updatedNode.Labels[nodeinfo.LabelNodeRoleControlPlane]).To(Equal(hypershiftLabelValue))
			})

			It("Should not label control plane node", func() {
				cpNode := &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "control-plane-1",
						Labels: map[string]string{
							nodeinfo.LabelNodeRoleControlPlane: "",
						},
					},
				}

				hco := commontestutils.NewHco()
				resources := []client.Object{hco, cpNode}
				cl := commontestutils.InitClient(resources)

				r := &ReconcileNodeCounter{
					Client:                       cl,
					nodeEvents:                   nodeEvents,
					HandleHyperShiftNodeLabeling: HandleHyperShiftNodeLabeling,
				}

				nodeinfo.HandleNodeChanges = func(_ context.Context, _ client.Client, _ *hcov1beta1.HyperConverged, _ logr.Logger) (bool, error) {
					return false, nil
				}

				nodeRequest := reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name: "control-plane-1",
					},
				}

				res, err := r.Reconcile(context.TODO(), nodeRequest)
				Expect(err).ToNot(HaveOccurred())
				Expect(res.IsZero()).To(BeTrue())

				// Verify the node label wasn't changed
				updatedNode := &corev1.Node{}
				err = cl.Get(context.TODO(), client.ObjectKey{Name: "control-plane-1"}, updatedNode)
				Expect(err).ToNot(HaveOccurred())
				Expect(updatedNode.Labels[nodeinfo.LabelNodeRoleControlPlane]).To(Equal(""))
			})

			It("Should skip labeling when shouldLabelNodes is false", func() {
				workerNode := &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "worker-1",
						Labels: map[string]string{
							nodeinfo.LabelNodeRoleWorker: "",
						},
					},
				}

				hco := commontestutils.NewHco()
				resources := []client.Object{hco, workerNode}
				cl := commontestutils.InitClient(resources)

				r := &ReconcileNodeCounter{
					Client:                       cl,
					nodeEvents:                   nodeEvents,
					HandleHyperShiftNodeLabeling: staleHyperShiftNodeLabeling,
				}

				nodeinfo.HandleNodeChanges = func(_ context.Context, _ client.Client, _ *hcov1beta1.HyperConverged, _ logr.Logger) (bool, error) {
					return false, nil
				}

				nodeRequest := reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name: "worker-1",
					},
				}

				res, err := r.Reconcile(context.TODO(), nodeRequest)
				Expect(err).ToNot(HaveOccurred())
				Expect(res.IsZero()).To(BeTrue())

				// Verify the node was not labeled
				updatedNode := &corev1.Node{}
				err = cl.Get(context.TODO(), client.ObjectKey{Name: "worker-1"}, updatedNode)
				Expect(err).ToNot(HaveOccurred())
				Expect(updatedNode.Labels).NotTo(HaveKey(nodeinfo.LabelNodeRoleControlPlane))
			})

			It("Should handle missing node gracefully", func() {
				hco := commontestutils.NewHco()
				resources := []client.Object{hco}
				cl := commontestutils.InitClient(resources)

				r := &ReconcileNodeCounter{
					Client:                       cl,
					nodeEvents:                   nodeEvents,
					HandleHyperShiftNodeLabeling: HandleHyperShiftNodeLabeling,
				}

				nodeinfo.HandleNodeChanges = func(_ context.Context, _ client.Client, _ *hcov1beta1.HyperConverged, _ logr.Logger) (bool, error) {
					return false, nil
				}

				nodeRequest := reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name: "non-existent-node",
					},
				}

				// Should not return error for missing node
				res, err := r.Reconcile(context.TODO(), nodeRequest)
				Expect(err).ToNot(HaveOccurred())
				Expect(res.IsZero()).To(BeTrue())
			})

			It("Should not label nodes for HCO events", func() {
				workerNode := &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "worker-1",
						Labels: map[string]string{
							nodeinfo.LabelNodeRoleWorker: "",
						},
					},
				}

				hco := commontestutils.NewHco()
				resources := []client.Object{hco, workerNode}
				cl := commontestutils.InitClient(resources)

				r := &ReconcileNodeCounter{
					Client:     cl,
					nodeEvents: nodeEvents,
				}

				nodeinfo.HandleNodeChanges = func(_ context.Context, _ client.Client, _ *hcov1beta1.HyperConverged, _ logr.Logger) (bool, error) {
					return false, nil
				}

				// Use hcoReq instead of node request
				res, err := r.Reconcile(context.TODO(), hcoReq)
				Expect(err).ToNot(HaveOccurred())
				Expect(res.IsZero()).To(BeTrue())

				// Verify the node was not labeled (HCO event should skip node labeling)
				updatedNode := &corev1.Node{}
				err = cl.Get(context.TODO(), client.ObjectKey{Name: "worker-1"}, updatedNode)
				Expect(err).ToNot(HaveOccurred())
				Expect(updatedNode.Labels).NotTo(HaveKey(nodeinfo.LabelNodeRoleControlPlane))
			})

			It("Should label all nodes at startup", func() {
				worker1 := &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "worker-1",
						Labels: map[string]string{
							nodeinfo.LabelNodeRoleWorker: "",
						},
					},
				}
				worker2 := &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "worker-2",
						Labels: map[string]string{
							nodeinfo.LabelNodeRoleWorker: "",
						},
					},
				}
				cpNode := &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "control-plane-1",
						Labels: map[string]string{
							nodeinfo.LabelNodeRoleControlPlane: "",
						},
					},
				}

				resources := []client.Object{worker1, worker2, cpNode}
				cl := commontestutils.InitClient(resources)

				r := &ReconcileNodeCounter{
					Client:     cl,
					nodeEvents: nodeEvents,
				}

				err := r.labelAllNodesAtStartup(context.TODO())
				Expect(err).ToNot(HaveOccurred())

				// Verify worker nodes were labeled
				updatedWorker1 := &corev1.Node{}
				err = cl.Get(context.TODO(), client.ObjectKey{Name: "worker-1"}, updatedWorker1)
				Expect(err).ToNot(HaveOccurred())
				Expect(updatedWorker1.Labels[nodeinfo.LabelNodeRoleControlPlane]).To(Equal(hypershiftLabelValue))

				updatedWorker2 := &corev1.Node{}
				err = cl.Get(context.TODO(), client.ObjectKey{Name: "worker-2"}, updatedWorker2)
				Expect(err).ToNot(HaveOccurred())
				Expect(updatedWorker2.Labels[nodeinfo.LabelNodeRoleControlPlane]).To(Equal(hypershiftLabelValue))

				// Verify control plane node was not changed
				updatedCP := &corev1.Node{}
				err = cl.Get(context.TODO(), client.ObjectKey{Name: "control-plane-1"}, updatedCP)
				Expect(err).ToNot(HaveOccurred())
				Expect(updatedCP.Labels[nodeinfo.LabelNodeRoleControlPlane]).To(Equal(""))
			})
		})

		Context("isWorkerNode helper", func() {
			It("Should identify worker node correctly", func() {
				workerNode := &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "worker-1",
						Labels: map[string]string{
							nodeinfo.LabelNodeRoleWorker: "",
						},
					},
				}
				Expect(isWorkerNode(workerNode)).To(BeTrue())
			})

			It("Should not identify control plane node as worker", func() {
				cpNode := &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "control-plane-1",
						Labels: map[string]string{
							nodeinfo.LabelNodeRoleControlPlane: "",
						},
					},
				}
				Expect(isWorkerNode(cpNode)).To(BeFalse())
			})

			It("Should not identify master node as worker", func() {
				masterNode := &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "master-1",
						Labels: map[string]string{
							nodeinfo.LabelNodeRoleMaster: "",
						},
					},
				}
				Expect(isWorkerNode(masterNode)).To(BeFalse())
			})

			It("Should not identify node with both worker and control-plane labels as worker", func() {
				mixedNode := &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "mixed-1",
						Labels: map[string]string{
							nodeinfo.LabelNodeRoleWorker:       "",
							nodeinfo.LabelNodeRoleControlPlane: "",
						},
					},
				}
				Expect(isWorkerNode(mixedNode)).To(BeFalse())
			})

			It("Should not identify node without labels as worker", func() {
				unlabeledNode := &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "unlabeled-1",
					},
				}
				Expect(isWorkerNode(unlabeledNode)).To(BeFalse())
			})
		})

	})
})
