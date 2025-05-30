package nodeinfo_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	nodeinfo2 "github.com/kubevirt/hyperconverged-cluster-operator/pkg/internal/nodeinfo"
)

var _ = Describe("HighAvailability", func() {
	DescribeTable("should determine if the cluster is highly available", func(ctx context.Context, nodes []client.Object, cpExists, cpHA, infraHA bool) {
		s := runtime.NewScheme()
		Expect(corev1.AddToScheme(s)).To(Succeed())
		cli := fake.NewClientBuilder().WithScheme(s).WithObjects(nodes...).Build()

		Expect(nodeinfo2.HandleNodeChanges(ctx, cli)).To(Succeed())
		Expect(nodeinfo2.IsControlPlaneNodeExists()).To(Equal(cpExists))
		Expect(nodeinfo2.IsControlPlaneHighlyAvailable()).To(Equal(cpHA))
		Expect(nodeinfo2.IsInfrastructureHighlyAvailable()).To(Equal(infraHA))
	},
		Entry("no nodes", []client.Object{}, false, false, false),
		Entry("one control plane node", genNodeList(1, 0, 0), true, false, false),
		Entry("two control plane nodes", genNodeList(2, 0, 0), true, false, false),
		Entry("three control plane nodes", genNodeList(3, 0, 0), true, true, false),
		Entry("four control plane nodes", genNodeList(4, 0, 0), true, true, false),
		Entry("one control plane and one master", genNodeList(1, 1, 0), true, false, false),
		Entry("two control planes and one master", genNodeList(2, 1, 0), true, true, false),

		Entry("one control plane and one worker node", genNodeList(1, 0, 1), true, false, false),
		Entry("one control plane and two worker nodes", genNodeList(1, 0, 2), true, false, true),
		Entry("three control plane and three worker nodes", genNodeList(3, 0, 3), true, true, true),
		Entry("one control plane, two masters, and three worker nodes", genNodeList(1, 2, 3), true, true, true),
	)
})

func genNodeList(controlPlanes, masters, workers int) []client.Object {
	nodesArray := make([]client.Object, 0, controlPlanes+masters+workers)

	for i := range controlPlanes {
		cpNode := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("control-plane-%d", i),
				Labels: map[string]string{
					nodeinfo2.LabelNodeRoleControlPlane: "",
				},
			},
		}
		nodesArray = append(nodesArray, cpNode)
	}

	for i := range masters {
		masterNode := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("master-%d", i),
				Labels: map[string]string{
					nodeinfo2.LabelNodeRoleMaster: "",
				},
			},
		}
		nodesArray = append(nodesArray, masterNode)
	}

	for i := range workers {
		workerNode := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("worker-%d", i),
				Labels: map[string]string{
					nodeinfo2.LabelNodeRoleWorker: "",
				},
			},
		}
		nodesArray = append(nodesArray, workerNode)
	}

	return nodesArray
}
