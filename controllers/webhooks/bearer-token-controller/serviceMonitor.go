package bearer_token_controller

import (
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/alerts"
)

func newWHServiceMonitorReconciler(namespace string, owner metav1.OwnerReference) *alerts.ServiceMonitorReconciler {
	return alerts.CreateServiceMonitorReconciler(newServiceMonitor(namespace, owner))
}

func newServiceMonitor(namespace string, owner metav1.OwnerReference) *monitoringv1.ServiceMonitor {
	smLabels := getLabels()
	spec := monitoringv1.ServiceMonitorSpec{
		Selector: metav1.LabelSelector{
			MatchLabels: smLabels,
		},
		Endpoints: []monitoringv1.Endpoint{
			{
				Port:   alerts.OperatorPortName,
				Scheme: "https",
				Authorization: &monitoringv1.SafeAuthorization{
					Credentials: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: secretName,
						},
						Key: "token",
					},
				},
				TLSConfig: &monitoringv1.TLSConfig{
					SafeTLSConfig: monitoringv1.SafeTLSConfig{
						InsecureSkipVerify: ptr.To(true),
					},
				},
			},
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
