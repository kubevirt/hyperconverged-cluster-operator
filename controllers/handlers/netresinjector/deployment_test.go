package netresinjector

import (
	"context"
	"maps"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/nodeinfo"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

var _ = Describe("Network Resources Injector Deployment", func() {
	const testImage = "quay.io/kubevirt/network-resources-injector:test"

	var (
		hco *hcov1.HyperConverged
		req *common.HcoRequest
		cl  client.Client
	)

	BeforeEach(func() {
		hco = commontestutils.NewHco()
		hco.Spec.Deployment.DeployNetworkResourcesInjector = new(true)
		req = commontestutils.NewReq(hco)
		Expect(os.Setenv(hcoutil.NetworkResourcesInjectorImageEnvV, testImage)).To(Succeed())

		DeferCleanup(func() {
			Expect(os.Unsetenv(hcoutil.NetworkResourcesInjectorImageEnvV)).To(Succeed())
		})
	})

	Context("newDeployment", func() {
		It("should have all default values", func() {
			origFunc := nodeinfo.IsInfrastructureHighlyAvailable
			DeferCleanup(func() {
				nodeinfo.IsInfrastructureHighlyAvailable = origFunc
			})

			nodeinfo.IsInfrastructureHighlyAvailable = func() bool {
				return true
			}

			dep := newDeployment(hco)

			Expect(dep.Name).To(Equal(deploymentName))
			Expect(dep.Namespace).To(Equal(hco.Namespace))
			Expect(dep.Labels).To(HaveKeyWithValue(hcoutil.AppLabel, hcoutil.HyperConvergedName))
			Expect(dep.Labels).To(HaveKeyWithValue(hcoutil.AppLabelComponent, string(hcoutil.AppComponentNetResInjector)))

			Expect(dep.Spec.Replicas).To(HaveValue(Equal(int32(2))))
			Expect(dep.Spec.Selector.MatchLabels).To(HaveKeyWithValue(hcoutil.AppLabel, hcoutil.HyperConvergedName))
			Expect(dep.Spec.Selector.MatchLabels).To(HaveKeyWithValue(hcoutil.AppLabelComponent, string(hcoutil.AppComponentNetResInjector)))

			Expect(dep.Spec.Template.Labels).To(HaveKeyWithValue(hcoutil.AppLabel, hcoutil.HyperConvergedName))
			Expect(dep.Spec.Template.Labels).To(HaveKeyWithValue(hcoutil.AppLabelComponent, string(hcoutil.AppComponentNetResInjector)))

			Expect(dep.Spec.Template.Spec.ServiceAccountName).To(Equal(serviceAccountName))
			Expect(dep.Spec.Template.Spec.PriorityClassName).To(Equal("system-cluster-critical"))

			// Verify control plane scheduling
			Expect(dep.Spec.Template.Spec.NodeSelector).To(HaveKeyWithValue("kubernetes.io/os", "linux"))
			Expect(dep.Spec.Template.Spec.Tolerations).To(HaveLen(2))
			Expect(dep.Spec.Template.Spec.Tolerations[0].Key).To(Equal("CriticalAddonsOnly"))
			Expect(dep.Spec.Template.Spec.Tolerations[1].Key).To(Equal("node-role.kubernetes.io/control-plane"))

			Expect(dep.Spec.Template.Spec.Affinity).ToNot(BeNil())
			Expect(dep.Spec.Template.Spec.Affinity.NodeAffinity).ToNot(BeNil())
			// Verify preferred scheduling for control plane compatibility (HyperShift, SNO)
			Expect(dep.Spec.Template.Spec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution).To(HaveLen(3))
			Expect(dep.Spec.Template.Spec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].Preference.MatchExpressions[0].Key).To(Equal("node-role.kubernetes.io/control-plane"))
			Expect(dep.Spec.Template.Spec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution[1].Preference.MatchExpressions[0].Key).To(Equal("node-role.kubevirt.io/control-plane"))
			Expect(dep.Spec.Template.Spec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution[2].Preference.MatchExpressions[0].Key).To(Equal("node-role.kubernetes.io/worker"))

			Expect(dep.Spec.Template.Spec.Affinity.PodAntiAffinity).ToNot(BeNil())
			Expect(dep.Spec.Template.Spec.Affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution).To(HaveLen(1))

			Expect(dep.Spec.Template.Spec.Containers).To(HaveLen(1))
			container := dep.Spec.Template.Spec.Containers[0]
			Expect(container.Name).To(Equal("webhook-server"))
			Expect(container.Image).To(Equal(testImage))
			Expect(container.ImagePullPolicy).To(Equal(corev1.PullIfNotPresent))
			Expect(container.Command).To(ConsistOf("webhook"))

			Expect(container.SecurityContext).ToNot(BeNil())
			Expect(container.SecurityContext.AllowPrivilegeEscalation).To(HaveValue(Equal(false)))
			Expect(container.SecurityContext.ReadOnlyRootFilesystem).To(HaveValue(Equal(true)))
			Expect(container.SecurityContext.RunAsNonRoot).To(HaveValue(Equal(true)))
			Expect(container.SecurityContext.Capabilities).ToNot(BeNil())
			Expect(container.SecurityContext.Capabilities.Drop).To(ConsistOf(corev1.Capability("ALL")))

			Expect(container.VolumeMounts).To(HaveLen(1))
			Expect(container.VolumeMounts[0].Name).To(Equal("tls"))
			Expect(container.VolumeMounts[0].MountPath).To(Equal(tlsMountPath))
			Expect(container.VolumeMounts[0].ReadOnly).To(BeTrue())

			Expect(dep.Spec.Template.Spec.Volumes).To(HaveLen(1))
			Expect(dep.Spec.Template.Spec.Volumes[0].Name).To(Equal("tls"))
			Expect(dep.Spec.Template.Spec.Volumes[0].Secret.SecretName).To(Equal(tlsSecretName))
		})

		It("should set only one replica in an SNO cluster", func() {
			origFunc := nodeinfo.IsInfrastructureHighlyAvailable
			DeferCleanup(func() {
				nodeinfo.IsInfrastructureHighlyAvailable = origFunc
			})

			nodeinfo.IsInfrastructureHighlyAvailable = func() bool {
				return false
			}

			dep := newDeployment(hco)
			Expect(dep.Spec.Replicas).To(HaveValue(Equal(int32(1))))
		})
	})

	Context("Deployment handler", func() {
		It("should create Deployment if it does not exist", func() {
			cl = commontestutils.InitClient([]client.Object{hco})

			handler := NewDeploymentHandler(cl, commontestutils.GetScheme())
			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Created).To(BeTrue())

			foundDeps := &appsv1.DeploymentList{}
			Expect(cl.List(context.Background(), foundDeps)).To(Succeed())
			Expect(foundDeps.Items).To(HaveLen(1))
			Expect(foundDeps.Items[0].Name).To(Equal(deploymentName))
		})
	})

	Context("Deployment update", func() {
		It("should update Deployment fields if not matched to the requirements", func() {
			originalDep := newDeployment(hco)
			modifiedDep := originalDep.DeepCopy()
			modifiedDep.Spec.Template.Spec.Containers[0].Image = "malicious:tag"
			modifiedDep.Spec.Template.Spec.Containers[0].SecurityContext.RunAsNonRoot = new(false)
			modifiedDep.Spec.Template.Spec.Volumes = nil
			cl = commontestutils.InitClient([]client.Object{hco, modifiedDep})

			handler := NewDeploymentHandler(cl, commontestutils.GetScheme())
			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Updated).To(BeTrue())

			reconciledDep := &appsv1.Deployment{}
			Expect(cl.Get(context.Background(), client.ObjectKey{Name: res.Name, Namespace: hco.Namespace}, reconciledDep)).To(Succeed())
			Expect(reconciledDep.Spec.Template.Spec.Containers[0].Image).To(Equal(testImage))
			Expect(reconciledDep.Spec.Template.Spec.Containers[0].SecurityContext.RunAsNonRoot).To(HaveValue(Equal(true)))
			Expect(reconciledDep.Spec.Template.Spec.Volumes).To(Equal(originalDep.Spec.Template.Spec.Volumes))
		})

		It("should reconcile labels if they are missing while preserving user labels", func() {
			dep := newDeployment(hco)
			expectedLabels := maps.Clone(dep.Labels)
			delete(dep.Labels, hcoutil.AppLabelComponent)
			dep.Labels["user-added-label"] = "user-value"
			cl = commontestutils.InitClient([]client.Object{hco, dep})

			handler := NewDeploymentHandler(cl, commontestutils.GetScheme())
			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Updated).To(BeTrue())

			foundDep := &appsv1.Deployment{}
			Expect(cl.Get(context.Background(), client.ObjectKey{Name: deploymentName, Namespace: hco.Namespace}, foundDep)).To(Succeed())

			for key, value := range expectedLabels {
				Expect(foundDep.Labels).To(HaveKeyWithValue(key, value))
			}
			Expect(foundDep.Labels).To(HaveKeyWithValue("user-added-label", "user-value"))
		})
	})
})
