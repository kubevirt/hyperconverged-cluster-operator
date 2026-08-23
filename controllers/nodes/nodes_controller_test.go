package nodes

import (
	"context"
	"errors"
	"os"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
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
				nodeinfo.HandleNodeChanges = func(_ context.Context, _ client.Client, _ *hcov1.HyperConverged, _ logr.Logger) (bool, error) {
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
				nodeinfo.HandleNodeChanges = func(_ context.Context, _ client.Client, _ *hcov1.HyperConverged, _ logr.Logger) (bool, error) {
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
				nodeinfo.HandleNodeChanges = func(_ context.Context, _ client.Client, _ *hcov1.HyperConverged, _ logr.Logger) (bool, error) {
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
				nodeinfo.HandleNodeChanges = func(_ context.Context, _ client.Client, _ *hcov1.HyperConverged, _ logr.Logger) (bool, error) {
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
			It("Should label worker node with kubevirt control-plane label", func() {
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

				nodeinfo.HandleNodeChanges = func(_ context.Context, _ client.Client, _ *hcov1.HyperConverged, _ logr.Logger) (bool, error) {
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

				updatedNode := &corev1.Node{}
				err = cl.Get(context.TODO(), client.ObjectKey{Name: "worker-1"}, updatedNode)
				Expect(err).ToNot(HaveOccurred())
				Expect(updatedNode.Labels).To(HaveKey(nodeinfo.LabelNodeRoleKubevirtControlPlane))
				Expect(updatedNode.Labels[nodeinfo.LabelNodeRoleKubevirtControlPlane]).To(Equal(hypershiftLabelValue))
				Expect(updatedNode.Labels).NotTo(HaveKey(nodeinfo.LabelNodeRoleControlPlane))
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

				nodeinfo.HandleNodeChanges = func(_ context.Context, _ client.Client, _ *hcov1.HyperConverged, _ logr.Logger) (bool, error) {
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

				updatedNode := &corev1.Node{}
				err = cl.Get(context.TODO(), client.ObjectKey{Name: "control-plane-1"}, updatedNode)
				Expect(err).ToNot(HaveOccurred())
				Expect(updatedNode.Labels[nodeinfo.LabelNodeRoleControlPlane]).To(Equal(""))
				Expect(updatedNode.Labels).NotTo(HaveKey(nodeinfo.LabelNodeRoleKubevirtControlPlane))
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

				nodeinfo.HandleNodeChanges = func(_ context.Context, _ client.Client, _ *hcov1.HyperConverged, _ logr.Logger) (bool, error) {
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

				updatedNode := &corev1.Node{}
				err = cl.Get(context.TODO(), client.ObjectKey{Name: "worker-1"}, updatedNode)
				Expect(err).ToNot(HaveOccurred())
				Expect(updatedNode.Labels).NotTo(HaveKey(nodeinfo.LabelNodeRoleKubevirtControlPlane))
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

				nodeinfo.HandleNodeChanges = func(_ context.Context, _ client.Client, _ *hcov1.HyperConverged, _ logr.Logger) (bool, error) {
					return false, nil
				}

				nodeRequest := reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name: "non-existent-node",
					},
				}

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

				nodeinfo.HandleNodeChanges = func(_ context.Context, _ client.Client, _ *hcov1.HyperConverged, _ logr.Logger) (bool, error) {
					return false, nil
				}

				res, err := r.Reconcile(context.TODO(), hcoReq)
				Expect(err).ToNot(HaveOccurred())
				Expect(res.IsZero()).To(BeTrue())

				updatedNode := &corev1.Node{}
				err = cl.Get(context.TODO(), client.ObjectKey{Name: "worker-1"}, updatedNode)
				Expect(err).ToNot(HaveOccurred())
				Expect(updatedNode.Labels).NotTo(HaveKey(nodeinfo.LabelNodeRoleKubevirtControlPlane))
				Expect(updatedNode.Labels).NotTo(HaveKey(nodeinfo.LabelNodeRoleControlPlane))
			})

			It("Should label all nodes at startup with kubevirt control-plane label", func() {
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
					Client:                       cl,
					nodeEvents:                   nodeEvents,
					HandleHyperShiftNodeLabeling: HandleHyperShiftNodeLabeling,
				}

				err := r.labelAllNodesAtStartup(context.TODO())
				Expect(err).ToNot(HaveOccurred())

				updatedWorker1 := &corev1.Node{}
				err = cl.Get(context.TODO(), client.ObjectKey{Name: "worker-1"}, updatedWorker1)
				Expect(err).ToNot(HaveOccurred())
				Expect(updatedWorker1.Labels[nodeinfo.LabelNodeRoleKubevirtControlPlane]).To(Equal(hypershiftLabelValue))
				Expect(updatedWorker1.Labels).NotTo(HaveKey(nodeinfo.LabelNodeRoleControlPlane))

				updatedWorker2 := &corev1.Node{}
				err = cl.Get(context.TODO(), client.ObjectKey{Name: "worker-2"}, updatedWorker2)
				Expect(err).ToNot(HaveOccurred())
				Expect(updatedWorker2.Labels[nodeinfo.LabelNodeRoleKubevirtControlPlane]).To(Equal(hypershiftLabelValue))
				Expect(updatedWorker2.Labels).NotTo(HaveKey(nodeinfo.LabelNodeRoleControlPlane))

				// Verify control plane node was not changed
				updatedCP := &corev1.Node{}
				err = cl.Get(context.TODO(), client.ObjectKey{Name: "control-plane-1"}, updatedCP)
				Expect(err).ToNot(HaveOccurred())
				Expect(updatedCP.Labels[nodeinfo.LabelNodeRoleControlPlane]).To(Equal(""))
				Expect(updatedCP.Labels).NotTo(HaveKey(nodeinfo.LabelNodeRoleKubevirtControlPlane))
			})

			Context("Upgrade scenarios", func() {
				It("Should replace old kubernetes control-plane label with kubevirt label on upgrade", func() {
					workerNode := &corev1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name: "worker-1",
							Labels: map[string]string{
								nodeinfo.LabelNodeRoleWorker:       "",
								nodeinfo.LabelNodeRoleControlPlane: hypershiftLabelValue,
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

					nodeinfo.HandleNodeChanges = func(_ context.Context, _ client.Client, _ *hcov1.HyperConverged, _ logr.Logger) (bool, error) {
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

					updatedNode := &corev1.Node{}
					err = cl.Get(context.TODO(), client.ObjectKey{Name: "worker-1"}, updatedNode)
					Expect(err).ToNot(HaveOccurred())
					Expect(updatedNode.Labels).To(HaveKey(nodeinfo.LabelNodeRoleKubevirtControlPlane))
					Expect(updatedNode.Labels[nodeinfo.LabelNodeRoleKubevirtControlPlane]).To(Equal(hypershiftLabelValue))
					Expect(updatedNode.Labels).NotTo(HaveKey(nodeinfo.LabelNodeRoleControlPlane))
				})

				It("Should replace old label on all nodes at startup (upgrade)", func() {
					worker1 := &corev1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name: "worker-1",
							Labels: map[string]string{
								nodeinfo.LabelNodeRoleWorker:       "",
								nodeinfo.LabelNodeRoleControlPlane: hypershiftLabelValue,
							},
						},
					}
					worker2 := &corev1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name: "worker-2",
							Labels: map[string]string{
								nodeinfo.LabelNodeRoleWorker:       "",
								nodeinfo.LabelNodeRoleControlPlane: hypershiftLabelValue,
							},
						},
					}

					resources := []client.Object{worker1, worker2}
					cl := commontestutils.InitClient(resources)

					r := &ReconcileNodeCounter{
						Client:                       cl,
						nodeEvents:                   nodeEvents,
						HandleHyperShiftNodeLabeling: HandleHyperShiftNodeLabeling,
					}

					err := r.labelAllNodesAtStartup(context.TODO())
					Expect(err).ToNot(HaveOccurred())

					for _, name := range []string{"worker-1", "worker-2"} {
						updatedNode := &corev1.Node{}
						err = cl.Get(context.TODO(), client.ObjectKey{Name: name}, updatedNode)
						Expect(err).ToNot(HaveOccurred())
						Expect(updatedNode.Labels[nodeinfo.LabelNodeRoleKubevirtControlPlane]).To(Equal(hypershiftLabelValue))
						Expect(updatedNode.Labels).NotTo(HaveKey(nodeinfo.LabelNodeRoleControlPlane))
					}
				})

				It("Should not remove kubernetes control-plane label if not set by HCO", func() {
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

					nodeinfo.HandleNodeChanges = func(_ context.Context, _ client.Client, _ *hcov1.HyperConverged, _ logr.Logger) (bool, error) {
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

					updatedNode := &corev1.Node{}
					err = cl.Get(context.TODO(), client.ObjectKey{Name: "control-plane-1"}, updatedNode)
					Expect(err).ToNot(HaveOccurred())
					Expect(updatedNode.Labels[nodeinfo.LabelNodeRoleControlPlane]).To(Equal(""))
					Expect(updatedNode.Labels).NotTo(HaveKey(nodeinfo.LabelNodeRoleKubevirtControlPlane))
				})

				It("Should not patch node already having the new label and no old label", func() {
					workerNode := &corev1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name: "worker-1",
							Labels: map[string]string{
								nodeinfo.LabelNodeRoleWorker:               "",
								nodeinfo.LabelNodeRoleKubevirtControlPlane: hypershiftLabelValue,
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

					nodeinfo.HandleNodeChanges = func(_ context.Context, _ client.Client, _ *hcov1.HyperConverged, _ logr.Logger) (bool, error) {
						return false, nil
					}

					nodeRequest := reconcile.Request{
						NamespacedName: types.NamespacedName{
							Name: "worker-1",
						},
					}

					origNode := &corev1.Node{}
					err := cl.Get(context.TODO(), client.ObjectKey{Name: "worker-1"}, origNode)
					Expect(err).ToNot(HaveOccurred())
					originalRV := origNode.ResourceVersion

					res, err := r.Reconcile(context.TODO(), nodeRequest)
					Expect(err).ToNot(HaveOccurred())
					Expect(res.IsZero()).To(BeTrue())

					updatedNode := &corev1.Node{}
					err = cl.Get(context.TODO(), client.ObjectKey{Name: "worker-1"}, updatedNode)
					Expect(err).ToNot(HaveOccurred())
					Expect(updatedNode.ResourceVersion).To(Equal(originalRV), "node should not have been patched")
					Expect(updatedNode.Labels[nodeinfo.LabelNodeRoleKubevirtControlPlane]).To(Equal(hypershiftLabelValue))
					Expect(updatedNode.Labels).NotTo(HaveKey(nodeinfo.LabelNodeRoleControlPlane))
				})
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

			It("Should not identify node with both worker and non-HCO control-plane labels as worker", func() {
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

			It("Should identify worker node with HCO-set control-plane label as worker (upgrade path)", func() {
				upgradedNode := &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "worker-hco-labeled",
						Labels: map[string]string{
							nodeinfo.LabelNodeRoleWorker:       "",
							nodeinfo.LabelNodeRoleControlPlane: hypershiftLabelValue,
						},
					},
				}
				Expect(isWorkerNode(upgradedNode)).To(BeTrue())
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

	Context("Classic OCP operator node placement", func() {
		const infraLabel = "node-role.kubernetes.io/infra"

		var nodeEvents chan event.GenericEvent
		BeforeEach(func() {
			nodeEvents = make(chan event.GenericEvent, 1)
			Expect(os.Setenv(hcoutil.OperatorNamespaceEnv, commontestutils.Namespace)).To(Succeed())
			DeferCleanup(func() {
				close(nodeEvents)
			})
		})

		origHandleNodeChanges := nodeinfo.HandleNodeChanges
		AfterEach(func() {
			nodeinfo.HandleNodeChanges = origHandleNodeChanges
		})

		infraNode := func() *corev1.Node {
			return &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "infra-1",
					Labels: map[string]string{
						nodeinfo.LabelNodeRoleWorker: "",
						infraLabel:                   "",
						corev1.LabelOSStable:         "linux",
					},
				},
			}
		}

		workerNode := func() *corev1.Node {
			return &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "worker-1",
					Labels: map[string]string{
						nodeinfo.LabelNodeRoleWorker: "",
						corev1.LabelOSStable:         "linux",
					},
				},
			}
		}

		cpNode := func() *corev1.Node {
			return &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "control-plane-1",
					Labels: map[string]string{
						nodeinfo.LabelNodeRoleControlPlane: "",
						corev1.LabelOSStable:               "linux",
					},
				},
			}
		}

		hcoOperatorDep := func(selector map[string]string) *appsv1.Deployment {
			return &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      hcoutil.OperatorName,
					Namespace: commontestutils.Namespace,
				},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							NodeSelector: selector,
						},
					},
				},
			}
		}

		placementReconciler := func(cl client.Client) *ReconcileNodeCounter {
			return &ReconcileNodeCounter{
				Client:                                 cl,
				nodeEvents:                             nodeEvents,
				HandleClassicOperatorPlacementLabeling: HandleClassicOperatorPlacementLabeling,
			}
		}

		It("Should label infra nodes matching HCO operator nodeSelector", func() {
			hco := commontestutils.NewHco()
			resources := []client.Object{
				hco,
				infraNode(),
				workerNode(),
				cpNode(),
				hcoOperatorDep(map[string]string{
					corev1.LabelOSStable: "linux",
					infraLabel:           "",
				}),
			}
			cl := commontestutils.InitClient(resources)

			nodeinfo.HandleNodeChanges = func(_ context.Context, _ client.Client, _ *hcov1.HyperConverged, _ logr.Logger) (bool, error) {
				return false, nil
			}

			res, err := placementReconciler(cl).Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "infra-1"}})
			Expect(err).ToNot(HaveOccurred())
			Expect(res.IsZero()).To(BeTrue())

			updatedInfra := &corev1.Node{}
			err = cl.Get(context.TODO(), client.ObjectKey{Name: "infra-1"}, updatedInfra)
			Expect(err).ToNot(HaveOccurred())
			Expect(updatedInfra.Labels[nodeinfo.LabelNodeRoleKubevirtControlPlane]).To(Equal(hypershiftLabelValue))
		})

		It("Should not label workers that do not match the selector", func() {
			hco := commontestutils.NewHco()
			resources := []client.Object{
				hco,
				workerNode(),
				hcoOperatorDep(map[string]string{
					corev1.LabelOSStable: "linux",
					infraLabel:           "",
				}),
			}
			cl := commontestutils.InitClient(resources)

			nodeinfo.HandleNodeChanges = func(_ context.Context, _ client.Client, _ *hcov1.HyperConverged, _ logr.Logger) (bool, error) {
				return false, nil
			}

			res, err := placementReconciler(cl).Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "worker-1"}})
			Expect(err).ToNot(HaveOccurred())
			Expect(res.IsZero()).To(BeTrue())

			updatedWorker := &corev1.Node{}
			err = cl.Get(context.TODO(), client.ObjectKey{Name: "worker-1"}, updatedWorker)
			Expect(err).ToNot(HaveOccurred())
			Expect(updatedWorker.Labels).NotTo(HaveKey(nodeinfo.LabelNodeRoleKubevirtControlPlane))
		})

		It("Should not label kubernetes control-plane nodes", func() {
			hco := commontestutils.NewHco()
			resources := []client.Object{
				hco,
				cpNode(),
				hcoOperatorDep(map[string]string{
					corev1.LabelOSStable: "linux",
					infraLabel:           "",
				}),
			}
			cl := commontestutils.InitClient(resources)

			nodeinfo.HandleNodeChanges = func(_ context.Context, _ client.Client, _ *hcov1.HyperConverged, _ logr.Logger) (bool, error) {
				return false, nil
			}

			res, err := placementReconciler(cl).Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "control-plane-1"}})
			Expect(err).ToNot(HaveOccurred())
			Expect(res.IsZero()).To(BeTrue())

			updatedCP := &corev1.Node{}
			err = cl.Get(context.TODO(), client.ObjectKey{Name: "control-plane-1"}, updatedCP)
			Expect(err).ToNot(HaveOccurred())
			Expect(updatedCP.Labels).NotTo(HaveKey(nodeinfo.LabelNodeRoleKubevirtControlPlane))
		})

		It("Should not label nodes when the operator only has the default OS selector", func() {
			hco := commontestutils.NewHco()
			resources := []client.Object{
				hco,
				infraNode(),
				hcoOperatorDep(map[string]string{
					corev1.LabelOSStable: "linux",
				}),
			}
			cl := commontestutils.InitClient(resources)

			nodeinfo.HandleNodeChanges = func(_ context.Context, _ client.Client, _ *hcov1.HyperConverged, _ logr.Logger) (bool, error) {
				return false, nil
			}

			res, err := placementReconciler(cl).Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "infra-1"}})
			Expect(err).ToNot(HaveOccurred())
			Expect(res.IsZero()).To(BeTrue())

			updatedInfra := &corev1.Node{}
			err = cl.Get(context.TODO(), client.ObjectKey{Name: "infra-1"}, updatedInfra)
			Expect(err).ToNot(HaveOccurred())
			Expect(updatedInfra.Labels).NotTo(HaveKey(nodeinfo.LabelNodeRoleKubevirtControlPlane))
		})

		It("Should not label nodes when the HCO operator Deployment is missing", func() {
			hco := commontestutils.NewHco()
			resources := []client.Object{hco, infraNode()}
			cl := commontestutils.InitClient(resources)

			nodeinfo.HandleNodeChanges = func(_ context.Context, _ client.Client, _ *hcov1.HyperConverged, _ logr.Logger) (bool, error) {
				return false, nil
			}

			res, err := placementReconciler(cl).Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "infra-1"}})
			Expect(err).ToNot(HaveOccurred())
			Expect(res.IsZero()).To(BeTrue())

			updatedInfra := &corev1.Node{}
			err = cl.Get(context.TODO(), client.ObjectKey{Name: "infra-1"}, updatedInfra)
			Expect(err).ToNot(HaveOccurred())
			Expect(updatedInfra.Labels).NotTo(HaveKey(nodeinfo.LabelNodeRoleKubevirtControlPlane))
		})

		It("Should not patch a node that already has the placement label", func() {
			node := infraNode()
			node.Labels[nodeinfo.LabelNodeRoleKubevirtControlPlane] = hypershiftLabelValue
			hco := commontestutils.NewHco()
			resources := []client.Object{
				hco,
				node,
				hcoOperatorDep(map[string]string{
					corev1.LabelOSStable: "linux",
					infraLabel:           "",
				}),
			}
			cl := commontestutils.InitClient(resources)

			nodeinfo.HandleNodeChanges = func(_ context.Context, _ client.Client, _ *hcov1.HyperConverged, _ logr.Logger) (bool, error) {
				return false, nil
			}

			origNode := &corev1.Node{}
			err := cl.Get(context.TODO(), client.ObjectKey{Name: "infra-1"}, origNode)
			Expect(err).ToNot(HaveOccurred())
			originalRV := origNode.ResourceVersion

			res, err := placementReconciler(cl).Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "infra-1"}})
			Expect(err).ToNot(HaveOccurred())
			Expect(res.IsZero()).To(BeTrue())

			updatedInfra := &corev1.Node{}
			err = cl.Get(context.TODO(), client.ObjectKey{Name: "infra-1"}, updatedInfra)
			Expect(err).ToNot(HaveOccurred())
			Expect(updatedInfra.ResourceVersion).To(Equal(originalRV), "node should not have been patched")
			Expect(updatedInfra.Labels[nodeinfo.LabelNodeRoleKubevirtControlPlane]).To(Equal(hypershiftLabelValue))
		})

		It("Should not patch an unlabeled node that should stay unlabeled", func() {
			hco := commontestutils.NewHco()
			resources := []client.Object{
				hco,
				workerNode(),
				hcoOperatorDep(map[string]string{
					corev1.LabelOSStable: "linux",
					infraLabel:           "",
				}),
			}
			cl := commontestutils.InitClient(resources)

			nodeinfo.HandleNodeChanges = func(_ context.Context, _ client.Client, _ *hcov1.HyperConverged, _ logr.Logger) (bool, error) {
				return false, nil
			}

			origNode := &corev1.Node{}
			err := cl.Get(context.TODO(), client.ObjectKey{Name: "worker-1"}, origNode)
			Expect(err).ToNot(HaveOccurred())
			originalRV := origNode.ResourceVersion

			res, err := placementReconciler(cl).Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "worker-1"}})
			Expect(err).ToNot(HaveOccurred())
			Expect(res.IsZero()).To(BeTrue())

			updatedWorker := &corev1.Node{}
			err = cl.Get(context.TODO(), client.ObjectKey{Name: "worker-1"}, updatedWorker)
			Expect(err).ToNot(HaveOccurred())
			Expect(updatedWorker.ResourceVersion).To(Equal(originalRV), "node should not have been patched")
			Expect(updatedWorker.Labels).NotTo(HaveKey(nodeinfo.LabelNodeRoleKubevirtControlPlane))
		})

		It("Should return an error when getting the operator Deployment fails", func() {
			hco := commontestutils.NewHco()
			resources := []client.Object{
				hco,
				infraNode(),
				hcoOperatorDep(map[string]string{
					infraLabel: "",
				}),
			}
			cl := commontestutils.InitClient(resources)
			cl.InitiateGetErrors(func(key client.ObjectKey) error {
				if key.Name == hcoutil.OperatorName {
					return apierrors.NewInternalError(errors.New("boom"))
				}
				return nil
			})

			nodeinfo.HandleNodeChanges = func(_ context.Context, _ client.Client, _ *hcov1.HyperConverged, _ logr.Logger) (bool, error) {
				return false, nil
			}

			_, err := placementReconciler(cl).Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "infra-1"}})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to get " + hcoutil.OperatorName + " deployment"))
		})

		It("Should remove the HCO kubevirt control-plane label when custom placement is cleared", func() {
			node := infraNode()
			node.Labels[nodeinfo.LabelNodeRoleKubevirtControlPlane] = hypershiftLabelValue
			hco := commontestutils.NewHco()
			resources := []client.Object{
				hco,
				node,
				hcoOperatorDep(map[string]string{
					corev1.LabelOSStable: "linux",
				}),
			}
			cl := commontestutils.InitClient(resources)

			nodeinfo.HandleNodeChanges = func(_ context.Context, _ client.Client, _ *hcov1.HyperConverged, _ logr.Logger) (bool, error) {
				return false, nil
			}

			res, err := placementReconciler(cl).Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "infra-1"}})
			Expect(err).ToNot(HaveOccurred())
			Expect(res.IsZero()).To(BeTrue())

			updatedInfra := &corev1.Node{}
			err = cl.Get(context.TODO(), client.ObjectKey{Name: "infra-1"}, updatedInfra)
			Expect(err).ToNot(HaveOccurred())
			Expect(updatedInfra.Labels).NotTo(HaveKey(nodeinfo.LabelNodeRoleKubevirtControlPlane))
		})

		It("Should label all matching nodes on a placementReq", func() {
			hco := commontestutils.NewHco()
			resources := []client.Object{
				hco,
				infraNode(),
				workerNode(),
				hcoOperatorDep(map[string]string{
					corev1.LabelOSStable: "linux",
					infraLabel:           "",
				}),
			}
			cl := commontestutils.InitClient(resources)

			nodeinfo.HandleNodeChanges = func(_ context.Context, _ client.Client, _ *hcov1.HyperConverged, _ logr.Logger) (bool, error) {
				return false, nil
			}

			res, err := placementReconciler(cl).Reconcile(context.TODO(), placementReq)
			Expect(err).ToNot(HaveOccurred())
			Expect(res.IsZero()).To(BeTrue())

			updatedInfra := &corev1.Node{}
			err = cl.Get(context.TODO(), client.ObjectKey{Name: "infra-1"}, updatedInfra)
			Expect(err).ToNot(HaveOccurred())
			Expect(updatedInfra.Labels[nodeinfo.LabelNodeRoleKubevirtControlPlane]).To(Equal(hypershiftLabelValue))

			updatedWorker := &corev1.Node{}
			err = cl.Get(context.TODO(), client.ObjectKey{Name: "worker-1"}, updatedWorker)
			Expect(err).ToNot(HaveOccurred())
			Expect(updatedWorker.Labels).NotTo(HaveKey(nodeinfo.LabelNodeRoleKubevirtControlPlane))
		})
	})

	Context("operatorDeploymentPredicate", func() {
		BeforeEach(func() {
			Expect(os.Setenv(hcoutil.OperatorNamespaceEnv, commontestutils.Namespace)).To(Succeed())
		})

		hcoDep := func(ns string, selector map[string]string) *appsv1.Deployment {
			return &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      hcoutil.OperatorName,
					Namespace: ns,
				},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							NodeSelector: selector,
						},
					},
				},
			}
		}

		It("Should reconcile create and delete of the HCO operator Deployment", func() {
			pred := operatorDeploymentPredicate{}
			dep := hcoDep(commontestutils.Namespace, map[string]string{"node-role.kubernetes.io/infra": ""})
			Expect(pred.Create(event.TypedCreateEvent[*appsv1.Deployment]{Object: dep})).To(BeTrue())
			Expect(pred.Delete(event.TypedDeleteEvent[*appsv1.Deployment]{Object: dep})).To(BeTrue())
			Expect(pred.Generic(event.TypedGenericEvent[*appsv1.Deployment]{Object: dep})).To(BeFalse())
		})

		It("Should ignore other Deployments", func() {
			pred := operatorDeploymentPredicate{}
			other := hcoDep(commontestutils.Namespace, nil)
			other.Name = "virt-operator"
			Expect(pred.Create(event.TypedCreateEvent[*appsv1.Deployment]{Object: other})).To(BeFalse())
			Expect(pred.Update(event.TypedUpdateEvent[*appsv1.Deployment]{ObjectOld: other, ObjectNew: other})).To(BeFalse())
		})

		It("Should reconcile only when the operator nodeSelector changes", func() {
			pred := operatorDeploymentPredicate{}
			oldDep := hcoDep(commontestutils.Namespace, map[string]string{corev1.LabelOSStable: "linux"})
			newDep := hcoDep(commontestutils.Namespace, map[string]string{
				corev1.LabelOSStable:            "linux",
				"node-role.kubernetes.io/infra": "",
			})
			Expect(pred.Update(event.TypedUpdateEvent[*appsv1.Deployment]{ObjectOld: oldDep, ObjectNew: newDep})).To(BeTrue())
			Expect(pred.Update(event.TypedUpdateEvent[*appsv1.Deployment]{ObjectOld: newDep, ObjectNew: newDep})).To(BeFalse())
		})
	})
})
