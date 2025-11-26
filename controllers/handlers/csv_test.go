package handlers

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	csvv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/components"
	fakeownresources "github.com/kubevirt/hyperconverged-cluster-operator/pkg/ownresources/fake"
)

var _ = Describe("CSV Operand", func() {
	var (
		hco *hcov1beta1.HyperConverged
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
			Expect(foundResource.Annotations).To(HaveKeyWithValue(components.DisableOperandDeletionAnnotation, "true"))
		})
	})

	Context("UninstallStrategy is BlockUninstallIfWorkloadsExist", func() {
		It("should set console.openshift.io/disable-operand-delete to true", func() {
			hco.Spec.UninstallStrategy = hcov1beta1.HyperConvergedUninstallStrategyBlockUninstallIfWorkloadsExist
			foundResource := ensure(req, hco)
			Expect(foundResource.Annotations).To(HaveKeyWithValue(components.DisableOperandDeletionAnnotation, "true"))
		})

		It("should set console.openshift.io/disable-operand-delete to true on changing from RemoveWorkloads", func() {
			hco.Spec.UninstallStrategy = hcov1beta1.HyperConvergedUninstallStrategyRemoveWorkloads
			foundResource := ensure(req, hco)
			Expect(foundResource.Annotations).To(HaveKeyWithValue(components.DisableOperandDeletionAnnotation, "false"))

			hco.Spec.UninstallStrategy = hcov1beta1.HyperConvergedUninstallStrategyBlockUninstallIfWorkloadsExist
			foundResource = ensure(req, hco)
			Expect(foundResource.Annotations).To(HaveKeyWithValue(components.DisableOperandDeletionAnnotation, "true"))
		})
	})

	Context("UninstallStrategy is RemoveWorkloads", func() {
		It("should set console.openshift.io/disable-operand-delete to false", func() {
			hco.Spec.UninstallStrategy = hcov1beta1.HyperConvergedUninstallStrategyRemoveWorkloads
			foundResource := ensure(req, hco)
			Expect(foundResource.Annotations).To(HaveKeyWithValue(components.DisableOperandDeletionAnnotation, "false"))
		})

		It("should set console.openshift.io/disable-operand-delete to false on changing from BlockUninstallIfWorkloadsExist", func() {
			hco.Spec.UninstallStrategy = hcov1beta1.HyperConvergedUninstallStrategyBlockUninstallIfWorkloadsExist
			foundResource := ensure(req, hco)
			Expect(foundResource.Annotations).To(HaveKeyWithValue(components.DisableOperandDeletionAnnotation, "true"))

			hco.Spec.UninstallStrategy = hcov1beta1.HyperConvergedUninstallStrategyRemoveWorkloads
			foundResource = ensure(req, hco)
			Expect(foundResource.Annotations).To(HaveKeyWithValue(components.DisableOperandDeletionAnnotation, "false"))
		})
	})
})

func ensure(req *common.HcoRequest, hco *hcov1beta1.HyperConverged) *csvv1alpha1.ClusterServiceVersion {
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
