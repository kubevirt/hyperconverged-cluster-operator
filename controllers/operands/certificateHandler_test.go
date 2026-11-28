package operands

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

type updateSpy struct {
	client.Client
}

func (s *updateSpy) Update(_ context.Context, _ client.Object, _ ...client.UpdateOption) error {
	return nil
}

var _ = Describe("Certificate handler", func() {
	var hooks *certHooks

	BeforeEach(func() {
		hooks = &certHooks{}
	})

	Context("NewCertificate", func() {
		It("should return a valid Certificate with all required fields", func() {
			cert := NewCertificate("test-cert", "test-secret", []string{"test.svc"}, hcoutil.AppComponentNetResInjector)

			Expect(cert.GetAPIVersion()).To(Equal(certManagerAPIVersion))
			Expect(cert.GetKind()).To(Equal(certificateKind))
			Expect(cert.GetName()).To(Equal("test-cert"))
			Expect(cert.GetNamespace()).To(Equal(hcoutil.GetOperatorNamespaceFromEnv()))
			Expect(cert.GetLabels()).To(HaveKeyWithValue(hcoutil.AppLabelComponent, string(hcoutil.AppComponentNetResInjector)))

			secretName, _, _ := unstructured.NestedString(cert.Object, "spec", "secretName")
			Expect(secretName).To(Equal("test-secret"))
			dnsNames, _, _ := unstructured.NestedStringSlice(cert.Object, "spec", "dnsNames")
			Expect(dnsNames).To(Equal([]string{"test.svc"}))
			issuerRef, _, _ := unstructured.NestedMap(cert.Object, "spec", "issuerRef")
			Expect(issuerRef["name"]).To(Equal("selfsigned"))
		})
	})

	Context("GetEmptyCr", func() {
		It("should return an empty Certificate with apiVersion and kind", func() {
			obj := hooks.GetEmptyCr()
			Expect(obj).ToNot(BeNil())

			cert, ok := obj.(*unstructured.Unstructured)
			Expect(ok).To(BeTrue())

			Expect(cert.GetAPIVersion()).To(Equal(certManagerAPIVersion))
			Expect(cert.GetKind()).To(Equal(certificateKind))
			Expect(cert.GetName()).To(BeEmpty())
			Expect(cert.GetNamespace()).To(BeEmpty())
		})
	})

	Context("UpdateCR", func() {
		var (
			existing *unstructured.Unstructured
			desired  *unstructured.Unstructured
			req      *common.HcoRequest
			cl       client.Client
		)

		BeforeEach(func() {
			ns := "test-namespace"
			hco := commontestutils.NewHco()
			req = commontestutils.NewReq(hco)
			cl = &updateSpy{}

			existing = &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": certManagerAPIVersion,
					"kind":       certificateKind,
					"metadata": map[string]any{
						"name":      "test-cert",
						"namespace": ns,
						"labels": map[string]any{
							"app": "test",
						},
					},
					"spec": map[string]any{
						"secretName": "test-secret",
						"dnsNames":   []any{"test." + ns + ".svc"},
						"issuerRef":  map[string]any{"name": "selfsigned"},
					},
				},
			}

			desired = &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": certManagerAPIVersion,
					"kind":       certificateKind,
					"metadata": map[string]any{
						"name":      "test-cert",
						"namespace": ns,
						"labels": map[string]any{
							"app": "test",
						},
					},
					"spec": map[string]any{
						"secretName": "test-secret",
						"dnsNames":   []any{"test." + ns + ".svc"},
						"issuerRef":  map[string]any{"name": "selfsigned"},
					},
				},
			}
		})

		It("should return no update needed when everything matches", func() {
			needsUpdate, overwritten, err := hooks.UpdateCR(req, cl, existing, desired)

			Expect(err).ToNot(HaveOccurred())
			Expect(needsUpdate).To(BeFalse())
			Expect(overwritten).To(BeFalse())
		})

		It("should detect and update label changes", func() {
			_ = unstructured.SetNestedStringMap(desired.Object, map[string]string{
				"app":     "test",
				"version": "v1",
			}, "metadata", "labels")

			needsUpdate, overwritten, err := hooks.UpdateCR(req, cl, existing, desired)

			Expect(err).ToNot(HaveOccurred())
			Expect(needsUpdate).To(BeTrue())
			Expect(overwritten).To(BeFalse())

			updatedLabels, _, _ := unstructured.NestedStringMap(existing.Object, "metadata", "labels")
			Expect(updatedLabels).To(HaveKeyWithValue("version", "v1"))
		})

		It("should detect and update dnsNames changes", func() {
			_ = unstructured.SetNestedStringSlice(desired.Object, []string{
				"new-service.test-namespace.svc",
				"another-service.test-namespace.svc",
			}, "spec", "dnsNames")

			needsUpdate, overwritten, err := hooks.UpdateCR(req, cl, existing, desired)

			Expect(err).ToNot(HaveOccurred())
			Expect(needsUpdate).To(BeTrue())
			Expect(overwritten).To(BeFalse())

			updatedDNS, _, _ := unstructured.NestedStringSlice(existing.Object, "spec", "dnsNames")
			Expect(updatedDNS).To(HaveLen(2))
			Expect(updatedDNS).To(ContainElement("new-service.test-namespace.svc"))
		})

		It("should detect and update issuerRef changes", func() {
			_ = unstructured.SetNestedMap(desired.Object, map[string]any{
				"name": "ca-issuer",
				"kind": "ClusterIssuer",
			}, "spec", "issuerRef")

			needsUpdate, overwritten, err := hooks.UpdateCR(req, cl, existing, desired)

			Expect(err).ToNot(HaveOccurred())
			Expect(needsUpdate).To(BeTrue())
			Expect(overwritten).To(BeFalse())

			updatedIssuerRef, _, _ := unstructured.NestedMap(existing.Object, "spec", "issuerRef")
			Expect(updatedIssuerRef["name"]).To(Equal("ca-issuer"))
			Expect(updatedIssuerRef["kind"]).To(Equal("ClusterIssuer"))
		})

		It("should detect and update secretName changes", func() {
			_ = unstructured.SetNestedField(desired.Object, "new-secret-name", "spec", "secretName")

			needsUpdate, overwritten, err := hooks.UpdateCR(req, cl, existing, desired)

			Expect(err).ToNot(HaveOccurred())
			Expect(needsUpdate).To(BeTrue())
			Expect(overwritten).To(BeFalse())

			updatedSecretName, _, _ := unstructured.NestedString(existing.Object, "spec", "secretName")
			Expect(updatedSecretName).To(Equal("new-secret-name"))
		})

		It("should detect multiple changes at once", func() {
			_ = unstructured.SetNestedStringMap(desired.Object, map[string]string{
				"new-label": "value",
			}, "metadata", "labels")
			_ = unstructured.SetNestedField(desired.Object, "updated-secret", "spec", "secretName")
			_ = unstructured.SetNestedMap(desired.Object, map[string]any{
				"name": "new-issuer",
			}, "spec", "issuerRef")

			needsUpdate, overwritten, err := hooks.UpdateCR(req, cl, existing, desired)

			Expect(err).ToNot(HaveOccurred())
			Expect(needsUpdate).To(BeTrue())
			Expect(overwritten).To(BeFalse())

			updatedLabels, _, _ := unstructured.NestedStringMap(existing.Object, "metadata", "labels")
			Expect(updatedLabels).To(HaveKeyWithValue("new-label", "value"))

			updatedSecretName, _, _ := unstructured.NestedString(existing.Object, "spec", "secretName")
			Expect(updatedSecretName).To(Equal("updated-secret"))

			updatedIssuerRef, _, _ := unstructured.NestedMap(existing.Object, "spec", "issuerRef")
			Expect(updatedIssuerRef["name"]).To(Equal("new-issuer"))
		})

		It("should handle nil existing object gracefully", func() {
			needsUpdate, overwritten, err := hooks.UpdateCR(nil, nil, nil, desired)

			Expect(err).ToNot(HaveOccurred())
			Expect(needsUpdate).To(BeFalse())
			Expect(overwritten).To(BeFalse())
		})

		It("should handle nil desired object gracefully", func() {
			needsUpdate, overwritten, err := hooks.UpdateCR(nil, nil, existing, nil)

			Expect(err).ToNot(HaveOccurred())
			Expect(needsUpdate).To(BeFalse())
			Expect(overwritten).To(BeFalse())
		})

		It("should handle wrong type for existing object gracefully", func() {
			wrongType := &unstructured.UnstructuredList{}
			needsUpdate, overwritten, err := hooks.UpdateCR(nil, nil, wrongType, desired)

			Expect(err).ToNot(HaveOccurred())
			Expect(needsUpdate).To(BeFalse())
			Expect(overwritten).To(BeFalse())
		})

		It("should handle wrong type for desired object gracefully", func() {
			wrongType := &unstructured.UnstructuredList{}
			needsUpdate, overwritten, err := hooks.UpdateCR(nil, nil, existing, wrongType)

			Expect(err).ToNot(HaveOccurred())
			Expect(needsUpdate).To(BeFalse())
			Expect(overwritten).To(BeFalse())
		})
	})
})
