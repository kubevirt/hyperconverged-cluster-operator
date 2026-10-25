package bearer_token_controller

import (
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/alerts"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

func newWHServiceMonitorReconciler(namespace string, owner metav1.OwnerReference, refresher alerts.Refresher, isCertBasedAuth bool) *alerts.ServiceMonitorReconciler {
	sm := newServiceMonitor(namespace, owner, isCertBasedAuth)
	return alerts.CreateServiceMonitorReconciler(sm, refresher, isCertBasedAuth)
}

// the webhook metrics service is now created by the deployer from a static manifest.
// But the ServiceMonitor uses the same name. We want to edit the existing one
// instead of creating new one. So we'll name it with the old service name.
const serviceMonitorName = "hyperconverged-cluster-webhook-operator-metrics"

func newServiceMonitor(namespace string, owner metav1.OwnerReference, isCertBasedAuth bool) *monitoringv1.ServiceMonitor {
	spec := monitoringv1.ServiceMonitorSpec{
		Selector: metav1.LabelSelector{
			MatchLabels: map[string]string{
				hcoutil.AppLabel:          hcoutil.HCOWebhookName,
				hcoutil.AppLabelPartOf:    hcoutil.HyperConvergedCluster,
				hcoutil.AppLabelComponent: string(hcoutil.AppComponentMonitoring),
			},
		},
	}

	var httpConfig monitoringv1.HTTPConfigWithTLSFiles
	if isCertBasedAuth {
		httpConfig = alerts.CertBasedHTTPConfig(namespace, hcoutil.WebhookMetricServiceName)
		spec.NamespaceSelector = monitoringv1.NamespaceSelector{
			MatchNames: []string{namespace},
		}
		spec.ScrapeClassName = new(hcoutil.OpenShiftScrapeClassName)
	} else {
		httpConfig = alerts.TokenBasedHTTPConfig(secretName)
	}

	spec.Endpoints = []monitoringv1.Endpoint{
		alerts.CreateEndpoint(httpConfig),
	}

	return &monitoringv1.ServiceMonitor{
		TypeMeta: metav1.TypeMeta{
			APIVersion: monitoringv1.SchemeGroupVersion.String(),
			Kind:       monitoringv1.ServiceMonitorsKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            serviceMonitorName,
			Labels:          getLabels(),
			Namespace:       namespace,
			OwnerReferences: []metav1.OwnerReference{owner},
		},
		Spec: spec,
	}
}
