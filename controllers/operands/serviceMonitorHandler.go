package operands

import (
	"errors"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

func NewServiceMonitorHandler(cli client.Client, scheme *runtime.Scheme, required *monitoringv1.ServiceMonitor) *GenericOperand {
	return NewGenericOperand(cli, scheme, monitoringv1.ServiceMonitorsKind, &serviceMonitorHooks{required: required.DeepCopy()}, false)
}

type serviceMonitorHooks struct {
	required *monitoringv1.ServiceMonitor
}

func (h *serviceMonitorHooks) GetFullCr(_ *hcov1.HyperConverged) (client.Object, error) {
	return h.required.DeepCopy(), nil
}

func (h *serviceMonitorHooks) GetEmptyCr() client.Object {
	return &monitoringv1.ServiceMonitor{}
}

func (h *serviceMonitorHooks) UpdateCR(req *common.HcoRequest, cli client.Client, exists runtime.Object, required runtime.Object) (bool, bool, error) {
	monitor, ok1 := required.(*monitoringv1.ServiceMonitor)
	found, ok2 := exists.(*monitoringv1.ServiceMonitor)
	if !ok1 || !ok2 {
		return false, false, errors.New("can't convert to ServiceMonitor")
	}

	if hcoutil.CompareLabels(monitor, found) && equality.Semantic.DeepEqual(monitor.Spec, found.Spec) {
		return false, false, nil
	}

	logUpdate(req, monitoringv1.ServiceMonitorsKind, found.Name)
	hcoutil.MergeLabels(&monitor.ObjectMeta, &found.ObjectMeta)
	monitor.Spec.DeepCopyInto(&found.Spec)

	if err := cli.Update(req.Ctx, found); err != nil {
		return false, false, err
	}
	return true, !req.HCOTriggered, nil
}
