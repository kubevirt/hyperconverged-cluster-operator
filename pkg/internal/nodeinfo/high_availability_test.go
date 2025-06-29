package nodeinfo_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/internal/nodeinfo"
)

var _ = Describe("HighAvailability", func() {

	var scheme *runtime.Scheme

	BeforeEach(func() {
		scheme = runtime.NewScheme()
		Expect(corev1.AddToScheme(scheme)).To(Succeed())
	})

	DescribeTable("should determine if the cluster is highly available", func(ctx context.Context, nodes []client.Object, cpExists, cpHA, infraHA gomegatypes.GomegaMatcher) {
		cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(nodes...).Build()

		_, err := nodeinfo.HandleNodeChanges(ctx, cli, nil, GinkgoLogr)
		Expect(err).ToNot(HaveOccurred())
		Expect(nodeinfo.IsControlPlaneNodeExists()).To(cpExists)
		Expect(nodeinfo.IsControlPlaneHighlyAvailable()).To(cpHA)
		Expect(nodeinfo.IsInfrastructureHighlyAvailable()).To(infraHA)
	},
		Entry("no nodes", []client.Object{}, BeFalse(), BeFalse(), BeFalse()),
		Entry("one control plane node", genNodeList(1, 0, 0), BeTrue(), BeFalse(), BeFalse()),
		Entry("two control plane nodes", genNodeList(2, 0, 0), BeTrue(), BeFalse(), BeFalse()),
		Entry("three control plane nodes", genNodeList(3, 0, 0), BeTrue(), BeTrue(), BeFalse()),
		Entry("four control plane nodes", genNodeList(4, 0, 0), BeTrue(), BeTrue(), BeFalse()),
		Entry("one control plane and one master", genNodeList(1, 1, 0), BeTrue(), BeFalse(), BeFalse()),
		Entry("two control planes and one master", genNodeList(2, 1, 0), BeTrue(), BeTrue(), BeFalse()),

		Entry("one control plane and one worker node", genNodeList(1, 0, 1), BeTrue(), BeFalse(), BeFalse()),
		Entry("one control plane and two worker nodes", genNodeList(1, 0, 2), BeTrue(), BeFalse(), BeTrue()),
		Entry("three control plane and three worker nodes", genNodeList(3, 0, 3), BeTrue(), BeTrue(), BeTrue()),
		Entry("one control plane, two masters, and three worker nodes", genNodeList(1, 2, 3), BeTrue(), BeTrue(), BeTrue()),
	)

	Context("check if HandleNodeChanges returns 'changed' for high availability", func() {
		BeforeEach(func() {
			nodes := genNodeList(3, 0, 3)
			cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(nodes...).Build()
			_, err := nodeinfo.HandleNodeChanges(context.TODO(), cli, nil, GinkgoLogr)
			Expect(err).ToNot(HaveOccurred())

			Expect(nodeinfo.IsControlPlaneNodeExists()).To(BeTrue())
			Expect(nodeinfo.IsControlPlaneHighlyAvailable()).To(BeTrue())
			Expect(nodeinfo.IsInfrastructureHighlyAvailable()).To(BeTrue())
		})

		It("should not changed if control plane architectures were not changed", func() {
			nodes := genNodeList(3, 0, 3)

			cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(nodes...).Build()

			changed, err := nodeinfo.HandleNodeChanges(context.TODO(), cli, nil, GinkgoLogr)
			Expect(err).ToNot(HaveOccurred())
			Expect(changed).To(BeFalse())
			Expect(nodeinfo.IsControlPlaneNodeExists()).To(BeTrue())
			Expect(nodeinfo.IsControlPlaneHighlyAvailable()).To(BeTrue())
			Expect(nodeinfo.IsInfrastructureHighlyAvailable()).To(BeTrue())
		})

		It("should not changed if getting master nodes instead of control plane", func() {
			nodes := genNodeList(0, 3, 3)

			cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(nodes...).Build()

			changed, err := nodeinfo.HandleNodeChanges(context.TODO(), cli, nil, GinkgoLogr)
			Expect(err).ToNot(HaveOccurred())
			Expect(changed).To(BeFalse())
			Expect(nodeinfo.IsControlPlaneNodeExists()).To(BeTrue())
			Expect(nodeinfo.IsControlPlaneHighlyAvailable()).To(BeTrue())
			Expect(nodeinfo.IsInfrastructureHighlyAvailable()).To(BeTrue())
		})

		It("should not changed if control plane architectures were not changed, even with different node list", func() {
			nodes := genNodeList(6, 6, 30)

			cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(nodes...).Build()

			changed, err := nodeinfo.HandleNodeChanges(context.TODO(), cli, nil, GinkgoLogr)
			Expect(err).ToNot(HaveOccurred())
			Expect(changed).To(BeFalse())
			Expect(nodeinfo.IsControlPlaneNodeExists()).To(BeTrue())
			Expect(nodeinfo.IsControlPlaneHighlyAvailable()).To(BeTrue())
			Expect(nodeinfo.IsInfrastructureHighlyAvailable()).To(BeTrue())
		})

		It("should changed if control plane became non HA", func() {
			nodes := genNodeList(2, 0, 3)

			cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(nodes...).Build()

			changed, err := nodeinfo.HandleNodeChanges(context.TODO(), cli, nil, GinkgoLogr)
			Expect(err).ToNot(HaveOccurred())
			Expect(changed).To(BeTrue())
			Expect(nodeinfo.IsControlPlaneNodeExists()).To(BeTrue())
			Expect(nodeinfo.IsControlPlaneHighlyAvailable()).To(BeFalse())
			Expect(nodeinfo.IsInfrastructureHighlyAvailable()).To(BeTrue())
		})

		It("should changed if control plane does not exists", func() {
			nodes := genNodeList(0, 0, 3)

			cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(nodes...).Build()

			changed, err := nodeinfo.HandleNodeChanges(context.TODO(), cli, nil, GinkgoLogr)
			Expect(err).ToNot(HaveOccurred())
			Expect(changed).To(BeTrue())
			Expect(nodeinfo.IsControlPlaneNodeExists()).To(BeFalse())
			Expect(nodeinfo.IsControlPlaneHighlyAvailable()).To(BeFalse())
			Expect(nodeinfo.IsInfrastructureHighlyAvailable()).To(BeTrue())
		})

		It("should changed if control plane became non HA, and then, w/o CP", func() {
			nodes := genNodeList(2, 0, 3)

			cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(nodes...).Build()

			changed, err := nodeinfo.HandleNodeChanges(context.TODO(), cli, nil, GinkgoLogr)
			Expect(err).ToNot(HaveOccurred())
			Expect(changed).To(BeTrue())
			Expect(nodeinfo.IsControlPlaneNodeExists()).To(BeTrue())
			Expect(nodeinfo.IsControlPlaneHighlyAvailable()).To(BeFalse())
			Expect(nodeinfo.IsInfrastructureHighlyAvailable()).To(BeTrue())

			By("removing control plane nodes - should changed again")
			nodes = genNodeList(0, 0, 3)

			cli = fake.NewClientBuilder().WithScheme(scheme).WithObjects(nodes...).Build()

			changed, err = nodeinfo.HandleNodeChanges(context.TODO(), cli, nil, GinkgoLogr)
			Expect(err).ToNot(HaveOccurred())
			Expect(changed).To(BeTrue())
			Expect(nodeinfo.IsControlPlaneNodeExists()).To(BeFalse())
			Expect(nodeinfo.IsControlPlaneHighlyAvailable()).To(BeFalse())
			Expect(nodeinfo.IsInfrastructureHighlyAvailable()).To(BeTrue())
		})

		It("should changed if workloads became not HA", func() {
			nodes := genNodeList(3, 0, 1)

			cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(nodes...).Build()

			changed, err := nodeinfo.HandleNodeChanges(context.TODO(), cli, nil, GinkgoLogr)
			Expect(err).ToNot(HaveOccurred())
			Expect(changed).To(BeTrue())

			Expect(nodeinfo.IsControlPlaneNodeExists()).To(BeTrue())
			Expect(nodeinfo.IsControlPlaneHighlyAvailable()).To(BeTrue())
			Expect(nodeinfo.IsInfrastructureHighlyAvailable()).To(BeFalse())
		})
	})
})

func genNodeList(controlPlanes, masters, workers int) []client.Object {
	nodesArray := make([]client.Object, 0, controlPlanes+masters+workers)

	for i := range controlPlanes {
		cpNode := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("control-plane-%d", i),
				Labels: map[string]string{
					nodeinfo.LabelNodeRoleControlPlane: "",
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
					nodeinfo.LabelNodeRoleMaster: "",
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
					nodeinfo.LabelNodeRoleWorker: "",
				},
			},
		}
		nodesArray = append(nodesArray, workerNode)
	}

	return nodesArray
}
