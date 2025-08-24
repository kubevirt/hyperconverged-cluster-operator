package tests_test

import (
	"context"
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "github.com/openshift/api/image/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cdiv1beta1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	sspv1beta3 "kubevirt.io/ssp-operator/api/v1beta3"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	goldenimages "github.com/kubevirt/hyperconverged-cluster-operator/controllers/handlers/golden-images"
	tests "github.com/kubevirt/hyperconverged-cluster-operator/tests/func-tests"
)

const (
	defaultImageNamespace      = "kubevirt-os-images"
	cdiImmediateBindAnnotation = "cdi.kubevirt.io/storage.bind.immediate.requested"
)

var (
	expectedImages       = []string{"centos-stream10-image-cron", "centos-stream9-image-cron", "centos-stream9-image-cron-is", "fedora-image-cron"}
	imageNamespace       = defaultImageNamespace
	expectedImageStreams = []tests.ImageStreamConfig{
		{
			Name:         "centos-stream9",
			RegistryName: "quay.io/containerdisks/centos-stream:9",
			UsageImages:  []string{"centos-stream9-image-cron-is"},
		},
	}
)

var _ = Describe("golden image test", Label("data-import-cron"), Serial, Ordered, Label(tests.OpenshiftLabel), func() {
	var (
		cli client.Client
	)

	tests.FlagParse()

	if nsFromConfig := tests.GetConfig().DataImportCron.Namespace; len(nsFromConfig) > 0 {
		imageNamespace = nsFromConfig
	}

	if imageNamespaceEnv, ok := os.LookupEnv("IMAGES_NS"); ok && len(imageNamespaceEnv) > 0 {
		imageNamespace = imageNamespaceEnv
	}

	if expectedImagesFromConfig := tests.GetConfig().DataImportCron.ExpectedDataImportCrons; len(expectedImagesFromConfig) > 0 {
		expectedImages = expectedImagesFromConfig
	}
	sort.Strings(expectedImages)

	if expectedISFromConfig := tests.GetConfig().DataImportCron.ExpectedImageStream; len(expectedISFromConfig) > 0 {
		expectedImageStreams = expectedISFromConfig
	}

	BeforeEach(func(ctx context.Context) {
		cli = tests.GetControllerRuntimeClient()

		tests.FailIfNotOpenShift(ctx, cli, "golden image test")
	})

	Context("test image-streams", func() {
		var isEntries []TableEntry
		for _, is := range expectedImageStreams {
			isEntries = append(isEntries, Entry(fmt.Sprintf("check the %s imagestream", is.Name), is))
		}

		DescribeTable("check that imagestream created", func(ctx context.Context, expectedIS tests.ImageStreamConfig) {
			is := getImageStream(ctx, cli, expectedIS.Name, imageNamespace)

			Expect(is.Spec.Tags[0].From).ToNot(BeNil())
			Expect(is.Spec.Tags[0].From.Kind).To(Equal("DockerImage"))
			Expect(is.Spec.Tags[0].From.Name).To(Equal(expectedIS.RegistryName))
		},
			isEntries,
		)

		DescribeTable("check imagestream reconciliation", func(ctx context.Context, expectedIS tests.ImageStreamConfig) {
			is := getImageStream(ctx, cli, expectedIS.Name, imageNamespace)

			expectedValue := is.GetLabels()["app.kubernetes.io/part-of"]
			Expect(expectedValue).ToNot(Equal("wrongValue"))

			patchOp := []byte(`[{"op": "replace", "path": "/metadata/labels/app.kubernetes.io~1part-of", "value": "wrong-value"}]`)
			patch := client.RawPatch(types.JSONPatchType, patchOp)

			Eventually(func(ctx context.Context) error {
				return cli.Patch(ctx, is, patch)
			}).WithTimeout(time.Second * 5).WithPolling(time.Millisecond * 100).WithContext(ctx).Should(Succeed())

			Eventually(func(g Gomega, ctx context.Context) string {
				is = getImageStream(ctx, cli, expectedIS.Name, imageNamespace)
				return is.GetLabels()["app.kubernetes.io/part-of"]
			}).WithTimeout(time.Second * 15).WithPolling(time.Millisecond * 100).WithContext(ctx).Should(Equal(expectedValue))
		},
			isEntries,
		)
	})

	It("make sure the enabler is set", func(ctx context.Context) {
		hco := tests.GetHCO(ctx, cli)
		Expect(hco.Spec.EnableCommonBootImageImport).To(HaveValue(BeTrue()))
	})

	Context("check default golden images", func() {
		It("should propagate the DICT to SSP", func(ctx context.Context) {
			Eventually(func(g Gomega, ctx context.Context) []string {
				ssp := getSSP(ctx, cli)
				g.Expect(ssp.Spec.CommonTemplates.DataImportCronTemplates).To(HaveLen(len(expectedImages)))

				imageNames := make([]string, len(expectedImages))
				for i, image := range ssp.Spec.CommonTemplates.DataImportCronTemplates {
					imageNames[i] = image.Name
				}
				sort.Strings(imageNames)
				return imageNames
			}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).WithContext(ctx).Should(Equal(expectedImages))
		})

		It("should have all the images in the HyperConverged status", func(ctx context.Context) {
			Eventually(func(g Gomega, ctx context.Context) []string {
				hco := tests.GetHCO(ctx, cli)

				g.Expect(hco.Status.DataImportCronTemplates).To(HaveLen(len(expectedImages)))

				imageNames := make([]string, len(expectedImages))
				for i, image := range hco.Status.DataImportCronTemplates {
					imageNames[i] = image.Name
				}

				sort.Strings(imageNames)
				return imageNames
			}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).WithContext(ctx).Should(Equal(expectedImages))
		})

		It("should have all the DataImportCron resources", func(ctx context.Context) {
			Eventually(func(g Gomega, ctx context.Context) []string {
				dicList := &cdiv1beta1.DataImportCronList{}
				Expect(cli.List(ctx, dicList, client.InNamespace(imageNamespace))).To(Succeed())

				g.Expect(dicList.Items).To(HaveLen(len(expectedImages)))

				imageNames := make([]string, len(expectedImages))
				for i, image := range dicList.Items {
					imageNames[i] = image.Name
				}

				sort.Strings(imageNames)
				return imageNames
			}).WithTimeout(5 * time.Minute).WithPolling(5 * time.Second).WithContext(ctx).Should(Equal(expectedImages))
		})
	})

	Context("check imagestream images", func() {
		var isUsageEntries []TableEntry
		for _, is := range expectedImageStreams {
			for _, image := range is.UsageImages {
				isUsageEntries = append(isUsageEntries, Entry(fmt.Sprintf("%s should have imageStream source", image), image, is.Name))
			}
		}

		DescribeTable("check the images that use image streams", func(ctx context.Context, imageName, streamName string) {
			dic := &cdiv1beta1.DataImportCron{
				ObjectMeta: metav1.ObjectMeta{
					Name:      imageName,
					Namespace: imageNamespace,
				},
			}

			Expect(cli.Get(ctx, client.ObjectKeyFromObject(dic), dic)).To(Succeed())

			Expect(dic.Spec.Template.Spec.Source).ToNot(BeNil())
			Expect(dic.Spec.Template.Spec.Source.Registry).ToNot(BeNil())
			Expect(dic.Spec.Template.Spec.Source.Registry.ImageStream).To(HaveValue(Equal(streamName)))
			Expect(dic.Spec.Template.Spec.Source.Registry.PullMethod).To(HaveValue(Equal(cdiv1beta1.RegistryPullNode)))
		}, isUsageEntries)
	})

	Context("disable the feature", func() {
		It("Should set the FG to false", func(ctx context.Context) {
			patch := []byte(`[{ "op": "replace", "path": "/spec/enableCommonBootImageImport", "value": false }]`)
			Eventually(func(ctx context.Context) error {
				return tests.PatchHCO(ctx, cli, patch)
			}).WithTimeout(5 * time.Second).WithPolling(100 * time.Millisecond).WithContext(ctx).Should(Succeed())
		})

		var isEntries []TableEntry
		for _, is := range expectedImageStreams {
			isEntries = append(isEntries, Entry(fmt.Sprintf("check the %s imagestream", is.Name), is))
		}

		if len(isEntries) > 0 {
			DescribeTable("imageStream should be removed", func(ctx context.Context, expectedIS tests.ImageStreamConfig) {
				Eventually(func(ctx context.Context) error {
					is := &v1.ImageStream{
						ObjectMeta: metav1.ObjectMeta{
							Name:      expectedIS.Name,
							Namespace: imageNamespace,
						},
					}

					return cli.Get(ctx, client.ObjectKeyFromObject(is), is)
				}).WithTimeout(5 * time.Second).WithPolling(100 * time.Millisecond).WithContext(ctx).Should(MatchError(errors.IsNotFound, "not found error"))
			}, isEntries)
		}

		It("should empty the DICT in SSP", func(ctx context.Context) {
			Eventually(func(g Gomega, ctx context.Context) []sspv1beta3.DataImportCronTemplate {
				ssp := getSSP(ctx, cli)
				return ssp.Spec.CommonTemplates.DataImportCronTemplates
			}).WithTimeout(5 * time.Second).WithPolling(100 * time.Millisecond).WithContext(ctx).Should(BeEmpty())
		})

		It("should have no images in the HyperConverged status", func(ctx context.Context) {
			Eventually(func(ctx context.Context) []hcov1beta1.DataImportCronTemplateStatus {
				hco := tests.GetHCO(ctx, cli)
				return hco.Status.DataImportCronTemplates
			}).WithTimeout(5 * time.Second).WithPolling(100 * time.Millisecond).WithContext(ctx).Should(BeEmpty())
		})

		It("should have no images", func(ctx context.Context) {
			Eventually(func(g Gomega, ctx context.Context) []v1.ImageStream {
				isList := &v1.ImageStreamList{}
				Expect(cli.List(ctx, isList, client.InNamespace(imageNamespace))).To(Succeed())

				return isList.Items
			}).WithTimeout(5 * time.Minute).WithPolling(time.Second).WithContext(ctx).Should(BeEmpty())
		})
	})

	Context("enable the feature again", func() {
		It("Should set the FG to false", func(ctx context.Context) {
			patch := []byte(`[{ "op": "replace", "path": "/spec/enableCommonBootImageImport", "value": true }]`)
			Eventually(func(ctx context.Context) error {
				return tests.PatchHCO(ctx, cli, patch)
			}).WithTimeout(5 * time.Second).WithPolling(100 * time.Millisecond).WithContext(ctx).Should(Succeed())
		})

		var isEntries []TableEntry
		for _, is := range expectedImageStreams {
			isEntries = append(isEntries, Entry(fmt.Sprintf("check the %s imagestream", is.Name), is))
		}

		if len(isEntries) > 0 {
			DescribeTable("imageStream should be recovered", func(ctx context.Context, expectedIS tests.ImageStreamConfig) {
				Eventually(func(g Gomega, ctx context.Context) error {
					is := v1.ImageStream{
						ObjectMeta: metav1.ObjectMeta{
							Name:      expectedIS.Name,
							Namespace: imageNamespace,
						},
					}
					return cli.Get(ctx, client.ObjectKeyFromObject(&is), &is)
				}).WithTimeout(5 * time.Second).WithPolling(100 * time.Millisecond).WithContext(ctx).ShouldNot(HaveOccurred())
			}, isEntries)
		}

		It("should propagate the DICT in SSP", func(ctx context.Context) {
			Eventually(func(g Gomega, ctx context.Context) []sspv1beta3.DataImportCronTemplate {
				ssp := getSSP(ctx, cli)
				return ssp.Spec.CommonTemplates.DataImportCronTemplates
			}).WithTimeout(5 * time.Second).WithPolling(100 * time.Millisecond).WithContext(ctx).Should(HaveLen(len(expectedImages)))
		})

		It("should have all the images in the HyperConverged status", func(ctx context.Context) {
			Eventually(func(ctx context.Context) []hcov1beta1.DataImportCronTemplateStatus {
				hco := tests.GetHCO(ctx, cli)
				return hco.Status.DataImportCronTemplates
			}).WithTimeout(5 * time.Second).WithPolling(100 * time.Millisecond).WithContext(ctx).Should(HaveLen(len(expectedImages)))
		})

		It("should restore all the DataImportCron resources", func(ctx context.Context) {
			Eventually(func(g Gomega, ctx context.Context) []cdiv1beta1.DataImportCron {
				dicList := &cdiv1beta1.DataImportCronList{}
				Expect(cli.List(ctx, dicList, client.InNamespace(imageNamespace))).To(Succeed())

				return dicList.Items
			}).WithTimeout(5 * time.Minute).WithPolling(5 * time.Second).WithContext(ctx).Should(HaveLen(len(expectedImages)))
		})
	})

	Context("test annotations", func() {

		AfterEach(func(ctx context.Context) {
			Eventually(func(g Gomega, ctx context.Context) {
				hc := tests.GetHCO(ctx, cli)

				// make sure there no user-defined DICT
				if len(hc.Spec.DataImportCronTemplates) > 0 {
					hc.APIVersion = "hco.kubevirt.io/v1beta1"
					hc.Kind = "HyperConverged"
					hc.Spec.DataImportCronTemplates = nil

					tests.UpdateHCORetry(ctx, cli, hc)
				}

			}).WithPolling(time.Second * 3).WithTimeout(time.Second * 60).WithContext(ctx).Should(Succeed())
		})

		It("should add missing annotation in the DICT", func(ctx context.Context) {
			Eventually(func(g Gomega, ctx context.Context) {
				hc := tests.GetHCO(ctx, cli)

				hc.Spec.DataImportCronTemplates = []hcov1beta1.DataImportCronTemplate{
					getDICT(),
				}

				tests.UpdateHCORetry(ctx, cli, hc)
				newHC := tests.GetHCO(ctx, cli)

				g.Expect(newHC.Spec.DataImportCronTemplates).To(HaveLen(1))
				g.Expect(newHC.Spec.DataImportCronTemplates[0].Annotations).To(HaveKeyWithValue(cdiImmediateBindAnnotation, "true"), "should add the missing annotation")
			}).WithPolling(time.Second * 3).WithTimeout(time.Second * 60).WithContext(ctx).Should(Succeed())
		})

		It("should not change existing annotation in the DICT", func(ctx context.Context) {
			Eventually(func(g Gomega, ctx context.Context) {
				hc := tests.GetHCO(ctx, cli)

				hc.Spec.DataImportCronTemplates = []hcov1beta1.DataImportCronTemplate{
					getDICT(),
				}

				hc.Spec.DataImportCronTemplates[0].Annotations = map[string]string{
					cdiImmediateBindAnnotation: "false",
				}

				tests.UpdateHCORetry(ctx, cli, hc)
				newHC := tests.GetHCO(ctx, cli)

				g.Expect(newHC.Spec.DataImportCronTemplates).To(HaveLen(1))
				g.Expect(newHC.Spec.DataImportCronTemplates[0].Annotations).To(HaveKeyWithValue(cdiImmediateBindAnnotation, "false"), "should not change existing annotation")
			}).WithPolling(time.Second * 3).WithTimeout(time.Second * 60).WithContext(ctx).Should(Succeed())
		})
	})

	Context("test multi-arch", Label("multi-arch"), func() {
		var (
			archs  []string
			origHC *hcov1beta1.HyperConverged
		)

		BeforeEach(func(ctx context.Context) {
			var err error
			archs, err = getArchs(ctx)
			Expect(err).ToNot(HaveOccurred(), "failed to get worker nodes architectures")

			GinkgoWriter.Printf("Worker nodes architectures: %v\n", archs)

			const patchTmplt = `[{ "op": "replace", "path": "/spec/featureGates/enableMultiArchBootImageImport", "value": %t }]`
			Eventually(func(ctx context.Context) error {
				return tests.PatchHCO(ctx, cli, []byte(fmt.Sprintf(patchTmplt, true)))
			}).WithTimeout(10 * time.Second).
				WithPolling(500 * time.Millisecond).
				WithContext(ctx).
				Should(Succeed())

			removeCustomDICTFromHC(ctx, cli)

			Eventually(func(ctx context.Context) error {
				origHC = tests.GetHCO(ctx, cli)
				for _, dictStatus := range origHC.Status.DataImportCronTemplates {
					if dictStatus.Status.OriginalSupportedArchitectures == "" {
						return fmt.Errorf("the OriginalSupportedArchitectures is not set yet in the DICT %q", dictStatus.Name)
					}
				}

				return nil
			}).WithTimeout(20*time.Second).
				WithPolling(500*time.Millisecond).
				WithContext(ctx).
				Should(Succeed(), tests.PrintHyperConvergedBecause(origHC, "the dictStatus.Status.OriginalSupportedArchitectures field should not be empty"))

			DeferCleanup(func(ctx context.Context) {
				Eventually(func(ctx context.Context) error {
					return tests.PatchHCO(ctx, cli, []byte(fmt.Sprintf(patchTmplt, false)))
				}).WithTimeout(10 * time.Second).
					WithPolling(500 * time.Millisecond).
					WithContext(ctx).
					Should(Succeed())

				removeCustomDICTFromHC(ctx, cli)
			})
		})

		It("should have the architectures in the HCO Status", func(ctx context.Context) {
			hc := tests.GetHCO(ctx, cli)
			Expect(hc.Status.NodeInfo.WorkloadsArchitectures).To(Equal(archs))
		})

		It("should have the architectures in the SSP CR", func(ctx context.Context) {
			var hc *hcov1beta1.HyperConverged
			Eventually(func(g Gomega, ctx context.Context) {
				ssp := getSSP(ctx, cli)
				g.Expect(ssp.Spec.CommonTemplates.DataImportCronTemplates).To(HaveLen(len(expectedImages)))

				hc = tests.GetHCO(ctx, cli)

				for _, dict := range ssp.Spec.CommonTemplates.DataImportCronTemplates {
					hcoDict, exists := getHCODICT(hc, dict.Name)
					g.Expect(exists).To(BeTrue(), "should have the DICT in the HCO status")

					if _, hcoAnnotationExists := hcoDict.Annotations[goldenimages.MultiArchDICTAnnotation]; !hcoAnnotationExists {
						GinkgoLogr.Info(fmt.Sprintf("The %q DICT does not have the multi-arch annotation in the HCO status; skipping it", dict.Name))
						continue
					}

					multiArchAnnotation, exists := dict.GetAnnotations()[goldenimages.MultiArchDICTAnnotation]

					g.Expect(exists).To(BeTrue(), "should have the multi-arch annotation in the DICT")
					g.Expect(multiArchAnnotation).ToNot(Equal(""), "should have a value in the the multi-arch annotation; the %q DICT is:\n%#v", dict.Name, dict)

					expectedArches := getExpectedArchs(hcoDict.Status.OriginalSupportedArchitectures, archs)

					g.Expect(multiArchAnnotation).To(Equal(expectedArches), "the SSP %q DICT %q annotation should be %q", dict.Name, goldenimages.MultiArchDICTAnnotation, expectedArches)
				}

			}).WithTimeout(10*time.Second).
				WithPolling(500*time.Millisecond).
				WithContext(ctx).
				Should(Succeed(), tests.PrintHyperConverged(hc))
		})

		It("should have the architectures in a user-defined DICT the SSP CR", func(ctx context.Context) {
			var (
				hc           *hcov1beta1.HyperConverged
				hcCustomDict hcov1beta1.DataImportCronTemplate
			)

			By("adding a user define DICT to the HyperConverged CR, with some supported and some unsupported architectures")
			Eventually(func(g Gomega, ctx context.Context) {
				hc = tests.GetHCO(ctx, cli)

				g.Expect(hc.Status.DataImportCronTemplates).ToNot(BeEmpty())
				hc.Status.DataImportCronTemplates[0].DataImportCronTemplate.DeepCopyInto(&hcCustomDict)

				hcCustomDict.Name = "custom-dict"
				hcCustomDict.Spec.ManagedDataSource = "custom-source"
				if hcCustomDict.Annotations == nil {
					hcCustomDict.Annotations = make(map[string]string)
				}

				customDictArchs := append(archs, "someOtherArch1", "someOtherArch2")
				hcCustomDict.Annotations[goldenimages.MultiArchDICTAnnotation] = strings.Join(customDictArchs, ",")

				hc.Spec.DataImportCronTemplates = []hcov1beta1.DataImportCronTemplate{hcCustomDict}

				var err error
				_, err = tests.UpdateHCO(ctx, cli, hc)
				g.Expect(err).ToNot(HaveOccurred(), "failed to update HCO with custom DICT")

			}).WithTimeout(10*time.Second).
				WithPolling(500*time.Millisecond).
				WithContext(ctx).
				Should(Succeed(), tests.PrintHyperConverged(hc))

			By("Check that only the cluster support architectures are in the multi-arch annotation in SSP")
			Eventually(func(g Gomega, ctx context.Context) {
				ssp := getSSP(ctx, cli)
				g.Expect(ssp.Spec.CommonTemplates.DataImportCronTemplates).To(HaveLen(len(expectedImages) + 1))

				idx := slices.IndexFunc(ssp.Spec.CommonTemplates.DataImportCronTemplates, func(d sspv1beta3.DataImportCronTemplate) bool {
					return d.Name == "custom-dict"
				})

				g.Expect(idx).To(BeNumerically(">", -1), "should have the custom-dict in the SSP")
				sspDict := ssp.Spec.CommonTemplates.DataImportCronTemplates[idx]

				multiArchAnnotation, exists := sspDict.GetAnnotations()[goldenimages.MultiArchDICTAnnotation]

				g.Expect(exists).To(BeTrue(), "should have the multi-arch annotation in the DICT")
				g.Expect(multiArchAnnotation).ToNot(BeEmpty(), "should have a value in the the multi-arch annotation")

				expectedArches := getExpectedArchs(hcCustomDict.Annotations[goldenimages.MultiArchDICTAnnotation], archs)

				g.Expect(multiArchAnnotation).To(Equal(expectedArches), "the SSP %q DICT %q annotation should be %q", "custom-dict", goldenimages.MultiArchDICTAnnotation, expectedArches)

			}).WithTimeout(10*time.Second).
				WithPolling(500*time.Millisecond).
				WithContext(ctx).
				Should(Succeed(), tests.PrintHyperConverged(hc))
		})

		It("should have the architectures in a customized common DICT the SSP CR", func(ctx context.Context) {
			var (
				hcCustomDict                   hcov1beta1.DataImportCronTemplate
				originalSupportedArchitectures string
				expectedArches                 string
				hc                             *hcov1beta1.HyperConverged
			)

			By("modify a common DICT in the HyperConverged CR, with some supported and some unsupported architectures")
			Eventually(func(g Gomega, ctx context.Context) {
				hc = tests.GetHCO(ctx, cli)

				g.Expect(hc.Status.DataImportCronTemplates).ToNot(BeEmpty())
				hc.Status.DataImportCronTemplates[0].DataImportCronTemplate.DeepCopyInto(&hcCustomDict)
				originalSupportedArchitectures = hc.Status.DataImportCronTemplates[0].Status.OriginalSupportedArchitectures

				if hcCustomDict.Annotations == nil {
					hcCustomDict.Annotations = make(map[string]string)
				}

				customDictArchs := append(archs, "someOtherArch1", "someOtherArch2")
				testAnnotation := strings.Join(customDictArchs, ",")
				hcCustomDict.Annotations[goldenimages.MultiArchDICTAnnotation] = testAnnotation
				expectedArches = getExpectedArchs(testAnnotation, archs)

				hc.Spec.DataImportCronTemplates = []hcov1beta1.DataImportCronTemplate{hcCustomDict}

				var err error
				_, err = tests.UpdateHCO(ctx, cli, hc)
				g.Expect(err).ToNot(HaveOccurred(), "failed to update HCO with custom DICT")

			}).WithTimeout(10 * time.Second).
				WithPolling(500 * time.Millisecond).
				WithContext(ctx).
				Should(Succeed())

			By("Check that only the cluster support architectures are in the multi-arch annotation in SSP")
			Eventually(func(g Gomega, ctx context.Context) {
				ssp := getSSP(ctx, cli)
				g.Expect(ssp.Spec.CommonTemplates.DataImportCronTemplates).To(HaveLen(len(expectedImages)))

				idx := slices.IndexFunc(ssp.Spec.CommonTemplates.DataImportCronTemplates, func(d sspv1beta3.DataImportCronTemplate) bool {
					return d.Name == hcCustomDict.Name
				})

				g.Expect(idx).To(BeNumerically(">", -1), "should have the %q in the SSP", hcCustomDict.Name)
				sspDict := ssp.Spec.CommonTemplates.DataImportCronTemplates[idx]

				multiArchAnnotation, exists := sspDict.GetAnnotations()[goldenimages.MultiArchDICTAnnotation]

				g.Expect(exists).To(BeTrue(), "should have the multi-arch annotation in the DICT")
				g.Expect(multiArchAnnotation).ToNot(BeEmpty(), "should have a value in the the multi-arch annotation")

				g.Expect(multiArchAnnotation).To(Equal(expectedArches), "the SSP %q DICT %q annotation should be %q", hcCustomDict.Name, goldenimages.MultiArchDICTAnnotation, expectedArches)
			}).WithTimeout(10*time.Second).
				WithPolling(500*time.Millisecond).
				WithContext(ctx).
				Should(Succeed(), tests.PrintHyperConverged(hc))

			By("Check DICT in HCO status")
			Eventually(func(g Gomega, ctx context.Context) {
				hc = tests.GetHCO(ctx, cli)
				idx := slices.IndexFunc(hc.Status.DataImportCronTemplates, func(d hcov1beta1.DataImportCronTemplateStatus) bool {
					return d.Name == hcCustomDict.Name
				})
				g.Expect(idx).To(BeNumerically(">", -1), "should have the %q in the HC status", hcCustomDict.Name)

				hcoDictStatus := hc.Status.DataImportCronTemplates[idx]
				g.Expect(hcoDictStatus.Annotations).To(HaveKeyWithValue(goldenimages.MultiArchDICTAnnotation, expectedArches))
				g.Expect(hcoDictStatus.Status.OriginalSupportedArchitectures).To(Equal(originalSupportedArchitectures))
				g.Expect(hcoDictStatus.Status.Conditions).To(BeEmpty())

			}).WithTimeout(60*time.Second).
				WithPolling(time.Second).
				WithContext(ctx).
				Should(Succeed(), tests.PrintOrigAndCurrentHyperConvergeds(origHC, hc))
		})

		When("the multi-arch annotation is not set in the DICT", func() {
			It("should not implement multi-arch changes for user defined DICT", func(ctx context.Context) {
				var (
					hcCustomDict hcov1beta1.DataImportCronTemplate
					hc           *hcov1beta1.HyperConverged
				)

				By("Add a user-defined DICT to the HyperConverged CR, without the multi-arch annotation")
				Eventually(func(g Gomega, ctx context.Context) {
					hc = tests.GetHCO(ctx, cli)

					g.Expect(hc.Status.DataImportCronTemplates).ToNot(BeEmpty())
					hc.Status.DataImportCronTemplates[0].DataImportCronTemplate.DeepCopyInto(&hcCustomDict)

					hcCustomDict.Name = "custom-dict"
					hcCustomDict.Spec.ManagedDataSource = "custom-source"
					delete(hcCustomDict.Annotations, goldenimages.MultiArchDICTAnnotation)

					hc.Spec.DataImportCronTemplates = []hcov1beta1.DataImportCronTemplate{hcCustomDict}

					var err error
					_, err = tests.UpdateHCO(ctx, cli, hc)
					g.Expect(err).ToNot(HaveOccurred(), "failed to update HCO with custom DICT")

				}).WithTimeout(10*time.Second).
					WithPolling(500*time.Millisecond).
					WithContext(ctx).
					Should(Succeed(), tests.PrintHyperConverged(hc))

				By("Check that the custom DICT is in the SSP, without the multi-arch annotation, and that the DICT in the HC status also does not have the annotation")
				Eventually(func(g Gomega, ctx context.Context) {
					ssp := getSSP(ctx, cli)
					g.Expect(ssp.Spec.CommonTemplates.DataImportCronTemplates).To(HaveLen(len(expectedImages) + 1))

					idx := slices.IndexFunc(ssp.Spec.CommonTemplates.DataImportCronTemplates, func(d sspv1beta3.DataImportCronTemplate) bool {
						return d.Name == "custom-dict"
					})

					g.Expect(idx).To(BeNumerically(">", -1), "should have the custom-dict DICT in the SSP")
					sspDict := ssp.Spec.CommonTemplates.DataImportCronTemplates[idx]

					g.Expect(sspDict.GetAnnotations()).ToNot(HaveKey(goldenimages.MultiArchDICTAnnotation))

					hc = tests.GetHCO(ctx, cli)
					idx = slices.IndexFunc(hc.Status.DataImportCronTemplates, func(d hcov1beta1.DataImportCronTemplateStatus) bool {
						return d.Name == "custom-dict"
					})
					g.Expect(idx).To(BeNumerically(">", -1), "should have the custom-dict in the HC status")

					hcoDictStatus := hc.Status.DataImportCronTemplates[idx]
					g.Expect(hcoDictStatus.Annotations).ToNot(HaveKey(goldenimages.MultiArchDICTAnnotation))
					g.Expect(hcoDictStatus.Status.OriginalSupportedArchitectures).To(Equal(""))
					g.Expect(hcoDictStatus.Status.Conditions).To(BeEmpty())

				}).WithTimeout(10*time.Second).
					WithPolling(500*time.Millisecond).
					WithContext(ctx).
					Should(Succeed(), tests.PrintOrigAndCurrentHyperConvergeds(origHC, hc))
			})
		})

		When("there are no supported architectures", func() {
			It("should not add a user-defined DICT to the SSP CR", func(ctx context.Context) {
				var (
					hcCustomDict hcov1beta1.DataImportCronTemplate
					hc           *hcov1beta1.HyperConverged
				)

				By("Add a user-defined DICT to the HyperConverged CR, with no supported architectures")
				Eventually(func(g Gomega, ctx context.Context) {
					hc = tests.GetHCO(ctx, cli)

					g.Expect(hc.Status.DataImportCronTemplates).ToNot(BeEmpty())
					hc.Status.DataImportCronTemplates[0].DataImportCronTemplate.DeepCopyInto(&hcCustomDict)

					hcCustomDict.Name = "custom-dict"
					hcCustomDict.Spec.ManagedDataSource = "custom-source"
					if hcCustomDict.Annotations == nil {
						hcCustomDict.Annotations = make(map[string]string)
					}

					hcCustomDict.Annotations[goldenimages.MultiArchDICTAnnotation] = "someOtherArch1,someOtherArch2"

					hc.Spec.DataImportCronTemplates = []hcov1beta1.DataImportCronTemplate{hcCustomDict}

					var err error
					_, err = tests.UpdateHCO(ctx, cli, hc)
					g.Expect(err).ToNot(HaveOccurred(), "failed to update HCO with custom DICT")

				}).WithTimeout(10 * time.Second).
					WithPolling(500 * time.Millisecond).
					WithContext(ctx).
					Should(Succeed())

				By("Check that the custom DICT is not in the SSP, and its corresponding object in the HC status has the Deploy=False condition")
				Consistently(func(g Gomega, ctx context.Context) {
					ssp := getSSP(ctx, cli)
					g.Expect(ssp.Spec.CommonTemplates.DataImportCronTemplates).To(HaveLen(len(expectedImages)))

					idx := slices.IndexFunc(ssp.Spec.CommonTemplates.DataImportCronTemplates, func(d sspv1beta3.DataImportCronTemplate) bool {
						return d.Name == "custom-dict"
					})
					g.Expect(idx).To(Equal(-1), "should not have the custom-dict in the SSP")
				}).Within(60*time.Second).
					WithPolling(time.Second).
					WithContext(ctx).
					Should(Succeed(), tests.PrintHyperConverged(hc))

				By("Check the DICT in the HC status")
				Eventually(func(g Gomega, ctx context.Context) {
					hc = tests.GetHCO(ctx, cli)
					idx := slices.IndexFunc(hc.Status.DataImportCronTemplates, func(d hcov1beta1.DataImportCronTemplateStatus) bool {
						return d.Name == "custom-dict"
					})
					g.Expect(idx).To(BeNumerically(">", -1), "should have the custom-dict in the HC status")

					hcoDictStatus := hc.Status.DataImportCronTemplates[idx]
					g.Expect(hcoDictStatus.Annotations).To(HaveKeyWithValue(goldenimages.MultiArchDICTAnnotation, ""))
					g.Expect(hcoDictStatus.Status.OriginalSupportedArchitectures).To(Equal("someOtherArch1,someOtherArch2"))

					g.Expect(hcoDictStatus.Status.Conditions).To(HaveLen(1), "should have one condition in the DICT status")
					g.Expect(hcoDictStatus.Status.Conditions[0].Type).To(Equal("Deployed"))
					g.Expect(hcoDictStatus.Status.Conditions[0].Reason).To(Equal("UnsupportedArchitectures"))
				}).WithTimeout(60*time.Second).
					WithPolling(time.Second).
					WithContext(ctx).
					Should(Succeed(), tests.PrintOrigAndCurrentHyperConvergeds(origHC, hc))
			})

			It("when the image was changed, should not add a customized common DICT to the SSP CR", func(ctx context.Context) {
				var (
					hcCustomDict hcov1beta1.DataImportCronTemplate
					hc           *hcov1beta1.HyperConverged
				)

				By("modify a common DICT in the HyperConverged CR, to have no supported architectures")
				Eventually(func(g Gomega, ctx context.Context) {
					hc = tests.GetHCO(ctx, cli)

					g.Expect(len(hc.Status.DataImportCronTemplates)).To(BeNumerically(">", 1))
					hc.Status.DataImportCronTemplates[0].DataImportCronTemplate.DeepCopyInto(&hcCustomDict)

					if hcCustomDict.Annotations == nil {
						hcCustomDict.Annotations = make(map[string]string)
					}

					// modify the image source. use the next image in the list. Now the image source is not the same
					// and HCO should use the customized annotation
					nextDICT := hc.Status.DataImportCronTemplates[1].DataImportCronTemplate
					hcCustomDict.Spec.Template.Spec.Source.Registry = nextDICT.Spec.Template.Spec.Source.Registry.DeepCopy()

					hcCustomDict.Annotations[goldenimages.MultiArchDICTAnnotation] = "someOtherArch1,someOtherArch2"

					hc.Spec.DataImportCronTemplates = []hcov1beta1.DataImportCronTemplate{hcCustomDict}

					var err error
					_, err = tests.UpdateHCO(ctx, cli, hc)
					g.Expect(err).ToNot(HaveOccurred(), "failed to update HCO with custom DICT")

				}).WithTimeout(10 * time.Second).
					WithPolling(500 * time.Millisecond).
					WithContext(ctx).
					Should(Succeed())

				By("Check that the modified DICT is not in the SSP, and its corresponding object in the HC status has the Deploy=False condition")
				Eventually(func(g Gomega, ctx context.Context) {
					ssp := getSSP(ctx, cli)
					g.Expect(ssp.Spec.CommonTemplates.DataImportCronTemplates).To(HaveLen(len(expectedImages) - 1))

					idx := slices.IndexFunc(ssp.Spec.CommonTemplates.DataImportCronTemplates, func(d sspv1beta3.DataImportCronTemplate) bool {
						return d.Name == hcCustomDict.Name
					})

					g.Expect(idx).To(Equal(-1), "should not have the custom-dict in the SSP")
				}).WithTimeout(60*time.Second).
					WithPolling(time.Second).
					WithContext(ctx).
					Should(Succeed(), tests.PrintHyperConverged(hc))

				By("Check the DICT in the HC status")
				Eventually(func(g Gomega, ctx context.Context) {
					hc = tests.GetHCO(ctx, cli)
					idx := slices.IndexFunc(hc.Status.DataImportCronTemplates, func(d hcov1beta1.DataImportCronTemplateStatus) bool {
						return d.Name == hcCustomDict.Name
					})
					g.Expect(idx).To(BeNumerically(">", -1), "should have the %q in the HC status", hcCustomDict.Name)

					hcoDictStatus := hc.Status.DataImportCronTemplates[idx]
					g.Expect(hcoDictStatus.Annotations).To(HaveKeyWithValue(goldenimages.MultiArchDICTAnnotation, ""))
					g.Expect(hcoDictStatus.Status.OriginalSupportedArchitectures).To(Equal("someOtherArch1,someOtherArch2"))

					g.Expect(hcoDictStatus.Status.Conditions).To(HaveLen(1), "should have one condition in the DICT status")
					g.Expect(hcoDictStatus.Status.Conditions[0].Type).To(Equal("Deployed"))
					g.Expect(hcoDictStatus.Status.Conditions[0].Reason).To(Equal("UnsupportedArchitectures"))
				}).WithTimeout(10*time.Second).
					WithPolling(500*time.Millisecond).
					WithContext(ctx).
					Should(Succeed(), tests.PrintOrigAndCurrentHyperConvergeds(origHC, hc))
			})

			It("when the image not changed, should add a customized common DICT to the SSP CR", func(ctx context.Context) {
				var (
					hcCustomDict                   hcov1beta1.DataImportCronTemplate
					originalSupportedArchitectures string
					hc                             *hcov1beta1.HyperConverged
				)

				By("modify a common DICT in the HyperConverged CR, to have no supported architectures")
				Eventually(func(g Gomega, ctx context.Context) {
					hc = tests.GetHCO(ctx, cli)

					g.Expect(len(hc.Status.DataImportCronTemplates)).To(BeNumerically(">", 1))
					hc.Status.DataImportCronTemplates[0].DataImportCronTemplate.DeepCopyInto(&hcCustomDict)

					if hcCustomDict.Annotations == nil {
						hcCustomDict.Annotations = make(map[string]string)
					}

					originalSupportedArchitectures = hc.Status.DataImportCronTemplates[0].Status.OriginalSupportedArchitectures
					hcCustomDict.Annotations[goldenimages.MultiArchDICTAnnotation] = "someOtherArch1,someOtherArch2"

					hc.Spec.DataImportCronTemplates = []hcov1beta1.DataImportCronTemplate{hcCustomDict}

					var err error
					_, err = tests.UpdateHCO(ctx, cli, hc)
					g.Expect(err).ToNot(HaveOccurred(), "failed to update HCO with custom DICT")

				}).WithTimeout(10 * time.Second).
					WithPolling(500 * time.Millisecond).
					WithContext(ctx).
					Should(Succeed())

				By("Check that the modified DICT is in the SSP")
				Eventually(func(g Gomega, ctx context.Context) {
					ssp := getSSP(ctx, cli)
					g.Expect(ssp.Spec.CommonTemplates.DataImportCronTemplates).To(HaveLen(len(expectedImages)))

					idx := slices.IndexFunc(ssp.Spec.CommonTemplates.DataImportCronTemplates, func(d sspv1beta3.DataImportCronTemplate) bool {
						return d.Name == hcCustomDict.Name
					})

					g.Expect(idx).To(BeNumerically(">", -1), "should have the custom-dict in the SSP")
				}).WithTimeout(60*time.Second).
					WithPolling(time.Second).
					WithContext(ctx).
					Should(Succeed(), tests.PrintHyperConverged(hc))

				By("Check the DICT in the HC status")
				Eventually(func(g Gomega, ctx context.Context) {
					hc := tests.GetHCO(ctx, cli)
					idx := slices.IndexFunc(hc.Status.DataImportCronTemplates, func(d hcov1beta1.DataImportCronTemplateStatus) bool {
						return d.Name == hcCustomDict.Name
					})
					g.Expect(idx).To(BeNumerically(">", -1), "should have the %q in the HC status", hcCustomDict.Name)

					By("check that we used the original annotation, not the custom one")
					hcoDictStatus := hc.Status.DataImportCronTemplates[idx]
					expectedAnnotation := getExpectedArchs(originalSupportedArchitectures, archs)
					g.Expect(hcoDictStatus.Annotations).To(HaveKeyWithValue(goldenimages.MultiArchDICTAnnotation, expectedAnnotation),
						"DICT name: %q; node architectures: %q; originalSupportedArchitectures: %q; expected annotation: %q",
						hcCustomDict.Name, archs, originalSupportedArchitectures, expectedAnnotation)
					g.Expect(hcoDictStatus.Status.OriginalSupportedArchitectures).To(Equal(originalSupportedArchitectures))

					g.Expect(hcoDictStatus.Status.Conditions).To(BeEmpty(), "should have no conditions in the DICT status")
				}).WithTimeout(60*time.Second).
					WithPolling(time.Second).
					WithContext(ctx).
					Should(Succeed(), tests.PrintOrigAndCurrentHyperConvergeds(origHC, hc))
			})
		})
	})
})

