package observabilitycontroller

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

func NewServiceAccountHandler(cli client.Client, scheme *runtime.Scheme) operands.Operand {
	return operands.NewConditionalHandler(
		operands.NewServiceAccountHandler(cli, scheme, newServiceAccount),
		shouldDeploy,
		func(hc *hcov1.HyperConverged) client.Object {
			return newServiceAccount()
		},
	)
}

func newServiceAccount() *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceAccountName,
			Namespace: hcoutil.GetOperatorNamespaceFromEnv(),
			Labels:    operands.GetLabels(hcoutil.AppComponentObservability),
		},
	}
}
