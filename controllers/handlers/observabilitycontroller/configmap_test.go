package observabilitycontroller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/dirtest"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

const sampleGuestDeviceDriversConfig = `version: v1alpha1
devices:
  - deviceID: "1041"
    deviceName: "VirtIO network device"
    current:
      - guestOSVersionID: "2022"
        driverVersion: "100.95.104.26200"
`

var _ = Describe("readGuestDeviceDriversConfig", func() {
	AfterEach(func() {
		Expect(readGuestDeviceDriversConfig(dirtest.New())).To(Succeed())
	})

	It("should default to empty content when the directory does not exist", func() {
		Expect(readGuestDeviceDriversConfig(dirtest.New())).To(Succeed())
		Expect(guestDeviceDriversConfig).To(BeEmpty())
	})

	It("should default to empty content when the file does not exist", func() {
		fs := dirtest.New(dirtest.WithDir("guestDeviceDrivers"))
		Expect(readGuestDeviceDriversConfig(fs)).To(Succeed())
		Expect(guestDeviceDriversConfig).To(BeEmpty())
	})

	It("should load the file content when present", func() {
		fs := dirtest.New(dirtest.WithFile(guestDeviceDriversManifestLocation, []byte(sampleGuestDeviceDriversConfig)))
		Expect(readGuestDeviceDriversConfig(fs)).To(Succeed())
		Expect(guestDeviceDriversConfig).To(Equal(sampleGuestDeviceDriversConfig))
	})
})

var _ = Describe("GuestDeviceDriverExists", func() {
	BeforeEach(func() {
		originalContent := guestDeviceDriversConfig
		DeferCleanup(func() {
			guestDeviceDriversConfig = originalContent
		})
	})

	It("should return false when no build-time content was injected", func() {
		guestDeviceDriversConfig = ""
		Expect(GuestDeviceDriverExists()).To(BeFalse())
	})

	It("should return true when build-time content was injected", func() {
		guestDeviceDriversConfig = sampleGuestDeviceDriversConfig
		Expect(GuestDeviceDriverExists()).To(BeTrue())
	})
})

var _ = Describe("Guest Device Drivers ConfigMap", func() {
	var (
		hco *hcov1.HyperConverged
		req *common.HcoRequest
		cl  client.Client
	)

	BeforeEach(func() {
		hco = commontestutils.NewHco()
		req = commontestutils.NewReq(hco)

		originalContent := guestDeviceDriversConfig

		DeferCleanup(func() {
			guestDeviceDriversConfig = originalContent
		})

		guestDeviceDriversConfig = sampleGuestDeviceDriversConfig
	})

	Context("newGuestDeviceDriversConfigMap", func() {
		It("should build a ConfigMap with the injected content", func() {
			cm := newGuestDeviceDriversConfigMap()
			Expect(cm.Name).To(Equal(guestDeviceDriversConfigMapName))
			Expect(cm.Namespace).To(Equal(hcoutil.GetOperatorNamespaceFromEnv()))
			Expect(cm.Data).To(HaveKeyWithValue(guestDeviceDriversConfigMapKey, sampleGuestDeviceDriversConfig))
		})
	})

	Context("when the deployObservabilityController feature gate is disabled", func() {
		It("should not create the ConfigMap", func() {
			cl = commontestutils.InitClient([]client.Object{hco})

			handler := NewGuestDeviceDriversConfigMapHandler(cl, commontestutils.GetScheme())
			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Created).To(BeFalse())

			foundCMs := &corev1.ConfigMapList{}
			Expect(cl.List(context.Background(), foundCMs)).To(Succeed())
			Expect(foundCMs.Items).To(BeEmpty())
		})

		It("should leave an admin-created ConfigMap of the same name untouched", func() {
			adminCM := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      guestDeviceDriversConfigMapName,
					Namespace: hcoutil.GetOperatorNamespaceFromEnv(),
				},
				Data: map[string]string{guestDeviceDriversConfigMapKey: "admin-provided"},
			}
			cl = commontestutils.InitClient([]client.Object{hco, adminCM})

			handler := NewGuestDeviceDriversConfigMapHandler(cl, commontestutils.GetScheme())
			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Deleted).To(BeFalse())

			found := &corev1.ConfigMap{}
			Expect(cl.Get(context.Background(), client.ObjectKeyFromObject(adminCM), found)).To(Succeed())
			Expect(found.Data).To(HaveKeyWithValue(guestDeviceDriversConfigMapKey, "admin-provided"))
		})

		It("should delete a ConfigMap it created earlier", func() {
			createdByHCO := newGuestDeviceDriversConfigMap()
			cl = commontestutils.InitClient([]client.Object{hco, createdByHCO})

			handler := NewGuestDeviceDriversConfigMapHandler(cl, commontestutils.GetScheme())
			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Deleted).To(BeTrue())

			foundCMs := &corev1.ConfigMapList{}
			Expect(cl.List(context.Background(), foundCMs)).To(Succeed())
			Expect(foundCMs.Items).To(BeEmpty())
		})
	})

	Context("when the deployObservabilityController feature gate is enabled", func() {
		BeforeEach(func() {
			hco.Spec.FeatureGates.Enable(featureGateName)
		})

		It("should create the ConfigMap with the injected content", func() {
			cl = commontestutils.InitClient([]client.Object{hco})

			handler := NewGuestDeviceDriversConfigMapHandler(cl, commontestutils.GetScheme())
			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Created).To(BeTrue())

			foundCMs := &corev1.ConfigMapList{}
			Expect(cl.List(context.Background(), foundCMs)).To(Succeed())
			Expect(foundCMs.Items).To(HaveLen(1))
			Expect(foundCMs.Items[0].Name).To(Equal(guestDeviceDriversConfigMapName))
			Expect(foundCMs.Items[0].Data).To(HaveKeyWithValue(guestDeviceDriversConfigMapKey, sampleGuestDeviceDriversConfig))
		})

		It("should reconcile an existing ConfigMap back to the injected content", func() {
			staleCM := newGuestDeviceDriversConfigMap()
			staleCM.Data[guestDeviceDriversConfigMapKey] = "stale-content"
			cl = commontestutils.InitClient([]client.Object{hco, staleCM})

			handler := NewGuestDeviceDriversConfigMapHandler(cl, commontestutils.GetScheme())
			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Updated).To(BeTrue())

			found := &corev1.ConfigMap{}
			Expect(cl.Get(context.Background(), client.ObjectKeyFromObject(staleCM), found)).To(Succeed())
			Expect(found.Data).To(HaveKeyWithValue(guestDeviceDriversConfigMapKey, sampleGuestDeviceDriversConfig))
		})
	})
})
