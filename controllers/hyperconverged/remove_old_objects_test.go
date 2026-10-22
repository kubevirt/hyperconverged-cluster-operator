package hyperconverged

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/reference"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/handlers/observabilitycontroller"
)

var _ = Describe("removeOldGuestDeviceDriversConfigMap", func() {
	var (
		hco *hcov1.HyperConverged
		req *common.HcoRequest
		cl  client.Client
	)

	BeforeEach(func() {
		hco = commontestutils.NewHco()
		req = commontestutils.NewReq(hco)
	})

	Context("when build-time content is absent", func() {
		BeforeEach(func() {
			Expect(observabilitycontroller.GuestDeviceDriverExists()).To(BeFalse())
		})

		It("should delete a ConfigMap created by an earlier build that did ship content", func() {
			createdByEarlierBuild := observabilitycontroller.NewGuestDeviceDriversConfigMapWithNameOnly()
			createdByEarlierBuild.Data = map[string]string{"drivers.yaml": "version: v1alpha1\ndevices: []\n"}
			cl = commontestutils.InitClient([]client.Object{hco, createdByEarlierBuild})

			removeOldGuestDeviceDriversConfigMap(req, cl)

			foundCMs := &corev1.ConfigMapList{}
			Expect(cl.List(context.Background(), foundCMs)).To(Succeed())
			Expect(foundCMs.Items).To(BeEmpty())
		})

		It("should leave an admin-created ConfigMap of the same name untouched", func() {
			adminCM := observabilitycontroller.NewGuestDeviceDriversConfigMapWithNameOnly()
			adminCM.Labels = nil
			adminCM.Data = map[string]string{"drivers.yaml": "admin-provided"}
			cl = commontestutils.InitClient([]client.Object{hco, adminCM})

			removeOldGuestDeviceDriversConfigMap(req, cl)

			found := &corev1.ConfigMap{}
			Expect(cl.Get(context.Background(), client.ObjectKeyFromObject(adminCM), found)).To(Succeed())
			Expect(found.Data).To(HaveKeyWithValue("drivers.yaml", "admin-provided"))
		})

		It("should do nothing when no ConfigMap exists", func() {
			cl = commontestutils.InitClient([]client.Object{hco})

			removeOldGuestDeviceDriversConfigMap(req, cl)

			foundCMs := &corev1.ConfigMapList{}
			Expect(cl.List(context.Background(), foundCMs)).To(Succeed())
			Expect(foundCMs.Items).To(BeEmpty())
		})

		It("should remove the stale relatedObjects entry once the ConfigMap is deleted", func() {
			createdByEarlierBuild := observabilitycontroller.NewGuestDeviceDriversConfigMapWithNameOnly()
			createdByEarlierBuild.Data = map[string]string{"drivers.yaml": "version: v1alpha1\ndevices: []\n"}
			cl = commontestutils.InitClient([]client.Object{hco, createdByEarlierBuild})

			objectRef, err := reference.GetReference(cl.Scheme(), createdByEarlierBuild)
			Expect(err).ToNot(HaveOccurred())
			hco.Status.RelatedObjects = append(hco.Status.RelatedObjects, *objectRef)

			removeOldGuestDeviceDriversConfigMap(req, cl)

			Expect(req.Instance.Status.RelatedObjects).To(BeEmpty())
			Expect(req.StatusDirty).To(BeTrue())
		})

		It("should not mark the status dirty when there was no relatedObjects entry to remove", func() {
			cl = commontestutils.InitClient([]client.Object{hco})

			removeOldGuestDeviceDriversConfigMap(req, cl)

			Expect(req.StatusDirty).To(BeFalse())
		})
	})
})
