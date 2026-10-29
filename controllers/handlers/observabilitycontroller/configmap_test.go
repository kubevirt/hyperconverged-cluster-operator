package observabilitycontroller

import (
	"context"
	"path"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/dirtest"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
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
	It("should return empty content when the file does not exist", func() {
		content, err := readGuestDeviceDriversConfig(dirtest.New())
		Expect(err).ToNot(HaveOccurred())
		Expect(content).To(BeEmpty())
	})

	It("should return empty content when only the directory exists", func() {
		fs := dirtest.New(dirtest.WithDir(GuestDeviceDriversManifestLocation))
		content, err := readGuestDeviceDriversConfig(fs)
		Expect(err).ToNot(HaveOccurred())
		Expect(content).To(BeEmpty())
	})

	It("should load the file content when present", func() {
		fs := dirtest.New(dirtest.WithFile(
			path.Join(GuestDeviceDriversManifestLocation, guestDeviceDriversConfigMapKey),
			[]byte(sampleGuestDeviceDriversConfig),
		))
		content, err := readGuestDeviceDriversConfig(fs)
		Expect(err).ToNot(HaveOccurred())
		Expect(content).To(Equal(sampleGuestDeviceDriversConfig))
	})
})

var _ = Describe("GetGuestDeviceDriversHandlers", func() {
	It("should not create handlers when the file is missing", func() {
		handlerList, err := GetGuestDeviceDriversHandlers(
			GinkgoLogr, commontestutils.InitClient(nil), commontestutils.GetScheme(), nil, dirtest.New())
		Expect(err).ToNot(HaveOccurred())
		Expect(handlerList).To(BeEmpty())
	})

	It("should not create handlers when the file has no content", func() {
		fs := dirtest.New(dirtest.WithFile(
			path.Join(GuestDeviceDriversManifestLocation, guestDeviceDriversConfigMapKey),
			[]byte("   \n"),
		))
		handlerList, err := GetGuestDeviceDriversHandlers(
			GinkgoLogr, commontestutils.InitClient(nil), commontestutils.GetScheme(), nil, fs)
		Expect(err).ToNot(HaveOccurred())
		Expect(handlerList).To(BeEmpty())
	})

	It("should create a handler when the file has content", func() {
		fs := dirtest.New(dirtest.WithFile(
			path.Join(GuestDeviceDriversManifestLocation, guestDeviceDriversConfigMapKey),
			[]byte(sampleGuestDeviceDriversConfig),
		))
		handlerList, err := GetGuestDeviceDriversHandlers(
			GinkgoLogr, commontestutils.InitClient(nil), commontestutils.GetScheme(), nil, fs)
		Expect(err).ToNot(HaveOccurred())
		Expect(handlerList).To(HaveLen(1))
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
	})

	newHandler := func() operands.Operand {
		return newGuestDeviceDriversConfigMapHandler(cl, commontestutils.GetScheme(), sampleGuestDeviceDriversConfig)
	}

	Context("when the deployObservabilityController feature gate is disabled", func() {
		It("should not create the ConfigMap", func() {
			cl = commontestutils.InitClient([]client.Object{hco})

			res := newHandler().Ensure(req)

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

			res := newHandler().Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Deleted).To(BeFalse())

			found := &corev1.ConfigMap{}
			Expect(cl.Get(context.Background(), client.ObjectKeyFromObject(adminCM), found)).To(Succeed())
			Expect(found.Data).To(HaveKeyWithValue(guestDeviceDriversConfigMapKey, "admin-provided"))
		})

		It("should delete a ConfigMap it created earlier", func() {
			createdByHCO := newGuestDeviceDriversConfigMap(sampleGuestDeviceDriversConfig)
			cl = commontestutils.InitClient([]client.Object{hco, createdByHCO})

			res := newHandler().Ensure(req)

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

			res := newHandler().Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Created).To(BeTrue())

			foundCMs := &corev1.ConfigMapList{}
			Expect(cl.List(context.Background(), foundCMs)).To(Succeed())
			Expect(foundCMs.Items).To(HaveLen(1))
			Expect(foundCMs.Items[0].Name).To(Equal(guestDeviceDriversConfigMapName))
			Expect(foundCMs.Items[0].Data).To(HaveKeyWithValue(guestDeviceDriversConfigMapKey, sampleGuestDeviceDriversConfig))
		})

		It("should reconcile an existing ConfigMap back to the injected content", func() {
			staleCM := newGuestDeviceDriversConfigMap(sampleGuestDeviceDriversConfig)
			staleCM.Data[guestDeviceDriversConfigMapKey] = "stale-content"
			cl = commontestutils.InitClient([]client.Object{hco, staleCM})

			res := newHandler().Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Updated).To(BeTrue())

			found := &corev1.ConfigMap{}
			Expect(cl.Get(context.Background(), client.ObjectKeyFromObject(staleCM), found)).To(Succeed())
			Expect(found.Data).To(HaveKeyWithValue(guestDeviceDriversConfigMapKey, sampleGuestDeviceDriversConfig))
		})
	})
})
