package aie_webhook

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

func NewAIEWebhookServiceHandler(Client client.Client, Scheme *runtime.Scheme) operands.Operand {
	return operands.NewConditionalHandler(
		operands.NewServiceHandler(Client, Scheme, newAIEWebhookService),
		shouldDeployAIEWebhook,
		func(hc *hcov1beta1.HyperConverged) client.Object {
			return newAIEWebhookServiceWithNameOnly(hc)
		},
	)
}

func newAIEWebhookServiceWithNameOnly(hc *hcov1beta1.HyperConverged) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      aieWebhookName,
			Namespace: hc.Namespace,
			Labels:    operands.GetLabels(hc, appComponent),
		},
	}
}

func newAIEWebhookService(hc *hcov1beta1.HyperConverged) *corev1.Service {
	svc := newAIEWebhookServiceWithNameOnly(hc)
	svc.Spec = corev1.ServiceSpec{
		Ports: []corev1.ServicePort{
			{
				Name:       "https",
				Port:       443,
				TargetPort: intstr.FromInt32(9443),
				Protocol:   corev1.ProtocolTCP,
			},
		},
		Selector: map[string]string{
			hcoutil.AppLabel:          hcoutil.HyperConvergedName,
			hcoutil.AppLabelComponent: string(appComponent),
		},
	}
	return svc
}
