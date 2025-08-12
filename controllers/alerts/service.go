package alerts

import (
	"context"
	"os"
	"reflect"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

const (
	OperatorPortName    = "http-metrics"
	defaultOperatorName = "hyperconverged-cluster-operator"
	OperatorNameEnv     = "OPERATOR_NAME"
	metricsSuffix       = "-operator-metrics"
	serviceName         = hcoutil.HyperConvergedName + metricsSuffix
)

type MetricServiceReconciler struct {
	theService *corev1.Service
}

func CreateMetricServiceReconciler(theService *corev1.Service) *MetricServiceReconciler {
	return &MetricServiceReconciler{theService: theService}
}

func newMetricServiceReconciler(namespace string, owner metav1.OwnerReference) *MetricServiceReconciler {
	return CreateMetricServiceReconciler(NewMetricsService(namespace, owner))
}

func (r MetricServiceReconciler) Kind() string {
	return "Service"
}

func (r MetricServiceReconciler) ResourceName() string {
	return r.theService.Name
}

func (r MetricServiceReconciler) GetFullResource() client.Object {
	return r.theService.DeepCopy()
}

func (r MetricServiceReconciler) EmptyObject() client.Object {
	return &corev1.Service{}
}

func (r MetricServiceReconciler) UpdateExistingResource(ctx context.Context, cl client.Client, resource client.Object, logger logr.Logger) (client.Object, bool, error) {
	found := resource.(*corev1.Service)

	modified := false
	if !reflect.DeepEqual(found.Spec.Selector, r.theService.Spec.Selector) ||
		!reflect.DeepEqual(found.Spec.Ports, r.theService.Spec.Ports) {

		clusterIP := found.Spec.ClusterIP
		r.theService.Spec.DeepCopyInto(&found.Spec)
		found.Spec.ClusterIP = clusterIP // restore
		modified = true
	}

	modified = updateCommonDetails(&r.theService.ObjectMeta, &found.ObjectMeta) || modified

	if modified {
		err := cl.Update(ctx, found)
		if err != nil {
			logger.Error(err, "failed to update the Service", "serviceName", r.theService.Name)
			return nil, false, err
		}
		logger.Info("successfully updated the Service", "serviceName", r.theService.Name)
	}
	return found, modified, nil
}

func NewMetricsService(namespace string, owner metav1.OwnerReference) *corev1.Service {
	servicePorts := []corev1.ServicePort{
		{
			Port:     hcoutil.MetricsPort,
			Name:     OperatorPortName,
			Protocol: corev1.ProtocolTCP,
			TargetPort: intstr.IntOrString{
				Type: intstr.Int, IntVal: hcoutil.MetricsPort,
			},
		},
	}

	operatorName := defaultOperatorName
	val, ok := os.LookupEnv(OperatorNameEnv)
	if ok && val != "" {
		operatorName = val
	}
	labelSelect := map[string]string{"name": operatorName}

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
			Labels:          hcoutil.GetLabels(hcoutil.HyperConvergedName, hcoutil.AppComponentMonitoring),
			Namespace:       namespace,
			OwnerReferences: []metav1.OwnerReference{owner},
		},
		Spec: spec,
	}
}
