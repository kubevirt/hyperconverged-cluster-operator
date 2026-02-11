package hyperconverged

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	openshiftconfigv1 "github.com/openshift/api/config/v1"
	consolev1 "github.com/openshift/api/console/v1"
	conditionsv1 "github.com/openshift/custom-resource-status/conditions/v1"
	objectreferencesv1 "github.com/openshift/custom-resource-status/objectreferences/v1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimetav1 "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/reference"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kubevirtcorev1 "kubevirt.io/api/core/v1"
	cdiv1beta1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	sspv1beta3 "kubevirt.io/ssp-operator/api/v1beta3"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/alerts"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/handlers"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/reqresolver"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/monitoring/hyperconverged/metrics"
	fakeownresources "github.com/kubevirt/hyperconverged-cluster-operator/pkg/ownresources/fake"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/tlssecprofile"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
	"github.com/kubevirt/hyperconverged-cluster-operator/version"
)

// name and namespace of our primary resource
const (
	name      = "kubevirt-hyperconverged"
	namespace = "kubevirt-hyperconverged"
)

var _ = Describe("HyperconvergedController", func() {

	getClusterInfo := hcoutil.GetClusterInfo

	origOperatorCondVarName := os.Getenv(hcoutil.OperatorConditionNameEnvVar)
	origVirtIOWinContainer := os.Getenv("VIRTIOWIN_CONTAINER")
	origOperatorNS := os.Getenv("OPERATOR_NAMESPACE")
	origVersion := os.Getenv(hcoutil.HcoKvIoVersionName)

	BeforeEach(func() {
		hcoutil.GetClusterInfo = func() hcoutil.ClusterInfo {
			return commontestutils.ClusterInfoMock{}
		}
		fakeownresources.OLMV0OwnResourcesMock()

		Expect(os.Setenv(hcoutil.OperatorConditionNameEnvVar, "OPERATOR_CONDITION")).To(Succeed())
		Expect(os.Setenv("VIRTIOWIN_CONTAINER", commontestutils.VirtioWinImage)).To(Succeed())
		Expect(os.Setenv("OPERATOR_NAMESPACE", namespace)).To(Succeed())
		Expect(os.Setenv(hcoutil.HcoKvIoVersionName, version.Version)).To(Succeed())

		reqresolver.GeneratePlaceHolders()

		DeferCleanup(func() {
			hcoutil.GetClusterInfo = getClusterInfo
			fakeownresources.ResetOwnResources()

			Expect(os.Setenv(hcoutil.OperatorConditionNameEnvVar, origOperatorCondVarName)).To(Succeed())
			Expect(os.Setenv("VIRTIOWIN_CONTAINER", origVirtIOWinContainer)).To(Succeed())
			Expect(os.Setenv("OPERATOR_NAMESPACE", origOperatorNS)).To(Succeed())
			Expect(os.Setenv(hcoutil.HcoKvIoVersionName, origVersion)).To(Succeed())
		})
	})

	Describe("Reconcile HyperConverged", func() {

		Context("HCO Lifecycle", func() {

			var (
				hcoNamespace *corev1.Namespace
			)

			BeforeEach(func() {
				hcoNamespace = commontestutils.NewHcoNamespace()
			})

			It("should handle not found", func() {
				cl := commontestutils.InitClient([]client.Object{})
				r := initReconciler(cl, nil)

				res, err := r.Reconcile(context.TODO(), request)
				Expect(err).ToNot(HaveOccurred())
				Expect(res).To(Equal(reconcile.Result{}))
				validateOperatorCondition(r, metav1.ConditionTrue, hcoutil.UpgradeableAllowReason, hcoutil.UpgradeableAllowMessage)
				verifyHyperConvergedCRExistsMetricFalse()
			})

			It("should ignore invalid requests", func() {
				hco := commontestutils.NewHco()
				hco.ObjectMeta = metav1.ObjectMeta{
					Name:      "invalid",
					Namespace: "invalid",
				}
				cl := commontestutils.InitClient([]client.Object{hcoNamespace, hco})
				r := initReconciler(cl, nil)

				// Do the reconcile
				var invalidRequest = reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      "invalid",
						Namespace: "invalid",
					},
				}
				res, err := r.Reconcile(context.TODO(), invalidRequest)
				Expect(err).ToNot(HaveOccurred())
				Expect(res).To(Equal(reconcile.Result{}))

				// Get the HCO
				foundResource := &hcov1beta1.HyperConverged{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: hco.Name, Namespace: hco.Namespace},
						foundResource),
				).ToNot(HaveOccurred())
				// Check conditions
				Expect(foundResource.Status.Conditions).To(ContainElement(commontestutils.RepresentCondition(metav1.Condition{
					Type:    hcov1beta1.ConditionReconcileComplete,
					Status:  metav1.ConditionFalse,
					Reason:  invalidRequestReason,
					Message: fmt.Sprintf(invalidRequestMessageFormat, name, namespace),
				})))
			})

			It("should create all managed resources", func() {

				hco := commontestutils.NewHco()
				hco.Spec.FeatureGates = hcov1beta1.HyperConvergedFeatureGates{
					DownwardMetrics: ptr.To(true),
					VideoConfig:     ptr.To(true),
				}

				ci := hcoutil.GetClusterInfo()
				cl := commontestutils.InitClient([]client.Object{hcoNamespace, hco, commontestutils.GetCSV()})
				monitoringReconciler := alerts.NewMonitoringReconciler(ci, cl, commontestutils.NewEventEmitterMock(), commontestutils.GetScheme())

				r := initReconciler(cl, nil)
				r.monitoringReconciler = monitoringReconciler

				// Do the reconcile
				res, err := r.Reconcile(context.TODO(), request)
				Expect(err).ToNot(HaveOccurred())
				Expect(res).To(Equal(reconcile.Result{RequeueAfter: requeueAfter}))
				validateOperatorCondition(r, metav1.ConditionTrue, hcoutil.UpgradeableAllowReason, hcoutil.UpgradeableAllowMessage)
				verifyHyperConvergedCRExistsMetricTrue()

				// Get the HCO
				foundResource := &hcov1beta1.HyperConverged{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: hco.Name, Namespace: hco.Namespace},
						foundResource),
				).ToNot(HaveOccurred())
				// Check conditions
				Expect(foundResource.Status.Conditions).To(ContainElement(commontestutils.RepresentCondition(metav1.Condition{
					Type:    hcov1beta1.ConditionReconcileComplete,
					Status:  metav1.ConditionUnknown,
					Reason:  reconcileInit,
					Message: reconcileInitMessage,
				})))
				Expect(foundResource.Status.Conditions).To(ContainElement(commontestutils.RepresentCondition(metav1.Condition{
					Type:    hcov1beta1.ConditionAvailable,
					Status:  metav1.ConditionFalse,
					Reason:  reconcileInit,
					Message: "Initializing HyperConverged cluster",
				})))
				Expect(foundResource.Status.Conditions).To(ContainElement(commontestutils.RepresentCondition(metav1.Condition{
					Type:    hcov1beta1.ConditionProgressing,
					Status:  metav1.ConditionTrue,
					Reason:  reconcileInit,
					Message: "Initializing HyperConverged cluster",
				})))
				Expect(foundResource.Status.Conditions).To(ContainElement(commontestutils.RepresentCondition(metav1.Condition{
					Type:    hcov1beta1.ConditionDegraded,
					Status:  metav1.ConditionFalse,
					Reason:  reconcileInit,
					Message: "Initializing HyperConverged cluster",
				})))
				Expect(foundResource.Status.Conditions).To(ContainElement(commontestutils.RepresentCondition(metav1.Condition{
					Type:    hcov1beta1.ConditionUpgradeable,
					Status:  metav1.ConditionUnknown,
					Reason:  reconcileInit,
					Message: "Initializing HyperConverged cluster",
				})))

				verifySystemHealthStatusError(foundResource)

				expectedFeatureGates := []string{
					"CPUManager",
					"Snapshot",
					"HotplugVolumes",
					"HostDevices",
					"WithHostModelCPU",
					"HypervStrictCheck",
					"ExpandDisks",
					"DownwardMetrics",
					"VMExport",
					"KubevirtSeccompProfile",
					"VideoConfig",
					"DecentralizedLiveMigration",
				}
				// Get the KV
				kvList := &kubevirtcorev1.KubeVirtList{}
				Expect(cl.List(context.TODO(), kvList)).To(Succeed())
				Expect(kvList.Items).To(HaveLen(1))
				kv := kvList.Items[0]
				Expect(kv.Spec.Configuration.DeveloperConfiguration).ToNot(BeNil())
				Expect(kv.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(HaveLen(len(expectedFeatureGates)))
				Expect(kv.Spec.Configuration.DeveloperConfiguration.FeatureGates).To(ContainElements(expectedFeatureGates))

				// Ensure the KubeVirt seccomp profile is set
				Expect(kv.Spec.Configuration.SeccompConfiguration).ToNot(BeNil())
				Expect(kv.Spec.Configuration.SeccompConfiguration.VirtualMachineInstanceProfile).ToNot(BeNil())
				Expect(kv.Spec.Configuration.SeccompConfiguration.VirtualMachineInstanceProfile.CustomProfile).ToNot(BeNil())
				Expect(kv.Spec.Configuration.SeccompConfiguration.VirtualMachineInstanceProfile.CustomProfile.RuntimeDefaultProfile).To(BeFalse())
				Expect(*kv.Spec.Configuration.SeccompConfiguration.VirtualMachineInstanceProfile.CustomProfile.LocalhostProfile).To(Equal("kubevirt/kubevirt.json"))

				res, err = r.Reconcile(context.TODO(), request)
				Expect(err).ToNot(HaveOccurred())
				Expect(res.IsZero()).To(BeTrue())
				validateOperatorCondition(r, metav1.ConditionTrue, hcoutil.UpgradeableAllowReason, hcoutil.UpgradeableAllowMessage)
				verifyHyperConvergedCRExistsMetricTrue()

				// Get the HCO
				foundResource = &hcov1beta1.HyperConverged{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: hco.Name, Namespace: hco.Namespace},
						foundResource),
				).ToNot(HaveOccurred())
				// Check conditions
				Expect(foundResource.Status.RelatedObjects).To(HaveLen(32))
				expectedRef := corev1.ObjectReference{
					Kind:            "PrometheusRule",
					Namespace:       namespace,
					Name:            "kubevirt-hyperconverged-prometheus-rule",
					APIVersion:      "monitoring.coreos.com/v1",
					ResourceVersion: "1",
				}
				Expect(foundResource.Status.RelatedObjects).To(ContainElement(expectedRef))
			})

			It("should find all managed resources", func() {

				expected := getBasicDeployment()

				expected.kv.Status.Conditions = nil
				expected.cdi.Status.Conditions = nil
				expected.cna.Status.Conditions = nil
				expected.ssp.Status.Conditions = nil

				pm := &monitoringv1.PrometheusRule{
					TypeMeta: metav1.TypeMeta{
						Kind:       monitoringv1.PrometheusRuleKind,
						APIVersion: "monitoring.coreos.com/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace:       namespace,
						Name:            "kubevirt-hyperconverged-prometheus-rule",
						UID:             "1234567890",
						ResourceVersion: "123",
					},
					Spec: monitoringv1.PrometheusRuleSpec{},
				}

				resources := expected.toArray()
				resources = append(resources, pm)
				cl := commontestutils.InitClient(resources)

				r := initReconciler(cl, nil)
				r.monitoringReconciler = alerts.NewMonitoringReconciler(hcoutil.GetClusterInfo(), cl, commontestutils.NewEventEmitterMock(), commontestutils.GetScheme())

				// Do the reconcile
				res, err := r.Reconcile(context.TODO(), request)
				Expect(err).ToNot(HaveOccurred())
				Expect(res).To(Equal(reconcile.Result{}))

				verifyHyperConvergedCRExistsMetricTrue()

				// Get the HCO
				foundResource := &hcov1beta1.HyperConverged{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: expected.hco.Name, Namespace: expected.hco.Namespace},
						foundResource),
				).ToNot(HaveOccurred())
				// Check conditions
				Expect(foundResource.Status.Conditions).To(ContainElement(commontestutils.RepresentCondition(metav1.Condition{
					Type:    hcov1beta1.ConditionReconcileComplete,
					Status:  metav1.ConditionTrue,
					Reason:  reconcileCompleted,
					Message: reconcileCompletedMessage,
				})))

				verifySystemHealthStatusError(foundResource)

				Expect(foundResource.Status.RelatedObjects).To(HaveLen(23))
				expectedRef := corev1.ObjectReference{
					Kind:            "PrometheusRule",
					Namespace:       namespace,
					Name:            "kubevirt-hyperconverged-prometheus-rule",
					APIVersion:      "monitoring.coreos.com/v1",
					ResourceVersion: "124",
					UID:             "1234567890",
				}
				Expect(foundResource.Status.RelatedObjects).To(ContainElement(expectedRef))
			})

			It("should label all managed resources", func() {
				expected := getBasicDeployment()

				cl := expected.initClient()
				r := initReconciler(cl, nil)

				// Do the reconcile
				res, err := r.Reconcile(context.TODO(), request)
				Expect(err).ToNot(HaveOccurred())
				Expect(res).To(Equal(reconcile.Result{}))

				// Get the HCO
				foundResource := &hcov1beta1.HyperConverged{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: expected.hco.Name, Namespace: expected.hco.Namespace},
						foundResource),
				).ToNot(HaveOccurred())

				// Check whether related objects have the labels or not
				Expect(foundResource.Status.RelatedObjects).ToNot(BeNil())
				for _, relatedObj := range foundResource.Status.RelatedObjects {
					foundRelatedObj := &unstructured.Unstructured{}
					foundRelatedObj.SetGroupVersionKind(relatedObj.GetObjectKind().GroupVersionKind())
					Expect(
						cl.Get(context.TODO(),
							types.NamespacedName{Name: relatedObj.Name, Namespace: relatedObj.Namespace},
							foundRelatedObj),
					).ToNot(HaveOccurred())

					foundLabels := foundRelatedObj.GetLabels()
					Expect(foundLabels[hcoutil.AppLabel]).To(Equal(expected.hco.Name))
					Expect(foundLabels[hcoutil.AppLabelPartOf]).To(Equal(hcoutil.HyperConvergedCluster))
					Expect(foundLabels[hcoutil.AppLabelManagedBy]).To(Equal(hcoutil.OperatorName))
					Expect(foundLabels[hcoutil.AppLabelVersion]).To(Equal(version.Version))
					Expect(foundLabels[hcoutil.AppLabelComponent]).ToNot(BeNil())
				}
			})

			It("should update resource versions of objects in relatedObjects", func() {

				expected := getBasicDeployment()
				cl := expected.initClient()

				r := initReconciler(cl, nil)

				// Reconcile to get all related objects under HCO's status
				res, err := r.Reconcile(context.TODO(), request)
				Expect(err).ToNot(HaveOccurred())
				Expect(res).To(Equal(reconcile.Result{}))

				// Update Kubevirt (an example of secondary CR)
				foundKubevirt := &kubevirtcorev1.KubeVirt{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: expected.kv.Name, Namespace: expected.kv.Namespace},
						foundKubevirt),
				).ToNot(HaveOccurred())
				foundKubevirt.Labels = map[string]string{"key": "value"}
				Expect(cl.Update(context.TODO(), foundKubevirt)).ToNot(HaveOccurred())

				// mock a reconciliation triggered by a change in secondary CR
				rq := reqresolver.GetSecondaryCRRequest()

				// Reconcile again to update HCO's status
				res, err = r.Reconcile(context.TODO(), rq)
				Expect(err).ToNot(HaveOccurred())
				Expect(res).To(Equal(reconcile.Result{}))

				// Get the latest objects
				latestHCO := &hcov1beta1.HyperConverged{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: expected.hco.Name, Namespace: expected.hco.Namespace},
						latestHCO),
				).ToNot(HaveOccurred())

				latestKubevirt := &kubevirtcorev1.KubeVirt{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: expected.kv.Name, Namespace: expected.kv.Namespace},
						latestKubevirt),
				).ToNot(HaveOccurred())

				kubevirtRef, err := reference.GetReference(cl.Scheme(), latestKubevirt)
				Expect(err).ToNot(HaveOccurred())
				// This fails when resource versions are not up-to-date
				Expect(latestHCO.Status.RelatedObjects).To(ContainElement(*kubevirtRef))
			})

			It("should update APIVersion of objects in relatedObjects", func() {

				expected := getBasicDeployment()
				cl := expected.initClient()
				r := initReconciler(cl, nil)

				// Reconcile to get all related objects under HCO's status
				res, err := r.Reconcile(context.TODO(), request)
				Expect(err).ToNot(HaveOccurred())
				Expect(res).To(Equal(reconcile.Result{}))

				// Get the latest objects
				HCO := &hcov1beta1.HyperConverged{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: expected.hco.Name, Namespace: expected.hco.Namespace},
						HCO),
				).ToNot(HaveOccurred())

				// Mock an outdated APIVersion on one of the resources
				consolePlugin := &consolev1.ConsolePlugin{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: expected.consolePlugin.Name, Namespace: expected.consolePlugin.Namespace},
						consolePlugin),
				).ToNot(HaveOccurred())
				newCpRef, err := reference.GetReference(cl.Scheme(), consolePlugin)
				Expect(err).ToNot(HaveOccurred())
				outdatedCpRef := newCpRef.DeepCopy()
				outdatedCpRef.APIVersion = "console.openshift.io/v1alpha1"
				Expect(objectreferencesv1.RemoveObjectReference(&HCO.Status.RelatedObjects, *newCpRef)).ToNot(HaveOccurred())
				Expect(objectreferencesv1.SetObjectReference(&HCO.Status.RelatedObjects, *outdatedCpRef)).ToNot(HaveOccurred())
				Expect(
					cl.Status().Update(context.TODO(), HCO),
				).ToNot(HaveOccurred())

				HCO = &hcov1beta1.HyperConverged{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: expected.hco.Name, Namespace: expected.hco.Namespace},
						HCO),
				).ToNot(HaveOccurred())
				Expect(HCO.Status.RelatedObjects).ToNot(ContainElement(*newCpRef))
				Expect(HCO.Status.RelatedObjects).To(ContainElement(*outdatedCpRef))

				// Update Kubevirt (an example of secondary CR)
				foundKubevirt := &kubevirtcorev1.KubeVirt{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: expected.kv.Name, Namespace: expected.kv.Namespace},
						foundKubevirt),
				).ToNot(HaveOccurred())
				foundKubevirt.Labels = map[string]string{"key": "value"}
				Expect(cl.Update(context.TODO(), foundKubevirt)).ToNot(HaveOccurred())

				// mock a reconciliation triggered by a change in secondary CR
				rq := reqresolver.GetSecondaryCRRequest()

				// Reconcile again to update HCO's status
				res, err = r.Reconcile(context.TODO(), rq)
				Expect(err).ToNot(HaveOccurred())
				Expect(res).To(Equal(reconcile.Result{}))

				// Get the latest objects
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: expected.hco.Name, Namespace: expected.hco.Namespace},
						HCO),
				).ToNot(HaveOccurred())

				Expect(HCO.Status.RelatedObjects).ToNot(ContainElement(*outdatedCpRef))
				Expect(HCO.Status.RelatedObjects).To(ContainElement(*newCpRef))

			})

			It("should update resource versions of objects in relatedObjects even when there is no update on secondary CR", func() {

				expected := getBasicDeployment()
				cl := expected.initClient()

				r := initReconciler(cl, nil)

				// Reconcile to get all related objects under HCO's status
				res, err := r.Reconcile(context.TODO(), request)
				Expect(err).ToNot(HaveOccurred())
				Expect(res).To(Equal(reconcile.Result{}))

				// Update Kubevirt's resource version (an example of secondary CR)
				foundKubevirt := &kubevirtcorev1.KubeVirt{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: expected.kv.Name, Namespace: expected.kv.Namespace},
						foundKubevirt),
				).ToNot(HaveOccurred())
				// no change. only to bump resource version
				Expect(cl.Update(context.TODO(), foundKubevirt)).ToNot(HaveOccurred())

				// mock a reconciliation triggered by a change in secondary CR
				rq := reqresolver.GetSecondaryCRRequest()

				// Reconcile again to update HCO's status
				res, err = r.Reconcile(context.TODO(), rq)
				Expect(err).ToNot(HaveOccurred())
				Expect(res).To(Equal(reconcile.Result{}))

				// Get the latest objects
				latestHCO := &hcov1beta1.HyperConverged{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: expected.hco.Name, Namespace: expected.hco.Namespace},
						latestHCO),
				).ToNot(HaveOccurred())

				latestKubevirt := &kubevirtcorev1.KubeVirt{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: expected.kv.Name, Namespace: expected.kv.Namespace},
						latestKubevirt),
				).ToNot(HaveOccurred())

				kubevirtRef, err := reference.GetReference(cl.Scheme(), latestKubevirt)
				Expect(err).ToNot(HaveOccurred())
				// This fails when resource versions are not up-to-date
				Expect(latestHCO.Status.RelatedObjects).To(ContainElement(*kubevirtRef))
			})

			It("should set different template namespace to ssp CR", func() {
				expected := getBasicDeployment()
				expected.hco.Spec.CommonTemplatesNamespace = &expected.hco.Namespace

				cl := expected.initClient()
				r := initReconciler(cl, nil)

				// Do the reconcile
				res, err := r.Reconcile(context.TODO(), request)
				Expect(err).ToNot(HaveOccurred())
				Expect(res).To(Equal(reconcile.Result{}))

				foundResource := &sspv1beta3.SSP{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: expected.ssp.Name, Namespace: expected.hco.Namespace},
						foundResource),
				).ToNot(HaveOccurred())

				Expect(foundResource.Spec.CommonTemplates.Namespace).To(Equal(expected.hco.Namespace), "common-templates namespace should be "+expected.hco.Namespace)
			})

			It("should complete when components are finished", func() {
				expected := getBasicDeployment()

				cl := expected.initClient()
				r := initReconciler(cl, nil)

				// Do the reconcile
				res, err := r.Reconcile(context.TODO(), request)
				Expect(err).ToNot(HaveOccurred())
				Expect(res).To(Equal(reconcile.Result{}))

				// Get the HCO
				foundResource := &hcov1beta1.HyperConverged{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: expected.hco.Name, Namespace: expected.hco.Namespace},
						foundResource),
				).ToNot(HaveOccurred())
				// Check conditions
				Expect(foundResource.Status.Conditions).To(ContainElement(commontestutils.RepresentCondition(metav1.Condition{
					Type:    hcov1beta1.ConditionReconcileComplete,
					Status:  metav1.ConditionTrue,
					Reason:  reconcileCompleted,
					Message: reconcileCompletedMessage,
				})))
				Expect(foundResource.Status.Conditions).To(ContainElement(commontestutils.RepresentCondition(metav1.Condition{
					Type:    hcov1beta1.ConditionAvailable,
					Status:  metav1.ConditionTrue,
					Reason:  reconcileCompleted,
					Message: reconcileCompletedMessage,
				})))
				Expect(foundResource.Status.Conditions).To(ContainElement(commontestutils.RepresentCondition(metav1.Condition{
					Type:    hcov1beta1.ConditionProgressing,
					Status:  metav1.ConditionFalse,
					Reason:  reconcileCompleted,
					Message: reconcileCompletedMessage,
				})))
				Expect(foundResource.Status.Conditions).To(ContainElement(commontestutils.RepresentCondition(metav1.Condition{
					Type:    hcov1beta1.ConditionDegraded,
					Status:  metav1.ConditionFalse,
					Reason:  reconcileCompleted,
					Message: reconcileCompletedMessage,
				})))
				Expect(foundResource.Status.Conditions).To(ContainElement(commontestutils.RepresentCondition(metav1.Condition{
					Type:    hcov1beta1.ConditionUpgradeable,
					Status:  metav1.ConditionTrue,
					Reason:  reconcileCompleted,
					Message: reconcileCompletedMessage,
				})))

				verifySystemHealthStatusHealthy(foundResource)
			})

			It("should increment counter when out-of-band change overwritten", func() {
				hco := commontestutils.NewHco()
				hco.Spec.Infra = hcov1beta1.HyperConvergedConfig{NodePlacement: commontestutils.NewNodePlacement()}
				hco.Spec.Workloads = hcov1beta1.HyperConvergedConfig{NodePlacement: commontestutils.NewNodePlacement()}
				existingResource, err := handlers.NewKubeVirt(hco, namespace)
				Expect(err).ToNot(HaveOccurred())
				existingResource.APIVersion, existingResource.Kind = kubevirtcorev1.KubeVirtGroupVersionKind.ToAPIVersionAndKind() // necessary for metrics

				// now, modify KV's node placement
				existingResource.Spec.Infra.NodePlacement.Tolerations = append(hco.Spec.Infra.NodePlacement.Tolerations, corev1.Toleration{
					Key: "key3", Operator: "operator3", Value: "value3", Effect: "effect3", TolerationSeconds: ptr.To[int64](3),
				})
				existingResource.Spec.Workloads.NodePlacement.Tolerations = append(hco.Spec.Workloads.NodePlacement.Tolerations, corev1.Toleration{
					Key: "key3", Operator: "operator3", Value: "value3", Effect: "effect3", TolerationSeconds: ptr.To[int64](3),
				})

				existingResource.Spec.Infra.NodePlacement.NodeSelector["key1"] = "BADvalue1"
				existingResource.Spec.Workloads.NodePlacement.NodeSelector["key2"] = "BADvalue2"

				cl := commontestutils.InitClient([]client.Object{hcoNamespace, hco, existingResource})
				r := initReconciler(cl, nil)

				// mock a reconciliation triggered by a change in secondary CR
				rq := reqresolver.GetSecondaryCRRequest()

				counterValueBefore, err := metrics.GetOverwrittenModificationsCount("KubeVirt", existingResource.Name)
				Expect(err).ToNot(HaveOccurred())

				// Do the reconcile
				res, err := r.Reconcile(context.TODO(), rq)
				Expect(err).ToNot(HaveOccurred())
				Expect(res).To(Equal(reconcile.Result{RequeueAfter: requeueAfter}))

				foundResource := &kubevirtcorev1.KubeVirt{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
						foundResource),
				).To(Succeed())

				Expect(existingResource.Spec.Infra.NodePlacement.Tolerations).To(HaveLen(3))
				Expect(existingResource.Spec.Workloads.NodePlacement.Tolerations).To(HaveLen(3))
				Expect(existingResource.Spec.Infra.NodePlacement.NodeSelector["key1"]).To(Equal("BADvalue1"))
				Expect(existingResource.Spec.Workloads.NodePlacement.NodeSelector["key2"]).To(Equal("BADvalue2"))

				Expect(foundResource.Spec.Infra.NodePlacement.Tolerations).To(HaveLen(2))
				Expect(foundResource.Spec.Workloads.NodePlacement.Tolerations).To(HaveLen(2))
				Expect(foundResource.Spec.Infra.NodePlacement.NodeSelector["key1"]).To(Equal("value1"))
				Expect(foundResource.Spec.Workloads.NodePlacement.NodeSelector["key2"]).To(Equal("value2"))

				counterValueAfter, err := metrics.GetOverwrittenModificationsCount("KubeVirt", foundResource.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(counterValueAfter).To(Equal(counterValueBefore + 1))

			})

			It("should not increment counter when CR was changed by HCO", func() {
				hco := commontestutils.NewHco()
				hco.Spec.Infra = hcov1beta1.HyperConvergedConfig{NodePlacement: commontestutils.NewNodePlacement()}
				hco.Spec.Workloads = hcov1beta1.HyperConvergedConfig{NodePlacement: commontestutils.NewNodePlacement()}
				existingResource, err := handlers.NewKubeVirt(hco, namespace)
				Expect(err).ToNot(HaveOccurred())
				existingResource.Kind = kubevirtcorev1.KubeVirtGroupVersionKind.Kind // necessary for metrics

				// now, modify KV's node placement
				existingResource.Spec.Infra.NodePlacement.Tolerations = append(hco.Spec.Infra.NodePlacement.Tolerations, corev1.Toleration{
					Key: "key3", Operator: "operator3", Value: "value3", Effect: "effect3", TolerationSeconds: ptr.To[int64](3),
				})
				existingResource.Spec.Workloads.NodePlacement.Tolerations = append(hco.Spec.Workloads.NodePlacement.Tolerations, corev1.Toleration{
					Key: "key3", Operator: "operator3", Value: "value3", Effect: "effect3", TolerationSeconds: ptr.To[int64](3),
				})

				existingResource.Spec.Infra.NodePlacement.NodeSelector["key1"] = "BADvalue1"
				existingResource.Spec.Workloads.NodePlacement.NodeSelector["key2"] = "BADvalue2"

				cl := commontestutils.InitClient([]client.Object{hcoNamespace, hco, existingResource})
				r := initReconciler(cl, nil)

				counterValueBefore, err := metrics.GetOverwrittenModificationsCount(existingResource.Kind, existingResource.Name)
				Expect(err).ToNot(HaveOccurred())

				// Do the reconcile triggered by HCO
				res, err := r.Reconcile(context.TODO(), request)
				Expect(err).ToNot(HaveOccurred())
				Expect(res).To(Equal(reconcile.Result{RequeueAfter: requeueAfter}))

				foundResource := &kubevirtcorev1.KubeVirt{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: existingResource.Name, Namespace: existingResource.Namespace},
						foundResource),
				).To(Succeed())

				Expect(existingResource.Spec.Infra.NodePlacement.Tolerations).To(HaveLen(3))
				Expect(existingResource.Spec.Workloads.NodePlacement.Tolerations).To(HaveLen(3))
				Expect(existingResource.Spec.Infra.NodePlacement.NodeSelector["key1"]).To(Equal("BADvalue1"))
				Expect(existingResource.Spec.Workloads.NodePlacement.NodeSelector["key2"]).To(Equal("BADvalue2"))

				Expect(foundResource.Spec.Infra.NodePlacement.Tolerations).To(HaveLen(2))
				Expect(foundResource.Spec.Workloads.NodePlacement.Tolerations).To(HaveLen(2))
				Expect(foundResource.Spec.Infra.NodePlacement.NodeSelector["key1"]).To(Equal("value1"))
				Expect(foundResource.Spec.Workloads.NodePlacement.NodeSelector["key2"]).To(Equal("value2"))

				counterValueAfter, err := metrics.GetOverwrittenModificationsCount("KubeVirt", foundResource.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(counterValueAfter).To(Equal(counterValueBefore))

			})

			It(`should be not available when components with missing "Available" condition`, func() {
				expected := getBasicDeployment()

				var cl *commontestutils.HcoTestClient
				By("Check KV", func() {
					origKvConds := expected.kv.Status.Conditions
					expected.kv.Status.Conditions = expected.kv.Status.Conditions[1:]

					cl = expected.initClient()
					foundResource, reconciler, requeue := doReconcile(cl, expected.hco, nil)
					Expect(requeue).To(BeFalse())
					checkAvailability(foundResource, metav1.ConditionFalse)

					expected.kv.Status.Conditions = origKvConds
					cl = expected.initClient()
					foundResource, _, requeue = doReconcile(cl, expected.hco, reconciler)
					Expect(requeue).To(BeFalse())
					checkAvailability(foundResource, metav1.ConditionTrue)
				})

				By("Check CDI", func() {
					origConds := expected.cdi.Status.Conditions
					expected.cdi.Status.Conditions = expected.cdi.Status.Conditions[1:]
					cl = expected.initClient()
					foundResource, reconciler, requeue := doReconcile(cl, expected.hco, nil)
					Expect(requeue).To(BeFalse())
					checkAvailability(foundResource, metav1.ConditionFalse)

					expected.cdi.Status.Conditions = origConds
					cl = expected.initClient()
					foundResource, _, requeue = doReconcile(cl, expected.hco, reconciler)
					Expect(requeue).To(BeFalse())
					checkAvailability(foundResource, metav1.ConditionTrue)
				})

				By("Check CNA", func() {
					origConds := expected.cna.Status.Conditions

					expected.cna.Status.Conditions = expected.cna.Status.Conditions[1:]
					cl = expected.initClient()
					foundResource, reconciler, requeue := doReconcile(cl, expected.hco, nil)
					Expect(requeue).To(BeFalse())
					checkAvailability(foundResource, metav1.ConditionFalse)

					expected.cna.Status.Conditions = origConds
					cl = expected.initClient()
					foundResource, _, requeue = doReconcile(cl, expected.hco, reconciler)
					Expect(requeue).To(BeFalse())
					checkAvailability(foundResource, metav1.ConditionTrue)
				})
				By("Check SSP", func() {
					origConds := expected.ssp.Status.Conditions
					expected.ssp.Status.Conditions = expected.ssp.Status.Conditions[1:]
					cl = expected.initClient()
					foundResource, reconciler, requeue := doReconcile(cl, expected.hco, nil)
					Expect(requeue).To(BeFalse())
					checkAvailability(foundResource, metav1.ConditionFalse)

					expected.ssp.Status.Conditions = origConds
					cl = expected.initClient()
					foundResource, _, requeue = doReconcile(cl, expected.hco, reconciler)
					Expect(requeue).To(BeFalse())
					checkAvailability(foundResource, metav1.ConditionTrue)
				})
			})

			It(`should delete HCO`, func() {

				// First, create HCO and check it
				expected := getBasicDeployment()
				cl := expected.initClient()
				r := initReconciler(cl, nil)
				monitoringReconciler := alerts.NewMonitoringReconciler(hcoutil.GetClusterInfo(), cl, commontestutils.NewEventEmitterMock(), commontestutils.GetScheme())
				r.monitoringReconciler = monitoringReconciler

				res, err := r.Reconcile(context.TODO(), request)
				Expect(err).ToNot(HaveOccurred())
				Expect(res).To(Equal(reconcile.Result{}))

				foundResource := &hcov1beta1.HyperConverged{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: expected.hco.Name, Namespace: expected.hco.Namespace},
						foundResource),
				).To(Succeed())

				Expect(foundResource.Status.RelatedObjects).ToNot(BeNil())
				Expect(foundResource.Status.RelatedObjects).To(HaveLen(23))
				Expect(foundResource.Finalizers).To(Equal([]string{FinalizerName}))

				// Now, delete HCO
				delTime := time.Now().UTC().Add(-1 * time.Minute)
				expected.hco.DeletionTimestamp = &metav1.Time{Time: delTime}
				expected.hco.Finalizers = []string{FinalizerName}
				cl = expected.initClient()

				r = initReconciler(cl, nil)
				res, err = r.Reconcile(context.TODO(), request)
				Expect(err).ToNot(HaveOccurred())
				Expect(res).To(Equal(reconcile.Result{RequeueAfter: requeueAfter}))

				res, err = r.Reconcile(context.TODO(), request)
				Expect(err).ToNot(HaveOccurred())
				Expect(res.IsZero()).To(BeTrue())

				foundResource = &hcov1beta1.HyperConverged{}
				err = cl.Get(context.TODO(),
					types.NamespacedName{Name: expected.hco.Name, Namespace: expected.hco.Namespace},
					foundResource)
				Expect(err).To(MatchError(apierrors.IsNotFound, "not found error"))

				verifyHyperConvergedCRExistsMetricFalse()
			})

			It(`should set a finalizer on HCO CR`, func() {
				expected := getBasicDeployment()
				cl := expected.initClient()
				r := initReconciler(cl, nil)
				res, err := r.Reconcile(context.TODO(), request)
				Expect(err).ToNot(HaveOccurred())
				Expect(res).To(Equal(reconcile.Result{}))

				foundResource := &hcov1beta1.HyperConverged{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: expected.hco.Name, Namespace: expected.hco.Namespace},
						foundResource),
				).To(Succeed())

				Expect(foundResource.Status.RelatedObjects).ToNot(BeNil())
				Expect(foundResource.Finalizers).To(Equal([]string{FinalizerName}))
			})

			It("Should not be ready if one of the operands is returns error, on create", func() {
				hco := commontestutils.NewHco()
				cl := commontestutils.InitClient([]client.Object{hcoNamespace, hco})
				cl.InitiateCreateErrors(func(obj client.Object) error {
					if _, ok := obj.(*cdiv1beta1.CDI); ok {
						return errors.New("fake create error")
					}
					return nil
				})
				r := initReconciler(cl, nil)

				// Do the reconcile
				res, err := r.Reconcile(context.TODO(), request)
				Expect(err).ToNot(HaveOccurred())
				Expect(res).To(Equal(reconcile.Result{RequeueAfter: requeueAfter}))

				// Get the HCO
				foundResource := &hcov1beta1.HyperConverged{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: hco.Name, Namespace: hco.Namespace},
						foundResource),
				).To(Succeed())

				// Check condition
				foundCond := false
				for _, cond := range foundResource.Status.Conditions {
					if cond.Type == hcov1beta1.ConditionReconcileComplete {
						foundCond = true
						Expect(cond.Status).To(Equal(metav1.ConditionFalse))
						Expect(cond.Message).To(ContainSubstring("fake create error"))
						break
					}
				}
				Expect(foundCond).To(BeTrue())
			})

			It("Should be ready even if one of the operands is returns error, on update", func() {
				expected := getBasicDeployment()
				expected.kv.Spec.Configuration.DeveloperConfiguration = &kubevirtcorev1.DeveloperConfiguration{
					FeatureGates: []string{"fakeFg"}, // force update
				}
				cl := expected.initClient()
				cl.InitiateUpdateErrors(func(obj client.Object) error {
					if _, ok := obj.(*kubevirtcorev1.KubeVirt); ok {
						return errors.New("fake update error")
					}
					return nil
				})

				hco := commontestutils.NewHco()
				r := initReconciler(cl, nil)

				// Do the reconcile
				res, err := r.Reconcile(context.TODO(), request)
				Expect(err).ToNot(HaveOccurred())
				Expect(res.IsZero()).To(BeTrue())

				// Get the HCO
				foundHyperConverged := &hcov1beta1.HyperConverged{}
				Expect(
					cl.Get(context.TODO(),
						types.NamespacedName{Name: hco.Name, Namespace: hco.Namespace},
						foundHyperConverged),
				).To(Succeed())

				// Check condition
				foundCond := false
				for _, cond := range foundHyperConverged.Status.Conditions {
					if cond.Type == hcov1beta1.ConditionReconcileComplete {
						foundCond = true
						Expect(cond.Status).To(Equal(metav1.ConditionFalse))
						Expect(cond.Message).To(ContainSubstring("fake update error"))
						break
					}
				}
				Expect(foundCond).To(BeTrue())
			})

			It("Should upgrade the status.observedGeneration field", func() {
				expected := getBasicDeployment()
				expected.hco.Generation = 10
				cl := expected.initClient()
				foundResource, _, _ := doReconcile(cl, expected.hco, nil)

				Expect(foundResource.Status.ObservedGeneration).To(BeEquivalentTo(10))
			})

			It("Should update memory overcommit metrics according to the CR", func() {
				expected := getBasicDeployment()
				expected.hco.Spec.HigherWorkloadDensity = &hcov1beta1.HigherWorkloadDensityConfiguration{
					MemoryOvercommitPercentage: 42,
				}

				cl := expected.initClient()
				r := initReconciler(cl, nil)

				// Do the reconcile
				res, err := r.Reconcile(context.TODO(), request)
				Expect(err).ToNot(HaveOccurred())
				Expect(res).To(Equal(reconcile.Result{}))

				value, err := metrics.GetHCOMetricMemoryOvercommitPercentage()
				Expect(err).ToNot(HaveOccurred())
				Expect(int(value)).To(BeEquivalentTo(42))

			})
		})

		Context("TLS Security Profile", func() {

			BeforeEach(func() {
				externalClusterInfo := hcoutil.GetClusterInfo
				hcoutil.GetClusterInfo = getClusterInfo

				DeferCleanup(func() {
					hcoutil.GetClusterInfo = externalClusterInfo
				})
			})

			It("Should refresh use the APIServer if the TLSSecurityProfile is not set in the HyperConverged CR", func(ctx context.Context) {

				initialTLSSecurityProfile := &openshiftconfigv1.TLSSecurityProfile{
					Type:         openshiftconfigv1.TLSProfileIntermediateType,
					Intermediate: &openshiftconfigv1.IntermediateTLSProfile{},
				}
				customTLSSecurityProfile := &openshiftconfigv1.TLSSecurityProfile{
					Type:   openshiftconfigv1.TLSProfileModernType,
					Modern: &openshiftconfigv1.ModernTLSProfile{},
				}

				apiServer := &openshiftconfigv1.APIServer{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster",
					},
					Spec: openshiftconfigv1.APIServerSpec{
						TLSSecurityProfile: initialTLSSecurityProfile,
					},
				}

				expected := getBasicDeployment()
				Expect(expected.hco.Spec.TLSSecurityProfile).To(BeNil())

				resources := expected.toArray()
				resources = append(resources, apiServer)
				cl := commontestutils.InitClient(resources)

				_, err := tlssecprofile.Refresh(ctx, cl)
				Expect(err).ToNot(HaveOccurred())
				r := initReconciler(cl, nil)

				// Reconcile to get all related objects under HCO's status
				res, err := r.Reconcile(ctx, request)
				Expect(err).ToNot(HaveOccurred())
				Expect(res).To(Equal(reconcile.Result{}))

				foundResource := &hcov1beta1.HyperConverged{}
				Expect(
					cl.Get(ctx,
						types.NamespacedName{Name: expected.hco.Name, Namespace: expected.hco.Namespace},
						foundResource),
				).ToNot(HaveOccurred())
				checkAvailability(foundResource, metav1.ConditionTrue)
				Expect(foundResource.Spec.TLSSecurityProfile).To(BeNil(), "TLSSecurityProfile on HCO CR should still be nil")

				By("Verify that Kubevirt was properly configured with initialTLSSecurityProfile")
				kv := handlers.NewKubeVirtWithNameOnly(foundResource)
				Expect(
					cl.Get(ctx,
						types.NamespacedName{Name: kv.Name, Namespace: kv.Namespace},
						kv),
				).To(Succeed())

				Expect(kv.Spec.Configuration.TLSConfiguration.MinTLSVersion).To(Equal(kubevirtcorev1.VersionTLS12))
				Expect(kv.Spec.Configuration.TLSConfiguration.Ciphers).To(Equal([]string{
					"TLS_AES_128_GCM_SHA256",
					"TLS_AES_256_GCM_SHA384",
					"TLS_CHACHA20_POLY1305_SHA256",
					"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
					"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
					"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384",
					"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
					"TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256",
					"TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256",
				}))

				By("Verify that CDI was properly configured with initialTLSSecurityProfile")
				cdi := handlers.NewCDIWithNameOnly(foundResource)
				Expect(
					cl.Get(ctx,
						types.NamespacedName{Name: cdi.Name, Namespace: cdi.Namespace},
						cdi),
				).To(Succeed())

				Expect(cdi.Spec.Config.TLSSecurityProfile).To(Equal(openshift2CdiSecProfile(initialTLSSecurityProfile)))

				By("Verify that CNA was properly configured with initialTLSSecurityProfile")
				cna := handlers.NewNetworkAddonsWithNameOnly(foundResource)
				Expect(
					cl.Get(ctx,
						types.NamespacedName{Name: cna.Name, Namespace: cna.Namespace},
						cna),
				).To(Succeed())

				Expect(cna.Spec.TLSSecurityProfile).To(Equal(initialTLSSecurityProfile))

				By("Verify that SSP was properly configured with initialTLSSecurityProfile")
				ssp := handlers.NewSSPWithNameOnly(foundResource)
				Expect(
					cl.Get(ctx,
						types.NamespacedName{Name: ssp.Name, Namespace: ssp.Namespace},
						ssp),
				).To(Succeed())

				Expect(ssp.Spec.TLSSecurityProfile).To(Equal(initialTLSSecurityProfile))

				// Update ApiServer CR
				apiServer.Spec.TLSSecurityProfile = customTLSSecurityProfile
				Expect(cl.Update(ctx, apiServer)).To(Succeed())
				Expect(tlssecprofile.GetTLSSecurityProfile(expected.hco.Spec.TLSSecurityProfile)).To(Equal(initialTLSSecurityProfile), "should still return the cached value (initial value)")

				// this is done in the apiserver controller
				modified, err := tlssecprofile.Refresh(ctx, cl)
				Expect(err).ToNot(HaveOccurred())
				Expect(modified).To(BeTrue())
				// mock a reconciliation triggered by a change in the APIServer controller
				rq := reqresolver.GetAPIServerCRRequest()

				// Reconcile again to make sure all the CRs get updated with the new TLS security profile
				res, err = r.Reconcile(ctx, rq)
				Expect(err).ToNot(HaveOccurred())
				Expect(res).To(Equal(reconcile.Result{}))

				Expect(
					cl.Get(ctx,
						types.NamespacedName{Name: expected.hco.Name, Namespace: expected.hco.Namespace},
						foundResource),
				).ToNot(HaveOccurred())
				checkAvailability(foundResource, metav1.ConditionTrue)
				Expect(foundResource.Spec.TLSSecurityProfile).To(BeNil(), "TLSSecurityProfile on HCO CR should still be nil")

				Expect(tlssecprofile.GetTLSSecurityProfile(expected.hco.Spec.TLSSecurityProfile)).To(Equal(customTLSSecurityProfile), "should return the up-to-date value")

				By("Verify that Kubevirt was properly updated with customTLSSecurityProfile")
				kv = handlers.NewKubeVirtWithNameOnly(foundResource)
				Expect(
					cl.Get(ctx,
						types.NamespacedName{Name: kv.Name, Namespace: kv.Namespace},
						kv),
				).To(Succeed())

				Expect(kv.Spec.Configuration.TLSConfiguration.MinTLSVersion).To(Equal(kubevirtcorev1.VersionTLS13))
				Expect(kv.Spec.Configuration.TLSConfiguration.Ciphers).To(BeEmpty())

				By("Verify that CDI was properly updated with customTLSSecurityProfile")
				cdi = handlers.NewCDIWithNameOnly(foundResource)
				Expect(
					cl.Get(ctx,
						types.NamespacedName{Name: cdi.Name, Namespace: cdi.Namespace},
						cdi),
				).To(Succeed())

				Expect(cdi.Spec.Config.TLSSecurityProfile).To(Equal(openshift2CdiSecProfile(customTLSSecurityProfile)))

				By("Verify that CNA was properly updated with customTLSSecurityProfile")
				cna = handlers.NewNetworkAddonsWithNameOnly(foundResource)
				Expect(
					cl.Get(ctx,
						types.NamespacedName{Name: cna.Name, Namespace: cna.Namespace},
						cna),
				).To(Succeed())

				Expect(cna.Spec.TLSSecurityProfile).To(Equal(customTLSSecurityProfile))

				By("Verify that SSP was properly updated with customTLSSecurityProfile")
				ssp = handlers.NewSSPWithNameOnly(foundResource)
				Expect(
					cl.Get(ctx,
						types.NamespacedName{Name: ssp.Name, Namespace: ssp.Namespace},
						ssp),
				).To(Succeed())

				Expect(ssp.Spec.TLSSecurityProfile).To(Equal(customTLSSecurityProfile))
			})

			It("Should use the TLSSecurityProfile from the HyperConverged CR, if it set", func(ctx context.Context) {

				initialTLSSecurityProfile := &openshiftconfigv1.TLSSecurityProfile{
					Type:         openshiftconfigv1.TLSProfileIntermediateType,
					Intermediate: &openshiftconfigv1.IntermediateTLSProfile{},
				}
				customTLSSecurityProfile := &openshiftconfigv1.TLSSecurityProfile{
					Type:   openshiftconfigv1.TLSProfileModernType,
					Modern: &openshiftconfigv1.ModernTLSProfile{},
				}

				apiServer := &openshiftconfigv1.APIServer{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster",
					},
					Spec: openshiftconfigv1.APIServerSpec{
						TLSSecurityProfile: initialTLSSecurityProfile,
					},
				}

				expected := getBasicDeployment()
				Expect(expected.hco.Spec.TLSSecurityProfile).To(BeNil())

				resources := expected.toArray()
				resources = append(resources, apiServer)
				cl := commontestutils.InitClient(resources)

				_, err := tlssecprofile.Refresh(ctx, cl)
				Expect(err).ToNot(HaveOccurred())
				r := initReconciler(cl, nil)

				// Reconcile to get all related objects under HCO's status
				res, err := r.Reconcile(ctx, request)
				Expect(err).ToNot(HaveOccurred())
				Expect(res).To(Equal(reconcile.Result{}))

				foundHCO := &hcov1beta1.HyperConverged{}
				Expect(
					cl.Get(ctx,
						types.NamespacedName{Name: expected.hco.Name, Namespace: expected.hco.Namespace},
						foundHCO),
				).ToNot(HaveOccurred())

				Expect(foundHCO.Spec.TLSSecurityProfile).To(BeNil(), "TLSSecurityProfile on HCO CR should still be nil")

				By("Verify that Kubevirt was properly configured with initialTLSSecurityProfile")
				kv := handlers.NewKubeVirtWithNameOnly(foundHCO)
				Expect(
					cl.Get(ctx,
						types.NamespacedName{Name: kv.Name, Namespace: kv.Namespace},
						kv),
				).To(Succeed())

				Expect(kv.Spec.Configuration.TLSConfiguration.MinTLSVersion).To(Equal(kubevirtcorev1.VersionTLS12))
				Expect(kv.Spec.Configuration.TLSConfiguration.Ciphers).To(Equal([]string{
					"TLS_AES_128_GCM_SHA256",
					"TLS_AES_256_GCM_SHA384",
					"TLS_CHACHA20_POLY1305_SHA256",
					"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
					"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
					"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384",
					"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
					"TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256",
					"TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256",
				}))

				By("Verify that CDI was properly configured with initialTLSSecurityProfile")
				cdi := handlers.NewCDIWithNameOnly(foundHCO)
				Expect(
					cl.Get(ctx,
						types.NamespacedName{Name: cdi.Name, Namespace: cdi.Namespace},
						cdi),
				).To(Succeed())

				Expect(cdi.Spec.Config.TLSSecurityProfile).To(Equal(openshift2CdiSecProfile(initialTLSSecurityProfile)))

				By("Verify that CNA was properly configured with initialTLSSecurityProfile")
				cna := handlers.NewNetworkAddonsWithNameOnly(foundHCO)
				Expect(
					cl.Get(ctx,
						types.NamespacedName{Name: cna.Name, Namespace: cna.Namespace},
						cna),
				).To(Succeed())

				Expect(cna.Spec.TLSSecurityProfile).To(Equal(initialTLSSecurityProfile))

				By("Verify that SSP was properly configured with initialTLSSecurityProfile")
				ssp := handlers.NewSSPWithNameOnly(foundHCO)
				Expect(
					cl.Get(ctx,
						types.NamespacedName{Name: ssp.Name, Namespace: ssp.Namespace},
						ssp),
				).To(Succeed())

				Expect(ssp.Spec.TLSSecurityProfile).To(Equal(initialTLSSecurityProfile))

				By("Update HyperConverged CR with customTLSSecurityProfile")
				foundHCO.Spec.TLSSecurityProfile = customTLSSecurityProfile
				Expect(cl.Update(ctx, foundHCO)).To(Succeed())

				// Reconcile again to make sure all the CRs get updated with the new TLS security profile
				res, err = r.Reconcile(ctx, request)
				Expect(err).ToNot(HaveOccurred())
				Expect(res).To(Equal(reconcile.Result{}))

				Expect(
					cl.Get(ctx,
						types.NamespacedName{Name: expected.hco.Name, Namespace: expected.hco.Namespace},
						foundHCO),
				).ToNot(HaveOccurred())
				checkAvailability(foundHCO, metav1.ConditionTrue)
				Expect(foundHCO.Spec.TLSSecurityProfile).To(Equal(customTLSSecurityProfile), "TLSSecurityProfile on HCO CR should be updated")

				By("Verify that Kubevirt was properly updated with customTLSSecurityProfile")
				kv = handlers.NewKubeVirtWithNameOnly(foundHCO)
				Expect(
					cl.Get(ctx,
						types.NamespacedName{Name: kv.Name, Namespace: kv.Namespace},
						kv),
				).To(Succeed())

				Expect(kv.Spec.Configuration.TLSConfiguration.MinTLSVersion).To(Equal(kubevirtcorev1.VersionTLS13))
				Expect(kv.Spec.Configuration.TLSConfiguration.Ciphers).To(BeEmpty())

				By("Verify that CDI was properly updated with customTLSSecurityProfile")
				cdi = handlers.NewCDIWithNameOnly(foundHCO)
				Expect(
					cl.Get(ctx,
						types.NamespacedName{Name: cdi.Name, Namespace: cdi.Namespace},
						cdi),
				).To(Succeed())

				Expect(cdi.Spec.Config.TLSSecurityProfile).To(Equal(openshift2CdiSecProfile(customTLSSecurityProfile)))

				By("Verify that CNA was properly updated with customTLSSecurityProfile")
				cna = handlers.NewNetworkAddonsWithNameOnly(foundHCO)
				Expect(
					cl.Get(ctx,
						types.NamespacedName{Name: cna.Name, Namespace: cna.Namespace},
						cna),
				).To(Succeed())

				Expect(cna.Spec.TLSSecurityProfile).To(Equal(customTLSSecurityProfile))

				By("Verify that SSP was properly updated with customTLSSecurityProfile")
				ssp = handlers.NewSSPWithNameOnly(foundHCO)
				Expect(
					cl.Get(ctx,
						types.NamespacedName{Name: ssp.Name, Namespace: ssp.Namespace},
						ssp),
				).To(Succeed())

				Expect(ssp.Spec.TLSSecurityProfile).To(Equal(customTLSSecurityProfile))
			})
		})

		Context("Validate OLM required fields", func() {
			var (
				expected  *BasicExpected
				origConds []metav1.Condition
			)

			BeforeEach(func() {
				expected = getBasicDeployment()
				origConds = expected.hco.Status.Conditions
			})

			It("Should set required fields on init", func() {
				expected.hco.Status.Conditions = nil

				cl := expected.initClient()
				foundResource, _, requeue := doReconcile(cl, expected.hco, nil)
				Expect(requeue).To(BeTrue())

				Expect(foundResource.Labels[hcoutil.AppLabel]).To(Equal(hcoutil.HyperConvergedName))
			})

			It("Should set required fields when missing", func() {
				expected.hco.Status.Conditions = origConds
				// old HCO Version is set
				cl := expected.initClient()
				foundResource, _, requeue := doReconcile(cl, expected.hco, nil)
				Expect(requeue).To(BeFalse())

				Expect(foundResource.Labels[hcoutil.AppLabel]).To(Equal(hcoutil.HyperConvergedName))
			})
		})

		Context("Aggregate Negative Conditions", func() {
			const errorReason = "CdiTestError1"

			It("should be degraded when a component is degraded", func() {
				expected := getBasicDeployment()
				conditionsv1.SetStatusCondition(&expected.cdi.Status.Conditions, conditionsv1.Condition{
					Type:    conditionsv1.ConditionDegraded,
					Status:  corev1.ConditionTrue,
					Reason:  errorReason,
					Message: "CDI Test Error message",
				})
				cl := expected.initClient()
				foundResource, r, _ := doReconcile(cl, expected.hco, nil)

				conditions := foundResource.Status.Conditions
				_, _ = fmt.Fprintln(GinkgoWriter, "\nActual Conditions:")
				wr := json.NewEncoder(GinkgoWriter)
				wr.SetIndent("", "  ")
				_ = wr.Encode(conditions)

				cd := apimetav1.FindStatusCondition(conditions, hcov1beta1.ConditionReconcileComplete)
				Expect(cd.Status).To(BeEquivalentTo(metav1.ConditionTrue))
				Expect(cd.Reason).To(Equal(reconcileCompleted))
				cd = apimetav1.FindStatusCondition(conditions, hcov1beta1.ConditionAvailable)
				Expect(cd.Status).To(BeEquivalentTo(metav1.ConditionFalse))
				Expect(cd.Reason).To(Equal(commonDegradedReason))
				Expect(cd.Message).To(Equal("HCO is not available due to degraded components"))
				cd = apimetav1.FindStatusCondition(conditions, hcov1beta1.ConditionProgressing)
				Expect(cd.Status).To(BeEquivalentTo(metav1.ConditionFalse))
				Expect(cd.Reason).To(Equal(reconcileCompleted))
				cd = apimetav1.FindStatusCondition(conditions, hcov1beta1.ConditionDegraded)
				Expect(cd.Status).To(BeEquivalentTo(metav1.ConditionTrue))
				Expect(cd.Reason).To(Equal("CDIDegraded"))
				cd = apimetav1.FindStatusCondition(conditions, hcov1beta1.ConditionUpgradeable)
				Expect(cd.Status).To(BeEquivalentTo(metav1.ConditionFalse))
				Expect(cd.Reason).To(Equal(commonDegradedReason))
				Expect(cd.Message).To(Equal("HCO is not Upgradeable due to degraded components"))

				By("operator condition should be true even the upgradeable is false")
				validateOperatorCondition(r, metav1.ConditionTrue, hcoutil.UpgradeableAllowReason, hcoutil.UpgradeableAllowMessage)
			})

			It("should be degraded when a component is degraded + Progressing", func() {
				expected := getBasicDeployment()
				conditionsv1.SetStatusCondition(&expected.cdi.Status.Conditions, conditionsv1.Condition{
					Type:    conditionsv1.ConditionDegraded,
					Status:  corev1.ConditionTrue,
					Reason:  errorReason,
					Message: "CDI Test Error message",
				})
				conditionsv1.SetStatusCondition(&expected.cdi.Status.Conditions, conditionsv1.Condition{
					Type:    conditionsv1.ConditionProgressing,
					Status:  corev1.ConditionTrue,
					Reason:  "progressingError",
					Message: "CDI Test Error message",
				})
				cl := expected.initClient()
				foundResource, r, _ := doReconcile(cl, expected.hco, nil)

				conditions := foundResource.Status.Conditions
				_, _ = fmt.Fprintln(GinkgoWriter, "\nActual Conditions:")
				wr := json.NewEncoder(GinkgoWriter)
				wr.SetIndent("", "  ")
				_ = wr.Encode(conditions)

				cd := apimetav1.FindStatusCondition(conditions, hcov1beta1.ConditionReconcileComplete)
				Expect(cd.Status).To(BeEquivalentTo(metav1.ConditionTrue))
				Expect(cd.Reason).To(Equal(reconcileCompleted))
				cd = apimetav1.FindStatusCondition(conditions, hcov1beta1.ConditionAvailable)
				Expect(cd.Status).To(BeEquivalentTo(metav1.ConditionFalse))
				Expect(cd.Reason).To(Equal(commonDegradedReason))
				cd = apimetav1.FindStatusCondition(conditions, hcov1beta1.ConditionProgressing)
				Expect(cd.Status).To(BeEquivalentTo(metav1.ConditionTrue))
				Expect(cd.Reason).To(Equal("CDIProgressing"))
				cd = apimetav1.FindStatusCondition(conditions, hcov1beta1.ConditionDegraded)
				Expect(cd.Status).To(BeEquivalentTo(metav1.ConditionTrue))
				Expect(cd.Reason).To(Equal("CDIDegraded"))
				cd = apimetav1.FindStatusCondition(conditions, hcov1beta1.ConditionUpgradeable)
				Expect(cd.Status).To(BeEquivalentTo(metav1.ConditionFalse))
				Expect(cd.Reason).To(Equal("CDIProgressing"))

				By("operator condition should be true even the upgradeable is false")
				validateOperatorCondition(r, metav1.ConditionTrue, hcoutil.UpgradeableAllowReason, hcoutil.UpgradeableAllowMessage)
			})

			It("should be degraded when a component is degraded + !Available", func() {
				expected := getBasicDeployment()
				conditionsv1.SetStatusCondition(&expected.cdi.Status.Conditions, conditionsv1.Condition{
					Type:    conditionsv1.ConditionDegraded,
					Status:  corev1.ConditionTrue,
					Reason:  errorReason,
					Message: "CDI Test Error message",
				})
				conditionsv1.SetStatusCondition(&expected.cdi.Status.Conditions, conditionsv1.Condition{
					Type:    conditionsv1.ConditionAvailable,
					Status:  corev1.ConditionFalse,
					Reason:  "AvailableError",
					Message: "CDI Test Error message",
				})
				cl := expected.initClient()
				foundResource, r, _ := doReconcile(cl, expected.hco, nil)

				conditions := foundResource.Status.Conditions
				_, _ = fmt.Fprintln(GinkgoWriter, "\nActual Conditions:")
				wr := json.NewEncoder(GinkgoWriter)
				wr.SetIndent("", "  ")
				_ = wr.Encode(conditions)

				cd := apimetav1.FindStatusCondition(conditions, hcov1beta1.ConditionReconcileComplete)
				Expect(cd.Status).To(BeEquivalentTo(metav1.ConditionTrue))
				Expect(cd.Reason).To(Equal(reconcileCompleted))
				cd = apimetav1.FindStatusCondition(conditions, hcov1beta1.ConditionAvailable)
				Expect(cd.Status).To(BeEquivalentTo(metav1.ConditionFalse))
				Expect(cd.Reason).To(Equal("CDINotAvailable"))
				cd = apimetav1.FindStatusCondition(conditions, hcov1beta1.ConditionProgressing)
				Expect(cd.Status).To(BeEquivalentTo(metav1.ConditionFalse))
				Expect(cd.Reason).To(Equal(reconcileCompleted))
				cd = apimetav1.FindStatusCondition(conditions, hcov1beta1.ConditionDegraded)
				Expect(cd.Status).To(BeEquivalentTo(metav1.ConditionTrue))
				Expect(cd.Reason).To(Equal("CDIDegraded"))
				cd = apimetav1.FindStatusCondition(conditions, hcov1beta1.ConditionUpgradeable)
				Expect(cd.Status).To(BeEquivalentTo(metav1.ConditionFalse))
				Expect(cd.Reason).To(Equal(commonDegradedReason))

				By("operator condition should be true even the upgradeable is false")
				validateOperatorCondition(r, metav1.ConditionTrue, hcoutil.UpgradeableAllowReason, hcoutil.UpgradeableAllowMessage)
			})

			It("should be Progressing when a component is Progressing", func() {
				expected := getBasicDeployment()
				conditionsv1.SetStatusCondition(&expected.cdi.Status.Conditions, conditionsv1.Condition{
					Type:    conditionsv1.ConditionProgressing,
					Status:  corev1.ConditionTrue,
					Reason:  errorReason,
					Message: "CDI Test Error message",
				})
				cl := expected.initClient()
				foundResource, r, _ := doReconcile(cl, expected.hco, nil)

				conditions := foundResource.Status.Conditions
				_, _ = fmt.Fprintln(GinkgoWriter, "\nActual Conditions:")
				wr := json.NewEncoder(GinkgoWriter)
				wr.SetIndent("", "  ")
				_ = wr.Encode(conditions)

				cd := apimetav1.FindStatusCondition(conditions, hcov1beta1.ConditionReconcileComplete)
				Expect(cd.Status).To(BeEquivalentTo(metav1.ConditionTrue))
				Expect(cd.Reason).To(Equal(reconcileCompleted))
				cd = apimetav1.FindStatusCondition(conditions, hcov1beta1.ConditionAvailable)
				Expect(cd.Status).To(BeEquivalentTo(metav1.ConditionTrue))
				Expect(cd.Reason).To(Equal(reconcileCompleted))
				cd = apimetav1.FindStatusCondition(conditions, hcov1beta1.ConditionProgressing)
				Expect(cd.Status).To(BeEquivalentTo(metav1.ConditionTrue))
				Expect(cd.Reason).To(Equal("CDIProgressing"))
				cd = apimetav1.FindStatusCondition(conditions, hcov1beta1.ConditionDegraded)
				Expect(cd.Status).To(BeEquivalentTo(metav1.ConditionFalse))
				Expect(cd.Reason).To(Equal(reconcileCompleted))
				cd = apimetav1.FindStatusCondition(conditions, hcov1beta1.ConditionUpgradeable)
				Expect(cd.Status).To(BeEquivalentTo(metav1.ConditionFalse))
				Expect(cd.Reason).To(Equal("CDIProgressing"))

				By("operator condition should be true even the upgradeable is false")
				validateOperatorCondition(r, metav1.ConditionTrue, hcoutil.UpgradeableAllowReason, hcoutil.UpgradeableAllowMessage)
			})

			It("should be Progressing when a component is Progressing + !Available", func() {
				expected := getBasicDeployment()
				conditionsv1.SetStatusCondition(&expected.cdi.Status.Conditions, conditionsv1.Condition{
					Type:    conditionsv1.ConditionProgressing,
					Status:  corev1.ConditionTrue,
					Reason:  errorReason,
					Message: "CDI Test Error message",
				})
				conditionsv1.SetStatusCondition(&expected.cdi.Status.Conditions, conditionsv1.Condition{
					Type:    conditionsv1.ConditionAvailable,
					Status:  corev1.ConditionFalse,
					Reason:  "AvailableError",
					Message: "CDI Test Error message",
				})
				cl := expected.initClient()
				foundResource, r, _ := doReconcile(cl, expected.hco, nil)

				conditions := foundResource.Status.Conditions
				_, _ = fmt.Fprintln(GinkgoWriter, "\nActual Conditions:")
				wr := json.NewEncoder(GinkgoWriter)
				wr.SetIndent("", "  ")
				_ = wr.Encode(conditions)

				cd := apimetav1.FindStatusCondition(conditions, hcov1beta1.ConditionReconcileComplete)
				Expect(cd.Status).To(BeEquivalentTo(metav1.ConditionTrue))
				Expect(cd.Reason).To(Equal(reconcileCompleted))
				cd = apimetav1.FindStatusCondition(conditions, hcov1beta1.ConditionAvailable)
				Expect(cd.Status).To(BeEquivalentTo(metav1.ConditionFalse))
				Expect(cd.Reason).To(Equal("CDINotAvailable"))
				cd = apimetav1.FindStatusCondition(conditions, hcov1beta1.ConditionProgressing)
				Expect(cd.Status).To(BeEquivalentTo(metav1.ConditionTrue))
				Expect(cd.Reason).To(Equal("CDIProgressing"))
				cd = apimetav1.FindStatusCondition(conditions, hcov1beta1.ConditionDegraded)
				Expect(cd.Status).To(BeEquivalentTo(metav1.ConditionFalse))
				Expect(cd.Reason).To(Equal(reconcileCompleted))
				cd = apimetav1.FindStatusCondition(conditions, hcov1beta1.ConditionUpgradeable)
				Expect(cd.Status).To(BeEquivalentTo(metav1.ConditionFalse))
				Expect(cd.Reason).To(Equal("CDIProgressing"))

				By("operator condition should be true even the upgradeable is false")
				validateOperatorCondition(r, metav1.ConditionTrue, hcoutil.UpgradeableAllowReason, hcoutil.UpgradeableAllowMessage)
			})

			It("should be not Available when a component is not Available", func() {
				expected := getBasicDeployment()
				conditionsv1.SetStatusCondition(&expected.cdi.Status.Conditions, conditionsv1.Condition{
					Type:    conditionsv1.ConditionAvailable,
					Status:  corev1.ConditionFalse,
					Reason:  "AvailableError",
					Message: "CDI Test Error message",
				})
				cl := expected.initClient()
				foundResource, _, _ := doReconcile(cl, expected.hco, nil)

				conditions := foundResource.Status.Conditions
				_, _ = fmt.Fprintln(GinkgoWriter, "\nActual Conditions:")
				wr := json.NewEncoder(GinkgoWriter)
				wr.SetIndent("", "  ")
				_ = wr.Encode(conditions)

				cd := apimetav1.FindStatusCondition(conditions, hcov1beta1.ConditionReconcileComplete)
				Expect(cd.Status).To(BeEquivalentTo(metav1.ConditionTrue))
				Expect(cd.Reason).To(Equal(reconcileCompleted))
				cd = apimetav1.FindStatusCondition(conditions, hcov1beta1.ConditionAvailable)
				Expect(cd.Status).To(BeEquivalentTo(metav1.ConditionFalse))
				Expect(cd.Reason).To(Equal("CDINotAvailable"))
				cd = apimetav1.FindStatusCondition(conditions, hcov1beta1.ConditionProgressing)
				Expect(cd.Status).To(BeEquivalentTo(metav1.ConditionFalse))
				Expect(cd.Reason).To(Equal(reconcileCompleted))
				cd = apimetav1.FindStatusCondition(conditions, hcov1beta1.ConditionDegraded)
				Expect(cd.Status).To(BeEquivalentTo(metav1.ConditionFalse))
				Expect(cd.Reason).To(Equal(reconcileCompleted))
				cd = apimetav1.FindStatusCondition(conditions, hcov1beta1.ConditionUpgradeable)
				Expect(cd.Status).To(BeEquivalentTo(metav1.ConditionTrue))
				Expect(cd.Reason).To(Equal(reconcileCompleted))
			})

			It("should be with all positive condition when all components working properly", func() {
				expected := getBasicDeployment()
				cl := expected.initClient()
				foundResource, _, _ := doReconcile(cl, expected.hco, nil)

				conditions := foundResource.Status.Conditions
				_, _ = fmt.Fprintln(GinkgoWriter, "\nActual Conditions:")
				wr := json.NewEncoder(GinkgoWriter)
				wr.SetIndent("", "  ")
				_ = wr.Encode(conditions)

				cd := apimetav1.FindStatusCondition(conditions, hcov1beta1.ConditionReconcileComplete)
				Expect(cd.Status).To(BeEquivalentTo(metav1.ConditionTrue))
				Expect(cd.Reason).To(Equal(reconcileCompleted))
				cd = apimetav1.FindStatusCondition(conditions, hcov1beta1.ConditionAvailable)
				Expect(cd.Status).To(BeEquivalentTo(metav1.ConditionTrue))
				Expect(cd.Reason).To(Equal(reconcileCompleted))
				cd = apimetav1.FindStatusCondition(conditions, hcov1beta1.ConditionProgressing)
				Expect(cd.Status).To(BeEquivalentTo(metav1.ConditionFalse))
				Expect(cd.Reason).To(Equal(reconcileCompleted))
				cd = apimetav1.FindStatusCondition(conditions, hcov1beta1.ConditionDegraded)
				Expect(cd.Status).To(BeEquivalentTo(metav1.ConditionFalse))
				Expect(cd.Reason).To(Equal(reconcileCompleted))
				cd = apimetav1.FindStatusCondition(conditions, hcov1beta1.ConditionUpgradeable)
				Expect(cd.Status).To(BeEquivalentTo(metav1.ConditionTrue))
				Expect(cd.Reason).To(Equal(reconcileCompleted))
			})

			It("should set the status of the last faulty component", func() {
				expected := getBasicDeployment()
				conditionsv1.SetStatusCondition(&expected.cdi.Status.Conditions, conditionsv1.Condition{
					Type:    conditionsv1.ConditionAvailable,
					Status:  corev1.ConditionFalse,
					Reason:  "AvailableError",
					Message: "CDI Test Error message",
				})

				conditionsv1.SetStatusCondition(&expected.cna.Status.Conditions, conditionsv1.Condition{
					Type:    conditionsv1.ConditionAvailable,
					Status:  corev1.ConditionFalse,
					Reason:  "AvailableError",
					Message: "CNA Test Error message",
				})
				cl := expected.initClient()
				foundResource, _, _ := doReconcile(cl, expected.hco, nil)

				conditions := foundResource.Status.Conditions
				_, _ = fmt.Fprintln(GinkgoWriter, "\nActual Conditions:")
				wr := json.NewEncoder(GinkgoWriter)
				wr.SetIndent("", "  ")
				_ = wr.Encode(conditions)

				cd := apimetav1.FindStatusCondition(conditions, hcov1beta1.ConditionReconcileComplete)
				Expect(cd.Status).To(BeEquivalentTo(metav1.ConditionTrue))
				Expect(cd.Reason).To(Equal(reconcileCompleted))
				cd = apimetav1.FindStatusCondition(conditions, hcov1beta1.ConditionAvailable)
				Expect(cd.Status).To(BeEquivalentTo(metav1.ConditionFalse))
				Expect(cd.Reason).To(Equal("NetworkAddonsConfigNotAvailable"))
				cd = apimetav1.FindStatusCondition(conditions, hcov1beta1.ConditionProgressing)
				Expect(cd.Status).To(BeEquivalentTo(metav1.ConditionFalse))
				Expect(cd.Reason).To(Equal(reconcileCompleted))
				cd = apimetav1.FindStatusCondition(conditions, hcov1beta1.ConditionDegraded)
				Expect(cd.Status).To(BeEquivalentTo(metav1.ConditionFalse))
				Expect(cd.Reason).To(Equal(reconcileCompleted))
				cd = apimetav1.FindStatusCondition(conditions, hcov1beta1.ConditionUpgradeable)
				Expect(cd.Status).To(BeEquivalentTo(metav1.ConditionTrue))
				Expect(cd.Reason).To(Equal(reconcileCompleted))
			})

			It("should not be upgradeable when a component is not upgradeable", func() {
				expected := getBasicDeployment()
				conditionsv1.SetStatusCondition(&expected.cdi.Status.Conditions, conditionsv1.Condition{
					Type:    conditionsv1.ConditionUpgradeable,
					Status:  corev1.ConditionFalse,
					Reason:  errorReason,
					Message: "CDI Test Error message",
				})
				cl := expected.initClient()
				foundResource, r, _ := doReconcile(cl, expected.hco, nil)

				conditions := foundResource.Status.Conditions
				GinkgoWriter.Println("\nActual Conditions:")
				wr := json.NewEncoder(GinkgoWriter)
				wr.SetIndent("", "  ")
				_ = wr.Encode(conditions)

				cd := apimetav1.FindStatusCondition(conditions, hcov1beta1.ConditionReconcileComplete)
				Expect(cd.Status).To(BeEquivalentTo(metav1.ConditionTrue))
				Expect(cd.Reason).To(Equal(reconcileCompleted))

				cd = apimetav1.FindStatusCondition(conditions, hcov1beta1.ConditionAvailable)
				Expect(cd.Status).To(BeEquivalentTo(metav1.ConditionTrue))
				Expect(cd.Reason).To(Equal(reconcileCompleted))

				cd = apimetav1.FindStatusCondition(conditions, hcov1beta1.ConditionProgressing)
				Expect(cd.Status).To(BeEquivalentTo(metav1.ConditionFalse))
				Expect(cd.Reason).To(Equal(reconcileCompleted))

				cd = apimetav1.FindStatusCondition(conditions, hcov1beta1.ConditionDegraded)
				Expect(cd.Status).To(BeEquivalentTo(metav1.ConditionFalse))
				Expect(cd.Reason).To(Equal(reconcileCompleted))

				cd = apimetav1.FindStatusCondition(conditions, hcov1beta1.ConditionUpgradeable)
				Expect(cd.Status).To(BeEquivalentTo(metav1.ConditionFalse))
				Expect(cd.Reason).To(Equal("CDINotUpgradeable"))
				Expect(cd.Message).To(Equal("CDI is not upgradeable: CDI Test Error message"))

				By("operator condition should be false")
				validateOperatorCondition(r, metav1.ConditionFalse, "CDINotUpgradeable", "is not upgradeable:")
			})

			It("should not be with its own reason and message if a component is not upgradeable, even if there are it also progressing", func() {
				expected := getBasicDeployment()
				conditionsv1.SetStatusCondition(&expected.cdi.Status.Conditions, conditionsv1.Condition{
					Type:    conditionsv1.ConditionUpgradeable,
					Status:  corev1.ConditionFalse,
					Reason:  errorReason,
					Message: "CDI Upgrade Error message",
				})
				conditionsv1.SetStatusCondition(&expected.cdi.Status.Conditions, conditionsv1.Condition{
					Type:    conditionsv1.ConditionProgressing,
					Status:  corev1.ConditionTrue,
					Reason:  errorReason,
					Message: "CDI Test Error message",
				})

				cl := expected.initClient()
				foundResource, r, _ := doReconcile(cl, expected.hco, nil)

				conditions := foundResource.Status.Conditions
				GinkgoWriter.Println("\nActual Conditions:")
				wr := json.NewEncoder(GinkgoWriter)
				wr.SetIndent("", "  ")
				_ = wr.Encode(conditions)

				cd := apimetav1.FindStatusCondition(conditions, hcov1beta1.ConditionReconcileComplete)
				Expect(cd.Status).To(BeEquivalentTo(metav1.ConditionTrue))
				Expect(cd.Reason).To(Equal(reconcileCompleted))

				cd = apimetav1.FindStatusCondition(conditions, hcov1beta1.ConditionAvailable)
				Expect(cd.Status).To(BeEquivalentTo(metav1.ConditionTrue))
				Expect(cd.Reason).To(Equal(reconcileCompleted))

				cd = apimetav1.FindStatusCondition(conditions, hcov1beta1.ConditionProgressing)
				Expect(cd.Status).To(BeEquivalentTo(metav1.ConditionTrue))
				Expect(cd.Reason).To(Equal("CDIProgressing"))

				cd = apimetav1.FindStatusCondition(conditions, hcov1beta1.ConditionDegraded)
				Expect(cd.Status).To(BeEquivalentTo(metav1.ConditionFalse))
				Expect(cd.Reason).To(Equal(reconcileCompleted))

				cd = apimetav1.FindStatusCondition(conditions, hcov1beta1.ConditionUpgradeable)
				Expect(cd.Status).To(BeEquivalentTo(metav1.ConditionFalse))
				Expect(cd.Reason).To(Equal("CDINotUpgradeable"))
				Expect(cd.Message).To(Equal("CDI is not upgradeable: CDI Upgrade Error message"))

				By("operator condition should be false")
				validateOperatorCondition(r, metav1.ConditionFalse, "CDINotUpgradeable", "is not upgradeable:")
			})

			It("should not be with its own reason and message if a component is not upgradeable, even if there are it also degraded", func() {
				expected := getBasicDeployment()
				conditionsv1.SetStatusCondition(&expected.cdi.Status.Conditions, conditionsv1.Condition{
					Type:    conditionsv1.ConditionUpgradeable,
					Status:  corev1.ConditionFalse,
					Reason:  errorReason,
					Message: "CDI Upgrade Error message",
				})
				conditionsv1.SetStatusCondition(&expected.cdi.Status.Conditions, conditionsv1.Condition{
					Type:    conditionsv1.ConditionDegraded,
					Status:  corev1.ConditionTrue,
					Reason:  errorReason,
					Message: "CDI Test Error message",
				})

				cl := expected.initClient()
				foundResource, r, _ := doReconcile(cl, expected.hco, nil)

				conditions := foundResource.Status.Conditions
				GinkgoWriter.Println("\nActual Conditions:")
				wr := json.NewEncoder(GinkgoWriter)
				wr.SetIndent("", "  ")
				_ = wr.Encode(conditions)

				cd := apimetav1.FindStatusCondition(conditions, hcov1beta1.ConditionReconcileComplete)
				Expect(cd.Status).To(BeEquivalentTo(metav1.ConditionTrue))
				Expect(cd.Reason).To(Equal(reconcileCompleted))

				cd = apimetav1.FindStatusCondition(conditions, hcov1beta1.ConditionAvailable)
				Expect(cd.Status).To(BeEquivalentTo(metav1.ConditionFalse))
				Expect(cd.Reason).To(Equal(commonDegradedReason))

				cd = apimetav1.FindStatusCondition(conditions, hcov1beta1.ConditionProgressing)
				Expect(cd.Status).To(BeEquivalentTo(metav1.ConditionFalse))
				Expect(cd.Reason).To(Equal(reconcileCompleted))

				cd = apimetav1.FindStatusCondition(conditions, hcov1beta1.ConditionDegraded)
				Expect(cd.Status).To(BeEquivalentTo(metav1.ConditionTrue))
				Expect(cd.Reason).To(Equal("CDIDegraded"))

				cd = apimetav1.FindStatusCondition(conditions, hcov1beta1.ConditionUpgradeable)
				Expect(cd.Status).To(BeEquivalentTo(metav1.ConditionFalse))
				Expect(cd.Reason).To(Equal("CDINotUpgradeable"))
				Expect(cd.Message).To(Equal("CDI is not upgradeable: CDI Upgrade Error message"))

				By("operator condition should be false")
				validateOperatorCondition(r, metav1.ConditionFalse, "CDINotUpgradeable", "is not upgradeable:")
			})
		})

		Context("Update Conflict Error", func() {
			BeforeEach(func() {
				Expect(os.Setenv("VIRTIOWIN_CONTAINER", commontestutils.VirtioWinImage)).To(Succeed())
			})

			It("Should requeue in case of update conflict", func() {
				expected := getBasicDeployment()
				expected.hco.Labels = nil
				cl := expected.initClient()
				rsc := schema.GroupResource{Group: hcoutil.APIVersionGroup, Resource: "hyperconvergeds.hco.kubevirt.io"}
				cl.InitiateUpdateErrors(func(obj client.Object) error {
					if _, ok := obj.(*hcov1beta1.HyperConverged); ok {
						return apierrors.NewConflict(rsc, "hco", errors.New("test error"))
					}
					return nil
				})
				r := initReconciler(cl, nil)

				r.ownVersion = cmp.Or(os.Getenv(hcoutil.HcoKvIoVersionName), version.Version)

				res, err := r.Reconcile(context.TODO(), request)
				Expect(err).To(MatchError(apierrors.IsConflict, "conflict error"))
				Expect(res.RequeueAfter).To(Equal(requeueAfter))
			})

			It("Should requeue in case of update status conflict", func() {
				expected := getBasicDeployment()
				expected.hco.Status.Conditions = nil
				cl := expected.initClient()
				rs := schema.GroupResource{Group: hcoutil.APIVersionGroup, Resource: "hyperconvergeds.hco.kubevirt.io"}
				cl.Status().(*commontestutils.HcoTestStatusWriter).InitiateErrors(apierrors.NewConflict(rs, "hco", errors.New("test error")))
				r := initReconciler(cl, nil)

				r.ownVersion = cmp.Or(os.Getenv(hcoutil.HcoKvIoVersionName), version.Version)

				res, err := r.Reconcile(context.TODO(), request)
				Expect(err).To(MatchError(apierrors.IsConflict, "conflict error"))
				Expect(res.RequeueAfter).To(Equal(requeueAfter))

			})
		})

		Context("Detection of a tainted configuration", func() {
			var (
				hcoNamespace *corev1.Namespace
				hco          *hcov1beta1.HyperConverged
			)
			BeforeEach(func() {
				hcoNamespace = commontestutils.NewHcoNamespace()
				hco = commontestutils.NewHco()
				UpdateVersion(&hco.Status, hcoVersionName, version.Version)
			})

			Context("Detection of a tainted configuration for kubevirt", func() {

				It("Raises a TaintedConfiguration condition upon detection of such configuration", func() {
					hco.Annotations = map[string]string{
						common.JSONPatchKVAnnotationName: `
						[
							{
								"op": "add",
								"path": "/spec/configuration/migrations",
								"value": {"allowPostCopy": true}
							}
						]`,
					}
					metrics.SetUnsafeModificationCount(0, common.JSONPatchKVAnnotationName)

					cl := commontestutils.InitClient([]client.Object{hcoNamespace, hco})
					r := initReconciler(cl, nil)

					By("Reconcile", func() {
						res, err := r.Reconcile(context.TODO(), request)
						Expect(err).ToNot(HaveOccurred())
						Expect(res).To(Equal(reconcile.Result{RequeueAfter: requeueAfter}))
					})

					foundResource := &hcov1beta1.HyperConverged{}
					Expect(
						cl.Get(context.TODO(),
							types.NamespacedName{Name: hco.Name, Namespace: hco.Namespace},
							foundResource),
					).To(Succeed())

					By("Verify HC conditions", func() {
						Expect(foundResource.Status.Conditions).To(ContainElement(commontestutils.RepresentCondition(metav1.Condition{
							Type:    hcov1beta1.ConditionTaintedConfiguration,
							Status:  metav1.ConditionTrue,
							Reason:  taintedConfigurationReason,
							Message: taintedConfigurationMessage,
						})))
					})

					By("verify that the metrics match to the annotation", func() {
						verifyUnsafeMetrics(1, common.JSONPatchKVAnnotationName)
					})

					By("Verify that KV was modified by the annotation", func() {
						kv := handlers.NewKubeVirtWithNameOnly(hco)
						Expect(
							cl.Get(context.TODO(),
								types.NamespacedName{Name: kv.Name, Namespace: kv.Namespace},
								kv),
						).To(Succeed())

						Expect(kv.Spec.Configuration.MigrationConfiguration).ToNot(BeNil())
						Expect(kv.Spec.Configuration.MigrationConfiguration.AllowPostCopy).ToNot(BeNil())
						Expect(*kv.Spec.Configuration.MigrationConfiguration.AllowPostCopy).To(BeTrue())
					})
				})

				It("Removes the TaintedConfiguration condition upon removal of such configuration", func() {
					hco.Status.Conditions = append(hco.Status.Conditions, metav1.Condition{
						Type:    hcov1beta1.ConditionTaintedConfiguration,
						Status:  metav1.ConditionTrue,
						Reason:  taintedConfigurationReason,
						Message: taintedConfigurationMessage,
					})

					metrics.SetUnsafeModificationCount(5, common.JSONPatchKVAnnotationName)

					cl := commontestutils.InitClient([]client.Object{hcoNamespace, hco})
					r := initReconciler(cl, nil)

					// Do the reconcile
					res, err := r.Reconcile(context.TODO(), request)
					Expect(err).ToNot(HaveOccurred())

					// Expecting "Requeue: false" since the conditions aren't empty
					Expect(res.IsZero()).To(BeTrue())

					// Get the HCO
					foundResource := &hcov1beta1.HyperConverged{}
					Expect(
						cl.Get(context.TODO(),
							types.NamespacedName{Name: hco.Name, Namespace: hco.Namespace},
							foundResource),
					).To(Succeed())

					// Check conditions
					Expect(foundResource.Status.Conditions).ToNot(ContainElement(commontestutils.RepresentCondition(metav1.Condition{
						Type:    hcov1beta1.ConditionTaintedConfiguration,
						Status:  metav1.ConditionTrue,
						Reason:  taintedConfigurationReason,
						Message: taintedConfigurationMessage,
					})))
					By("verify that the metrics match to the annotation", func() {
						verifyUnsafeMetrics(0, common.JSONPatchKVAnnotationName)
					})
				})

				It("Removes the TaintedConfiguration condition if the annotation is wrong", func() {
					hco.Status.Conditions = append(hco.Status.Conditions, metav1.Condition{
						Type:    hcov1beta1.ConditionTaintedConfiguration,
						Status:  metav1.ConditionTrue,
						Reason:  taintedConfigurationReason,
						Message: taintedConfigurationMessage,
					})

					metrics.SetUnsafeModificationCount(5, common.JSONPatchKVAnnotationName)

					hco.Annotations = map[string]string{
						// Set bad json format (missing comma)
						common.JSONPatchKVAnnotationName: `
						[
							{
								"op": "add"
								"path": "/spec/configuration/migrations",
								"value": {"allowPostCopy": true}
							}
						]`,
					}

					cl := commontestutils.InitClient([]client.Object{hcoNamespace, hco})
					r := initReconciler(cl, nil)

					By("Reconcile", func() {
						res, err := r.Reconcile(context.TODO(), request)
						Expect(err).ToNot(HaveOccurred())
						Expect(res.RequeueAfter).To(BeZero())
					})

					foundResource := &hcov1beta1.HyperConverged{}
					Expect(
						cl.Get(context.TODO(),
							types.NamespacedName{Name: hco.Name, Namespace: hco.Namespace},
							foundResource),
					).To(Succeed())

					// Check conditions
					Expect(foundResource.Status.Conditions).ToNot(ContainElement(commontestutils.RepresentCondition(metav1.Condition{
						Type:    hcov1beta1.ConditionTaintedConfiguration,
						Status:  metav1.ConditionTrue,
						Reason:  taintedConfigurationReason,
						Message: taintedConfigurationMessage,
					})))

					By("verify that the metrics match to the annotation", func() {
						verifyUnsafeMetrics(0, common.JSONPatchKVAnnotationName)
					})
				})
			})

			Context("Detection of a tainted configuration for cdi", func() {

				It("Raises a TaintedConfiguration condition upon detection of such configuration", func() {
					hco.Annotations = map[string]string{
						common.JSONPatchCDIAnnotationName: `[
					{
						"op": "add",
						"path": "/spec/config/featureGates/-",
						"value": "fg1"
					},
					{
						"op": "add",
						"path": "/spec/config/filesystemOverhead",
						"value": {"global": "50", "storageClass": {"AAA": "75", "BBB": "25"}}
					}
				]`,
					}

					metrics.SetUnsafeModificationCount(0, common.JSONPatchCDIAnnotationName)

					cl := commontestutils.InitClient([]client.Object{hcoNamespace, hco})
					r := initReconciler(cl, nil)

					By("Reconcile", func() {
						res, err := r.Reconcile(context.TODO(), request)
						Expect(err).ToNot(HaveOccurred())
						Expect(res).To(Equal(reconcile.Result{RequeueAfter: requeueAfter}))
					})

					foundResource := &hcov1beta1.HyperConverged{}
					Expect(
						cl.Get(context.TODO(),
							types.NamespacedName{Name: hco.Name, Namespace: hco.Namespace},
							foundResource),
					).To(Succeed())

					By("Verify HC conditions", func() {
						Expect(foundResource.Status.Conditions).To(ContainElement(commontestutils.RepresentCondition(metav1.Condition{
							Type:    hcov1beta1.ConditionTaintedConfiguration,
							Status:  metav1.ConditionTrue,
							Reason:  taintedConfigurationReason,
							Message: taintedConfigurationMessage,
						})))
					})

					By("verify that the metrics match to the annotation", func() {
						verifyUnsafeMetrics(2, common.JSONPatchCDIAnnotationName)
					})

					By("Verify that CDI was modified by the annotation", func() {
						cdi := handlers.NewCDIWithNameOnly(hco)
						Expect(
							cl.Get(context.TODO(),
								types.NamespacedName{Name: cdi.Name, Namespace: cdi.Namespace},
								cdi),
						).To(Succeed())

						Expect(cdi.Spec.Config.FeatureGates).To(ContainElement("fg1"))
						Expect(cdi.Spec.Config.FilesystemOverhead).ToNot(BeNil())
						Expect(cdi.Spec.Config.FilesystemOverhead.Global).To(BeEquivalentTo("50"))
						Expect(cdi.Spec.Config.FilesystemOverhead.StorageClass).ToNot(BeNil())
						Expect(cdi.Spec.Config.FilesystemOverhead.StorageClass["AAA"]).To(BeEquivalentTo("75"))
						Expect(cdi.Spec.Config.FilesystemOverhead.StorageClass["BBB"]).To(BeEquivalentTo("25"))

					})
				})

				It("Removes the TaintedConfiguration condition upon removal of such configuration", func() {
					hco.Status.Conditions = append(hco.Status.Conditions, metav1.Condition{
						Type:    hcov1beta1.ConditionTaintedConfiguration,
						Status:  metav1.ConditionTrue,
						Reason:  taintedConfigurationReason,
						Message: taintedConfigurationMessage,
					})

					metrics.SetUnsafeModificationCount(5, common.JSONPatchCDIAnnotationName)

					cl := commontestutils.InitClient([]client.Object{hcoNamespace, hco})
					r := initReconciler(cl, nil)

					// Do the reconcile
					res, err := r.Reconcile(context.TODO(), request)
					Expect(err).ToNot(HaveOccurred())

					// Expecting "Requeue: false" since the conditions aren't empty
					Expect(res.RequeueAfter).To(BeZero())

					// Get the HCO
					foundResource := &hcov1beta1.HyperConverged{}
					Expect(
						cl.Get(context.TODO(),
							types.NamespacedName{Name: hco.Name, Namespace: hco.Namespace},
							foundResource),
					).To(Succeed())

					// Check conditions
					Expect(foundResource.Status.Conditions).ToNot(ContainElement(commontestutils.RepresentCondition(metav1.Condition{
						Type:    hcov1beta1.ConditionTaintedConfiguration,
						Status:  metav1.ConditionTrue,
						Reason:  taintedConfigurationReason,
						Message: taintedConfigurationMessage,
					})))
					By("verify that the metrics match to the annotation", func() {
						verifyUnsafeMetrics(0, common.JSONPatchKVAnnotationName)
					})
				})

				It("Removes the TaintedConfiguration condition if the annotation is wrong", func() {
					hco.Status.Conditions = append(hco.Status.Conditions, metav1.Condition{
						Type:    hcov1beta1.ConditionTaintedConfiguration,
						Status:  metav1.ConditionTrue,
						Reason:  taintedConfigurationReason,
						Message: taintedConfigurationMessage,
					})

					metrics.SetUnsafeModificationCount(5, common.JSONPatchCDIAnnotationName)

					hco.Annotations = map[string]string{
						// Set bad json format (missing comma)
						common.JSONPatchKVAnnotationName: `[{`,
					}

					cl := commontestutils.InitClient([]client.Object{hcoNamespace, hco})
					r := initReconciler(cl, nil)

					By("Reconcile", func() {
						res, err := r.Reconcile(context.TODO(), request)
						Expect(err).ToNot(HaveOccurred())
						Expect(res.RequeueAfter).To(BeZero())
					})

					foundResource := &hcov1beta1.HyperConverged{}
					Expect(
						cl.Get(context.TODO(),
							types.NamespacedName{Name: hco.Name, Namespace: hco.Namespace},
							foundResource),
					).To(Succeed())

					// Check conditions
					Expect(foundResource.Status.Conditions).ToNot(ContainElement(commontestutils.RepresentCondition(metav1.Condition{
						Type:    hcov1beta1.ConditionTaintedConfiguration,
						Status:  metav1.ConditionTrue,
						Reason:  taintedConfigurationReason,
						Message: taintedConfigurationMessage,
					})))
					By("verify that the metrics match to the annotation", func() {
						verifyUnsafeMetrics(0, common.JSONPatchKVAnnotationName)
					})
				})
			})

			Context("Detection of a tainted configuration for cna", func() {

				It("Raises a TaintedConfiguration condition upon detection of such configuration", func() {
					hco.Annotations = map[string]string{
						common.JSONPatchCNAOAnnotationName: `[
							{
								"op": "add",
								"path": "/spec/kubeMacPool",
								"value": {"rangeStart": "1.1.1.1.1.1", "rangeEnd": "5.5.5.5.5.5" }
							},
							{
								"op": "add",
								"path": "/spec/imagePullPolicy",
								"value": "Always"
							}
						]`,
					}

					metrics.SetUnsafeModificationCount(0, common.JSONPatchCNAOAnnotationName)

					cl := commontestutils.InitClient([]client.Object{hcoNamespace, hco})
					r := initReconciler(cl, nil)

					By("Reconcile", func() {
						res, err := r.Reconcile(context.TODO(), request)
						Expect(err).ToNot(HaveOccurred())
						Expect(res).To(Equal(reconcile.Result{RequeueAfter: requeueAfter}))
					})

					foundResource := &hcov1beta1.HyperConverged{}
					Expect(
						cl.Get(context.TODO(),
							types.NamespacedName{Name: hco.Name, Namespace: hco.Namespace},
							foundResource),
					).To(Succeed())

					By("Verify HC conditions", func() {
						Expect(foundResource.Status.Conditions).To(ContainElement(commontestutils.RepresentCondition(metav1.Condition{
							Type:    hcov1beta1.ConditionTaintedConfiguration,
							Status:  metav1.ConditionTrue,
							Reason:  taintedConfigurationReason,
							Message: taintedConfigurationMessage,
						})))
					})

					By("verify that the metrics match to the annotation", func() {
						verifyUnsafeMetrics(2, common.JSONPatchCNAOAnnotationName)
					})

					By("Verify that CNA was modified by the annotation", func() {
						cna := handlers.NewNetworkAddonsWithNameOnly(hco)
						Expect(
							cl.Get(context.TODO(),
								types.NamespacedName{Name: cna.Name, Namespace: cna.Namespace},
								cna),
						).To(Succeed())

						Expect(cna.Spec.KubeMacPool).ToNot(BeNil())
						Expect(cna.Spec.KubeMacPool.RangeStart).To(Equal("1.1.1.1.1.1"))
						Expect(cna.Spec.KubeMacPool.RangeEnd).To(Equal("5.5.5.5.5.5"))
						Expect(cna.Spec.ImagePullPolicy).To(BeEquivalentTo("Always"))
					})
				})

				It("Removes the TaintedConfiguration condition upon removal of such configuration", func() {
					hco.Status.Conditions = append(hco.Status.Conditions, metav1.Condition{
						Type:    hcov1beta1.ConditionTaintedConfiguration,
						Status:  metav1.ConditionTrue,
						Reason:  taintedConfigurationReason,
						Message: taintedConfigurationMessage,
					})
					metrics.SetUnsafeModificationCount(5, common.JSONPatchCNAOAnnotationName)

					cl := commontestutils.InitClient([]client.Object{hcoNamespace, hco})
					r := initReconciler(cl, nil)

					// Do the reconcile
					res, err := r.Reconcile(context.TODO(), request)
					Expect(err).ToNot(HaveOccurred())

					// Expecting "Requeue: false" since the conditions aren't empty
					Expect(res.RequeueAfter).To(BeZero())

					// Get the HCO
					foundResource := &hcov1beta1.HyperConverged{}
					Expect(
						cl.Get(context.TODO(),
							types.NamespacedName{Name: hco.Name, Namespace: hco.Namespace},
							foundResource),
					).To(Succeed())

					// Check conditions
					Expect(foundResource.Status.Conditions).ToNot(ContainElement(commontestutils.RepresentCondition(metav1.Condition{
						Type:    hcov1beta1.ConditionTaintedConfiguration,
						Status:  metav1.ConditionTrue,
						Reason:  taintedConfigurationReason,
						Message: taintedConfigurationMessage,
					})))
					By("verify that the metrics match to the annotation", func() {
						verifyUnsafeMetrics(0, common.JSONPatchCNAOAnnotationName)
					})
				})

				It("Removes the TaintedConfiguration condition if the annotation is wrong", func() {
					hco.Annotations = map[string]string{
						// Set bad json
						common.JSONPatchKVAnnotationName: `[{`,
					}
					metrics.SetUnsafeModificationCount(5, common.JSONPatchCNAOAnnotationName)

					cl := commontestutils.InitClient([]client.Object{hcoNamespace, hco})
					r := initReconciler(cl, nil)

					By("Reconcile", func() {
						res, err := r.Reconcile(context.TODO(), request)
						Expect(err).ToNot(HaveOccurred())
						Expect(res).To(Equal(reconcile.Result{RequeueAfter: requeueAfter}))
					})

					foundResource := &hcov1beta1.HyperConverged{}
					Expect(
						cl.Get(context.TODO(),
							types.NamespacedName{Name: hco.Name, Namespace: hco.Namespace},
							foundResource),
					).To(Succeed())

					// Check conditions
					Expect(foundResource.Status.Conditions).ToNot(ContainElement(commontestutils.RepresentCondition(metav1.Condition{
						Type:    hcov1beta1.ConditionTaintedConfiguration,
						Status:  metav1.ConditionTrue,
						Reason:  taintedConfigurationReason,
						Message: taintedConfigurationMessage,
					})))
					By("verify that the metrics match to the annotation", func() {
						verifyUnsafeMetrics(0, common.JSONPatchCNAOAnnotationName)
					})
				})
			})

			Context("Detection of a tainted configuration for SSP", func() {

				It("Raises a TaintedConfiguration condition upon detection of such configuration", func() {
					hco.Annotations = map[string]string{
						common.JSONPatchSSPAnnotationName: `[
							{
								"op": "replace",
								"path": "/spec/templateValidator/replicas",
								"value": 5
							}
						]`,
					}

					metrics.SetUnsafeModificationCount(0, common.JSONPatchSSPAnnotationName)

					cl := commontestutils.InitClient([]client.Object{hcoNamespace, hco})
					r := initReconciler(cl, nil)

					By("Reconcile", func() {
						res, err := r.Reconcile(context.TODO(), request)
						Expect(err).ToNot(HaveOccurred())
						Expect(res).To(Equal(reconcile.Result{RequeueAfter: requeueAfter}))
					})

					foundResource := &hcov1beta1.HyperConverged{}
					Expect(
						cl.Get(context.TODO(),
							types.NamespacedName{Name: hco.Name, Namespace: hco.Namespace},
							foundResource),
					).To(Succeed())

					By("Verify HC conditions", func() {
						Expect(foundResource.Status.Conditions).To(ContainElement(commontestutils.RepresentCondition(metav1.Condition{
							Type:    hcov1beta1.ConditionTaintedConfiguration,
							Status:  metav1.ConditionTrue,
							Reason:  taintedConfigurationReason,
							Message: taintedConfigurationMessage,
						})))
					})

					By("verify that the metrics match to the annotation", func() {
						verifyUnsafeMetrics(1, common.JSONPatchSSPAnnotationName)
					})

					By("Verify that SSP was modified by the annotation", func() {
						ssp := handlers.NewSSPWithNameOnly(hco)
						Expect(
							cl.Get(context.TODO(),
								types.NamespacedName{Name: ssp.Name, Namespace: ssp.Namespace},
								ssp),
						).To(Succeed())

						Expect(ssp.Spec.TemplateValidator.Replicas).ToNot(BeNil())
						Expect(*ssp.Spec.TemplateValidator.Replicas).To(Equal(int32(5)))
					})
				})

				It("Removes the TaintedConfiguration condition upon removal of such configuration", func() {
					hco.Status.Conditions = append(hco.Status.Conditions, metav1.Condition{
						Type:    hcov1beta1.ConditionTaintedConfiguration,
						Status:  metav1.ConditionTrue,
						Reason:  taintedConfigurationReason,
						Message: taintedConfigurationMessage,
					})
					metrics.SetUnsafeModificationCount(5, common.JSONPatchSSPAnnotationName)

					cl := commontestutils.InitClient([]client.Object{hcoNamespace, hco})
					r := initReconciler(cl, nil)

					// Do the reconcile
					res, err := r.Reconcile(context.TODO(), request)
					Expect(err).ToNot(HaveOccurred())

					// Expecting "Requeue: false" since the conditions aren't empty
					Expect(res.RequeueAfter).To(BeZero())

					// Get the HCO
					foundResource := &hcov1beta1.HyperConverged{}
					Expect(
						cl.Get(context.TODO(),
							types.NamespacedName{Name: hco.Name, Namespace: hco.Namespace},
							foundResource),
					).To(Succeed())

					// Check conditions
					Expect(foundResource.Status.Conditions).ToNot(ContainElement(commontestutils.RepresentCondition(metav1.Condition{
						Type:    hcov1beta1.ConditionTaintedConfiguration,
						Status:  metav1.ConditionTrue,
						Reason:  taintedConfigurationReason,
						Message: taintedConfigurationMessage,
					})))
					By("verify that the metrics match to the annotation", func() {
						verifyUnsafeMetrics(0, common.JSONPatchSSPAnnotationName)
					})
				})

				It("Removes the TaintedConfiguration condition if the annotation is wrong", func() {
					hco.Annotations = map[string]string{
						// Set bad json
						common.JSONPatchSSPAnnotationName: `[{`,
					}
					metrics.SetUnsafeModificationCount(5, common.JSONPatchSSPAnnotationName)

					cl := commontestutils.InitClient([]client.Object{hcoNamespace, hco})
					r := initReconciler(cl, nil)

					By("Reconcile", func() {
						res, err := r.Reconcile(context.TODO(), request)
						Expect(err).ToNot(HaveOccurred())
						Expect(res).To(Equal(reconcile.Result{RequeueAfter: requeueAfter}))
					})

					foundResource := &hcov1beta1.HyperConverged{}
					Expect(
						cl.Get(context.TODO(),
							types.NamespacedName{Name: hco.Name, Namespace: hco.Namespace},
							foundResource),
					).To(Succeed())

					// Check conditions
					Expect(foundResource.Status.Conditions).ToNot(ContainElement(commontestutils.RepresentCondition(metav1.Condition{
						Type:    hcov1beta1.ConditionTaintedConfiguration,
						Status:  metav1.ConditionTrue,
						Reason:  taintedConfigurationReason,
						Message: taintedConfigurationMessage,
					})))
					By("verify that the metrics match to the annotation", func() {
						verifyUnsafeMetrics(0, common.JSONPatchSSPAnnotationName)
					})
				})
			})

			Context("Detection of a tainted configuration for all the annotations", func() {
				It("Raises a TaintedConfiguration condition upon detection of such configuration", func() {
					hco.Annotations = map[string]string{
						common.JSONPatchKVAnnotationName: `
						[
							{
								"op": "add",
								"path": "/spec/configuration/migrations",
								"value": {"allowPostCopy": true}
							}
						]`,
						common.JSONPatchCDIAnnotationName: `[
							{
								"op": "add",
								"path": "/spec/config/featureGates/-",
								"value": "fg1"
							},
							{
								"op": "add",
								"path": "/spec/config/filesystemOverhead",
								"value": {"global": "50", "storageClass": {"AAA": "75", "BBB": "25"}}
							}
						]`,
						common.JSONPatchCNAOAnnotationName: `[
							{
								"op": "add",
								"path": "/spec/kubeMacPool",
								"value": {"rangeStart": "1.1.1.1.1.1", "rangeEnd": "5.5.5.5.5.5" }
							},
							{
								"op": "add",
								"path": "/spec/imagePullPolicy",
								"value": "Always"
							}
						]`,
						common.JSONPatchSSPAnnotationName: `[
							{
								"op": "replace",
								"path": "/spec/templateValidator/replicas",
								"value": 5
							}
						]`,
					}
					metrics.SetUnsafeModificationCount(0, common.JSONPatchKVAnnotationName)
					metrics.SetUnsafeModificationCount(0, common.JSONPatchCDIAnnotationName)
					metrics.SetUnsafeModificationCount(0, common.JSONPatchCNAOAnnotationName)
					metrics.SetUnsafeModificationCount(0, common.JSONPatchSSPAnnotationName)

					cl := commontestutils.InitClient([]client.Object{hcoNamespace, hco})
					r := initReconciler(cl, nil)

					By("Reconcile", func() {
						res, err := r.Reconcile(context.TODO(), request)
						Expect(err).ToNot(HaveOccurred())
						Expect(res).To(Equal(reconcile.Result{RequeueAfter: requeueAfter}))
					})

					foundResource := &hcov1beta1.HyperConverged{}
					Expect(
						cl.Get(context.TODO(),
							types.NamespacedName{Name: hco.Name, Namespace: hco.Namespace},
							foundResource),
					).To(Succeed())

					By("Verify HC conditions", func() {
						Expect(foundResource.Status.Conditions).To(ContainElement(commontestutils.RepresentCondition(metav1.Condition{
							Type:    hcov1beta1.ConditionTaintedConfiguration,
							Status:  metav1.ConditionTrue,
							Reason:  taintedConfigurationReason,
							Message: taintedConfigurationMessage,
						})))
					})

					By("verify that the metrics match to the annotation", func() {
						verifyUnsafeMetrics(1, common.JSONPatchKVAnnotationName)
						verifyUnsafeMetrics(2, common.JSONPatchCDIAnnotationName)
						verifyUnsafeMetrics(2, common.JSONPatchCNAOAnnotationName)
						verifyUnsafeMetrics(1, common.JSONPatchSSPAnnotationName)
					})

					By("Verify that KV was modified by the annotation", func() {
						kv := handlers.NewKubeVirtWithNameOnly(hco)
						Expect(
							cl.Get(context.TODO(),
								types.NamespacedName{Name: kv.Name, Namespace: kv.Namespace},
								kv),
						).To(Succeed())

						Expect(kv.Spec.Configuration.MigrationConfiguration).ToNot(BeNil())
						Expect(kv.Spec.Configuration.MigrationConfiguration.AllowPostCopy).ToNot(BeNil())
						Expect(*kv.Spec.Configuration.MigrationConfiguration.AllowPostCopy).To(BeTrue())
					})
					By("Verify that CDI was modified by the annotation", func() {
						cdi := handlers.NewCDIWithNameOnly(hco)
						Expect(
							cl.Get(context.TODO(),
								types.NamespacedName{Name: cdi.Name, Namespace: cdi.Namespace},
								cdi),
						).To(Succeed())

						Expect(cdi.Spec.Config.FeatureGates).To(ContainElement("fg1"))
						Expect(cdi.Spec.Config.FilesystemOverhead).ToNot(BeNil())
						Expect(cdi.Spec.Config.FilesystemOverhead.Global).To(BeEquivalentTo("50"))
						Expect(cdi.Spec.Config.FilesystemOverhead.StorageClass).ToNot(BeNil())
						Expect(cdi.Spec.Config.FilesystemOverhead.StorageClass["AAA"]).To(BeEquivalentTo("75"))
						Expect(cdi.Spec.Config.FilesystemOverhead.StorageClass["BBB"]).To(BeEquivalentTo("25"))

					})
					By("Verify that CNA was modified by the annotation", func() {
						cna := handlers.NewNetworkAddonsWithNameOnly(hco)
						Expect(
							cl.Get(context.TODO(),
								types.NamespacedName{Name: cna.Name, Namespace: cna.Namespace},
								cna),
						).To(Succeed())

						Expect(cna.Spec.KubeMacPool).ToNot(BeNil())
						Expect(cna.Spec.KubeMacPool.RangeStart).To(Equal("1.1.1.1.1.1"))
						Expect(cna.Spec.KubeMacPool.RangeEnd).To(Equal("5.5.5.5.5.5"))
						Expect(cna.Spec.ImagePullPolicy).To(BeEquivalentTo("Always"))
					})
					By("Verify that SSP was modified by the annotation", func() {
						ssp := handlers.NewSSPWithNameOnly(hco)
						Expect(
							cl.Get(context.TODO(),
								types.NamespacedName{Name: ssp.Name, Namespace: ssp.Namespace},
								ssp),
						).To(Succeed())

						Expect(ssp.Spec.TemplateValidator.Replicas).ToNot(BeNil())
						Expect(*ssp.Spec.TemplateValidator.Replicas).To(Equal(int32(5)))
					})
				})
			})
		})
	})
})

