package handlers

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	csvv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	fakeownresources "github.com/kubevirt/hyperconverged-cluster-operator/pkg/ownresources/fake"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

var _ = Describe("CSV Operand", func() {
	var (
		hco *hcov1.HyperConverged
		req *common.HcoRequest
	)

	BeforeEach(func() {
		fakeownresources.OLMV0OwnResourcesMock()
		hco = commontestutils.NewHco()
		req = commontestutils.NewReq(hco)

		DeferCleanup(func() {
			fakeownresources.ResetOwnResources()
		})
	})

	Context("UninstallStrategy is missing", func() {
		It("should set console.openshift.io/disable-operand-delete to true", func() {
			foundResource := ensure(req, hco)
			Expect(foundResource.Annotations).To(HaveKeyWithValue(hcoutil.DisableOperandDeletionAnnotation, "true"))
		})
	})

	Context("UninstallStrategy is BlockUninstallIfWorkloadsExist", func() {
		It("should set console.openshift.io/disable-operand-delete to true", func() {
			hco.Spec.Deployment.UninstallStrategy = hcov1.HyperConvergedUninstallStrategyBlockUninstallIfWorkloadsExist
			foundResource := ensure(req, hco)
			Expect(foundResource.Annotations).To(HaveKeyWithValue(hcoutil.DisableOperandDeletionAnnotation, "true"))
		})

		It("should set console.openshift.io/disable-operand-delete to true on changing from RemoveWorkloads", func() {
			hco.Spec.Deployment.UninstallStrategy = hcov1.HyperConvergedUninstallStrategyRemoveWorkloads
			foundResource := ensure(req, hco)
			Expect(foundResource.Annotations).To(HaveKeyWithValue(hcoutil.DisableOperandDeletionAnnotation, "false"))

			hco.Spec.Deployment.UninstallStrategy = hcov1.HyperConvergedUninstallStrategyBlockUninstallIfWorkloadsExist
			foundResource = ensure(req, hco)
			Expect(foundResource.Annotations).To(HaveKeyWithValue(hcoutil.DisableOperandDeletionAnnotation, "true"))
		})
	})

	Context("UninstallStrategy is RemoveWorkloads", func() {
		It("should set console.openshift.io/disable-operand-delete to false", func() {
			hco.Spec.Deployment.UninstallStrategy = hcov1.HyperConvergedUninstallStrategyRemoveWorkloads
			foundResource := ensure(req, hco)
			Expect(foundResource.Annotations).To(HaveKeyWithValue(hcoutil.DisableOperandDeletionAnnotation, "false"))
		})

		It("should set console.openshift.io/disable-operand-delete to false on changing from BlockUninstallIfWorkloadsExist", func() {
			hco.Spec.Deployment.UninstallStrategy = hcov1.HyperConvergedUninstallStrategyBlockUninstallIfWorkloadsExist
			foundResource := ensure(req, hco)
			Expect(foundResource.Annotations).To(HaveKeyWithValue(hcoutil.DisableOperandDeletionAnnotation, "true"))

			hco.Spec.Deployment.UninstallStrategy = hcov1.HyperConvergedUninstallStrategyRemoveWorkloads
			foundResource = ensure(req, hco)
			Expect(foundResource.Annotations).To(HaveKeyWithValue(hcoutil.DisableOperandDeletionAnnotation, "false"))
		})
	})
})

func ensure(req *common.HcoRequest, hco *hcov1.HyperConverged) *csvv1alpha1.ClusterServiceVersion {
	GinkgoHelper()

	csv := commontestutils.GetCSV()

	cl := commontestutils.InitClient([]client.Object{hco, csv})
	handler := NewCsvHandler(cl)
	res := handler.Ensure(req)
	Expect(res.UpgradeDone).To(BeTrue())
	Expect(res.Err).ToNot(HaveOccurred())

	foundResource := &csvv1alpha1.ClusterServiceVersion{}
	Expect(
		cl.Get(context.TODO(),
			types.NamespacedName{Name: csv.Name, Namespace: csv.Namespace},
			foundResource),
	).To(Succeed())
	return foundResource
}
