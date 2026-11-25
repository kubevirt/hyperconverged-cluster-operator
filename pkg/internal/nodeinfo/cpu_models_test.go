package nodeinfo_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kubevirtv1 "kubevirt.io/api/core/v1"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/internal/nodeinfo"
)

var _ = Describe("CPU models", func() {
	var scheme *runtime.Scheme

	BeforeEach(func() {
		nodeinfo.ResetCPUModelsCache()
		scheme = runtime.NewScheme()
		Expect(corev1.AddToScheme(scheme)).To(Succeed())
	})

	It("tracks models on schedulable nodes", func(ctx context.Context) {
		Expect(applyNodes(ctx, scheme, []client.Object{
			cpuModelNode("intel-a", true, "Skylake-Server", "Haswell-noTSX", "Penryn", "EPYC"),
			cpuModelNode("intel-b", true, "Haswell-noTSX", "Penryn"),
			cpuModelNode("unschedulable", false, "Opteron_G1"),
		})).To(BeTrue())

		Expect(nodeinfo.GetCPUModels()).To(Equal([]hcov1.CPUModelStatus{
			{Name: "Skylake-Server"},
			{Name: "Haswell-noTSX"},
			{Name: "Penryn"},
			{Name: "EPYC"},
		}))

		Expect(applyNodes(ctx, scheme, []client.Object{
			cpuModelNode("intel-a", true, "Haswell-noTSX", "Penryn"),
			cpuModelNode("intel-b", true, "Penryn"),
		})).To(BeTrue())
		Expect(nodeinfo.GetCPUModels()).To(Equal([]hcov1.CPUModelStatus{
			{Name: "Haswell-noTSX"},
			{Name: "Penryn"},
		}))

		Expect(applyNodes(ctx, scheme, []client.Object{
			cpuModelNode("worker", false, "Penryn"),
		})).To(BeTrue())
		Expect(nodeinfo.GetCPUModels()).To(BeEmpty())
		Expect(applyNodes(ctx, scheme, []client.Object{
			cpuModelNode("worker", false, "Penryn"),
		})).To(BeFalse())
	})

	It("orders models from newest family to oldest", func(ctx context.Context) {
		applyNodes(ctx, scheme, []client.Object{
			cpuModelNode("worker", true,
				"Penryn",
				"CustomModel",
				"Cascadelake-Server",
				"Cascadelake-Server-v5",
				"Cascadelake-Server-noTSX",
				"EPYC-Milan",
			),
		})

		Expect(nodeinfo.GetCPUModels()).To(Equal([]hcov1.CPUModelStatus{
			{Name: "Cascadelake-Server-v5"},
			{Name: "Cascadelake-Server"},
			{Name: "Cascadelake-Server-noTSX"},
			{Name: "Penryn"},
			{Name: "EPYC-Milan"},
			{Name: "CustomModel"},
		}))
	})

	It("ignores non CPU-model labels", func(ctx context.Context) {
		node := cpuModelNode("worker", true, "Penryn")
		node.Labels[kubevirtv1.CPUModelLabel+"Haswell-noTSX"] = "false"
		node.Labels[kubevirtv1.CPUFeatureLabel+"avx"] = "true"
		node.Labels[kubevirtv1.HostModelCPULabel+"Skylake-Server"] = "true"
		node.Labels[kubevirtv1.CPUModelLabel] = "true"

		applyNodes(ctx, scheme, []client.Object{node})
		Expect(nodeinfo.GetCPUModels()).To(Equal([]hcov1.CPUModelStatus{{Name: "Penryn"}}))
	})

	It("does not report a change when the model list is unchanged", func(ctx context.Context) {
		nodes := []client.Object{
			cpuModelNode("intel-a", true, "Skylake-Server", "Penryn"),
			cpuModelNode("intel-b", true, "Penryn"),
		}
		Expect(applyNodes(ctx, scheme, nodes)).To(BeTrue())
		Expect(applyNodes(ctx, scheme, nodes)).To(BeFalse())
		Expect(nodeinfo.GetCPUModels()).To(Equal([]hcov1.CPUModelStatus{
			{Name: "Skylake-Server"},
			{Name: "Penryn"},
		}))
	})

	It("does not report a change when another node still advertises the model", func(ctx context.Context) {
		keep := cpuModelNode("intel-a", true, "Penryn", "Skylake-Server")
		drop := cpuModelNode("intel-b", true, "Skylake-Server")
		Expect(applyNodes(ctx, scheme, []client.Object{keep, drop})).To(BeTrue())

		drop = cpuModelNode("intel-b", true, "Penryn")
		Expect(applyNodes(ctx, scheme, []client.Object{keep, drop})).To(BeFalse())
		Expect(nodeinfo.GetCPUModels()).To(Equal([]hcov1.CPUModelStatus{
			{Name: "Skylake-Server"},
			{Name: "Penryn"},
		}))
	})
})

func applyNodes(ctx context.Context, scheme *runtime.Scheme, nodes []client.Object) bool {
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(nodes...).Build()
	changed, err := nodeinfo.HandleNodeChanges(ctx, cl, nil, GinkgoLogr)
	Expect(err).NotTo(HaveOccurred())
	return changed
}

func cpuModelNode(name string, schedulable bool, models ...string) *corev1.Node {
	labels := map[string]string{
		nodeinfo.LabelNodeRoleWorker: "",
	}
	if schedulable {
		labels[kubevirtv1.NodeSchedulable] = "true"
	}
	for _, model := range models {
		labels[kubevirtv1.CPUModelLabel+model] = "true"
	}

	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
	}
}
