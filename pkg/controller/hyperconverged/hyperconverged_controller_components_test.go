package hyperconverged

import (
	"os"
	"reflect"

	networkaddonsshared "github.com/kubevirt/cluster-network-addons-operator/pkg/apis/networkaddonsoperator/shared"
	networkaddonsv1 "github.com/kubevirt/cluster-network-addons-operator/pkg/apis/networkaddonsoperator/v1"
	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/pkg/apis/hco/v1beta1"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
	sspv1 "github.com/kubevirt/kubevirt-ssp-operator/pkg/apis/kubevirt/v1"
	vmimportv1beta1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1beta1"
	consolev1 "github.com/openshift/api/console/v1"
	conditionsv1 "github.com/openshift/custom-resource-status/conditions/v1"
	"github.com/openshift/custom-resource-status/testlib"
	corev1 "k8s.io/api/core/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kubevirtv1 "kubevirt.io/client-go/api/v1"
	cdiv1beta1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	virtconfig "kubevirt.io/kubevirt/pkg/virt-config"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"context"
	"fmt"

	"k8s.io/client-go/tools/reference"
)

var _ = Describe("HyperConverged Components", func() {

	Context("KubeVirt Priority Classes", func() {

		var hco *hcov1beta1.HyperConverged
		var req *hcoRequest

		BeforeEach(func() {
			hco = newHco()
			req = newReq(hco)
		})

		It("should create if not present", func() {
			expectedResource := hco.NewKubeVirtPriorityClass()
			cl := initClient([]runtime.Object{})
			r := initReconciler(cl)
			res := r.ensureKubeVirtPriorityClass(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			key, err := client.ObjectKeyFromObject(expectedResource)
			Expect(err).ToNot(HaveOccurred())
			foundResource := &schedulingv1.PriorityClass{}
			Expect(cl.Get(context.TODO(), key, foundResource)).To(BeNil())
			Expect(foundResource.Name).To(Equal(expectedResource.Name))
			Expect(foundResource.Value).To(Equal(expectedResource.Value))
			Expect(foundResource.GlobalDefault).To(Equal(expectedResource.GlobalDefault))
		})

		It("should do nothing if already exists", func() {
			expectedResource := hco.NewKubeVirtPriorityClass()
			cl := initClient([]runtime.Object{expectedResource})
			r := initReconciler(cl)
			res := r.ensureKubeVirtPriorityClass(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			objectRef, err := reference.GetReference(r.scheme, expectedResource)
			Expect(err).To(BeNil())
			Expect(hco.Status.RelatedObjects).To(ContainElement(*objectRef))
		})

		DescribeTable("should update if something changed", func(modifiedResource *schedulingv1.PriorityClass) {
			cl := initClient([]runtime.Object{modifiedResource})
			r := initReconciler(cl)
			res := r.ensureKubeVirtPriorityClass(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			expectedResource := hco.NewKubeVirtPriorityClass()
			key, err := client.ObjectKeyFromObject(expectedResource)
			Expect(err).ToNot(HaveOccurred())
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
		var req *hcoRequest

		updatableKeys := [...]string{virtconfig.SmbiosConfigKey, virtconfig.MachineTypeKey, virtconfig.SELinuxLauncherTypeKey, virtconfig.FeatureGatesKey}
		unupdatableKeys := [...]string{virtconfig.MigrationsConfigKey, virtconfig.NetworkInterfaceKey}

		BeforeEach(func() {
			hco = newHco()
			req = newReq(hco)

			os.Setenv("SMBIOS", "new-smbios-value-that-we-have-to-set")
			os.Setenv("MACHINETYPE", "new-machinetype-value-that-we-have-to-set")
		})

		It("should create if not present", func() {
			expectedResource := newKubeVirtConfigForCR(req.instance, namespace)
			cl := initClient([]runtime.Object{})
			r := initReconciler(cl)
			res := r.ensureKubeVirtConfig(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			foundResource := &corev1.ConfigMap{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: expectedResource.Name, Namespace: expectedResource.Namespace},
					foundResource),
			).To(BeNil())
			Expect(foundResource.Name).To(Equal(expectedResource.Name))
			Expect(foundResource.Labels).Should(HaveKeyWithValue(hcoutil.AppLabel, name))
			Expect(foundResource.Namespace).To(Equal(expectedResource.Namespace))
		})

		It("should find if present", func() {
			expectedResource := newKubeVirtConfigForCR(hco, namespace)
			expectedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", expectedResource.Namespace, expectedResource.Name)
			cl := initClient([]runtime.Object{hco, expectedResource})
			r := initReconciler(cl)
			res := r.ensureKubeVirtConfig(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			// Check HCO's status
			Expect(hco.Status.RelatedObjects).To(Not(BeNil()))
			objectRef, err := reference.GetReference(r.scheme, expectedResource)
			Expect(err).To(BeNil())
			// ObjectReference should have been added
			Expect(hco.Status.RelatedObjects).To(ContainElement(*objectRef))
		})

		It("should update only a few keys and only when in upgrade mode", func() {
			expectedResource := newKubeVirtConfigForCR(hco, namespace)
			expectedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", expectedResource.Namespace, expectedResource.Name)
			outdatedResource := newKubeVirtConfigForCR(hco, namespace)
			outdatedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", outdatedResource.Namespace, outdatedResource.Name)
			// values we should update
			outdatedResource.Data[virtconfig.SmbiosConfigKey] = "old-smbios-value-that-we-have-to-update"
			outdatedResource.Data[virtconfig.MachineTypeKey] = "old-machinetype-value-that-we-have-to-update"
			outdatedResource.Data[virtconfig.SELinuxLauncherTypeKey] = "old-selinuxlauncher-value-that-we-have-to-update"
			outdatedResource.Data[virtconfig.FeatureGatesKey] = "old-featuregates-value-that-we-have-to-update"
			// values we should preserve
			outdatedResource.Data[virtconfig.MigrationsConfigKey] = "old-migrationsconfig-value-that-we-should-preserve"
			outdatedResource.Data[virtconfig.NetworkInterfaceKey] = "old-defaultnetworkinterface-value-that-we-should-preserve"

			cl := initClient([]runtime.Object{hco, outdatedResource})
			r := initReconciler(cl)

			// force upgrade mode
			r.upgradeMode = true
			res := r.ensureKubeVirtConfig(req)
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
		})

		It("should not touch it when not in in upgrade mode", func() {
			expectedResource := newKubeVirtConfigForCR(hco, namespace)
			expectedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", expectedResource.Namespace, expectedResource.Name)
			outdatedResource := newKubeVirtConfigForCR(hco, namespace)
			outdatedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", outdatedResource.Namespace, outdatedResource.Name)
			// values we should update
			outdatedResource.Data[virtconfig.SmbiosConfigKey] = "old-smbios-value-that-we-have-to-update"
			outdatedResource.Data[virtconfig.MachineTypeKey] = "old-machinetype-value-that-we-have-to-update"
			outdatedResource.Data[virtconfig.SELinuxLauncherTypeKey] = "old-selinuxlauncher-value-that-we-have-to-update"
			outdatedResource.Data[virtconfig.FeatureGatesKey] = "old-featuregates-value-that-we-have-to-update"
			// values we should preserve
			outdatedResource.Data[virtconfig.MigrationsConfigKey] = "old-migrationsconfig-value-that-we-should-preserve"
			outdatedResource.Data[virtconfig.DefaultNetworkInterface] = "old-defaultnetworkinterface-value-that-we-should-preserve"

			cl := initClient([]runtime.Object{hco, outdatedResource})
			r := initReconciler(cl)

			// ensure that we are not in upgrade mode
			r.upgradeMode = false

			res := r.ensureKubeVirtConfig(req)
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
	})

	Context("KubeVirt Storage Config", func() {
		var hco *hcov1beta1.HyperConverged
		var req *hcoRequest

		BeforeEach(func() {
			hco = newHco()
			req = newReq(hco)
		})

		It("should create if not present", func() {
			expectedResource := newKubeVirtStorageConfigForCR(hco, namespace)
			cl := initClient([]runtime.Object{})
			r := initReconciler(cl)
			err := r.ensureKubeVirtStorageConfig(req)
			Expect(err).To(BeNil())

			foundResource := &corev1.ConfigMap{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: expectedResource.Name, Namespace: expectedResource.Namespace},
					foundResource),
			).To(BeNil())
			Expect(foundResource.Name).To(Equal(expectedResource.Name))
			Expect(foundResource.Labels).Should(HaveKeyWithValue(hcoutil.AppLabel, name))
			Expect(foundResource.Namespace).To(Equal(expectedResource.Namespace))
		})

		It("should find if present", func() {
			expectedResource := newKubeVirtStorageConfigForCR(hco, namespace)
			expectedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", expectedResource.Namespace, expectedResource.Name)
			cl := initClient([]runtime.Object{hco, expectedResource})
			r := initReconciler(cl)
			err := r.ensureKubeVirtStorageConfig(req)
			Expect(err).To(BeNil())

			// Check HCO's status
			Expect(hco.Status.RelatedObjects).To(Not(BeNil()))
			objectRef, err := reference.GetReference(r.scheme, expectedResource)
			Expect(err).To(BeNil())
			// ObjectReference should have been added
			Expect(hco.Status.RelatedObjects).To(ContainElement(*objectRef))
		})

		It("volumeMode should be filesystem when platform is baremetal", func() {
			hco.Spec.BareMetalPlatform = true

			expectedResource := newKubeVirtStorageConfigForCR(hco, namespace)
			Expect(expectedResource.Data["volumeMode"]).To(Equal("Filesystem"))
		})

		It("volumeMode should be filesystem when platform is not baremetal", func() {
			hco.Spec.BareMetalPlatform = false

			expectedResource := newKubeVirtStorageConfigForCR(hco, namespace)
			Expect(expectedResource.Data["volumeMode"]).To(Equal("Filesystem"))
		})

		It("local storage class name should be available when specified", func() {
			hco.Spec.LocalStorageClassName = "local"

			expectedResource := newKubeVirtStorageConfigForCR(hco, namespace)
			Expect(expectedResource.Data["local.accessMode"]).To(Equal("ReadWriteOnce"))
			Expect(expectedResource.Data["local.volumeMode"]).To(Equal("Filesystem"))
		})
	})

	Context("KubeVirt", func() {
		var hco *hcov1beta1.HyperConverged
		var req *hcoRequest

		BeforeEach(func() {
			hco = newHco()
			req = newReq(hco)
		})

		It("should create if not present", func() {
			expectedResource := hco.NewKubeVirt(namespace)
			cl := initClient([]runtime.Object{})
			r := initReconciler(cl)
			res := r.ensureKubeVirt(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			foundResource := &kubevirtv1.KubeVirt{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: expectedResource.Name, Namespace: expectedResource.Namespace},
					foundResource),
			).To(BeNil())
			Expect(foundResource.Name).To(Equal(expectedResource.Name))
			Expect(foundResource.Labels).Should(HaveKeyWithValue(hcoutil.AppLabel, name))
			Expect(foundResource.Namespace).To(Equal(expectedResource.Namespace))
		})

		It("should find if present", func() {
			expectedResource := hco.NewKubeVirt(namespace)
			expectedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", expectedResource.Namespace, expectedResource.Name)
			cl := initClient([]runtime.Object{hco, expectedResource})
			r := initReconciler(cl)
			res := r.ensureKubeVirt(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			// Check HCO's status
			Expect(hco.Status.RelatedObjects).To(Not(BeNil()))
			objectRef, err := reference.GetReference(r.scheme, expectedResource)
			Expect(err).To(BeNil())
			// ObjectReference should have been added
			Expect(hco.Status.RelatedObjects).To(ContainElement(*objectRef))
			// Check conditions
			Expect(req.conditions[conditionsv1.ConditionAvailable]).To(testlib.RepresentCondition(conditionsv1.Condition{
				Type:    conditionsv1.ConditionAvailable,
				Status:  corev1.ConditionFalse,
				Reason:  "KubeVirtConditions",
				Message: "KubeVirt resource has no conditions",
			}))
			Expect(req.conditions[conditionsv1.ConditionProgressing]).To(testlib.RepresentCondition(conditionsv1.Condition{
				Type:    conditionsv1.ConditionProgressing,
				Status:  corev1.ConditionTrue,
				Reason:  "KubeVirtConditions",
				Message: "KubeVirt resource has no conditions",
			}))
			Expect(req.conditions[conditionsv1.ConditionUpgradeable]).To(testlib.RepresentCondition(conditionsv1.Condition{
				Type:    conditionsv1.ConditionUpgradeable,
				Status:  corev1.ConditionFalse,
				Reason:  "KubeVirtConditions",
				Message: "KubeVirt resource has no conditions",
			}))
		})

		It("should set default UninstallStrategy if missing", func() {
			expectedResource := hco.NewKubeVirt(namespace)
			expectedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", expectedResource.Namespace, expectedResource.Name)
			missingUSResource := hco.NewKubeVirt(namespace)
			missingUSResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", missingUSResource.Namespace, missingUSResource.Name)
			missingUSResource.Spec.UninstallStrategy = ""

			cl := initClient([]runtime.Object{hco, missingUSResource})
			r := initReconciler(cl)
			res := r.ensureKubeVirt(req)
			Expect(res.UpgradeDone).To(BeFalse())
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
			existingResource := hco.NewKubeVirt()

			hco.Spec.Infra = hcov1beta1.HyperConvergedConfig{NodePlacement: NewHyperConvergedConfig()}
			hco.Spec.Workloads = hcov1beta1.HyperConvergedConfig{NodePlacement: NewHyperConvergedConfig()}

			cl := initClient([]runtime.Object{hco, existingResource})
			r := initReconciler(cl)
			res := r.ensureKubeVirt(req)
			Expect(res.UpgradeDone).To(BeFalse())
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
			reflect.DeepEqual(foundResource.Spec.Workloads.NodePlacement.Tolerations, hco.Spec.Workloads.NodePlacement.Tolerations)

			Expect(req.conditions).To(BeEmpty())
		})

		It("should remove node placement if missing in HCO CR", func() {

			hcoNodePlacement := newHco()
			hcoNodePlacement.Spec.Infra = hcov1beta1.HyperConvergedConfig{NodePlacement: NewHyperConvergedConfig()}
			hcoNodePlacement.Spec.Workloads = hcov1beta1.HyperConvergedConfig{NodePlacement: NewHyperConvergedConfig()}
			existingResource := hcoNodePlacement.NewKubeVirt()

			cl := initClient([]runtime.Object{hco, existingResource})
			r := initReconciler(cl)
			res := r.ensureKubeVirt(req)
			Expect(res.UpgradeDone).To(BeFalse())
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

			Expect(req.conditions).To(BeEmpty())
		})

		It("should modify node placement according to HCO CR", func() {
			hco.Spec.Infra = hcov1beta1.HyperConvergedConfig{NodePlacement: NewHyperConvergedConfig()}
			hco.Spec.Workloads = hcov1beta1.HyperConvergedConfig{NodePlacement: NewHyperConvergedConfig()}
			existingResource := hco.NewKubeVirt()

			// now, modify HCO's node placement
			seconds3 := int64(3)
			hco.Spec.Infra.NodePlacement.Tolerations = append(hco.Spec.Infra.NodePlacement.Tolerations, corev1.Toleration{
				Key: "key3", Operator: "operator3", Value: "value3", Effect: "effect3", TolerationSeconds: &seconds3,
			})

			hco.Spec.Workloads.NodePlacement.NodeSelector["key1"] = "something else"

			cl := initClient([]runtime.Object{hco, existingResource})
			r := initReconciler(cl)
			res := r.ensureKubeVirt(req)
			Expect(res.UpgradeDone).To(BeFalse())
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

			Expect(req.conditions).To(BeEmpty())
		})

		It("should handle conditions", func() {
			expectedResource := hco.NewKubeVirt(namespace)
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
			cl := initClient([]runtime.Object{hco, expectedResource})
			r := initReconciler(cl)
			res := r.ensureKubeVirt(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			// Check HCO's status
			Expect(hco.Status.RelatedObjects).To(Not(BeNil()))
			objectRef, err := reference.GetReference(r.scheme, expectedResource)
			Expect(err).To(BeNil())
			// ObjectReference should have been added
			Expect(hco.Status.RelatedObjects).To(ContainElement(*objectRef))
			// Check conditions
			Expect(req.conditions[conditionsv1.ConditionAvailable]).To(testlib.RepresentCondition(conditionsv1.Condition{
				Type:    conditionsv1.ConditionAvailable,
				Status:  corev1.ConditionFalse,
				Reason:  "KubeVirtNotAvailable",
				Message: "KubeVirt is not available: Bar",
			}))
			Expect(req.conditions[conditionsv1.ConditionProgressing]).To(testlib.RepresentCondition(conditionsv1.Condition{
				Type:    conditionsv1.ConditionProgressing,
				Status:  corev1.ConditionTrue,
				Reason:  "KubeVirtProgressing",
				Message: "KubeVirt is progressing: Bar",
			}))
			Expect(req.conditions[conditionsv1.ConditionUpgradeable]).To(testlib.RepresentCondition(conditionsv1.Condition{
				Type:    conditionsv1.ConditionUpgradeable,
				Status:  corev1.ConditionFalse,
				Reason:  "KubeVirtProgressing",
				Message: "KubeVirt is progressing: Bar",
			}))
			Expect(req.conditions[conditionsv1.ConditionDegraded]).To(testlib.RepresentCondition(conditionsv1.Condition{
				Type:    conditionsv1.ConditionDegraded,
				Status:  corev1.ConditionTrue,
				Reason:  "KubeVirtDegraded",
				Message: "KubeVirt is degraded: Bar",
			}))
		})
	})

	Context("CDI", func() {
		var hco *hcov1beta1.HyperConverged
		var req *hcoRequest

		BeforeEach(func() {
			hco = newHco()
			req = newReq(hco)
		})

		It("should create if not present", func() {
			expectedResource := hco.NewCDI()
			cl := initClient([]runtime.Object{})
			r := initReconciler(cl)
			res := r.ensureCDI(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			foundResource := &cdiv1beta1.CDI{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: expectedResource.Name, Namespace: expectedResource.Namespace},
					foundResource),
			).To(BeNil())
			Expect(foundResource.Name).To(Equal(expectedResource.Name))
			Expect(foundResource.Labels).Should(HaveKeyWithValue(hcoutil.AppLabel, name))
			Expect(foundResource.Namespace).To(Equal(expectedResource.Namespace))
		})

		It("should find if present", func() {
			expectedResource := hco.NewCDI()
			expectedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", expectedResource.Namespace, expectedResource.Name)
			cl := initClient([]runtime.Object{hco, expectedResource})
			r := initReconciler(cl)
			res := r.ensureCDI(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			// Check HCO's status
			Expect(hco.Status.RelatedObjects).To(Not(BeNil()))
			objectRef, err := reference.GetReference(r.scheme, expectedResource)
			Expect(err).To(BeNil())
			// ObjectReference should have been added
			Expect(hco.Status.RelatedObjects).To(ContainElement(*objectRef))
			// Check conditions
			Expect(req.conditions[conditionsv1.ConditionAvailable]).To(testlib.RepresentCondition(conditionsv1.Condition{
				Type:    conditionsv1.ConditionAvailable,
				Status:  corev1.ConditionFalse,
				Reason:  "CDIConditions",
				Message: "CDI resource has no conditions",
			}))
			Expect(req.conditions[conditionsv1.ConditionProgressing]).To(testlib.RepresentCondition(conditionsv1.Condition{
				Type:    conditionsv1.ConditionProgressing,
				Status:  corev1.ConditionTrue,
				Reason:  "CDIConditions",
				Message: "CDI resource has no conditions",
			}))
			Expect(req.conditions[conditionsv1.ConditionUpgradeable]).To(testlib.RepresentCondition(conditionsv1.Condition{
				Type:    conditionsv1.ConditionUpgradeable,
				Status:  corev1.ConditionFalse,
				Reason:  "CDIConditions",
				Message: "CDI resource has no conditions",
			}))
		})

		It("should set default UninstallStrategy if missing", func() {
			expectedResource := hco.NewCDI(namespace)
			expectedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", expectedResource.Namespace, expectedResource.Name)
			missingUSResource := hco.NewCDI(namespace)
			missingUSResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", missingUSResource.Namespace, missingUSResource.Name)
			missingUSResource.Spec.UninstallStrategy = nil

			cl := initClient([]runtime.Object{hco, missingUSResource})
			r := initReconciler(cl)
			res := r.ensureCDI(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			foundResource := &cdiv1beta1.CDI{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: expectedResource.Name, Namespace: expectedResource.Namespace},
					foundResource),
			).To(BeNil())
			Expect(*foundResource.Spec.UninstallStrategy).To(Equal(*expectedResource.Spec.UninstallStrategy))
		})

		It("should add node placement if missing in CDI", func() {
			existingResource := hco.NewCDI()

			hco.Spec.Infra = hcov1beta1.HyperConvergedConfig{NodePlacement: NewHyperConvergedConfig()}
			hco.Spec.Workloads = hcov1beta1.HyperConvergedConfig{NodePlacement: NewHyperConvergedConfig()}

			cl := initClient([]runtime.Object{hco, existingResource})
			r := initReconciler(cl)
			res := r.ensureCDI(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			foundResource := &cdiv1beta1.CDI{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
					foundResource),
			).To(BeNil())

			Expect(existingResource.Spec.Infra.Affinity).To(BeNil())
			Expect(existingResource.Spec.Infra.Tolerations).To(BeEmpty())
			Expect(existingResource.Spec.Infra.NodeSelector).To(BeNil())
			Expect(existingResource.Spec.Workloads.Affinity).To(BeNil())
			Expect(existingResource.Spec.Workloads.Tolerations).To(BeEmpty())
			Expect(existingResource.Spec.Workloads.NodeSelector).To(BeNil())

			Expect(foundResource.Spec.Infra.Affinity).ToNot(BeNil())
			Expect(foundResource.Spec.Infra.NodeSelector["key1"]).Should(Equal("value1"))
			Expect(foundResource.Spec.Infra.NodeSelector["key2"]).Should(Equal("value2"))

			Expect(foundResource.Spec.Workloads).ToNot(BeNil())
			reflect.DeepEqual(foundResource.Spec.Workloads.Tolerations, hco.Spec.Workloads.NodePlacement.Tolerations)

			Expect(req.conditions).To(BeEmpty())
		})

		It("should remove node placement if missing in HCO CR", func() {

			hcoNodePlacement := newHco()
			hcoNodePlacement.Spec.Infra = hcov1beta1.HyperConvergedConfig{NodePlacement: NewHyperConvergedConfig()}
			hcoNodePlacement.Spec.Workloads = hcov1beta1.HyperConvergedConfig{NodePlacement: NewHyperConvergedConfig()}
			existingResource := hcoNodePlacement.NewCDI()

			cl := initClient([]runtime.Object{hco, existingResource})
			r := initReconciler(cl)
			res := r.ensureCDI(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			foundResource := &cdiv1beta1.CDI{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
					foundResource),
			).To(BeNil())

			Expect(existingResource.Spec.Infra.Affinity).ToNot(BeNil())
			Expect(existingResource.Spec.Infra.Tolerations).ToNot(BeEmpty())
			Expect(existingResource.Spec.Infra.NodeSelector).ToNot(BeNil())
			Expect(existingResource.Spec.Workloads.Affinity).ToNot(BeNil())
			Expect(existingResource.Spec.Workloads.Tolerations).ToNot(BeEmpty())
			Expect(existingResource.Spec.Workloads.NodeSelector).ToNot(BeNil())

			Expect(foundResource.Spec.Infra.Affinity).To(BeNil())
			Expect(foundResource.Spec.Infra.Tolerations).To(BeEmpty())
			Expect(foundResource.Spec.Infra.NodeSelector).To(BeNil())
			Expect(foundResource.Spec.Workloads.Affinity).To(BeNil())
			Expect(foundResource.Spec.Workloads.Tolerations).To(BeEmpty())
			Expect(foundResource.Spec.Workloads.NodeSelector).To(BeNil())

			Expect(req.conditions).To(BeEmpty())
		})

		It("should modify node placement according to HCO CR", func() {
			hco.Spec.Infra = hcov1beta1.HyperConvergedConfig{NodePlacement: NewHyperConvergedConfig()}
			hco.Spec.Workloads = hcov1beta1.HyperConvergedConfig{NodePlacement: NewHyperConvergedConfig()}
			existingResource := hco.NewCDI()

			// now, modify HCO's node placement
			seconds3 := int64(3)
			hco.Spec.Infra.NodePlacement.Tolerations = append(hco.Spec.Infra.NodePlacement.Tolerations, corev1.Toleration{
				Key: "key3", Operator: "operator3", Value: "value3", Effect: "effect3", TolerationSeconds: &seconds3,
			})

			hco.Spec.Workloads.NodePlacement.NodeSelector["key1"] = "something else"

			cl := initClient([]runtime.Object{hco, existingResource})
			r := initReconciler(cl)
			res := r.ensureCDI(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			foundResource := &cdiv1beta1.CDI{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
					foundResource),
			).To(BeNil())

			Expect(existingResource.Spec.Infra.Tolerations).To(HaveLen(2))
			Expect(existingResource.Spec.Workloads.NodeSelector["key1"]).Should(Equal("value1"))

			Expect(foundResource.Spec.Infra.Tolerations).To(HaveLen(3))
			Expect(foundResource.Spec.Workloads.NodeSelector["key1"]).Should(Equal("something else"))

			Expect(req.conditions).To(BeEmpty())
		})

		It("should handle conditions", func() {
			expectedResource := hco.NewCDI()
			expectedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", expectedResource.Namespace, expectedResource.Name)
			expectedResource.Status.Conditions = []conditionsv1.Condition{
				conditionsv1.Condition{
					Type:    conditionsv1.ConditionAvailable,
					Status:  corev1.ConditionFalse,
					Reason:  "Foo",
					Message: "Bar",
				},
				conditionsv1.Condition{
					Type:    conditionsv1.ConditionProgressing,
					Status:  corev1.ConditionTrue,
					Reason:  "Foo",
					Message: "Bar",
				},
				conditionsv1.Condition{
					Type:    conditionsv1.ConditionDegraded,
					Status:  corev1.ConditionTrue,
					Reason:  "Foo",
					Message: "Bar",
				},
			}
			cl := initClient([]runtime.Object{hco, expectedResource})
			r := initReconciler(cl)
			res := r.ensureCDI(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			// Check HCO's status
			Expect(hco.Status.RelatedObjects).To(Not(BeNil()))
			objectRef, err := reference.GetReference(r.scheme, expectedResource)
			Expect(err).To(BeNil())
			// ObjectReference should have been added
			Expect(hco.Status.RelatedObjects).To(ContainElement(*objectRef))
			// Check conditions
			Expect(req.conditions[conditionsv1.ConditionAvailable]).To(testlib.RepresentCondition(conditionsv1.Condition{
				Type:    conditionsv1.ConditionAvailable,
				Status:  corev1.ConditionFalse,
				Reason:  "CDINotAvailable",
				Message: "CDI is not available: Bar",
			}))
			Expect(req.conditions[conditionsv1.ConditionProgressing]).To(testlib.RepresentCondition(conditionsv1.Condition{
				Type:    conditionsv1.ConditionProgressing,
				Status:  corev1.ConditionTrue,
				Reason:  "CDIProgressing",
				Message: "CDI is progressing: Bar",
			}))
			Expect(req.conditions[conditionsv1.ConditionUpgradeable]).To(testlib.RepresentCondition(conditionsv1.Condition{
				Type:    conditionsv1.ConditionUpgradeable,
				Status:  corev1.ConditionFalse,
				Reason:  "CDIProgressing",
				Message: "CDI is progressing: Bar",
			}))
			Expect(req.conditions[conditionsv1.ConditionDegraded]).To(testlib.RepresentCondition(conditionsv1.Condition{
				Type:    conditionsv1.ConditionDegraded,
				Status:  corev1.ConditionTrue,
				Reason:  "CDIDegraded",
				Message: "CDI is degraded: Bar",
			}))
		})
	})

	Context("NetworkAddonsConfig", func() {
		var hco *hcov1beta1.HyperConverged
		var req *hcoRequest

		BeforeEach(func() {
			hco = newHco()
			req = newReq(hco)
		})

		It("should create if not present", func() {
			expectedResource := hco.NewNetworkAddons()
			cl := initClient([]runtime.Object{})
			r := initReconciler(cl)
			res := r.ensureNetworkAddons(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			foundResource := &networkaddonsv1.NetworkAddonsConfig{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: expectedResource.Name, Namespace: expectedResource.Namespace},
					foundResource),
			).To(BeNil())
			Expect(foundResource.Name).To(Equal(expectedResource.Name))
			Expect(foundResource.Labels).Should(HaveKeyWithValue(hcoutil.AppLabel, name))
			Expect(foundResource.Namespace).To(Equal(expectedResource.Namespace))
			Expect(foundResource.Spec.Multus).To(Equal(&networkaddonsshared.Multus{}))
			Expect(foundResource.Spec.LinuxBridge).To(Equal(&networkaddonsshared.LinuxBridge{}))
			Expect(foundResource.Spec.KubeMacPool).To(Equal(&networkaddonsshared.KubeMacPool{}))
		})

		It("should find if present", func() {
			expectedResource := hco.NewNetworkAddons()
			expectedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", expectedResource.Namespace, expectedResource.Name)
			cl := initClient([]runtime.Object{hco, expectedResource})
			r := initReconciler(cl)
			res := r.ensureNetworkAddons(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			// Check HCO's status
			Expect(hco.Status.RelatedObjects).To(Not(BeNil()))
			objectRef, err := reference.GetReference(r.scheme, expectedResource)
			Expect(err).To(BeNil())
			// ObjectReference should have been added
			Expect(hco.Status.RelatedObjects).To(ContainElement(*objectRef))
			// Check conditions
			Expect(req.conditions[conditionsv1.ConditionAvailable]).To(testlib.RepresentCondition(conditionsv1.Condition{
				Type:    conditionsv1.ConditionAvailable,
				Status:  corev1.ConditionFalse,
				Reason:  "NetworkAddonsConfigConditions",
				Message: "NetworkAddonsConfig resource has no conditions",
			}))
			Expect(req.conditions[conditionsv1.ConditionProgressing]).To(testlib.RepresentCondition(conditionsv1.Condition{
				Type:    conditionsv1.ConditionProgressing,
				Status:  corev1.ConditionTrue,
				Reason:  "NetworkAddonsConfigConditions",
				Message: "NetworkAddonsConfig resource has no conditions",
			}))
			Expect(req.conditions[conditionsv1.ConditionUpgradeable]).To(testlib.RepresentCondition(conditionsv1.Condition{
				Type:    conditionsv1.ConditionUpgradeable,
				Status:  corev1.ConditionFalse,
				Reason:  "NetworkAddonsConfigConditions",
				Message: "NetworkAddonsConfig resource has no conditions",
			}))
		})

		It("should find reconcile to default", func() {
			existingResource := hco.NewNetworkAddons()
			existingResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", existingResource.Namespace, existingResource.Name)
			existingResource.Spec.ImagePullPolicy = corev1.PullAlways // set non-default value

			cl := initClient([]runtime.Object{hco, existingResource})
			r := initReconciler(cl)
			res := r.ensureNetworkAddons(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			foundResource := &networkaddonsv1.NetworkAddonsConfig{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
					foundResource),
			).To(BeNil())
			Expect(foundResource.Spec.ImagePullPolicy).To(BeEmpty())

			Expect(req.conditions).To(BeEmpty())
		})

		It("should add node placement if missing in CNAO", func() {
			existingResource := hco.NewNetworkAddons()

			hco.Spec.Infra = hcov1beta1.HyperConvergedConfig{NewHyperConvergedConfig()}
			hco.Spec.Workloads = hcov1beta1.HyperConvergedConfig{NewHyperConvergedConfig()}

			cl := initClient([]runtime.Object{hco, existingResource})
			r := initReconciler(cl)
			res := r.ensureNetworkAddons(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			foundResource := &networkaddonsv1.NetworkAddonsConfig{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
					foundResource),
			).To(BeNil())

			Expect(existingResource.Spec.PlacementConfiguration).To(BeNil())
			Expect(foundResource.Spec.PlacementConfiguration).ToNot(BeNil())
			placementConfig := foundResource.Spec.PlacementConfiguration
			Expect(placementConfig.Infra).ToNot(BeNil())
			Expect(placementConfig.Infra.NodeSelector["key1"]).Should(Equal("value1"))
			Expect(placementConfig.Infra.NodeSelector["key2"]).Should(Equal("value2"))

			Expect(placementConfig.Workloads).ToNot(BeNil())
			reflect.DeepEqual(placementConfig.Workloads.Tolerations, hco.Spec.Workloads.NodePlacement.Tolerations)

			Expect(req.conditions).To(BeEmpty())
		})

		It("should remove node placement if missing in HCO CR", func() {

			hcoNodePlacement := newHco()
			hcoNodePlacement.Spec.Infra = hcov1beta1.HyperConvergedConfig{NodePlacement: NewHyperConvergedConfig()}
			hcoNodePlacement.Spec.Workloads = hcov1beta1.HyperConvergedConfig{NodePlacement: NewHyperConvergedConfig()}
			existingResource := hcoNodePlacement.NewNetworkAddons()

			cl := initClient([]runtime.Object{hco, existingResource})
			r := initReconciler(cl)
			res := r.ensureNetworkAddons(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			foundResource := &networkaddonsv1.NetworkAddonsConfig{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
					foundResource),
			).To(BeNil())

			Expect(existingResource.Spec.PlacementConfiguration).ToNot(BeNil())
			Expect(foundResource.Spec.PlacementConfiguration).To(BeNil())

			Expect(req.conditions).To(BeEmpty())
		})

		It("should modify node placement according to HCO CR", func() {

			hco.Spec.Infra = hcov1beta1.HyperConvergedConfig{NodePlacement: NewHyperConvergedConfig()}
			hco.Spec.Workloads = hcov1beta1.HyperConvergedConfig{NodePlacement: NewHyperConvergedConfig()}
			existingResource := hco.NewNetworkAddons()

			// now, modify HCO's node placement
			seconds3 := int64(3)
			hco.Spec.Infra.NodePlacement.Tolerations = append(hco.Spec.Infra.NodePlacement.Tolerations, corev1.Toleration{
				Key: "key3", Operator: "operator3", Value: "value3", Effect: "effect3", TolerationSeconds: &seconds3,
			})

			hco.Spec.Workloads.NodePlacement.NodeSelector["key1"] = "something else"

			cl := initClient([]runtime.Object{hco, existingResource})
			r := initReconciler(cl)
			res := r.ensureNetworkAddons(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			foundResource := &networkaddonsv1.NetworkAddonsConfig{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
					foundResource),
			).To(BeNil())

			Expect(existingResource.Spec.PlacementConfiguration).ToNot(BeNil())
			Expect(existingResource.Spec.PlacementConfiguration.Infra.Tolerations).To(HaveLen(2))
			Expect(existingResource.Spec.PlacementConfiguration.Workloads.NodeSelector["key1"]).Should(Equal("value1"))

			Expect(foundResource.Spec.PlacementConfiguration).ToNot(BeNil())
			Expect(foundResource.Spec.PlacementConfiguration.Infra.Tolerations).To(HaveLen(3))
			Expect(foundResource.Spec.PlacementConfiguration.Workloads.NodeSelector["key1"]).Should(Equal("something else"))

			Expect(req.conditions).To(BeEmpty())
		})

		It("should handle conditions", func() {
			expectedResource := hco.NewNetworkAddons()
			expectedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", expectedResource.Namespace, expectedResource.Name)
			expectedResource.Status.Conditions = []conditionsv1.Condition{
				conditionsv1.Condition{
					Type:    conditionsv1.ConditionAvailable,
					Status:  corev1.ConditionFalse,
					Reason:  "Foo",
					Message: "Bar",
				},
				conditionsv1.Condition{
					Type:    conditionsv1.ConditionProgressing,
					Status:  corev1.ConditionTrue,
					Reason:  "Foo",
					Message: "Bar",
				},
				conditionsv1.Condition{
					Type:    conditionsv1.ConditionDegraded,
					Status:  corev1.ConditionTrue,
					Reason:  "Foo",
					Message: "Bar",
				},
			}
			cl := initClient([]runtime.Object{hco, expectedResource})
			r := initReconciler(cl)
			res := r.ensureNetworkAddons(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			// Check HCO's status
			Expect(hco.Status.RelatedObjects).To(Not(BeNil()))
			objectRef, err := reference.GetReference(r.scheme, expectedResource)
			Expect(err).To(BeNil())
			// ObjectReference should have been added
			Expect(hco.Status.RelatedObjects).To(ContainElement(*objectRef))
			// Check conditions
			Expect(req.conditions[conditionsv1.ConditionAvailable]).To(testlib.RepresentCondition(conditionsv1.Condition{
				Type:    conditionsv1.ConditionAvailable,
				Status:  corev1.ConditionFalse,
				Reason:  "NetworkAddonsConfigNotAvailable",
				Message: "NetworkAddonsConfig is not available: Bar",
			}))
			Expect(req.conditions[conditionsv1.ConditionProgressing]).To(testlib.RepresentCondition(conditionsv1.Condition{
				Type:    conditionsv1.ConditionProgressing,
				Status:  corev1.ConditionTrue,
				Reason:  "NetworkAddonsConfigProgressing",
				Message: "NetworkAddonsConfig is progressing: Bar",
			}))
			Expect(req.conditions[conditionsv1.ConditionUpgradeable]).To(testlib.RepresentCondition(conditionsv1.Condition{
				Type:    conditionsv1.ConditionUpgradeable,
				Status:  corev1.ConditionFalse,
				Reason:  "NetworkAddonsConfigProgressing",
				Message: "NetworkAddonsConfig is progressing: Bar",
			}))
			Expect(req.conditions[conditionsv1.ConditionDegraded]).To(testlib.RepresentCondition(conditionsv1.Condition{
				Type:    conditionsv1.ConditionDegraded,
				Status:  corev1.ConditionTrue,
				Reason:  "NetworkAddonsConfigDegraded",
				Message: "NetworkAddonsConfig is degraded: Bar",
			}))
		})
	})

	Context("KubeVirtCommonTemplatesBundle", func() {
		var hco *hcov1beta1.HyperConverged
		var req *hcoRequest

		BeforeEach(func() {
			hco = newHco()
			req = newReq(hco)
		})

		It("should create if not present", func() {
			expectedResource := hco.NewKubeVirtCommonTemplateBundle()
			cl := initClient([]runtime.Object{})
			r := initReconciler(cl)
			res := r.ensureKubeVirtCommonTemplateBundle(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			foundResource := &sspv1.KubevirtCommonTemplatesBundle{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: expectedResource.Name, Namespace: expectedResource.Namespace},
					foundResource),
			).To(BeNil())
			Expect(foundResource.Name).To(Equal(expectedResource.Name))
			Expect(foundResource.Labels).Should(HaveKeyWithValue(hcoutil.AppLabel, name))
			Expect(foundResource.Namespace).To(Equal(expectedResource.Namespace))
		})

		It("should find if present", func() {
			expectedResource := hco.NewKubeVirtCommonTemplateBundle()
			expectedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", expectedResource.Namespace, expectedResource.Name)
			cl := initClient([]runtime.Object{hco, expectedResource})
			r := initReconciler(cl)
			res := r.ensureKubeVirtCommonTemplateBundle(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			// Check HCO's status
			Expect(hco.Status.RelatedObjects).To(Not(BeNil()))
			objectRef, err := reference.GetReference(r.scheme, expectedResource)
			Expect(err).To(BeNil())
			// ObjectReference should have been added
			Expect(hco.Status.RelatedObjects).To(ContainElement(*objectRef))
		})

		It("should reconcile to default", func() {
			existingResource := hco.NewKubeVirtCommonTemplateBundle()
			existingResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", existingResource.Namespace, existingResource.Name)

			existingResource.Spec.Version = "Non default value"

			cl := initClient([]runtime.Object{hco, existingResource})
			r := initReconciler(cl)
			res := r.ensureKubeVirtCommonTemplateBundle(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			foundResource := &sspv1.KubevirtCommonTemplatesBundle{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
					foundResource),
			).To(BeNil())
			Expect(foundResource.Spec.Version).To(BeEmpty())
		})

		// TODO: add tests to ensure that HCO properly propagates NodePlacement from its CR

		// TODO: temporary avoid checking conditions on KubevirtCommonTemplatesBundle because it's currently
		// broken on k8s. Revert this when we will be able to fix it
		/*
			It("should handle conditions", func() {
				expectedResource := newKubeVirtCommonTemplateBundleForCR(hco, OpenshiftNamespace)
				expectedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", expectedResource.Namespace, expectedResource.Name)
				expectedResource.Status.Conditions = []conditionsv1.Condition{
					conditionsv1.Condition{
						Type:    conditionsv1.ConditionAvailable,
						Status:  corev1.ConditionFalse,
						Reason:  "Foo",
						Message: "Bar",
					},
					conditionsv1.Condition{
						Type:    conditionsv1.ConditionProgressing,
						Status:  corev1.ConditionTrue,
						Reason:  "Foo",
						Message: "Bar",
					},
					conditionsv1.Condition{
						Type:    conditionsv1.ConditionDegraded,
						Status:  corev1.ConditionTrue,
						Reason:  "Foo",
						Message: "Bar",
					},
				}
				cl := initClient([]runtime.Object{hco, expectedResource})
				r := initReconciler(cl)
				Expect(r.ensureKubeVirtCommonTemplateBundle(req)).To(BeNil())

				// Check HCO's status
				Expect(hco.Status.RelatedObjects).To(Not(BeNil()))
				objectRef, err := reference.GetReference(r.scheme, expectedResource)
				Expect(err).To(BeNil())
				// ObjectReference should have been added
				Expect(hco.Status.RelatedObjects).To(ContainElement(*objectRef))
				// Check conditions
				Expect(req.conditions[]).To(ContainElement(testlib.RepresentCondition(conditionsv1.Condition{
					Type:    conditionsv1.ConditionAvailable,
					Status:  corev1.ConditionFalse,
					Reason:  "KubevirtCommonTemplatesBundleNotAvailable",
					Message: "KubevirtCommonTemplatesBundle is not available: Bar",
				})))
				Expect(req.conditions[]).To(ContainElement(testlib.RepresentCondition(conditionsv1.Condition{
					Type:    conditionsv1.ConditionProgressing,
					Status:  corev1.ConditionTrue,
					Reason:  "KubevirtCommonTemplatesBundleProgressing",
					Message: "KubevirtCommonTemplatesBundle is progressing: Bar",
				})))
				Expect(req.conditions[]).To(ContainElement(testlib.RepresentCondition(conditionsv1.Condition{
					Type:    conditionsv1.ConditionUpgradeable,
					Status:  corev1.ConditionFalse,
					Reason:  "KubevirtCommonTemplatesBundleProgressing",
					Message: "KubevirtCommonTemplatesBundle is progressing: Bar",
				})))
				Expect(req.conditions[]).To(ContainElement(testlib.RepresentCondition(conditionsv1.Condition{
					Type:    conditionsv1.ConditionDegraded,
					Status:  corev1.ConditionTrue,
					Reason:  "KubevirtCommonTemplatesBundleDegraded",
					Message: "KubevirtCommonTemplatesBundle is degraded: Bar",
				})))
			})
		*/
	})

	Context("KubeVirtNodeLabellerBundle", func() {
		var hco *hcov1beta1.HyperConverged
		var req *hcoRequest

		BeforeEach(func() {
			hco = newHco()
			req = newReq(hco)
		})

		It("should create if not present", func() {
			expectedResource := newKubeVirtNodeLabellerBundleForCR(hco, namespace)
			cl := initClient([]runtime.Object{})
			r := initReconciler(cl)
			res := r.ensureKubeVirtNodeLabellerBundle(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			foundResource := &sspv1.KubevirtNodeLabellerBundle{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: expectedResource.Name, Namespace: expectedResource.Namespace},
					foundResource),
			).To(BeNil())
			Expect(foundResource.Name).To(Equal(expectedResource.Name))
			Expect(foundResource.Labels).Should(HaveKeyWithValue(hcoutil.AppLabel, name))
			Expect(foundResource.Namespace).To(Equal(expectedResource.Namespace))
		})

		It("should find if present", func() {
			expectedResource := newKubeVirtNodeLabellerBundleForCR(hco, namespace)
			expectedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", expectedResource.Namespace, expectedResource.Name)
			cl := initClient([]runtime.Object{hco, expectedResource})
			r := initReconciler(cl)
			res := r.ensureKubeVirtNodeLabellerBundle(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			// Check HCO's status
			Expect(hco.Status.RelatedObjects).To(Not(BeNil()))
			objectRef, err := reference.GetReference(r.scheme, expectedResource)
			Expect(err).To(BeNil())
			// ObjectReference should have been added
			Expect(hco.Status.RelatedObjects).To(ContainElement(*objectRef))
		})

		It("should reconcile to default", func() {
			existingResource := newKubeVirtNodeLabellerBundleForCR(hco, namespace)
			existingResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", existingResource.Namespace, existingResource.Name)

			existingResource.Spec.Version = "Non default value"

			cl := initClient([]runtime.Object{hco, existingResource})
			r := initReconciler(cl)
			res := r.ensureKubeVirtNodeLabellerBundle(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			foundResource := &sspv1.KubevirtNodeLabellerBundle{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
					foundResource),
			).To(BeNil())
			Expect(foundResource.Spec.Version).To(BeEmpty())
		})

		It("should add node placement if missing in KubeVirtNodeLabellerBundle", func() {
			existingResource := newKubeVirtNodeLabellerBundleForCR(hco, namespace)

			hco.Spec.Workloads.NodePlacement = NewHyperConvergedConfig()

			cl := initClient([]runtime.Object{hco, existingResource})
			r := initReconciler(cl)
			res := r.ensureKubeVirtNodeLabellerBundle(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			foundResource := &sspv1.KubevirtNodeLabellerBundle{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
					foundResource),
			).To(BeNil())

			Expect(existingResource.Spec.Affinity.NodeAffinity).To(BeNil())
			Expect(existingResource.Spec.Affinity.PodAffinity).To(BeNil())
			Expect(existingResource.Spec.Affinity.PodAntiAffinity).To(BeNil())
			Expect(foundResource.Spec.Affinity.NodeAffinity).ToNot(BeNil())
			Expect(foundResource.Spec.NodeSelector["key1"]).Should(Equal("value1"))
			Expect(foundResource.Spec.NodeSelector["key2"]).Should(Equal("value2"))

			reflect.DeepEqual(foundResource.Spec.Tolerations, hco.Spec.Workloads.NodePlacement.Tolerations)

			Expect(req.conditions).To(BeEmpty())
		})

		It("should remove node placement if missing in HCO CR", func() {

			hcoNodePlacement := newHco()
			hcoNodePlacement.Spec.Workloads.NodePlacement = NewHyperConvergedConfig()
			existingResource := newKubeVirtNodeLabellerBundleForCR(hcoNodePlacement, namespace)

			cl := initClient([]runtime.Object{hco, existingResource})
			r := initReconciler(cl)
			res := r.ensureKubeVirtNodeLabellerBundle(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			foundResource := &sspv1.KubevirtNodeLabellerBundle{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
					foundResource),
			).To(BeNil())

			Expect(existingResource.Spec.Affinity.NodeAffinity).ToNot(BeNil())
			Expect(foundResource.Spec.Affinity.NodeAffinity).To(BeNil())

			Expect(req.conditions).To(BeEmpty())
		})

		It("should modify node placement according to HCO CR", func() {

			hco.Spec.Workloads.NodePlacement = NewHyperConvergedConfig()
			existingResource := newKubeVirtNodeLabellerBundleForCR(hco, namespace)

			// now, modify HCO's node placement
			seconds3 := int64(3)
			hco.Spec.Workloads.NodePlacement.Tolerations = append(hco.Spec.Workloads.NodePlacement.Tolerations, corev1.Toleration{
				Key: "key3", Operator: "operator3", Value: "value3", Effect: "effect3", TolerationSeconds: &seconds3,
			})

			hco.Spec.Workloads.NodePlacement.NodeSelector["key1"] = "something else"

			cl := initClient([]runtime.Object{hco, existingResource})
			r := initReconciler(cl)
			res := r.ensureKubeVirtNodeLabellerBundle(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			foundResource := &sspv1.KubevirtNodeLabellerBundle{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
					foundResource),
			).To(BeNil())

			Expect(existingResource.Spec.Affinity.NodeAffinity).ToNot(BeNil())
			Expect(existingResource.Spec.Tolerations).To(HaveLen(2))
			Expect(existingResource.Spec.NodeSelector["key1"]).Should(Equal("value1"))

			Expect(foundResource.Spec.Affinity.NodeAffinity).ToNot(BeNil())
			Expect(foundResource.Spec.Tolerations).To(HaveLen(3))
			Expect(foundResource.Spec.NodeSelector["key1"]).Should(Equal("something else"))

			Expect(req.conditions).To(BeEmpty())
		})

		// TODO: temporary avoid checking conditions on KubevirtNodeLabellerBundle because it's currently
		// broken on k8s. Revert this when we will be able to fix it
		/*
			It("should handle conditions", func() {
				expectedResource := newKubeVirtNodeLabellerBundleForCR(hco, namespace)
				expectedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", expectedResource.Namespace, expectedResource.Name)
				expectedResource.Status.Conditions = []conditionsv1.Condition{
					conditionsv1.Condition{
						Type:    conditionsv1.ConditionAvailable,
						Status:  corev1.ConditionFalse,
						Reason:  "Foo",
						Message: "Bar",
					},
					conditionsv1.Condition{
						Type:    conditionsv1.ConditionProgressing,
						Status:  corev1.ConditionTrue,
						Reason:  "Foo",
						Message: "Bar",
					},
					conditionsv1.Condition{
						Type:    conditionsv1.ConditionDegraded,
						Status:  corev1.ConditionTrue,
						Reason:  "Foo",
						Message: "Bar",
					},
				}
				cl := initClient([]runtime.Object{hco, expectedResource})
				r := initReconciler(cl)
				Expect(r.ensureKubeVirtNodeLabellerBundle(req)).To(BeNil())

				// Check HCO's status
				Expect(hco.Status.RelatedObjects).To(Not(BeNil()))
				objectRef, err := reference.GetReference(r.scheme, expectedResource)
				Expect(err).To(BeNil())
				// ObjectReference should have been added
				Expect(hco.Status.RelatedObjects).To(ContainElement(*objectRef))
				// Check conditions
				Expect(req.conditions[]).To(ContainElement(testlib.RepresentCondition(conditionsv1.Condition{
					Type:    conditionsv1.ConditionAvailable,
					Status:  corev1.ConditionFalse,
					Reason:  "KubevirtNodeLabellerBundleNotAvailable",
					Message: "KubevirtNodeLabellerBundle is not available: Bar",
				})))
				Expect(req.conditions[]).To(ContainElement(testlib.RepresentCondition(conditionsv1.Condition{
					Type:    conditionsv1.ConditionProgressing,
					Status:  corev1.ConditionTrue,
					Reason:  "KubevirtNodeLabellerBundleProgressing",
					Message: "KubevirtNodeLabellerBundle is progressing: Bar",
				})))
				Expect(req.conditions[]).To(ContainElement(testlib.RepresentCondition(conditionsv1.Condition{
					Type:    conditionsv1.ConditionUpgradeable,
					Status:  corev1.ConditionFalse,
					Reason:  "KubevirtNodeLabellerBundleProgressing",
					Message: "KubevirtNodeLabellerBundle is progressing: Bar",
				})))
				Expect(req.conditions[]).To(ContainElement(testlib.RepresentCondition(conditionsv1.Condition{
					Type:    conditionsv1.ConditionDegraded,
					Status:  corev1.ConditionTrue,
					Reason:  "KubevirtNodeLabellerBundleDegraded",
					Message: "KubevirtNodeLabellerBundle is degraded: Bar",
				})))
			})
		*/

		//It("should request KVM without any extra setting", func() {
		//	os.Unsetenv("KVM_EMULATION")
		//
		//	expectedResource := newKubeVirtNodeLabellerBundleForCR(hco, namespace)
		//	Expect(expectedResource.Spec.UseKVM).To(BeTrue())
		//})
		//
		//It("should not request KVM if emulation requested", func() {
		//	err := os.Setenv("KVM_EMULATION", "true")
		//	Expect(err).NotTo(HaveOccurred())
		//	defer os.Unsetenv("KVM_EMULATION")
		//
		//	expectedResource := newKubeVirtNodeLabellerBundleForCR(hco, namespace)
		//	Expect(expectedResource.Spec.UseKVM).To(BeFalse())
		//})

		//It("should request KVM if emulation value not set", func() {
		//	err := os.Setenv("KVM_EMULATION", "")
		//	Expect(err).NotTo(HaveOccurred())
		//	defer os.Unsetenv("KVM_EMULATION")
		//
		//	expectedResource := newKubeVirtNodeLabellerBundleForCR(hco, namespace)
		//	Expect(expectedResource.Spec.UseKVM).To(BeTrue())
		//})
	})

	Context("KubeVirtTemplateValidator", func() {
		var hco *hcov1beta1.HyperConverged
		var req *hcoRequest

		BeforeEach(func() {
			hco = newHco()
			req = newReq(hco)
		})

		It("should create if not present", func() {
			expectedResource := newKubeVirtTemplateValidatorForCR(hco, namespace)
			cl := initClient([]runtime.Object{})
			r := initReconciler(cl)
			res := r.ensureKubeVirtTemplateValidator(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			foundResource := &sspv1.KubevirtTemplateValidator{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: expectedResource.Name, Namespace: expectedResource.Namespace},
					foundResource),
			).To(BeNil())
			Expect(foundResource.Name).To(Equal(expectedResource.Name))
			Expect(foundResource.Labels).Should(HaveKeyWithValue(hcoutil.AppLabel, name))
			Expect(foundResource.Namespace).To(Equal(expectedResource.Namespace))
		})

		It("should find if present", func() {
			expectedResource := newKubeVirtTemplateValidatorForCR(hco, namespace)
			expectedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", expectedResource.Namespace, expectedResource.Name)
			cl := initClient([]runtime.Object{hco, expectedResource})
			r := initReconciler(cl)
			res := r.ensureKubeVirtTemplateValidator(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			// Check HCO's status
			Expect(hco.Status.RelatedObjects).To(Not(BeNil()))
			objectRef, err := reference.GetReference(r.scheme, expectedResource)
			Expect(err).To(BeNil())
			// ObjectReference should have been added
			Expect(hco.Status.RelatedObjects).To(ContainElement(*objectRef))
		})

		It("should reconcile to default", func() {
			existingResource := newKubeVirtTemplateValidatorForCR(hco, namespace)
			existingResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", existingResource.Namespace, existingResource.Name)

			existingResource.Spec.TemplateValidatorReplicas = 5 // set non-default value

			cl := initClient([]runtime.Object{hco, existingResource})
			r := initReconciler(cl)
			res := r.ensureKubeVirtTemplateValidator(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			foundResource := &sspv1.KubevirtTemplateValidator{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
					foundResource),
			).To(BeNil())
			Expect(foundResource.Spec.TemplateValidatorReplicas).To(BeZero())
		})

		It("should add node placement if missing in KubeVirtTemplateValidator", func() {
			existingResource := newKubeVirtTemplateValidatorForCR(hco, namespace)

			hco.Spec.Infra.NodePlacement = NewHyperConvergedConfig()

			cl := initClient([]runtime.Object{hco, existingResource})
			r := initReconciler(cl)
			res := r.ensureKubeVirtTemplateValidator(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			foundResource := &sspv1.KubevirtTemplateValidator{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
					foundResource),
			).To(BeNil())

			Expect(existingResource.Spec.Affinity.NodeAffinity).To(BeNil())
			Expect(existingResource.Spec.Affinity.PodAffinity).To(BeNil())
			Expect(existingResource.Spec.Affinity.PodAntiAffinity).To(BeNil())
			Expect(foundResource.Spec.Affinity.NodeAffinity).ToNot(BeNil())
			Expect(foundResource.Spec.NodeSelector["key1"]).Should(Equal("value1"))
			Expect(foundResource.Spec.NodeSelector["key2"]).Should(Equal("value2"))

			reflect.DeepEqual(foundResource.Spec.Tolerations, hco.Spec.Infra.NodePlacement.Tolerations)

			Expect(req.conditions).To(BeEmpty())
		})

		It("should remove node placement if missing in HCO CR", func() {

			hcoNodePlacement := newHco()
			hcoNodePlacement.Spec.Infra.NodePlacement = NewHyperConvergedConfig()
			existingResource := newKubeVirtTemplateValidatorForCR(hcoNodePlacement, namespace)

			cl := initClient([]runtime.Object{hco, existingResource})
			r := initReconciler(cl)
			res := r.ensureKubeVirtTemplateValidator(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			foundResource := &sspv1.KubevirtTemplateValidator{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
					foundResource),
			).To(BeNil())

			Expect(existingResource.Spec.Affinity.NodeAffinity).ToNot(BeNil())
			Expect(foundResource.Spec.Affinity.NodeAffinity).To(BeNil())

			Expect(req.conditions).To(BeEmpty())
		})

		It("should modify node placement according to HCO CR", func() {

			hco.Spec.Infra.NodePlacement = NewHyperConvergedConfig()
			existingResource := newKubeVirtTemplateValidatorForCR(hco, namespace)

			// now, modify HCO's node placement
			seconds3 := int64(3)
			hco.Spec.Infra.NodePlacement.Tolerations = append(hco.Spec.Infra.NodePlacement.Tolerations, corev1.Toleration{
				Key: "key3", Operator: "operator3", Value: "value3", Effect: "effect3", TolerationSeconds: &seconds3,
			})

			hco.Spec.Infra.NodePlacement.NodeSelector["key1"] = "something else"

			cl := initClient([]runtime.Object{hco, existingResource})
			r := initReconciler(cl)
			res := r.ensureKubeVirtTemplateValidator(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			foundResource := &sspv1.KubevirtTemplateValidator{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
					foundResource),
			).To(BeNil())

			Expect(existingResource.Spec.Affinity.NodeAffinity).ToNot(BeNil())
			Expect(existingResource.Spec.Tolerations).To(HaveLen(2))
			Expect(existingResource.Spec.NodeSelector["key1"]).Should(Equal("value1"))

			Expect(foundResource.Spec.Affinity.NodeAffinity).ToNot(BeNil())
			Expect(foundResource.Spec.Tolerations).To(HaveLen(3))
			Expect(foundResource.Spec.NodeSelector["key1"]).Should(Equal("something else"))

			Expect(req.conditions).To(BeEmpty())
		})

		// TODO: temporary avoid checking conditions on KubevirtTemplateValidator because it's currently
		// broken on k8s. Revert this when we will be able to fix it
		/*It("should handle conditions", func() {
			expectedResource := newKubeVirtTemplateValidatorForCR(hco, namespace)
			expectedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", expectedResource.Namespace, expectedResource.Name)
			expectedResource.Status.Conditions = []conditionsv1.Condition{
				conditionsv1.Condition{
					Type:    conditionsv1.ConditionAvailable,
					Status:  corev1.ConditionFalse,
					Reason:  "Foo",
					Message: "Bar",
				},
				conditionsv1.Condition{
					Type:    conditionsv1.ConditionProgressing,
					Status:  corev1.ConditionTrue,
					Reason:  "Foo",
					Message: "Bar",
				},
				conditionsv1.Condition{
					Type:    conditionsv1.ConditionDegraded,
					Status:  corev1.ConditionTrue,
					Reason:  "Foo",
					Message: "Bar",
				},
			}
			cl := initClient([]runtime.Object{hco, expectedResource})
			r := initReconciler(cl)
			Expect(r.ensureKubeVirtTemplateValidator(req)).To(BeNil())

			// Check HCO's status
			Expect(hco.Status.RelatedObjects).To(Not(BeNil()))
			objectRef, err := reference.GetReference(r.scheme, expectedResource)
			Expect(err).To(BeNil())
			// ObjectReference should have been added
			Expect(hco.Status.RelatedObjects).To(ContainElement(*objectRef))
			// Check conditions
			Expect(req.conditions[]).To(ContainElement(testlib.RepresentCondition(conditionsv1.Condition{
				Type:    conditionsv1.ConditionAvailable,
				Status:  corev1.ConditionFalse,
				Reason:  "KubevirtTemplateValidatorNotAvailable",
				Message: "KubevirtTemplateValidator is not available: Bar",
			})))
			Expect(req.conditions[]).To(ContainElement(testlib.RepresentCondition(conditionsv1.Condition{
				Type:    conditionsv1.ConditionProgressing,
				Status:  corev1.ConditionTrue,
				Reason:  "KubevirtTemplateValidatorProgressing",
				Message: "KubevirtTemplateValidator is progressing: Bar",
			})))
			Expect(req.conditions[]).To(ContainElement(testlib.RepresentCondition(conditionsv1.Condition{
				Type:    conditionsv1.ConditionUpgradeable,
				Status:  corev1.ConditionFalse,
				Reason:  "KubevirtTemplateValidatorProgressing",
				Message: "KubevirtTemplateValidator is progressing: Bar",
			})))
			Expect(req.conditions[]).To(ContainElement(testlib.RepresentCondition(conditionsv1.Condition{
				Type:    conditionsv1.ConditionDegraded,
				Status:  corev1.ConditionTrue,
				Reason:  "KubevirtTemplateValidatorDegraded",
				Message: "KubevirtTemplateValidator is degraded: Bar",
			})))
		})*/
	})

	Context("KubeVirtMetricsAggregation", func() {
		var hco *hcov1beta1.HyperConverged
		var req *hcoRequest

		BeforeEach(func() {
			hco = newHco()
			req = newReq(hco)
		})

		It("should create if not present", func() {
			expectedResource := newKubeVirtMetricsAggregationForCR(hco, namespace)
			cl := initClient([]runtime.Object{})
			r := initReconciler(cl)
			res := r.ensureKubeVirtMetricsAggregation(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			foundResource := &sspv1.KubevirtMetricsAggregation{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: expectedResource.Name, Namespace: expectedResource.Namespace},
					foundResource),
			).To(BeNil())
			Expect(foundResource.Name).To(Equal(expectedResource.Name))
			Expect(foundResource.Labels).Should(HaveKeyWithValue(hcoutil.AppLabel, name))
			Expect(foundResource.Namespace).To(Equal(expectedResource.Namespace))
		})

		It("should find if present", func() {
			expectedResource := newKubeVirtMetricsAggregationForCR(hco, namespace)
			expectedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", expectedResource.Namespace, expectedResource.Name)
			cl := initClient([]runtime.Object{hco, expectedResource})
			r := initReconciler(cl)
			res := r.ensureKubeVirtMetricsAggregation(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			// Check HCO's status
			Expect(hco.Status.RelatedObjects).To(Not(BeNil()))
			objectRef, err := reference.GetReference(r.scheme, expectedResource)
			Expect(err).To(BeNil())
			// ObjectReference should have been added
			Expect(hco.Status.RelatedObjects).To(ContainElement(*objectRef))
		})

		It("should reconcile to default", func() {
			existingResource := newKubeVirtMetricsAggregationForCR(hco, namespace)
			existingResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", existingResource.Namespace, existingResource.Name)

			existingResource.Spec.Version = "non-default value"

			cl := initClient([]runtime.Object{hco, existingResource})
			r := initReconciler(cl)
			res := r.ensureKubeVirtMetricsAggregation(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			foundResource := &sspv1.KubevirtMetricsAggregation{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
					foundResource),
			).To(BeNil())
			Expect(foundResource.Spec.Version).To(BeEmpty())
		})

		// TODO: add tests to ensure that HCO properly propagates NodePlacement from its CR

		// TODO: temporary avoid checking conditions on KubevirtTemplateValidator because it's currently
		// broken on k8s. Revert this when we will be able to fix it
		/*It("should handle conditions", func() {
			expectedResource := newKubeVirtTemplateValidatorForCR(hco, namespace)
			expectedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", expectedResource.Namespace, expectedResource.Name)
			expectedResource.Status.Conditions = []conditionsv1.Condition{
				conditionsv1.Condition{
					Type:    conditionsv1.ConditionAvailable,
					Status:  corev1.ConditionFalse,
					Reason:  "Foo",
					Message: "Bar",
				},
				conditionsv1.Condition{
					Type:    conditionsv1.ConditionProgressing,
					Status:  corev1.ConditionTrue,
					Reason:  "Foo",
					Message: "Bar",
				},
				conditionsv1.Condition{
					Type:    conditionsv1.ConditionDegraded,
					Status:  corev1.ConditionTrue,
					Reason:  "Foo",
					Message: "Bar",
				},
			}
			cl := initClient([]runtime.Object{hco, expectedResource})
			r := initReconciler(cl)
			Expect(r.ensureKubeVirtTemplateValidator(req)).To(BeNil())

			// Check HCO's status
			Expect(hco.Status.RelatedObjects).To(Not(BeNil()))
			objectRef, err := reference.GetReference(r.scheme, expectedResource)
			Expect(err).To(BeNil())
			// ObjectReference should have been added
			Expect(hco.Status.RelatedObjects).To(ContainElement(*objectRef))
			// Check conditions
			Expect(req.conditions[]).To(ContainElement(testlib.RepresentCondition(conditionsv1.Condition{
				Type:    conditionsv1.ConditionAvailable,
				Status:  corev1.ConditionFalse,
				Reason:  "KubevirtTemplateValidatorNotAvailable",
				Message: "KubevirtTemplateValidator is not available: Bar",
			})))
			Expect(req.conditions[]).To(ContainElement(testlib.RepresentCondition(conditionsv1.Condition{
				Type:    conditionsv1.ConditionProgressing,
				Status:  corev1.ConditionTrue,
				Reason:  "KubevirtTemplateValidatorProgressing",
				Message: "KubevirtTemplateValidator is progressing: Bar",
			})))
			Expect(req.conditions[]).To(ContainElement(testlib.RepresentCondition(conditionsv1.Condition{
				Type:    conditionsv1.ConditionUpgradeable,
				Status:  corev1.ConditionFalse,
				Reason:  "KubevirtTemplateValidatorProgressing",
				Message: "KubevirtTemplateValidator is progressing: Bar",
			})))
			Expect(req.conditions[]).To(ContainElement(testlib.RepresentCondition(conditionsv1.Condition{
				Type:    conditionsv1.ConditionDegraded,
				Status:  corev1.ConditionTrue,
				Reason:  "KubevirtTemplateValidatorDegraded",
				Message: "KubevirtTemplateValidator is degraded: Bar",
			})))
		})*/
	})

	Context("Manage IMS Config", func() {

		var hco *hcov1beta1.HyperConverged
		var req *hcoRequest

		BeforeEach(func() {
			os.Setenv("CONVERSION_CONTAINER", "new-conversion-container-value")
			os.Setenv("VMWARE_CONTAINER", "new-vmware-container-value")
			hco = newHco()
			req = newReq(hco)
		})

		It("should error if environment vars not specified", func() {
			os.Unsetenv("CONVERSION_CONTAINER")
			os.Unsetenv("VMWARE_CONTAINER")

			cl := initClient([]runtime.Object{})
			r := initReconciler(cl)
			res := r.ensureIMSConfig(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).ToNot(BeNil())
		})

		It("should create if not present", func() {
			expectedResource := newIMSConfigForCR(hco, namespace)
			cl := initClient([]runtime.Object{})
			r := initReconciler(cl)
			res := r.ensureIMSConfig(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			foundResource := &corev1.ConfigMap{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: expectedResource.Name, Namespace: expectedResource.Namespace},
					foundResource),
			).To(BeNil())
			Expect(foundResource.Name).To(Equal(expectedResource.Name))
			Expect(foundResource.Labels).Should(HaveKeyWithValue(hcoutil.AppLabel, name))
			Expect(foundResource.Namespace).To(Equal(expectedResource.Namespace))
		})

		It("should find if present", func() {
			expectedResource := newIMSConfigForCR(hco, namespace)
			expectedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", expectedResource.Namespace, expectedResource.Name)
			cl := initClient([]runtime.Object{hco, expectedResource})
			r := initReconciler(cl)
			res := r.ensureIMSConfig(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			// Check HCO's status
			Expect(hco.Status.RelatedObjects).To(Not(BeNil()))
			objectRef, err := reference.GetReference(r.scheme, expectedResource)
			Expect(err).To(BeNil())
			// ObjectReference should have been added
			Expect(hco.Status.RelatedObjects).To(ContainElement(*objectRef))
		})

		// in an ideal world HCO should be managing the whole config map,
		// now due to a bad design only a few values of this config map are
		// really managed by HCO while others are managed by other entities
		// TODO: fix this bad design splitting the config map into two distinct objects and reconcile the whole object here
		It("should (partially!!!) reconcile according to env values", func() {
			convk := "v2v-conversion-image"
			vmwarek := "kubevirt-vmware-image"
			updatableKeys := [...]string{convk, vmwarek}
			unupdatableKeyValues := map[string]string{
				"ext_key_1": "ext_value_1",
				"ext_key_2": "ext_value_2",
			}

			expectedResource := newIMSConfigForCR(hco, namespace)
			expectedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", expectedResource.Namespace, expectedResource.Name)
			outdatedResource := newIMSConfigForCR(hco, namespace)
			outdatedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", outdatedResource.Namespace, outdatedResource.Name)
			// values we should update
			outdatedResource.Data[convk] = "old-conversion-container-value-we-have-to-update"
			outdatedResource.Data[vmwarek] = "old-vmware-container-value-we-have-to-update"
			// add values we should not touch
			for k, v := range unupdatableKeyValues {
				outdatedResource.Data[k] = v
			}

			cl := initClient([]runtime.Object{hco, outdatedResource})
			r := initReconciler(cl)

			res := r.ensureIMSConfig(req)
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

			for k, v := range unupdatableKeyValues {
				Expect(foundResource.Data).To(HaveKeyWithValue(k, v))
			}

		})

	})

	Context("Vm Import", func() {

		var hco *hcov1beta1.HyperConverged
		var req *hcoRequest

		BeforeEach(func() {
			hco = newHco()
			req = newReq(hco)
		})

		It("should create if not present", func() {
			expectedResource := newVMImportForCR(hco, namespace)
			cl := initClient([]runtime.Object{})
			r := initReconciler(cl)

			res := r.ensureVMImport(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			foundResource := &vmimportv1beta1.VMImportConfig{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: expectedResource.Name, Namespace: expectedResource.Namespace},
					foundResource),
			).To(BeNil())
			Expect(foundResource.Name).To(Equal(expectedResource.Name))
			Expect(foundResource.Labels).Should(HaveKeyWithValue(hcoutil.AppLabel, name))
			Expect(foundResource.Namespace).To(Equal(expectedResource.Namespace))
		})

		It("should find if present", func() {
			expectedResource := newVMImportForCR(hco, namespace)
			expectedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/vmimportconfigs/%s", expectedResource.Namespace, expectedResource.Name)
			cl := initClient([]runtime.Object{hco, expectedResource})
			r := initReconciler(cl)
			res := r.ensureVMImport(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			// Check HCO's status
			Expect(hco.Status.RelatedObjects).To(Not(BeNil()))
			objectRef, err := reference.GetReference(r.scheme, expectedResource)
			Expect(err).To(BeNil())
			// ObjectReference should have been added
			Expect(hco.Status.RelatedObjects).To(ContainElement(*objectRef))
		})

		It("should reconcile to default", func() {
			existingResource := newVMImportForCR(hco, namespace)
			existingResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", existingResource.Namespace, existingResource.Name)

			existingResource.Spec.ImagePullPolicy = corev1.PullAlways // set non-default value

			cl := initClient([]runtime.Object{hco, existingResource})
			r := initReconciler(cl)
			res := r.ensureVMImport(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			foundResource := &vmimportv1beta1.VMImportConfig{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
					foundResource),
			).To(BeNil())
			Expect(foundResource.Spec.ImagePullPolicy).To(BeEmpty())
		})

		It("should add node placement if missing in VM-Import", func() {
			existingResource := newVMImportForCR(hco, namespace)

			hco.Spec.Infra = hcov1beta1.HyperConvergedConfig{NodePlacement: NewHyperConvergedConfig()}
			hco.Spec.Workloads = hcov1beta1.HyperConvergedConfig{NodePlacement: NewHyperConvergedConfig()}

			cl := initClient([]runtime.Object{hco, existingResource})
			r := initReconciler(cl)
			res := r.ensureVMImport(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			foundResource := &vmimportv1beta1.VMImportConfig{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
					foundResource),
			).To(BeNil())

			Expect(existingResource.Spec.Infra.Affinity).To(BeNil())
			Expect(existingResource.Spec.Infra.NodeSelector).To(BeEmpty())
			Expect(existingResource.Spec.Infra.Tolerations).To(BeEmpty())

			Expect(foundResource.Spec.Infra.Affinity).ToNot(BeNil())
			Expect(foundResource.Spec.Infra.NodeSelector).ToNot(BeEmpty())
			Expect(foundResource.Spec.Infra.Tolerations).ToNot(BeEmpty())

			infra := foundResource.Spec.Infra
			Expect(infra.NodeSelector["key1"]).Should(Equal("value1"))
			Expect(infra.NodeSelector["key2"]).Should(Equal("value2"))

			reflect.DeepEqual(infra.Tolerations, hco.Spec.Infra.NodePlacement.Tolerations)
			reflect.DeepEqual(infra.Affinity, hco.Spec.Infra.NodePlacement.Affinity)

			Expect(req.conditions).To(BeEmpty())
		})

		It("should remove node placement if missing in HCO CR", func() {

			hcoNodePlacement := newHco()
			hcoNodePlacement.Spec.Infra = hcov1beta1.HyperConvergedConfig{NodePlacement: NewHyperConvergedConfig()}
			hcoNodePlacement.Spec.Workloads = hcov1beta1.HyperConvergedConfig{NodePlacement: NewHyperConvergedConfig()}
			existingResource := newVMImportForCR(hcoNodePlacement, namespace)

			cl := initClient([]runtime.Object{hco, existingResource})
			r := initReconciler(cl)
			res := r.ensureVMImport(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			foundResource := &vmimportv1beta1.VMImportConfig{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
					foundResource),
			).To(BeNil())

			Expect(existingResource.Spec.Infra.Affinity.NodeAffinity).ToNot(BeNil())
			Expect(existingResource.Spec.Infra.NodeSelector).ToNot(BeEmpty())
			Expect(existingResource.Spec.Infra.Tolerations).ToNot(BeEmpty())

			Expect(foundResource.Spec.Infra.Affinity).To(BeNil())
			Expect(foundResource.Spec.Infra.NodeSelector).To(BeEmpty())
			Expect(foundResource.Spec.Infra.Tolerations).To(BeEmpty())

			Expect(req.conditions).To(BeEmpty())
		})

		It("should modify node placement according to HCO CR", func() {

			hco.Spec.Infra = hcov1beta1.HyperConvergedConfig{NodePlacement: NewHyperConvergedConfig()}
			hco.Spec.Workloads = hcov1beta1.HyperConvergedConfig{NodePlacement: NewHyperConvergedConfig()}
			existingResource := newVMImportForCR(hco, namespace)

			// now, modify HCO's node placement
			seconds3 := int64(3)
			hco.Spec.Infra.NodePlacement.Tolerations = append(hco.Spec.Infra.NodePlacement.Tolerations, corev1.Toleration{
				Key: "key3", Operator: "operator3", Value: "value3", Effect: "effect3", TolerationSeconds: &seconds3,
			})

			hco.Spec.Infra.NodePlacement.NodeSelector["key1"] = "something else"

			cl := initClient([]runtime.Object{hco, existingResource})
			r := initReconciler(cl)
			res := r.ensureVMImport(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			foundResource := &vmimportv1beta1.VMImportConfig{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
					foundResource),
			).To(BeNil())

			Expect(existingResource.Spec.Infra.Affinity).ToNot(BeNil())
			Expect(existingResource.Spec.Infra.Tolerations).To(HaveLen(2))
			Expect(existingResource.Spec.Infra.NodeSelector["key1"]).Should(Equal("value1"))

			Expect(foundResource.Spec.Infra.Affinity).ToNot(BeNil())
			Expect(foundResource.Spec.Infra.Tolerations).To(HaveLen(3))
			Expect(foundResource.Spec.Infra.NodeSelector["key1"]).Should(Equal("something else"))

			Expect(req.conditions).To(BeEmpty())
		})
	})

	Context("ConsoleCLIDownload", func() {

		var hco *hcov1beta1.HyperConverged
		var req *hcoRequest

		BeforeEach(func() {
			hco = newHco()
			req = newReq(hco)
		})

		It("should create if not present", func() {
			expectedResource := hco.NewConsoleCLIDownload()
			cl := initClient([]runtime.Object{})
			r := initReconciler(cl)

			err := r.ensureConsoleCLIDownload(req)
			Expect(err).To(BeNil())

			foundResource := &consolev1.ConsoleCLIDownload{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: expectedResource.Name, Namespace: expectedResource.Namespace},
					foundResource),
			).To(BeNil())
			Expect(foundResource.Name).To(Equal(expectedResource.Name))
			Expect(foundResource.Labels).Should(HaveKeyWithValue(hcoutil.AppLabel, name))
			Expect(foundResource.Namespace).To(Equal(expectedResource.Namespace))
		})

		It("should find if present", func() {
			expectedResource := hco.NewConsoleCLIDownload()
			expectedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/consoleclidownloads/%s", expectedResource.Namespace, expectedResource.Name)
			cl := initClient([]runtime.Object{hco, expectedResource})
			r := initReconciler(cl)
			err := r.ensureConsoleCLIDownload(req)
			Expect(err).To(BeNil())

			// Check HCO's status
			Expect(hco.Status.RelatedObjects).To(Not(BeNil()))
			objectRef, err := reference.GetReference(r.scheme, expectedResource)
			Expect(err).To(BeNil())
			// ObjectReference should have been added
			Expect(hco.Status.RelatedObjects).To(ContainElement(*objectRef))
		})

		DescribeTable("should update if something changed", func(modifiedResource *consolev1.ConsoleCLIDownload) {
			os.Setenv(hcoutil.KubevirtVersionEnvV, "100")
			cl := initClient([]runtime.Object{modifiedResource})
			r := initReconciler(cl)
			err := r.ensureConsoleCLIDownload(req)
			Expect(err).To(BeNil())
			expectedResource := hco.NewConsoleCLIDownload()
			key, err := client.ObjectKeyFromObject(expectedResource)
			Expect(err).ToNot(HaveOccurred())
			foundResource := &consolev1.ConsoleCLIDownload{}
			Expect(cl.Get(context.TODO(), key, foundResource))
			Expect(foundResource.Spec.Links[0].Href).To(Equal(expectedResource.Spec.Links[0].Href))
			Expect(foundResource.Spec.Links[0].Text).To(Equal(expectedResource.Spec.Links[0].Text))
		},
			Entry("with modified download link",
				&consolev1.ConsoleCLIDownload{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "console.openshift.io/v1",
						Kind:       "ConsoleCLIDownload",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "virtctl-clidownloads-kubevirt-hyperconverged",
					},

					Spec: consolev1.ConsoleCLIDownloadSpec{
						Links: []consolev1.CLIDownloadLink{
							{
								Href: "https://dummy.url1.com",
								Text: "KubeVirt 100 release downloads",
							},
						},
					},
				}),
			Entry("with modified download text",
				&consolev1.ConsoleCLIDownload{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "console.openshift.io/v1",
						Kind:       "ConsoleCLIDownload",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "virtctl-clidownloads-kubevirt-hyperconverged",
					},
					Spec: consolev1.ConsoleCLIDownloadSpec{
						Links: []consolev1.CLIDownloadLink{
							{
								Href: "https://github.com/kubevirt/kubevirt/releases/100",
								Text: "dummy text 1",
							},
						},
					},
				}),
		)
	})
})
