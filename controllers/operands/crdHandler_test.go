package operands

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

func newTestCRD() *apiextensionsv1.CustomResourceDefinition {
	return &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "tests.example.kubevirt.io",
			Labels: GetLabels(hcoutil.AppComponentDeployment),
		},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: "example.kubevirt.io",
			Names: apiextensionsv1.CustomResourceDefinitionNames{Kind: "Test", Plural: "tests"},
			Scope: apiextensionsv1.NamespaceScoped,
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
				{
					Name:    "v1",
					Served:  true,
					Storage: true,
					Schema: &apiextensionsv1.CustomResourceValidation{
						OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{Type: "object"},
					},
				},
			},
		},
	}
}

var _ = Describe("CRD Handler", func() {
	var (
		hco *hcov1.HyperConverged
		req *common.HcoRequest
	)

	BeforeEach(func() {
		hco = commontestutils.NewHco()
		req = commontestutils.NewReq(hco)
	})

	getCRD := func(cl client.Client) *apiextensionsv1.CustomResourceDefinition {
		found := &apiextensionsv1.CustomResourceDefinition{}
		Expect(cl.Get(context.Background(), client.ObjectKeyFromObject(newTestCRD()), found)).To(Succeed())
		return found
	}

	It("should create the CRD", func() {
		cl := commontestutils.InitClient([]client.Object{hco})
		res := NewCRDHandler(cl, commontestutils.GetScheme(), newTestCRD()).Ensure(req)

		Expect(res.Err).ToNot(HaveOccurred())
		Expect(res.Created).To(BeTrue())
		Expect(getCRD(cl).Spec.Group).To(Equal("example.kubevirt.io"))
	})

	It("should not update for fields the apiserver defaults", func() {
		existing := newTestCRD()
		existing.Spec.Names.Singular = "test"
		existing.Spec.Names.ListKind = "TestList"
		existing.Spec.Conversion = &apiextensionsv1.CustomResourceConversion{Strategy: apiextensionsv1.NoneConverter}
		cl := commontestutils.InitClient([]client.Object{hco, existing})

		res := NewCRDHandler(cl, commontestutils.GetScheme(), newTestCRD()).Ensure(req)

		Expect(res.Err).ToNot(HaveOccurred())
		Expect(res.Updated).To(BeFalse())
	})

	It("should update a modified spec", func() {
		existing := newTestCRD()
		existing.Spec.Versions[0].Served = false
		cl := commontestutils.InitClient([]client.Object{hco, existing})

		req.HCOTriggered = false
		res := NewCRDHandler(cl, commontestutils.GetScheme(), newTestCRD()).Ensure(req)

		Expect(res.Err).ToNot(HaveOccurred())
		Expect(res.Updated).To(BeTrue())
		Expect(res.Overwritten).To(BeTrue())
		Expect(getCRD(cl).Spec.Versions[0].Served).To(BeTrue())
	})

	It("should restore missing labels and keep others", func() {
		existing := newTestCRD()
		existing.Labels = map[string]string{"user-label": "value"}
		cl := commontestutils.InitClient([]client.Object{hco, existing})

		res := NewCRDHandler(cl, commontestutils.GetScheme(), newTestCRD()).Ensure(req)

		Expect(res.Err).ToNot(HaveOccurred())
		Expect(res.Updated).To(BeTrue())
		labels := getCRD(cl).Labels
		Expect(labels).To(HaveKeyWithValue("user-label", "value"))
		Expect(labels).To(HaveKeyWithValue(hcoutil.AppLabel, hcoutil.HyperConvergedName))
	})

	It("should revert an added version", func() {
		existing := newTestCRD()
		existing.Spec.Versions = append(existing.Spec.Versions, apiextensionsv1.CustomResourceDefinitionVersion{
			Name: "v2", Served: true,
			Schema: &apiextensionsv1.CustomResourceValidation{OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{Type: "object"}},
		})
		cl := commontestutils.InitClient([]client.Object{hco, existing})

		res := NewCRDHandler(cl, commontestutils.GetScheme(), newTestCRD()).Ensure(req)

		Expect(res.Err).ToNot(HaveOccurred())
		Expect(res.Updated).To(BeTrue())
		Expect(getCRD(cl).Spec.Versions).To(HaveLen(1))
	})

	It("should wait without an error while the CRD is being deleted", func() {
		existing := newTestCRD()
		existing.Finalizers = []string{"test.kubevirt.io/finalizer"}
		existing.Spec.Versions[0].Served = false
		cl := commontestutils.InitClient([]client.Object{hco, existing})
		Expect(cl.Delete(context.Background(), existing)).To(Succeed())

		res := NewCRDHandler(cl, commontestutils.GetScheme(), newTestCRD()).Ensure(req)

		Expect(res.Err).ToNot(HaveOccurred())
		Expect(res.Updated).To(BeFalse())
		Expect(getCRD(cl).DeletionTimestamp).ToNot(BeNil())
	})
})
