package components

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("GetDeploymentSpecCliDownloads", func() {
	var params *DeploymentOperatorParams

	BeforeEach(func() {
		params = &DeploymentOperatorParams{
			CliDownloadsImage:  "registry.example.com/virt-artifacts-server:latest",
			VirtIOWinContainer: "registry.example.com/virtio-win-rhel9:latest",
			ImagePullPolicy:    "IfNotPresent",
			HcoKvIoVersion:     "1.0.0",
		}
	})

	It("should not have init containers", func() {
		spec := GetDeploymentSpecCliDownloads(params)
		Expect(spec.Template.Spec.InitContainers).To(BeEmpty())
	})

	It("should define a virtio-win image volume referencing the virtio-win container", func() {
		spec := GetDeploymentSpecCliDownloads(params)

		Expect(spec.Template.Spec.Volumes).To(HaveLen(1))
		vol := spec.Template.Spec.Volumes[0]
		Expect(vol.Name).To(Equal("virtio-win-data"))
		Expect(vol.VolumeSource.Image).ToNot(BeNil())
		Expect(vol.VolumeSource.Image.Reference).To(Equal(params.VirtIOWinContainer))
	})

	It("should mount only virtio-win.iso via subPath in the server container", func() {
		spec := GetDeploymentSpecCliDownloads(params)

		serverC := spec.Template.Spec.Containers[0]
		Expect(serverC.VolumeMounts).To(HaveLen(1))
		vm := serverC.VolumeMounts[0]
		Expect(vm.Name).To(Equal("virtio-win-data"))
		Expect(vm.MountPath).To(Equal("/home/server/src/virtio-win/virtio-win.iso"))
		Expect(vm.SubPath).To(Equal("disk/virtio-win.iso"))
		Expect(vm.ReadOnly).To(BeTrue())
	})

	It("should not request ephemeral storage", func() {
		spec := GetDeploymentSpecCliDownloads(params)

		serverC := spec.Template.Spec.Containers[0]
		Expect(serverC.Resources.Requests).ToNot(HaveKey(corev1.ResourceEphemeralStorage))
		Expect(serverC.Resources.Limits).To(BeNil())
	})
})
