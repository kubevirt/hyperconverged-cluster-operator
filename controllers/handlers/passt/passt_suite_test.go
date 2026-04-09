package passt_test

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

func TestPasst(t *testing.T) {
	origNS, ok := os.LookupEnv(hcoutil.OperatorNamespaceEnv)
	_ = os.Setenv(hcoutil.OperatorNamespaceEnv, commontestutils.Namespace)

	defer func() {
		if ok {
			_ = os.Setenv(hcoutil.OperatorNamespaceEnv, origNS)
		} else {
			_ = os.Unsetenv(hcoutil.OperatorNamespaceEnv)
		}
	}()

	RegisterFailHandler(Fail)
	RunSpecs(t, "Passt Suite")
}
