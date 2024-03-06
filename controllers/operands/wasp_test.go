package operands

import (
	"context"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"

	"github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
)

var _ = Describe("wasp agent", func() {
	var (
		hco *v1beta1.HyperConverged
		req *common.HcoRequest
		cl  client.Client
	)

	BeforeEach(func() {
		hco = commontestutils.NewHco()
		req = commontestutils.NewReq(hco)
		err := os.Setenv("WASP_AGENT_IMAGE", "1")
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		err := os.Unsetenv("WASP_AGENT_IMAGE")
		Expect(err).ToNot(HaveOccurred())
	})

	When("enableHigherDensityWithSwap FG is set", func() {
		BeforeEach(func() {
			hco.Spec.FeatureGates.EnableHigherDensityWithSwap = ptr.To(true)
		})
		It("should update wasp daemonset with DRY_RUN env var when dry run annotation is set", func() {
			wasp := NewWasp(hco)
			hco.Annotations = map[string]string{
				waspDryRunAnnotation: "true",
			}
			cl = commontestutils.InitClient([]client.Object{hco, wasp})
			handler := newWaspHandler(cl, commontestutils.GetScheme())

			res := handler.ensure(req)
			Expect(res.Err).ShouldNot(HaveOccurred())
			Expect(res.Created).Should(BeFalse())
			Expect(res.Updated).Should(BeTrue())
			Expect(res.Deleted).Should(BeFalse())

			foundDs := &appsv1.DaemonSet{}
			Expect(cl.Get(
				context.Background(),
				types.NamespacedName{Name: wasp.Name, Namespace: wasp.Namespace},
				foundDs)).Should(Succeed())
			container := foundDs.Spec.Template.Spec.Containers[0]
			actualValue := ""
			for _, envVar := range container.Env {
				if envVar.Name == "DRY_RUN" {
					actualValue = envVar.Value
				}
			}
			Expect(actualValue).To(Equal("true"))

		})
		Context("should create if not present - ", func() {
			It("wasp-agent DaemonSet", func() {
				cl = commontestutils.InitClient([]client.Object{hco})
				handler := newWaspHandler(cl, commontestutils.GetScheme())

				expectedDs := NewWasp(hco)
				res := handler.ensure(req)
				Expect(res.Err).ShouldNot(HaveOccurred())
				Expect(res.Name).Should(Equal("wasp-agent"))
				Expect(res.Created).Should(BeTrue())
				Expect(res.Updated).Should(BeFalse())
				Expect(res.Deleted).Should(BeFalse())
				foundDs := &appsv1.DaemonSet{}
				Expect(cl.Get(
					context.Background(),
					types.NamespacedName{Name: expectedDs.Name, Namespace: expectedDs.Namespace},
					foundDs)).Should(Succeed())

				Expect(foundDs.Name).To(Equal(expectedDs.Name))
				Expect(foundDs.Labels).Should(HaveKeyWithValue(hcoutil.AppLabel, commontestutils.Name))
				Expect(foundDs.Namespace).To(Equal(expectedDs.Namespace))
			})
			It("wasp-agent ClusterRole", func() {
				cl = commontestutils.InitClient([]client.Object{hco})
				handler := newWaspClusterRoleHandler(cl, commontestutils.GetScheme())

				expectedClusterRole := newWaspClusterRole(hco)
				res := handler.ensure(req)
				Expect(res.Err).ShouldNot(HaveOccurred())
				Expect(res.Name).Should(Equal("wasp-agent"))
				Expect(res.Created).Should(BeTrue())
				Expect(res.Updated).Should(BeFalse())
				Expect(res.Deleted).Should(BeFalse())

				foundClusterRole := &rbacv1.ClusterRole{}
				Expect(cl.Get(context.Background(), client.ObjectKey{Name: res.Name}, foundClusterRole)).Should(Succeed())
				Expect(expectedClusterRole.Name).To(Equal(foundClusterRole.Name))
				Expect(expectedClusterRole.Labels).Should(HaveKeyWithValue(hcoutil.AppLabel, commontestutils.Name))
			})
			It("wasp-agent ClusterRoleBinding", func() {
				cl = commontestutils.InitClient([]client.Object{hco})
				handler := newWaspClusterRoleBindingHandler(cl, commontestutils.GetScheme())

				expectedClusterRoleBinding := newWaspClusterRoleBinding(hco)
				res := handler.ensure(req)
				Expect(res.Err).ShouldNot(HaveOccurred())
				Expect(res.Name).Should(Equal("wasp-agent"))
				Expect(res.Created).Should(BeTrue())
				Expect(res.Updated).Should(BeFalse())
				Expect(res.Deleted).Should(BeFalse())

				foundClusterRoleBinding := &rbacv1.ClusterRoleBinding{}
				Expect(cl.Get(context.Background(), client.ObjectKey{Name: res.Name}, foundClusterRoleBinding)).Should(Succeed())
				Expect(expectedClusterRoleBinding.Name).To(Equal(foundClusterRoleBinding.Name))
				Expect(expectedClusterRoleBinding.Labels).Should(HaveKeyWithValue(hcoutil.AppLabel, commontestutils.Name))
			})
		})
		Context("should reconcile to default if changed - ", func() {
			It("wasp-agent DaemonSet", func() {
				expectedDs := NewWasp(hco)
				outdatedDs := NewWasp(hco)

				outdatedDs.Spec.Template.Spec.ServiceAccountName = "wrong-sa"
				outdatedDs.ObjectMeta.Labels[hcoutil.AppLabel] = "wrong label"
				cl = commontestutils.InitClient([]client.Object{hco, outdatedDs})

				handler := newWaspHandler(cl, commontestutils.GetScheme())
				res := handler.ensure(req)
				Expect(res.Err).ToNot(HaveOccurred())
				Expect(res.UpgradeDone).To(BeFalse())
				Expect(res.Updated).To(BeTrue())

				foundDs := &appsv1.DaemonSet{}
				Expect(cl.Get(
					context.Background(),
					types.NamespacedName{Name: expectedDs.Name, Namespace: expectedDs.Namespace},
					foundDs)).Should(Succeed())

				Expect(foundDs.ObjectMeta.Labels).ToNot(Equal(outdatedDs.ObjectMeta.Labels))
				Expect(foundDs.ObjectMeta.Labels).To(Equal(expectedDs.ObjectMeta.Labels))
				Expect(foundDs.Spec.Template.Spec.ServiceAccountName).ToNot(Equal(outdatedDs.Spec.Template.Spec.ServiceAccountName))
			})
			It("wasp-agent ClusterRole", func() {
				expectedClusterRole := newWaspClusterRole(hco)
				outdatedClusterRole := newWaspClusterRole(hco)

				outdatedClusterRole.Rules[0].Resources[0] = "wrong-resource"
				outdatedClusterRole.Rules[0].Verbs[0] = "wrong-verb"
				outdatedClusterRole.ObjectMeta.Labels[hcoutil.AppLabel] = "wrong label"
				cl = commontestutils.InitClient([]client.Object{hco, outdatedClusterRole})

				handler := newWaspClusterRoleHandler(cl, commontestutils.GetScheme())
				res := handler.ensure(req)
				Expect(res.Err).ToNot(HaveOccurred())
				Expect(res.UpgradeDone).To(BeFalse())
				Expect(res.Updated).To(BeTrue())

				foundClusterRole := &rbacv1.ClusterRole{}
				Expect(cl.Get(context.Background(), client.ObjectKey{Name: res.Name}, foundClusterRole)).Should(Succeed())
				Expect(foundClusterRole.ObjectMeta.Labels).ToNot(Equal(outdatedClusterRole.ObjectMeta.Labels))
				Expect(foundClusterRole.ObjectMeta.Labels).To(Equal(expectedClusterRole.ObjectMeta.Labels))
				Expect(foundClusterRole.Rules).To(Equal(expectedClusterRole.Rules))

			})
			It("wasp-agent ClusterRoleBinding", func() {
				expectedClusterRoleBinding := newWaspClusterRoleBinding(hco)
				outdatedClusterRoleBinding := newWaspClusterRoleBinding(hco)

				outdatedClusterRoleBinding.Subjects[0].Name = "wrong-name"
				outdatedClusterRoleBinding.ObjectMeta.Labels[hcoutil.AppLabel] = "wrong label"
				cl = commontestutils.InitClient([]client.Object{hco, outdatedClusterRoleBinding})

				handler := newWaspClusterRoleBindingHandler(cl, commontestutils.GetScheme())
				res := handler.ensure(req)
				Expect(res.Err).ToNot(HaveOccurred())
				Expect(res.UpgradeDone).To(BeFalse())
				Expect(res.Updated).To(BeTrue())

				foundClusterRoleBinding := &rbacv1.ClusterRoleBinding{}
				Expect(cl.Get(context.Background(), client.ObjectKey{Name: res.Name}, foundClusterRoleBinding)).Should(Succeed())
				Expect(foundClusterRoleBinding.ObjectMeta.Labels).ToNot(Equal(outdatedClusterRoleBinding.ObjectMeta.Labels))
				Expect(foundClusterRoleBinding.ObjectMeta.Labels).To(Equal(expectedClusterRoleBinding.ObjectMeta.Labels))
				Expect(foundClusterRoleBinding.Subjects).To(Equal(expectedClusterRoleBinding.Subjects))
			})
		})

	})

	When("enableHigherDensityWithSwap FG is unset", func() {
		Context("should do nothing if not present - ", func() {
			It("wasp-agent DaemonSet", func() {
				cl = commontestutils.InitClient([]client.Object{hco})
				handler := newWaspHandler(cl, commontestutils.GetScheme())
				res := handler.ensure(req)

				Expect(res.Err).ShouldNot(HaveOccurred())
				Expect(res.Created).Should(BeFalse())
				Expect(res.Updated).Should(BeFalse())
				Expect(res.Deleted).Should(BeFalse())

				foundDs := &appsv1.DaemonSetList{}
				Expect(cl.List(context.Background(), foundDs)).Should(Succeed())
				Expect(foundDs.Items).Should(BeEmpty())
			})
			It("wasp-agent ClusterRole", func() {
				cl = commontestutils.InitClient([]client.Object{hco})
				handler := newWaspClusterRoleHandler(cl, commontestutils.GetScheme())
				res := handler.ensure(req)

				Expect(res.Err).ShouldNot(HaveOccurred())
				Expect(res.Created).Should(BeFalse())
				Expect(res.Updated).Should(BeFalse())
				Expect(res.Deleted).Should(BeFalse())

				foundClusterRole := &rbacv1.ClusterRoleList{}
				Expect(cl.List(context.Background(), foundClusterRole)).Should(Succeed())
				Expect(foundClusterRole.Items).Should(BeEmpty())
			})
			It("wasp-agent ClusterRoleBinding", func() {
				cl = commontestutils.InitClient([]client.Object{hco})
				handler := newWaspClusterRoleBindingHandler(cl, commontestutils.GetScheme())
				res := handler.ensure(req)

				Expect(res.Err).ShouldNot(HaveOccurred())
				Expect(res.Created).Should(BeFalse())
				Expect(res.Updated).Should(BeFalse())
				Expect(res.Deleted).Should(BeFalse())

				foundClusterRoleBinding := &rbacv1.ClusterRoleBindingList{}
				Expect(cl.List(context.Background(), foundClusterRoleBinding)).Should(Succeed())
				Expect(foundClusterRoleBinding.Items).Should(BeEmpty())
			})
		})
		Context("should remove if present - ", func() {
			It("wasp-agent DaemonSet", func() {
				newWasp := NewWasp(hco)
				cl = commontestutils.InitClient([]client.Object{hco, newWasp})
				handler := newWaspHandler(cl, commontestutils.GetScheme())

				res := handler.ensure(req)
				Expect(res.Err).ShouldNot(HaveOccurred())
				Expect(res.Name).Should(Equal(newWasp.Name))
				Expect(res.Created).Should(BeFalse())
				Expect(res.Updated).Should(BeFalse())
				Expect(res.Deleted).Should(BeTrue())

				foundDs := &appsv1.DaemonSetList{}
				Expect(cl.List(context.Background(), foundDs)).Should(Succeed())
				Expect(foundDs.Items).Should(BeEmpty())
			})
			It("wasp-agent ClusterRole", func() {
				newWaspClusterRole := newWaspClusterRole(hco)
				cl = commontestutils.InitClient([]client.Object{hco, newWaspClusterRole})
				handler := newWaspClusterRoleHandler(cl, commontestutils.GetScheme())

				res := handler.ensure(req)
				Expect(res.Err).ShouldNot(HaveOccurred())
				Expect(res.Name).Should(Equal(newWaspClusterRole.Name))
				Expect(res.Created).Should(BeFalse())
				Expect(res.Updated).Should(BeFalse())
				Expect(res.Deleted).Should(BeTrue())

				foundClusterRole := &rbacv1.ClusterRoleList{}
				Expect(cl.List(context.Background(), foundClusterRole)).Should(Succeed())
				Expect(foundClusterRole.Items).Should(BeEmpty())
			})
			It("wasp-agent ClusterRoleBinding", func() {
				newWaspClusterRoleBinding := newWaspClusterRoleBinding(hco)
				cl = commontestutils.InitClient([]client.Object{hco, newWaspClusterRoleBinding})
				handler := newWaspClusterRoleBindingHandler(cl, commontestutils.GetScheme())

				res := handler.ensure(req)
				Expect(res.Err).ShouldNot(HaveOccurred())
				Expect(res.Name).Should(Equal(newWaspClusterRoleBinding.Name))
				Expect(res.Created).Should(BeFalse())
				Expect(res.Updated).Should(BeFalse())
				Expect(res.Deleted).Should(BeTrue())

				foundClusterRoleBinding := &rbacv1.ClusterRoleBindingList{}
				Expect(cl.List(context.Background(), foundClusterRoleBinding)).Should(Succeed())
				Expect(foundClusterRoleBinding.Items).Should(BeEmpty())
			})
		})
	})

	Context("Node placement", func() {
		BeforeEach(func() {
			hco.Spec.FeatureGates.EnableHigherDensityWithSwap = ptr.To(true)
		})
		It("should add node placement if missing", func() {

			By("creating wasp agent based on HCO spec w/o node placement")
			existingResource := NewWasp(hco)

			By("adding node placement to HCO spec")
			hco.Spec.Workloads.NodePlacement = commontestutils.NewNodePlacement()

			By("Running the client and handler")
			cl = commontestutils.InitClient([]client.Object{hco, existingResource})
			handler := newWaspHandler(cl, commontestutils.GetScheme())
			res := handler.ensure(req)
			Expect(res.Err).ShouldNot(HaveOccurred())
			Expect(res.Created).Should(BeFalse())
			Expect(res.Updated).Should(BeTrue())
			Expect(res.Overwritten).To(BeFalse())
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).ToNot(HaveOccurred())

			foundResource := &appsv1.DaemonSet{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
					foundResource),
			).To(Succeed())

			Expect(existingResource.Spec.Template.Spec.NodeSelector).To(BeEmpty())
			Expect(existingResource.Spec.Template.Spec.Affinity).To(BeNil())
			Expect(existingResource.Spec.Template.Spec.Tolerations).To(BeEmpty())

			Expect(foundResource.Spec.Template.Spec.NodeSelector).To(BeEquivalentTo(hco.Spec.Workloads.NodePlacement.NodeSelector))
			Expect(foundResource.Spec.Template.Spec.Affinity).To(BeEquivalentTo(hco.Spec.Workloads.NodePlacement.Affinity))
			Expect(foundResource.Spec.Template.Spec.Tolerations).To(BeEquivalentTo(hco.Spec.Workloads.NodePlacement.Tolerations))
		})
		It("should remove node placement if missing in HCO CR", func() {
			By("create fake old HCO CR with node placement configuration")
			oldHco := commontestutils.NewHco()
			oldHco.Spec.Workloads.NodePlacement = commontestutils.NewNodePlacement()

			By("create wasp DS based on old HCO")
			existingResource := NewWasp(oldHco)

			By("initialize client based on current HCO without node placement")
			cl := commontestutils.InitClient([]client.Object{hco, existingResource})

			handler := newWaspHandler(cl, commontestutils.GetScheme())
			res := handler.ensure(req)
			Expect(res.Created).To(BeFalse())
			Expect(res.Updated).To(BeTrue())
			Expect(res.Overwritten).To(BeFalse())
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).ToNot(HaveOccurred())

			foundResource := &appsv1.DaemonSet{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
					foundResource),
			).To(Succeed())

			Expect(existingResource.Spec.Template.Spec.NodeSelector).ToNot(BeEmpty())
			Expect(existingResource.Spec.Template.Spec.Affinity).ToNot(BeNil())
			Expect(existingResource.Spec.Template.Spec.Tolerations).ToNot(BeEmpty())
			Expect(foundResource.Spec.Template.Spec.NodeSelector).To(BeEmpty())
			Expect(foundResource.Spec.Template.Spec.Affinity).To(BeNil())
			Expect(foundResource.Spec.Template.Spec.Tolerations).To(BeEmpty())
			Expect(req.Conditions).To(BeEmpty())
		})
		It("should modify node placement according to HCO CR", func() {
			By("create wasp agent based on current HCO spec")
			hco.Spec.Workloads.NodePlacement = commontestutils.NewNodePlacement()
			existingResource := NewWasp(hco)

			By("modify HCO spec with updated node placement")
			hco.Spec.Workloads.NodePlacement.Tolerations = append(hco.Spec.Workloads.NodePlacement.Tolerations, v1.Toleration{
				Key: "key12", Operator: "operator12", Value: "value12", Effect: "effect12", TolerationSeconds: ptr.To[int64](12),
			})
			hco.Spec.Workloads.NodePlacement.NodeSelector["key2"] = "something entirely else"

			By("initialize client based on updated HCO spec")
			cl := commontestutils.InitClient([]client.Object{hco, existingResource})

			handler := newWaspHandler(cl, commontestutils.GetScheme())
			res := handler.ensure(req)
			Expect(res.Created).To(BeFalse())
			Expect(res.Updated).To(BeTrue())
			Expect(res.Overwritten).To(BeFalse())
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).ToNot(HaveOccurred())

			foundResource := &appsv1.DaemonSet{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
					foundResource),
			).To(Succeed())

			Expect(existingResource.Spec.Template.Spec.Affinity.NodeAffinity).ToNot(BeNil())
			Expect(existingResource.Spec.Template.Spec.Tolerations).To(HaveLen(2))
			Expect(existingResource.Spec.Template.Spec.NodeSelector).Should(HaveKeyWithValue("key2", "value2"))

			Expect(foundResource.Spec.Template.Spec.Affinity.NodeAffinity).ToNot(BeNil())
			Expect(foundResource.Spec.Template.Spec.Tolerations).To(HaveLen(3))
			Expect(foundResource.Spec.Template.Spec.NodeSelector).Should(HaveKeyWithValue("key2", "something entirely else"))

		})
		It("should overwrite node placement if directly set on wasp agent DaemonSet", func() {
			hco.Spec.Workloads = hcov1beta1.HyperConvergedConfig{NodePlacement: commontestutils.NewNodePlacement()}
			existingResource := NewWasp(hco)

			By("mock a reconciliation triggered by a change in the deployment")
			req.HCOTriggered = false

			By("modify deployment Kubevirt Console Plugin Deployment node placement")
			existingResource.Spec.Template.Spec.Tolerations = append(hco.Spec.Workloads.NodePlacement.Tolerations, v1.Toleration{
				Key: "key12", Operator: "operator12", Value: "value12", Effect: "effect12", TolerationSeconds: ptr.To[int64](12),
			})
			existingResource.Spec.Template.Spec.NodeSelector["key2"] = "BADvalue2"

			cl := commontestutils.InitClient([]client.Object{hco, existingResource})
			handler := newWaspHandler(cl, commontestutils.GetScheme())
			res := handler.ensure(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Updated).To(BeTrue())
			Expect(res.Overwritten).To(BeTrue())
			Expect(res.Err).ToNot(HaveOccurred())

			foundResource := &appsv1.DaemonSet{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
					foundResource),
			).To(Succeed())

			Expect(existingResource.Spec.Template.Spec.Tolerations).To(HaveLen(3))
			Expect(existingResource.Spec.Template.Spec.NodeSelector).Should(HaveKeyWithValue("key2", "BADvalue2"))

			Expect(foundResource.Spec.Template.Spec.Tolerations).To(HaveLen(2))
			Expect(foundResource.Spec.Template.Spec.NodeSelector).Should(HaveKeyWithValue("key2", "value2"))

			Expect(req.Conditions).To(BeEmpty())
		})
	})
})
