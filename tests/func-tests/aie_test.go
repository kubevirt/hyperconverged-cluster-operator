package tests_test

import (
	"context"
	"slices"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubevirtcorev1 "kubevirt.io/api/core/v1"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	"github.com/kubevirt/hyperconverged-cluster-operator/api/v1/featuregates"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/handlers/aie"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
	tests "github.com/kubevirt/hyperconverged-cluster-operator/tests/func-tests"
)

var _ = Describe("AIE launcher admission policy", Label("AIE"), Serial, func() {
	tests.FlagParse()

	It("mutates new launcher Pods and removes the policy when disabled", func(ctx context.Context) {
		cli := tests.GetControllerRuntimeClient()
		hco := &hcov1.HyperConverged{}
		key := client.ObjectKey{Name: "kubevirt-hyperconverged", Namespace: tests.InstallNamespace}
		Expect(cli.Get(ctx, key, hco)).To(Succeed())
		original := hco.DeepCopy().Spec.FeatureGates
		DeferCleanup(func(ctx context.Context) {
			Expect(cli.Get(ctx, key, hco)).To(Succeed())
			before := hco.DeepCopy()
			hco.Spec.FeatureGates = original
			Expect(cli.Patch(ctx, hco, client.MergeFrom(before))).To(Succeed())
		})
		deployments := &appsv1.DeploymentList{}
		Expect(cli.List(ctx, deployments, client.InNamespace(tests.InstallNamespace), client.MatchingLabels{"name": "hyperconverged-cluster-operator"})).To(Succeed())
		Expect(deployments.Items).To(HaveLen(1))
		env := deployments.Items[0].Spec.Template.Spec.Containers[0].Env
		imageIndex := slices.IndexFunc(env, func(variable corev1.EnvVar) bool {
			return variable.Name == hcoutil.AIELauncherImageEnvV
		})
		if imageIndex == -1 || env[imageIndex].Value == "" {
			Skip("this bundle does not provide an AIE launcher image")
		}
		image := env[imageIndex].Value
		before := hco.DeepCopy()
		hco.Spec.FeatureGates.Enable(featuregates.NVIDIAAIE)
		Expect(cli.Patch(ctx, hco, client.MergeFrom(before))).To(Succeed())
		Eventually(func(g Gomega) {
			g.Expect(cli.Get(ctx, client.ObjectKey{Name: "kubevirt-aie-launcher"}, &admissionv1.MutatingAdmissionPolicy{})).To(Succeed())
			g.Expect(cli.Get(ctx, client.ObjectKey{Name: "kubevirt-aie-launcher-binding"}, &admissionv1.MutatingAdmissionPolicyBinding{})).To(Succeed())
			g.Expect(cli.Get(ctx, client.ObjectKey{Name: "kubevirt-aie-webhook"}, &admissionv1.MutatingWebhookConfiguration{})).To(MatchError(apierrors.IsNotFound, "not found"))
			g.Expect(cli.Get(ctx, client.ObjectKey{Name: "kubevirt-aie-webhook", Namespace: tests.InstallNamespace}, &appsv1.Deployment{})).To(MatchError(apierrors.IsNotFound, "not found"))
		}).WithContext(ctx).WithTimeout(time.Minute).Should(Succeed())

		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "aie-policy-test-", Namespace: tests.InstallNamespace,
				Labels:          map[string]string{kubevirtcorev1.AppLabel: aie.LauncherLabelValue},
				Annotations:     map[string]string{"user.example/keep": "yes"},
				OwnerReferences: []metav1.OwnerReference{{APIVersion: "kubevirt.io/v1", Kind: "VirtualMachineInstance", Name: "aie-policy-test", UID: types.UID("aie-policy-test"), Controller: new(true)}},
			},
			Spec: corev1.PodSpec{
				InitContainers: []corev1.Container{
					{Name: "unrelated-init", Image: "registry.k8s.io/pause:3.10"},
					{Name: "container-disk-binary", Image: "registry.k8s.io/pause:3.10", Args: []string{"keep"}},
					{Name: "image-volume-binary", Image: "registry.k8s.io/pause:3.10"},
					{Name: "guest-console-log", Image: "registry.k8s.io/pause:3.10", RestartPolicy: new(corev1.ContainerRestartPolicyAlways)},
					{Name: "volumecontainerdisk-init", Image: "registry.example/guest-disk:test"},
				},
				NodeSelector: map[string]string{"test.example/node": "keep"},
				Affinity:     &corev1.Affinity{NodeAffinity: &corev1.NodeAffinity{RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{{MatchExpressions: []corev1.NodeSelectorRequirement{{Key: "test.example/location", Operator: corev1.NodeSelectorOpExists}}}}}}},
				Containers:   []corev1.Container{{Name: "sidecar", Image: "registry.k8s.io/pause:3.10"}, {Name: "compute", Image: "registry.k8s.io/pause:3.10", Args: []string{"keep"}, Resources: corev1.ResourceRequirements{Limits: corev1.ResourceList{"devices.kubevirt.io/iommufd": resource.MustParse("1")}}}},
			},
		}
		Eventually(func(g Gomega) {
			mutated := pod.DeepCopy()
			g.Expect(cli.Create(ctx, mutated, client.DryRunAll)).To(Succeed())
			g.Expect(mutated.Spec.Containers[0].Image).To(Equal(pod.Spec.Containers[0].Image))
			g.Expect(mutated.Spec.Containers[1].Image).To(Equal(image))
			for i, original := range pod.Spec.InitContainers {
				expected := original.DeepCopy()
				if i >= 1 && i <= 3 {
					expected.Image = image
				}
				g.Expect(mutated.Spec.InitContainers[i].Image).To(Equal(expected.Image))
				g.Expect(mutated.Spec.InitContainers[i].Args).To(Equal(expected.Args))
				g.Expect(mutated.Spec.InitContainers[i].RestartPolicy).To(Equal(expected.RestartPolicy))
			}
			g.Expect(mutated.Spec.Containers[1].Args).To(Equal(pod.Spec.Containers[1].Args))
			g.Expect(mutated.Spec.NodeSelector).To(Equal(pod.Spec.NodeSelector))
			g.Expect(mutated.Spec.Affinity).To(Equal(pod.Spec.Affinity))
			g.Expect(mutated.Spec.Containers[1].Resources.Limits).To(Equal(pod.Spec.Containers[1].Resources.Limits))
			g.Expect(mutated.Annotations).To(HaveKeyWithValue("user.example/keep", "yes"))
			g.Expect(mutated.Annotations).To(HaveKeyWithValue(aie.AlternativeLauncherAnnotation, image))
		}).WithContext(ctx).WithTimeout(time.Minute).Should(Succeed())

		By("leaving unrelated Pods unchanged")
		unrelated := pod.DeepCopy()
		unrelated.Labels = nil
		Expect(cli.Create(ctx, unrelated, client.DryRunAll)).To(Succeed())
		Expect(unrelated.Spec.Containers[1].Image).To(Equal(pod.Spec.Containers[1].Image))
		for i := range pod.Spec.InitContainers {
			Expect(unrelated.Spec.InitContainers[i].Image).To(Equal(pod.Spec.InitContainers[i].Image))
		}
		Expect(unrelated.Annotations).NotTo(HaveKey(aie.AlternativeLauncherAnnotation))

		Expect(cli.Get(ctx, key, hco)).To(Succeed())
		before = hco.DeepCopy()
		hco.Spec.FeatureGates.Disable(featuregates.NVIDIAAIE)
		Expect(cli.Patch(ctx, hco, client.MergeFrom(before))).To(Succeed())
		Eventually(func(g Gomega) {
			g.Expect(cli.Get(ctx, client.ObjectKey{Name: "kubevirt-aie-launcher"}, &admissionv1.MutatingAdmissionPolicy{})).To(MatchError(apierrors.IsNotFound, "not found"))
			g.Expect(cli.Get(ctx, client.ObjectKey{Name: "kubevirt-aie-launcher-binding"}, &admissionv1.MutatingAdmissionPolicyBinding{})).To(MatchError(apierrors.IsNotFound, "not found"))
		}).WithContext(ctx).WithTimeout(time.Minute).Should(Succeed())
	})
})
