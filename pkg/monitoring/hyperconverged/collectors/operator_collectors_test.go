package collectors

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/nodeinfo"
)

var _ = Describe("HyperConverged Collectors", func() {
	var hco *hcov1beta1.HyperConverged

	BeforeEach(func() {
		hco = commontestutils.NewHco()

		origNodeInfoFunc := nodeinfo.GetWorkloadsArchitectures

		DeferCleanup(func() {
			nodeinfo.GetWorkloadsArchitectures = origNodeInfoFunc
		})
	})

	Describe("kubevirt_hco_multi_arch_boot_images_enabled", func() {
		When("cluster is multi architectures", func() {
			BeforeEach(func() {
				nodeinfo.GetWorkloadsArchitectures = func() []string {
					return []string{"arch1", "arch2"}
				}
			})

			When("we deploy DICTs", func() {
				BeforeEach(func() {
					hco.Status.DataImportCronTemplates = []hcov1beta1.DataImportCronTemplateStatus{
						{
							DataImportCronTemplate: hcov1beta1.DataImportCronTemplate{
								ObjectMeta: metav1.ObjectMeta{
									Name: "image1",
								},
							},
						},
						{
							DataImportCronTemplate: hcov1beta1.DataImportCronTemplate{
								ObjectMeta: metav1.ObjectMeta{
									Name: "image2",
								},
							},
						},
					}
				})

				It("should be set and enabled, if multi-arch dict enabled", func() {
					hco.Spec.FeatureGates.EnableMultiArchBootImageImport = ptr.To(true)

					cli := commontestutils.InitClient([]client.Object{hco})
					isSet, isEnabled := isMultiArchBootImagesFeatureEnabled(cli)
					Expect(isSet).To(BeTrue())
					Expect(isEnabled).To(BeTrue())
				})

				It("should be set and disabled, if multi-arch dict disabled", func() {
					hco.Spec.FeatureGates.EnableMultiArchBootImageImport = ptr.To(false)

					cli := commontestutils.InitClient([]client.Object{hco})
					isSet, isEnabled := isMultiArchBootImagesFeatureEnabled(cli)
					Expect(isSet).To(BeTrue())
					Expect(isEnabled).To(BeFalse())
				})

				It("should be set and disabled, if multi-arch dict is not set", func() {
					hco.Spec.FeatureGates.EnableMultiArchBootImageImport = nil

					cli := commontestutils.InitClient([]client.Object{hco})
					isSet, isEnabled := isMultiArchBootImagesFeatureEnabled(cli)
					Expect(isSet).To(BeTrue())
					Expect(isEnabled).To(BeFalse())
				})
			})

			When("we don't deploy DICTS", func() {
				BeforeEach(func() {
					hco.Status.DataImportCronTemplates = nil
				})

				It("should not be set, if multi-arch dict enabled", func() {
					hco.Spec.FeatureGates.EnableMultiArchBootImageImport = ptr.To(true)

					cli := commontestutils.InitClient([]client.Object{hco})
					isSet, isEnabled := isMultiArchBootImagesFeatureEnabled(cli)
					Expect(isSet).To(BeFalse())
					Expect(isEnabled).To(BeFalse())
				})

				It("should not be set, if multi-arch dict disabled", func() {
					hco.Spec.FeatureGates.EnableMultiArchBootImageImport = ptr.To(false)

					cli := commontestutils.InitClient([]client.Object{hco})
					isSet, isEnabled := isMultiArchBootImagesFeatureEnabled(cli)
					Expect(isSet).To(BeFalse())
					Expect(isEnabled).To(BeFalse())
				})
			})
		})

		When("cluster is with single architecture", func() {
			BeforeEach(func() {
				nodeinfo.GetWorkloadsArchitectures = func() []string {
					return []string{"single-arch"}
				}
			})

			When("we deploy DICTs", func() {
				BeforeEach(func() {
					hco.Status.DataImportCronTemplates = []hcov1beta1.DataImportCronTemplateStatus{
						{
							DataImportCronTemplate: hcov1beta1.DataImportCronTemplate{
								ObjectMeta: metav1.ObjectMeta{
									Name: "image1",
								},
							},
						},
						{
							DataImportCronTemplate: hcov1beta1.DataImportCronTemplate{
								ObjectMeta: metav1.ObjectMeta{
									Name: "image2",
								},
							},
						},
					}
				})

				It("should not be set, if multi-arch dict enabled", func() {
					hco.Spec.FeatureGates.EnableMultiArchBootImageImport = ptr.To(true)

					cli := commontestutils.InitClient([]client.Object{hco})
					isSet, isEnabled := isMultiArchBootImagesFeatureEnabled(cli)
					Expect(isSet).To(BeFalse())
					Expect(isEnabled).To(BeFalse())
				})

				It("should not be set, if multi-arch dict disabled", func() {
					hco.Spec.FeatureGates.EnableMultiArchBootImageImport = ptr.To(false)

					cli := commontestutils.InitClient([]client.Object{hco})
					isSet, isEnabled := isMultiArchBootImagesFeatureEnabled(cli)

					Expect(isSet).To(BeFalse())
					Expect(isEnabled).To(BeFalse())
				})

				It("should not be set, if multi-arch dict is not set", func() {
					hco.Spec.FeatureGates.EnableMultiArchBootImageImport = nil

					cli := commontestutils.InitClient([]client.Object{hco})
					isSet, isEnabled := isMultiArchBootImagesFeatureEnabled(cli)

					Expect(isSet).To(BeFalse())
					Expect(isEnabled).To(BeFalse())
				})
			})

			When("we don't deploy DICTs", func() {
				BeforeEach(func() {
					hco.Status.DataImportCronTemplates = nil
				})

				It("should not be set, if multi-arch dict enabled", func() {
					hco.Spec.FeatureGates.EnableMultiArchBootImageImport = ptr.To(true)

					cli := commontestutils.InitClient([]client.Object{hco})
					isSet, isEnabled := isMultiArchBootImagesFeatureEnabled(cli)

					Expect(isSet).To(BeFalse())
					Expect(isEnabled).To(BeFalse())
				})

				It("should not be set, if multi-arch dict disabled", func() {
					hco.Spec.FeatureGates.EnableMultiArchBootImageImport = ptr.To(false)

					cli := commontestutils.InitClient([]client.Object{hco})
					isSet, isEnabled := isMultiArchBootImagesFeatureEnabled(cli)

					Expect(isSet).To(BeFalse())
					Expect(isEnabled).To(BeFalse())
				})
			})
		})
	})
})

func isMultiArchBootImagesFeatureEnabled(cli client.Client) (isSet, isEnabled bool) {
	callback := getMultiArchBootImagesStatusCallback(cli, commontestutils.Namespace)

	res := callback()
	if len(res) == 0 {
		return false, false
	}

	isEnabled = res[0].Value == multiArchBootImagesFeatureEnabled

	return true, isEnabled
}
