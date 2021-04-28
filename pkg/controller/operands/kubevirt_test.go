package operands

import (
	"context"
	"fmt"
	"os"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	conditionsv1 "github.com/openshift/custom-resource-status/conditions/v1"
	"github.com/openshift/custom-resource-status/testlib"
	corev1 "k8s.io/api/core/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/reference"
	kubevirtv1 "kubevirt.io/client-go/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/pkg/apis/hco/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/controller/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/controller/commonTestUtils"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

var _ = Describe("KubeVirt Operand", func() {
	var (
		basicNumFgOnOpenshift = len(hardCodeKvFgs) + len(sspConditionKvFgs)
	)

	Context("KubeVirt Priority Classes", func() {

		var hco *hcov1beta1.HyperConverged
		var req *common.HcoRequest

		BeforeEach(func() {
			hco = commonTestUtils.NewHco()
			req = commonTestUtils.NewReq(hco)
		})

		It("should create if not present", func() {
			expectedResource := NewKubeVirtPriorityClass(hco)
			cl := commonTestUtils.InitClient([]runtime.Object{})
			handler := (*genericOperand)(newKvPriorityClassHandler(cl, commonTestUtils.GetScheme()))
			res := handler.ensure(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			key := client.ObjectKeyFromObject(expectedResource)
			foundResource := &schedulingv1.PriorityClass{}
			Expect(cl.Get(context.TODO(), key, foundResource)).To(BeNil())
			Expect(foundResource.Name).To(Equal(expectedResource.Name))
			Expect(foundResource.Value).To(Equal(expectedResource.Value))
			Expect(foundResource.GlobalDefault).To(Equal(expectedResource.GlobalDefault))
		})

		It("should do nothing if already exists", func() {
			expectedResource := NewKubeVirtPriorityClass(hco)
			cl := commonTestUtils.InitClient([]runtime.Object{expectedResource})
			handler := (*genericOperand)(newKvPriorityClassHandler(cl, commonTestUtils.GetScheme()))
			res := handler.ensure(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			objectRef, err := reference.GetReference(handler.Scheme, expectedResource)
			Expect(err).To(BeNil())
			Expect(hco.Status.RelatedObjects).To(ContainElement(*objectRef))
		})

		DescribeTable("should update if something changed", func(modifiedResource *schedulingv1.PriorityClass) {
			cl := commonTestUtils.InitClient([]runtime.Object{modifiedResource})
			handler := (*genericOperand)(newKvPriorityClassHandler(cl, commonTestUtils.GetScheme()))
			res := handler.ensure(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			expectedResource := NewKubeVirtPriorityClass(hco)
			key := client.ObjectKeyFromObject(expectedResource)
			foundResource := &schedulingv1.PriorityClass{}
			Expect(cl.Get(context.TODO(), key, foundResource))
			Expect(foundResource.Name).To(Equal(expectedResource.Name))
			Expect(foundResource.Value).To(Equal(expectedResource.Value))
			Expect(foundResource.GlobalDefault).To(Equal(expectedResource.GlobalDefault))
		},
			Entry("with modified value",
				&schedulingv1.PriorityClass{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "scheduling.k8s.io/v1",
						Kind:       "PriorityClass",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "kubevirt-cluster-critical",
					},
					Value:         1,
					GlobalDefault: false,
					Description:   "",
				}),
			Entry("with modified global default",
				&schedulingv1.PriorityClass{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "scheduling.k8s.io/v1",
						Kind:       "PriorityClass",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "kubevirt-cluster-critical",
					},
					Value:         1000000000,
					GlobalDefault: true,
					Description:   "",
				}),
		)

	})

	Context("KubeVirt", func() {
		var hco *hcov1beta1.HyperConverged
		var req *common.HcoRequest

		defer os.Unsetenv(smbiosEnvName)
		defer os.Unsetenv(machineTypeEnvName)
		defer os.Unsetenv(kvmEmulationEnvName)

		BeforeEach(func() {
			hco = commonTestUtils.NewHco()
			req = commonTestUtils.NewReq(hco)
			os.Setenv(smbiosEnvName,
				`Family: smbios family
Product: smbios product
Manufacturer: smbios manufacturer
Sku: 1.2.3
Version: 1.2.3`)

			os.Setenv(machineTypeEnvName, "machine-type")
			os.Setenv(kvmEmulationEnvName, "false")
		})

		It("should create if not present", func() {
			mandatoryKvFeatureGates = getMandatoryKvFeatureGates(false)
			hco.Spec.FeatureGates = hcov1beta1.HyperConvergedFeatureGates{
				WithHostPassthroughCPU: true,
			}

			expectedResource, err := NewKubeVirt(hco, commonTestUtils.Namespace)
			Expect(err).ToNot(HaveOccurred())
			cl := commonTestUtils.InitClient([]runtime.Object{})
			handler := (*genericOperand)(newKubevirtHandler(cl, commonTestUtils.GetScheme()))
			res := handler.ensure(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			foundResource := &kubevirtv1.KubeVirt{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: expectedResource.Name, Namespace: expectedResource.Namespace},
					foundResource),
			).To(BeNil())
			Expect(foundResource.Name).To(Equal(expectedResource.Name))
			Expect(foundResource.Labels).Should(HaveKeyWithValue(hcoutil.AppLabel, commonTestUtils.Name))
			Expect(foundResource.Namespace).To(Equal(expectedResource.Namespace))

			Expect(foundResource.Spec.Configuration.DeveloperConfiguration).ToNot(BeNil())
			Expect(foundResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(HaveLen(basicNumFgOnOpenshift + 1))
			Expect(foundResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(ContainElements(
				hardCodeKvFgs,
			))
			Expect(foundResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(ContainElements(
				sspConditionKvFgs,
			))
			Expect(foundResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(ContainElement(
				kvWithHostPassthroughCPU,
			))

			Expect(foundResource.Spec.Configuration.MachineType).Should(Equal("machine-type"))

			Expect(foundResource.Spec.Configuration.SMBIOSConfig).ToNot(BeNil())
			Expect(foundResource.Spec.Configuration.SMBIOSConfig.Family).Should(Equal("smbios family"))
			Expect(foundResource.Spec.Configuration.SMBIOSConfig.Product).Should(Equal("smbios product"))
			Expect(foundResource.Spec.Configuration.SMBIOSConfig.Manufacturer).Should(Equal("smbios manufacturer"))
			Expect(foundResource.Spec.Configuration.SMBIOSConfig.Sku).Should(Equal("1.2.3"))
			Expect(foundResource.Spec.Configuration.SMBIOSConfig.Version).Should(Equal("1.2.3"))

			Expect(foundResource.Spec.Configuration.SELinuxLauncherType).Should(Equal(SELinuxLauncherType))

			Expect(foundResource.Spec.Configuration.NetworkConfiguration).ToNot(BeNil())
			Expect(foundResource.Spec.Configuration.NetworkConfiguration.NetworkInterface).Should(Equal(string(kubevirtv1.MasqueradeInterface)))

			// LiveMigration Configurations
			mc := foundResource.Spec.Configuration.MigrationConfiguration
			Expect(mc).ToNot(BeNil())
			Expect(*mc.BandwidthPerMigration).Should(Equal(resource.MustParse("64Mi")))
			Expect(*mc.CompletionTimeoutPerGiB).Should(Equal(int64(800)))
			Expect(*mc.ParallelMigrationsPerCluster).Should(Equal(uint32(5)))
			Expect(*mc.ParallelOutboundMigrationsPerNode).Should(Equal(uint32(2)))
			Expect(*mc.ProgressTimeout).Should(Equal(int64(150)))
		})

		It("should find if present", func() {
			expectedResource, err := NewKubeVirt(hco, commonTestUtils.Namespace)
			Expect(err).ToNot(HaveOccurred())
			expectedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", expectedResource.Namespace, expectedResource.Name)
			cl := commonTestUtils.InitClient([]runtime.Object{hco, expectedResource})
			handler := (*genericOperand)(newKubevirtHandler(cl, commonTestUtils.GetScheme()))
			res := handler.ensure(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			// Check HCO's status
			Expect(hco.Status.RelatedObjects).To(Not(BeNil()))
			objectRef, err := reference.GetReference(handler.Scheme, expectedResource)
			Expect(err).To(BeNil())
			// ObjectReference should have been added
			Expect(hco.Status.RelatedObjects).To(ContainElement(*objectRef))
			// Check conditions
			Expect(req.Conditions[conditionsv1.ConditionAvailable]).To(testlib.RepresentCondition(conditionsv1.Condition{
				Type:    conditionsv1.ConditionAvailable,
				Status:  corev1.ConditionFalse,
				Reason:  "KubeVirtConditions",
				Message: "KubeVirt resource has no conditions",
			}))
			Expect(req.Conditions[conditionsv1.ConditionProgressing]).To(testlib.RepresentCondition(conditionsv1.Condition{
				Type:    conditionsv1.ConditionProgressing,
				Status:  corev1.ConditionTrue,
				Reason:  "KubeVirtConditions",
				Message: "KubeVirt resource has no conditions",
			}))
			Expect(req.Conditions[conditionsv1.ConditionUpgradeable]).To(testlib.RepresentCondition(conditionsv1.Condition{
				Type:    conditionsv1.ConditionUpgradeable,
				Status:  corev1.ConditionFalse,
				Reason:  "KubeVirtConditions",
				Message: "KubeVirt resource has no conditions",
			}))
		})

		It("should force mandatory configurations", func() {
			mandatoryKvFeatureGates = getMandatoryKvFeatureGates(false)
			hco.Spec.FeatureGates = hcov1beta1.HyperConvergedFeatureGates{
				WithHostPassthroughCPU: true,
			}

			os.Setenv(smbiosEnvName,
				`Family: smbios family
Product: smbios product
Manufacturer: smbios manufacturer
Sku: 1.2.3
Version: 1.2.3`)
			os.Setenv(machineTypeEnvName, "machine-type")

			existKv, err := NewKubeVirt(hco, commonTestUtils.Namespace)
			Expect(err).ToNot(HaveOccurred())
			existKv.Spec.Configuration.DeveloperConfiguration = &kubevirtv1.DeveloperConfiguration{
				FeatureGates: []string{"wrongFG1", "wrongFG2", "wrongFG3"},
			}
			existKv.Spec.Configuration.MachineType = "wrong machine type"
			existKv.Spec.Configuration.SMBIOSConfig = &kubevirtv1.SMBiosConfiguration{
				Family:       "wrong family",
				Product:      "wrong product",
				Manufacturer: "wrong manifaturer",
				Sku:          "0.0.0",
				Version:      "1.1.1",
			}
			existKv.Spec.Configuration.SELinuxLauncherType = "wrongSELinuxLauncherType"
			existKv.Spec.Configuration.NetworkConfiguration = &kubevirtv1.NetworkConfiguration{
				NetworkInterface: "wrong network interface",
			}
			existKv.Spec.Configuration.EmulatedMachines = []string{"wrong"}

			existKv.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", existKv.Namespace, existKv.Name)

			// LiveMigration Configurations
			bandwidthPerMigration := resource.MustParse("16Mi")
			wrongNumeric64Value := int64(0)
			wrongNumeric32Value := uint32(0)
			existKv.Spec.Configuration.MigrationConfiguration = &kubevirtv1.MigrationConfiguration{
				BandwidthPerMigration:             &bandwidthPerMigration,
				CompletionTimeoutPerGiB:           &wrongNumeric64Value,
				ParallelMigrationsPerCluster:      &wrongNumeric32Value,
				ParallelOutboundMigrationsPerNode: &wrongNumeric32Value,
				ProgressTimeout:                   &wrongNumeric64Value,
			}

			cl := commonTestUtils.InitClient([]runtime.Object{hco, existKv})
			handler := (*genericOperand)(newKubevirtHandler(cl, commonTestUtils.GetScheme()))
			res := handler.ensure(req)

			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Updated).To(BeTrue())
			Expect(res.Err).To(BeNil())

			foundResource := &kubevirtv1.KubeVirt{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: existKv.Name, Namespace: existKv.Namespace},
					foundResource),
			).To(BeNil())
			Expect(foundResource.Spec.Configuration.DeveloperConfiguration).ToNot(BeNil())
			Expect(foundResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(HaveLen(basicNumFgOnOpenshift + 1))
			Expect(foundResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(ContainElements(
				hardCodeKvFgs,
			))
			Expect(foundResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(ContainElements(
				sspConditionKvFgs,
			))
			Expect(foundResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(ContainElement(
				kvWithHostPassthroughCPU,
			))

			Expect(foundResource.Spec.Configuration.MachineType).Should(Equal("machine-type"))

			Expect(foundResource.Spec.Configuration.SMBIOSConfig).ToNot(BeNil())
			Expect(foundResource.Spec.Configuration.SMBIOSConfig.Family).Should(Equal("smbios family"))
			Expect(foundResource.Spec.Configuration.SMBIOSConfig.Product).Should(Equal("smbios product"))
			Expect(foundResource.Spec.Configuration.SMBIOSConfig.Manufacturer).Should(Equal("smbios manufacturer"))
			Expect(foundResource.Spec.Configuration.SMBIOSConfig.Sku).Should(Equal("1.2.3"))
			Expect(foundResource.Spec.Configuration.SMBIOSConfig.Version).Should(Equal("1.2.3"))

			Expect(foundResource.Spec.Configuration.SELinuxLauncherType).Should(Equal(SELinuxLauncherType))

			Expect(foundResource.Spec.Configuration.NetworkConfiguration).ToNot(BeNil())
			Expect(foundResource.Spec.Configuration.NetworkConfiguration.NetworkInterface).Should(Equal(string(kubevirtv1.MasqueradeInterface)))

			Expect(foundResource.Spec.Configuration.EmulatedMachines).Should(BeEmpty())

			// LiveMigration Configurations
			mc := foundResource.Spec.Configuration.MigrationConfiguration
			Expect(mc).ToNot(BeNil())
			Expect(*mc.BandwidthPerMigration).Should(Equal(resource.MustParse("64Mi")))
			Expect(*mc.CompletionTimeoutPerGiB).Should(Equal(int64(800)))
			Expect(*mc.ParallelMigrationsPerCluster).Should(Equal(uint32(5)))
			Expect(*mc.ParallelOutboundMigrationsPerNode).Should(Equal(uint32(2)))
			Expect(*mc.ProgressTimeout).Should(Equal(int64(150)))
		})

		It("should fail if the SMBIOS is wrongly formatted mandatory configurations", func() {
			hco.Spec.FeatureGates = hcov1beta1.HyperConvergedFeatureGates{
				WithHostPassthroughCPU: true,
			}

			_ = os.Setenv(smbiosEnvName, "WRONG YAML")

			_, err := NewKubeVirt(hco, commonTestUtils.Namespace)
			Expect(err).To(HaveOccurred())
		})

		It("should fail if the Spec.LiveMigrationConfig.BandwidthPerMigration is wrongly formatted", func() {
			wrongFormat := "Wrong Format"
			hco.Spec.LiveMigrationConfig.BandwidthPerMigration = &wrongFormat

			_, err := NewKubeVirt(hco, commonTestUtils.Namespace)
			Expect(err).To(HaveOccurred())
		})

		It("should set default UninstallStrategy if missing", func() {
			expectedResource, err := NewKubeVirt(hco, commonTestUtils.Namespace)
			Expect(err).ToNot(HaveOccurred())
			expectedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", expectedResource.Namespace, expectedResource.Name)
			missingUSResource, err := NewKubeVirt(hco, commonTestUtils.Namespace)
			Expect(err).ToNot(HaveOccurred())
			missingUSResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", missingUSResource.Namespace, missingUSResource.Name)
			missingUSResource.Spec.UninstallStrategy = ""

			cl := commonTestUtils.InitClient([]runtime.Object{hco, missingUSResource})
			handler := (*genericOperand)(newKubevirtHandler(cl, commonTestUtils.GetScheme()))
			res := handler.ensure(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Updated).To(BeTrue())
			Expect(res.Overwritten).To(BeFalse())
			Expect(res.Err).To(BeNil())

			foundResource := &kubevirtv1.KubeVirt{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: expectedResource.Name, Namespace: expectedResource.Namespace},
					foundResource),
			).To(BeNil())
			Expect(foundResource.Spec.UninstallStrategy).To(Equal(expectedResource.Spec.UninstallStrategy))
		})

		It("should propagate the live migration configuration from the HC", func() {
			existKv, err := NewKubeVirt(hco)
			Expect(err).ToNot(HaveOccurred())

			bandwidthPerMigration := "16Mi"
			completionTimeoutPerGiB := int64(100)
			parallelOutboundMigrationsPerNode := uint32(7)
			parallelMigrationsPerCluster := uint32(18)
			progressTimeout := int64(5000)

			hco.Spec.LiveMigrationConfig.BandwidthPerMigration = &bandwidthPerMigration
			hco.Spec.LiveMigrationConfig.CompletionTimeoutPerGiB = &completionTimeoutPerGiB
			hco.Spec.LiveMigrationConfig.ParallelOutboundMigrationsPerNode = &parallelOutboundMigrationsPerNode
			hco.Spec.LiveMigrationConfig.ParallelMigrationsPerCluster = &parallelMigrationsPerCluster
			hco.Spec.LiveMigrationConfig.ProgressTimeout = &progressTimeout

			cl := commonTestUtils.InitClient([]runtime.Object{hco, existKv})
			handler := (*genericOperand)(newKubevirtHandler(cl, commonTestUtils.GetScheme()))
			res := handler.ensure(req)

			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Updated).To(BeTrue())
			Expect(res.Err).To(BeNil())

			foundResource := &kubevirtv1.KubeVirt{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: existKv.Name, Namespace: existKv.Namespace},
					foundResource),
			).To(BeNil())

			mc := foundResource.Spec.Configuration.MigrationConfiguration
			Expect(mc).ToNot(BeNil())
			Expect(*mc.BandwidthPerMigration).To(Equal(resource.MustParse(bandwidthPerMigration)))
			Expect(*mc.CompletionTimeoutPerGiB).To(Equal(completionTimeoutPerGiB))
			Expect(*mc.ParallelOutboundMigrationsPerNode).To(Equal(parallelOutboundMigrationsPerNode))
			Expect(*mc.ParallelMigrationsPerCluster).To(Equal(parallelMigrationsPerCluster))
			Expect(*mc.ProgressTimeout).To(Equal(progressTimeout))
		})

		It("should propagate the permitted host devices configuration from the HC", func() {
			existKv, err := NewKubeVirt(hco)
			Expect(err).ToNot(HaveOccurred())

			hco.Spec.PermittedHostDevices = &hcov1beta1.PermittedHostDevices{
				PciHostDevices: []hcov1beta1.PciHostDevice{
					{
						PCIVendorSelector:        "vendor1",
						ResourceName:             "resourceName1",
						ExternalResourceProvider: true,
					},
					{
						PCIVendorSelector:        "vendor2",
						ResourceName:             "resourceName2",
						ExternalResourceProvider: false,
					},
					{
						PCIVendorSelector:        "vendor3",
						ResourceName:             "resourceName3",
						ExternalResourceProvider: true,
					},
				},
				MediatedDevices: []hcov1beta1.MediatedHostDevice{
					{
						MDEVNameSelector:         "selector1",
						ResourceName:             "resource1",
						ExternalResourceProvider: true,
					},
					{
						MDEVNameSelector:         "selector2",
						ResourceName:             "resource2",
						ExternalResourceProvider: false,
					},
					{
						MDEVNameSelector:         "selector3",
						ResourceName:             "resource3",
						ExternalResourceProvider: true,
					},
					{
						MDEVNameSelector:         "selector4",
						ResourceName:             "resource4",
						ExternalResourceProvider: false,
					},
				},
			}

			cl := commonTestUtils.InitClient([]runtime.Object{hco, existKv})
			handler := (*genericOperand)(newKubevirtHandler(cl, commonTestUtils.GetScheme()))
			res := handler.ensure(req)

			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Updated).To(BeTrue())
			Expect(res.Err).To(BeNil())

			foundResource := &kubevirtv1.KubeVirt{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: existKv.Name, Namespace: existKv.Namespace},
					foundResource),
			).To(BeNil())

			phd := foundResource.Spec.Configuration.PermittedHostDevices
			Expect(phd).ToNot(BeNil())
			Expect(phd.PciHostDevices).To(HaveLen(3))
			Expect(phd.PciHostDevices).To(ContainElements(
				kubevirtv1.PciHostDevice{
					PCIVendorSelector:        "vendor1",
					ResourceName:             "resourceName1",
					ExternalResourceProvider: true,
				},
				kubevirtv1.PciHostDevice{
					PCIVendorSelector:        "vendor2",
					ResourceName:             "resourceName2",
					ExternalResourceProvider: false,
				},
				kubevirtv1.PciHostDevice{
					PCIVendorSelector:        "vendor3",
					ResourceName:             "resourceName3",
					ExternalResourceProvider: true,
				},
			))

			Expect(phd.MediatedDevices).To(HaveLen(4))
			Expect(phd.MediatedDevices).To(ContainElements(
				kubevirtv1.MediatedHostDevice{
					MDEVNameSelector:         "selector1",
					ResourceName:             "resource1",
					ExternalResourceProvider: true,
				},
				kubevirtv1.MediatedHostDevice{
					MDEVNameSelector:         "selector2",
					ResourceName:             "resource2",
					ExternalResourceProvider: false,
				},
				kubevirtv1.MediatedHostDevice{
					MDEVNameSelector:         "selector3",
					ResourceName:             "resource3",
					ExternalResourceProvider: true,
				},
				kubevirtv1.MediatedHostDevice{
					MDEVNameSelector:         "selector4",
					ResourceName:             "resource4",
					ExternalResourceProvider: false,
				},
			))
		})

		It("should update the permitted host devices configuration from the HC", func() {
			existKv, err := NewKubeVirt(hco)
			Expect(err).ToNot(HaveOccurred())

			existKv.Spec.Configuration.PermittedHostDevices = &kubevirtv1.PermittedHostDevices{
				PciHostDevices: []kubevirtv1.PciHostDevice{
					{
						PCIVendorSelector:        "other1",
						ResourceName:             "otherResourceName1",
						ExternalResourceProvider: true,
					},
					{
						PCIVendorSelector:        "other2",
						ResourceName:             "otherResourceName2",
						ExternalResourceProvider: false,
					},
					{
						PCIVendorSelector:        "other3",
						ResourceName:             "otherResourceName3",
						ExternalResourceProvider: true,
					},
					{
						PCIVendorSelector:        "other4",
						ResourceName:             "otherResourceName4",
						ExternalResourceProvider: true,
					},
				},
				MediatedDevices: []kubevirtv1.MediatedHostDevice{
					{
						MDEVNameSelector:         "otherSelector1",
						ResourceName:             "otherResource1",
						ExternalResourceProvider: false,
					},
					{
						MDEVNameSelector:         "otherSelector2",
						ResourceName:             "otherResource2",
						ExternalResourceProvider: true,
					},
					{
						MDEVNameSelector:         "otherSelector3",
						ResourceName:             "otherResource3",
						ExternalResourceProvider: true,
					},
				},
			}

			hco.Spec.PermittedHostDevices = &hcov1beta1.PermittedHostDevices{
				PciHostDevices: []hcov1beta1.PciHostDevice{
					{
						PCIVendorSelector:        "vendor1",
						ResourceName:             "resourceName1",
						ExternalResourceProvider: true,
					},
					{
						PCIVendorSelector:        "vendor2",
						ResourceName:             "resourceName2",
						ExternalResourceProvider: false,
					},
					{
						PCIVendorSelector:        "vendor3",
						ResourceName:             "resourceName3",
						ExternalResourceProvider: true,
					},
				},
				MediatedDevices: []hcov1beta1.MediatedHostDevice{
					{
						MDEVNameSelector:         "selector1",
						ResourceName:             "resource1",
						ExternalResourceProvider: true,
					},
					{
						MDEVNameSelector:         "selector2",
						ResourceName:             "resource2",
						ExternalResourceProvider: false,
					},
					{
						MDEVNameSelector:         "selector3",
						ResourceName:             "resource3",
						ExternalResourceProvider: true,
					},
					{
						MDEVNameSelector:         "selector4",
						ResourceName:             "resource4",
						ExternalResourceProvider: false,
					},
				},
			}

			cl := commonTestUtils.InitClient([]runtime.Object{hco, existKv})

			By("Check before reconciling", func() {
				foundResource := &kubevirtv1.KubeVirt{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: existKv.Name, Namespace: existKv.Namespace},
						foundResource),
				).To(BeNil())

				phd := foundResource.Spec.Configuration.PermittedHostDevices
				Expect(phd).ToNot(BeNil())
				Expect(phd.PciHostDevices).To(HaveLen(4))
				Expect(phd.PciHostDevices).To(ContainElements(
					kubevirtv1.PciHostDevice{
						PCIVendorSelector:        "other1",
						ResourceName:             "otherResourceName1",
						ExternalResourceProvider: true,
					},
					kubevirtv1.PciHostDevice{
						PCIVendorSelector:        "other2",
						ResourceName:             "otherResourceName2",
						ExternalResourceProvider: false,
					},
					kubevirtv1.PciHostDevice{
						PCIVendorSelector:        "other3",
						ResourceName:             "otherResourceName3",
						ExternalResourceProvider: true,
					},
					kubevirtv1.PciHostDevice{
						PCIVendorSelector:        "other4",
						ResourceName:             "otherResourceName4",
						ExternalResourceProvider: true,
					},
				))

				Expect(phd.MediatedDevices).To(HaveLen(3))
				Expect(phd.MediatedDevices).To(ContainElements(
					kubevirtv1.MediatedHostDevice{
						MDEVNameSelector:         "otherSelector1",
						ResourceName:             "otherResource1",
						ExternalResourceProvider: false,
					},
					kubevirtv1.MediatedHostDevice{
						MDEVNameSelector:         "otherSelector2",
						ResourceName:             "otherResource2",
						ExternalResourceProvider: true,
					},
					kubevirtv1.MediatedHostDevice{
						MDEVNameSelector:         "otherSelector3",
						ResourceName:             "otherResource3",
						ExternalResourceProvider: true,
					},
				))

			})

			handler := (*genericOperand)(newKubevirtHandler(cl, commonTestUtils.GetScheme()))
			res := handler.ensure(req)

			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Updated).To(BeTrue())
			Expect(res.Err).To(BeNil())

			foundResource := &kubevirtv1.KubeVirt{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: existKv.Name, Namespace: existKv.Namespace},
					foundResource),
			).To(BeNil())

			By("Check after reconcile")
			phd := foundResource.Spec.Configuration.PermittedHostDevices
			Expect(phd).ToNot(BeNil())
			Expect(phd.PciHostDevices).To(HaveLen(3))
			Expect(phd.PciHostDevices).To(ContainElements(
				kubevirtv1.PciHostDevice{
					PCIVendorSelector:        "vendor1",
					ResourceName:             "resourceName1",
					ExternalResourceProvider: true,
				},
				kubevirtv1.PciHostDevice{
					PCIVendorSelector:        "vendor2",
					ResourceName:             "resourceName2",
					ExternalResourceProvider: false,
				},
				kubevirtv1.PciHostDevice{
					PCIVendorSelector:        "vendor3",
					ResourceName:             "resourceName3",
					ExternalResourceProvider: true,
				},
			))

			Expect(phd.MediatedDevices).To(HaveLen(4))
			Expect(phd.MediatedDevices).To(ContainElements(
				kubevirtv1.MediatedHostDevice{
					MDEVNameSelector:         "selector1",
					ResourceName:             "resource1",
					ExternalResourceProvider: true,
				},
				kubevirtv1.MediatedHostDevice{
					MDEVNameSelector:         "selector2",
					ResourceName:             "resource2",
					ExternalResourceProvider: false,
				},
				kubevirtv1.MediatedHostDevice{
					MDEVNameSelector:         "selector3",
					ResourceName:             "resource3",
					ExternalResourceProvider: true,
				},
				kubevirtv1.MediatedHostDevice{
					MDEVNameSelector:         "selector4",
					ResourceName:             "resource4",
					ExternalResourceProvider: false,
				},
			))
		})

		Context("Test node placement", func() {
			It("should add node placement if missing in KubeVirt", func() {
				existingResource, err := NewKubeVirt(hco)
				Expect(err).ToNot(HaveOccurred())

				hco.Spec.Infra = hcov1beta1.HyperConvergedConfig{NodePlacement: commonTestUtils.NewNodePlacement()}
				hco.Spec.Workloads = hcov1beta1.HyperConvergedConfig{NodePlacement: commonTestUtils.NewNodePlacement()}

				cl := commonTestUtils.InitClient([]runtime.Object{hco, existingResource})
				handler := (*genericOperand)(newKubevirtHandler(cl, commonTestUtils.GetScheme()))
				res := handler.ensure(req)
				Expect(res.UpgradeDone).To(BeFalse())
				Expect(res.Updated).To(BeTrue())
				Expect(res.Overwritten).To(BeFalse())
				Expect(res.Err).To(BeNil())

				foundResource := &kubevirtv1.KubeVirt{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
						foundResource),
				).To(BeNil())

				Expect(existingResource.Spec.Infra).To(BeNil())
				Expect(existingResource.Spec.Workloads).To(BeNil())

				Expect(foundResource.Spec.Infra).ToNot(BeNil())
				Expect(foundResource.Spec.Infra.NodePlacement).ToNot(BeNil())
				Expect(foundResource.Spec.Infra.NodePlacement.Affinity).ToNot(BeNil())
				Expect(foundResource.Spec.Infra.NodePlacement.NodeSelector["key1"]).Should(Equal("value1"))
				Expect(foundResource.Spec.Infra.NodePlacement.NodeSelector["key2"]).Should(Equal("value2"))

				Expect(foundResource.Spec.Workloads).ToNot(BeNil())
				Expect(foundResource.Spec.Workloads.NodePlacement).ToNot(BeNil())
				Expect(foundResource.Spec.Workloads.NodePlacement.Tolerations).Should(Equal(hco.Spec.Workloads.NodePlacement.Tolerations))

				Expect(req.Conditions).To(BeEmpty())
			})

			It("should remove node placement if missing in HCO CR", func() {
				hcoNodePlacement := commonTestUtils.NewHco()
				hcoNodePlacement.Spec.Infra = hcov1beta1.HyperConvergedConfig{NodePlacement: commonTestUtils.NewNodePlacement()}
				hcoNodePlacement.Spec.Workloads = hcov1beta1.HyperConvergedConfig{NodePlacement: commonTestUtils.NewNodePlacement()}
				existingResource, err := NewKubeVirt(hcoNodePlacement)
				Expect(err).ToNot(HaveOccurred())

				cl := commonTestUtils.InitClient([]runtime.Object{hco, existingResource})
				handler := (*genericOperand)(newKubevirtHandler(cl, commonTestUtils.GetScheme()))
				res := handler.ensure(req)
				Expect(res.UpgradeDone).To(BeFalse())
				Expect(res.Updated).To(BeTrue())
				Expect(res.Overwritten).To(BeFalse())
				Expect(res.Err).To(BeNil())

				foundResource := &kubevirtv1.KubeVirt{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
						foundResource),
				).To(BeNil())

				Expect(existingResource.Spec.Infra).ToNot(BeNil())
				Expect(existingResource.Spec.Workloads).ToNot(BeNil())

				Expect(foundResource.Spec.Infra).To(BeNil())
				Expect(foundResource.Spec.Workloads).To(BeNil())

				Expect(req.Conditions).To(BeEmpty())
			})

			It("should modify node placement according to HCO CR", func() {
				hco.Spec.Infra = hcov1beta1.HyperConvergedConfig{NodePlacement: commonTestUtils.NewNodePlacement()}
				hco.Spec.Workloads = hcov1beta1.HyperConvergedConfig{NodePlacement: commonTestUtils.NewNodePlacement()}
				existingResource, err := NewKubeVirt(hco)
				Expect(err).ToNot(HaveOccurred())

				// now, modify HCO's node placement
				seconds3 := int64(3)
				hco.Spec.Infra.NodePlacement.Tolerations = append(hco.Spec.Infra.NodePlacement.Tolerations, corev1.Toleration{
					Key: "key3", Operator: "operator3", Value: "value3", Effect: "effect3", TolerationSeconds: &seconds3,
				})

				hco.Spec.Workloads.NodePlacement.NodeSelector["key1"] = "something else"

				cl := commonTestUtils.InitClient([]runtime.Object{hco, existingResource})
				handler := (*genericOperand)(newKubevirtHandler(cl, commonTestUtils.GetScheme()))
				res := handler.ensure(req)
				Expect(res.UpgradeDone).To(BeFalse())
				Expect(res.Updated).To(BeTrue())
				Expect(res.Err).To(BeNil())

				foundResource := &kubevirtv1.KubeVirt{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
						foundResource),
				).To(BeNil())

				Expect(existingResource.Spec.Infra).ToNot(BeNil())
				Expect(existingResource.Spec.Infra.NodePlacement).ToNot(BeNil())
				Expect(existingResource.Spec.Infra.NodePlacement.Tolerations).To(HaveLen(2))
				Expect(existingResource.Spec.Workloads).ToNot(BeNil())

				Expect(existingResource.Spec.Workloads.NodePlacement).ToNot(BeNil())
				Expect(existingResource.Spec.Workloads.NodePlacement.NodeSelector["key1"]).Should(Equal("value1"))

				Expect(foundResource.Spec.Infra).ToNot(BeNil())
				Expect(foundResource.Spec.Infra.NodePlacement).ToNot(BeNil())
				Expect(foundResource.Spec.Infra.NodePlacement.Tolerations).To(HaveLen(3))

				Expect(foundResource.Spec.Workloads).ToNot(BeNil())
				Expect(foundResource.Spec.Workloads.NodePlacement).ToNot(BeNil())
				Expect(foundResource.Spec.Workloads.NodePlacement.NodeSelector["key1"]).Should(Equal("something else"))

				Expect(req.Conditions).To(BeEmpty())
			})

			It("should overwrite node placement if directly set on KV CR", func() {
				hco.Spec.Infra = hcov1beta1.HyperConvergedConfig{NodePlacement: commonTestUtils.NewNodePlacement()}
				hco.Spec.Workloads = hcov1beta1.HyperConvergedConfig{NodePlacement: commonTestUtils.NewNodePlacement()}
				existingResource, err := NewKubeVirt(hco)
				Expect(err).ToNot(HaveOccurred())

				// mock a reconciliation triggered by a change in KV CR
				req.HCOTriggered = false

				// now, modify KV's node placement
				seconds3 := int64(3)
				existingResource.Spec.Infra.NodePlacement.Tolerations = append(hco.Spec.Infra.NodePlacement.Tolerations, corev1.Toleration{
					Key: "key3", Operator: "operator3", Value: "value3", Effect: "effect3", TolerationSeconds: &seconds3,
				})
				existingResource.Spec.Workloads.NodePlacement.Tolerations = append(hco.Spec.Workloads.NodePlacement.Tolerations, corev1.Toleration{
					Key: "key3", Operator: "operator3", Value: "value3", Effect: "effect3", TolerationSeconds: &seconds3,
				})

				existingResource.Spec.Infra.NodePlacement.NodeSelector["key1"] = "BADvalue1"
				existingResource.Spec.Workloads.NodePlacement.NodeSelector["key2"] = "BADvalue2"

				cl := commonTestUtils.InitClient([]runtime.Object{hco, existingResource})
				handler := (*genericOperand)(newKubevirtHandler(cl, commonTestUtils.GetScheme()))
				res := handler.ensure(req)
				Expect(res.UpgradeDone).To(BeFalse())
				Expect(res.Updated).To(BeTrue())
				Expect(res.Overwritten).To(BeTrue())
				Expect(res.Err).To(BeNil())

				foundResource := &kubevirtv1.KubeVirt{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
						foundResource),
				).To(BeNil())

				Expect(existingResource.Spec.Infra.NodePlacement.Tolerations).To(HaveLen(3))
				Expect(existingResource.Spec.Workloads.NodePlacement.Tolerations).To(HaveLen(3))
				Expect(existingResource.Spec.Infra.NodePlacement.NodeSelector["key1"]).Should(Equal("BADvalue1"))
				Expect(existingResource.Spec.Workloads.NodePlacement.NodeSelector["key2"]).Should(Equal("BADvalue2"))

				Expect(foundResource.Spec.Infra.NodePlacement.Tolerations).To(HaveLen(2))
				Expect(foundResource.Spec.Workloads.NodePlacement.Tolerations).To(HaveLen(2))
				Expect(foundResource.Spec.Infra.NodePlacement.NodeSelector["key1"]).Should(Equal("value1"))
				Expect(foundResource.Spec.Workloads.NodePlacement.NodeSelector["key2"]).Should(Equal("value2"))

				Expect(req.Conditions).To(BeEmpty())
			})
		})

		Context("Feature Gates", func() {
			Context("test feature gates in NewKubeVirt", func() {
				It("should add the WithHostPassthroughCPU feature gate if it's set in HyperConverged CR", func() {
					// one enabled, one disabled and one missing
					hco.Spec.FeatureGates = hcov1beta1.HyperConvergedFeatureGates{
						WithHostPassthroughCPU: true,
					}

					existingResource, err := NewKubeVirt(hco)
					Expect(err).ToNot(HaveOccurred())
					By("KV CR should contain the HotplugVolumes feature gate", func() {
						Expect(existingResource.Spec.Configuration.DeveloperConfiguration).NotTo(BeNil())
						Expect(existingResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(ContainElement(kvWithHostPassthroughCPU))
					})
				})

				It("should not add the WithHostPassthroughCPU feature gate if it's disabled in HyperConverged CR", func() {
					// one enabled, one disabled and one missing
					hco.Spec.FeatureGates = hcov1beta1.HyperConvergedFeatureGates{
						WithHostPassthroughCPU: false,
					}

					existingResource, err := NewKubeVirt(hco)
					Expect(err).ToNot(HaveOccurred())
					By("KV CR should contain the HotplugVolumes feature gate", func() {
						Expect(existingResource.Spec.Configuration.DeveloperConfiguration).NotTo(BeNil())
						Expect(existingResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).ToNot(ContainElement("WithHostPassthroughCPU"))
					})
				})

				It("should add the SRIOVLiveMigration feature gate if it's set in HyperConverged CR", func() {
					// one enabled, one disabled and one missing
					hco.Spec.FeatureGates = hcov1beta1.HyperConvergedFeatureGates{
						SRIOVLiveMigration: true,
					}

					existingResource, err := NewKubeVirt(hco)
					Expect(err).ToNot(HaveOccurred())
					By("KV CR should contain the HotplugVolumes feature gate", func() {
						Expect(existingResource.Spec.Configuration.DeveloperConfiguration).NotTo(BeNil())
						Expect(existingResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(ContainElement(kvSRIOVLiveMigration))
					})
				})

				It("should not add the SRIOVLiveMigration feature gate if it's disabled in HyperConverged CR", func() {
					// one enabled, one disabled and one missing
					hco.Spec.FeatureGates = hcov1beta1.HyperConvergedFeatureGates{
						SRIOVLiveMigration: false,
					}

					existingResource, err := NewKubeVirt(hco)
					Expect(err).ToNot(HaveOccurred())
					By("KV CR should contain the HotplugVolumes feature gate", func() {
						Expect(existingResource.Spec.Configuration.DeveloperConfiguration).NotTo(BeNil())
						Expect(existingResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).ToNot(ContainElement("SRIOVLiveMigration"))
					})
				})

				It("should not add the feature gates if FeatureGates field is empty", func() {
					mandatoryKvFeatureGates = getMandatoryKvFeatureGates(false)
					hco.Spec.FeatureGates = hcov1beta1.HyperConvergedFeatureGates{}

					existingResource, err := NewKubeVirt(hco)
					Expect(err).ToNot(HaveOccurred())

					Expect(existingResource.Spec.Configuration.DeveloperConfiguration).ToNot(BeNil())
					fgList := getKvFeatureGateList(&hco.Spec.FeatureGates)
					Expect(fgList).To(HaveLen(basicNumFgOnOpenshift))
					Expect(fgList).Should(ContainElements(hardCodeKvFgs))
					Expect(fgList).Should(ContainElements(sspConditionKvFgs))
				})
			})

			Context("test feature gates in KV handler", func() {
				It("should add feature gates if they are set to true", func() {
					existingResource, err := NewKubeVirt(hco)
					Expect(err).ToNot(HaveOccurred())

					hco.Spec.FeatureGates = hcov1beta1.HyperConvergedFeatureGates{
						WithHostPassthroughCPU: true,
						SRIOVLiveMigration:     true,
					}

					cl := commonTestUtils.InitClient([]runtime.Object{hco, existingResource})
					handler := (*genericOperand)(newKubevirtHandler(cl, commonTestUtils.GetScheme()))
					res := handler.ensure(req)
					Expect(res.UpgradeDone).To(BeFalse())
					Expect(res.Updated).To(BeTrue())
					Expect(res.Overwritten).To(BeFalse())
					Expect(res.Err).To(BeNil())

					foundResource := &kubevirtv1.KubeVirt{}
					Expect(
						cl.Get(context.TODO(),
							types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
							foundResource),
					).To(BeNil())

					By("KV CR should contain the HC enabled managed feature gates", func() {
						Expect(foundResource.Spec.Configuration.DeveloperConfiguration).NotTo(BeNil())
						Expect(foundResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).
							To(ContainElements(kvWithHostPassthroughCPU, kvSRIOVLiveMigration))
					})
				})

				It("should not add feature gates if they are set to false", func() {
					existingResource, err := NewKubeVirt(hco)
					Expect(err).ToNot(HaveOccurred())

					hco.Spec.FeatureGates = hcov1beta1.HyperConvergedFeatureGates{
						WithHostPassthroughCPU: false,
						SRIOVLiveMigration:     false,
					}

					cl := commonTestUtils.InitClient([]runtime.Object{hco, existingResource})
					handler := (*genericOperand)(newKubevirtHandler(cl, commonTestUtils.GetScheme()))
					res := handler.ensure(req)
					Expect(res.UpgradeDone).To(BeFalse())
					Expect(res.Updated).To(BeFalse())
					Expect(res.Overwritten).To(BeFalse())
					Expect(res.Err).To(BeNil())

					foundResource := &kubevirtv1.KubeVirt{}
					Expect(
						cl.Get(context.TODO(),
							types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
							foundResource),
					).To(BeNil())

					By("KV CR should contain the HC enabled managed feature gates", func() {
						mandatoryKvFeatureGates = getMandatoryKvFeatureGates(false)
						Expect(foundResource.Spec.Configuration.DeveloperConfiguration).ToNot(BeNil())
						fgList := getKvFeatureGateList(&hco.Spec.FeatureGates)
						Expect(fgList).To(HaveLen(basicNumFgOnOpenshift))
						Expect(fgList).Should(ContainElements(hardCodeKvFgs))
						Expect(fgList).Should(ContainElements(sspConditionKvFgs))
					})
				})

				It("should not add feature gates if they are not exist", func() {
					existingResource, err := NewKubeVirt(hco)
					Expect(err).ToNot(HaveOccurred())

					hco.Spec.FeatureGates = hcov1beta1.HyperConvergedFeatureGates{}

					cl := commonTestUtils.InitClient([]runtime.Object{hco, existingResource})
					handler := (*genericOperand)(newKubevirtHandler(cl, commonTestUtils.GetScheme()))
					res := handler.ensure(req)
					Expect(res.UpgradeDone).To(BeFalse())
					Expect(res.Updated).To(BeFalse())
					Expect(res.Overwritten).To(BeFalse())
					Expect(res.Err).To(BeNil())

					foundResource := &kubevirtv1.KubeVirt{}
					Expect(
						cl.Get(context.TODO(),
							types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
							foundResource),
					).To(BeNil())

					By("KV CR should contain the HC enabled managed feature gates", func() {
						mandatoryKvFeatureGates = getMandatoryKvFeatureGates(false)
						Expect(foundResource.Spec.Configuration.DeveloperConfiguration).ToNot(BeNil())
						fgList := getKvFeatureGateList(&hco.Spec.FeatureGates)
						Expect(fgList).To(HaveLen(basicNumFgOnOpenshift))
						Expect(fgList).Should(ContainElements(hardCodeKvFgs))
						Expect(fgList).Should(ContainElements(sspConditionKvFgs))
					})
				})

				It("should keep FG if already exist", func() {
					mandatoryKvFeatureGates = getMandatoryKvFeatureGates(true)
					fgs := append(hardCodeKvFgs, kvWithHostPassthroughCPU, kvSRIOVLiveMigration)
					existingResource, err := NewKubeVirt(hco)
					Expect(err).ToNot(HaveOccurred())
					existingResource.Spec.Configuration.DeveloperConfiguration = &kubevirtv1.DeveloperConfiguration{
						FeatureGates: fgs,
					}

					hco.Spec.FeatureGates = hcov1beta1.HyperConvergedFeatureGates{
						WithHostPassthroughCPU: true,
						SRIOVLiveMigration:     true,
					}

					By("Make sure the existing KV is with the the expected FGs", func() {
						Expect(existingResource.Spec.Configuration.DeveloperConfiguration).NotTo(BeNil())
						Expect(existingResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).
							To(ContainElements(kvWithHostPassthroughCPU, kvSRIOVLiveMigration))
					})

					cl := commonTestUtils.InitClient([]runtime.Object{hco, existingResource})
					handler := (*genericOperand)(newKubevirtHandler(cl, commonTestUtils.GetScheme()))
					res := handler.ensure(req)
					Expect(res.UpgradeDone).To(BeFalse())
					Expect(res.Updated).To(BeFalse())
					Expect(res.Overwritten).To(BeFalse())
					Expect(res.Err).To(BeNil())

					foundResource := &kubevirtv1.KubeVirt{}
					Expect(
						cl.Get(context.TODO(),
							types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
							foundResource),
					).To(BeNil())

					Expect(foundResource.Spec.Configuration.DeveloperConfiguration).NotTo(BeNil())
					Expect(foundResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).
						To(ContainElements(kvWithHostPassthroughCPU, kvSRIOVLiveMigration))
				})

				It("should remove FG if it disabled in HC CR", func() {
					mandatoryKvFeatureGates = getMandatoryKvFeatureGates(false)
					existingResource, err := NewKubeVirt(hco)
					Expect(err).ToNot(HaveOccurred())
					existingResource.Spec.Configuration.DeveloperConfiguration = &kubevirtv1.DeveloperConfiguration{
						FeatureGates: []string{kvWithHostPassthroughCPU, kvSRIOVLiveMigration},
					}

					By("Make sure the existing KV is with the the expected FGs", func() {
						Expect(existingResource.Spec.Configuration.DeveloperConfiguration).ToNot(BeNil())
						Expect(existingResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).
							To(ContainElements(kvWithHostPassthroughCPU, kvSRIOVLiveMigration))
					})

					hco.Spec.FeatureGates = hcov1beta1.HyperConvergedFeatureGates{
						WithHostPassthroughCPU: false,
						SRIOVLiveMigration:     false,
					}

					cl := commonTestUtils.InitClient([]runtime.Object{hco, existingResource})
					handler := (*genericOperand)(newKubevirtHandler(cl, commonTestUtils.GetScheme()))
					res := handler.ensure(req)
					Expect(res.UpgradeDone).To(BeFalse())
					Expect(res.Updated).To(BeTrue())
					Expect(res.Overwritten).To(BeFalse())
					Expect(res.Err).To(BeNil())

					foundResource := &kubevirtv1.KubeVirt{}
					Expect(
						cl.Get(context.TODO(),
							types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
							foundResource),
					).To(BeNil())

					Expect(foundResource.Spec.Configuration.DeveloperConfiguration).ToNot(BeNil())
					Expect(foundResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(HaveLen(basicNumFgOnOpenshift))
					Expect(foundResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(ContainElements(hardCodeKvFgs))
					Expect(foundResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(ContainElements(sspConditionKvFgs))
				})

				It("should remove FG if it missing from the HC CR", func() {
					mandatoryKvFeatureGates = getMandatoryKvFeatureGates(false)
					existingResource, err := NewKubeVirt(hco)
					Expect(err).ToNot(HaveOccurred())
					existingResource.Spec.Configuration.DeveloperConfiguration = &kubevirtv1.DeveloperConfiguration{
						FeatureGates: []string{kvWithHostPassthroughCPU, kvSRIOVLiveMigration},
					}

					By("Make sure the existing KV is with the the expected FGs", func() {
						Expect(existingResource.Spec.Configuration.DeveloperConfiguration).ToNot(BeNil())
						Expect(existingResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).
							To(ContainElements(kvWithHostPassthroughCPU, kvSRIOVLiveMigration))
					})

					hco.Spec.FeatureGates = hcov1beta1.HyperConvergedFeatureGates{}

					cl := commonTestUtils.InitClient([]runtime.Object{hco, existingResource})
					handler := (*genericOperand)(newKubevirtHandler(cl, commonTestUtils.GetScheme()))
					res := handler.ensure(req)
					Expect(res.UpgradeDone).To(BeFalse())
					Expect(res.Updated).To(BeTrue())
					Expect(res.Overwritten).To(BeFalse())
					Expect(res.Err).To(BeNil())

					foundResource := &kubevirtv1.KubeVirt{}
					Expect(
						cl.Get(context.TODO(),
							types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
							foundResource),
					).To(BeNil())

					Expect(foundResource.Spec.Configuration.DeveloperConfiguration).ToNot(BeNil())
					Expect(foundResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(HaveLen(basicNumFgOnOpenshift))
					Expect(foundResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(ContainElements(hardCodeKvFgs))
					Expect(foundResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(ContainElements(sspConditionKvFgs))
				})

				It("should remove FG if it the HC CR does not contain the featureGates field", func() {
					mandatoryKvFeatureGates = getMandatoryKvFeatureGates(true)
					existingResource, err := NewKubeVirt(hco)
					Expect(err).ToNot(HaveOccurred())
					existingResource.Spec.Configuration.DeveloperConfiguration = &kubevirtv1.DeveloperConfiguration{
						FeatureGates: []string{kvWithHostPassthroughCPU, kvSRIOVLiveMigration},
					}

					By("Make sure the existing KV is with the the expected FGs", func() {
						Expect(existingResource.Spec.Configuration.DeveloperConfiguration).ToNot(BeNil())
						Expect(existingResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).
							To(ContainElements(kvWithHostPassthroughCPU, kvSRIOVLiveMigration))
					})

					hco.Spec.FeatureGates = hcov1beta1.HyperConvergedFeatureGates{}

					cl := commonTestUtils.InitClient([]runtime.Object{hco, existingResource})
					handler := (*genericOperand)(newKubevirtHandler(cl, commonTestUtils.GetScheme()))
					res := handler.ensure(req)
					Expect(res.UpgradeDone).To(BeFalse())
					Expect(res.Updated).To(BeTrue())
					Expect(res.Overwritten).To(BeFalse())
					Expect(res.Err).To(BeNil())

					foundResource := &kubevirtv1.KubeVirt{}
					Expect(
						cl.Get(context.TODO(),
							types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
							foundResource),
					).To(BeNil())

					Expect(foundResource.Spec.Configuration.DeveloperConfiguration).ToNot(BeNil())
					Expect(foundResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(HaveLen(len(hardCodeKvFgs)))
					Expect(foundResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(ContainElements(hardCodeKvFgs))
				})
			})

			Context("Test getKvFeatureGateList", func() {
				DescribeTable("Should return featureGate slice",
					func(isKVMEmulation bool, fgs *hcov1beta1.HyperConvergedFeatureGates, expectedLength int, expectedFgs [][]string) {
						mandatoryKvFeatureGates = getMandatoryKvFeatureGates(isKVMEmulation)
						fgList := getKvFeatureGateList(fgs)
						Expect(getKvFeatureGateList(fgs)).To(HaveLen(expectedLength))
						for _, expected := range expectedFgs {
							Expect(fgList).Should(ContainElements(expected))
						}
					},
					Entry("When not using kvm-emulation and FG is empty",
						false,
						&hcov1beta1.HyperConvergedFeatureGates{},
						basicNumFgOnOpenshift,
						[][]string{hardCodeKvFgs, sspConditionKvFgs},
					),
					Entry("When using kvm-emulation and FG is empty",
						true,
						&hcov1beta1.HyperConvergedFeatureGates{},
						len(hardCodeKvFgs),
						[][]string{hardCodeKvFgs},
					),
					Entry("When not using kvm-emulation and all FGs are disabled",
						false,
						&hcov1beta1.HyperConvergedFeatureGates{SRIOVLiveMigration: false, WithHostPassthroughCPU: false},
						basicNumFgOnOpenshift,
						[][]string{hardCodeKvFgs, sspConditionKvFgs},
					),
					Entry("When using kvm-emulation all FGs are disabled",
						true,
						&hcov1beta1.HyperConvergedFeatureGates{SRIOVLiveMigration: false, WithHostPassthroughCPU: false},
						len(hardCodeKvFgs),
						[][]string{hardCodeKvFgs},
					),
					Entry("When not using kvm-emulation and all FGs are enabled",
						false,
						&hcov1beta1.HyperConvergedFeatureGates{SRIOVLiveMigration: true, WithHostPassthroughCPU: true},
						basicNumFgOnOpenshift+2,
						[][]string{hardCodeKvFgs, sspConditionKvFgs, {kvWithHostPassthroughCPU}},
					),
					Entry("When using kvm-emulation all FGs are enabled",
						true,
						&hcov1beta1.HyperConvergedFeatureGates{SRIOVLiveMigration: true, WithHostPassthroughCPU: true},
						len(hardCodeKvFgs)+2,
						[][]string{hardCodeKvFgs, {kvWithHostPassthroughCPU}},
					))
			})

			Context("Test getMandatoryKvFeatureGates", func() {
				It("Should include the sspConditionKvFgs if running in openshift", func() {
					fgs := getMandatoryKvFeatureGates(false)
					Expect(fgs).To(HaveLen(basicNumFgOnOpenshift))
					Expect(fgs).To(ContainElements(hardCodeKvFgs))
					Expect(fgs).To(ContainElements(sspConditionKvFgs))
				})

				It("Should not include the sspConditionKvFgs if not running in openshift", func() {
					fgs := getMandatoryKvFeatureGates(true)
					Expect(fgs).To(HaveLen(len(hardCodeKvFgs)))
					Expect(fgs).To(ContainElements(hardCodeKvFgs))
				})
			})
		})

		Context("Obsolete CPU Models", func() {
			Context("test Obsolete CPU Models in NewKubeVirt", func() {
				It("should add obsolete CPU Models if exists in HC CR", func() {
					hco.Spec.ObsoleteCPUs = &hcov1beta1.HyperConvergedObsoleteCPUs{
						CPUModels: []string{"aaa", "bbb", "ccc"},
					}

					kv, err := NewKubeVirt(hco)
					Expect(err).ToNot(HaveOccurred())

					Expect(kv.Spec.Configuration.ObsoleteCPUModels).To(HaveLen(3 + len(hardcodedObsoleteCPUModels)))
					Expect(kv.Spec.Configuration.ObsoleteCPUModels).To(HaveKeyWithValue("aaa", true))
					Expect(kv.Spec.Configuration.ObsoleteCPUModels).To(HaveKeyWithValue("bbb", true))
					Expect(kv.Spec.Configuration.ObsoleteCPUModels).To(HaveKeyWithValue("ccc", true))
					for _, cpu := range hardcodedObsoleteCPUModels {
						Expect(kv.Spec.Configuration.ObsoleteCPUModels[cpu]).Should(BeTrue())
					}

					Expect(kv.Spec.Configuration.MinCPUModel).Should(BeEmpty())
				})

				It("should add min CPU Model if exists in HC CR", func() {
					hco.Spec.ObsoleteCPUs = &hcov1beta1.HyperConvergedObsoleteCPUs{
						MinCPUModel: "Penryn",
					}

					kv, err := NewKubeVirt(hco)
					Expect(err).ToNot(HaveOccurred())

					Expect(kv.Spec.Configuration.ObsoleteCPUModels).ShouldNot(BeEmpty())
					for _, cpu := range hardcodedObsoleteCPUModels {
						Expect(kv.Spec.Configuration.ObsoleteCPUModels[cpu]).Should(BeTrue())
					}
					Expect(kv.Spec.Configuration.MinCPUModel).Should(Equal("Penryn"))
				})

				It("should not add min CPU Model and obsolete CPU Models if HC does not contain ObsoleteCPUs", func() {
					kv, err := NewKubeVirt(hco)
					Expect(err).ToNot(HaveOccurred())

					Expect(kv.Spec.Configuration.ObsoleteCPUModels).Should(HaveLen(len(hardcodedObsoleteCPUModels)))
					for _, cpu := range hardcodedObsoleteCPUModels {
						Expect(kv.Spec.Configuration.ObsoleteCPUModels[cpu]).Should(BeTrue())
					}

					Expect(kv.Spec.Configuration.MinCPUModel).Should(BeEmpty())
				})

				It("should not add min CPU Model and add only the hard coded obsolete CPU Models if ObsoleteCPUs is empty", func() {
					hco.Spec.ObsoleteCPUs = &hcov1beta1.HyperConvergedObsoleteCPUs{}
					kv, err := NewKubeVirt(hco)
					Expect(err).ToNot(HaveOccurred())

					Expect(kv.Spec.Configuration.ObsoleteCPUModels).Should(HaveLen(len(hardcodedObsoleteCPUModels)))
					for _, cpu := range hardcodedObsoleteCPUModels {
						Expect(kv.Spec.Configuration.ObsoleteCPUModels[cpu]).Should(BeTrue())
					}

					Expect(kv.Spec.Configuration.MinCPUModel).Should(BeEmpty())
				})
			})

			Context("test Obsolete CPU Models in KV handler", func() {
				It("Should add obsolete CPU model if they are set in HC CR", func() {
					existingKV, err := NewKubeVirt(hco)
					Expect(err).ToNot(HaveOccurred())

					hco.Spec.ObsoleteCPUs = &hcov1beta1.HyperConvergedObsoleteCPUs{
						CPUModels:   []string{"aaa", "bbb", "ccc"},
						MinCPUModel: "Penryn",
					}

					cl := commonTestUtils.InitClient([]runtime.Object{hco, existingKV})
					handler := (*genericOperand)(newKubevirtHandler(cl, commonTestUtils.GetScheme()))
					res := handler.ensure(req)
					Expect(res.UpgradeDone).To(BeFalse())
					Expect(res.Updated).To(BeTrue())
					Expect(res.Overwritten).To(BeFalse())
					Expect(res.Err).To(BeNil())

					foundKV := &kubevirtv1.KubeVirt{}
					Expect(
						cl.Get(context.TODO(),
							types.NamespacedName{Name: existingKV.Name, Namespace: existingKV.Namespace},
							foundKV),
					).To(BeNil())

					By("KV CR should contain the HC obsolete CPU models and minCPUModel", func() {
						Expect(foundKV.Spec.Configuration.ObsoleteCPUModels).Should(HaveLen(3 + len(hardcodedObsoleteCPUModels)))
						Expect(foundKV.Spec.Configuration.ObsoleteCPUModels).Should(HaveKeyWithValue("aaa", true))
						Expect(foundKV.Spec.Configuration.ObsoleteCPUModels).Should(HaveKeyWithValue("bbb", true))
						Expect(foundKV.Spec.Configuration.ObsoleteCPUModels).Should(HaveKeyWithValue("ccc", true))
						for _, cpu := range hardcodedObsoleteCPUModels {
							Expect(foundKV.Spec.Configuration.ObsoleteCPUModels[cpu]).Should(BeTrue())
						}

						Expect(foundKV.Spec.Configuration.MinCPUModel).Should(Equal("Penryn"))
					})

				})

				It("Should modify obsolete CPU model if they are not the same as in HC CR", func() {
					existingKV, err := NewKubeVirt(hco)
					Expect(err).ToNot(HaveOccurred())
					existingKV.Spec.Configuration.MinCPUModel = "Haswell"
					existingKV.Spec.Configuration.ObsoleteCPUModels = map[string]bool{
						"shouldStay":      true,
						"shouldBeTrue":    false,
						"shouldBeRemoved": true,
					}

					hco.Spec.ObsoleteCPUs = &hcov1beta1.HyperConvergedObsoleteCPUs{
						CPUModels:   []string{"shouldStay", "shouldBeTrue", "newOne"},
						MinCPUModel: "Penryn",
					}

					cl := commonTestUtils.InitClient([]runtime.Object{hco, existingKV})
					handler := (*genericOperand)(newKubevirtHandler(cl, commonTestUtils.GetScheme()))
					res := handler.ensure(req)
					Expect(res.UpgradeDone).To(BeFalse())
					Expect(res.Updated).To(BeTrue())
					Expect(res.Overwritten).To(BeFalse())
					Expect(res.Err).To(BeNil())

					foundKV := &kubevirtv1.KubeVirt{}
					Expect(
						cl.Get(context.TODO(),
							types.NamespacedName{Name: existingKV.Name, Namespace: existingKV.Namespace},
							foundKV),
					).To(BeNil())

					By("KV CR should contain the HC obsolete CPU models and minCPUModel", func() {
						Expect(foundKV.Spec.Configuration.ObsoleteCPUModels).Should(HaveLen(3 + len(hardcodedObsoleteCPUModels)))
						Expect(foundKV.Spec.Configuration.ObsoleteCPUModels).Should(HaveKeyWithValue("shouldStay", true))
						Expect(foundKV.Spec.Configuration.ObsoleteCPUModels).Should(HaveKeyWithValue("shouldBeTrue", true))
						Expect(foundKV.Spec.Configuration.ObsoleteCPUModels).Should(HaveKeyWithValue("newOne", true))
						for _, cpu := range hardcodedObsoleteCPUModels {
							Expect(foundKV.Spec.Configuration.ObsoleteCPUModels[cpu]).Should(BeTrue())
						}

						Expect(foundKV.Spec.Configuration.ObsoleteCPUModels).ShouldNot(HaveKey("shouldBeRemoved"))

						Expect(foundKV.Spec.Configuration.MinCPUModel).Should(Equal("Penryn"))
					})
				})

				It("Should remove obsolete CPU model if they are not set in HC CR", func() {
					existingKV, err := NewKubeVirt(hco)
					Expect(err).ToNot(HaveOccurred())
					existingKV.Spec.Configuration.MinCPUModel = "Penryn"
					existingKV.Spec.Configuration.ObsoleteCPUModels = map[string]bool{
						"aaa": true,
						"bbb": true,
						"ccc": true,
					}

					cl := commonTestUtils.InitClient([]runtime.Object{hco, existingKV})
					handler := (*genericOperand)(newKubevirtHandler(cl, commonTestUtils.GetScheme()))
					res := handler.ensure(req)
					Expect(res.UpgradeDone).To(BeFalse())
					Expect(res.Updated).To(BeTrue())
					Expect(res.Overwritten).To(BeFalse())
					Expect(res.Err).To(BeNil())

					foundKV := &kubevirtv1.KubeVirt{}
					Expect(
						cl.Get(context.TODO(),
							types.NamespacedName{Name: existingKV.Name, Namespace: existingKV.Namespace},
							foundKV),
					).To(BeNil())

					By("KV CR ObsoleteCPUModels field should contain only the hard-coded values", func() {
						Expect(foundKV.Spec.Configuration.ObsoleteCPUModels).ShouldNot(BeEmpty())
						for _, cpu := range hardcodedObsoleteCPUModels {
							Expect(foundKV.Spec.Configuration.ObsoleteCPUModels[cpu]).Should(BeTrue())
						}
					})

					By("KV CR minCPUModel field should be empty", func() {
						Expect(foundKV.Spec.Configuration.MinCPUModel).Should(BeEmpty())
					})
				})
			})
		})

		Context("Certificate rotation strategy", func() {
			It("should add certificate rotation strategy if missing in KV", func() {
				existingResource, err := NewKubeVirt(hco)
				Expect(err).ToNot(HaveOccurred())

				hco.Spec.CertConfig = hcov1beta1.HyperConvergedCertConfig{
					CA: hcov1beta1.CertRotateConfigCA{
						Duration:    metav1.Duration{Duration: 24 * time.Hour},
						RenewBefore: metav1.Duration{Duration: 1 * time.Hour},
					},
					Server: hcov1beta1.CertRotateConfigServer{
						Duration:    metav1.Duration{Duration: 12 * time.Hour},
						RenewBefore: metav1.Duration{Duration: 30 * time.Minute},
					},
				}

				cl := commonTestUtils.InitClient([]runtime.Object{hco, existingResource})
				handler := (*genericOperand)(newKubevirtHandler(cl, commonTestUtils.GetScheme()))
				res := handler.ensure(req)
				Expect(res.UpgradeDone).To(BeFalse())
				Expect(res.Updated).To(BeTrue())
				Expect(res.Err).To(BeNil())

				foundResource := &kubevirtv1.KubeVirt{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
						foundResource),
				).To(BeNil())

				Expect(foundResource.Spec.CertificateRotationStrategy).ToNot(BeNil())
				certificateRotationStrategy := foundResource.Spec.CertificateRotationStrategy
				Expect(certificateRotationStrategy.SelfSigned.CA.Duration.Duration.String()).Should(Equal("24h0m0s"))
				Expect(certificateRotationStrategy.SelfSigned.CA.RenewBefore.Duration.String()).Should(Equal("1h0m0s"))
				Expect(certificateRotationStrategy.SelfSigned.Server.Duration.Duration.String()).Should(Equal("12h0m0s"))
				Expect(certificateRotationStrategy.SelfSigned.Server.RenewBefore.Duration.String()).Should(Equal("30m0s"))

				Expect(req.Conditions).To(BeEmpty())
			})

			It("should set certificate rotation strategy to defaults if missing in HCO CR", func() {
				existingResource := NewKubeVirtWithNameOnly(hco)

				cl := commonTestUtils.InitClient([]runtime.Object{hco})
				handler := (*genericOperand)(newKubevirtHandler(cl, commonTestUtils.GetScheme()))
				res := handler.ensure(req)
				Expect(res.UpgradeDone).To(BeFalse())
				Expect(res.Updated).To(BeFalse())
				Expect(res.Err).To(BeNil())

				foundResource := &kubevirtv1.KubeVirt{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
						foundResource),
				).To(BeNil())

				Expect(existingResource.Spec.CertificateRotationStrategy.SelfSigned).To(BeNil())

				Expect(foundResource.Spec.CertificateRotationStrategy).ToNot(BeNil())
				certificateRotationStrategy := foundResource.Spec.CertificateRotationStrategy
				Expect(certificateRotationStrategy.SelfSigned.CA.Duration.Duration.String()).Should(Equal("48h0m0s"))
				Expect(certificateRotationStrategy.SelfSigned.CA.RenewBefore.Duration.String()).Should(Equal("24h0m0s"))
				Expect(certificateRotationStrategy.SelfSigned.Server.Duration.Duration.String()).Should(Equal("24h0m0s"))
				Expect(certificateRotationStrategy.SelfSigned.Server.RenewBefore.Duration.String()).Should(Equal("12h0m0s"))

				Expect(req.Conditions).To(BeEmpty())
			})

			It("should modify certificate rotation strategy according to HCO CR", func() {

				hco.Spec.CertConfig = hcov1beta1.HyperConvergedCertConfig{
					CA: hcov1beta1.CertRotateConfigCA{
						Duration:    metav1.Duration{Duration: 24 * time.Hour},
						RenewBefore: metav1.Duration{Duration: 1 * time.Hour},
					},
					Server: hcov1beta1.CertRotateConfigServer{
						Duration:    metav1.Duration{Duration: 12 * time.Hour},
						RenewBefore: metav1.Duration{Duration: 30 * time.Minute},
					},
				}
				existingResource, err := NewKubeVirt(hco)
				Expect(err).ToNot(HaveOccurred())

				By("Modify HCO's cert configuration")
				hco.Spec.CertConfig.CA.Duration.Duration *= 2
				hco.Spec.CertConfig.CA.RenewBefore.Duration *= 2
				hco.Spec.CertConfig.Server.Duration.Duration *= 2
				hco.Spec.CertConfig.Server.RenewBefore.Duration *= 2

				cl := commonTestUtils.InitClient([]runtime.Object{hco, existingResource})
				handler := (*genericOperand)(newKubevirtHandler(cl, commonTestUtils.GetScheme()))
				res := handler.ensure(req)
				Expect(res.UpgradeDone).To(BeFalse())
				Expect(res.Updated).To(BeTrue())
				Expect(res.Err).To(BeNil())

				foundResource := &kubevirtv1.KubeVirt{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
						foundResource),
				).To(BeNil())

				Expect(existingResource.Spec.CertificateRotationStrategy).ToNot(BeNil())
				existingCertificateRotationStrategy := existingResource.Spec.CertificateRotationStrategy
				Expect(existingCertificateRotationStrategy.SelfSigned.CA.Duration.Duration.String()).Should(Equal("24h0m0s"))
				Expect(existingCertificateRotationStrategy.SelfSigned.CA.RenewBefore.Duration.String()).Should(Equal("1h0m0s"))
				Expect(existingCertificateRotationStrategy.SelfSigned.Server.Duration.Duration.String()).Should(Equal("12h0m0s"))
				Expect(existingCertificateRotationStrategy.SelfSigned.Server.RenewBefore.Duration.String()).Should(Equal("30m0s"))

				Expect(foundResource.Spec.CertificateRotationStrategy).ToNot(BeNil())
				foundCertificateRotationStrategy := foundResource.Spec.CertificateRotationStrategy
				Expect(foundCertificateRotationStrategy.SelfSigned.CA.Duration.Duration.String()).Should(Equal("48h0m0s"))
				Expect(foundCertificateRotationStrategy.SelfSigned.CA.RenewBefore.Duration.String()).Should(Equal("2h0m0s"))
				Expect(foundCertificateRotationStrategy.SelfSigned.Server.Duration.Duration.String()).Should(Equal("24h0m0s"))
				Expect(foundCertificateRotationStrategy.SelfSigned.Server.RenewBefore.Duration.String()).Should(Equal("1h0m0s"))

				Expect(req.Conditions).To(BeEmpty())
			})

			It("should overwrite certificate rotation strategy if directly set on KV CR", func() {

				hco.Spec.CertConfig = hcov1beta1.HyperConvergedCertConfig{
					CA: hcov1beta1.CertRotateConfigCA{
						Duration:    metav1.Duration{Duration: 24 * time.Hour},
						RenewBefore: metav1.Duration{Duration: 1 * time.Hour},
					},
					Server: hcov1beta1.CertRotateConfigServer{
						Duration:    metav1.Duration{Duration: 12 * time.Hour},
						RenewBefore: metav1.Duration{Duration: 30 * time.Minute},
					},
				}
				existingResource, err := NewKubeVirt(hco)
				Expect(err).ToNot(HaveOccurred())

				By("Mock a reconciliation triggered by a change in KV CR")
				req.HCOTriggered = false

				By("Modify KV's cert configuration")
				existingResource.Spec.CertificateRotationStrategy.SelfSigned.CA.Duration = &metav1.Duration{Duration: 48 * time.Hour}
				existingResource.Spec.CertificateRotationStrategy.SelfSigned.CA.RenewBefore = &metav1.Duration{Duration: 2 * time.Hour}
				existingResource.Spec.CertificateRotationStrategy.SelfSigned.Server.Duration = &metav1.Duration{Duration: 24 * time.Hour}
				existingResource.Spec.CertificateRotationStrategy.SelfSigned.Server.RenewBefore = &metav1.Duration{Duration: 1 * time.Hour}

				cl := commonTestUtils.InitClient([]runtime.Object{hco, existingResource})
				handler := (*genericOperand)(newKubevirtHandler(cl, commonTestUtils.GetScheme()))
				res := handler.ensure(req)
				Expect(res.UpgradeDone).To(BeFalse())
				Expect(res.Updated).To(BeTrue())
				Expect(res.Overwritten).To(BeTrue())
				Expect(res.Err).To(BeNil())

				foundResource := &kubevirtv1.KubeVirt{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
						foundResource),
				).To(BeNil())

				Expect(existingResource.Spec.CertificateRotationStrategy).ToNot(BeNil())
				existingCertificateRotationStrategy := existingResource.Spec.CertificateRotationStrategy
				Expect(existingCertificateRotationStrategy.SelfSigned.CA.Duration.Duration.String()).Should(Equal("48h0m0s"))
				Expect(existingCertificateRotationStrategy.SelfSigned.CA.RenewBefore.Duration.String()).Should(Equal("2h0m0s"))
				Expect(existingCertificateRotationStrategy.SelfSigned.Server.Duration.Duration.String()).Should(Equal("24h0m0s"))
				Expect(existingCertificateRotationStrategy.SelfSigned.Server.RenewBefore.Duration.String()).Should(Equal("1h0m0s"))

				Expect(foundResource.Spec.CertificateRotationStrategy).ToNot(BeNil())
				foundCertificateRotationStrategy := foundResource.Spec.CertificateRotationStrategy
				Expect(foundCertificateRotationStrategy.SelfSigned.CA.Duration.Duration.String()).Should(Equal("24h0m0s"))
				Expect(foundCertificateRotationStrategy.SelfSigned.CA.RenewBefore.Duration.String()).Should(Equal("1h0m0s"))
				Expect(foundCertificateRotationStrategy.SelfSigned.Server.Duration.Duration.String()).Should(Equal("12h0m0s"))
				Expect(foundCertificateRotationStrategy.SelfSigned.Server.RenewBefore.Duration.String()).Should(Equal("30m0s"))

				Expect(req.Conditions).To(BeEmpty())
			})
		})

		It("should handle conditions", func() {
			expectedResource, err := NewKubeVirt(hco, commonTestUtils.Namespace)
			Expect(err).ToNot(HaveOccurred())
			expectedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", expectedResource.Namespace, expectedResource.Name)
			expectedResource.Status.Conditions = []kubevirtv1.KubeVirtCondition{
				kubevirtv1.KubeVirtCondition{
					Type:    kubevirtv1.KubeVirtConditionAvailable,
					Status:  corev1.ConditionFalse,
					Reason:  "Foo",
					Message: "Bar",
				},
				kubevirtv1.KubeVirtCondition{
					Type:    kubevirtv1.KubeVirtConditionProgressing,
					Status:  corev1.ConditionTrue,
					Reason:  "Foo",
					Message: "Bar",
				},
				kubevirtv1.KubeVirtCondition{
					Type:    kubevirtv1.KubeVirtConditionDegraded,
					Status:  corev1.ConditionTrue,
					Reason:  "Foo",
					Message: "Bar",
				},
			}
			cl := commonTestUtils.InitClient([]runtime.Object{hco, expectedResource})
			handler := (*genericOperand)(newKubevirtHandler(cl, commonTestUtils.GetScheme()))
			res := handler.ensure(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			// Check HCO's status
			Expect(hco.Status.RelatedObjects).To(Not(BeNil()))
			objectRef, err := reference.GetReference(handler.Scheme, expectedResource)
			Expect(err).To(BeNil())
			// ObjectReference should have been added
			Expect(hco.Status.RelatedObjects).To(ContainElement(*objectRef))
			// Check conditions
			Expect(req.Conditions[conditionsv1.ConditionAvailable]).To(testlib.RepresentCondition(conditionsv1.Condition{
				Type:    conditionsv1.ConditionAvailable,
				Status:  corev1.ConditionFalse,
				Reason:  "KubeVirtNotAvailable",
				Message: "KubeVirt is not available: Bar",
			}))
			Expect(req.Conditions[conditionsv1.ConditionProgressing]).To(testlib.RepresentCondition(conditionsv1.Condition{
				Type:    conditionsv1.ConditionProgressing,
				Status:  corev1.ConditionTrue,
				Reason:  "KubeVirtProgressing",
				Message: "KubeVirt is progressing: Bar",
			}))
			Expect(req.Conditions[conditionsv1.ConditionUpgradeable]).To(testlib.RepresentCondition(conditionsv1.Condition{
				Type:    conditionsv1.ConditionUpgradeable,
				Status:  corev1.ConditionFalse,
				Reason:  "KubeVirtProgressing",
				Message: "KubeVirt is progressing: Bar",
			}))
			Expect(req.Conditions[conditionsv1.ConditionDegraded]).To(testlib.RepresentCondition(conditionsv1.Condition{
				Type:    conditionsv1.ConditionDegraded,
				Status:  corev1.ConditionTrue,
				Reason:  "KubeVirtDegraded",
				Message: "KubeVirt is degraded: Bar",
			}))
		})

		Context("jsonpath Annotation", func() {
			mandatoryKvFeatureGates = getMandatoryKvFeatureGates(false)
			It("Should create KV object with changes from the annotation", func() {

				hco.Annotations = map[string]string{common.JSONPatchKVAnnotationName: `[
					{
						"op": "add",
						"path": "/spec/configuration/cpuRequest",
						"value": "12m"
					},
					{
						"op": "add",
						"path": "/spec/configuration/developerConfiguration",
						"value": {"featureGates": ["fg1"]}
					},
					{
						"op": "add",
						"path": "/spec/configuration/developerConfiguration/featureGates/-",
						"value": "fg2"
					}
				]`}

				kv, err := NewKubeVirt(hco)
				Expect(err).ToNot(HaveOccurred())
				Expect(kv).ToNot(BeNil())
				Expect(kv.Spec.Configuration.DeveloperConfiguration).ToNot(BeNil())
				Expect(kv.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(HaveLen(2))
				Expect(kv.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(ContainElement("fg1"))
				Expect(kv.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(ContainElement("fg2"))
				Expect(kv.Spec.Configuration.CPURequest).ToNot(BeNil())

				quantity, err := resource.ParseQuantity("12m")
				Expect(err).ToNot(HaveOccurred())
				Expect(kv.Spec.Configuration.CPURequest).ToNot(BeNil())
				Expect(*kv.Spec.Configuration.CPURequest).Should(Equal(quantity))
			})

			It("Should fail to create KV object with wrong jsonPatch", func() {
				hco.Annotations = map[string]string{common.JSONPatchKVAnnotationName: `[
					{
						"op": "notExists",
						"path": "/spec/config/featureGates/-",
						"value": "fg1"
					}
				]`}

				_, err := NewKubeVirt(hco)
				Expect(err).To(HaveOccurred())
			})

			It("Ensure func should create KV object with changes from the annotation", func() {
				hco.Annotations = map[string]string{common.JSONPatchKVAnnotationName: `[
					{
						"op": "add",
						"path": "/spec/configuration/cpuRequest",
						"value": "12m"
					},
					{
						"op": "add",
						"path": "/spec/configuration/developerConfiguration",
						"value": {"featureGates": ["fg1"]}
					},
					{
						"op": "add",
						"path": "/spec/configuration/developerConfiguration/featureGates/-",
						"value": "fg2"
					}
				]`}

				expectedResource := NewKubeVirtWithNameOnly(hco)
				cl := commonTestUtils.InitClient([]runtime.Object{hco})
				handler := (*genericOperand)(newKubevirtHandler(cl, commonTestUtils.GetScheme()))
				res := handler.ensure(req)
				Expect(res.Created).To(BeTrue())
				Expect(res.UpgradeDone).To(BeFalse())
				Expect(res.Err).To(BeNil())

				kv := &kubevirtv1.KubeVirt{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: expectedResource.Name, Namespace: expectedResource.Namespace},
						kv),
				).ToNot(HaveOccurred())

				Expect(kv).ToNot(BeNil())
				Expect(kv.Spec.Configuration.DeveloperConfiguration).ToNot(BeNil())
				Expect(kv.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(HaveLen(2))
				Expect(kv.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(ContainElement("fg1"))
				Expect(kv.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(ContainElement("fg2"))
				Expect(kv.Spec.Configuration.CPURequest).ToNot(BeNil())

				quantity, err := resource.ParseQuantity("12m")
				Expect(err).ToNot(HaveOccurred())
				Expect(kv.Spec.Configuration.CPURequest).ToNot(BeNil())
				Expect(*kv.Spec.Configuration.CPURequest).Should(Equal(quantity))
			})

			It("Ensure func should fail to create KV object with wrong jsonPatch", func() {
				hco.Annotations = map[string]string{common.JSONPatchKVAnnotationName: `[
					{
						"op": "notExists",
						"path": "/spec/configuration/developerConfiguration",
						"value": {"featureGates": ["fg1"]}
					}
				]`}

				expectedResource := NewKubeVirtWithNameOnly(hco)
				cl := commonTestUtils.InitClient([]runtime.Object{hco})
				handler := (*genericOperand)(newKubevirtHandler(cl, commonTestUtils.GetScheme()))
				res := handler.ensure(req)
				Expect(res.Err).To(HaveOccurred())

				kv := &kubevirtv1.KubeVirt{}

				err := cl.Get(context.TODO(),
					types.NamespacedName{Name: expectedResource.Name, Namespace: expectedResource.Namespace},
					kv)

				Expect(err).To(HaveOccurred())
				Expect(errors.IsNotFound(err)).To(BeTrue())
			})

			It("Ensure func should update KV object with changes from the annotation", func() {
				existsCdi, err := NewKubeVirt(hco)
				Expect(err).ToNot(HaveOccurred())

				hco.Annotations = map[string]string{common.JSONPatchKVAnnotationName: `[
					{
						"op": "add",
						"path": "/spec/configuration/cpuRequest",
						"value": "12m"
					},
					{
						"op": "add",
						"path": "/spec/configuration/developerConfiguration",
						"value": {"featureGates": ["fg1"]}
					},
					{
						"op": "add",
						"path": "/spec/configuration/developerConfiguration/featureGates/-",
						"value": "fg2"
					}
				]`}

				cl := commonTestUtils.InitClient([]runtime.Object{hco, existsCdi})

				handler := (*genericOperand)(newKubevirtHandler(cl, commonTestUtils.GetScheme()))
				res := handler.ensure(req)
				Expect(res.Err).ToNot(HaveOccurred())
				Expect(res.Updated).To(BeTrue())
				Expect(res.UpgradeDone).To(BeFalse())

				kv := &kubevirtv1.KubeVirt{}

				expectedResource := NewKubeVirtWithNameOnly(hco)
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: expectedResource.Name, Namespace: expectedResource.Namespace},
						kv),
				).ToNot(HaveOccurred())

				Expect(kv.Spec.Configuration.DeveloperConfiguration).ToNot(BeNil())
				Expect(kv.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(HaveLen(2))
				Expect(kv.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(ContainElement("fg1"))
				Expect(kv.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(ContainElement("fg2"))
				Expect(kv.Spec.Configuration.CPURequest).ToNot(BeNil())

				quantity, err := resource.ParseQuantity("12m")
				Expect(err).ToNot(HaveOccurred())
				Expect(kv.Spec.Configuration.CPURequest).ToNot(BeNil())
				Expect(*kv.Spec.Configuration.CPURequest).Should(Equal(quantity))
			})

			It("Ensure func should fail to update KV object with wrong jsonPatch", func() {
				existsKv, err := NewKubeVirt(hco)
				Expect(err).ToNot(HaveOccurred())

				hco.Annotations = map[string]string{common.JSONPatchKVAnnotationName: `[
					{
						"op": "notExistsOp",
						"path": "/spec/configuration/cpuRequest",
						"value": "12m"
					}
				]`}

				cl := commonTestUtils.InitClient([]runtime.Object{hco, existsKv})

				handler := (*genericOperand)(newKubevirtHandler(cl, commonTestUtils.GetScheme()))
				res := handler.ensure(req)
				Expect(res.Err).To(HaveOccurred())

				kv := &kubevirtv1.KubeVirt{}

				expectedResource := NewKubeVirtWithNameOnly(hco)
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: expectedResource.Name, Namespace: expectedResource.Namespace},
						kv),
				).ToNot(HaveOccurred())

				Expect(kv.Spec.Configuration.DeveloperConfiguration).ToNot(BeNil())
				Expect(kv.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(HaveLen(len(mandatoryKvFeatureGates)))
				Expect(kv.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(ContainElements(hardCodeKvFgs))
				Expect(kv.Spec.Configuration.CPURequest).To(BeNil())

			})
		})

		Context("Cache", func() {
			cl := commonTestUtils.InitClient([]runtime.Object{})
			handler := newKubevirtHandler(cl, commonTestUtils.GetScheme())

			It("should start with empty cache", func() {
				Expect(handler.hooks.(*kubevirtHooks).cache).To(BeNil())
			})

			It("should update the cache when reading full CR", func() {
				cr, err := handler.hooks.getFullCr(hco)
				Expect(err).ToNot(HaveOccurred())
				Expect(cr).ToNot(BeNil())
				Expect(handler.hooks.(*kubevirtHooks).cache).ToNot(BeNil())

				By("compare pointers to make sure cache is working", func() {
					Expect(handler.hooks.(*kubevirtHooks).cache == cr).Should(BeTrue())

					crII, err := handler.hooks.getFullCr(hco)
					Expect(err).ToNot(HaveOccurred())
					Expect(crII).ToNot(BeNil())
					Expect(cr == crII).Should(BeTrue())
				})
			})

			It("should remove the cache on reset", func() {
				handler.hooks.(*kubevirtHooks).reset()
				Expect(handler.hooks.(*kubevirtHooks).cache).To(BeNil())
			})

			It("check that reset actually cause creating of a new cached instance", func() {
				crI, err := handler.hooks.getFullCr(hco)
				Expect(err).ToNot(HaveOccurred())
				Expect(crI).ToNot(BeNil())
				Expect(handler.hooks.(*kubevirtHooks).cache).ToNot(BeNil())

				handler.hooks.(*kubevirtHooks).reset()
				Expect(handler.hooks.(*kubevirtHooks).cache).To(BeNil())

				crII, err := handler.hooks.getFullCr(hco)
				Expect(err).ToNot(HaveOccurred())
				Expect(crII).ToNot(BeNil())
				Expect(handler.hooks.(*kubevirtHooks).cache).ToNot(BeNil())

				Expect(crI == crII).To(BeFalse())
				Expect(handler.hooks.(*kubevirtHooks).cache == crI).To(BeFalse())
				Expect(handler.hooks.(*kubevirtHooks).cache == crII).To(BeTrue())
			})
		})
	})

	Context("Test hcLiveMigrationToKv", func() {

		bandwidthPerMigration := "64Mi"
		completionTimeoutPerGiB := int64(100)
		parallelMigrationsPerCluster := uint32(100)
		parallelOutboundMigrationsPerNode := uint32(100)
		progressTimeout := int64(100)

		It("should create valid KV LM config from a valid HC LM config", func() {
			lmc := hcov1beta1.LiveMigrationConfigurations{
				BandwidthPerMigration:             &bandwidthPerMigration,
				CompletionTimeoutPerGiB:           &completionTimeoutPerGiB,
				ParallelMigrationsPerCluster:      &parallelMigrationsPerCluster,
				ParallelOutboundMigrationsPerNode: &parallelOutboundMigrationsPerNode,
				ProgressTimeout:                   &progressTimeout,
			}
			mc, err := hcLiveMigrationToKv(lmc)
			Expect(err).ToNot(HaveOccurred())

			Expect(*mc.BandwidthPerMigration).Should(Equal(resource.MustParse(bandwidthPerMigration)))
			Expect(*mc.CompletionTimeoutPerGiB).Should(Equal(completionTimeoutPerGiB))
			Expect(*mc.ParallelMigrationsPerCluster).Should(Equal(parallelMigrationsPerCluster))
			Expect(*mc.ParallelOutboundMigrationsPerNode).Should(Equal(parallelOutboundMigrationsPerNode))
			Expect(*mc.ProgressTimeout).Should(Equal(progressTimeout))
		})

		It("should create valid empty KV LM config from a valid empty HC LM config", func() {
			lmc := hcov1beta1.LiveMigrationConfigurations{}
			mc, err := hcLiveMigrationToKv(lmc)
			Expect(err).ToNot(HaveOccurred())

			Expect(mc.BandwidthPerMigration).Should(BeNil())
			Expect(mc.CompletionTimeoutPerGiB).Should(BeNil())
			Expect(mc.ParallelMigrationsPerCluster).Should(BeNil())
			Expect(mc.ParallelOutboundMigrationsPerNode).Should(BeNil())
			Expect(mc.ProgressTimeout).Should(BeNil())
		})

		It("should return error if the value of the BandwidthPerMigration field is not valid", func() {
			wrongBandwidthPerMigration := "Wrong BandwidthPerMigration"
			lmc := hcov1beta1.LiveMigrationConfigurations{
				BandwidthPerMigration:             &wrongBandwidthPerMigration,
				CompletionTimeoutPerGiB:           &completionTimeoutPerGiB,
				ParallelMigrationsPerCluster:      &parallelMigrationsPerCluster,
				ParallelOutboundMigrationsPerNode: &parallelOutboundMigrationsPerNode,
				ProgressTimeout:                   &progressTimeout,
			}
			mc, err := hcLiveMigrationToKv(lmc)
			Expect(err).To(HaveOccurred())
			Expect(mc).To(BeNil())
		})
	})

	Context("Test toKvPermittedHostDevices", func() {
		It("should return nil if the input is nil", func() {
			Expect(toKvPermittedHostDevices(nil)).To(BeNil())
		})

		It("should return an empty lists if the input is empty", func() {
			kvCopy := toKvPermittedHostDevices(&hcov1beta1.PermittedHostDevices{})
			Expect(kvCopy).ToNot(BeNil())
			Expect(kvCopy.PciHostDevices).To(BeEmpty())
			Expect(kvCopy.MediatedDevices).To(BeEmpty())
		})

		It("should copy all the values", func() {
			hcoCopy := &hcov1beta1.PermittedHostDevices{
				PciHostDevices: []hcov1beta1.PciHostDevice{
					{
						PCIVendorSelector:        "vendor1",
						ResourceName:             "resourceName1",
						ExternalResourceProvider: true,
					},
					{
						PCIVendorSelector:        "vendor2",
						ResourceName:             "resourceName2",
						ExternalResourceProvider: false,
					},
					{
						PCIVendorSelector:        "vendor3",
						ResourceName:             "resourceName3",
						ExternalResourceProvider: true,
					},
				},
				MediatedDevices: []hcov1beta1.MediatedHostDevice{
					{
						MDEVNameSelector:         "selector1",
						ResourceName:             "resource1",
						ExternalResourceProvider: true,
					},
					{
						MDEVNameSelector:         "selector2",
						ResourceName:             "resource2",
						ExternalResourceProvider: false,
					},
					{
						MDEVNameSelector:         "selector3",
						ResourceName:             "resource3",
						ExternalResourceProvider: true,
					},
					{
						MDEVNameSelector:         "selector4",
						ResourceName:             "resource4",
						ExternalResourceProvider: false,
					},
				},
			}

			kvCopy := toKvPermittedHostDevices(hcoCopy)
			Expect(kvCopy).ToNot(BeNil())

			Expect(kvCopy.PciHostDevices).To(HaveLen(3))
			Expect(kvCopy.PciHostDevices).To(ContainElements(
				kubevirtv1.PciHostDevice{
					PCIVendorSelector:        "vendor1",
					ResourceName:             "resourceName1",
					ExternalResourceProvider: true,
				},
				kubevirtv1.PciHostDevice{
					PCIVendorSelector:        "vendor2",
					ResourceName:             "resourceName2",
					ExternalResourceProvider: false,
				},
				kubevirtv1.PciHostDevice{
					PCIVendorSelector:        "vendor3",
					ResourceName:             "resourceName3",
					ExternalResourceProvider: true,
				},
			))

			Expect(kvCopy.MediatedDevices).To(HaveLen(4))
			Expect(kvCopy.MediatedDevices).To(ContainElements(
				kubevirtv1.MediatedHostDevice{
					MDEVNameSelector:         "selector1",
					ResourceName:             "resource1",
					ExternalResourceProvider: true,
				},
				kubevirtv1.MediatedHostDevice{
					MDEVNameSelector:         "selector2",
					ResourceName:             "resource2",
					ExternalResourceProvider: false,
				},
				kubevirtv1.MediatedHostDevice{
					MDEVNameSelector:         "selector3",
					ResourceName:             "resource3",
					ExternalResourceProvider: true,
				},
				kubevirtv1.MediatedHostDevice{
					MDEVNameSelector:         "selector4",
					ResourceName:             "resource4",
					ExternalResourceProvider: false,
				},
			))
		})
	})
})
