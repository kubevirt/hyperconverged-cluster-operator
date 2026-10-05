package alerts

import (
	"context"
	"reflect"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

const (
	OperatorPortName = "http-metrics"
	OperatorNameEnv  = "OPERATOR_NAME"

	serviceName = hcoutil.OperatorMetricServiceName
)

type MetricServiceReconciler struct {
	theService *corev1.Service
}

func CreateMetricServiceReconciler(theService *corev1.Service) *MetricServiceReconciler {
	return &MetricServiceReconciler{theService: theService}
}

// NOTE: the service for the operator is no longer created here.
// The operator metrics Service is installed from static manifests so that the
// metrics TLS certificate can be provisioned and mounted before the operator starts.

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
