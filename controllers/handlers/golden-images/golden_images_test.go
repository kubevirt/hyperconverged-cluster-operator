package golden_images

import (
	"fmt"
	"maps"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/nodeinfo"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
	cdiv1beta1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

func TestOperators(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Golden Images Suite")
}

var _ = Describe("Test data import cron template", func() {
	dir := path.Join(os.TempDir(), fmt.Sprint(time.Now().UTC().Unix()))
	origFunc := getDataImportCronTemplatesFileLocation

	var (
		hco *hcov1beta1.HyperConverged

		image1, image2, image3, image4                         hcov1beta1.DataImportCronTemplate
		statusImage1, statusImage2, statusImage3, statusImage4 hcov1beta1.DataImportCronTemplateStatus

		testFilesLocation = getTestFilesLocation() + "/dataImportCronTemplates"
	)
	BeforeEach(func() {
		hco = commontestutils.NewHco()

		image1, statusImage1 = makeDICT(1, true)
		image2, statusImage2 = makeDICT(2, true)
		image3, statusImage3 = makeDICT(3, false)
		image4, statusImage4 = makeDICT(4, false)

		getDataImportCronTemplatesFileLocation = func() string {
			return dir
		}
	})

	AfterEach(func() {
		getDataImportCronTemplatesFileLocation = origFunc
	})

	It("should read the dataImportCronTemplates file", func() {
		By("directory does not exist - no error")
		Expect(readDataImportCronTemplatesFromFile()).To(Succeed())
		Expect(dataImportCronTemplateHardCodedMap).To(BeEmpty())

		By("file does not exist - no error")
		Expect(os.Mkdir(dir, os.ModePerm)).To(Succeed())
		defer func() { _ = os.RemoveAll(dir) }()

		Expect(readDataImportCronTemplatesFromFile()).To(Succeed())
		Expect(dataImportCronTemplateHardCodedMap).To(BeEmpty())

		destFile := path.Join(dir, "dataImportCronTemplates.yaml")

		By("valid file exits")
		Expect(commontestutils.CopyFile(destFile, path.Join(testFilesLocation, "dataImportCronTemplates.yaml"))).To(Succeed())
		defer os.Remove(destFile)
		Expect(readDataImportCronTemplatesFromFile()).To(Succeed())
		Expect(dataImportCronTemplateHardCodedMap).To(HaveLen(2))

		By("the file is wrong")
		Expect(commontestutils.CopyFile(destFile, path.Join(testFilesLocation, "wrongDataImportCronTemplates.yaml"))).To(Succeed())
		defer os.Remove(destFile)
		Expect(readDataImportCronTemplatesFromFile()).To(HaveOccurred())
		Expect(dataImportCronTemplateHardCodedMap).To(BeEmpty())
	})

	Context("test GetDataImportCronTemplates", func() {
		It("should not return the hard coded list dataImportCron FeatureGate is false", func() {
			hco.Spec.EnableCommonBootImageImport = ptr.To(false)
			dataImportCronTemplateHardCodedMap = map[string]hcov1beta1.DataImportCronTemplate{
				image1.Name: image1,
				image2.Name: image2,
			}
			hco.Spec.DataImportCronTemplates = []hcov1beta1.DataImportCronTemplate{image3, image4}
			list, err := GetDataImportCronTemplates(hco)
			Expect(err).ToNot(HaveOccurred())
			Expect(list).To(HaveLen(2))
			Expect(list).To(ContainElements(statusImage3, statusImage4))

			hco.Spec.DataImportCronTemplates = []hcov1beta1.DataImportCronTemplate{}
			list, err = GetDataImportCronTemplates(hco)
			Expect(err).ToNot(HaveOccurred())
			Expect(list).To(BeEmpty())
		})

		It("should return an empty list if both the hard-coded list and the list from HC are empty", func() {
			hcoWithEmptyList := commontestutils.NewHco()
			hcoWithEmptyList.Spec.EnableCommonBootImageImport = ptr.To(true)
			hcoWithEmptyList.Spec.DataImportCronTemplates = []hcov1beta1.DataImportCronTemplate{}
			hcoWithNilList := commontestutils.NewHco()
			hcoWithNilList.Spec.EnableCommonBootImageImport = ptr.To(true)
			hcoWithNilList.Spec.DataImportCronTemplates = nil

			dataImportCronTemplateHardCodedMap = nil
			Expect(GetDataImportCronTemplates(hcoWithNilList)).To(BeNil())
			Expect(GetDataImportCronTemplates(hcoWithEmptyList)).To(BeNil())
			dataImportCronTemplateHardCodedMap = make(map[string]hcov1beta1.DataImportCronTemplate)
			Expect(GetDataImportCronTemplates(hcoWithNilList)).To(BeNil())
			Expect(GetDataImportCronTemplates(hcoWithEmptyList)).To(BeNil())
		})

		It("Should add the CR list to the hard-coded list", func() {
			dataImportCronTemplateHardCodedMap = map[string]hcov1beta1.DataImportCronTemplate{
				image1.Name: image1,
				image2.Name: image2,
			}
			hco.Spec.EnableCommonBootImageImport = ptr.To(true)
			hco.Spec.DataImportCronTemplates = []hcov1beta1.DataImportCronTemplate{image3, image4}
			goldenImageList, err := GetDataImportCronTemplates(hco)
			Expect(err).ToNot(HaveOccurred())
			Expect(goldenImageList).To(HaveLen(4))
			Expect(goldenImageList).To(HaveCap(4))
			Expect(goldenImageList).To(ContainElements(statusImage1, statusImage2, statusImage3, statusImage4))
		})

		It("Should not add a common DIC template if it marked as disabled", func() {
			dataImportCronTemplateHardCodedMap = map[string]hcov1beta1.DataImportCronTemplate{
				image1.Name: image1,
				image2.Name: image2,
			}
			hco.Spec.EnableCommonBootImageImport = ptr.To(true)

			disabledImage1, _ := makeDICT(1, true)
			disableDict(&disabledImage1)
			enabledImage2, expectedStatus2 := makeDICT(2, true)
			enableDict(&enabledImage2, &expectedStatus2)
			expectedStatus2.Status.Modified = true

			hco.Spec.DataImportCronTemplates = []hcov1beta1.DataImportCronTemplate{disabledImage1, enabledImage2, image3, image4}
			goldenImageList, err := GetDataImportCronTemplates(hco)
			Expect(err).ToNot(HaveOccurred())
			Expect(goldenImageList).To(HaveLen(3))
			Expect(goldenImageList).To(HaveCap(4))

			Expect(goldenImageList[0].Status).To(Equal(expectedStatus2.Status))
			Expect(goldenImageList[0].ObjectMeta).To(Equal(expectedStatus2.ObjectMeta))
			Expect(goldenImageList[0].Spec).To(Equal(expectedStatus2.Spec))
			Expect(goldenImageList).To(ContainElements(expectedStatus2, statusImage3, statusImage4))
		})

		It("should not add user DIC template if it is disabled", func() {
			dataImportCronTemplateHardCodedMap = nil
			hco.Spec.EnableCommonBootImageImport = ptr.To(true)

			disableDict(&image1)
			enableDict(&image2, &statusImage2)

			hco.Spec.DataImportCronTemplates = []hcov1beta1.DataImportCronTemplate{image1, image2}
			goldenImageList, err := GetDataImportCronTemplates(hco)
			Expect(err).ToNot(HaveOccurred())
			Expect(goldenImageList).To(HaveLen(1))

			statusImageEnabled := hcov1beta1.DataImportCronTemplateStatus{
				DataImportCronTemplate: image2,
				Status:                 hcov1beta1.DataImportCronStatus{},
			}

			Expect(goldenImageList).To(ContainElements(statusImageEnabled))
		})

		It("Should reject if the CR list contain DIC templates with the same name, when there are also common DIC templates", func() {
			dataImportCronTemplateHardCodedMap = map[string]hcov1beta1.DataImportCronTemplate{
				image1.Name: image1,
				image2.Name: image2,
			}
			hco.Spec.EnableCommonBootImageImport = ptr.To(true)

			image3.Name = image4.Name

			hco.Spec.DataImportCronTemplates = []hcov1beta1.DataImportCronTemplate{image3, image4}
			_, err := GetDataImportCronTemplates(hco)
			Expect(err).To(HaveOccurred())
		})

		It("Should reject if the CR list contain DIC templates with the same name", func() {
			hco.Spec.EnableCommonBootImageImport = ptr.To(true)

			image3.Name = image4.Name

			hco.Spec.DataImportCronTemplates = []hcov1beta1.DataImportCronTemplate{image3, image4}
			_, err := GetDataImportCronTemplates(hco)
			Expect(err).To(HaveOccurred())
		})

		It("Should not add the CR list to the hard-coded list, if it's empty", func() {
			By("CR list is nil")
			dataImportCronTemplateHardCodedMap = map[string]hcov1beta1.DataImportCronTemplate{
				image1.Name: image1,
				image2.Name: image2,
			}

			hco.Spec.EnableCommonBootImageImport = ptr.To(true)
			hco.Spec.DataImportCronTemplates = nil
			goldenImageList, err := GetDataImportCronTemplates(hco)
			Expect(err).ToNot(HaveOccurred())
			Expect(goldenImageList).To(HaveLen(2))
			Expect(goldenImageList).To(HaveCap(2))
			Expect(goldenImageList).To(ContainElements(statusImage1, statusImage2))

			By("CR list is empty")
			hco.Spec.DataImportCronTemplates = []hcov1beta1.DataImportCronTemplate{}
			goldenImageList, err = GetDataImportCronTemplates(hco)
			Expect(err).ToNot(HaveOccurred())
			Expect(goldenImageList).To(HaveLen(2))
			Expect(goldenImageList).To(ContainElements(statusImage1, statusImage2))
		})

		It("Should return only the CR list, if the hard-coded list is empty", func() {
			hco.Spec.EnableCommonBootImageImport = ptr.To(true)
			hco.Spec.DataImportCronTemplates = []hcov1beta1.DataImportCronTemplate{image3, image4}

			By("when dataImportCronTemplateHardCodedList is nil")
			dataImportCronTemplateHardCodedMap = nil
			goldenImageList, err := GetDataImportCronTemplates(hco)
			Expect(err).ToNot(HaveOccurred())
			Expect(goldenImageList).To(HaveLen(2))
			Expect(goldenImageList).To(HaveCap(2))
			Expect(goldenImageList).To(ContainElements(statusImage3, statusImage4))

			By("when dataImportCronTemplateHardCodedList is empty")
			dataImportCronTemplateHardCodedMap = map[string]hcov1beta1.DataImportCronTemplate{}
			goldenImageList, err = GetDataImportCronTemplates(hco)
			Expect(err).ToNot(HaveOccurred())
			Expect(goldenImageList).To(HaveLen(2))
			Expect(goldenImageList).To(HaveCap(2))
			Expect(goldenImageList).To(ContainElements(statusImage3, statusImage4))
		})

		It("Should replace the common DICT registry field if the CR list includes it", func() {
			const (
				modifiedURL = "docker://someregistry/modified"
				anotherURL  = "docker://someregistry/anotherURL"
			)

			image1.Spec.Template.Spec.Source = &cdiv1beta1.DataVolumeSource{
				Registry: &cdiv1beta1.DataVolumeSourceRegistry{URL: ptr.To(modifiedURL)},
			}

			dataImportCronTemplateHardCodedMap = map[string]hcov1beta1.DataImportCronTemplate{
				image1.Name: image1,
				image2.Name: image2,
			}

			hco.Spec.EnableCommonBootImageImport = ptr.To(true)

			modifiedImage1, _ := makeDICT(1, true)
			modifiedImage1.Spec.Template.Spec.Source = &cdiv1beta1.DataVolumeSource{
				Registry: &cdiv1beta1.DataVolumeSourceRegistry{URL: ptr.To(anotherURL)},
			}

			By("check that if the CR schedule is empty, HCO adds it from the common dict")
			modifiedImage1.Spec.Schedule = ""

			hco.Spec.DataImportCronTemplates = []hcov1beta1.DataImportCronTemplate{modifiedImage1, image3, image4}

			goldenImageList, err := GetDataImportCronTemplates(hco)
			Expect(err).ToNot(HaveOccurred())
			Expect(goldenImageList).To(HaveLen(4))
			Expect(goldenImageList).To(HaveCap(4))

			modifiedImage1.Spec.Schedule = image1.Spec.Schedule

			for _, dict := range goldenImageList {
				switch dict.Name {
				case "image1":
					Expect(dict.Spec).To(Equal(modifiedImage1.Spec))
					Expect(dict.Status.Modified).To(BeTrue())
					Expect(dict.Status.CommonTemplate).To(BeTrue())
				case "image2":
					Expect(dict.Status.Modified).To(BeFalse())
					Expect(dict.Status.CommonTemplate).To(BeTrue())
				}
			}
		})

		It("Should replace the common DICT storage field if the CR list includes it", func() {
			image1.Spec.Template.Spec.Storage = &cdiv1beta1.StorageSpec{
				VolumeName:       "volume-name",
				StorageClassName: ptr.To("testName"),
			}

			dataImportCronTemplateHardCodedMap = map[string]hcov1beta1.DataImportCronTemplate{
				image1.Name: image1,
				image2.Name: image2,
			}

			hco.Spec.EnableCommonBootImageImport = ptr.To(true)

			storageFromCr := &cdiv1beta1.StorageSpec{
				VolumeName: "another-class-name",

				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"key1": "value1",
						"key2": "value2",
					},
				},
			}

			modifiedImage1 := image1.DeepCopy()
			modifiedImage1.Spec.Template.Spec.Storage = storageFromCr.DeepCopy()

			hco.Spec.DataImportCronTemplates = []hcov1beta1.DataImportCronTemplate{*modifiedImage1, image3, image4}

			goldenImageList, err := GetDataImportCronTemplates(hco)
			Expect(err).ToNot(HaveOccurred())
			Expect(goldenImageList).To(HaveLen(4))
			Expect(goldenImageList).To(HaveCap(4))

			for _, dict := range goldenImageList {
				switch dict.Name {
				case "image1":
					Expect(dict.Spec.Template.Spec.Storage).To(BeEquivalentTo(storageFromCr))
					Expect(dict.Status.Modified).To(BeTrue())
					Expect(dict.Status.CommonTemplate).To(BeTrue())
				case "image2":
					Expect(dict.Status.Modified).To(BeFalse())
					Expect(dict.Status.CommonTemplate).To(BeTrue())
				}
			}
		})

		It("Should replace several common DICT fields, if the CR list includes it", func() {
			dataImportCronTemplateHardCodedMap = map[string]hcov1beta1.DataImportCronTemplate{
				image1.Name: image1,
				image2.Name: image2,
			}

			hco.Spec.EnableCommonBootImageImport = ptr.To(true)

			modifiedImage1 := image1.DeepCopy()
			modifiedImage1.Spec.RetentionPolicy = ptr.To(cdiv1beta1.DataImportCronRetainAll)
			modifiedImage1.Spec.GarbageCollect = ptr.To(cdiv1beta1.DataImportCronGarbageCollectOutdated)
			modifiedImage1.Spec.ImportsToKeep = ptr.To[int32](5)

			hco.Spec.DataImportCronTemplates = []hcov1beta1.DataImportCronTemplate{*modifiedImage1, image3, image4}

			goldenImageList, err := GetDataImportCronTemplates(hco)
			Expect(err).ToNot(HaveOccurred())
			Expect(goldenImageList).To(HaveLen(4))
			Expect(goldenImageList).To(HaveCap(4))

			for _, dict := range goldenImageList {
				switch dict.Name {
				case "image1":
					Expect(dict.Spec.RetentionPolicy).To(HaveValue(Equal(cdiv1beta1.DataImportCronRetainAll)))
					Expect(dict.Spec.GarbageCollect).To(HaveValue(Equal(cdiv1beta1.DataImportCronGarbageCollectOutdated)))
					Expect(dict.Spec.ImportsToKeep).To(HaveValue(Equal(int32(5))))

					Expect(dict.Status.Modified).To(BeTrue())
					Expect(dict.Status.CommonTemplate).To(BeTrue())
				case "image2":
					Expect(dict.Status.Modified).To(BeFalse())
					Expect(dict.Status.CommonTemplate).To(BeTrue())
				}
			}
		})

		It("Should add the cdi.kubevirt.io/storage.bind.immediate.requested annotation if missing", func() {
			image2.Annotations = map[string]string{
				CDIImmediateBindAnnotation: "false",
			}

			image3.Annotations = map[string]string{
				CDIImmediateBindAnnotation: "false",
			}

			dataImportCronTemplateHardCodedMap = map[string]hcov1beta1.DataImportCronTemplate{
				image1.Name: image1,
				image2.Name: image2,
			}

			hco.Spec.EnableCommonBootImageImport = ptr.To(true)

			hco.Spec.DataImportCronTemplates = []hcov1beta1.DataImportCronTemplate{image3, image4}

			goldenImageStatuses, err := GetDataImportCronTemplates(hco)
			Expect(err).ToNot(HaveOccurred())
			goldenImageList := HCODictSliceToSSP(hco, goldenImageStatuses)

			Expect(goldenImageList).To(HaveLen(4))
			Expect(goldenImageList).To(HaveCap(4))

			for _, dict := range goldenImageList {
				switch dict.Name {
				case "image1", "image4":
					Expect(dict.Annotations).To(HaveKeyWithValue(CDIImmediateBindAnnotation, "true"), "%s should have CDIImmediateBindAnnotation set to true", dict.Name)
				case "image2", "image3":
					Expect(dict.Annotations).To(HaveKeyWithValue(CDIImmediateBindAnnotation, "false"), "%s should have CDIImmediateBindAnnotation set to false", dict.Name)
				}
			}
		})

		It("Should add the cdi.kubevirt.io/storage.bind.immediate.requested annotation if missing, when customizing common dicts", func() {
			image1.Annotations = map[string]string{
				CDIImmediateBindAnnotation: "true",
			}

			image2.Annotations = map[string]string{
				CDIImmediateBindAnnotation: "true",
			}

			annotationModified := image1.DeepCopy()
			annotationModified.Annotations[CDIImmediateBindAnnotation] = "false"
			annotationMissingInCR := image2.DeepCopy()
			annotationMissingInCR.Annotations = nil
			annotationExistsInCR := image3.DeepCopy()
			annotationExistsInCR.Annotations = map[string]string{
				CDIImmediateBindAnnotation: "false",
			}
			annotationMissingInBoth := image4.DeepCopy()

			dataImportCronTemplateHardCodedMap = map[string]hcov1beta1.DataImportCronTemplate{
				image1.Name: image1,
				image2.Name: image2,
				image3.Name: image3,
				image4.Name: image4,
			}

			hco.Spec.EnableCommonBootImageImport = ptr.To(true)

			hco.Spec.DataImportCronTemplates = []hcov1beta1.DataImportCronTemplate{*annotationModified, *annotationMissingInCR, *annotationExistsInCR, *annotationMissingInBoth}

			goldenImageStatuses, err := GetDataImportCronTemplates(hco)
			Expect(err).ToNot(HaveOccurred())
			goldenImageList := HCODictSliceToSSP(hco, goldenImageStatuses)

			Expect(goldenImageList).To(HaveLen(4))
			Expect(goldenImageList).To(HaveCap(4))

			for _, dict := range goldenImageList {
				var expectedAnnotation string
				switch dict.Name {
				case "image1", "image3":
					expectedAnnotation = "false"
				case "image4", "image2":
					expectedAnnotation = "true"
				}
				Expect(dict.Annotations).To(HaveKeyWithValue(CDIImmediateBindAnnotation, expectedAnnotation), "%s should have CDIImmediateBindAnnotation set to %s", dict.Name, expectedAnnotation)
			}
		})

		It("should use custom namespace for common dicts, if defined in the hyperConverged CR", func() {
			const (
				customNS   = "custom-ns"
				modifiedNS = "some-other-ns"
			)
			dataImportCronTemplateHardCodedMap = map[string]hcov1beta1.DataImportCronTemplate{
				image1.Name: image1,
				image2.Name: image2,
			}

			withModifiedNS := image2.DeepCopy()
			withModifiedNS.Namespace = modifiedNS

			hco.Spec.EnableCommonBootImageImport = ptr.To(true)
			hco.Spec.CommonBootImageNamespace = ptr.To(customNS)

			hco.Spec.DataImportCronTemplates = []hcov1beta1.DataImportCronTemplate{*withModifiedNS, image3}

			goldenImageList, err := GetDataImportCronTemplates(hco)
			Expect(err).ToNot(HaveOccurred())
			Expect(goldenImageList).To(HaveLen(3))

			Expect(goldenImageList[0].Name).To(Equal("image1"))
			Expect(goldenImageList[0].Namespace).To(Equal(customNS))

			Expect(goldenImageList[1].Name).To(Equal("image2"))
			Expect(goldenImageList[1].Namespace).To(Equal(modifiedNS))

			Expect(goldenImageList[2].Name).To(Equal("image3"))
			Expect(goldenImageList[2].Namespace).To(Equal(image3.Namespace))
		})
	})

	Context("test ApplyDataImportSchedule", func() {
		It("should not set the schedule filed if missing from the status", func() {
			dataImportCronTemplateHardCodedMap = map[string]hcov1beta1.DataImportCronTemplate{
				image1.Name: image1,
				image2.Name: image2,
			}

			ApplyDataImportSchedule(hco)

			Expect(dataImportCronTemplateHardCodedMap[image1.Name].Spec.Schedule).To(Equal("1 */12 * * *"))
			Expect(dataImportCronTemplateHardCodedMap[image2.Name].Spec.Schedule).To(Equal("2 */12 * * *"))
		})

		It("should set the variable and the images, if the schedule is in the status field", func() {
			const schedule = "42 */1 * * *"
			hco.Status.DataImportSchedule = schedule

			dataImportCronTemplateHardCodedMap = map[string]hcov1beta1.DataImportCronTemplate{
				image1.Name: image1,
				image2.Name: image2,
			}

			ApplyDataImportSchedule(hco)
			for _, image := range dataImportCronTemplateHardCodedMap {
				Expect(image.Spec.Schedule).To(Equal(schedule))
			}
		})
	})

	Context("test data import cron templates in Status", func() {
		var destFile string
		BeforeEach(func() {
			Expect(os.Mkdir(dir, os.ModePerm)).To(Succeed())
			destFile = path.Join(dir, "dataImportCronTemplates.yaml")
			Expect(
				commontestutils.CopyFile(destFile, path.Join(testFilesLocation, "dataImportCronTemplates.yaml")),
			).To(Succeed())
			Expect(readDataImportCronTemplatesFromFile()).To(Succeed())
		})

		AfterEach(func() {
			_ = os.RemoveAll(dir)
			_ = os.Remove(destFile)
		})
	})

	Context("test isDataImportCronTemplateEnabled", func() {
		It("should be true if the annotation is missing", func() {
			image1.Annotations = nil
			Expect(isDataImportCronTemplateEnabled(image1)).To(BeTrue())
		})

		It("should be true if the annotation is missing", func() {
			image1.Annotations = make(map[string]string)
			Expect(isDataImportCronTemplateEnabled(image1)).To(BeTrue())
		})

		It("should be true if the annotation is set to 'true'", func() {
			image1.Annotations = map[string]string{hcoutil.DataImportCronEnabledAnnotation: "true"}
			Expect(isDataImportCronTemplateEnabled(image1)).To(BeTrue())
		})

		It("should be true if the annotation is set to 'TRUE'", func() {
			image1.Annotations = map[string]string{hcoutil.DataImportCronEnabledAnnotation: "TRUE"}
			Expect(isDataImportCronTemplateEnabled(image1)).To(BeTrue())
		})

		It("should be true if the annotation is set to 'TrUe'", func() {
			image1.Annotations = map[string]string{hcoutil.DataImportCronEnabledAnnotation: "TrUe"}
			Expect(isDataImportCronTemplateEnabled(image1)).To(BeTrue())
		})

		It("should be false if the annotation is empty", func() {
			image1.Annotations = map[string]string{hcoutil.DataImportCronEnabledAnnotation: ""}
			Expect(isDataImportCronTemplateEnabled(image1)).To(BeFalse())
		})

		It("should be false if the annotation is set to 'false'", func() {
			image1.Annotations = map[string]string{hcoutil.DataImportCronEnabledAnnotation: "false"}
			Expect(isDataImportCronTemplateEnabled(image1)).To(BeFalse())
		})

		It("should be false if the annotation is set to 'something-else'", func() {
			image1.Annotations = map[string]string{hcoutil.DataImportCronEnabledAnnotation: "something-else"}
			Expect(isDataImportCronTemplateEnabled(image1)).To(BeFalse())
		})
	})

	Context("heterogeneous cluster", func() {
		Context("the EnableMultiArchBootImageImport FG", func() {
			BeforeEach(func() {
				image1.Annotations = map[string]string{
					"testing.kubevirt.io/fake.annotation": "true",
					MultiArchDICTAnnotation:               "amd64,arm64,s390x",
				}
				image2.Annotations = map[string]string{
					"testing.kubevirt.io/fake.annotation": "true",
					MultiArchDICTAnnotation:               "amd64,arm64,s390x",
				}
				image3.Annotations = map[string]string{
					"testing.kubevirt.io/fake.annotation": "true",
					MultiArchDICTAnnotation:               "amd64,arm64,s390x",
				}
				image4.Annotations = map[string]string{
					"testing.kubevirt.io/fake.annotation": "true",
					MultiArchDICTAnnotation:               "amd64,arm64,s390x",
				}

				dataImportCronTemplateHardCodedMap = map[string]hcov1beta1.DataImportCronTemplate{
					image1.Name: image1,
					image2.Name: image2,
				}

				hco.Spec.DataImportCronTemplates = []hcov1beta1.DataImportCronTemplate{image3, image4}

				nodeinfo.GetWorkloadsArchitectures = func() []string {
					return []string{"amd64", "arm64", "s390x"}
				}
			})

			It("should drop the ssp.kubevirt.io/dict.architectures annotation, when the FG is disabled (default)", func() {
				hco.Spec.EnableCommonBootImageImport = ptr.To(true)
				hco.Spec.FeatureGates.EnableMultiArchBootImageImport = ptr.To(false)

				dictsStatuses, err := GetDataImportCronTemplates(hco)
				Expect(err).ToNot(HaveOccurred())
				Expect(dictsStatuses).To(HaveLen(4))

				for _, status := range dictsStatuses {
					Expect(status.Annotations).To(HaveKeyWithValue("testing.kubevirt.io/fake.annotation", "true"))
					Expect(status.Annotations).To(HaveKeyWithValue(MultiArchDICTAnnotation, "amd64,arm64,s390x"))
					Expect(status.Status.Conditions).To(BeEmpty())
				}

				sspDicts := HCODictSliceToSSP(hco, dictsStatuses)
				Expect(sspDicts).To(HaveLen(4))

				for _, dict := range sspDicts {
					Expect(dict.Annotations).To(HaveKeyWithValue(CDIImmediateBindAnnotation, "true"))
					Expect(dict.Annotations).To(HaveKeyWithValue("testing.kubevirt.io/fake.annotation", "true"))
					Expect(dict.Annotations).ToNot(HaveKey(MultiArchDICTAnnotation))
				}
			})

			It("should not drop the ssp.kubevirt.io/dict.architectures annotation, when the FG is enabled", func() {
				hco.Spec.EnableCommonBootImageImport = ptr.To(true)
				hco.Spec.FeatureGates.EnableMultiArchBootImageImport = ptr.To(true)

				dictsStatuses, err := GetDataImportCronTemplates(hco)
				Expect(err).ToNot(HaveOccurred())
				Expect(dictsStatuses).To(HaveLen(4))

				for _, status := range dictsStatuses {
					Expect(status.Annotations).To(HaveKeyWithValue("testing.kubevirt.io/fake.annotation", "true"))
					Expect(status.Annotations).To(HaveKeyWithValue(MultiArchDICTAnnotation, "amd64,arm64,s390x"))
					Expect(status.Status.Conditions).To(BeEmpty())
				}

				sspDicts := HCODictSliceToSSP(hco, dictsStatuses)
				Expect(sspDicts).To(HaveLen(4))

				for _, dict := range sspDicts {
					Expect(dict.Annotations).To(HaveKeyWithValue(CDIImmediateBindAnnotation, "true"))
					Expect(dict.Annotations).To(HaveKeyWithValue("testing.kubevirt.io/fake.annotation", "true"))
					Expect(dict.Annotations).To(HaveKeyWithValue(MultiArchDICTAnnotation, "amd64,arm64,s390x"))
				}
			})

			It("should remove unsupported architectures from the annotation", func() {
				hco.Spec.EnableCommonBootImageImport = ptr.To(true)
				hco.Spec.FeatureGates.EnableMultiArchBootImageImport = ptr.To(true)

				nodeinfo.GetWorkloadsArchitectures = func() []string {
					return []string{"amd64", "arm64"}
				}

				image1.Annotations[MultiArchDICTAnnotation] = "amd64,s390x"
				image2.Annotations[MultiArchDICTAnnotation] = "amd64,s390x"
				image3.Annotations[MultiArchDICTAnnotation] = "amd64,s390x"
				image4.Annotations[MultiArchDICTAnnotation] = "amd64,s390x"

				dictsStatuses, err := GetDataImportCronTemplates(hco)
				Expect(err).ToNot(HaveOccurred())
				Expect(dictsStatuses).To(HaveLen(4))

				for _, status := range dictsStatuses {
					Expect(status.Annotations).To(HaveKeyWithValue("testing.kubevirt.io/fake.annotation", "true"))
					Expect(status.Annotations).To(HaveKeyWithValue(MultiArchDICTAnnotation, "amd64"))
					Expect(status.Status.Conditions).To(BeEmpty())
				}

				sspDicts := HCODictSliceToSSP(hco, dictsStatuses)
				Expect(sspDicts).To(HaveLen(4))

				for _, dict := range sspDicts {
					Expect(dict.Annotations).To(HaveKeyWithValue(CDIImmediateBindAnnotation, "true"))
					Expect(dict.Annotations).To(HaveKeyWithValue("testing.kubevirt.io/fake.annotation", "true"))
					Expect(dict.Annotations).To(HaveKeyWithValue(MultiArchDICTAnnotation, "amd64"))
				}
			})

			It("should drop a DICT with no supported architectures", func() {
				hco.Spec.EnableCommonBootImageImport = ptr.To(true)
				hco.Spec.FeatureGates.EnableMultiArchBootImageImport = ptr.To(true)

				nodeinfo.GetWorkloadsArchitectures = func() []string {
					return []string{"amd64", "s390x"}
				}

				image2.Annotations[MultiArchDICTAnnotation] = "arm64"
				image4.Annotations[MultiArchDICTAnnotation] = "arm64"

				dictsStatuses, err := GetDataImportCronTemplates(hco)
				Expect(err).ToNot(HaveOccurred())
				Expect(dictsStatuses).To(HaveLen(4))

				Expect(dictsStatuses[0].Annotations).To(HaveKeyWithValue("testing.kubevirt.io/fake.annotation", "true"))
				Expect(dictsStatuses[0].Annotations).To(HaveKeyWithValue(MultiArchDICTAnnotation, "amd64,s390x"))
				Expect(dictsStatuses[0].Status.Conditions).To(BeEmpty())

				Expect(dictsStatuses[1].Annotations).To(HaveKeyWithValue("testing.kubevirt.io/fake.annotation", "true"))
				Expect(dictsStatuses[1].Annotations).To(HaveKeyWithValue(MultiArchDICTAnnotation, ""))
				Expect(dictsStatuses[1].Status.OriginalSupportedArchitectures).To(Equal("arm64"))
				Expect(meta.IsStatusConditionFalse(dictsStatuses[1].Status.Conditions, dictConditionDeployedType)).To(BeTrue())

				Expect(dictsStatuses[2].Annotations).To(HaveKeyWithValue("testing.kubevirt.io/fake.annotation", "true"))
				Expect(dictsStatuses[2].Annotations).To(HaveKeyWithValue(MultiArchDICTAnnotation, "amd64,s390x"))
				Expect(dictsStatuses[2].Status.Conditions).To(BeEmpty())

				Expect(dictsStatuses[3].Annotations).To(HaveKeyWithValue("testing.kubevirt.io/fake.annotation", "true"))
				Expect(dictsStatuses[3].Annotations).To(HaveKeyWithValue(MultiArchDICTAnnotation, ""))
				Expect(dictsStatuses[3].Status.OriginalSupportedArchitectures).To(Equal("arm64"))
				Expect(meta.IsStatusConditionFalse(dictsStatuses[3].Status.Conditions, dictConditionDeployedType)).To(BeTrue())

				sspDicts := HCODictSliceToSSP(hco, dictsStatuses)
				Expect(sspDicts).To(HaveLen(2))

				for _, dict := range sspDicts {
					Expect(dict.Annotations).To(HaveKeyWithValue(CDIImmediateBindAnnotation, "true"))
					Expect(dict.Annotations).To(HaveKeyWithValue("testing.kubevirt.io/fake.annotation", "true"))
					Expect(dict.Annotations).To(HaveKeyWithValue(MultiArchDICTAnnotation, "amd64,s390x"))
				}
			})

			It("should not add the multi-arch annotation if wasn't already exist in the original DICT", func() {
				hco.Spec.EnableCommonBootImageImport = ptr.To(true)
				hco.Spec.FeatureGates.EnableMultiArchBootImageImport = ptr.To(true)

				nodeinfo.GetWorkloadsArchitectures = func() []string {
					return []string{"amd64", "s390x", "other-arch"}
				}

				delete(image2.Annotations, MultiArchDICTAnnotation)
				delete(image4.Annotations, MultiArchDICTAnnotation)

				dictsStatuses, err := GetDataImportCronTemplates(hco)
				Expect(err).ToNot(HaveOccurred())
				Expect(dictsStatuses).To(HaveLen(4))

				for i, dictStatus := range dictsStatuses {
					if i%2 == 0 {
						Expect(dictStatus.Annotations).To(HaveKeyWithValue(MultiArchDICTAnnotation, "amd64,s390x"))
						Expect(dictStatus.Status.OriginalSupportedArchitectures).To(Equal("amd64,arm64,s390x"))
						Expect(dictStatus.Status.Conditions).To(BeEmpty())
					} else {
						Expect(dictStatus.Annotations).ToNot(HaveKey(MultiArchDICTAnnotation))
						Expect(dictStatus.Status.OriginalSupportedArchitectures).To(Equal(""))
						Expect(dictStatus.Status.Conditions).To(BeEmpty())
					}
				}

				sspDicts := HCODictSliceToSSP(hco, dictsStatuses)
				Expect(sspDicts).To(HaveLen(4))

				for i, dict := range sspDicts {
					Expect(dict.Annotations).To(HaveKeyWithValue(CDIImmediateBindAnnotation, "true"))
					Expect(dict.Annotations).To(HaveKeyWithValue("testing.kubevirt.io/fake.annotation", "true"))
					if i%2 == 0 {
						Expect(dict.Annotations).To(HaveKeyWithValue(MultiArchDICTAnnotation, "amd64,s390x"))
					} else {
						Expect(dict.Annotations).ToNot(HaveKey(MultiArchDICTAnnotation))
					}
				}
			})
		})

		Context("test customizeCommonDictAnnotations", func() {
			DescribeTable("should customize the common DICT annotations", func(modifyTargetDict, modifyCRDict func(*hcov1beta1.DataImportCronTemplate), enableMultiArchBootImageImport bool, matcher gomegatypes.GomegaMatcher) {
				crDict, targetDict := makeDICT(1, true)

				modifyTargetDict(&targetDict.DataImportCronTemplate)
				modifyCRDict(&crDict)
				origCrAnnotations := maps.Clone(crDict.Annotations)

				customizeCommonDictAnnotations(&targetDict, crDict, enableMultiArchBootImageImport)
				Expect(targetDict.Annotations).To(matcher)
				Expect(crDict.Annotations).To(Equal(origCrAnnotations))
			},
				Entry("if enableMultiArchBootImageImport if false, just copy from CR to target; when annotations are different",
					func(targetDict *hcov1beta1.DataImportCronTemplate) {
						targetDict.Annotations = map[string]string{MultiArchDICTAnnotation: "targetVal1,targetVal2,targetVal3"}
					},
					func(crDict *hcov1beta1.DataImportCronTemplate) {
						crDict.Annotations = map[string]string{MultiArchDICTAnnotation: "crVal1,crVal2,crVal3"}
					},
					false,
					HaveKeyWithValue(MultiArchDICTAnnotation, "crVal1,crVal2,crVal3"),
				),
				Entry("if enableMultiArchBootImageImport if false, leave the target value; when cr annotations is missing",
					func(targetDict *hcov1beta1.DataImportCronTemplate) {
						targetDict.Annotations = map[string]string{MultiArchDICTAnnotation: "targetVal1,targetVal2,targetVal3"}
					},
					func(crDict *hcov1beta1.DataImportCronTemplate) {
						crDict.Annotations = map[string]string{"testing.kubevirt.io/fake.annotation": "true"}
					},
					false,
					HaveKeyWithValue(MultiArchDICTAnnotation, "targetVal1,targetVal2,targetVal3"),
				),
				Entry("if enableMultiArchBootImageImport if false, just copy from CR to target; when target annotation is missing",
					func(targetDict *hcov1beta1.DataImportCronTemplate) {
						targetDict.Annotations = map[string]string{"testing.kubevirt.io/fake.annotation": "true"}
					},
					func(crDict *hcov1beta1.DataImportCronTemplate) {
						crDict.Annotations = map[string]string{MultiArchDICTAnnotation: "crVal1,crVal2,crVal3"}
					},
					false,
					HaveKeyWithValue(MultiArchDICTAnnotation, "crVal1,crVal2,crVal3"),
				),
				Entry("if enableMultiArchBootImageImport if false, do nothing; when both annotation are missing",
					func(targetDict *hcov1beta1.DataImportCronTemplate) {
						targetDict.Annotations = map[string]string{"testing.kubevirt.io/fake.annotation": "true"}
					},
					func(crDict *hcov1beta1.DataImportCronTemplate) {
						crDict.Annotations = map[string]string{"testing.kubevirt.io/fake.annotation": "true"}
					},
					false,
					Not(HaveKey(MultiArchDICTAnnotation)),
				),
				Entry("if reg was not changed, and MultiArchDICTAnnotation annotation exists in target, keep it; when annotations are the same",
					func(targetDict *hcov1beta1.DataImportCronTemplate) {
						targetDict.Annotations = map[string]string{MultiArchDICTAnnotation: "targetVal1,targetVal2,targetVal3"}
					},
					func(crDict *hcov1beta1.DataImportCronTemplate) {
						crDict.Annotations = map[string]string{MultiArchDICTAnnotation: "targetVal1,targetVal2,targetVal3"}
					},
					true,
					HaveKeyWithValue(MultiArchDICTAnnotation, "targetVal1,targetVal2,targetVal3"),
				),
				Entry("if reg was not changed, and MultiArchDICTAnnotation annotation exists in target, keep it; when annotations are different",
					func(targetDict *hcov1beta1.DataImportCronTemplate) {
						targetDict.Annotations = map[string]string{MultiArchDICTAnnotation: "targetVal1,targetVal2,targetVal3"}
					},
					func(crDict *hcov1beta1.DataImportCronTemplate) {
						crDict.Annotations = map[string]string{MultiArchDICTAnnotation: "crVal1,crVal2,crVal3"}
					},
					true,
					HaveKeyWithValue(MultiArchDICTAnnotation, "targetVal1,targetVal2,targetVal3"),
				),
				Entry("if reg was not changed, and MultiArchDICTAnnotation annotation exists in target, keep it; when annotations are missing from CR",
					func(targetDict *hcov1beta1.DataImportCronTemplate) {
						targetDict.Annotations = map[string]string{MultiArchDICTAnnotation: "targetVal1,targetVal2,targetVal3"}
					},
					func(crDict *hcov1beta1.DataImportCronTemplate) {
						crDict.Annotations = map[string]string{}
					},
					true,
					HaveKeyWithValue(MultiArchDICTAnnotation, "targetVal1,targetVal2,targetVal3"),
				),
				Entry("if reg was not changed, and MultiArchDICTAnnotation annotation exists in target, keep it; when annotations are missing from CR",
					func(targetDict *hcov1beta1.DataImportCronTemplate) {
						targetDict.Annotations = map[string]string{MultiArchDICTAnnotation: "targetVal1,targetVal2,targetVal3"}
					},
					func(crDict *hcov1beta1.DataImportCronTemplate) {
						crDict.Annotations = nil
					},
					true,
					HaveKeyWithValue(MultiArchDICTAnnotation, "targetVal1,targetVal2,targetVal3"),
				),
				Entry("if reg was not changed, and MultiArchDICTAnnotation annotation does not exist in target don't override; no target annotations",
					func(targetDict *hcov1beta1.DataImportCronTemplate) {
						targetDict.Annotations = nil
					},
					func(crDict *hcov1beta1.DataImportCronTemplate) {
						crDict.Annotations = map[string]string{MultiArchDICTAnnotation: "crVal1,crVal2,crVal3"}
					},
					true,
					Not(HaveKey(MultiArchDICTAnnotation)),
				),
				Entry("if reg was not changed, and MultiArchDICTAnnotation annotation does not exist in target don't override; empty target annotations",
					func(targetDict *hcov1beta1.DataImportCronTemplate) {
						targetDict.Annotations = make(map[string]string)
					},
					func(crDict *hcov1beta1.DataImportCronTemplate) {
						crDict.Annotations = map[string]string{MultiArchDICTAnnotation: "crVal1,crVal2,crVal3"}
					},
					true,
					Not(HaveKey(MultiArchDICTAnnotation)),
				),
				Entry("if reg was not changed, and MultiArchDICTAnnotation annotation does not exist in target, don't override; no MultiArchDICTAnnotation annotation in target",
					func(targetDict *hcov1beta1.DataImportCronTemplate) {
						targetDict.Annotations = map[string]string{
							"testing.kubevirt.io/fake.annotation": "true",
						}
					},
					func(crDict *hcov1beta1.DataImportCronTemplate) {
						crDict.Annotations = map[string]string{MultiArchDICTAnnotation: "crVal1,crVal2,crVal3"}
					},
					true,
					Not(HaveKey(MultiArchDICTAnnotation)),
				),
				Entry("if reg was changed, and MultiArchDICTAnnotation annotation exists in CR, copy to target; when annotations are different",
					func(targetDict *hcov1beta1.DataImportCronTemplate) {
						targetDict.Annotations = map[string]string{MultiArchDICTAnnotation: "targetVal1,targetVal2,targetVal3"}
					},
					func(crDict *hcov1beta1.DataImportCronTemplate) {
						crDict.Annotations = map[string]string{MultiArchDICTAnnotation: "crVal1,crVal2,crVal3"}
						crDict.Spec.Template.Spec.Source.Registry = &cdiv1beta1.DataVolumeSourceRegistry{URL: ptr.To("docker://someregistry/customized-image")}
					},
					true,
					HaveKeyWithValue(MultiArchDICTAnnotation, "crVal1,crVal2,crVal3"),
				),
				Entry("if reg was changed, and MultiArchDICTAnnotation annotation exists in CR, copy to target; when annotations are the same",
					func(targetDict *hcov1beta1.DataImportCronTemplate) {
						targetDict.Annotations = map[string]string{MultiArchDICTAnnotation: "crVal1,crVal2,crVal3"}
					},
					func(crDict *hcov1beta1.DataImportCronTemplate) {
						crDict.Annotations = map[string]string{MultiArchDICTAnnotation: "crVal1,crVal2,crVal3"}
						crDict.Spec.Template.Spec.Source.Registry = &cdiv1beta1.DataVolumeSourceRegistry{URL: ptr.To("docker://someregistry/customized-image")}
					},
					true,
					HaveKeyWithValue(MultiArchDICTAnnotation, "crVal1,crVal2,crVal3"),
				),
				Entry("if reg was changed, and MultiArchDICTAnnotation annotation exists in CR, copy to target; when no MultiArchDICTAnnotation is missing in target",
					func(targetDict *hcov1beta1.DataImportCronTemplate) {
						targetDict.Annotations = map[string]string{"testing.kubevirt.io/fake.annotation": "true"}
					},
					func(crDict *hcov1beta1.DataImportCronTemplate) {
						crDict.Annotations = map[string]string{MultiArchDICTAnnotation: "crVal1,crVal2,crVal3"}
						crDict.Spec.Template.Spec.Source.Registry = &cdiv1beta1.DataVolumeSourceRegistry{URL: ptr.To("docker://someregistry/customized-image")}
					},
					true,
					HaveKeyWithValue(MultiArchDICTAnnotation, "crVal1,crVal2,crVal3"),
				),
				Entry("if reg was changed, and MultiArchDICTAnnotation annotation exists in CR, copy to target; when no annotations in target",
					func(targetDict *hcov1beta1.DataImportCronTemplate) {
						targetDict.Annotations = nil
					},
					func(crDict *hcov1beta1.DataImportCronTemplate) {
						crDict.Annotations = map[string]string{MultiArchDICTAnnotation: "crVal1,crVal2,crVal3"}
						crDict.Spec.Template.Spec.Source.Registry = &cdiv1beta1.DataVolumeSourceRegistry{URL: ptr.To("docker://someregistry/customized-image")}
					},
					true,
					HaveKeyWithValue(MultiArchDICTAnnotation, "crVal1,crVal2,crVal3"),
				),
				Entry("if reg was changed, and MultiArchDICTAnnotation annotation exists in CR, copy to target; when annotations are empty in target",
					func(targetDict *hcov1beta1.DataImportCronTemplate) {
						targetDict.Annotations = map[string]string{}
					},
					func(crDict *hcov1beta1.DataImportCronTemplate) {
						crDict.Annotations = map[string]string{MultiArchDICTAnnotation: "crVal1,crVal2,crVal3"}
						crDict.Spec.Template.Spec.Source.Registry = &cdiv1beta1.DataVolumeSourceRegistry{URL: ptr.To("docker://someregistry/customized-image")}
					},
					true,
					HaveKeyWithValue(MultiArchDICTAnnotation, "crVal1,crVal2,crVal3"),
				),
				Entry("if reg was changed, and MultiArchDICTAnnotation annotation does not exist in CR, remove from target; when annotation exists in target",
					func(targetDict *hcov1beta1.DataImportCronTemplate) {
						targetDict.Annotations = map[string]string{MultiArchDICTAnnotation: "targetVal1,targetVal2,targetVal3"}
					},
					func(crDict *hcov1beta1.DataImportCronTemplate) {
						crDict.Annotations = map[string]string{"testing.kubevirt.io/fake.annotation": "true"}
						crDict.Spec.Template.Spec.Source.Registry = &cdiv1beta1.DataVolumeSourceRegistry{URL: ptr.To("docker://someregistry/customized-image")}
					},
					true,
					Not(HaveKey(MultiArchDICTAnnotation)),
				),
				Entry("if reg was changed, and MultiArchDICTAnnotation annotation does not exist in CR, remove from target; when annotation does not exist in target",
					func(targetDict *hcov1beta1.DataImportCronTemplate) {
						targetDict.Annotations = map[string]string{"testing.kubevirt.io/fake.annotation": "true"}
					},
					func(crDict *hcov1beta1.DataImportCronTemplate) {
						crDict.Annotations = map[string]string{"testing.kubevirt.io/fake.annotation": "true"}
						crDict.Spec.Template.Spec.Source.Registry = &cdiv1beta1.DataVolumeSourceRegistry{URL: ptr.To("docker://someregistry/customized-image")}
					},
					true,
					Not(HaveKey(MultiArchDICTAnnotation)),
				),
				Entry("if reg was changed, and MultiArchDICTAnnotation annotation does not exist in CR, remove from target; when annotations are nil in CR",
					func(targetDict *hcov1beta1.DataImportCronTemplate) {
						targetDict.Annotations = map[string]string{"testing.kubevirt.io/fake.annotation": "true"}
					},
					func(crDict *hcov1beta1.DataImportCronTemplate) {
						crDict.Annotations = nil
						crDict.Spec.Template.Spec.Source.Registry = &cdiv1beta1.DataVolumeSourceRegistry{URL: ptr.To("docker://someregistry/customized-image")}
					},
					true,
					Not(HaveKey(MultiArchDICTAnnotation)),
				),
				Entry("if reg was changed, and MultiArchDICTAnnotation annotation does not exist in CR, remove from target; when annotations are both nil",
					func(targetDict *hcov1beta1.DataImportCronTemplate) {
						targetDict.Annotations = nil
					},
					func(crDict *hcov1beta1.DataImportCronTemplate) {
						crDict.Annotations = nil
						crDict.Spec.Template.Spec.Source.Registry = &cdiv1beta1.DataVolumeSourceRegistry{URL: ptr.To("docker://someregistry/customized-image")}
					},
					true,
					Not(HaveKey(MultiArchDICTAnnotation)),
				),
			)
		})
	})
})

