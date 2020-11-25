package operands

import (
	"errors"
	"fmt"
	"reflect"
	"sync"

	sspv1beta1 "kubevirt.io/ssp-operator/api/v1beta1"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/pkg/apis/hco/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/controller/common"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
	conditionsv1 "github.com/openshift/custom-resource-status/conditions/v1"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// This is initially set to 2 replicas, to maintain the behavior of the previous SSP operator.
	// After SSP implements its defaulting webhook, we can change this to 0 replicas,
	// and let the webhook set the default.
	defaultTemplateValidatorReplicas = 2

	defaultCommonTemplatesNamespace = hcoutil.OpenshiftNamespace
)

type sspHandler struct {
	genericOperand

	crdsToRemove []string
}

func newSspHandler(Client client.Client, Scheme *runtime.Scheme) *sspHandler {
	return &sspHandler{
		genericOperand: genericOperand{
			Client:                 Client,
			Scheme:                 Scheme,
			crType:                 "SSP",
			isCr:                   true,
			removeExistingOwner:    false,
			setControllerReference: false,
			hooks:                  &sspHooks{},
		},

		crdsToRemove: []string{
			// These are the 2nd generation SSP CRDs,
			// where the group name has been changed to "ssp.kubevirt.io"
			"kubevirtcommontemplatesbundles.ssp.kubevirt.io",
			"kubevirtmetricsaggregations.ssp.kubevirt.io",
			"kubevirtnodelabellerbundles.ssp.kubevirt.io",
			"kubevirttemplatevalidators.ssp.kubevirt.io",

			// These are the original SSP CRDs, with the group name "kubevirt.io".
			// We attempt to remove these too, for upgrades from an older version.
			"kubevirtcommontemplatesbundles.kubevirt.io",
			"kubevirtmetricsaggregations.kubevirt.io",
			"kubevirtnodelabellerbundles.kubevirt.io",
			"kubevirttemplatevalidators.kubevirt.io",
		},
	}
}

func (handler *sspHandler) ensure(req *common.HcoRequest) *EnsureResult {
	res := handler.genericOperand.ensure(req)

	// Attempt to remove old CRDs
	if len(handler.crdsToRemove) > 0 && (!req.UpgradeMode || res.UpgradeDone) {
		unremovedCRDs := removeCRDs(handler.Client, req, handler.crdsToRemove)
		handler.crdsToRemove = unremovedCRDs
	}

	return res
}

type sspHooks struct{}

func (h sspHooks) getFullCr(hc *hcov1beta1.HyperConverged) runtime.Object {
	return NewSSP(hc)
}
func (h sspHooks) getEmptyCr() runtime.Object                         { return &sspv1beta1.SSP{} }
func (h sspHooks) validate() error                                    { return nil }
func (h sspHooks) postFound(*common.HcoRequest, runtime.Object) error { return nil }
func (h sspHooks) getConditions(cr runtime.Object) []conditionsv1.Condition {
	return cr.(*sspv1beta1.SSP).Status.Conditions
}
func (h sspHooks) checkComponentVersion(cr runtime.Object) bool {
	found := cr.(*sspv1beta1.SSP)
	return checkComponentVersion(hcoutil.SspVersionEnvV, found.Status.ObservedVersion)
}
func (h sspHooks) getObjectMeta(cr runtime.Object) *metav1.ObjectMeta {
	return &cr.(*sspv1beta1.SSP).ObjectMeta
}

func (h *sspHooks) updateCr(req *common.HcoRequest, client client.Client, exists runtime.Object, required runtime.Object) (bool, bool, error) {
	ssp, ok1 := required.(*sspv1beta1.SSP)
	found, ok2 := exists.(*sspv1beta1.SSP)
	if !ok1 || !ok2 {
		return false, false, errors.New("can't convert to SSP")
	}
	if !reflect.DeepEqual(found.Spec, ssp.Spec) {
		if req.HCOTriggered {
			req.Logger.Info("Updating existing SSP's Spec to new opinionated values")
		} else {
			req.Logger.Info("Reconciling an externally updated SSP's Spec to its opinionated values")
		}
		ssp.Spec.DeepCopyInto(&found.Spec)
		err := client.Update(req.Ctx, found)
		if err != nil {
			return false, false, err
		}
		return true, !req.HCOTriggered, nil
	}
	return false, false, nil
}

func NewSSP(hc *hcov1beta1.HyperConverged, opts ...string) *sspv1beta1.SSP {
	replicas := int32(defaultTemplateValidatorReplicas)

	spec := sspv1beta1.SSPSpec{
		TemplateValidator: sspv1beta1.TemplateValidator{
			Replicas: &replicas,
		},
		CommonTemplates: sspv1beta1.CommonTemplates{
			Namespace: defaultCommonTemplatesNamespace,
		},
		// NodeLabeller field is explicitly initialized to its zero-value,
		// in order to future-proof from bugs if SSP changes it to pointer-type,
		// causing nil pointers dereferences at the DeepCopyInto() below.
		NodeLabeller: sspv1beta1.NodeLabeller{},
	}

	if hc.Spec.Infra.NodePlacement != nil {
		spec.TemplateValidator.Placement = hc.Spec.Infra.NodePlacement.DeepCopy()
	}

	if hc.Spec.Workloads.NodePlacement != nil {
		spec.NodeLabeller.Placement = hc.Spec.Workloads.NodePlacement.DeepCopy()
	}

	return &sspv1beta1.SSP{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ssp-" + hc.Name,
			Labels:    getLabels(hc),
			Namespace: getNamespace(hc.Namespace, opts),
		},
		Spec: spec,
	}
}

// returns a slice of CRD names that weren't successfully removed
func removeCRDs(clt client.Client, req *common.HcoRequest, crdNames []string) []string {
	unremovedCRDs := make([]string, 0, len(crdNames))

	// The deletion is performed concurrently for all CRDs.
	var mutex sync.Mutex
	var wg sync.WaitGroup
	wg.Add(len(crdNames))

	for _, crdName := range crdNames {
		go func(crdName string) {
			removed := removeCRD(clt, req, crdName)

			// If removal failed for some reason, we'll retry in the next reconciliation loop.
			if !removed {
				mutex.Lock()
				defer mutex.Unlock()

				unremovedCRDs = append(unremovedCRDs, crdName)
			}

			wg.Done()
		}(crdName)
	}

	wg.Wait()

	return unremovedCRDs
}

// returns true if not found or if deletion succeeded, and false otherwise.
func removeCRD(clt client.Client, req *common.HcoRequest, crdName string) bool {
	found := &apiextensionsv1.CustomResourceDefinition{}
	key := client.ObjectKey{Namespace: hcoutil.UndefinedNamespace, Name: crdName}
	err := clt.Get(req.Ctx, key, found)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			req.Logger.Error(err, fmt.Sprintf("failed to read the %s CRD; %s", crdName, err.Error()))
			return false
		}
	} else {
		err = clt.Delete(req.Ctx, found)
		if err != nil {
			req.Logger.Error(err, fmt.Sprintf("failed to remove the %s CRD; %s", crdName, err.Error()))
			return false
		}

		req.Logger.Info("successfully removed CRD", "CRD Name", crdName)
	}

	return true
}
