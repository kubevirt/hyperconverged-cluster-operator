package virtimport

import (
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/alerts"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/virtimport"
)

const (
	servingCertSecretAnnotation = "service.beta.openshift.io/serving-cert-secret-name"
	tokenSecretKey              = "token"
	caCertKey                   = "ca.crt"
)

// newMetricsService returns the bundle metrics Service. On OpenShift, service-ca issues the
// serving certificate into the Secret the Deployment mounts.
func newMetricsService(b *virtimport.Bundle) *corev1.Service {
	svc := b.MetricsService.DeepCopy()
	setMetadata(&svc.ObjectMeta, true)
	if hcoutil.GetClusterInfo().IsOpenshift() {
		if svc.Annotations == nil {
			svc.Annotations = make(map[string]string, 1)
		}
		svc.Annotations[servingCertSecretAnnotation] = b.MetricsCertSecretName()
	}
	return svc
}

func newCertificate(b *virtimport.Bundle) *unstructured.Unstructured {
	secretName := b.MetricsCertSecretName()
	return operands.NewCertificate(secretName, secretName, []string{serviceHost(b)}, appComponent)
}

// newServiceMonitor adds the platform's TLS and auth to the bundle ServiceMonitor: the Prometheus
// client certificate on OpenShift, the bundle token elsewhere.
func newServiceMonitor(b *virtimport.Bundle) *monitoringv1.ServiceMonitor {
	sm := b.ServiceMonitor.DeepCopy()
	setMetadata(&sm.ObjectMeta, true)
	namespace := hcoutil.GetOperatorNamespaceFromEnv()
	sm.Spec.NamespaceSelector = monitoringv1.NamespaceSelector{MatchNames: []string{namespace}}

	endpoint := &sm.Spec.Endpoints[0]
	if hcoutil.GetClusterInfo().IsOpenshift() {
		sm.Spec.ScrapeClassName = new(hcoutil.OpenShiftScrapeClassName)
		endpoint.HTTPConfigWithTLSFiles = alerts.CertBasedHTTPConfig(namespace, b.MetricsService.Name)
		return sm
	}

	serverName := serviceHost(b)
	endpoint.TLSConfig = &monitoringv1.TLSConfig{
		SafeTLSConfig: monitoringv1.SafeTLSConfig{
			CA:                 monitoringv1.SecretOrConfigMap{Secret: secretKey(b.MetricsCertSecretName(), caCertKey)},
			ServerName:         &serverName,
			InsecureSkipVerify: new(false),
		},
	}
	if name := tokenSecretName(b); name != "" {
		endpoint.Authorization = &monitoringv1.SafeAuthorization{Credentials: secretKey(name, tokenSecretKey)}
	}
	return sm
}

// tokenSecretName returns the bundle token Secret, the parser allows at most one.
func tokenSecretName(b *virtimport.Bundle) string {
	if len(b.Secrets) == 0 {
		return ""
	}
	return b.Secrets[0].Name
}

func serviceHost(b *virtimport.Bundle) string {
	return b.MetricsService.Name + "." + hcoutil.GetOperatorNamespaceFromEnv() + ".svc"
}

func secretKey(name, key string) *corev1.SecretKeySelector {
	return &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: name}, Key: key}
}
