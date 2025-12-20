package nodeinfo_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/internal/nodeinfo"
)

var _ = Describe("CPU Models", func() {
	Context("3-node cluster with resource aggregation and weak node filtering", func() {
		// Create a 3-node cluster with varying CPU/memory and 4 model labels
		// Node 1: Strong node with Skylake and Haswell
		// Node 2: Medium node with Haswell and Broadwell
		// Node 3: Weak node with only Penryn (should be excluded from recommendations)
		var scheme *runtime.Scheme
		var nodes []client.Object
		var recommendedCpuModels []v1beta1.CpuModelInfo
		var modelMap map[string]v1beta1.CpuModelInfo

		BeforeEach(func(ctx context.Context) {
			scheme = runtime.NewScheme()
			Expect(corev1.AddToScheme(scheme)).To(Succeed())
			Expect(v1beta1.AddToScheme(scheme)).To(Succeed())

			nodes = []client.Object{
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "strong-node",
						Labels: map[string]string{
							nodeinfo.LabelNodeRoleWorker:                    "",
							"cpu-model.node.kubevirt.io/Skylake":            "true", // High PassMark (8900)
							"cpu-model.node.kubevirt.io/Haswell":            "true", // Medium PassMark (7200)
							"cpu-model.node.kubevirt.io/Cascadelake-Server": "true", // High PassMark (11000)
						},
					},
					Status: corev1.NodeStatus{
						NodeInfo: corev1.NodeSystemInfo{
							Architecture: "amd64",
						},
						Capacity: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("16"),
							corev1.ResourceMemory: resource.MustParse("64Gi"),
						},
						Allocatable: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("15"),
							corev1.ResourceMemory: resource.MustParse("60Gi"),
						},
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "medium-node",
						Labels: map[string]string{
							nodeinfo.LabelNodeRoleWorker:           "",
							"cpu-model.node.kubevirt.io/Haswell":   "true", // Medium PassMark (7200) - appears on 2 nodes
							"cpu-model.node.kubevirt.io/Broadwell": "true", // Medium PassMark (6800)
						},
					},
					Status: corev1.NodeStatus{
						NodeInfo: corev1.NodeSystemInfo{
							Architecture: "amd64",
						},
						Capacity: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("8"),
							corev1.ResourceMemory: resource.MustParse("32Gi"),
						},
						Allocatable: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("7"),
							corev1.ResourceMemory: resource.MustParse("30Gi"),
						},
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "weak-node",
						Labels: map[string]string{
							nodeinfo.LabelNodeRoleWorker:        "",
							"cpu-model.node.kubevirt.io/Penryn": "true", // Low PassMark (2400), only on weak node
						},
					},
					Status: corev1.NodeStatus{
						NodeInfo: corev1.NodeSystemInfo{
							Architecture: "amd64",
						},
						Capacity: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("2"),
							corev1.ResourceMemory: resource.MustParse("4Gi"),
						},
						Allocatable: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1.5"),
							corev1.ResourceMemory: resource.MustParse("3Gi"),
						},
					},
				},
			}

			cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(nodes...).Build()
			_, err := nodeinfo.HandleNodeChanges(ctx, cl, nil, GinkgoLogr)
			Expect(err).ToNot(HaveOccurred())

			recommendedCpuModels = nodeinfo.GetRecommendedCpuModels()
			Expect(recommendedCpuModels).ToNot(BeEmpty())

			modelMap = make(map[string]v1beta1.CpuModelInfo)
			for _, model := range recommendedCpuModels {
				modelMap[model.Name] = model
			}
		})

		It("should exclude models that appear only once on a weak node", func() {
			modelNames := make([]string, len(recommendedCpuModels))
			for i, model := range recommendedCpuModels {
				modelNames[i] = model.Name
			}
			Expect(modelNames).ToNot(ContainElement("Penryn"))
		})

		It("should correctly aggregate CPU across nodes", func() {
			Expect(modelMap).To(HaveKey("Skylake"))
			Expect(modelMap["Skylake"].CPU.String()).To(Equal("16"))

			Expect(modelMap).To(HaveKey("Haswell"))
			Expect(modelMap["Haswell"].CPU.String()).To(Equal("24")) // 16 + 8

			Expect(modelMap).To(HaveKey("Broadwell"))
			Expect(modelMap["Broadwell"].CPU.String()).To(Equal("8"))
		})

		It("should correctly aggregate memory across nodes", func() {
			Expect(modelMap).To(HaveKey("Skylake"))
			Expect(modelMap["Skylake"].Memory.String()).To(Equal("64Gi"))

			Expect(modelMap).To(HaveKey("Haswell"))
			Expect(modelMap["Haswell"].Memory.String()).To(Equal("96Gi")) // 64Gi + 32Gi

			Expect(modelMap).To(HaveKey("Broadwell"))
			Expect(modelMap["Broadwell"].Memory.String()).To(Equal("32Gi"))
		})

		It("should correctly count nodes for each model", func() {
			Expect(modelMap).To(HaveKey("Skylake"))
			Expect(modelMap["Skylake"].Nodes).To(Equal(1))

			Expect(modelMap).To(HaveKey("Haswell"))
			Expect(modelMap["Haswell"].Nodes).To(Equal(2))

			Expect(modelMap).To(HaveKey("Broadwell"))
			Expect(modelMap["Broadwell"].Nodes).To(Equal(1))
		})
	})
})