func removeCustomDICTFromHC(ctx context.Context, cli client.Client) {
	GinkgoHelper()
	// clear the DICTs if they exist. ignore error of not found, as it may not exist
	Eventually(func(ctx context.Context) error {
		return tests.PatchHCO(ctx, cli, []byte(`[{"op": "remove", "path": "/spec/dataImportCronTemplates"}]`))
	}).WithTimeout(10 * time.Second).WithPolling(500 * time.Millisecond).WithContext(ctx).
		Should(Or(Not(HaveOccurred()), MatchError(ContainSubstring("the server rejected our request due to an error in our request"))))

	Eventually(func(g Gomega, ctx context.Context) {
		hc := tests.GetHCO(ctx, cli)
		g.Expect(hc.Spec.DataImportCronTemplates).To(BeEmpty(), "should have no DataImportCronTemplates in the HyperConverged CR")
		g.Expect(hc.Status.DataImportCronTemplates).To(HaveLen(len(expectedImages)))

		for _, dictStatus := range hc.Status.DataImportCronTemplates {
			g.Expect(dictStatus.Status.CommonTemplate).To(BeTrueBecause("should only have common DICT, but found non-common DICT %q", dictStatus.Name))
			g.Expect(dictStatus.Status.Modified).To(BeFalseBecause("All DICTs should not be modified, but found modified DICT %q", dictStatus.Name))
		}
	}).WithTimeout(60 * time.Second).
		WithPolling(time.Second).WithContext(ctx).
		Should(Succeed())
}

