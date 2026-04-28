package handlers

import (
	"errors"
	"reflect"
	"sync"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	migrationv1alpha1 "kubevirt.io/kubevirt-migration-operator/api/v1alpha1"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/reformatobj"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

func NewMigControllerHandler(Client client.Client, Scheme *runtime.Scheme) *operands.GenericOperand {
	return operands.NewGenericOperand(Client, Scheme, "MigController", &migrationHooks{}, false)
}

type migrationHooks struct {
	sync.Mutex
	cache *migrationv1alpha1.MigController
}

func (h *migrationHooks) GetFullCr(hc *hcov1beta1.HyperConverged) (client.Object, error) {
	h.Lock()
	defer h.Unlock()

	if h.cache == nil {
		migController, err := NewMigController(hc)
		if err != nil {
			return nil, err
		}
		h.cache = migController
	}
	return h.cache, nil
}

func (*migrationHooks) GetEmptyCr() client.Object { return &migrationv1alpha1.MigController{} }

func (*migrationHooks) GetConditions(cr runtime.Object) []metav1.Condition {
	return operands.OSConditionsToK8s(cr.(*migrationv1alpha1.MigController).Status.Conditions)
}

func (*migrationHooks) CheckComponentVersion(cr runtime.Object) bool {
	found := cr.(*migrationv1alpha1.MigController)
	return operands.CheckComponentVersion(hcoutil.MigrationOperatorVersionEnvV, found.Status.ObservedVersion)
}

func (h *migrationHooks) Reset() {
	h.Lock()
	defer h.Unlock()

	h.cache = nil
}

func (*migrationHooks) UpdateCR(req *common.HcoRequest, Client client.Client, exists runtime.Object, required runtime.Object) (bool, bool, error) {
	migController, ok1 := required.(*migrationv1alpha1.MigController)
	found, ok2 := exists.(*migrationv1alpha1.MigController)
	if !ok1 || !ok2 {
		return false, false, errors.New("can't convert to MigController")
	}

	if !reflect.DeepEqual(found.Spec, migController.Spec) ||
		!hcoutil.CompareLabels(migController, found) {
		overwritten := false
		if req.HCOTriggered {
			req.Logger.Info("Updating existing MigController's Spec to new opinionated values")
		} else {
			req.Logger.Info("Reconciling an externally updated MigController's Spec to its opinionated values")
			overwritten = true
		}
		hcoutil.MergeLabels(&migController.ObjectMeta, &found.ObjectMeta)
		migController.Spec.DeepCopyInto(&found.Spec)
		err := Client.Update(req.Ctx, found)
		if err != nil {
			return false, false, err
		}
		return true, overwritten, nil
	}
	return false, false, nil
}

func NewMigController(hc *hcov1beta1.HyperConverged) (*migrationv1alpha1.MigController, error) {
	spec := migrationv1alpha1.MigControllerSpec{
		ImagePullPolicy: corev1.PullIfNotPresent,
	}

	if hc.Spec.Infra.NodePlacement != nil {
		hc.Spec.Infra.NodePlacement.DeepCopyInto(&spec.Infra)
	}

	migController := NewMigControllerWithNameOnly(hc)
	migController.Spec = spec

	return reformatobj.ReformatObj(migController)
}

func NewMigControllerWithNameOnly(hc *hcov1beta1.HyperConverged) *migrationv1alpha1.MigController {
	return &migrationv1alpha1.MigController{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "migcontroller-" + hc.Name,
			Namespace: hc.Namespace,
			Labels:    operands.GetLabels(hc, hcoutil.AppComponentMigration),
		},
	}
}
