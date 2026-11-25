package operands

import (
	"errors"
	"maps"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

// NewSecretHandler reconciles a Secret whose data is filled in by someone else, like a
// ServiceAccount token Secret. Only its labels and annotations are reconciled.
func NewSecretHandler(cli client.Client, scheme *runtime.Scheme, required *corev1.Secret) *GenericOperand {
	return NewGenericOperand(cli, scheme, "Secret", &secretHooks{required: required.DeepCopy()}, false)
}

type secretHooks struct {
	required *corev1.Secret
}

func (h *secretHooks) GetFullCr(_ *hcov1.HyperConverged) (client.Object, error) {
	return h.required.DeepCopy(), nil
}

func (h *secretHooks) GetEmptyCr() client.Object {
	return &corev1.Secret{}
}

func (h *secretHooks) UpdateCR(req *common.HcoRequest, cli client.Client, exists runtime.Object, required runtime.Object) (bool, bool, error) {
	secret, ok1 := required.(*corev1.Secret)
	found, ok2 := exists.(*corev1.Secret)
	if !ok1 || !ok2 {
		return false, false, errors.New("can't convert to Secret")
	}

	if hcoutil.CompareLabels(secret, found) && hcoutil.IsSubset(secret.Annotations, found.Annotations) {
		return false, false, nil
	}

	logUpdate(req, "Secret", found.Name)
	hcoutil.MergeLabels(&secret.ObjectMeta, &found.ObjectMeta)
	if found.Annotations == nil {
		found.Annotations = make(map[string]string, len(secret.Annotations))
	}
	maps.Copy(found.Annotations, secret.Annotations)

	if err := cli.Update(req.Ctx, found); err != nil {
		return false, false, err
	}
	return true, !req.HCOTriggered, nil
}
