package aie

import (
	"context"
	"maps"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

var _ = Describe("IOMMUFD Device Plugin DaemonSet", func() {
	var (
		hco *hcov1beta1.HyperConverged
		req *common.HcoRequest
		cl  client.Client
	)

	BeforeEach(func() {
		hco = commontestutils.NewHco()
		hco.Annotations = make(map[string]string)
		req = commontestutils.NewReq(hco)
	})

	Context("newIOMMUFDDevicePluginDaemonSet", func() {
		It("should have all default values", func() {
			const testImage = "quay.io/kubevirt/iommufd-device-plugin:test"
			os.Setenv(hcoutil.IOMMUFDDevicePluginImageEnvV, testImage)
			DeferCleanup(func() {
				Expect(os.Unsetenv(hcoutil.IOMMUFDDevicePluginImageEnvV)).To(Succeed())
			})

			ds := newIOMMUFDDevicePluginDaemonSet(hco)
			Expect(ds.Name).To(Equal("iommufd-device-plugin"))
			Expect(ds.Namespace).To(Equal(hco.Namespace))
			Expect(ds.Labels).To(HaveKeyWithValue(hcoutil.AppLabel, hcoutil.HyperConvergedName))
			Expect(ds.Labels).To(HaveKeyWithValue(hcoutil.AppLabelComponent, string(hcoutil.AppComponentIOMMUFDDevicePlugin)))

			Expect(ds.Spec.Template.Spec.ServiceAccountName).To(Equal("iommufd-device-plugin"))
			Expect(ds.Spec.Template.Spec.PriorityClassName).To(Equal("system-node-critical"))
			Expect(ds.Spec.UpdateStrategy.Type).To(Equal(appsv1.RollingUpdateDaemonSetStrategyType))

			Expect(ds.Spec.Template.Spec.Containers).To(HaveLen(1))
			container := ds.Spec.Template.Spec.Containers[0]
			Expect(container.Name).To(Equal("iommufd-device-plugin"))
			Expect(container.Image).To(Equal(testImage))
			Expect(container.ImagePullPolicy).To(Equal(corev1.PullIfNotPresent))
			Expect(container.Args).To(ConsistOf("-log-level=info", "-socket-dir=/var/run/kubevirt/fd-sockets"))
			Expect(container.SecurityContext.Privileged).To(Equal(ptr.To(true)))

			Expect(container.VolumeMounts).To(HaveLen(3))
			Expect(ds.Spec.Template.Spec.Volumes).To(HaveLen(3))

			Expect(ds.Spec.Template.Spec.Volumes[2].HostPath.Type).To(Equal(ptr.To(corev1.HostPathDirectoryOrCreate)))
		})
	})

	Context("IOMMUFD device plugin DaemonSet deployment", func() {
		It("should not create if deploy-aie-webhook annotation is absent", func() {
			delete(hco.Annotations, DeployAIEAnnotation)
			cl = commontestutils.InitClient([]client.Object{hco})

			handler := NewIOMMUFDDevicePluginDaemonSetHandler(cl, commontestutils.GetScheme())

			res := handler.Ensure(req)
			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Created).To(BeFalse())
			Expect(res.Updated).To(BeFalse())
			Expect(res.Deleted).To(BeFalse())

			foundDs := &appsv1.DaemonSetList{}
			Expect(cl.List(context.Background(), foundDs)).To(Succeed())
			Expect(foundDs.Items).To(BeEmpty())
		})

		It("should delete DaemonSet when deploy-aie-webhook annotation is removed", func() {
			delete(hco.Annotations, DeployAIEAnnotation)
			ds := newIOMMUFDDevicePluginDaemonSet(hco)
			cl = commontestutils.InitClient([]client.Object{hco, ds})

			handler := NewIOMMUFDDevicePluginDaemonSetHandler(cl, commontestutils.GetScheme())

			res := handler.Ensure(req)
			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Name).To(Equal(ds.Name))
			Expect(res.Created).To(BeFalse())
			Expect(res.Updated).To(BeFalse())
			Expect(res.Deleted).To(BeTrue())

			foundDs := &appsv1.DaemonSetList{}
			Expect(cl.List(context.Background(), foundDs)).To(Succeed())
			Expect(foundDs.Items).To(BeEmpty())
		})

		It("should create DaemonSet when deploy-aie-webhook annotation is true", func() {
			hco.Annotations[DeployAIEAnnotation] = "true"
			cl = commontestutils.InitClient([]client.Object{hco})

			handler := NewIOMMUFDDevicePluginDaemonSetHandler(cl, commontestutils.GetScheme())

			res := handler.Ensure(req)
			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Name).To(Equal("iommufd-device-plugin"))
			Expect(res.Created).To(BeTrue())
			Expect(res.Updated).To(BeFalse())
			Expect(res.Deleted).To(BeFalse())

			foundDs := &appsv1.DaemonSetList{}
			Expect(cl.List(context.Background(), foundDs)).To(Succeed())
			Expect(foundDs.Items).To(HaveLen(1))
			Expect(foundDs.Items[0].Name).To(Equal("iommufd-device-plugin"))
		})
	})

	Context("IOMMUFD device plugin DaemonSet update", func() {
		It("should update DaemonSet fields if not matched to the requirements", func() {
			hco.Annotations[DeployAIEAnnotation] = "true"
			originalDs := newIOMMUFDDevicePluginDaemonSet(hco)
			modifiedDs := originalDs.DeepCopy()
			modifiedDs.Spec.Template.Spec.Containers[0].Image = "malicious:tag"
			modifiedDs.Spec.Template.Spec.Containers[0].SecurityContext.Privileged = ptr.To(false)
			modifiedDs.Spec.Template.Spec.Volumes = nil
			cl = commontestutils.InitClient([]client.Object{hco, modifiedDs})

			handler := NewIOMMUFDDevicePluginDaemonSetHandler(cl, commontestutils.GetScheme())

			res := handler.Ensure(req)
			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Created).To(BeFalse())
			Expect(res.Updated).To(BeTrue())
			Expect(res.Deleted).To(BeFalse())

			reconciledDs := &appsv1.DaemonSet{}
			Expect(cl.Get(context.Background(), client.ObjectKey{Name: res.Name, Namespace: hco.Namespace}, reconciledDs)).To(Succeed())
			Expect(reconciledDs.Spec.Template.Spec.Containers[0].Image).
				To(Equal(originalDs.Spec.Template.Spec.Containers[0].Image))
			Expect(reconciledDs.Spec.Template.Spec.Containers[0].SecurityContext.Privileged).
				To(Equal(originalDs.Spec.Template.Spec.Containers[0].SecurityContext.Privileged))
			Expect(reconciledDs.Spec.Template.Spec.Volumes).
				To(Equal(originalDs.Spec.Template.Spec.Volumes))
		})

		It("should reconcile labels if they are missing while preserving user labels", func() {
			hco.Annotations[DeployAIEAnnotation] = "true"
			ds := newIOMMUFDDevicePluginDaemonSet(hco)
			expectedLabels := maps.Clone(ds.Labels)
			delete(ds.Labels, "app.kubernetes.io/component")
			ds.Labels["user-added-label"] = "user-value"
			cl = commontestutils.InitClient([]client.Object{hco, ds})

			handler := NewIOMMUFDDevicePluginDaemonSetHandler(cl, commontestutils.GetScheme())

			res := handler.Ensure(req)
			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Created).To(BeFalse())
			Expect(res.Updated).To(BeTrue())
			Expect(res.Deleted).To(BeFalse())

			foundDs := &appsv1.DaemonSet{}
			Expect(cl.Get(context.Background(), client.ObjectKey{Name: "iommufd-device-plugin", Namespace: hco.Namespace}, foundDs)).To(Succeed())

			for key, value := range expectedLabels {
				Expect(foundDs.Labels).To(HaveKeyWithValue(key, value))
			}
			Expect(foundDs.Labels).To(HaveKeyWithValue("user-added-label", "user-value"))
		})
	})
})
