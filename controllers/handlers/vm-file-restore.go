package handlers

import (
	"errors"
	"reflect"
	"slices"
	"sync"

	openshiftconfigv1 "github.com/openshift/api/config/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	vmfr "kubevirt.io/vm-file-restore-operator/api/v1alpha1"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/tlssecprofile"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

const (
	vmfrCRName = "vm-file-restore-" + hcoutil.HyperConvergedName
)

func NewVMFileRestoreHandler(cli client.Client, scheme *runtime.Scheme) operands.Operand {
	return operands.NewGenericOperand(cli, scheme, "FileRestoreOperator", &vmfrHooks{}, true)
}

type vmfrHooks struct {
	sync.Mutex
	cache *vmfr.FileRestoreOperator
}

func (h *vmfrHooks) GetFullCr(hc *hcov1.HyperConverged) (client.Object, error) {
	h.Lock()
	defer h.Unlock()

	if h.cache == nil {
		cr := NewFileRestoreOperator(hc)
		h.cache = cr
	}
	return h.cache, nil
}

func (*vmfrHooks) GetEmptyCr() client.Object { return &vmfr.FileRestoreOperator{} }
func (*vmfrHooks) GetConditions(cr runtime.Object) []metav1.Condition {
	return cr.(*vmfr.FileRestoreOperator).Status.Conditions
}
func (*vmfrHooks) CheckComponentVersion(cr runtime.Object) bool {
	found := cr.(*vmfr.FileRestoreOperator)
	return operands.CheckComponentVersion(hcoutil.VMFileRestoreOperatorVersionEnvV, found.Status.ObservedVersion)
}
func (h *vmfrHooks) Reset() {
	h.Lock()
	defer h.Unlock()
	h.cache = nil
}

func (*vmfrHooks) UpdateCR(req *common.HcoRequest, cli client.Client, exists runtime.Object, required runtime.Object) (bool, bool, error) {
	cr, ok1 := required.(*vmfr.FileRestoreOperator)
	found, ok2 := exists.(*vmfr.FileRestoreOperator)
	if !ok1 || !ok2 {
		return false, false, errors.New("can't convert to KubeVirt")
	}

	if !reflect.DeepEqual(found.Spec, cr.Spec) ||
		!hcoutil.CompareLabels(cr, found) {
		if req.HCOTriggered {
			req.Logger.Info("Updating existing FileRestoreOperator's Spec to new opinionated values")
		} else {
			req.Logger.Info("Reconciling an externally updated FileRestoreOperator's Spec to its opinionated values")
		}
		hcoutil.MergeLabels(&cr.ObjectMeta, &found.ObjectMeta)

		cr.Spec.DeepCopyInto(&found.Spec)
		err := cli.Update(req.Ctx, found)
		if err != nil {
			return false, false, err
		}
		return true, !req.HCOTriggered, nil
	}
	return false, false, nil
}

func NewFileRestoreOperator(hc *hcov1.HyperConverged) *vmfr.FileRestoreOperator {
	cr := NewFileRestoreOperatorWithNameOnly()

	spec := vmfr.FileRestoreOperatorSpec{
		ImagePullPolicy: corev1.PullIfNotPresent,
	}

	tlsProfile := tlssecprofile.GetTLSSecurityProfile(hc.Spec.Security.TLSSecurityProfile)
	spec.TLSSecurityProfile = openshift2FileRestoreSecProfile(tlsProfile)

	cr.Spec = spec

	return cr
}

func NewFileRestoreOperatorWithNameOnly() *vmfr.FileRestoreOperator {
	return &vmfr.FileRestoreOperator{
		ObjectMeta: metav1.ObjectMeta{
			Name:      vmfrCRName,
			Labels:    operands.GetLabels(hcoutil.AppComponentFileRestore),
			Namespace: hcoutil.GetOperatorNamespaceFromEnv(),
		},
	}
}

func openshift2FileRestoreSecProfile(hcProfile *openshiftconfigv1.TLSSecurityProfile) *vmfr.TLSSecurityProfile {
	var custom *vmfr.CustomTLSProfile
	if hcProfile.Custom != nil {
		custom = &vmfr.CustomTLSProfile{
			TLSProfileSpec: vmfr.TLSProfileSpec{
				Ciphers:       slices.Clone(hcProfile.Custom.Ciphers),
				MinTLSVersion: vmfr.TLSProtocolVersion(hcProfile.Custom.MinTLSVersion),
			},
		}
	}

	return &vmfr.TLSSecurityProfile{
		Type:         vmfr.TLSProfileType(hcProfile.Type),
		Old:          (*vmfr.OldTLSProfile)(hcProfile.Old.DeepCopy()),
		Intermediate: (*vmfr.IntermediateTLSProfile)(hcProfile.Intermediate.DeepCopy()),
		Modern:       (*vmfr.ModernTLSProfile)(hcProfile.Modern.DeepCopy()),
		Custom:       custom,
	}
}
