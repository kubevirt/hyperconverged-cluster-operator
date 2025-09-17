package alerts

import (
	"context"
	"reflect"

	"github.com/go-logr/logr"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

type ServiceMonitorReconciler struct {
	refresher         Refresher
	theServiceMonitor *monitoringv1.ServiceMonitor
}

func CreateServiceMonitorReconciler(serviceMonitor *monitoringv1.ServiceMonitor, rfr Refresher) *ServiceMonitorReconciler {
	return &ServiceMonitorReconciler{
		theServiceMonitor: serviceMonitor,
		refresher:         rfr,
	}
}

func newServiceMonitorReconciler(namespace string, owner metav1.OwnerReference, rfr Refresher) *ServiceMonitorReconciler {
	return CreateServiceMonitorReconciler(NewServiceMonitor(namespace, owner), rfr)
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

	if err := r.refresher.refresh(func() error {
		return r.deleteServiceMonitor(ctx, cl, found)
	}); err != nil {
		return nil, false, err
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

func NewServiceMonitor(namespace string, owner metav1.OwnerReference) *monitoringv1.ServiceMonitor {
	labels := hcoutil.GetLabels(hcoutil.HyperConvergedName, hcoutil.AppComponentMonitoring)
	spec := monitoringv1.ServiceMonitorSpec{
		Selector: metav1.LabelSelector{
			MatchLabels: labels,
		},
		Endpoints: []monitoringv1.Endpoint{
			{
				Port:   OperatorPortName,
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
			Labels:          labels,
			Namespace:       namespace,
			OwnerReferences: []metav1.OwnerReference{owner},
		},
		Spec: spec,
	}
}
