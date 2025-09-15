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

const (
	SecretCreateTimeAnn = "hco.kubevirt.io/secret-creation-time"
)

type ServiceMonitorReconciler struct {
	theServiceMonitor *monitoringv1.ServiceMonitor
}

func CreateServiceMonitorReconciler(serviceMonitor *monitoringv1.ServiceMonitor) *ServiceMonitorReconciler {
	return &ServiceMonitorReconciler{theServiceMonitor: serviceMonitor}
}

func newServiceMonitorReconciler(namespace string, owner metav1.OwnerReference) *ServiceMonitorReconciler {
	return CreateServiceMonitorReconciler(NewServiceMonitor(namespace, owner))
}

func (r ServiceMonitorReconciler) Kind() string {
	return monitoringv1.ServiceMonitorsKind
}

func (r ServiceMonitorReconciler) ResourceName() string {
	return r.theServiceMonitor.Name
}

func (r ServiceMonitorReconciler) GetFullResource() client.Object {
	sm := r.theServiceMonitor.DeepCopy()
	if sm.Annotations == nil {
		sm.Annotations = make(map[string]string)
	}

	sm.Annotations[SecretCreateTimeAnn] = secretCreationTime.Get()

	return sm
}

func (r ServiceMonitorReconciler) EmptyObject() client.Object {
	return &monitoringv1.ServiceMonitor{}
}

func (r ServiceMonitorReconciler) UpdateExistingResource(ctx context.Context, cl client.Client, resource client.Object, logger logr.Logger) (client.Object, bool, error) {
	found := resource.(*monitoringv1.ServiceMonitor)
	modified := false
	if !reflect.DeepEqual(found.Spec, r.theServiceMonitor.Spec) {
		r.theServiceMonitor.Spec.DeepCopyInto(&found.Spec)
		modified = true
	}

	secretTime := secretCreationTime.Get()
	if r.theServiceMonitor.Annotations[SecretCreateTimeAnn] != secretTime {
		if r.theServiceMonitor.Annotations == nil {
			r.theServiceMonitor.Annotations = make(map[string]string)
		}
		r.theServiceMonitor.Annotations[SecretCreateTimeAnn] = secretTime
	}

	if found.Annotations[SecretCreateTimeAnn] != r.theServiceMonitor.Annotations[SecretCreateTimeAnn] {
		if found.Annotations == nil {
			found.Annotations = make(map[string]string)
		}
		found.Annotations[SecretCreateTimeAnn] = secretTime
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
