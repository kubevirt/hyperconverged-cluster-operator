package bearer_token_controller

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

func TestWebhookBearerToken(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Webhook Bearer Token Controller Suite")
}

var _ = BeforeSuite(func() {
	origNS, origNSSet := os.LookupEnv(hcoutil.OperatorNamespaceEnv)
	_ = os.Setenv(hcoutil.OperatorNamespaceEnv, commontestutils.Namespace)

	DeferCleanup(func() {
		if origNSSet {
			_ = os.Setenv(hcoutil.OperatorNamespaceEnv, origNS)
		} else {
			_ = os.Unsetenv(hcoutil.OperatorNamespaceEnv)
		}
	})
})
