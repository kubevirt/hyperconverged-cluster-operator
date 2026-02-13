package mutator

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

var _ = Describe("LauncherPodMutator", func() {
	const testNamespace = "kubevirt-hyperconverged"

	Context("when feature is enabled", func() {
		var hc *hcov1beta1.HyperConverged

		BeforeEach(func() {
			hc = &hcov1beta1.HyperConverged{
				ObjectMeta: metav1.ObjectMeta{
					Name:      util.HyperConvergedName,
					Namespace: testNamespace,
				},
				Spec: hcov1beta1.HyperConvergedSpec{
					WebhooksConfig: &hcov1beta1.WebhooksConfig{
						LauncherPodMutator: &hcov1beta1.LauncherPodMutatorConfig{},
					},
				},
			}
		})

		It("should remove velero backup hook annotations from launcher pods", func() {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "virt-launcher-test-vm",
					Namespace: "default",
					Labels: map[string]string{
						"kubevirt.io": "virt-launcher",
					},
					Annotations: map[string]string{
						"pre.hook.backup.velero.io/command":   "['/bin/bash', '-c', 'echo hello']",
						"pre.hook.backup.velero.io/container": "compute",
						"post.hook.backup.velero.io/command":  "['/bin/bash', '-c', 'echo goodbye']",
						"some.other.annotation":               "keep-this",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "compute",
							Image: "test",
						},
					},
				},
			}

			cli := commontestutils.InitClient([]client.Object{hc})
			mutator := initLauncherPodMutator(cli)
			req := admission.Request{AdmissionRequest: newCreateRequest(pod, testCodec)}

			res := mutator.Handle(context.TODO(), req)
			Expect(res.Allowed).To(BeTrue())
			Expect(res.Patches).To(HaveLen(3))

			patches := res.Patches
			for _, patch := range patches {
				Expect(patch.Operation).To(Equal("remove"))
				Expect(patch.Path).To(ContainSubstring("/metadata/annotations/"))
			}
		})

		It("should not mutate non-launcher pods", func() {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "regular-pod",
					Namespace: "default",
					Labels: map[string]string{
						"app": "my-app",
					},
					Annotations: map[string]string{
						"pre.hook.backup.velero.io/command": "['/bin/bash', '-c', 'echo hello']",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "app",
							Image: "test",
						},
					},
				},
			}

			cli := commontestutils.InitClient([]client.Object{hc})
			mutator := initLauncherPodMutator(cli)
			req := admission.Request{AdmissionRequest: newCreateRequest(pod, testCodec)}

			res := mutator.Handle(context.TODO(), req)
			Expect(res.Allowed).To(BeTrue())
			Expect(res.Patches).To(BeEmpty())
		})

		It("should handle pods with no annotations", func() {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "virt-launcher-test-vm",
					Namespace: "default",
					Labels: map[string]string{
						"kubevirt.io": "virt-launcher",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "compute",
							Image: "test",
						},
					},
				},
			}

			cli := commontestutils.InitClient([]client.Object{hc})
			mutator := initLauncherPodMutator(cli)
			req := admission.Request{AdmissionRequest: newCreateRequest(pod, testCodec)}

			res := mutator.Handle(context.TODO(), req)
			Expect(res.Allowed).To(BeTrue())
			Expect(res.Patches).To(BeEmpty())
		})

		It("should handle pods with annotations but none to remove", func() {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "virt-launcher-test-vm",
					Namespace: "default",
					Labels: map[string]string{
						"kubevirt.io": "virt-launcher",
					},
					Annotations: map[string]string{
						"some.other.annotation": "keep-this",
						"another.annotation":    "also-keep",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "compute",
							Image: "test",
						},
					},
				},
			}

			cli := commontestutils.InitClient([]client.Object{hc})
			mutator := initLauncherPodMutator(cli)
			req := admission.Request{AdmissionRequest: newCreateRequest(pod, testCodec)}

			res := mutator.Handle(context.TODO(), req)
			Expect(res.Allowed).To(BeTrue())
			Expect(res.Patches).To(BeEmpty())
		})

		It("should remove all velero annotations when present", func() {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "virt-launcher-test-vm",
					Namespace: "default",
					Labels: map[string]string{
						"kubevirt.io": "virt-launcher",
					},
					Annotations: map[string]string{
						"pre.hook.backup.velero.io/command":    "['/bin/bash', '-c', 'echo hello']",
						"pre.hook.backup.velero.io/container":  "compute",
						"pre.hook.backup.velero.io/on-error":   "Fail",
						"pre.hook.backup.velero.io/timeout":    "30s",
						"post.hook.backup.velero.io/command":   "['/bin/bash', '-c', 'echo goodbye']",
						"post.hook.backup.velero.io/container": "compute",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "compute",
							Image: "test",
						},
					},
				},
			}

			cli := commontestutils.InitClient([]client.Object{hc})
			mutator := initLauncherPodMutator(cli)
			req := admission.Request{AdmissionRequest: newCreateRequest(pod, testCodec)}

			res := mutator.Handle(context.TODO(), req)
			Expect(res.Allowed).To(BeTrue())
			Expect(res.Patches).To(HaveLen(6))
		})
	})

	Context("when feature is disabled", func() {
		It("should not remove annotations when LauncherPodMutator is nil", func() {
			hc := &hcov1beta1.HyperConverged{
				ObjectMeta: metav1.ObjectMeta{
					Name:      util.HyperConvergedName,
					Namespace: testNamespace,
				},
				Spec: hcov1beta1.HyperConvergedSpec{
					WebhooksConfig: &hcov1beta1.WebhooksConfig{
						LauncherPodMutator: nil,
					},
				},
			}

			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "virt-launcher-test-vm",
					Namespace: "default",
					Labels: map[string]string{
						"kubevirt.io": "virt-launcher",
					},
					Annotations: map[string]string{
						"pre.hook.backup.velero.io/command": "['/bin/bash', '-c', 'echo hello']",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "compute",
							Image: "test",
						},
					},
				},
			}

			cli := commontestutils.InitClient([]client.Object{hc})
			mutator := initLauncherPodMutator(cli)
			req := admission.Request{AdmissionRequest: newCreateRequest(pod, testCodec)}

			res := mutator.Handle(context.TODO(), req)
			Expect(res.Allowed).To(BeTrue())
			Expect(res.Patches).To(BeEmpty())
		})

		It("should not remove annotations when config is nil", func() {
			hc := &hcov1beta1.HyperConverged{
				ObjectMeta: metav1.ObjectMeta{
					Name:      util.HyperConvergedName,
					Namespace: testNamespace,
				},
				Spec: hcov1beta1.HyperConvergedSpec{},
			}

			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "virt-launcher-test-vm",
					Namespace: "default",
					Labels: map[string]string{
						"kubevirt.io": "virt-launcher",
					},
					Annotations: map[string]string{
						"pre.hook.backup.velero.io/command": "['/bin/bash', '-c', 'echo hello']",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "compute",
							Image: "test",
						},
					},
				},
			}

			cli := commontestutils.InitClient([]client.Object{hc})
			mutator := initLauncherPodMutator(cli)
			req := admission.Request{AdmissionRequest: newCreateRequest(pod, testCodec)}

			res := mutator.Handle(context.TODO(), req)
			Expect(res.Allowed).To(BeTrue())
			Expect(res.Patches).To(BeEmpty())
		})
	})

	Context("edge cases", func() {
		It("should handle missing HyperConverged CR gracefully", func() {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "virt-launcher-test-vm",
					Namespace: "default",
					Labels: map[string]string{
						"kubevirt.io": "virt-launcher",
					},
					Annotations: map[string]string{
						"pre.hook.backup.velero.io/command": "['/bin/bash', '-c', 'echo hello']",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "compute",
							Image: "test",
						},
					},
				},
			}

			cli := commontestutils.InitClient(nil)
			mutator := initLauncherPodMutator(cli)
			req := admission.Request{AdmissionRequest: newCreateRequest(pod, testCodec)}

			res := mutator.Handle(context.TODO(), req)
			Expect(res.Allowed).To(BeTrue())
			Expect(res.Patches).To(BeEmpty())
		})

		It("should handle UPDATE operations by ignoring them", func() {
			hc := &hcov1beta1.HyperConverged{
				ObjectMeta: metav1.ObjectMeta{
					Name:      util.HyperConvergedName,
					Namespace: testNamespace,
				},
				Spec: hcov1beta1.HyperConvergedSpec{
					WebhooksConfig: &hcov1beta1.WebhooksConfig{
						LauncherPodMutator: &hcov1beta1.LauncherPodMutatorConfig{},
					},
				},
			}

			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "virt-launcher-test-vm",
					Namespace: "default",
					Labels: map[string]string{
						"kubevirt.io": "virt-launcher",
					},
					Annotations: map[string]string{
						"pre.hook.backup.velero.io/command": "['/bin/bash', '-c', 'echo hello']",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "compute",
							Image: "test",
						},
					},
				},
			}

			cli := commontestutils.InitClient([]client.Object{hc})
			mutator := initLauncherPodMutator(cli)
			req := admission.Request{AdmissionRequest: newRequest(admissionv1.Update, pod, testCodec)}

			res := mutator.Handle(context.TODO(), req)
			Expect(res.Allowed).To(BeTrue())
			Expect(res.Patches).To(BeEmpty())
		})

		It("should handle DELETE operations by ignoring them", func() {
			hc := &hcov1beta1.HyperConverged{
				ObjectMeta: metav1.ObjectMeta{
					Name:      util.HyperConvergedName,
					Namespace: testNamespace,
				},
				Spec: hcov1beta1.HyperConvergedSpec{
					WebhooksConfig: &hcov1beta1.WebhooksConfig{
						LauncherPodMutator: &hcov1beta1.LauncherPodMutatorConfig{},
					},
				},
			}

			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "virt-launcher-test-vm",
					Namespace: "default",
					Labels: map[string]string{
						"kubevirt.io": "virt-launcher",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "compute",
							Image: "test",
						},
					},
				},
			}

			cli := commontestutils.InitClient([]client.Object{hc})
			mutator := initLauncherPodMutator(cli)
			req := admission.Request{AdmissionRequest: newRequest(admissionv1.Delete, pod, testCodec)}

			res := mutator.Handle(context.TODO(), req)
			Expect(res.Allowed).To(BeTrue())
			Expect(res.Patches).To(BeEmpty())
		})

		It("should handle invalid pod request gracefully", func() {
			cli := commontestutils.InitClient(nil)
			mutator := initLauncherPodMutator(cli)
			req := admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{Operation: admissionv1.Create}}

			res := mutator.Handle(context.TODO(), req)
			Expect(res.Allowed).To(BeFalse())
		})
	})

	Context("helper functions", func() {
		It("isLauncherPod should correctly identify launcher pods", func() {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"kubevirt.io": "virt-launcher",
					},
				},
			}
			Expect(isLauncherPod(pod)).To(BeTrue())
		})

		It("isLauncherPod should return false for non-launcher pods", func() {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "my-app",
					},
				},
			}
			Expect(isLauncherPod(pod)).To(BeFalse())
		})

		It("isLauncherPod should return false for pods with no labels", func() {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{},
			}
			Expect(isLauncherPod(pod)).To(BeFalse())
		})

		It("generateVeleroAnnotationRemovalPatches should create correct patches", func() {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"pre.hook.backup.velero.io/command":   "['/bin/bash', '-c', 'echo hello']",
						"pre.hook.backup.velero.io/container": "compute",
						"post.hook.backup.velero.io/command":  "['/bin/bash', '-c', 'echo goodbye']",
						"keep-this":                           "value",
					},
				},
			}

			patches := generateVeleroAnnotationRemovalPatches(pod)

			Expect(patches).To(HaveLen(3))
			for _, patch := range patches {
				Expect(patch.Operation).To(Equal("remove"))
				Expect(patch.Path).To(ContainSubstring("/metadata/annotations/"))
			}
		})

		It("generateVeleroAnnotationRemovalPatches should handle empty annotations", func() {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{},
			}

			patches := generateVeleroAnnotationRemovalPatches(pod)

			Expect(patches).To(BeEmpty())
		})

		It("generateVeleroAnnotationRemovalPatches should only remove existing velero annotations", func() {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"some.other.annotation": "value",
					},
				},
			}

			patches := generateVeleroAnnotationRemovalPatches(pod)

			Expect(patches).To(BeEmpty())
		})
	})
})

func initLauncherPodMutator(testClient client.Client) *LauncherPodMutator {
	decoder := admission.NewDecoder(mutatorScheme)
	return NewLauncherPodMutator(testClient, decoder, "kubevirt-hyperconverged")
}
