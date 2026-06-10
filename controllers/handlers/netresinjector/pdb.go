package netresinjector

import (
	"errors"
	"reflect"

	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

func NewPDBHandler(cli client.Client, scheme *runtime.Scheme) operands.Operand {
	return operands.NewGenericOperand(cli, scheme, "PodDisruptionBudget",
		&pdbHooks{pdb: newPDB()}, true)
}

type pdbHooks struct {
	pdb *policyv1.PodDisruptionBudget
}

func (h *pdbHooks) GetFullCr(_ *hcov1.HyperConverged) (client.Object, error) {
	return h.pdb.DeepCopy(), nil
}

func (*pdbHooks) GetEmptyCr() client.Object {
	return &policyv1.PodDisruptionBudget{}
}

func (*pdbHooks) UpdateCR(req *common.HcoRequest, Client client.Client, exists runtime.Object, required runtime.Object) (bool, bool, error) {
	pdb, ok1 := required.(*policyv1.PodDisruptionBudget)
	found, ok2 := exists.(*policyv1.PodDisruptionBudget)
	if !ok1 || !ok2 {
		return false, false, errors.New("can't convert to PodDisruptionBudget")
	}

	if hasPDBRightFields(found, pdb) {
		return false, false, nil
	}

	if req.HCOTriggered {
		req.Logger.Info("Updating existing PodDisruptionBudget Spec to new opinionated values")
	} else {
		req.Logger.Info("Reconciling an externally updated PodDisruptionBudget's Spec to its opinionated values")
	}
	hcoutil.MergeLabels(&pdb.ObjectMeta, &found.ObjectMeta)
	pdb.Spec.DeepCopyInto(&found.Spec)
	err := Client.Update(req.Ctx, found)
	if err != nil {
		return false, false, err
	}

	return true, !req.HCOTriggered, nil
}

func hasPDBRightFields(found, required *policyv1.PodDisruptionBudget) bool {
	return hcoutil.CompareLabels(required, found) &&
		reflect.DeepEqual(required.Spec.Selector, found.Spec.Selector) &&
		reflect.DeepEqual(required.Spec.MinAvailable, found.Spec.MinAvailable) &&
		reflect.DeepEqual(required.Spec.MaxUnavailable, found.Spec.MaxUnavailable)
}

func NewPDBWithNameOnly() *policyv1.PodDisruptionBudget {
	return &policyv1.PodDisruptionBudget{
		TypeMeta: metav1.TypeMeta{
			APIVersion: policyv1.SchemeGroupVersion.String(),
			Kind:       "PodDisruptionBudget",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName + "-pdb",
			Namespace: hcoutil.GetOperatorNamespaceFromEnv(),
			Labels:    operands.GetLabels(hcoutil.AppComponentNetResInjector),
		},
	}
}

func newPDB() *policyv1.PodDisruptionBudget {
	pdb := NewPDBWithNameOnly()
	pdb.Spec = policyv1.PodDisruptionBudgetSpec{
		MinAvailable: &intstr.IntOrString{Type: intstr.Int, IntVal: 1},
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				hcoutil.AppLabel:          hcoutil.HyperConvergedName,
				hcoutil.AppLabelComponent: string(hcoutil.AppComponentNetResInjector),
			},
		},
	}
	return pdb
}
