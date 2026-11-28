package operandhandler

import (
	"context"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/dirtest"
	fakeownresources "github.com/kubevirt/hyperconverged-cluster-operator/pkg/ownresources/fake"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
	bundle "github.com/kubevirt/hyperconverged-cluster-operator/pkg/virtimport"
)

var _ = Describe("Test operandHandler with virt-import", func() {
	var (
		hco            *hcov1.HyperConverged
		ci             commontestutils.ClusterInfoMock
		eventEmitter   *commontestutils.EventEmitterMock
		vapName        string
		deploymentName string
		hcoTestObjects []client.Object
	)

	restoreEnv := func(name string) {
		orig, wasSet := os.LookupEnv(name)
		DeferCleanup(func() {
			if wasSet {
				Expect(os.Setenv(name, orig)).To(Succeed())
			} else {
				Expect(os.Unsetenv(name)).To(Succeed())
			}
		})
	}

	setEnv := func(name, value string) {
		restoreEnv(name)
		Expect(os.Setenv(name, value)).To(Succeed())
	}

	unsetEnv := func(name string) {
		restoreEnv(name)
		Expect(os.Unsetenv(name)).To(Succeed())
	}

	BeforeEach(func() {
		fakeownresources.OLMV0OwnResourcesMock()
		origNS := os.Getenv(hcoutil.OperatorNamespaceEnv)
		Expect(os.Setenv(hcoutil.OperatorNamespaceEnv, commontestutils.Namespace)).To(Succeed())
		DeferCleanup(func() {
			fakeownresources.ResetOwnResources()
			Expect(os.Setenv(hcoutil.OperatorNamespaceEnv, origNS)).To(Succeed())
		})

		hco = commontestutils.NewHco()
		hco.Spec.FeatureGates.Enable("deployVirtImport")
		ci = commontestutils.ClusterInfoMock{}
		eventEmitter = commontestutils.NewEventEmitterMock()
		hcoTestObjects = []client.Object{commontestutils.NewHcoNamespace(), hco, commontestutils.GetCSV()}

		b, err := bundle.GetBundle()
		Expect(err).ToNot(HaveOccurred())
		vapName = b.ValidatingAdmissionPolicies[0].Name
		deploymentName = b.Deployment.Name
	})

	getVAP := func(cli client.Client) error {
		return cli.Get(context.Background(), client.ObjectKey{Name: vapName}, &admissionregistrationv1.ValidatingAdmissionPolicy{})
	}

	getDeployment := func(cli client.Client) error {
		return cli.Get(context.Background(), client.ObjectKey{Name: deploymentName, Namespace: commontestutils.Namespace}, &appsv1.Deployment{})
	}

	DescribeTable("should not register the virt-import handlers without both image env vars",
		func(controllerImage, fetcherImage string) {
			for name, value := range map[string]string{
				hcoutil.VirtImportControllerImageEnvV: controllerImage,
				hcoutil.VirtImportFetcherImageEnvV:    fetcherImage,
			} {
				if value == "" {
					unsetEnv(name)
				} else {
					setEnv(name, value)
				}
			}

			cli := commontestutils.InitClient(hcoTestObjects)
			handler := NewOperandHandler(cli, commontestutils.GetScheme(), ci, eventEmitter)

			Expect(handler.Ensure(commontestutils.NewReq(hco))).To(Succeed())
			Expect(getVAP(cli)).To(MatchError(apierrors.IsNotFound, "not found error"))
			Expect(getDeployment(cli)).To(MatchError(apierrors.IsNotFound, "not found error"))
		},
		Entry("none set", "", ""),
		Entry("only the controller image set", "quay.io/kubevirt/virt-import-controller@sha256:1111", ""),
		Entry("only the fetcher image set", "", "quay.io/kubevirt/virt-import-fetcher@sha256:2222"),
	)

	Context("with the image env vars", func() {
		BeforeEach(func() {
			setEnv(hcoutil.VirtImportControllerImageEnvV, "quay.io/kubevirt/virt-import-controller@sha256:1111")
			setEnv(hcoutil.VirtImportFetcherImageEnvV, "quay.io/kubevirt/virt-import-fetcher@sha256:2222")
		})

		It("should deploy virt-import and remove it on HC deletion", func() {
			cli := commontestutils.InitClient(hcoTestObjects)
			handler := NewOperandHandler(cli, commontestutils.GetScheme(), ci, eventEmitter)
			handler.FirstUseInitiation(commontestutils.GetScheme(), ci, hco, dirtest.New())

			req := commontestutils.NewReq(hco)
			Expect(handler.Ensure(req)).To(Succeed())
			Expect(getVAP(cli)).To(Succeed())
			Expect(getDeployment(cli)).To(Succeed())

			Expect(handler.EnsureDeleted(req)).To(Succeed())
			Expect(getVAP(cli)).To(MatchError(apierrors.IsNotFound, "not found error"))
			Expect(getDeployment(cli)).To(MatchError(apierrors.IsNotFound, "not found error"))
		})
	})
})
