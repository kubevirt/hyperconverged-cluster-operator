package hyperconverged

import (
	rbacv1 "k8s.io/api/rbac/v1"
	"os"

	networkaddonsv1alpha1 "github.com/kubevirt/cluster-network-addons-operator/pkg/apis/networkaddonsoperator/v1alpha1"
	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/pkg/apis/hco/v1beta1"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
	sspv1 "github.com/kubevirt/kubevirt-ssp-operator/pkg/apis/kubevirt/v1"
	vmimportv1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1alpha1"
	consolev1 "github.com/openshift/api/console/v1"
	conditionsv1 "github.com/openshift/custom-resource-status/conditions/v1"
	"github.com/openshift/custom-resource-status/testlib"
	corev1 "k8s.io/api/core/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kubevirtv1 "kubevirt.io/client-go/api/v1"
	cdiv1alpha1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
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

		It("should update if something changed", func() {
			expectedResource := newKubeVirtConfigForCR(hco, namespace)
			expectedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", expectedResource.Namespace, expectedResource.Name)
			outdatedResource := newKubeVirtConfigForCR(hco, namespace)
			outdatedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", outdatedResource.Namespace, outdatedResource.Name)
			// values we should update
			outdatedResource.Data[virtconfig.SmbiosConfigKey] = "old-smbios-value-that-we-have-to-update"
			outdatedResource.Data[virtconfig.MachineTypeKey] = "old-machinetype-value-that-we-have-to-update"
			outdatedResource.Data[virtconfig.SELinuxLauncherTypeKey] = "old-selinuxlauncher-value-that-we-have-to-update"
			outdatedResource.Data[virtconfig.FeatureGatesKey] = "old-featuregates-value-that-we-have-to-update"
			outdatedResource.Data[virtconfig.MigrationsConfigKey] = "old-migrationsconfig-value-that-we-have-to-update"
			outdatedResource.Data[virtconfig.NetworkInterfaceKey] = "old-defaultnetworkinterface-value-that-we-have-to-update"

			cl := initClient([]runtime.Object{hco, outdatedResource})
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

			Expect(foundResource.Data).To(Not(Equal(outdatedResource.Data)))
			Expect(foundResource.Data).To(Equal(expectedResource.Data))
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

		It("should update if something changed", func() {
			expectedResource := newKubeVirtStorageConfigForCR(hco, namespace)
			expectedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", expectedResource.Namespace, expectedResource.Name)
			outdatedResource := newKubeVirtStorageConfigForCR(hco, namespace)
			outdatedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", outdatedResource.Namespace, outdatedResource.Name)
			// values we should update
			outdatedResource.Data["accessMode"] = "old-accessMode-value-that-we-have-to-update"
			outdatedResource.Data["volumeMode"] = "old-volumeMode-value-that-we-have-to-update"
			outdatedResource.Data["local-sc.accessMode"] = "old-local-sc.accessMode-value-that-we-have-to-update"
			outdatedResource.Data["local-sc.volumeMode"] = "old-local-sc.volumeMode-value-that-we-have-to-update"

			cl := initClient([]runtime.Object{hco, outdatedResource})
			r := initReconciler(cl)

			err := r.ensureKubeVirtStorageConfig(req)
			Expect(err).To(BeNil())

			foundResource := &corev1.ConfigMap{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: expectedResource.Name, Namespace: expectedResource.Namespace},
					foundResource),
			).To(BeNil())

			Expect(foundResource.Data).To(Not(Equal(outdatedResource.Data)))
			Expect(foundResource.Data).To(Equal(expectedResource.Data))
		})

	})

	Context("KubeVirt Storage Role", func() {
		var hco *hcov1beta1.HyperConverged
		var req *hcoRequest

		BeforeEach(func() {
			hco = newHco()
			req = newReq(hco)
		})

		It("should create if not present", func() {
			expectedResource := newKubeVirtStorageRoleForCR(hco, namespace)
			cl := initClient([]runtime.Object{})
			r := initReconciler(cl)
			err := r.ensureKubeVirtStorageRole(req)
			Expect(err).To(BeNil())

			foundResource := &rbacv1.Role{}
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
			expectedResource := newKubeVirtStorageRoleForCR(hco, namespace)
			expectedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", expectedResource.Namespace, expectedResource.Name)
			cl := initClient([]runtime.Object{hco, expectedResource})
			r := initReconciler(cl)
			err := r.ensureKubeVirtStorageRole(req)
			Expect(err).To(BeNil())

			// Check HCO's status
			Expect(hco.Status.RelatedObjects).To(Not(BeNil()))
			objectRef, err := reference.GetReference(r.scheme, expectedResource)
			Expect(err).To(BeNil())
			// ObjectReference should have been added
			Expect(hco.Status.RelatedObjects).To(ContainElement(*objectRef))
		})

		It("should update if something changed", func() {
			expectedResource := newKubeVirtStorageRoleForCR(hco, namespace)
			expectedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", expectedResource.Namespace, expectedResource.Name)
			outdatedResource := newKubeVirtStorageRoleForCR(hco, namespace)
			outdatedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", outdatedResource.Namespace, outdatedResource.Name)
			// values we should update
			outdatedResource.Rules[0].APIGroups = []string{"test"}
			outdatedResource.Rules[0].Resources = []string{"test"}
			outdatedResource.Rules[0].ResourceNames = []string{"test"}
			outdatedResource.Rules[0].Verbs = []string{"get"}

			cl := initClient([]runtime.Object{hco, outdatedResource})
			r := initReconciler(cl)

			err := r.ensureKubeVirtStorageRole(req)
			Expect(err).To(BeNil())

			foundResource := &rbacv1.Role{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: expectedResource.Name, Namespace: expectedResource.Namespace},
					foundResource),
			).To(BeNil())

			Expect(foundResource.Rules).To(Not(Equal(outdatedResource.Rules)))
			Expect(foundResource.Rules).To(Equal(expectedResource.Rules))
		})

	})

	Context("KubeVirt Storage Role Binding", func() {
		var hco *hcov1beta1.HyperConverged
		var req *hcoRequest

		BeforeEach(func() {
			hco = newHco()
			req = newReq(hco)
		})

		It("should create if not present", func() {
			expectedResource := newKubeVirtStorageRoleBindingForCR(hco, namespace)
			cl := initClient([]runtime.Object{})
			r := initReconciler(cl)
			err := r.ensureKubeVirtStorageRoleBinding(req)
			Expect(err).To(BeNil())

			foundResource := &rbacv1.RoleBinding{}
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
			expectedResource := newKubeVirtStorageRoleBindingForCR(hco, namespace)
			expectedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", expectedResource.Namespace, expectedResource.Name)
			cl := initClient([]runtime.Object{hco, expectedResource})
			r := initReconciler(cl)
			err := r.ensureKubeVirtStorageRoleBinding(req)
			Expect(err).To(BeNil())

			// Check HCO's status
			Expect(hco.Status.RelatedObjects).To(Not(BeNil()))
			objectRef, err := reference.GetReference(r.scheme, expectedResource)
			Expect(err).To(BeNil())
			// ObjectReference should have been added
			Expect(hco.Status.RelatedObjects).To(ContainElement(*objectRef))
		})

		It("should update if something changed", func() {
			expectedResource := newKubeVirtStorageRoleBindingForCR(hco, namespace)
			expectedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", expectedResource.Namespace, expectedResource.Name)
			outdatedResource := newKubeVirtStorageRoleBindingForCR(hco, namespace)
			outdatedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", outdatedResource.Namespace, outdatedResource.Name)
			// values we should update
			outdatedResource.RoleRef.APIGroup = "testAPIGroup"
			outdatedResource.RoleRef.Kind = "testKind"
			outdatedResource.RoleRef.Name = "testName"
			outdatedResource.Subjects[0].APIGroup = "testAPIGroup"
			outdatedResource.Subjects[0].Kind = "testKind"
			outdatedResource.Subjects[0].Name = "testName"

			cl := initClient([]runtime.Object{hco, outdatedResource})
			r := initReconciler(cl)

			err := r.ensureKubeVirtStorageRoleBinding(req)
			Expect(err).To(BeNil())

			foundResource := &rbacv1.RoleBinding{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: expectedResource.Name, Namespace: expectedResource.Namespace},
					foundResource),
			).To(BeNil())

			Expect(foundResource.RoleRef).To(Not(Equal(outdatedResource.RoleRef)))
			Expect(foundResource.RoleRef).To(Equal(expectedResource.RoleRef))
			Expect(foundResource.Subjects).To(Not(Equal(outdatedResource.Subjects)))
			Expect(foundResource.Subjects).To(Equal(expectedResource.Subjects))
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

		It("should update if something changed", func() {
			expectedResource := hco.NewKubeVirt(namespace)
			expectedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", expectedResource.Namespace, expectedResource.Name)
			outdatedResource := hco.NewKubeVirt(namespace)
			outdatedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", outdatedResource.Namespace, outdatedResource.Name)
			// values we should update
			outdatedResource.Spec.UninstallStrategy = "old-uninstall-strategy-value-that-we-have-to-update"

			cl := initClient([]runtime.Object{hco, outdatedResource})
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

			Expect(foundResource.Spec).To(Not(Equal(outdatedResource.Spec)))
			Expect(foundResource.Spec).To(Equal(expectedResource.Spec))
		})

		// TODO: add tests to ensure that HCO properly propagates NodePlacement from its CR

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

			foundResource := &cdiv1alpha1.CDI{}
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

		It("should update if something changed", func() {
			expectedResource := hco.NewCDI(namespace)
			expectedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", expectedResource.Namespace, expectedResource.Name)
			outdatedResource := hco.NewCDI(namespace)
			outdatedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", outdatedResource.Namespace, outdatedResource.Name)
			// values we should update
			outdatedResource.Spec.UninstallStrategy = nil

			cl := initClient([]runtime.Object{hco, outdatedResource})
			r := initReconciler(cl)

			res := r.ensureCDI(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			foundResource := &cdiv1alpha1.CDI{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: expectedResource.Name, Namespace: expectedResource.Namespace},
					foundResource),
			).To(BeNil())

			Expect(foundResource.Spec).To(Not(Equal(outdatedResource.Spec)))
			Expect(foundResource.Spec).To(Equal(expectedResource.Spec))
		})

		// TODO: add tests to ensure that HCO properly propagates NodePlacement from its CR

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

			foundResource := &networkaddonsv1alpha1.NetworkAddonsConfig{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: expectedResource.Name, Namespace: expectedResource.Namespace},
					foundResource),
			).To(BeNil())
			Expect(foundResource.Name).To(Equal(expectedResource.Name))
			Expect(foundResource.Labels).Should(HaveKeyWithValue(hcoutil.AppLabel, name))
			Expect(foundResource.Namespace).To(Equal(expectedResource.Namespace))
			Expect(foundResource.Spec.Multus).To(Equal(&networkaddonsv1alpha1.Multus{}))
			Expect(foundResource.Spec.LinuxBridge).To(Equal(&networkaddonsv1alpha1.LinuxBridge{}))
			Expect(foundResource.Spec.KubeMacPool).To(Equal(&networkaddonsv1alpha1.KubeMacPool{}))
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

		It("should update if something changed", func() {
			expectedResource := hco.NewNetworkAddons(namespace)
			expectedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", expectedResource.Namespace, expectedResource.Name)
			outdatedResource := hco.NewNetworkAddons(namespace)
			outdatedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", outdatedResource.Namespace, outdatedResource.Name)
			// values we should update
			outdatedResource.Spec.Multus = nil
			outdatedResource.Spec.LinuxBridge = nil
			outdatedResource.Spec.Ovs = nil
			outdatedResource.Spec.NMState = nil
			outdatedResource.Spec.KubeMacPool = nil

			cl := initClient([]runtime.Object{hco, outdatedResource})
			r := initReconciler(cl)

			res := r.ensureNetworkAddons(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			foundResource := &networkaddonsv1alpha1.NetworkAddonsConfig{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: expectedResource.Name, Namespace: expectedResource.Namespace},
					foundResource),
			).To(BeNil())

			Expect(foundResource.Spec).To(Not(Equal(outdatedResource.Spec)))
			Expect(foundResource.Spec).To(Equal(expectedResource.Spec))
		})

		// TODO: add tests to ensure that HCO properly propagates NodePlacement from its CR

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

		It("should update if something changed", func() {
			expectedResource := hco.NewKubeVirtCommonTemplateBundle()
			expectedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", expectedResource.Namespace, expectedResource.Name)
			outdatedResource := hco.NewKubeVirtCommonTemplateBundle()
			outdatedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", outdatedResource.Namespace, outdatedResource.Name)
			// values we should update
			outdatedResource.Spec.Version = "v1-test"

			cl := initClient([]runtime.Object{hco, outdatedResource})
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

			Expect(foundResource.Spec).To(Not(Equal(outdatedResource.Spec)))
			Expect(foundResource.Spec).To(Equal(expectedResource.Spec))
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

		It("should update if something changed", func() {
			expectedResource := newKubeVirtNodeLabellerBundleForCR(hco, namespace)
			expectedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", expectedResource.Namespace, expectedResource.Name)
			outdatedResource := newKubeVirtNodeLabellerBundleForCR(hco, namespace)
			outdatedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", outdatedResource.Namespace, outdatedResource.Name)
			// values we should update
			outdatedResource.Spec.Version = "v1-test"

			cl := initClient([]runtime.Object{hco, outdatedResource})
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

			Expect(foundResource.Spec).To(Not(Equal(outdatedResource.Spec)))
			Expect(foundResource.Spec).To(Equal(expectedResource.Spec))
		})

		// TODO: add tests to ensure that HCO properly propagates NodePlacement from its CR

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

		It("should update if something changed", func() {
			expectedResource := newKubeVirtTemplateValidatorForCR(hco, namespace)
			expectedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", expectedResource.Namespace, expectedResource.Name)
			outdatedResource := newKubeVirtTemplateValidatorForCR(hco, namespace)
			outdatedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", outdatedResource.Namespace, outdatedResource.Name)
			// values we should update
			outdatedResource.Spec.Version = "v1-test"

			cl := initClient([]runtime.Object{hco, outdatedResource})
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

			Expect(foundResource.Spec).To(Not(Equal(outdatedResource.Spec)))
			Expect(foundResource.Spec).To(Equal(expectedResource.Spec))
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

		It("should update if something changed", func() {
			os.Setenv("CONVERSION_CONTAINER", "new-conversion-image")
			os.Setenv("VMWARE_CONTAINER", "new-vmwarec-image")
			expectedResource := newIMSConfigForCR(hco, namespace)
			expectedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", expectedResource.Namespace, expectedResource.Name)
			outdatedResource := newIMSConfigForCR(hco, namespace)
			outdatedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", outdatedResource.Namespace, outdatedResource.Name)
			// values we should update
			outdatedResource.Data["CONVERSION_CONTAINER"] = "old-conversion-image"
			outdatedResource.Data["VMWARE_CONTAINER"] = "old-vmwarec-image"

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

			Expect(foundResource.Data).To(Not(Equal(outdatedResource.Data)))
			Expect(foundResource.Data).To(Equal(expectedResource.Data))
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

			foundResource := &vmimportv1.VMImportConfig{}
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

		It("should update if something changed", func() {
			expectedResource := newVMImportForCR(hco, namespace)
			expectedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", expectedResource.Namespace, expectedResource.Name)
			outdatedResource := newVMImportForCR(hco, namespace)
			outdatedResource.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/dummies/%s", outdatedResource.Namespace, outdatedResource.Name)
			// values we should update
			outdatedResource.Spec.ImagePullPolicy = "old-value"

			cl := initClient([]runtime.Object{hco, outdatedResource})
			r := initReconciler(cl)

			res := r.ensureVMImport(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

			foundResource := &vmimportv1.VMImportConfig{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: expectedResource.Name, Namespace: expectedResource.Namespace},
					foundResource),
			).To(BeNil())

			Expect(foundResource.Spec).To(Not(Equal(outdatedResource.Spec)))
			Expect(foundResource.Spec).To(Equal(expectedResource.Spec))
		})

		// TODO: add tests to ensure that HCO properly propagates NodePlacement from its CR

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

			res := r.ensureConsoleCLIDownload(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

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
			res := r.ensureConsoleCLIDownload(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())

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
			res := r.ensureConsoleCLIDownload(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Err).To(BeNil())
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
