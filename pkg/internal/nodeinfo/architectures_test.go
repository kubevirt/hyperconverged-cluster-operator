package nodeinfo_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"kubevirt.io/controller-lifecycle-operator-sdk/api"

	"github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/internal/nodeinfo"
)

var _ = Describe("test node architectures", func() {
	var scheme *runtime.Scheme

	BeforeEach(func() {
		scheme = runtime.NewScheme()
		Expect(corev1.AddToScheme(scheme)).To(Succeed())
		Expect(v1beta1.AddToScheme(scheme)).To(Succeed())
	})

	Context("no node selection", func() {
		DescribeTable("When HyperConverged is not deployed", func(ctx context.Context, nodes []client.Object, expectedCP, expectedWL types.GomegaMatcher) {
			cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(nodes...).Build()
			_, err := nodeinfo.HandleNodeChanges(ctx, cl, nil, GinkgoLogr)
			Expect(err).ToNot(HaveOccurred())

			Expect(nodeinfo.GetControlPlaneArchitectures()).To(expectedCP)
			Expect(nodeinfo.GetWorkloadsArchitectures()).To(expectedWL)
		},
			// Test Worker nodes
			Entry("no nodes", nil, BeEmpty(), BeEmpty()),
			Entry("0 nodes", make([]client.Object, 0), BeEmpty(), BeEmpty()),
			Entry("1 worker, no architecture", []client.Object{&corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "worker1",
					Labels: map[string]string{
						nodeinfo.LabelNodeRoleWorker: "",
					},
				},
				Status: corev1.NodeStatus{
					NodeInfo: corev1.NodeSystemInfo{},
				},
			}}, BeEmpty(), BeEmpty()),
			Entry("1 worker, with architecture", []client.Object{&corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "worker1",
					Labels: map[string]string{
						nodeinfo.LabelNodeRoleWorker: "",
					},
				},
				Status: corev1.NodeStatus{
					NodeInfo: corev1.NodeSystemInfo{
						Architecture: "amd64",
					},
				},
			}}, BeEmpty(), And(HaveLen(1), ContainElement("amd64"))),
			Entry("2 workers, same architecture", []client.Object{
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "worker1",
						Labels: map[string]string{
							nodeinfo.LabelNodeRoleWorker: "",
						},
					},
					Status: corev1.NodeStatus{
						NodeInfo: corev1.NodeSystemInfo{
							Architecture: "amd64",
						},
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "worker2",
						Labels: map[string]string{
							nodeinfo.LabelNodeRoleWorker: "",
						},
					},
					Status: corev1.NodeStatus{
						NodeInfo: corev1.NodeSystemInfo{
							Architecture: "amd64",
						},
					},
				},
			}, BeEmpty(), And(HaveLen(1), ContainElement("amd64"))),
			Entry("2 workers, 2 architectures", []client.Object{
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "worker1",
						Labels: map[string]string{
							nodeinfo.LabelNodeRoleWorker: "",
						},
					},
					Status: corev1.NodeStatus{
						NodeInfo: corev1.NodeSystemInfo{
							Architecture: "amd64",
						},
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "worker2",
						Labels: map[string]string{
							nodeinfo.LabelNodeRoleWorker: "",
						},
					},
					Status: corev1.NodeStatus{
						NodeInfo: corev1.NodeSystemInfo{
							Architecture: "arm64",
						},
					},
				},
			}, BeEmpty(), And(HaveLen(2), ContainElements("amd64", "arm64"))),
			Entry("2 workers, 1 architecture", []client.Object{
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "worker1",
						Labels: map[string]string{
							nodeinfo.LabelNodeRoleWorker: "",
						},
					},
					Status: corev1.NodeStatus{
						NodeInfo: corev1.NodeSystemInfo{
							Architecture: "amd64",
						},
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "worker2",
						Labels: map[string]string{
							nodeinfo.LabelNodeRoleWorker: "",
						},
					},
					Status: corev1.NodeStatus{
						NodeInfo: corev1.NodeSystemInfo{},
					},
				},
			}, BeEmpty(), And(HaveLen(1), ContainElement("amd64"))),

			// Test Control Plane nodes
			Entry("1 control plane, no architecture", []client.Object{&corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cp1",
					Labels: map[string]string{
						nodeinfo.LabelNodeRoleControlPlane: "",
					},
				},
			}}, BeEmpty(), BeEmpty()),
			Entry("1 control plane, with architecture", []client.Object{&corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cp1",
					Labels: map[string]string{
						nodeinfo.LabelNodeRoleControlPlane: "",
					},
				},
				Status: corev1.NodeStatus{
					NodeInfo: corev1.NodeSystemInfo{
						Architecture: "amd64",
					},
				},
			}}, And(HaveLen(1), ContainElement("amd64")), BeEmpty()),
			Entry("1 master, with architecture", []client.Object{&corev1.Node{ // just to test the master label, once.
				ObjectMeta: metav1.ObjectMeta{
					Name: "cp1",
					Labels: map[string]string{
						nodeinfo.LabelNodeRoleMaster: "",
					},
				},
				Status: corev1.NodeStatus{
					NodeInfo: corev1.NodeSystemInfo{
						Architecture: "amd64",
					},
				},
			}}, And(HaveLen(1), ContainElement("amd64")), BeEmpty()),
			Entry("2 control planes, same architecture", []client.Object{
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cp1",
						Labels: map[string]string{
							nodeinfo.LabelNodeRoleControlPlane: "",
						},
					},
					Status: corev1.NodeStatus{
						NodeInfo: corev1.NodeSystemInfo{
							Architecture: "amd64",
						},
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cp2",
						Labels: map[string]string{
							nodeinfo.LabelNodeRoleControlPlane: "",
						},
					},
					Status: corev1.NodeStatus{
						NodeInfo: corev1.NodeSystemInfo{
							Architecture: "amd64",
						},
					},
				},
			}, And(HaveLen(1), ContainElement("amd64")), BeEmpty()),
			Entry("2 control planes, 2 architectures", []client.Object{
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cp1",
						Labels: map[string]string{
							nodeinfo.LabelNodeRoleControlPlane: "",
						},
					},
					Status: corev1.NodeStatus{
						NodeInfo: corev1.NodeSystemInfo{
							Architecture: "amd64",
						},
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cp2",
						Labels: map[string]string{
							nodeinfo.LabelNodeRoleControlPlane: "",
						},
					},
					Status: corev1.NodeStatus{
						NodeInfo: corev1.NodeSystemInfo{
							Architecture: "arm64",
						},
					},
				},
			}, And(HaveLen(2), ContainElements("amd64", "arm64")), BeEmpty()),

			// Mixed Control Plane and Worker nodes
			Entry("Mixed Control Plane and Worker nodes", []client.Object{
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "wp1-amd64",
						Labels: map[string]string{
							nodeinfo.LabelNodeRoleControlPlane: "",
						},
					},
					Status: corev1.NodeStatus{
						NodeInfo: corev1.NodeSystemInfo{Architecture: "amd64"},
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "master2-arm64",
						Labels: map[string]string{
							nodeinfo.LabelNodeRoleMaster: "",
						},
					},
					Status: corev1.NodeStatus{
						NodeInfo: corev1.NodeSystemInfo{Architecture: "arm64"},
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cp3-no-arch",
						Labels: map[string]string{
							nodeinfo.LabelNodeRoleControlPlane: "",
						},
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cp4-amd64",
						Labels: map[string]string{
							nodeinfo.LabelNodeRoleControlPlane: "",
						},
					},
					Status: corev1.NodeStatus{
						NodeInfo: corev1.NodeSystemInfo{Architecture: "amd64"},
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "wn1-arm64",
						Labels: map[string]string{
							nodeinfo.LabelNodeRoleWorker: "",
						},
					},
					Status: corev1.NodeStatus{
						NodeInfo: corev1.NodeSystemInfo{Architecture: "arm64"},
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "wn2-amd64",
						Labels: map[string]string{
							nodeinfo.LabelNodeRoleWorker: "",
						},
					},
					Status: corev1.NodeStatus{
						NodeInfo: corev1.NodeSystemInfo{Architecture: "amd64"},
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "wn3-amd64",
						Labels: map[string]string{
							nodeinfo.LabelNodeRoleWorker: "",
						},
					},
					Status: corev1.NodeStatus{
						NodeInfo: corev1.NodeSystemInfo{Architecture: "amd64"},
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "wn4-no-arch",
						Labels: map[string]string{
							nodeinfo.LabelNodeRoleWorker: "",
						},
					},
				},
			},
				And(HaveLen(2), ContainElements("amd64", "arm64")),
				And(HaveLen(2), ContainElements("amd64", "arm64")),
			),
			Entry("Single node cluster", []client.Object{&corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "worker1",
					Labels: map[string]string{
						nodeinfo.LabelNodeRoleControlPlane: "",
						nodeinfo.LabelNodeRoleWorker:       "",
					},
				},
				Status: corev1.NodeStatus{
					NodeInfo: corev1.NodeSystemInfo{
						Architecture: "amd64",
					},
				},
			}}, And(HaveLen(1), ContainElement("amd64")), And(HaveLen(1), ContainElement("amd64"))),
		)
	})

	Context("with node selection", func() {
		DescribeTable("When HyperConverged is deployed", func(ctx context.Context, nodes []client.Object, workloadsSettings v1beta1.HyperConvergedConfig, expectedWL types.GomegaMatcher) {

			hc := &v1beta1.HyperConverged{
				Spec: v1beta1.HyperConvergedSpec{
					Workloads: workloadsSettings,
				},
			}

			cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(nodes...).Build()
			_, err := nodeinfo.HandleNodeChanges(ctx, cl, hc, GinkgoLogr)
			Expect(err).ToNot(HaveOccurred())
			Expect(nodeinfo.GetWorkloadsArchitectures()).To(expectedWL)
		},
			Entry("1 worker, empty workloads settings", []client.Object{&corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "worker1",
					Labels: map[string]string{
						"label-a":                    "value-a",
						nodeinfo.LabelNodeRoleWorker: "",
					},
				},
				Status: corev1.NodeStatus{
					NodeInfo: corev1.NodeSystemInfo{},
				},
			}}, v1beta1.HyperConvergedConfig{}, BeEmpty()),
			Entry("1 worker, node selector - not match", []client.Object{&corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "worker1",
					Labels: map[string]string{
						"label-a":                    "value-a",
						nodeinfo.LabelNodeRoleWorker: "",
					},
				},
				Status: corev1.NodeStatus{
					NodeInfo: corev1.NodeSystemInfo{},
				},
			}}, v1beta1.HyperConvergedConfig{
				NodePlacement: &api.NodePlacement{
					NodeSelector: map[string]string{
						"label-b": "value-b",
					},
				},
			}, BeEmpty()),
			Entry("1 worker, node selector - match", []client.Object{&corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "worker1",
					Labels: map[string]string{
						"label-a":                    "value-a",
						nodeinfo.LabelNodeRoleWorker: "",
					},
				},
				Status: corev1.NodeStatus{
					NodeInfo: corev1.NodeSystemInfo{Architecture: "amd64"},
				},
			}}, v1beta1.HyperConvergedConfig{
				NodePlacement: &api.NodePlacement{
					NodeSelector: map[string]string{
						"label-a": "value-a",
					},
				},
			}, Equal([]string{"amd64"})),
			Entry("non-worker, node selector - match", []client.Object{&corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "worker1",
					Labels: map[string]string{
						"label-a": "value-a",
					},
				},
				Status: corev1.NodeStatus{
					NodeInfo: corev1.NodeSystemInfo{Architecture: "amd64"},
				},
			}}, v1beta1.HyperConvergedConfig{
				NodePlacement: &api.NodePlacement{
					NodeSelector: map[string]string{
						"label-a": "value-a",
					},
				},
			}, Equal([]string{"amd64"})),
			Entry("control-plane, node selector - match", []client.Object{&corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "worker1",
					Labels: map[string]string{
						nodeinfo.LabelNodeRoleControlPlane: "",
						"label-a":                          "value-a",
					},
				},
				Status: corev1.NodeStatus{
					NodeInfo: corev1.NodeSystemInfo{Architecture: "amd64"},
				},
			}}, v1beta1.HyperConvergedConfig{
				NodePlacement: &api.NodePlacement{
					NodeSelector: map[string]string{
						"label-a": "value-a",
					},
				},
			}, Equal([]string{"amd64"})),
			Entry("node selector w/ 2 labels - 1 match, 1 not", []client.Object{&corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "worker1",
					Labels: map[string]string{
						"label-a": "value-a",
					},
				},
				Status: corev1.NodeStatus{
					NodeInfo: corev1.NodeSystemInfo{Architecture: "amd64"},
				},
			}}, v1beta1.HyperConvergedConfig{
				NodePlacement: &api.NodePlacement{
					NodeSelector: map[string]string{
						"label-a": "value-a",
						"label-b": "value-b",
					},
				},
			}, BeEmpty()),
			Entry("2 nodes, 2 match", []client.Object{
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1-amd64",
						Labels: map[string]string{
							"label-a": "value-a",
						},
					},
					Status: corev1.NodeStatus{
						NodeInfo: corev1.NodeSystemInfo{Architecture: "amd64"},
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node2-arm64",
						Labels: map[string]string{
							"label-a": "value-a",
						},
					},
					Status: corev1.NodeStatus{
						NodeInfo: corev1.NodeSystemInfo{Architecture: "arm64"},
					},
				},
			}, v1beta1.HyperConvergedConfig{
				NodePlacement: &api.NodePlacement{
					NodeSelector: map[string]string{
						"label-a": "value-a",
					},
				},
			}, Equal([]string{"amd64", "arm64"})),
			Entry("non-worker, node affinity - not match", []client.Object{&corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "worker1",
					Labels: map[string]string{
						"label-a": "value-a",
					},
				},
				Status: corev1.NodeStatus{
					NodeInfo: corev1.NodeSystemInfo{Architecture: "amd64"},
				},
			}}, v1beta1.HyperConvergedConfig{
				NodePlacement: &api.NodePlacement{
					Affinity: &corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									{
										MatchExpressions: []corev1.NodeSelectorRequirement{
											{Key: "label-a", Operator: corev1.NodeSelectorOpIn, Values: []string{"value-b", "value-c"}},
										},
									},
								},
							},
						},
					},
				},
			}, BeEmpty()),
			Entry("non-worker, node affinity - match", []client.Object{&corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "worker1",
					Labels: map[string]string{
						"label-a": "value-a",
					},
				},
				Status: corev1.NodeStatus{
					NodeInfo: corev1.NodeSystemInfo{Architecture: "amd64"},
				},
			}}, v1beta1.HyperConvergedConfig{
				NodePlacement: &api.NodePlacement{
					Affinity: &corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									{
										MatchExpressions: []corev1.NodeSelectorRequirement{
											{Key: "label-a", Operator: corev1.NodeSelectorOpIn, Values: []string{"value-a", "value-b", "value-c"}},
										},
									},
								},
							},
						},
					},
				},
			}, Equal([]string{"amd64"})),
			Entry("3 nodes, w/ node affinity - 2 match, 1 not", []client.Object{
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "wl1-match",
						Labels: map[string]string{
							"label-a": "value-a",
						},
					},
					Status: corev1.NodeStatus{
						NodeInfo: corev1.NodeSystemInfo{Architecture: "amd64"},
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "wl2-not-match",
						Labels: map[string]string{
							"label-a": "value-b",
						},
					},
					Status: corev1.NodeStatus{
						NodeInfo: corev1.NodeSystemInfo{Architecture: "arm64"},
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "wl3-match",
						Labels: map[string]string{
							"label-a": "value-c",
						},
					},
					Status: corev1.NodeStatus{
						NodeInfo: corev1.NodeSystemInfo{Architecture: "s390x"},
					},
				},
			}, v1beta1.HyperConvergedConfig{
				NodePlacement: &api.NodePlacement{
					Affinity: &corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									{
										MatchExpressions: []corev1.NodeSelectorRequirement{
											{Key: "label-a", Operator: corev1.NodeSelectorOpIn, Values: []string{"value-a", "value-c"}},
										},
									},
								},
							},
						},
					},
				},
			}, Equal([]string{"amd64", "s390x"})),

			Entry("3 nodes, w/ node affinity - 2 match, 1 not", []client.Object{
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "wl1-match",
						Labels: map[string]string{
							"label-a": "value-a",
						},
					},
					Status: corev1.NodeStatus{
						NodeInfo: corev1.NodeSystemInfo{Architecture: "amd64"},
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "wl2-not-match",
						Labels: map[string]string{
							"label-a": "value-b",
						},
					},
					Status: corev1.NodeStatus{
						NodeInfo: corev1.NodeSystemInfo{Architecture: "arm64"},
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "wl3-match",
						Labels: map[string]string{
							"label-a": "value-c",
						},
					},
					Status: corev1.NodeStatus{
						NodeInfo: corev1.NodeSystemInfo{Architecture: "s390x"},
					},
				},
			}, v1beta1.HyperConvergedConfig{
				NodePlacement: &api.NodePlacement{
					Affinity: &corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									{
										MatchExpressions: []corev1.NodeSelectorRequirement{
											{Key: "label-a", Operator: corev1.NodeSelectorOpIn, Values: []string{"value-a", "value-c"}},
										},
									},
								},
							},
						},
					},
				},
			}, Equal([]string{"amd64", "s390x"})),
		)
	})

	Context("check if HandleNodeChanges returns 'changed' for architectures", func() {
		const (
			cpArch  = "cp-arch"
			wlArch1 = "wl-arch1"
			wlArch2 = "wl-arch2"
			wlArch3 = "wl-arch3"
			wlArch4 = "wl-arch4"
			wlArch5 = "wl-arch5"
			wlArch6 = "wl-arch6"

			cpArchDifferent = "cp-arch-different"
		)
		BeforeEach(func() {
			// make ControlPlaneArchitectures to be {"cp-arch"} and WorkloadsArchitectures
			// to be {"wl-arch1", "wl-arch2", "wl-arch3"}
			nodes := genNodeList(3, 0, 3)
			nodes[0].(*corev1.Node).Status.NodeInfo.Architecture = cpArch
			nodes[1].(*corev1.Node).Status.NodeInfo.Architecture = cpArch
			nodes[2].(*corev1.Node).Status.NodeInfo.Architecture = cpArch
			nodes[3].(*corev1.Node).Status.NodeInfo.Architecture = wlArch1
			nodes[4].(*corev1.Node).Status.NodeInfo.Architecture = wlArch2
			nodes[5].(*corev1.Node).Status.NodeInfo.Architecture = wlArch3
			cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(nodes...).Build()
			_, err := nodeinfo.HandleNodeChanges(context.TODO(), cli, nil, GinkgoLogr)
			Expect(err).ToNot(HaveOccurred())
			Expect(nodeinfo.GetControlPlaneArchitectures()).To(Equal([]string{"cp-arch"}))
			Expect(nodeinfo.GetWorkloadsArchitectures()).To(Equal([]string{wlArch1, wlArch2, wlArch3}))
		})

		It("should not changed if control plane architectures were not changed", func() {
			nodes := genNodeList(3, 0, 3)
			nodes[0].(*corev1.Node).Status.NodeInfo.Architecture = cpArch
			nodes[1].(*corev1.Node).Status.NodeInfo.Architecture = cpArch
			nodes[2].(*corev1.Node).Status.NodeInfo.Architecture = cpArch
			nodes[3].(*corev1.Node).Status.NodeInfo.Architecture = wlArch1
			nodes[4].(*corev1.Node).Status.NodeInfo.Architecture = wlArch2
			nodes[5].(*corev1.Node).Status.NodeInfo.Architecture = wlArch3

			cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(nodes...).Build()

			changed, err := nodeinfo.HandleNodeChanges(context.TODO(), cli, nil, GinkgoLogr)
			Expect(err).ToNot(HaveOccurred())
			Expect(changed).To(BeFalse())
			Expect(nodeinfo.GetControlPlaneArchitectures()).To(Equal([]string{"cp-arch"}))
			Expect(nodeinfo.GetWorkloadsArchitectures()).To(Equal([]string{wlArch1, wlArch2, wlArch3}))
		})

		It("should not changed if control got master instead of plane architectures", func() {
			nodes := genNodeList(0, 3, 3)
			nodes[0].(*corev1.Node).Status.NodeInfo.Architecture = cpArch
			nodes[1].(*corev1.Node).Status.NodeInfo.Architecture = cpArch
			nodes[2].(*corev1.Node).Status.NodeInfo.Architecture = cpArch
			nodes[3].(*corev1.Node).Status.NodeInfo.Architecture = wlArch1
			nodes[4].(*corev1.Node).Status.NodeInfo.Architecture = wlArch2
			nodes[5].(*corev1.Node).Status.NodeInfo.Architecture = wlArch3

			cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(nodes...).Build()

			changed, err := nodeinfo.HandleNodeChanges(context.TODO(), cli, nil, GinkgoLogr)
			Expect(err).ToNot(HaveOccurred())
			Expect(changed).To(BeFalse())
			Expect(nodeinfo.GetControlPlaneArchitectures()).To(Equal([]string{"cp-arch"}))
			Expect(nodeinfo.GetWorkloadsArchitectures()).To(Equal([]string{wlArch1, wlArch2, wlArch3}))
		})

		It("should not changed if control plane architectures were not changed, even with different node list", func() {
			nodes := genNodeList(6, 0, 9)
			nodes[0].(*corev1.Node).Status.NodeInfo.Architecture = cpArch
			nodes[1].(*corev1.Node).Status.NodeInfo.Architecture = cpArch
			nodes[2].(*corev1.Node).Status.NodeInfo.Architecture = cpArch
			nodes[3].(*corev1.Node).Status.NodeInfo.Architecture = cpArch
			nodes[4].(*corev1.Node).Status.NodeInfo.Architecture = cpArch
			nodes[5].(*corev1.Node).Status.NodeInfo.Architecture = cpArch
			nodes[6].(*corev1.Node).Status.NodeInfo.Architecture = wlArch1
			nodes[7].(*corev1.Node).Status.NodeInfo.Architecture = wlArch2
			nodes[8].(*corev1.Node).Status.NodeInfo.Architecture = wlArch3
			nodes[9].(*corev1.Node).Status.NodeInfo.Architecture = wlArch1
			nodes[10].(*corev1.Node).Status.NodeInfo.Architecture = wlArch2
			nodes[11].(*corev1.Node).Status.NodeInfo.Architecture = wlArch3
			nodes[12].(*corev1.Node).Status.NodeInfo.Architecture = wlArch1
			nodes[13].(*corev1.Node).Status.NodeInfo.Architecture = wlArch2
			nodes[14].(*corev1.Node).Status.NodeInfo.Architecture = wlArch3

			cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(nodes...).Build()

			changed, err := nodeinfo.HandleNodeChanges(context.TODO(), cli, nil, GinkgoLogr)
			Expect(err).ToNot(HaveOccurred())
			Expect(changed).To(BeFalse())
			Expect(nodeinfo.GetControlPlaneArchitectures()).To(Equal([]string{"cp-arch"}))
			Expect(nodeinfo.GetWorkloadsArchitectures()).To(Equal([]string{wlArch1, wlArch2, wlArch3}))
		})

		It("should changed if control plane architectures were changed", func() {
			nodes := genNodeList(3, 0, 3)
			nodes[0].(*corev1.Node).Status.NodeInfo.Architecture = cpArchDifferent
			nodes[1].(*corev1.Node).Status.NodeInfo.Architecture = cpArchDifferent
			nodes[2].(*corev1.Node).Status.NodeInfo.Architecture = cpArchDifferent
			nodes[3].(*corev1.Node).Status.NodeInfo.Architecture = wlArch1
			nodes[4].(*corev1.Node).Status.NodeInfo.Architecture = wlArch2
			nodes[5].(*corev1.Node).Status.NodeInfo.Architecture = wlArch3

			cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(nodes...).Build()

			changed, err := nodeinfo.HandleNodeChanges(context.TODO(), cli, nil, GinkgoLogr)
			Expect(err).ToNot(HaveOccurred())
			Expect(changed).To(BeTrue())
			Expect(nodeinfo.GetControlPlaneArchitectures()).To(Equal([]string{cpArchDifferent}))
			Expect(nodeinfo.GetWorkloadsArchitectures()).To(Equal([]string{wlArch1, wlArch2, wlArch3}))
		})

		It("should changed if one workloads architecture was dropped", func() {
			nodes := genNodeList(3, 0, 3)
			nodes[0].(*corev1.Node).Status.NodeInfo.Architecture = cpArch
			nodes[1].(*corev1.Node).Status.NodeInfo.Architecture = cpArch
			nodes[2].(*corev1.Node).Status.NodeInfo.Architecture = cpArch
			nodes[3].(*corev1.Node).Status.NodeInfo.Architecture = wlArch1
			nodes[4].(*corev1.Node).Status.NodeInfo.Architecture = wlArch1
			nodes[5].(*corev1.Node).Status.NodeInfo.Architecture = wlArch3

			cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(nodes...).Build()

			changed, err := nodeinfo.HandleNodeChanges(context.TODO(), cli, nil, GinkgoLogr)
			Expect(err).ToNot(HaveOccurred())
			Expect(changed).To(BeTrue())
			Expect(nodeinfo.GetControlPlaneArchitectures()).To(Equal([]string{cpArch}))
			Expect(nodeinfo.GetWorkloadsArchitectures()).To(Equal([]string{wlArch1, wlArch3}))
		})

		It("should changed if one workloads architecture was added", func() {
			nodes := genNodeList(3, 0, 4)
			nodes[0].(*corev1.Node).Status.NodeInfo.Architecture = cpArch
			nodes[1].(*corev1.Node).Status.NodeInfo.Architecture = cpArch
			nodes[2].(*corev1.Node).Status.NodeInfo.Architecture = cpArch
			nodes[3].(*corev1.Node).Status.NodeInfo.Architecture = wlArch1
			nodes[4].(*corev1.Node).Status.NodeInfo.Architecture = wlArch2
			nodes[5].(*corev1.Node).Status.NodeInfo.Architecture = wlArch3
			nodes[6].(*corev1.Node).Status.NodeInfo.Architecture = "wl-arch-added"

			cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(nodes...).Build()

			changed, err := nodeinfo.HandleNodeChanges(context.TODO(), cli, nil, GinkgoLogr)
			Expect(err).ToNot(HaveOccurred())
			Expect(changed).To(BeTrue())
			Expect(nodeinfo.GetControlPlaneArchitectures()).To(Equal([]string{cpArch}))
			Expect(nodeinfo.GetWorkloadsArchitectures()).To(Equal([]string{"wl-arch-added", wlArch1, wlArch2, wlArch3}))
		})

		It("should not changed if nodePlacement only select nodes with the same architecture", func() {
			nodes := genNodeList(0, 3, 6)
			nodes[0].(*corev1.Node).Status.NodeInfo.Architecture = cpArch
			nodes[1].(*corev1.Node).Status.NodeInfo.Architecture = cpArch
			nodes[2].(*corev1.Node).Status.NodeInfo.Architecture = cpArch
			nodes[3].(*corev1.Node).Status.NodeInfo.Architecture = wlArch1
			nodes[4].(*corev1.Node).Status.NodeInfo.Architecture = wlArch2
			nodes[5].(*corev1.Node).Status.NodeInfo.Architecture = wlArch3
			nodes[6].(*corev1.Node).Status.NodeInfo.Architecture = wlArch4
			nodes[7].(*corev1.Node).Status.NodeInfo.Architecture = wlArch5
			nodes[8].(*corev1.Node).Status.NodeInfo.Architecture = wlArch6

			nodes[3].(*corev1.Node).Labels["label-a"] = "value-a"
			nodes[4].(*corev1.Node).Labels["label-a"] = "value-a"
			nodes[5].(*corev1.Node).Labels["label-a"] = "value-a"

			hc := &v1beta1.HyperConverged{
				Spec: v1beta1.HyperConvergedSpec{
					Workloads: v1beta1.HyperConvergedConfig{
						NodePlacement: &api.NodePlacement{
							NodeSelector: map[string]string{
								"label-a": "value-a",
							},
						},
					},
				},
			}

			cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(nodes...).Build()

			changed, err := nodeinfo.HandleNodeChanges(context.TODO(), cli, hc, GinkgoLogr)
			Expect(err).ToNot(HaveOccurred())
			Expect(changed).To(BeFalse())
			Expect(nodeinfo.GetControlPlaneArchitectures()).To(Equal([]string{"cp-arch"}))
			Expect(nodeinfo.GetWorkloadsArchitectures()).To(Equal([]string{wlArch1, wlArch2, wlArch3}))
		})

		It("should changed if nodePlacement only select nodes with less architectures", func() {
			nodes := genNodeList(0, 3, 6)
			nodes[0].(*corev1.Node).Status.NodeInfo.Architecture = cpArch
			nodes[1].(*corev1.Node).Status.NodeInfo.Architecture = cpArch
			nodes[2].(*corev1.Node).Status.NodeInfo.Architecture = cpArch
			nodes[3].(*corev1.Node).Status.NodeInfo.Architecture = wlArch1
			nodes[4].(*corev1.Node).Status.NodeInfo.Architecture = wlArch2
			nodes[5].(*corev1.Node).Status.NodeInfo.Architecture = wlArch3
			nodes[6].(*corev1.Node).Status.NodeInfo.Architecture = wlArch4
			nodes[7].(*corev1.Node).Status.NodeInfo.Architecture = wlArch5
			nodes[8].(*corev1.Node).Status.NodeInfo.Architecture = wlArch6

			nodes[3].(*corev1.Node).Labels["label-a"] = "value-a"
			nodes[5].(*corev1.Node).Labels["label-a"] = "value-a"

			hc := &v1beta1.HyperConverged{
				Spec: v1beta1.HyperConvergedSpec{
					Workloads: v1beta1.HyperConvergedConfig{
						NodePlacement: &api.NodePlacement{
							NodeSelector: map[string]string{
								"label-a": "value-a",
							},
						},
					},
				},
			}

			cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(nodes...).Build()

			changed, err := nodeinfo.HandleNodeChanges(context.TODO(), cli, hc, GinkgoLogr)
			Expect(err).ToNot(HaveOccurred())
			Expect(changed).To(BeTrue())
			Expect(nodeinfo.GetControlPlaneArchitectures()).To(Equal([]string{"cp-arch"}))
			Expect(nodeinfo.GetWorkloadsArchitectures()).To(Equal([]string{wlArch1, wlArch3}))
		})

		It("should changed if nodePlacement only select nodes with additional architecture", func() {
			nodes := genNodeList(0, 3, 6)
			nodes[0].(*corev1.Node).Status.NodeInfo.Architecture = cpArch
			nodes[1].(*corev1.Node).Status.NodeInfo.Architecture = cpArch
			nodes[2].(*corev1.Node).Status.NodeInfo.Architecture = cpArch
			nodes[3].(*corev1.Node).Status.NodeInfo.Architecture = wlArch1
			nodes[4].(*corev1.Node).Status.NodeInfo.Architecture = wlArch2
			nodes[5].(*corev1.Node).Status.NodeInfo.Architecture = wlArch3
			nodes[6].(*corev1.Node).Status.NodeInfo.Architecture = wlArch4
			nodes[7].(*corev1.Node).Status.NodeInfo.Architecture = wlArch5
			nodes[8].(*corev1.Node).Status.NodeInfo.Architecture = wlArch6

			nodes[3].(*corev1.Node).Labels["label-a"] = "value-a"
			nodes[4].(*corev1.Node).Labels["label-a"] = "value-a"
			nodes[5].(*corev1.Node).Labels["label-a"] = "value-a"
			nodes[7].(*corev1.Node).Labels["label-a"] = "value-a"

			hc := &v1beta1.HyperConverged{
				Spec: v1beta1.HyperConvergedSpec{
					Workloads: v1beta1.HyperConvergedConfig{
						NodePlacement: &api.NodePlacement{
							NodeSelector: map[string]string{
								"label-a": "value-a",
							},
						},
					},
				},
			}

			cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(nodes...).Build()

			changed, err := nodeinfo.HandleNodeChanges(context.TODO(), cli, hc, GinkgoLogr)
			Expect(err).ToNot(HaveOccurred())
			Expect(changed).To(BeTrue())
			Expect(nodeinfo.GetControlPlaneArchitectures()).To(Equal([]string{"cp-arch"}))
			Expect(nodeinfo.GetWorkloadsArchitectures()).To(Equal([]string{wlArch1, wlArch2, wlArch3, wlArch5}))
		})
	})
})
