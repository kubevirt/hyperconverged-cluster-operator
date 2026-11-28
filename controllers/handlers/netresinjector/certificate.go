package netresinjector

import (
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

func NewCertManagerCertHandler(cli client.Client, scheme *runtime.Scheme) operands.Operand {
	dnsName := serviceName + "." + hcoutil.GetOperatorNamespaceFromEnv() + ".svc"
	return operands.NewCertificateHandler(cli, scheme,
		operands.NewCertificate(tlsCertificateName, tlsSecretName, []string{dnsName}, hcoutil.AppComponentNetResInjector))
}