func verifyUnsafeMetrics(expected int, annotation string) {
	count, err := metrics.GetUnsafeModificationsCount(annotation)
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
	ExpectWithOffset(1, count).To(BeEquivalentTo(expected))
}

func verifyHyperConvergedCRExistsMetricTrue() {
	hcExists, err := metrics.IsHCOMetricHyperConvergedExists()
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
	ExpectWithOffset(1, hcExists).To(BeTrue())
}

func verifyHyperConvergedCRExistsMetricFalse() {
	hcExists, err := metrics.IsHCOMetricHyperConvergedExists()
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
	ExpectWithOffset(1, hcExists).To(BeFalse())
}

func verifySystemHealthStatusHealthy(hco *hcov1beta1.HyperConverged) {
	ExpectWithOffset(1, hco.Status.SystemHealthStatus).To(Equal(systemHealthStatusHealthy))

	systemHealthStatusMetric, err := metrics.GetHCOMetricSystemHealthStatus()
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
	ExpectWithOffset(1, systemHealthStatusMetric).To(Equal(metrics.SystemHealthStatusHealthy))
}

func verifySystemHealthStatusError(hco *hcov1beta1.HyperConverged) {
	ExpectWithOffset(1, hco.Status.SystemHealthStatus).To(Equal(systemHealthStatusError))

	systemHealthStatusMetric, err := metrics.GetHCOMetricSystemHealthStatus()
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
	ExpectWithOffset(1, systemHealthStatusMetric).To(Equal(metrics.SystemHealthStatusError))
}