func getExpectedArchs(originalArchs string, archs []string) string {
	imageSupportedArchs := strings.Split(originalArchs, ",")
	var expectedArchs []string

	for _, arch := range imageSupportedArchs {
		if slices.Contains(archs, arch) {
			expectedArchs = append(expectedArchs, arch)
		}
	}

	slices.Sort(expectedArchs)
	return strings.Join(expectedArchs, ",")
}

func getHCODICT(hc *hcov1beta1.HyperConverged, name string) (hcov1beta1.DataImportCronTemplateStatus, bool) {
	for _, dict := range hc.Status.DataImportCronTemplates {
		if dict.Name == name {
			return dict, true
		}
	}

	return hcov1beta1.DataImportCronTemplateStatus{}, false
}

func getArchs(ctx context.Context) ([]string, error) {
	cli := tests.GetK8sClientSet()
	nodes, err := cli.CoreV1().Nodes().List(ctx, metav1.ListOptions{LabelSelector: "node-role.kubernetes.io/worker"})
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	archSet := sets.New[string]()
	for _, node := range nodes.Items {
		if arch := node.Status.NodeInfo.Architecture; len(arch) > 0 {
			archSet.Insert(arch)
		}
	}

	a := archSet.UnsortedList()
	slices.Sort(a)

	return a, nil
}