func enableDict(dict *hcov1beta1.DataImportCronTemplate, status *hcov1beta1.DataImportCronTemplateStatus) {
	if dict.Annotations == nil {
		dict.Annotations = make(map[string]string)
	}
	dict.Annotations[hcoutil.DataImportCronEnabledAnnotation] = "true"

	if status.Annotations == nil {
		status.Annotations = make(map[string]string)
	}
	status.Annotations[hcoutil.DataImportCronEnabledAnnotation] = "true"
}

func disableDict(dict *hcov1beta1.DataImportCronTemplate) {
	if dict.Annotations == nil {
		dict.Annotations = make(map[string]string)
	}
	dict.Annotations[hcoutil.DataImportCronEnabledAnnotation] = "false"
}

func makeDICT(num int, CommonTemplate bool) (hcov1beta1.DataImportCronTemplate, hcov1beta1.DataImportCronTemplateStatus) {
	name := fmt.Sprintf("image%d", num)

	dict := hcov1beta1.DataImportCronTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: &cdiv1beta1.DataImportCronSpec{
			Schedule: fmt.Sprintf("%d */12 * * *", num),
			Template: cdiv1beta1.DataVolume{
				Spec: cdiv1beta1.DataVolumeSpec{
					Source: &cdiv1beta1.DataVolumeSource{
						Registry: &cdiv1beta1.DataVolumeSourceRegistry{URL: ptr.To(fmt.Sprintf("docker://someregistry/%s", name))},
					},
				},
			},
			ManagedDataSource: name,
		},
	}

	return dict, hcov1beta1.DataImportCronTemplateStatus{
		DataImportCronTemplate: *dict.DeepCopy(),
		Status: hcov1beta1.DataImportCronStatus{
			CommonTemplate: CommonTemplate,
			Modified:       false,
		},
	}
}

const (
	pkgDirectory = "controllers/handlers/golden-images"
	testFilesLoc = "testFiles"
)

func getTestFilesLocation() string {
	wd, err := os.Getwd()
	Expect(err).ToNot(HaveOccurred())
	if strings.HasSuffix(wd, pkgDirectory) {
		return testFilesLoc
	}
	return path.Join(pkgDirectory, testFilesLoc)
}
