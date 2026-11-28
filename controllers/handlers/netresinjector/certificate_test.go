package netresinjector

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

var _ = Describe("Certificate Handler", func() {
	It("should return the network-resources-injector Certificate", func() {
		cl := commontestutils.InitClient(nil)
		handler := NewCertManagerCertHandler(cl, commontestutils.GetScheme())

		getter, ok := handler.(operands.CRGetter)
		Expect(ok).To(BeTrue())

		obj, err := getter.GetFullCr(commontestutils.NewHco())
		Expect(err).ToNot(HaveOccurred())

		cert, ok := obj.(*unstructured.Unstructured)
		Expect(ok).To(BeTrue())
		Expect(cert.GetName()).To(Equal(tlsCertificateName))
		Expect(cert.GetNamespace()).To(Equal(hcoutil.GetOperatorNamespaceFromEnv()))
		Expect(cert.GetLabels()).To(HaveKeyWithValue(hcoutil.AppLabelComponent, string(hcoutil.AppComponentNetResInjector)))

		secretName, _, _ := unstructured.NestedString(cert.Object, "spec", "secretName")
		Expect(secretName).To(Equal(tlsSecretName))
		dnsNames, _, _ := unstructured.NestedStringSlice(cert.Object, "spec", "dnsNames")
		Expect(dnsNames).To(ConsistOf(serviceName + "." + hcoutil.GetOperatorNamespaceFromEnv() + ".svc"))
	})
})
