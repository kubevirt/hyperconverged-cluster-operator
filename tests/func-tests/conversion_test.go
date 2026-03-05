package tests_test

import (
	"context"

	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
	tests "github.com/kubevirt/hyperconverged-cluster-operator/tests/func-tests"
)

var _ = Describe("check v1 <=> v1beta1 API conversion", func() {
	It("naively read HCO in v1 format", func(ctx context.Context) {
		hcv1 := &hcov1.HyperConverged{
			ObjectMeta: metav1.ObjectMeta{
				Name:      hcoutil.HyperConvergedName,
				Namespace: tests.InstallNamespace,
			},
		}

		cli := tests.GetControllerRuntimeClient()

		Expect(cli.Get(ctx, client.ObjectKeyFromObject(hcv1), hcv1)).To(Succeed())

		hcv1beta1 := tests.GetHCO(ctx, cli)
		converted := &hcov1.HyperConverged{}
		Expect(hcv1beta1.ConvertTo(converted)).To(Succeed())

		diff := cmp.Diff(hcv1, converted)
		if diff != "" {
			GinkgoWriter.Println(diff)
			Fail("v1 HyperConverged should be equal to the v1beta1 converted one")
		}
	})
})
