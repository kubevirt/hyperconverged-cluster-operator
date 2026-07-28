package observabilitycontroller

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

var _ = Describe("Observability Controller ServiceAccount", func() {
	Context("newServiceAccount", func() {
		It("should have all default values", func() {
			sa := newServiceAccount()
			Expect(sa.Name).To(Equal(serviceAccountName))
			Expect(sa.Namespace).To(Equal(hcoutil.GetOperatorNamespaceFromEnv()))
			Expect(sa.Labels).To(HaveKeyWithValue(hcoutil.AppLabel, hcoutil.HyperConvergedName))
			Expect(sa.Labels).To(HaveKeyWithValue(hcoutil.AppLabelComponent, string(hcoutil.AppComponentObservability)))
		})
	})
})
