package tests_test

import (
	"context"
	"os"
	"sort"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "github.com/openshift/api/image/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	"kubevirt.io/client-go/kubecli"
	cdiv1beta1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/kubevirt/tests/flags"
	"kubevirt.io/ssp-operator/api/v1beta2"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	tests "github.com/kubevirt/hyperconverged-cluster-operator/tests/func-tests"
)

const (
	defaultImageNamespace      = "kubevirt-os-images"
	cdiImmediateBindAnnotation = "cdi.kubevirt.io/storage.bind.immediate.requested"
)

var (
	dicGVR = schema.GroupVersionResource{
		Group:    "cdi.kubevirt.io",
		Version:  "v1beta1",
		Resource: "dataimportcrons",
	}

	isGVR = schema.GroupVersionResource{
		Group:    "image.openshift.io",
		Version:  "v1",
		Resource: "imagestreams",
	}

	sspGVR = schema.GroupVersionResource{
		Group:    "ssp.kubevirt.io",
		Version:  "v1beta2",
		Resource: "ssps",
	}

	expectedImages = []string{"centos-7-image-cron", "centos-stream8-image-cron", "centos-stream9-image-cron", "centos8-image-cron-is", "fedora-image-cron"}
	imageNamespace = defaultImageNamespace
)

var _ = Describe("golden image test", Label("data-import-cron"), Serial, Ordered, func() {
	var (
		cli kubecli.KubevirtClient
		ctx context.Context
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

	BeforeEach(func() {
		var err error
		cli, err = kubecli.GetKubevirtClient()
		Expect(err).ToNot(HaveOccurred())

		tests.SkipIfNotOpenShift(cli, "golden image test")

		ctx = context.Background()
	})

	Context("test image-streams", func() {
		DescribeTable("check that imagestream created", func(isName string) {
			unstructured, err := cli.DynamicClient().Resource(isGVR).Namespace(imageNamespace).Get(ctx, isName, metav1.GetOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			is := &v1.ImageStream{}
			Expect(runtime.DefaultUnstructuredConverter.FromUnstructured(unstructured.Object, is)).To(Succeed())

			Expect(is.Spec.Tags[0].From).ShouldNot(BeNil())
			Expect(is.Spec.Tags[0].From.Kind).Should(Equal("DockerImage"))
			Expect(is.Spec.Tags[0].From.Name).Should(Equal("quay.io/kubevirt/centos8-container-disk-images"))
		},
			Entry("check the centos8 imagestream", "centos8"),
		)

		DescribeTable("check imagestream reconciliation", func(isName string) {
			patchOp := []byte(`[{"op": "add", "path": "/metadata/labels/test-label", "value": "test"}]`)
			Eventually(func() error {
				_, err := cli.DynamicClient().Resource(isGVR).Namespace(imageNamespace).Patch(ctx, isName, types.JSONPatchType, patchOp, metav1.PatchOptions{})
				return err
			}).WithTimeout(time.Second * 5).WithPolling(time.Millisecond * 100).Should(Succeed())

			is := &v1.ImageStream{}
			Eventually(func(g Gomega) map[string]string {
				unstructured, err := cli.DynamicClient().Resource(isGVR).Namespace(imageNamespace).Get(ctx, isName, metav1.GetOptions{})
				g.Expect(err).ShouldNot(HaveOccurred())
				g.Expect(runtime.DefaultUnstructuredConverter.FromUnstructured(unstructured.Object, is)).To(Succeed())

				return is.GetLabels()
			}).WithTimeout(time.Second * 15).WithPolling(time.Millisecond * 100).ShouldNot(HaveKey("test-label"))
		},
			Entry("check the centos8 imagestream", "centos8"),
		)
	})

	It("make sure the feature gate is set", func() {
		hco := tests.GetHCO(ctx, cli)
		Expect(hco.Spec.FeatureGates.EnableCommonBootImageImport).To(HaveValue(BeTrue()))
	})

	Context("check default golden images", func() {
		It("should propagate the DICT to SSP", func() {
			Eventually(func(g Gomega) []string {
				unstructured, err := cli.DynamicClient().Resource(sspGVR).Namespace(flags.KubeVirtInstallNamespace).Get(ctx, "ssp-kubevirt-hyperconverged", metav1.GetOptions{})
				Expect(err).ShouldNot(HaveOccurred())

				ssp := &v1beta2.SSP{}
				Expect(runtime.DefaultUnstructuredConverter.FromUnstructured(unstructured.Object, ssp)).To(Succeed())

				Expect(ssp.Spec.CommonTemplates.DataImportCronTemplates).Should(HaveLen(len(expectedImages)))

				imageNames := make([]string, len(expectedImages))
				for i, image := range ssp.Spec.CommonTemplates.DataImportCronTemplates {
					imageNames[i] = image.Name
				}
				sort.Strings(imageNames)
				return imageNames
			}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(Equal(expectedImages))
		})

		It("should have all the images in the HyperConverged status", func() {
			Eventually(func(g Gomega) []string {
				hco := tests.GetHCO(ctx, cli)

				Expect(hco.Status.DataImportCronTemplates).Should(HaveLen(len(expectedImages)))

				imageNames := make([]string, len(expectedImages))
				for i, image := range hco.Status.DataImportCronTemplates {
					imageNames[i] = image.Name
				}

				sort.Strings(imageNames)
				return imageNames
			}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(Equal(expectedImages))
		})

		It("should have all the DataImportCron resources", func() {
			Eventually(func(g Gomega) []string {
				unstructured, err := cli.DynamicClient().Resource(dicGVR).Namespace(imageNamespace).List(ctx, metav1.ListOptions{})
				g.Expect(err).ShouldNot(HaveOccurred())

				items := make([]cdiv1beta1.DataImportCron, len(unstructured.Items))
				for i, item := range unstructured.Items {
					dic := cdiv1beta1.DataImportCron{}
					g.Expect(runtime.DefaultUnstructuredConverter.FromUnstructured(item.Object, &dic)).To(Succeed())
					items[i] = dic
				}
				g.Expect(items).Should(HaveLen(len(expectedImages)))

				imageNames := make([]string, len(expectedImages))
				for i, image := range items {
					imageNames[i] = image.Name
				}

				sort.Strings(imageNames)
				return imageNames
			}).WithTimeout(5 * time.Minute).WithPolling(5 * time.Second).Should(Equal(expectedImages))
		})
	})

	It("centos8-image-cron-is should have imageStream source", func() {
		dic := &cdiv1beta1.DataImportCron{}
		unstructured, err := cli.DynamicClient().Resource(dicGVR).Namespace(imageNamespace).Get(ctx, "centos8-image-cron-is", metav1.GetOptions{})
		Expect(err).ShouldNot(HaveOccurred())
		Expect(runtime.DefaultUnstructuredConverter.FromUnstructured(unstructured.Object, dic)).To(Succeed())

		Expect(dic.Spec.Template.Spec.Source).ShouldNot(BeNil())
		Expect(dic.Spec.Template.Spec.Source.Registry).ShouldNot(BeNil())
		Expect(dic.Spec.Template.Spec.Source.Registry.ImageStream).Should(HaveValue(Equal("centos8")))
		Expect(dic.Spec.Template.Spec.Source.Registry.PullMethod).Should(HaveValue(Equal(cdiv1beta1.RegistryPullNode)))
	})

	Context("disable the feature", func() {
		It("Should set the FG to false", func() {
			patch := []byte(`[{ "op": "replace", "path": "/spec/featureGates/enableCommonBootImageImport", "value": false }]`)
			Eventually(tests.PatchHCO).WithArguments(ctx, cli, patch).WithTimeout(5 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())
		})

		It("imageStream should be removed", func() {
			Eventually(func(g Gomega) bool {
				_, err := cli.DynamicClient().Resource(isGVR).Namespace(imageNamespace).Get(ctx, "centos8", metav1.GetOptions{})
				g.Expect(err).Should(HaveOccurred())
				return errors.IsNotFound(err)
			}).WithTimeout(5 * time.Second).WithPolling(100 * time.Millisecond).Should(BeTrue())
		})

		It("should empty the DICT in SSP", func() {
			Eventually(func(g Gomega) []v1beta2.DataImportCronTemplate {
				unstructured, err := cli.DynamicClient().Resource(sspGVR).Namespace(flags.KubeVirtInstallNamespace).Get(ctx, "ssp-kubevirt-hyperconverged", metav1.GetOptions{})
				g.Expect(err).ShouldNot(HaveOccurred())
				ssp := &v1beta2.SSP{}
				g.Expect(runtime.DefaultUnstructuredConverter.FromUnstructured(unstructured.Object, ssp)).To(Succeed())
				return ssp.Spec.CommonTemplates.DataImportCronTemplates
			}).WithTimeout(5 * time.Second).WithPolling(100 * time.Millisecond).Should(BeEmpty())
		})

		It("should have no images in the HyperConverged status", func() {
			Eventually(func() []hcov1beta1.DataImportCronTemplateStatus {
				hco := tests.GetHCO(ctx, cli)
				return hco.Status.DataImportCronTemplates
			}).WithTimeout(5 * time.Second).WithPolling(100 * time.Millisecond).Should(BeEmpty())
		})

		It("should have no images", func() {

			Eventually(func(g Gomega) []unstructured.Unstructured {
				list, err := cli.DynamicClient().Resource(dicGVR).Namespace(imageNamespace).List(ctx, metav1.ListOptions{})
				g.Expect(err).ShouldNot(HaveOccurred())

				return list.Items
			}).WithTimeout(5 * time.Minute).WithPolling(time.Second).Should(BeEmpty())
		})
	})

	Context("enable the feature again", func() {
		It("Should set the FG to false", func() {
			patch := []byte(`[{ "op": "replace", "path": "/spec/featureGates/enableCommonBootImageImport", "value": true }]`)
			Eventually(tests.PatchHCO).WithArguments(ctx, cli, patch).WithTimeout(5 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())
		})

		It("imageStream should be recovered", func() {
			Eventually(func(g Gomega) error {
				_, err := cli.DynamicClient().Resource(isGVR).Namespace(imageNamespace).Get(ctx, "centos8", metav1.GetOptions{})
				return err
			}).WithTimeout(5 * time.Second).WithPolling(100 * time.Millisecond).ShouldNot(HaveOccurred())
		})

		It("should propagate the DICT in SSP", func() {
			Eventually(func(g Gomega) []v1beta2.DataImportCronTemplate {
				unstructured, err := cli.DynamicClient().Resource(sspGVR).Namespace(flags.KubeVirtInstallNamespace).Get(ctx, "ssp-kubevirt-hyperconverged", metav1.GetOptions{})
				g.Expect(err).ShouldNot(HaveOccurred())
				ssp := &v1beta2.SSP{}
				g.Expect(runtime.DefaultUnstructuredConverter.FromUnstructured(unstructured.Object, ssp)).To(Succeed())
				return ssp.Spec.CommonTemplates.DataImportCronTemplates
			}).WithTimeout(5 * time.Second).WithPolling(100 * time.Millisecond).Should(HaveLen(len(expectedImages)))
		})

		It("should have all the images in the HyperConverged status", func() {
			Eventually(func() []hcov1beta1.DataImportCronTemplateStatus {
				hco := tests.GetHCO(ctx, cli)
				return hco.Status.DataImportCronTemplates
			}).WithTimeout(5 * time.Second).WithPolling(100 * time.Millisecond).Should(HaveLen(len(expectedImages)))
		})

		It("should restore all the DataImportCron resources", func() {
			Eventually(func(g Gomega) []unstructured.Unstructured {
				list, err := cli.DynamicClient().Resource(dicGVR).Namespace(imageNamespace).List(ctx, metav1.ListOptions{})
				g.Expect(err).ShouldNot(HaveOccurred())

				return list.Items
			}).WithTimeout(5 * time.Minute).WithPolling(5 * time.Second).Should(HaveLen(len(expectedImages)))
		})
	})

	Context("test annotations", func() {

		AfterEach(func() {
			Eventually(func(g Gomega) {
				hc := tests.GetHCO(ctx, cli)

				// make sure there no user-defined DICT
				if len(hc.Spec.DataImportCronTemplates) > 0 {
					hc.APIVersion = "hco.kubevirt.io/v1beta1"
					hc.Kind = "HyperConverged"
					hc.Spec.DataImportCronTemplates = nil

					tests.UpdateHCORetry(ctx, cli, hc)
				}

			}).WithPolling(time.Second * 3).WithTimeout(time.Second * 60).Should(Succeed())
		})

		It("should add missing annotation in the DICT", func() {
			Eventually(func(g Gomega) {
				hc := tests.GetHCO(ctx, cli)

				hc.Spec.DataImportCronTemplates = []hcov1beta1.DataImportCronTemplate{
					getDICT(),
				}

				tests.UpdateHCORetry(ctx, cli, hc)
				newHC := tests.GetHCO(ctx, cli)

				g.Expect(newHC.Spec.DataImportCronTemplates).To(HaveLen(1))
				g.Expect(newHC.Spec.DataImportCronTemplates[0].Annotations).To(HaveKeyWithValue(cdiImmediateBindAnnotation, "true"), "should add the missing annotation")
			}).WithPolling(time.Second * 3).WithTimeout(time.Second * 60).Should(Succeed())
		})

		It("should not change existing annotation in the DICT", func() {
			Eventually(func(g Gomega) {
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
			}).WithPolling(time.Second * 3).WithTimeout(time.Second * 60).Should(Succeed())
		})
	})
})

func getDICT() hcov1beta1.DataImportCronTemplate {
	gcType := cdiv1beta1.DataImportCronGarbageCollectOutdated
	pullMethod := cdiv1beta1.RegistryPullNode

	return hcov1beta1.DataImportCronTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name: "custom",
		},
		Spec: &cdiv1beta1.DataImportCronSpec{
			GarbageCollect:    &gcType,
			ManagedDataSource: "centos7",
			Schedule:          "18 1/12 * * *",
			Template: cdiv1beta1.DataVolume{
				Spec: cdiv1beta1.DataVolumeSpec{
					Source: &cdiv1beta1.DataVolumeSource{
						Registry: &cdiv1beta1.DataVolumeSourceRegistry{
							PullMethod: &pullMethod,
							URL:        ptr.To("docker://quay.io/containerdisks/centos:7-2009"),
						},
					},
					Storage: &cdiv1beta1.StorageSpec{
						Resources: corev1.ResourceRequirements{
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
