package v1beta1

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
)

func TestV1Beta1(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "v1beta1 Suite")
}

var _ = Describe("api/v1beta1", func() {
	Context("Conversion", func() {
		It("should allow v1beta1 => v1 conversion", func() {
			v1beta1hco := getV1Beta1HC()

			Expect(v1beta1hco.ConvertTo(&hcov1.HyperConverged{})).To(Succeed())
		})

		It("should allow v1 => v1beta1 conversion", func() {
			v1hco := getV1HC()

			Expect((&HyperConverged{}).ConvertFrom(v1hco)).To(Succeed())
		})
	})
})

func getV1HC() *hcov1.HyperConverged {
	GinkgoHelper()

	defaultScheme := runtime.NewScheme()
	Expect(hcov1.AddToScheme(defaultScheme)).To(Succeed())
	Expect(hcov1.RegisterDefaults(defaultScheme)).To(Succeed())
	defaultHco := &hcov1.HyperConverged{
		TypeMeta: metav1.TypeMeta{
			APIVersion: hcov1.APIVersion,
			Kind:       "HyperConverged",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubevirt-hyperconverged",
			Namespace: "kubevirt-hyperconverged",
		}}
	defaultScheme.Default(defaultHco)
	return defaultHco
}

func getV1Beta1HC() *HyperConverged {
	GinkgoHelper()

	defaultScheme := runtime.NewScheme()
	Expect(AddToScheme(defaultScheme)).To(Succeed())
	Expect(RegisterDefaults(defaultScheme)).To(Succeed())
	defaultHco := &HyperConverged{
		TypeMeta: metav1.TypeMeta{
			APIVersion: APIVersion,
			Kind:       "HyperConverged",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubevirt-hyperconverged",
			Namespace: "kubevirt-hyperconverged",
		}}
	defaultScheme.Default(defaultHco)
	return defaultHco
}
