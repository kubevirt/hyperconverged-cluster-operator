package operandhandler

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	consolev1 "github.com/openshift/api/console/v1"
	imagev1 "github.com/openshift/api/image/v1"
	objectreferencesv1 "github.com/openshift/custom-resource-status/objectreferences/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/reference"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	networkaddonsv1 "github.com/kubevirt/cluster-network-addons-operator/pkg/apis/networkaddonsoperator/v1"
	kubevirtcorev1 "kubevirt.io/api/core/v1"
	cdiv1beta1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/handlers"
)

func TestOperators(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "OperatorHandler Suite")
}

var _ = Describe("Test operandHandler", func() {
	origLogger := logger
	BeforeEach(func() {
		logger = GinkgoLogr
		DeferCleanup(func() {
			logger = origLogger
		})

		testFileLocation := getTestFilesLocation()

		qsVal, qsExists := os.LookupEnv(handlers.QuickStartManifestLocationVarName)
		Expect(os.Setenv(handlers.QuickStartManifestLocationVarName, testFileLocation+"/quickstarts")).To(Succeed())
		dashboardVal, dashboardExists := os.LookupEnv(handlers.DashboardManifestLocationVarName)
		Expect(os.Setenv(handlers.DashboardManifestLocationVarName, testFileLocation+"/dashboards")).To(Succeed())
		imageStreamVal, imageStreamExists := os.LookupEnv(handlers.ImageStreamManifestLocationVarName)
		Expect(os.Setenv(handlers.ImageStreamManifestLocationVarName, testFileLocation+"/imageStreams")).To(Succeed())

		DeferCleanup(func() {
			if qsExists {
				Expect(os.Setenv(handlers.QuickStartManifestLocationVarName, qsVal)).To(Succeed())
			} else {
				Expect(os.Unsetenv(handlers.QuickStartManifestLocationVarName)).To(Succeed())
			}
			if dashboardExists {
				Expect(os.Setenv(handlers.DashboardManifestLocationVarName, dashboardVal)).To(Succeed())
			} else {
				Expect(os.Unsetenv(handlers.DashboardManifestLocationVarName)).To(Succeed())
			}
			if imageStreamExists {
				Expect(os.Setenv(handlers.ImageStreamManifestLocationVarName, imageStreamVal)).To(Succeed())
			} else {
				Expect(os.Unsetenv(handlers.ImageStreamManifestLocationVarName)).To(Succeed())
			}
		})
	})

	Context("Test operandHandler", func() {

		var (
			hcoNamespace *corev1.Namespace
		)

		BeforeEach(func() {
			hcoNamespace = commontestutils.NewHcoNamespace()
		})

		It("should create all objects are created", func() {
			hco := commontestutils.NewHco()
			ci := commontestutils.ClusterInfoMock{}
			cli := commontestutils.InitClient([]client.Object{hcoNamespace, hco, ci.GetCSV()})

			eventEmitter := commontestutils.NewEventEmitterMock()

			handler := NewOperandHandler(cli, commontestutils.GetScheme(), ci, eventEmitter)
			handler.FirstUseInitiation(commontestutils.GetScheme(), ci, hco)

			req := commontestutils.NewReq(hco)

			Expect(handler.Ensure(req)).To(Succeed())
			expectedEvents := []commontestutils.MockEvent{
				{
					EventType: corev1.EventTypeNormal,
					Reason:    "Created",
					Msg:       "Created PriorityClass kubevirt-cluster-critical",
				},
				{
					EventType: corev1.EventTypeNormal,
					Reason:    "Created",
					Msg:       "Created KubeVirt kubevirt-kubevirt-hyperconverged",
				},
				{
					EventType: corev1.EventTypeNormal,
					Reason:    "Created",
					Msg:       "Created CDI cdi-kubevirt-hyperconverged",
				},
				{
					EventType: corev1.EventTypeNormal,
					Reason:    "Created",
					Msg:       "Created NetworkAddonsConfig cluster",
				},
				{
					EventType: corev1.EventTypeNormal,
					Reason:    "Created",
					Msg:       "Created SSP ssp-kubevirt-hyperconverged",
				},
				{
					EventType: corev1.EventTypeNormal,
					Reason:    "Created",
					Msg:       "Created ConsoleQuickStart test-quick-start",
				},
				{
					EventType: corev1.EventTypeNormal,
					Reason:    "Created",
					Msg:       "Created ConfigMap grafana-dashboard-kubevirt-top-consumers",
				},
			}
			Expect(eventEmitter.CheckEvents(expectedEvents)).To(BeTrue())

			By("make sure the KV object created", func() {
				// Read back KV
				kvList := kubevirtcorev1.KubeVirtList{}
				Expect(cli.List(req.Ctx, &kvList)).To(Succeed())
				Expect(kvList).ToNot(BeNil())
				Expect(kvList.Items).To(HaveLen(1))
				Expect(kvList.Items[0].Name).To(Equal("kubevirt-kubevirt-hyperconverged"))
			})

			By("make sure the CNA object created", func() {
				// Read back CNA
				cnaList := networkaddonsv1.NetworkAddonsConfigList{}
				Expect(cli.List(req.Ctx, &cnaList)).To(Succeed())
				Expect(cnaList).ToNot(BeNil())
				Expect(cnaList.Items).To(HaveLen(1))
				Expect(cnaList.Items[0].Name).To(Equal("cluster"))
			})

			By("make sure the CDI object created", func() {
				// Read back CDI
				cdiList := cdiv1beta1.CDIList{}
				Expect(cli.List(req.Ctx, &cdiList)).To(Succeed())
				Expect(cdiList).ToNot(BeNil())
				Expect(cdiList.Items).To(HaveLen(1))
				Expect(cdiList.Items[0].Name).To(Equal("cdi-kubevirt-hyperconverged"))
			})

			By("make sure the ConsoleQuickStart object created", func() {
				// Read back the ConsoleQuickStart
				qsList := consolev1.ConsoleQuickStartList{}
				Expect(cli.List(req.Ctx, &qsList)).To(Succeed())
				Expect(qsList).ToNot(BeNil())
				Expect(qsList.Items).To(HaveLen(1))
				Expect(qsList.Items[0].Name).To(Equal("test-quick-start"))
			})

			By("make sure the Dashboard confimap created", func() {
				cmList := corev1.ConfigMapList{}
				Expect(cli.List(req.Ctx, &cmList, &client.ListOptions{Namespace: "openshift-config-managed"})).To(Succeed())
				Expect(cmList).ToNot(BeNil())
				Expect(cmList.Items).To(HaveLen(1))
				Expect(cmList.Items[0].Name).To(Equal("grafana-dashboard-kubevirt-top-consumers"))
			})
		})

		It("should handle errors on Ensure loop", func() {
			hco := commontestutils.NewHco()
			cli := commontestutils.InitClient([]client.Object{hcoNamespace, hco})

			eventEmitter := commontestutils.NewEventEmitterMock()
			ci := commontestutils.ClusterInfoMock{}

			handler := NewOperandHandler(cli, commontestutils.GetScheme(), ci, eventEmitter)
			handler.FirstUseInitiation(commontestutils.GetScheme(), ci, hco)

			req := commontestutils.NewReq(hco)

			// fail to create CDI
			fakeError := fmt.Errorf("fake create CDI error")
			cli.InitiateCreateErrors(func(obj client.Object) error {
				if _, ok := obj.(*cdiv1beta1.CDI); ok {
					return fakeError
				}

				return nil
			})

			Expect(handler.Ensure(req)).To(Equal(fakeError))

			Expect(req.ComponentUpgradeInProgress).To(BeFalse())
			cond := req.Conditions[hcov1beta1.ConditionReconcileComplete]
			Expect(cond).ToNot(BeNil())
			Expect(cond.Status).To(Equal(metav1.ConditionFalse))
			Expect(cond.Reason).To(Equal(reconcileFailed))
			Expect(cond.Message).To(Equal(fmt.Sprintf("Error while reconciling: %v", fakeError)))

			By("make sure the CDI object not created", func() {
				// Read back CDI
				cdiList := cdiv1beta1.CDIList{}
				Expect(cli.List(req.Ctx, &cdiList)).To(Succeed())
				Expect(cdiList).ToNot(BeNil())
				Expect(cdiList.Items).To(BeEmpty())
			})
		})

		It("make sure the all objects are deleted", func() {
			hco := commontestutils.NewHco()
			ci := commontestutils.ClusterInfoMock{}
			cli := commontestutils.InitClient([]client.Object{hcoNamespace, hco, ci.GetCSV()})

			eventEmitter := commontestutils.NewEventEmitterMock()

			handler := NewOperandHandler(cli, commontestutils.GetScheme(), ci, eventEmitter)
			handler.FirstUseInitiation(commontestutils.GetScheme(), ci, hco)

			req := commontestutils.NewReq(hco)
			Expect(handler.Ensure(req)).To(Succeed())

			eventEmitter.Reset()
			Expect(handler.EnsureDeleted(req)).To(Succeed())

			expectedEvents := []commontestutils.MockEvent{
				{
					EventType: corev1.EventTypeNormal,
					Reason:    "Killing",
					Msg:       "Removed ConsoleCLIDownload virtctl-clidownloads-kubevirt-hyperconverged",
				},
				{
					EventType: corev1.EventTypeNormal,
					Reason:    "Killing",
					Msg:       "Removed NetworkAddonsConfig cluster",
				},
				{
					EventType: corev1.EventTypeNormal,
					Reason:    "Killing",
					Msg:       "Removed CDI cdi-kubevirt-hyperconverged",
				},
				{
					EventType: corev1.EventTypeNormal,
					Reason:    "Killing",
					Msg:       "Removed ConsoleQuickStart test-quick-start",
				},
				{
					EventType: corev1.EventTypeNormal,
					Reason:    "Killing",
					Msg:       "Removed SSP ssp-kubevirt-hyperconverged",
				},
				{
					EventType: corev1.EventTypeNormal,
					Reason:    "Killing",
					Msg:       "Removed KubeVirt kubevirt-kubevirt-hyperconverged",
				},
				{
					EventType: corev1.EventTypeNormal,
					Reason:    "Killing",
					Msg:       "Removed ConfigMap grafana-dashboard-kubevirt-top-consumers",
				},
			}
			Expect(eventEmitter.CheckEvents(expectedEvents)).To(BeTrue())

			By("check that KV is deleted", func() {
				// Read back KV
				kvList := kubevirtcorev1.KubeVirtList{}
				Expect(cli.List(req.Ctx, &kvList)).To(Succeed())
				Expect(kvList).ToNot(BeNil())
				Expect(kvList.Items).To(BeEmpty())
			})

			By("make sure the CNA object deleted", func() {
				// Read back CNA
				cnaList := networkaddonsv1.NetworkAddonsConfigList{}
				Expect(cli.List(req.Ctx, &cnaList)).To(Succeed())
				Expect(cnaList).ToNot(BeNil())
				Expect(cnaList.Items).To(BeEmpty())
			})

			By("make sure the CDI object deleted", func() {
				// Read back CDI
				cdiList := cdiv1beta1.CDIList{}
				Expect(cli.List(req.Ctx, &cdiList)).To(Succeed())
				Expect(cdiList).ToNot(BeNil())
				Expect(cdiList.Items).To(BeEmpty())
			})

			By("check that ConsoleQuickStart is deleted", func() {
				// Read back the ConsoleQuickStart
				qsList := consolev1.ConsoleQuickStartList{}
				Expect(cli.List(req.Ctx, &qsList)).To(Succeed())
				Expect(qsList).ToNot(BeNil())
				Expect(qsList.Items).To(BeEmpty())
			})
		})

		It("delete KV error handling", func() {
			hco := commontestutils.NewHco()
			ci := commontestutils.ClusterInfoMock{}
			cli := commontestutils.InitClient([]client.Object{hcoNamespace, hco, ci.GetCSV()})

			eventEmitter := commontestutils.NewEventEmitterMock()

			handler := NewOperandHandler(cli, commontestutils.GetScheme(), ci, eventEmitter)
			handler.FirstUseInitiation(commontestutils.GetScheme(), ci, hco)

			req := commontestutils.NewReq(hco)
			Expect(handler.Ensure(req)).To(Succeed())

			fakeError := fmt.Errorf("fake KV deletion error")
			cli.InitiateDeleteErrors(func(obj client.Object) error {
				if unstructed, ok := obj.(runtime.Unstructured); ok {
					kind := unstructed.GetObjectKind()
					if kind.GroupVersionKind().Kind == "KubeVirt" {
						return fakeError
					}
				}
				return nil
			})

			expectedEvents := []commontestutils.MockEvent{
				{
					EventType: corev1.EventTypeWarning,
					Reason:    ErrVirtUninstall,
					Msg:       uninstallVirtErrorMsg + fakeError.Error(),
				},
			}
			eventEmitter.Reset()
			err := handler.EnsureDeleted(req)
			Expect(err).To(Equal(fakeError))

			By("Check that event was emitted", func() {
				Expect(eventEmitter.CheckEvents(expectedEvents)).To(BeTrue())
			})

			By("check that KV still exists", func() {
				// Read back KV
				kvList := kubevirtcorev1.KubeVirtList{}
				Expect(cli.List(req.Ctx, &kvList)).To(Succeed())
				Expect(kvList).ToNot(BeNil())
				Expect(kvList.Items).To(HaveLen(1))
				Expect(kvList.Items[0].Name).To(Equal("kubevirt-kubevirt-hyperconverged"))
			})
		})

		It("delete CDI error handling", func() {
			hco := commontestutils.NewHco()
			ci := commontestutils.ClusterInfoMock{}
			cli := commontestutils.InitClient([]client.Object{hcoNamespace, hco, ci.GetCSV()})

			eventEmitter := commontestutils.NewEventEmitterMock()

			handler := NewOperandHandler(cli, commontestutils.GetScheme(), ci, eventEmitter)
			handler.FirstUseInitiation(commontestutils.GetScheme(), ci, hco)

			req := commontestutils.NewReq(hco)
			Expect(handler.Ensure(req)).To(Succeed())

			fakeError := fmt.Errorf("fake CDI deletion error")
			cli.InitiateDeleteErrors(func(obj client.Object) error {
				if unstructed, ok := obj.(runtime.Unstructured); ok {
					kind := unstructed.GetObjectKind()
					if kind.GroupVersionKind().Kind == "CDI" {
						return fakeError
					}
				}
				return nil
			})

			expectedEvents := []commontestutils.MockEvent{
				{
					EventType: corev1.EventTypeWarning,
					Reason:    ErrCDIUninstall,
					Msg:       uninstallCDIErrorMsg + fakeError.Error(),
				},
			}

			eventEmitter.Reset()
			err := handler.EnsureDeleted(req)
			Expect(err).To(Equal(fakeError))

			By("Check that event was emitted", func() {
				Expect(eventEmitter.CheckEvents(expectedEvents)).To(BeTrue())
			})

			By("make sure the CDI object still exists", func() {
				// Read back KV
				cdiList := cdiv1beta1.CDIList{}
				Expect(cli.List(req.Ctx, &cdiList)).To(Succeed())
				Expect(cdiList).ToNot(BeNil())
				Expect(cdiList.Items).To(HaveLen(1))
				Expect(cdiList.Items[0].Name).To(Equal("cdi-kubevirt-hyperconverged"))
			})
		})

		It("default delete error handling", func() {
			hco := commontestutils.NewHco()
			ci := commontestutils.ClusterInfoMock{}
			cli := commontestutils.InitClient([]client.Object{hcoNamespace, hco, ci.GetCSV()})

			fakeError := fmt.Errorf("fake CNA deletion error")
			eventEmitter := commontestutils.NewEventEmitterMock()

			handler := NewOperandHandler(cli, commontestutils.GetScheme(), ci, eventEmitter)
			handler.FirstUseInitiation(commontestutils.GetScheme(), ci, hco)

			req := commontestutils.NewReq(hco)
			Expect(handler.Ensure(req)).To(Succeed())

			cli.InitiateDeleteErrors(func(obj client.Object) error {
				if unstructed, ok := obj.(runtime.Unstructured); ok {
					kind := unstructed.GetObjectKind()
					if kind.GroupVersionKind().Kind == "NetworkAddonsConfig" {
						return fakeError
					}
				}
				return nil
			})

			expectedEvents := []commontestutils.MockEvent{
				{
					EventType: corev1.EventTypeWarning,
					Reason:    ErrHCOUninstall,
					Msg:       uninstallHCOErrorMsg,
				},
			}

			eventEmitter.Reset()
			err := handler.EnsureDeleted(req)
			Expect(err).To(Equal(fakeError))

			By("Check that event was emitted", func() {
				Expect(eventEmitter.CheckEvents(expectedEvents)).To(BeTrue())
			})

			By("make sure the CNA object still exists", func() {
				// Read back CNA
				cnaList := networkaddonsv1.NetworkAddonsConfigList{}
				Expect(cli.List(req.Ctx, &cnaList)).To(Succeed())
				Expect(cnaList).ToNot(BeNil())
				Expect(cnaList.Items).To(HaveLen(1))
				Expect(cnaList.Items[0].Name).To(Equal("cluster"))
			})
		})

		It("delete timeout error handling", func() {
			hco := commontestutils.NewHco()
			ci := commontestutils.ClusterInfoMock{}
			cli := commontestutils.InitClient([]client.Object{hcoNamespace, hco, ci.GetCSV()})

			eventEmitter := commontestutils.NewEventEmitterMock()

			handler := NewOperandHandler(cli, commontestutils.GetScheme(), ci, eventEmitter)
			handler.FirstUseInitiation(commontestutils.GetScheme(), ci, hco)

			req := commontestutils.NewReq(hco)
			Expect(handler.Ensure(req)).To(Succeed())

			cli.InitiateDeleteErrors(func(obj client.Object) error {
				if unstructed, ok := obj.(runtime.Unstructured); ok {
					kind := unstructed.GetObjectKind()
					if kind.GroupVersionKind().Kind == "NetworkAddonsConfig" {
						time.Sleep(time.Millisecond * 500)
					}
				}
				return nil
			})

			eventEmitter.Reset()
			ctx, cancelFunc := context.WithTimeout(req.Ctx, time.Millisecond*300)
			defer cancelFunc()
			req.Ctx = ctx
			err := handler.EnsureDeleted(req)
			Expect(err).To(MatchError("context deadline exceeded"))

			expectedEvents := []commontestutils.MockEvent{
				{
					EventType: corev1.EventTypeNormal,
					Reason:    "Killing",
					Msg:       "Removed NetworkAddonsConfig cluster",
				},
			}

			By("Check that event was *not* emitted", func() {
				Expect(eventEmitter.CheckEvents(expectedEvents)).To(BeFalse())
			})
		})
	})

	Context("test imageStream deletion", func() {
		It("should delete the ImageStream resource if the FG is not set, and emit event", func() {
			hcoNamespace := commontestutils.NewHcoNamespace()
			hco := commontestutils.NewHco()
			hco.Spec.EnableCommonBootImageImport = ptr.To(true)
			eventEmitter := commontestutils.NewEventEmitterMock()
			ci := commontestutils.ClusterInfoMock{}
			cli := commontestutils.InitClient([]client.Object{hcoNamespace, hco, ci.GetCSV()})

			handler := NewOperandHandler(cli, commontestutils.GetScheme(), ci, eventEmitter)
			handler.FirstUseInitiation(commontestutils.GetScheme(), ci, hco)

			req := commontestutils.NewReq(hco)
			Expect(handler.Ensure(req)).To(Succeed())

			ImageStreamObjects := &imagev1.ImageStreamList{}
			Expect(cli.List(context.TODO(), ImageStreamObjects)).To(Succeed())
			Expect(ImageStreamObjects.Items).To(HaveLen(1))
			Expect(ImageStreamObjects.Items[0].Name).To(Equal("test-image-stream"))

			objectRef, err := reference.GetReference(commontestutils.GetScheme(), &ImageStreamObjects.Items[0])
			Expect(err).ToNot(HaveOccurred())
			hco.Status.RelatedObjects = append(hco.Status.RelatedObjects, *objectRef)

			By("check related object - the imageStream ref should be there")
			existingRef, err := objectreferencesv1.FindObjectReference(hco.Status.RelatedObjects, *objectRef)
			Expect(err).ToNot(HaveOccurred())
			Expect(existingRef).ToNot(BeNil())

			By("Run again, this time when the FG is false")
			eventEmitter.Reset()
			hco.Spec.EnableCommonBootImageImport = ptr.To(false)
			req = commontestutils.NewReq(hco)
			Expect(handler.Ensure(req)).To(Succeed())

			By("check that the image stream was removed")
			ImageStreamObjects = &imagev1.ImageStreamList{}
			Expect(cli.List(context.TODO(), ImageStreamObjects)).To(Succeed())
			Expect(ImageStreamObjects.Items).To(BeEmpty())

			By("check that the delete event was emitted")
			expectedEvents := []commontestutils.MockEvent{
				{
					EventType: corev1.EventTypeNormal,
					Reason:    "Killing",
					Msg:       "Removed ImageStream test-image-stream",
				},
			}
			Expect(eventEmitter.CheckEvents(expectedEvents)).To(BeTrue())

			By("check that the related object was removed")
			existingRef, err = objectreferencesv1.FindObjectReference(hco.Status.RelatedObjects, *objectRef)
			Expect(err).ToNot(HaveOccurred())
			Expect(existingRef).To(BeNil())
		})

		It("should not emit event if the FG is not set and the image stream is not exist", func() {
			hcoNamespace := commontestutils.NewHcoNamespace()
			hco := commontestutils.NewHco()
			ci := commontestutils.ClusterInfoMock{}
			cli := commontestutils.InitClient([]client.Object{hcoNamespace, hco, ci.GetCSV()})

			eventEmitter := commontestutils.NewEventEmitterMock()

			handler := NewOperandHandler(cli, commontestutils.GetScheme(), ci, eventEmitter)
			handler.FirstUseInitiation(commontestutils.GetScheme(), ci, hco)

			req := commontestutils.NewReq(hco)
			Expect(handler.Ensure(req)).To(Succeed())

			expectedEvents := []commontestutils.MockEvent{
				{
					EventType: corev1.EventTypeNormal,
					Reason:    "Killing",
					Msg:       "Removed ImageStream test-image-stream",
				},
			}
			Expect(eventEmitter.CheckEvents(expectedEvents)).To(BeFalse())
		})

	})
})

func getTestFilesLocation() string {
	const (
		pkgDirectory = "controllers/operandhandler"
		testFilesLoc = "testFiles"
	)

	wd, err := os.Getwd()
	Expect(err).ToNot(HaveOccurred())
	if strings.HasSuffix(wd, pkgDirectory) {
		return testFilesLoc
	}
	return path.Join(pkgDirectory, testFilesLoc)
}
