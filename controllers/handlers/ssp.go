package handlers

import (
	"errors"
	"reflect"
	"slices"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sspv1beta3 "kubevirt.io/ssp-operator/api/v1beta3"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	goldenimages "github.com/kubevirt/hyperconverged-cluster-operator/controllers/handlers/golden-images"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/nodeinfo"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/reformatobj"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/tlssecprofile"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

const (
	// This is initially set to 2 replicas, to maintain the behavior of the previous SSP operator.
	// After SSP implements its defaulting webhook, we can change this to 0 replicas,
	// and let the webhook set the default.
	defaultTemplateValidatorReplicas = int32(2)

	defaultCommonTemplatesNamespace = util.OpenshiftNamespace
)

type sspHandler struct {
	handler *operands.GenericOperand
	hook    *sspHooks
}

func (h *sspHandler) Ensure(req *common.HcoRequest) *operands.EnsureResult {
	res := h.handler.Ensure(req)

	if res.Err == nil {
		h.hook.updateDICTsInHCStatus(req)
	}

	return res
}

func (h *sspHandler) Reset() {
	h.handler.Reset()
}

func NewSspHandler(Client client.Client, Scheme *runtime.Scheme) operands.Operand {
	hook := &sspHooks{}
	handler := operands.NewGenericOperand(Client, Scheme, "SSP", hook, false)

	return &sspHandler{
		handler: handler,
		hook:    hook,
	}
}

type sspHooks struct {
	sync.Mutex
	cache        *sspv1beta3.SSP
	dictStatuses []hcov1.DataImportCronTemplateStatus
}

func (h *sspHooks) GetFullCr(hc *hcov1.HyperConverged) (client.Object, error) {
	h.Lock()
	defer h.Unlock()

	if h.cache == nil {
		ssp, dictStatus, err := NewSSP(hc, false)
		if err != nil {
			return nil, err
		}
		h.cache = ssp
		h.dictStatuses = dictStatus
	}
	return h.cache, nil
}

func (*sspHooks) GetEmptyCr() client.Object { return &sspv1beta3.SSP{} }
func (*sspHooks) GetConditions(cr runtime.Object) []metav1.Condition {
	return operands.OSConditionsToK8s(cr.(*sspv1beta3.SSP).Status.Conditions)
}
func (*sspHooks) CheckComponentVersion(cr runtime.Object) bool {
	found := cr.(*sspv1beta3.SSP)
	return operands.CheckComponentVersion(util.SspVersionEnvV, found.Status.ObservedVersion)
}

func (h *sspHooks) Reset() {
	h.Lock()
	defer h.Unlock()

	h.cache = nil
	h.dictStatuses = nil
}

func (*sspHooks) UpdateCR(req *common.HcoRequest, client client.Client, exists runtime.Object, required runtime.Object) (bool, bool, error) {
	ssp, ok1 := required.(*sspv1beta3.SSP)
	found, ok2 := exists.(*sspv1beta3.SSP)
	if !ok1 || !ok2 {
		return false, false, errors.New("can't convert to SSP")
	}
	if !reflect.DeepEqual(ssp.Spec, found.Spec) ||
		!util.CompareLabels(ssp, found) {
		if req.HCOTriggered {
			req.Logger.Info("Updating existing SSP's Spec to new opinionated values")
		} else {
			req.Logger.Info("Reconciling an externally updated SSP's Spec to its opinionated values")
		}
		util.MergeLabels(&ssp.ObjectMeta, &found.ObjectMeta)
		ssp.Spec.DeepCopyInto(&found.Spec)
		err := client.Update(req.Ctx, found)
		if err != nil {
			return false, false, err
		}
		return true, !req.HCOTriggered, nil
	}
	return false, false, nil
}

func (h *sspHooks) updateDICTsInHCStatus(req *common.HcoRequest) {
	if !reflect.DeepEqual(h.dictStatuses, req.Instance.Status.DataImportCronTemplates) {
		req.Instance.Status.DataImportCronTemplates = h.dictStatuses
		req.StatusDirty = true
	}

	goldenimages.CheckDataImportCronTemplates(req.Instance)
}

func NewSSP(hc *hcov1.HyperConverged, useNodeInfoFromStatus bool) (*sspv1beta3.SSP, []hcov1.DataImportCronTemplateStatus, error) {
	templatesNamespace := ptr.Deref(hc.Spec.WorkloadSources.CommonTemplatesNamespace, defaultCommonTemplatesNamespace)

	goldenimages.ApplyDataImportSchedule(hc)

	dataImportCronStatuses, err := goldenimages.GetDataImportCronTemplates(hc)
	if err != nil {
		return nil, nil, err
	}

	var (
		cpArches []string
		wlArches []string
	)

	if useNodeInfoFromStatus {
		// The webhook pod does not run the node controller, and so the node-info is never
		// up-to-date. But the same info is copying to the HyperConverged CR status by the
		// operator, and that is good enough for the webhook.
		cpArches = slices.Clone(hc.Status.NodeInfo.ControlPlaneArchitectures)
		wlArches = slices.Clone(hc.Status.NodeInfo.WorkloadsArchitectures)
	} else {
		cpArches = nodeinfo.GetControlPlaneArchitectures()
		wlArches = nodeinfo.GetWorkloadsArchitectures()
	}

	var cluster *sspv1beta3.Cluster
	if len(cpArches) > 0 || len(wlArches) > 0 {
		cluster = &sspv1beta3.Cluster{
			ControlPlaneArchitectures: cpArches,
			WorkloadArchitectures:     wlArches,
		}
	}

	spec := sspv1beta3.SSPSpec{
		TemplateValidator: &sspv1beta3.TemplateValidator{
			Replicas: ptr.To(defaultTemplateValidatorReplicas),
		},
		CommonTemplates: sspv1beta3.CommonTemplates{
			Namespace:               templatesNamespace,
			DataImportCronTemplates: goldenimages.HCODictSliceToSSP(hc, dataImportCronStatuses),
		},

		Cluster: cluster,

		EnableMultipleArchitectures: ptr.To(hc.Spec.FeatureGates.IsEnabled(goldenimages.EnableMultiArchFeatureGate)),

		// NodeLabeller field is explicitly initialized to its zero-value,
		// in order to future-proof from bugs if SSP changes it to pointer-type,
		// causing nil pointers dereferences at the DeepCopyInto() below.
		TLSSecurityProfile: tlssecprofile.GetTLSSecurityProfile(hc.Spec.Security.TLSSecurityProfile),
	}

	if hc.Spec.Deployment.DeployVMConsoleProxy != nil {
		spec.TokenGenerationService = &sspv1beta3.TokenGenerationService{
			Enabled: *hc.Spec.Deployment.DeployVMConsoleProxy,
		}
	}

	if np := hc.Spec.Deployment.NodePlacements; np != nil && np.Infra != nil {
		spec.TemplateValidator.Placement = np.Infra.DeepCopy()
	}

	ssp := NewSSPWithNameOnly()
	ssp.Spec = spec

	if err = operands.ApplyPatchToSpec(hc, common.JSONPatchSSPAnnotationName, ssp); err != nil {
		return nil, nil, err
	}

	ssp, err = reformatobj.ReformatObj(ssp)
	if err != nil {
		return nil, nil, err
	}

	return ssp, dataImportCronStatuses, nil
}

func NewSSPWithNameOnly() *sspv1beta3.SSP {
	return &sspv1beta3.SSP{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ssp-" + util.HyperConvergedName,
			Labels:    operands.GetLabels(util.AppComponentSchedule),
			Namespace: util.GetOperatorNamespaceFromEnv(),
		},
	}
}
