package commontestutils

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

func CommonBeforeSuite() {
	GinkgoHelper()

	origNS, origNSSet := os.LookupEnv(hcoutil.OperatorNamespaceEnv)
	Expect(os.Setenv(hcoutil.OperatorNamespaceEnv, Namespace)).To(Succeed())

	DeferCleanup(func() {
		if origNSSet {
			Expect(os.Setenv(hcoutil.OperatorNamespaceEnv, origNS)).To(Succeed())
		} else {
			Expect(os.Unsetenv(hcoutil.OperatorNamespaceEnv)).To(Succeed())
		}
	})

}
