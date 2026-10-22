package alerts

import (
	"context"
	"fmt"
	"reflect"

	"github.com/go-logr/logr"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

type ServiceMonitorReconciler struct {
	refresher         Refresher
	isCertBasedAuth   bool
	theServiceMonitor *monitoringv1.ServiceMonitor
}

func CreateServiceMonitorReconciler(serviceMonitor *monitoringv1.ServiceMonitor, rfr Refresher, isCertBasedAuth bool) *ServiceMonitorReconciler {
	return &ServiceMonitorReconciler{
		theServiceMonitor: serviceMonitor,
		isCertBasedAuth:   isCertBasedAuth,
		refresher:         rfr,
	}
}

func newServiceMonitorReconciler(namespace string, owner metav1.OwnerReference, rfr Refresher, ci hcoutil.ClusterInfo) *ServiceMonitorReconciler {
	isCertBasedAuth := ci.IsOpenshift()
	sm := NewServiceMonitor(namespace, owner, isCertBasedAuth)
	return CreateServiceMonitorReconciler(sm, rfr, isCertBasedAuth)
}

func (r ServiceMonitorReconciler) Kind() string {
	return monitoringv1.ServiceMonitorsKind
}

func (r ServiceMonitorReconciler) ResourceName() string {
	return r.theServiceMonitor.Name
}

func (r ServiceMonitorReconciler) GetFullResource() client.Object {
	return r.theServiceMonitor.DeepCopy()
}

func (r ServiceMonitorReconciler) EmptyObject() client.Object {
	return &monitoringv1.ServiceMonitor{}
}

func (r ServiceMonitorReconciler) UpdateExistingResource(ctx context.Context, cl client.Client, resource client.Object, logger logr.Logger) (client.Object, bool, error) {
	found := resource.(*monitoringv1.ServiceMonitor)

	if !r.isCertBasedAuth {
		if err := r.refresher.refresh(func() error {
			return r.deleteServiceMonitor(ctx, cl, found)
		}); err != nil {
			return nil, false, err
		}
	}

	modified := false
	if !reflect.DeepEqual(found.Spec, r.theServiceMonitor.Spec) {
		r.theServiceMonitor.Spec.DeepCopyInto(&found.Spec)
		modified = true
	}

	modified = updateCommonDetails(&r.theServiceMonitor.ObjectMeta, &found.ObjectMeta) || modified

	if modified {
		err := cl.Update(ctx, found)
		if err != nil {
			logger.Error(err, "failed to update the ServiceMonitor")
			return nil, false, err
		}
		logger.Info("successfully updated the ServiceMonitor")
	}
	return found, modified, nil
}

func (r ServiceMonitorReconciler) deleteServiceMonitor(ctx context.Context, cl client.Client, found *monitoringv1.ServiceMonitor) error {
	return cl.Delete(ctx, found)
}

// the operator metrics service is now created by the deployer from a static manifest.
// But the ServiceMonitor uses the same name. We want to edit the existing one
// instead of creating new one. So we'll name it with the old service name.
const serviceMonitorName = "kubevirt-hyperconverged-operator-metrics"

func NewServiceMonitor(namespace string, owner metav1.OwnerReference, isCertBasedAuth bool) *monitoringv1.ServiceMonitor {
	spec := monitoringv1.ServiceMonitorSpec{
		Selector: metav1.LabelSelector{
			MatchLabels: map[string]string{
				hcoutil.AppLabel:          hcoutil.HCOOperatorName,
				hcoutil.AppLabelPartOf:    hcoutil.HyperConvergedCluster,
				hcoutil.AppLabelComponent: string(hcoutil.AppComponentMonitoring),
			},
		},
	}

	var httpConfig monitoringv1.HTTPConfigWithTLSFiles
	if isCertBasedAuth {
		httpConfig = CertBasedHTTPConfig(namespace, serviceName)
		spec.NamespaceSelector = monitoringv1.NamespaceSelector{
			MatchNames: []string{namespace},
		}
		spec.ScrapeClassName = new(hcoutil.OpenShiftScrapeClassName)
	} else {
		httpConfig = TokenBasedHTTPConfig(secretName)
	}

	spec.Endpoints = []monitoringv1.Endpoint{
		CreateEndpoint(httpConfig),
	}

	return &monitoringv1.ServiceMonitor{
		TypeMeta: metav1.TypeMeta{
			APIVersion: monitoringv1.SchemeGroupVersion.String(),
			Kind:       monitoringv1.ServiceMonitorsKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            serviceMonitorName,
			Labels:          hcoutil.GetLabels(hcoutil.HyperConvergedName, hcoutil.AppComponentMonitoring),
			Namespace:       namespace,
			OwnerReferences: []metav1.OwnerReference{owner},
		},
		Spec: spec,
	}
}

func CreateEndpoint(httpConfig monitoringv1.HTTPConfigWithTLSFiles) monitoringv1.Endpoint {
	return monitoringv1.Endpoint{
		Port:   OperatorPortName,
		Scheme: new(monitoringv1.Scheme("https")),
		HTTPConfigWithProxyAndTLSFiles: monitoringv1.HTTPConfigWithProxyAndTLSFiles{
			HTTPConfigWithTLSFiles: httpConfig,
		},
	}
}

func TokenBasedHTTPConfig(bearerTokenSecretName string) monitoringv1.HTTPConfigWithTLSFiles {
	return monitoringv1.HTTPConfigWithTLSFiles{
		HTTPConfigWithoutTLS: monitoringv1.HTTPConfigWithoutTLS{
			Authorization: &monitoringv1.SafeAuthorization{
				Credentials: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: bearerTokenSecretName,
					},
					Key: "token",
				},
			},
		},
		TLSConfig: &monitoringv1.TLSConfig{
			SafeTLSConfig: monitoringv1.SafeTLSConfig{
				InsecureSkipVerify: new(true),
			},
		},
	}
}

func CertBasedHTTPConfig(namespace, serviceName string) monitoringv1.HTTPConfigWithTLSFiles {
	serverName := fmt.Sprintf("%s.%s.svc", serviceName, namespace)

	return monitoringv1.HTTPConfigWithTLSFiles{
		TLSConfig: &monitoringv1.TLSConfig{
			SafeTLSConfig: monitoringv1.SafeTLSConfig{
				CA:                 monitoringv1.SecretOrConfigMap{},
				Cert:               monitoringv1.SecretOrConfigMap{},
				InsecureSkipVerify: new(false),
				ServerName:         &serverName,
			},
		},
	}
}
