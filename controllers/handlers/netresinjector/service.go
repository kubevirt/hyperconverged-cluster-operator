package netresinjector

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

func NewServiceHandler(cli client.Client, scheme *runtime.Scheme) operands.Operand {
	return operands.NewServiceHandler(cli, scheme, newService())
}

func newService() *corev1.Service {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: hcoutil.GetOperatorNamespaceFromEnv(),
			Labels:    operands.GetLabels(hcoutil.AppComponentNetResInjector),
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Port:       443,
					TargetPort: intstr.FromInt32(6443),
					Protocol:   corev1.ProtocolTCP,
				},
			},
			Selector: map[string]string{
				hcoutil.AppLabel:          hcoutil.HyperConvergedName,
				hcoutil.AppLabelComponent: string(hcoutil.AppComponentNetResInjector),
			},
		},
	}

	if hcoutil.GetClusterInfo().IsOpenshift() {
		svc.Annotations = map[string]string{
			"service.beta.openshift.io/serving-cert-secret-name": tlsSecretName,
		}
	}

	return svc
}
