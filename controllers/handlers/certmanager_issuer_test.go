package handlers

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

type issuerUpdateSpy struct {
	client.Client
	updateCalled bool
}

func (s *issuerUpdateSpy) Update(_ context.Context, _ client.Object, _ ...client.UpdateOption) error {
	s.updateCalled = true
	return nil
}

var _ = Describe("Cert-Manager Issuer Handler", func() {
	var hooks *issuerHooks

	BeforeEach(func() {
		hooks = &issuerHooks{}
	})

	Context("NewCertManagerIssuerHandler", func() {
		It("should create a handler that returns a valid Issuer via GetFullCr", func() {
			hco := commontestutils.NewHco()
			cl := &issuerUpdateSpy{}
			handler := NewCertManagerIssuerHandler(cl, commontestutils.GetScheme())

			getter, ok := handler.(operands.CRGetter)
			Expect(ok).To(BeTrue())

			obj, err := getter.GetFullCr(hco)
			Expect(err).ToNot(HaveOccurred())
			Expect(obj).ToNot(BeNil())

			issuer, ok := obj.(*unstructured.Unstructured)
			Expect(ok).To(BeTrue())
			Expect(issuer.GetAPIVersion()).To(Equal(certManagerAPIVersion))
			Expect(issuer.GetKind()).To(Equal(issuerKind))
			Expect(issuer.GetName()).To(Equal(issuerName))
			Expect(issuer.GetLabels()).ToNot(BeEmpty())
		})
	})

	Context("GetFullCr", func() {
		It("should return a valid Issuer with all required fields", func() {
			hco := commontestutils.NewHco()
			obj, err := hooks.GetFullCr(hco)

			Expect(err).ToNot(HaveOccurred())
			Expect(obj).ToNot(BeNil())

			issuer, ok := obj.(*unstructured.Unstructured)
			Expect(ok).To(BeTrue())

			Expect(issuer.GetAPIVersion()).To(Equal(certManagerAPIVersion))
			Expect(issuer.GetKind()).To(Equal(issuerKind))
			Expect(issuer.GetName()).To(Equal(issuerName))
			Expect(issuer.GetNamespace()).To(Equal(hcoutil.GetOperatorNamespaceFromEnv()))

			labels := issuer.GetLabels()
			expectedLabels := operands.GetLabels(hcoutil.AppComponentCompute)
			for k, v := range expectedLabels {
				Expect(labels).To(HaveKeyWithValue(k, v))
			}

			selfSigned, found, err := unstructured.NestedMap(issuer.Object, "spec", "selfSigned")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(selfSigned).ToNot(BeNil())
		})
	})

	Context("GetEmptyCr", func() {
		It("should return an empty Issuer with apiVersion and kind", func() {
			obj := hooks.GetEmptyCr()
			Expect(obj).ToNot(BeNil())

			issuer, ok := obj.(*unstructured.Unstructured)
			Expect(ok).To(BeTrue())

			Expect(issuer.GetAPIVersion()).To(Equal(certManagerAPIVersion))
			Expect(issuer.GetKind()).To(Equal(issuerKind))
			Expect(issuer.GetName()).To(BeEmpty())
			Expect(issuer.GetNamespace()).To(BeEmpty())
		})
	})

	Context("UpdateCR", func() {
		var (
			existing *unstructured.Unstructured
			desired  *unstructured.Unstructured
			req      *common.HcoRequest
			cl       *issuerUpdateSpy
		)

		BeforeEach(func() {
			hco := commontestutils.NewHco()
			req = commontestutils.NewReq(hco)
			cl = &issuerUpdateSpy{}

			existing = &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": certManagerAPIVersion,
					"kind":       issuerKind,
					"metadata": map[string]any{
						"name":      issuerName,
						"namespace": "test-namespace",
						"labels": map[string]any{
							"app": "kubevirt-hyperconverged",
						},
					},
					"spec": map[string]any{
						"selfSigned": map[string]any{},
					},
				},
			}

			desired = &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": certManagerAPIVersion,
					"kind":       issuerKind,
					"metadata": map[string]any{
						"name":      issuerName,
						"namespace": "test-namespace",
						"labels": map[string]any{
							"app": "kubevirt-hyperconverged",
						},
					},
					"spec": map[string]any{
						"selfSigned": map[string]any{},
					},
				},
			}
		})

		It("should return no update needed when labels match", func() {
			updated, overwritten, err := hooks.UpdateCR(req, cl, existing, desired)

			Expect(err).ToNot(HaveOccurred())
			Expect(updated).To(BeFalse())
			Expect(overwritten).To(BeFalse())
			Expect(cl.updateCalled).To(BeFalse())
		})

		It("should detect and update label changes", func() {
			_ = unstructured.SetNestedStringMap(desired.Object, map[string]string{
				"app":                          "kubevirt-hyperconverged",
				"app.kubernetes.io/component":  "compute",
				"app.kubernetes.io/managed-by": "hco-operator",
			}, "metadata", "labels")

			updated, overwritten, err := hooks.UpdateCR(req, cl, existing, desired)

			Expect(err).ToNot(HaveOccurred())
			Expect(updated).To(BeTrue())
			Expect(overwritten).To(BeFalse())
			Expect(cl.updateCalled).To(BeTrue())

			updatedLabels, _, _ := unstructured.NestedStringMap(existing.Object, "metadata", "labels")
			Expect(updatedLabels).To(HaveKeyWithValue("app.kubernetes.io/component", "compute"))
			Expect(updatedLabels).To(HaveKeyWithValue("app.kubernetes.io/managed-by", "hco-operator"))
		})

		It("should report overwrite when not HCO-triggered", func() {
			req.HCOTriggered = false
			_ = unstructured.SetNestedStringMap(desired.Object, map[string]string{
				"app":     "kubevirt-hyperconverged",
				"version": "v1",
			}, "metadata", "labels")

			updated, overwritten, err := hooks.UpdateCR(req, cl, existing, desired)

			Expect(err).ToNot(HaveOccurred())
			Expect(updated).To(BeTrue())
			Expect(overwritten).To(BeTrue())
			Expect(cl.updateCalled).To(BeTrue())
		})

		It("should handle nil existing object gracefully", func() {
			updated, overwritten, err := hooks.UpdateCR(nil, nil, nil, desired)

			Expect(err).ToNot(HaveOccurred())
			Expect(updated).To(BeFalse())
			Expect(overwritten).To(BeFalse())
		})

		It("should handle nil desired object gracefully", func() {
			updated, overwritten, err := hooks.UpdateCR(nil, nil, existing, nil)

			Expect(err).ToNot(HaveOccurred())
			Expect(updated).To(BeFalse())
			Expect(overwritten).To(BeFalse())
		})

		It("should handle wrong type for existing object gracefully", func() {
			wrongType := &unstructured.UnstructuredList{}
			updated, overwritten, err := hooks.UpdateCR(nil, nil, wrongType, desired)

			Expect(err).ToNot(HaveOccurred())
			Expect(updated).To(BeFalse())
			Expect(overwritten).To(BeFalse())
		})

		It("should handle wrong type for desired object gracefully", func() {
			wrongType := &unstructured.UnstructuredList{}
			updated, overwritten, err := hooks.UpdateCR(nil, nil, existing, wrongType)

			Expect(err).ToNot(HaveOccurred())
			Expect(updated).To(BeFalse())
			Expect(overwritten).To(BeFalse())
		})
	})
})