func searchInRelatedObjects(relatedObjects []corev1.ObjectReference, kind, name string) bool {
	for _, obj := range relatedObjects {
		if obj.Kind == kind && obj.Name == name {
			return true
		}
	}
	return false
}

func openshift2CdiSecProfile(hcProfile *openshiftconfigv1.TLSSecurityProfile) *cdiv1beta1.TLSSecurityProfile {
	var custom *cdiv1beta1.CustomTLSProfile
	if hcProfile.Custom != nil {
		custom = &cdiv1beta1.CustomTLSProfile{
			TLSProfileSpec: cdiv1beta1.TLSProfileSpec{
				Ciphers:       hcProfile.Custom.Ciphers,
				MinTLSVersion: cdiv1beta1.TLSProtocolVersion(hcProfile.Custom.MinTLSVersion),
			},
		}
	}

	return &cdiv1beta1.TLSSecurityProfile{
		Type:         cdiv1beta1.TLSProfileType(hcProfile.Type),
		Old:          (*cdiv1beta1.OldTLSProfile)(hcProfile.Old),
		Intermediate: (*cdiv1beta1.IntermediateTLSProfile)(hcProfile.Intermediate),
		Modern:       (*cdiv1beta1.ModernTLSProfile)(hcProfile.Modern),
		Custom:       custom,
	}
}
