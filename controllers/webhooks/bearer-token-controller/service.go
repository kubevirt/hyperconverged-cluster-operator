package bearer_token_controller

import (
	"os"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/alerts"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

const (
	metricsSuffix = "-operator-metrics"
	serviceName   = hcoutil.HCOWebhookName + metricsSuffix
)

func newWHMetricServiceReconciler(namespace string, owner metav1.OwnerReference) *alerts.MetricServiceReconciler {
	return alerts.CreateMetricServiceReconciler(NewMetricsService(namespace, owner))
}

func NewMetricsService(namespace string, owner metav1.OwnerReference) *corev1.Service {
	servicePorts := []corev1.ServicePort{
		{
			Port:     hcoutil.MetricsPort,
			Name:     alerts.OperatorPortName,
			Protocol: corev1.ProtocolTCP,
			TargetPort: intstr.IntOrString{
				Type: intstr.Int, IntVal: hcoutil.MetricsPort,
			},
		},
	}

	webhookName := hcoutil.HCOWebhookName
	val, ok := os.LookupEnv(alerts.OperatorNameEnv)
	if ok && val != "" {
		webhookName = val
	}
	labelSelect := map[string]string{"name": webhookName}

	spec := corev1.ServiceSpec{
		Ports:    servicePorts,
		Selector: labelSelect,
	}

	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            serviceName,
			Labels:          getLabels(),
			Namespace:       namespace,
			OwnerReferences: []metav1.OwnerReference{owner},
		},
		Spec: spec,
	}
}
