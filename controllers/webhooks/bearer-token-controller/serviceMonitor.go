package bearer_token_controller

import (
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/alerts"
)

func newWHServiceMonitorReconciler(namespace string, owner metav1.OwnerReference, refresher alerts.Refresher) *alerts.ServiceMonitorReconciler {
	return alerts.CreateServiceMonitorReconciler(newServiceMonitor(namespace, owner), refresher)
}

func newServiceMonitor(namespace string, owner metav1.OwnerReference) *monitoringv1.ServiceMonitor {
	smLabels := getLabels()
	spec := monitoringv1.ServiceMonitorSpec{
		Selector: metav1.LabelSelector{
			MatchLabels: smLabels,
		},
		Endpoints: []monitoringv1.Endpoint{
			alerts.CreateEndpoint(secretName),
		},
	}

	return &monitoringv1.ServiceMonitor{
		TypeMeta: metav1.TypeMeta{
			APIVersion: monitoringv1.SchemeGroupVersion.String(),
			Kind:       monitoringv1.ServiceMonitorsKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            serviceName,
			Labels:          smLabels,
			Namespace:       namespace,
			OwnerReferences: []metav1.OwnerReference{owner},
		},
		Spec: spec,
	}
}
