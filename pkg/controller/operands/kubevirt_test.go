package operands

import (
	"context"
	"fmt"

	"os"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/pkg/apis/hco/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/controller/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/controller/commonTestUtils"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
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
)

var _ = Describe("KubeVirt Operand", func() {

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

	Context("KubeVirt Config", func() {

		var hco *hcov1beta1.HyperConverged
		var req *common.HcoRequest

		updatableKeys := [...]string{SmbiosConfigKey, MachineTypeKey, SELinuxLauncherTypeKey, FeatureGatesKey}
		removeKeys := [...]string{MigrationsConfigKey}
		unupdatableKeys := [...]string{NetworkInterfaceKey}

		BeforeEach(func() {
			hco = commonTestUtils.NewHco()
			req = commonTestUtils.NewReq(hco)

			os.Setenv(smbiosEnvName, `Family: smbios family
Product: smbios product
Manufacturer: smbios manufacturer
Sku: 1.2.3
Version: 1.2.3`)
			os.Setenv(machineTypeEnvName, "new-machinetype-value-that-we-have-to-set")
		})

		It("should create if not present", func() {
			expectedResource := NewKubeVirtConfigForCR(req.Instance, commonTestUtils.Namespace)
			cl := commonTestUtils.InitClient([]runtime.Object{})

			handler := (*genericOperand)(newKvConfigHandler(cl, commonTestUtils.GetScheme()))
			res := handler.ensure(req)

			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			foundResource := &corev1.ConfigMap{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: expectedResource.Name, Namespace: expectedResource.Namespace},
					foundResource),
			).To(BeNil())
			Expect(foundResource.Name).To(Equal(expectedResource.Name))
			Expect(foundResource.Labels).Should(HaveKeyWithValue(hcoutil.AppLabel, commonTestUtils.Name))
			Expect(foundResource.Namespace).To(Equal(expectedResource.Namespace))
		})

		It("should find if present", func() {
			expectedResource := NewKubeVirtConfigForCR(hco, commonTestUtils.Namespace)
			expectedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", expectedResource.Namespace, expectedResource.Name)
			cl := commonTestUtils.InitClient([]runtime.Object{hco, expectedResource})
			handler := (*genericOperand)(newKvConfigHandler(cl, commonTestUtils.GetScheme()))
			res := handler.ensure(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			// Check HCO's status
			Expect(hco.Status.RelatedObjects).To(Not(BeNil()))
			objectRef, err := reference.GetReference(handler.Scheme, expectedResource)
			Expect(err).To(BeNil())
			// ObjectReference should have been added
			Expect(hco.Status.RelatedObjects).To(ContainElement(*objectRef))
		})

		It("should update only a few keys and only when in upgrade mode", func() {
			expectedResource := NewKubeVirtConfigForCR(hco, commonTestUtils.Namespace)
			expectedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", expectedResource.Namespace, expectedResource.Name)
			outdatedResource := NewKubeVirtConfigForCR(hco, commonTestUtils.Namespace)
			outdatedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", outdatedResource.Namespace, outdatedResource.Name)
			// values we should update
			outdatedResource.Data[SmbiosConfigKey] = "old-smbios-value-that-we-have-to-update"
			outdatedResource.Data[MachineTypeKey] = "old-machinetype-value-that-we-have-to-update"
			outdatedResource.Data[SELinuxLauncherTypeKey] = "old-selinuxlauncher-value-that-we-have-to-update"
			outdatedResource.Data[FeatureGatesKey] = "old-featuregates-value-that-we-have-to-update"
			// value that we should remove if configured
			outdatedResource.Data[MigrationsConfigKey] = "old-migrationsconfig-value-that-we-should-remove"
			// values we should preserve
			outdatedResource.Data[NetworkInterfaceKey] = "old-defaultnetworkinterface-value-that-we-should-preserve"

			cl := commonTestUtils.InitClient([]runtime.Object{hco, outdatedResource})
			handler := (*genericOperand)(newKvConfigHandler(cl, commonTestUtils.GetScheme()))

			// force upgrade mode
			req.UpgradeMode = true
			res := handler.ensure(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			foundResource := &corev1.ConfigMap{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: expectedResource.Name, Namespace: expectedResource.Namespace},
					foundResource),
			).To(BeNil())

			for _, k := range updatableKeys {
				Expect(foundResource.Data[k]).To(Not(Equal(outdatedResource.Data[k])))
				Expect(foundResource.Data[k]).To(Equal(expectedResource.Data[k]))
			}
			for _, k := range unupdatableKeys {
				Expect(foundResource.Data[k]).To(Equal(outdatedResource.Data[k]))
				Expect(foundResource.Data[k]).To(Not(Equal(expectedResource.Data[k])))
			}
			for _, k := range removeKeys {
				Expect(outdatedResource.Data).To(HaveKey(k))
				Expect(expectedResource.Data).To(Not(HaveKey(k)))
				Expect(foundResource.Data).To(Not(HaveKey(k)))
			}
		})

		It("should not touch it when not in in upgrade mode", func() {
			expectedResource := NewKubeVirtConfigForCR(hco, commonTestUtils.Namespace)
			expectedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", expectedResource.Namespace, expectedResource.Name)
			outdatedResource := NewKubeVirtConfigForCR(hco, commonTestUtils.Namespace)
			outdatedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", outdatedResource.Namespace, outdatedResource.Name)
			// values we should update
			outdatedResource.Data[SmbiosConfigKey] = "old-smbios-value-that-we-have-to-update"
			outdatedResource.Data[MachineTypeKey] = "old-machinetype-value-that-we-have-to-update"
			outdatedResource.Data[SELinuxLauncherTypeKey] = "old-selinuxlauncher-value-that-we-have-to-update"
			// values we should preserve
			outdatedResource.Data[MigrationsConfigKey] = "old-migrationsconfig-value-that-we-should-preserve"
			outdatedResource.Data[DefaultNetworkInterface] = "old-defaultnetworkinterface-value-that-we-should-preserve"

			cl := commonTestUtils.InitClient([]runtime.Object{hco, outdatedResource})
			handler := (*genericOperand)(newKvConfigHandler(cl, commonTestUtils.GetScheme()))

			// ensure that we are not in upgrade mode
			req.UpgradeMode = false

			res := handler.ensure(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			foundResource := &corev1.ConfigMap{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: expectedResource.Name, Namespace: expectedResource.Namespace},
					foundResource),
			).To(BeNil())

			Expect(foundResource.Data).To(Equal(outdatedResource.Data))
			Expect(foundResource.Data).To(Not(Equal(expectedResource.Data)))
		})

		Context("Feature Gates", func() {
			const cmFeatureGates = "DataVolumes,SRIOV,LiveMigration,CPUManager,CPUNodeDiscovery,Sidecar,Snapshot"

			var (
				enabled  = true
				disabled = false
			)

			It("should always set the hard-coded feature gates", func() {
				hco.Spec.FeatureGates = &hcov1beta1.HyperConvergedFeatureGates{}
				hco.Spec.FeatureGates.RebuildEnabledGateMap()

				existingResource := NewKubeVirtConfigForCR(hco, commonTestUtils.Namespace)
				Expect(existingResource.Data[FeatureGatesKey]).Should(Equal(cmFeatureGates))
			})
			Context("should handle feature gates on update", func() {
				cmFeatureGatesWithAllHCGates := fmt.Sprintf("%s,%s,%s,%s,%s", cmFeatureGates,
					HotplugVolumesGate,
					kvWithHostModelCPU,
					kvWithHostPassthroughCPU,
					kvHypervStrictCheck,
				)

				It("Should remove the non-ConfigMap FeatureGates from the CM if the FeatureGates field is nil", func() {
					existingResource := NewKubeVirtConfigForCR(hco, commonTestUtils.Namespace)
					existingResource.Data[FeatureGatesKey] = cmFeatureGatesWithAllHCGates

					hco.Spec.FeatureGates = nil
					hco.Spec.FeatureGates.RebuildEnabledGateMap()

					foundResource := &corev1.ConfigMap{}
					reconcileCm(hco, req, true, existingResource, foundResource)

					Expect(foundResource.Data[FeatureGatesKey]).Should(Equal(cmFeatureGates))
				})

				It("Should remove the non-ConfigMap FeatureGates from the CM if the FeatureGates field is empty", func() {
					existingResource := NewKubeVirtConfigForCR(hco, commonTestUtils.Namespace)
					existingResource.Data[FeatureGatesKey] = cmFeatureGatesWithAllHCGates

					hco.Spec.FeatureGates = &hcov1beta1.HyperConvergedFeatureGates{}
					hco.Spec.FeatureGates.RebuildEnabledGateMap()

					foundResource := &corev1.ConfigMap{}
					reconcileCm(hco, req, true, existingResource, foundResource)

					Expect(foundResource.Data[FeatureGatesKey]).Should(Equal(cmFeatureGates))
				})

				It("Should remove the non-ConfigMap Gates from the CM if they are disabled", func() {
					existingResource := NewKubeVirtConfigForCR(hco, commonTestUtils.Namespace)
					existingResource.Data[FeatureGatesKey] = cmFeatureGatesWithAllHCGates

					hco.Spec.FeatureGates = &hcov1beta1.HyperConvergedFeatureGates{
						HotplugVolumes:   &disabled,
						WithHostModelCPU: &enabled,
					}
					hco.Spec.FeatureGates.RebuildEnabledGateMap()

					foundResource := &corev1.ConfigMap{}
					reconcileCm(hco, req, true, existingResource, foundResource)

					Expect(foundResource.Data[FeatureGatesKey]).Should(Equal(cmFeatureGates + "," + kvWithHostModelCPU))
				})

				It("Should remove GPU the CM when its FeatureGate is disabled", func() {
					existingResource := NewKubeVirtConfigForCR(hco, commonTestUtils.Namespace)
					existingResource.Data[FeatureGatesKey] = fmt.Sprintf("%s,%s", cmFeatureGates, GPUGate)

					hco.Spec.FeatureGates = &hcov1beta1.HyperConvergedFeatureGates{
						GPU: &disabled,
					}
					hco.Spec.FeatureGates.RebuildEnabledGateMap()

					foundResource := &corev1.ConfigMap{}
					reconcileCm(hco, req, true, existingResource, foundResource)

					Expect(foundResource.Data[FeatureGatesKey]).Should(Equal(cmFeatureGates))
				})

				It("Should keep the HotplugVolumes gate from the CM if the HotplugVolumes FeatureGates is enabled", func() {
					existingResource := NewKubeVirtConfigForCR(hco, commonTestUtils.Namespace)
					existingResource.Data[FeatureGatesKey] = fmt.Sprintf("%s,%s", cmFeatureGates, HotplugVolumesGate)

					hco.Spec.FeatureGates = &hcov1beta1.HyperConvergedFeatureGates{
						HotplugVolumes: &enabled,
					}
					hco.Spec.FeatureGates.RebuildEnabledGateMap()

					foundResource := &corev1.ConfigMap{}
					reconcileCm(hco, req, false, existingResource, foundResource)

					Expect(foundResource.Data[FeatureGatesKey]).Should(Equal(fmt.Sprintf("%s,%s", cmFeatureGates, HotplugVolumesGate)))
				})

				It("Should add gates to the CM if they are enabled on the HC CR", func() {
					existingResource := NewKubeVirtConfigForCR(hco, commonTestUtils.Namespace)
					existingResource.Data[FeatureGatesKey] = cmFeatureGates

					hco.Spec.FeatureGates = &hcov1beta1.HyperConvergedFeatureGates{
						HotplugVolumes:         &enabled,
						WithHostPassthroughCPU: &enabled,
						WithHostModelCPU:       &enabled,
						HypervStrictCheck:      &enabled,
						SRIOVLiveMigration:     &enabled,
					}

					hco.Spec.FeatureGates.RebuildEnabledGateMap()

					foundResource := &corev1.ConfigMap{}
					reconcileCm(hco, req, true, existingResource, foundResource)

					Expect(foundResource.Data[FeatureGatesKey]).Should(ContainSubstring(cmFeatureGates))
					Expect(foundResource.Data[FeatureGatesKey]).Should(ContainSubstring(HotplugVolumesGate))
					Expect(foundResource.Data[FeatureGatesKey]).Should(ContainSubstring(kvWithHostPassthroughCPU))
					Expect(foundResource.Data[FeatureGatesKey]).Should(ContainSubstring(kvWithHostModelCPU))
					Expect(foundResource.Data[FeatureGatesKey]).Should(ContainSubstring(SRIOVLiveMigrationGate))
					Expect(foundResource.Data[FeatureGatesKey]).Should(ContainSubstring(kvHypervStrictCheck))
				})

				It("Should add SRIOVLiveMigration gate to the CM if SRIOVLiveMigration FeatureGate is enabled", func() {
					existingResource := NewKubeVirtConfigForCR(hco, commonTestUtils.Namespace)
					existingResource.Data[FeatureGatesKey] = cmFeatureGates

					hco.Spec.FeatureGates = &hcov1beta1.HyperConvergedFeatureGates{
						SRIOVLiveMigration: &enabled,
					}
					hco.Spec.FeatureGates.RebuildEnabledGateMap()

					foundResource := &corev1.ConfigMap{}
					reconcileCm(hco, req, true, existingResource, foundResource)

					Expect(foundResource.Data[FeatureGatesKey]).Should(ContainSubstring(cmFeatureGates))
					Expect(foundResource.Data[FeatureGatesKey]).Should(ContainSubstring(SRIOVLiveMigrationGate))
				})

				It("Should remove user modified FGs if SRIOVLiveMigration FeatureGate is enabled", func() {
					existingResource := NewKubeVirtConfigForCR(hco, commonTestUtils.Namespace)
					existingResource.Data[FeatureGatesKey] = cmFeatureGates + ",userDefinedFG"

					hco.Spec.FeatureGates = &hcov1beta1.HyperConvergedFeatureGates{
						SRIOVLiveMigration: &enabled,
					}
					hco.Spec.FeatureGates.RebuildEnabledGateMap()

					foundResource := &corev1.ConfigMap{}
					reconcileCm(hco, req, true, existingResource, foundResource)

					Expect(foundResource.Data[FeatureGatesKey]).Should(ContainSubstring(cmFeatureGates))
					Expect(foundResource.Data[FeatureGatesKey]).Should(ContainSubstring(SRIOVLiveMigrationGate))
					Expect(foundResource.Data[FeatureGatesKey]).ShouldNot(ContainSubstring("userDefinedFG"))
				})

				It("Should remove user modified FGs if GPU FeatureGate is disabled", func() {
					existingResource := NewKubeVirtConfigForCR(hco, commonTestUtils.Namespace)
					existingResource.Data[FeatureGatesKey] = cmFeatureGates + ",userDefinedFG"

					hco.Spec.FeatureGates = &hcov1beta1.HyperConvergedFeatureGates{
						GPU: &disabled,
					}

					foundResource := &corev1.ConfigMap{}
					reconcileCm(hco, req, true, existingResource, foundResource)

					Expect(foundResource.Data[FeatureGatesKey]).To(ContainSubstring(cmFeatureGates))
					Expect(foundResource.Data[FeatureGatesKey]).ToNot(ContainSubstring(GPUGate))
					Expect(foundResource.Data[FeatureGatesKey]).ToNot(ContainSubstring("userDefinedFG"))
				})
			})
		})

		Context("KubeVirt", func() {
			var hco *hcov1beta1.HyperConverged
			var req *common.HcoRequest

			defer os.Unsetenv(smbiosEnvName)
			defer os.Unsetenv(machineTypeEnvName)

			BeforeEach(func() {
				hco = commonTestUtils.NewHco()
				req = commonTestUtils.NewReq(hco)
			})

			enabled := true

			It("should create if not present", func() {
				os.Setenv(smbiosEnvName,
					`Family: smbios family
Product: smbios product
Manufacturer: smbios manufacturer
Sku: 1.2.3
Version: 1.2.3`)
				os.Setenv(machineTypeEnvName, "machine-type")
				hco.Spec.FeatureGates = &hcov1beta1.HyperConvergedFeatureGates{
					HotplugVolumes: &enabled,
				}
				hco.Spec.FeatureGates.RebuildEnabledGateMap()

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
				Expect(foundResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(HaveLen(8))
				Expect(foundResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(ContainElements(
					"DataVolumes", "SRIOV", "LiveMigration", "CPUManager", "CPUNodeDiscovery", "Sidecar", "Snapshot", "HotplugVolumes",
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
				hco.Spec.FeatureGates = &hcov1beta1.HyperConvergedFeatureGates{
					HotplugVolumes: &enabled,
				}
				hco.Spec.FeatureGates.RebuildEnabledGateMap()

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
				Expect(foundResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(HaveLen(8))
				Expect(foundResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(ContainElements(
					"DataVolumes", "SRIOV", "LiveMigration", "CPUManager", "CPUNodeDiscovery", "Sidecar", "Snapshot", "HotplugVolumes",
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

			Context("Feature Gates", func() {
				var (
					enabled  = true
					disabled = false
				)

				Context("test feature gates in NewKubeVirt", func() {
					It("should add the feature gates if they are set in HyperConverged CR", func() {
						hco.Spec.FeatureGates = &hcov1beta1.HyperConvergedFeatureGates{
							HotplugVolumes:         &enabled,
							WithHostPassthroughCPU: &enabled,
							WithHostModelCPU:       &enabled,
						}
						hco.Spec.FeatureGates.RebuildEnabledGateMap()

						existingResource, err := NewKubeVirt(hco)
						Expect(err).ToNot(HaveOccurred())
						By("KV CR should contain the HotplugVolumes feature gate", func() {
							Expect(existingResource.Spec.Configuration.DeveloperConfiguration).NotTo(BeNil())
							Expect(existingResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(ContainElements(HotplugVolumesGate, "WithHostPassthroughCPU", "WithHostModelCPU"))
						})
					})

					It("should add the HotplugVolumes feature gate if it's set in HyperConverged CR", func() {
						// one enabled, one disabled and one missing
						hco.Spec.FeatureGates = &hcov1beta1.HyperConvergedFeatureGates{
							HotplugVolumes:         &enabled,
							WithHostPassthroughCPU: &disabled,
						}
						hco.Spec.FeatureGates.RebuildEnabledGateMap()

						existingResource, err := NewKubeVirt(hco)
						Expect(err).ToNot(HaveOccurred())
						By("KV CR should contain the HotplugVolumes feature gate", func() {
							Expect(existingResource.Spec.Configuration.DeveloperConfiguration).NotTo(BeNil())
							Expect(existingResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(ContainElement(HotplugVolumesGate))
							Expect(existingResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).ToNot(ContainElement("WithHostPassthroughCPU"))
							Expect(existingResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).ToNot(ContainElement("WithHostModelCPU"))
						})
					})

					It("should add the WithHostPassthroughCPU feature gate if it's set in HyperConverged CR", func() {
						// one enabled, one disabled and one missing
						hco.Spec.FeatureGates = &hcov1beta1.HyperConvergedFeatureGates{
							WithHostPassthroughCPU: &enabled,
							WithHostModelCPU:       &disabled,
						}
						hco.Spec.FeatureGates.RebuildEnabledGateMap()

						existingResource, err := NewKubeVirt(hco)
						Expect(err).ToNot(HaveOccurred())
						By("KV CR should contain the HotplugVolumes feature gate", func() {
							Expect(existingResource.Spec.Configuration.DeveloperConfiguration).NotTo(BeNil())
							Expect(existingResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).ToNot(ContainElement(HotplugVolumesGate))
							Expect(existingResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(ContainElement("WithHostPassthroughCPU"))
							Expect(existingResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).ToNot(ContainElement("WithHostModelCPU"))
						})
					})

					It("should add the WithHostModelCPU feature gate if it's set in HyperConverged CR", func() {
						// one enabled, one disabled and one missing
						hco.Spec.FeatureGates = &hcov1beta1.HyperConvergedFeatureGates{
							HotplugVolumes:   &disabled,
							WithHostModelCPU: &enabled,
						}
						hco.Spec.FeatureGates.RebuildEnabledGateMap()

						existingResource, err := NewKubeVirt(hco)
						Expect(err).ToNot(HaveOccurred())
						By("KV CR should contain the HotplugVolumes feature gate", func() {
							Expect(existingResource.Spec.Configuration.DeveloperConfiguration).NotTo(BeNil())
							Expect(existingResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).ToNot(ContainElement(HotplugVolumesGate))
							Expect(existingResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).ToNot(ContainElement("WithHostPassthroughCPU"))
							Expect(existingResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(ContainElement("WithHostModelCPU"))
						})
					})

					It("should add the SRIOVLiveMigration feature gate if it's set in HyperConverged CR", func() {
						hco.Spec.FeatureGates = &hcov1beta1.HyperConvergedFeatureGates{
							SRIOVLiveMigration: &enabled,
						}
						hco.Spec.FeatureGates.RebuildEnabledGateMap()

						existingResource, err := NewKubeVirt(hco)
						Expect(err).ToNot(HaveOccurred())
						By("KV CR should contain the SRIOVLiveMigration feature gate", func() {
							Expect(existingResource.Spec.Configuration.DeveloperConfiguration).NotTo(BeNil())
							Expect(existingResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).
								To(ContainElement(SRIOVLiveMigrationGate))
						})
					})

					It("should add the GPU feature gate if it's set in HyperConverged CR", func() {
						hco.Spec.FeatureGates = &hcov1beta1.HyperConvergedFeatureGates{
							GPU: &enabled,
						}
						hco.Spec.FeatureGates.RebuildEnabledGateMap()

						existingResource, err := NewKubeVirt(hco)
						Expect(err).ToNot(HaveOccurred())
						By("KV CR should contain the GPU feature gate", func() {
							Expect(existingResource.Spec.Configuration.DeveloperConfiguration).NotTo(BeNil())
							Expect(existingResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).
								To(ContainElement(GPUGate))
						})
					})

					It("should add the HostDevices feature gate if it's set in HyperConverged CR", func() {
						hco.Spec.FeatureGates = &hcov1beta1.HyperConvergedFeatureGates{
							HostDevices: &enabled,
						}
						hco.Spec.FeatureGates.RebuildEnabledGateMap()

						existingResource, err := NewKubeVirt(hco)
						Expect(err).ToNot(HaveOccurred())
						By("KV CR should contain the GPU feature gate", func() {
							Expect(existingResource.Spec.Configuration.DeveloperConfiguration).NotTo(BeNil())
							Expect(existingResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).
								To(ContainElement(HostDevicesGate))
						})
					})

					It("should not add the feature gates if they are disabled in HyperConverged CR", func() {
						hco.Spec.FeatureGates = &hcov1beta1.HyperConvergedFeatureGates{
							HotplugVolumes:         &disabled,
							WithHostPassthroughCPU: &disabled,
							WithHostModelCPU:       &disabled,
							HypervStrictCheck:      &disabled,
						}
						hco.Spec.FeatureGates.RebuildEnabledGateMap()

						existingResource, err := NewKubeVirt(hco)
						Expect(err).ToNot(HaveOccurred())
						By("KV CR should contain only the hard coded feature gates", func() {
							Expect(existingResource.Spec.Configuration.DeveloperConfiguration).ToNot(BeNil())
							Expect(existingResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(HaveLen(len(hardCodeKvFgs)))
							Expect(existingResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(ContainElements(hardCodeKvFgs))
						})
					})

					It("should not add the feature gates if FeatureGates field is empty", func() {
						hco.Spec.FeatureGates = &hcov1beta1.HyperConvergedFeatureGates{}
						hco.Spec.FeatureGates.RebuildEnabledGateMap()

						existingResource, err := NewKubeVirt(hco)
						Expect(err).ToNot(HaveOccurred())
						By("KV CR should contain only the hard coded feature gates", func() {
							Expect(existingResource.Spec.Configuration.DeveloperConfiguration).ToNot(BeNil())
							Expect(existingResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(HaveLen(len(hardCodeKvFgs)))
							Expect(existingResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(ContainElements(hardCodeKvFgs))
						})
					})

					It("should not add the feature gates if FeatureGates field is not exist", func() {
						hco.Spec.FeatureGates = nil

						existingResource, err := NewKubeVirt(hco)
						Expect(err).ToNot(HaveOccurred())
						By("KV CR should contain only the hard coded feature gates", func() {
							Expect(existingResource.Spec.Configuration.DeveloperConfiguration).ToNot(BeNil())
							Expect(existingResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(HaveLen(len(hardCodeKvFgs)))
							Expect(existingResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(ContainElements(hardCodeKvFgs))
						})
					})
				})

				Context("test feature gates in KV handler", func() {
					It("should add feature gates if they are set to true", func() {
						existingResource, err := NewKubeVirt(hco)
						Expect(err).ToNot(HaveOccurred())

						hco.Spec.FeatureGates = &hcov1beta1.HyperConvergedFeatureGates{
							HotplugVolumes: &enabled,
						}
						hco.Spec.FeatureGates.RebuildEnabledGateMap()

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
							Expect(foundResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(ContainElement(HotplugVolumesGate))
						})
					})

					It("should not add feature gates if they are set to false", func() {
						existingResource, err := NewKubeVirt(hco)
						Expect(err).ToNot(HaveOccurred())

						hco.Spec.FeatureGates = &hcov1beta1.HyperConvergedFeatureGates{
							HotplugVolumes: &disabled,
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
							Expect(foundResource.Spec.Configuration.DeveloperConfiguration).ToNot(BeNil())
							Expect(foundResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(HaveLen(len(hardCodeKvFgs)))
							Expect(foundResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(ContainElements(hardCodeKvFgs))
						})
					})

					It("should not add feature gates if they are not exist", func() {
						existingResource, err := NewKubeVirt(hco)
						Expect(err).ToNot(HaveOccurred())

						hco.Spec.FeatureGates = &hcov1beta1.HyperConvergedFeatureGates{}

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
							Expect(foundResource.Spec.Configuration.DeveloperConfiguration).ToNot(BeNil())
							Expect(foundResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(HaveLen(len(hardCodeKvFgs)))
							Expect(foundResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(ContainElements(hardCodeKvFgs))
						})
					})

					It("should not add feature gates if the FeatureGates field is not exist", func() {
						existingResource, err := NewKubeVirt(hco)
						Expect(err).ToNot(HaveOccurred())

						hco.Spec.FeatureGates = nil

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
							Expect(foundResource.Spec.Configuration.DeveloperConfiguration).ToNot(BeNil())
							Expect(foundResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(HaveLen(len(hardCodeKvFgs)))
							Expect(foundResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(ContainElements(hardCodeKvFgs))
						})
					})

					It("should keep FG if already exist", func() {
						existingResource, err := NewKubeVirt(hco)
						Expect(err).ToNot(HaveOccurred())
						fgs := append(hardCodeKvFgs, HotplugVolumesGate)

						existingResource.Spec.Configuration.DeveloperConfiguration = &kubevirtv1.DeveloperConfiguration{
							FeatureGates: fgs,
						}

						hco.Spec.FeatureGates = &hcov1beta1.HyperConvergedFeatureGates{
							HotplugVolumes: &enabled,
						}
						hco.Spec.FeatureGates.RebuildEnabledGateMap()

						By("Make sure the existing KV is with the the expected FGs", func() {
							Expect(existingResource.Spec.Configuration.DeveloperConfiguration).NotTo(BeNil())
							Expect(existingResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(ContainElement(HotplugVolumesGate))
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
						Expect(foundResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(ContainElement(HotplugVolumesGate))
					})

					It("should remove FG if it disabled in HC CR", func() {
						existingResource, err := NewKubeVirt(hco)
						Expect(err).ToNot(HaveOccurred())
						existingResource.Spec.Configuration.DeveloperConfiguration = &kubevirtv1.DeveloperConfiguration{
							FeatureGates: []string{HotplugVolumesGate},
						}

						By("Make sure the existing KV is with the the expected FGs", func() {
							Expect(existingResource.Spec.Configuration.DeveloperConfiguration).ToNot(BeNil())
							Expect(existingResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(ContainElement(HotplugVolumesGate))
						})

						hco.Spec.FeatureGates = &hcov1beta1.HyperConvergedFeatureGates{
							HotplugVolumes: &disabled,
						}
						hco.Spec.FeatureGates.RebuildEnabledGateMap()

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
						Expect(foundResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).ToNot(ContainElement(HotplugVolumesGate))
					})

					It("should remove FG if it missing from the HC CR", func() {
						existingResource, err := NewKubeVirt(hco)
						Expect(err).ToNot(HaveOccurred())
						existingResource.Spec.Configuration.DeveloperConfiguration = &kubevirtv1.DeveloperConfiguration{
							FeatureGates: []string{HotplugVolumesGate},
						}

						By("Make sure the existing KV is with the the expected FGs", func() {
							Expect(existingResource.Spec.Configuration.DeveloperConfiguration).ToNot(BeNil())
							Expect(existingResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(ContainElement(HotplugVolumesGate))
						})

						hco.Spec.FeatureGates = &hcov1beta1.HyperConvergedFeatureGates{}

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

					It("should remove FG if it the HC CR does not contain the featureGates field", func() {
						existingResource, err := NewKubeVirt(hco)
						Expect(err).ToNot(HaveOccurred())
						existingResource.Spec.Configuration.DeveloperConfiguration = &kubevirtv1.DeveloperConfiguration{
							FeatureGates: []string{HotplugVolumesGate},
						}

						By("Make sure the existing KV is with the the expected FGs", func() {
							Expect(existingResource.Spec.Configuration.DeveloperConfiguration).ToNot(BeNil())
							Expect(existingResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(ContainElement(HotplugVolumesGate))
						})

						hco.Spec.FeatureGates = &hcov1beta1.HyperConvergedFeatureGates{
							HotplugVolumes: &disabled,
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
						Expect(foundResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(HaveLen(len(hardCodeKvFgs)))
						Expect(foundResource.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(ContainElements(hardCodeKvFgs))
					})
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
					Expect(kv.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(HaveLen(len(hardCodeKvFgs)))
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

			Context("Test getKVDevConfig", func() {
				origUseEmulation := os.Getenv(kvmEmulationEnvName)
				defer os.Setenv(kvmEmulationEnvName, origUseEmulation)

				It("should return only the hard code FGs if there are no FG in the CR", func() {
					cfg, err := getKVDevConfig(hco)
					Expect(err).ToNot(HaveOccurred())
					Expect(cfg).ToNot(BeNil())
					Expect(cfg.FeatureGates).To(HaveLen(len(hardCodeKvFgs)))
					Expect(cfg.FeatureGates).To(ContainElements(hardCodeKvFgs))
					Expect(cfg.UseEmulation).To(BeFalse())
				})

				It("should return only the hard code FGs if there are no FG in the CR and KVM_EMULATION is set to false", func() {
					os.Setenv(kvmEmulationEnvName, "false")
					cfg, err := getKVDevConfig(hco)
					Expect(err).ToNot(HaveOccurred())
					Expect(cfg.FeatureGates).To(HaveLen(len(hardCodeKvFgs)))
					Expect(cfg.FeatureGates).To(ContainElements(hardCodeKvFgs))
					Expect(cfg.UseEmulation).To(BeFalse())
				})

				It("should return error KVM_EMULATION is not set to wrong value", func() {
					os.Setenv(kvmEmulationEnvName, "fAlSe")
					cfg, err := getKVDevConfig(hco)
					Expect(err).To(HaveOccurred())
					Expect(cfg).To(BeNil())
				})

				It("should populate both fields if no FG in the CR and KVM_EMULATION is set to true", func() {
					os.Setenv(kvmEmulationEnvName, "true")
					cfg, err := getKVDevConfig(hco)
					Expect(err).ToNot(HaveOccurred())
					Expect(cfg.FeatureGates).To(HaveLen(len(hardCodeKvFgs)))
					Expect(cfg.FeatureGates).To(ContainElements(hardCodeKvFgs))
					Expect(cfg.UseEmulation).To(BeTrue())
				})

				It("should populate both fields if there are FGs in the CR and KVM_EMULATION is set to true", func() {
					enabled := true
					disabled := false

					os.Setenv(kvmEmulationEnvName, "true")
					hco.Spec.FeatureGates = &hcov1beta1.HyperConvergedFeatureGates{
						SRIOVLiveMigration: &enabled,
						HotplugVolumes:     &disabled,
						HostDevices:        &enabled,
					}
					hco.Spec.FeatureGates.RebuildEnabledGateMap()

					cfg, err := getKVDevConfig(hco)
					Expect(err).ToNot(HaveOccurred())
					Expect(cfg.FeatureGates).To(HaveLen(len(hardCodeKvFgs) + 2))
					Expect(cfg.FeatureGates).To(ContainElements(hardCodeKvFgs))
					Expect(cfg.FeatureGates).To(ContainElements("SRIOVLiveMigration", "HostDevices"))
					Expect(cfg.FeatureGates).ToNot(ContainElement("HotplugVolumes"))
					Expect(cfg.UseEmulation).To(BeTrue())
				})
			})
		})

		Context("Test getKvFeatureGateList", func() {

			enabled := true
			disabled := false

			It("Should create a slice only with hard-coded FGs if HyperConvergedFeatureGates is nil", func() {
				var fgs *hcov1beta1.HyperConvergedFeatureGates = nil
				Expect(getKvFeatureGateList(fgs)).To(HaveLen(len(hardCodeKvFgs)))
				Expect(getKvFeatureGateList(fgs)).To(ContainElements(hardCodeKvFgs))
			})

			It("Should create a slice only with hard-coded FGs if no FG exists", func() {
				fgs := &hcov1beta1.HyperConvergedFeatureGates{}
				Expect(getKvFeatureGateList(fgs)).To(HaveLen(len(hardCodeKvFgs)))
				Expect(getKvFeatureGateList(fgs)).To(ContainElements(hardCodeKvFgs))
			})

			It("Should create a slice only with hard-coded FGs if no FG is enabled", func() {
				fgs := &hcov1beta1.HyperConvergedFeatureGates{
					HotplugVolumes: &disabled,
				}
				Expect(getKvFeatureGateList(fgs)).To(HaveLen(len(hardCodeKvFgs)))
				Expect(getKvFeatureGateList(fgs)).To(ContainElements(hardCodeKvFgs))
			})

			It("Should create a slice if HotplugVolumes gate is enabled", func() {
				fgs := &hcov1beta1.HyperConvergedFeatureGates{
					HotplugVolumes: &enabled,
				}

				fgs.RebuildEnabledGateMap()
				fgList := getKvFeatureGateList(fgs)
				Expect(fgList).To(HaveLen(1 + len(hardCodeKvFgs)))
				Expect(fgList).To(ContainElement(HotplugVolumesGate))
				Expect(fgList).To(ContainElements(hardCodeKvFgs))
			})

			It("Should create a slice if WithHostPassthroughCPU gate is enabled", func() {
				fgs := &hcov1beta1.HyperConvergedFeatureGates{
					WithHostPassthroughCPU: &enabled,
				}
				fgs.RebuildEnabledGateMap()

				fgList := getKvFeatureGateList(fgs)
				Expect(fgList).To(HaveLen(1 + len(hardCodeKvFgs)))
				Expect(fgList).To(ContainElement(kvWithHostPassthroughCPU))
				Expect(fgList).To(ContainElements(hardCodeKvFgs))
			})

			It("Should create a slice if WithHostModelCPU gate is enabled", func() {
				enabled := true
				fgs := &hcov1beta1.HyperConvergedFeatureGates{
					WithHostModelCPU: &enabled,
				}
				fgs.RebuildEnabledGateMap()

				fgList := getKvFeatureGateList(fgs)
				Expect(fgList).To(HaveLen(1 + len(hardCodeKvFgs)))
				Expect(fgList).To(ContainElement(kvWithHostModelCPU))
				Expect(fgList).To(ContainElements(hardCodeKvFgs))
			})

			It("Should create a slice if SRIOVLiveMigration gate is enabled", func() {
				fgs := &hcov1beta1.HyperConvergedFeatureGates{
					SRIOVLiveMigration: &enabled,
				}
				fgs.RebuildEnabledGateMap()

				fgList := getKvFeatureGateList(fgs)
				Expect(fgList).To(HaveLen(1 + len(hardCodeKvFgs)))
				Expect(fgList).To(ContainElement(SRIOVLiveMigrationGate))
				Expect(fgList).To(ContainElements(hardCodeKvFgs))
			})

			It("Should create a slice if HypervStrictCheck gate is enabled", func() {
				enabled := true
				fgs := &hcov1beta1.HyperConvergedFeatureGates{
					HypervStrictCheck: &enabled,
				}
				fgs.RebuildEnabledGateMap()

				fgList := getKvFeatureGateList(fgs)
				Expect(fgList).To(HaveLen(1 + len(hardCodeKvFgs)))
				Expect(fgList).To(ContainElements(hardCodeKvFgs))
				Expect(fgList).To(ContainElement(kvHypervStrictCheck))
			})

			It("Should create a slice when all gates are enabled", func() {
				enabled := true
				fgs := &hcov1beta1.HyperConvergedFeatureGates{
					HotplugVolumes:         &enabled,
					WithHostPassthroughCPU: &enabled,
					WithHostModelCPU:       &enabled,
					HypervStrictCheck:      &enabled,
					SRIOVLiveMigration:     &enabled,
				}
				fgs.RebuildEnabledGateMap()

				fgList := getKvFeatureGateList(fgs)
				Expect(fgList).To(HaveLen(5 + len(hardCodeKvFgs)))
				Expect(fgList).To(ContainElements(hardCodeKvFgs))
				Expect(fgList).To(ContainElements(HotplugVolumesGate, kvWithHostPassthroughCPU, kvWithHostModelCPU, kvHypervStrictCheck, SRIOVLiveMigrationGate))
			})

			It("Should create a slice when part of gates are enabled", func() {
				enabled := true
				disabled := false
				fgs := &hcov1beta1.HyperConvergedFeatureGates{
					HotplugVolumes:         &enabled,
					WithHostPassthroughCPU: &disabled,
					WithHostModelCPU:       &enabled,
					HypervStrictCheck:      &disabled,
					SRIOVLiveMigration:     &enabled,
				}
				fgs.RebuildEnabledGateMap()

				fgList := getKvFeatureGateList(fgs)
				Expect(fgList).To(HaveLen(3 + len(hardCodeKvFgs)))
				Expect(fgList).Should(ContainElements(hardCodeKvFgs))
				Expect(fgList).Should(ContainElements(HotplugVolumesGate, kvWithHostModelCPU, SRIOVLiveMigrationGate))
				Expect(fgList).ShouldNot(ContainElements(kvWithHostPassthroughCPU, kvHypervStrictCheck))
			})
		})

		Context("Test kvFGListChanged", func() {
			enabled := true
			disabled := false

			It("should return false if both lists are empty", func() {
				Expect(kvFGListChanged(nil, nil)).Should(BeFalse())
				Expect(kvFGListChanged(nil, []string{})).Should(BeFalse())
				fgs := &hcov1beta1.HyperConvergedFeatureGates{}
				fgs.RebuildEnabledGateMap()
				Expect(kvFGListChanged(fgs, nil)).Should(BeFalse())
				Expect(kvFGListChanged(fgs, []string{})).Should(BeFalse())
			})

			It("Should return false if the list are identical", func() {
				fgs := &hcov1beta1.HyperConvergedFeatureGates{
					SRIOVLiveMigration:     &enabled,
					WithHostModelCPU:       &disabled,
					WithHostPassthroughCPU: &enabled,
				}
				fgs.RebuildEnabledGateMap()

				By("same order", func() {
					Expect(kvFGListChanged(fgs, []string{"WithHostPassthroughCPU", "SRIOVLiveMigration"})).To(BeFalse())
				})

				By("different order", func() {
					Expect(kvFGListChanged(fgs, []string{"SRIOVLiveMigration", "WithHostPassthroughCPU"})).To(BeFalse())
				})

			})

			It("Should return true if the list are not the same", func() {
				fgs := &hcov1beta1.HyperConvergedFeatureGates{
					SRIOVLiveMigration:     &enabled,
					WithHostModelCPU:       &disabled,
					WithHostPassthroughCPU: &enabled,
				}
				fgs.RebuildEnabledGateMap()

				By("different lengths", func() {
					Expect(kvFGListChanged(fgs, []string{"SRIOVLiveMigration", "WithHostModelCPU", "WithHostPassthroughCPU"})).To(BeTrue())
				})

				By("empty List", func() {
					Expect(kvFGListChanged(fgs, nil)).To(BeTrue())
					Expect(kvFGListChanged(nil, []string{"DataVolumes", "LiveMigration", "CPUManager"})).To(BeTrue())
				})

				By("Different values", func() {
					Expect(kvFGListChanged(fgs, []string{"DataVolumes", "CPUManager"})).To(BeTrue())
				})

			})
		})
	})
})

func reconcileCm(hco *hcov1beta1.HyperConverged, req *common.HcoRequest, expectUpdate bool, existingCM, foundCm *corev1.ConfigMap) {
	cl := commonTestUtils.InitClient([]runtime.Object{hco, existingCM})
	handler := (*genericOperand)(newKvConfigHandler(cl, commonTestUtils.GetScheme()))
	res := handler.ensure(req)
	if expectUpdate {
		ExpectWithOffset(1, res.Updated).To(BeTrue())
	} else {
		ExpectWithOffset(1, res.Updated).To(BeFalse())
	}
	ExpectWithOffset(1, res.Err).ToNot(HaveOccurred())

	ExpectWithOffset(1,
		cl.Get(context.TODO(),
			types.NamespacedName{Name: existingCM.Name, Namespace: existingCM.Namespace},
			foundCm),
	).To(BeNil())
}
