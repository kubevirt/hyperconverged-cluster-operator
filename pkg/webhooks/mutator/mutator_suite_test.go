package mutator

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"

	networkaddonsv1 "github.com/kubevirt/cluster-network-addons-operator/pkg/apis/networkaddonsoperator/v1"
	kubevirtcorev1 "kubevirt.io/api/core/v1"
	cdiv1beta1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	sspv1beta3 "kubevirt.io/ssp-operator/api/v1beta3"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
)

var (
	mutatorScheme = scheme.Scheme
	codecFactory  serializer.CodecFactory
	testCodec     runtime.Codec
)

func TestMutatorWebhook(t *testing.T) {
	RegisterFailHandler(Fail)

	BeforeSuite(func() {
		for _, f := range []func(*runtime.Scheme) error{
			hcov1beta1.AddToScheme,
			cdiv1beta1.AddToScheme,
			kubevirtcorev1.AddToScheme,
			networkaddonsv1.AddToScheme,
			sspv1beta3.AddToScheme,
			corev1.AddToScheme,
		} {
			Expect(f(mutatorScheme)).To(Succeed())
		}

		codecFactory = serializer.NewCodecFactory(mutatorScheme)
		testCodec = codecFactory.LegacyCodec(corev1.SchemeGroupVersion, hcov1beta1.SchemeGroupVersion)
	})

	RunSpecs(t, "Mutator Webhooks Suite")
}