func getDICT() hcov1beta1.DataImportCronTemplate {
	return hcov1beta1.DataImportCronTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name: "custom",
		},
		Spec: &cdiv1beta1.DataImportCronSpec{
			RetentionPolicy:   ptr.To(cdiv1beta1.DataImportCronRetainNone),
			GarbageCollect:    ptr.To(cdiv1beta1.DataImportCronGarbageCollectOutdated),
			ManagedDataSource: "centos10",
			Schedule:          "18 1/12 * * *",
			Template: cdiv1beta1.DataVolume{
				Spec: cdiv1beta1.DataVolumeSpec{
					Source: &cdiv1beta1.DataVolumeSource{
						Registry: &cdiv1beta1.DataVolumeSourceRegistry{
							PullMethod: ptr.To(cdiv1beta1.RegistryPullNode),
							URL:        ptr.To("docker://quay.io/containerdisks/centos-stream:10"),
						},
					},
					Storage: &cdiv1beta1.StorageSpec{
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								"storage": resource.MustParse("30Gi"),
							},
						},
					},
				},
			},
		},
	}
}

func getSSP(ctx context.Context, cli client.Client) *sspv1beta3.SSP {
	ssp := &sspv1beta3.SSP{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ssp-kubevirt-hyperconverged",
			Namespace: tests.InstallNamespace,
		},
	}

	Expect(cli.Get(ctx, client.ObjectKeyFromObject(ssp), ssp)).To(Succeed())
	return ssp
}

func getImageStream(ctx context.Context, cli client.Client, name, namespace string) *v1.ImageStream {
	is := &v1.ImageStream{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}

	Expect(cli.Get(ctx, client.ObjectKeyFromObject(is), is)).To(Succeed())

	return is
}
