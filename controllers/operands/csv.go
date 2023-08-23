package operands

import (
	"errors"
	"fmt"
	"strings"

	csvv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/components"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

type csvHandler genericOperand

func newCsvHandler(Client client.Client, Scheme *runtime.Scheme, ci hcoutil.ClusterInfo) *csvHandler {
	return &csvHandler{
		Client:                 Client,
		Scheme:                 Scheme,
		crType:                 "CSV",
		setControllerReference: false,
		hooks: &csvHooks{
			ci: ci,
		},
	}
}

type csvHooks struct {
	ci hcoutil.ClusterInfo
}

func (c csvHooks) getFullCr(hc *hcov1beta1.HyperConverged) (client.Object, error) {
	csv := c.ci.GetCSV()
	csv.Annotations[components.DisableOperandDeletionAnnotation] = "true"
	if hc.Spec.UninstallStrategy == hcov1beta1.HyperConvergedUninstallStrategyRemoveWorkloads {
		csv.Annotations[components.DisableOperandDeletionAnnotation] = "false"
	}
	return csv, nil
}

func (c csvHooks) getEmptyCr() client.Object { return &csvv1alpha1.ClusterServiceVersion{} }

func (c csvHooks) updateCr(req *common.HcoRequest, cli client.Client, exists runtime.Object, required runtime.Object) (bool, bool, error) {
	found, ok1 := exists.(*csvv1alpha1.ClusterServiceVersion)
	csv, ok2 := required.(*csvv1alpha1.ClusterServiceVersion)
	if !ok1 || !ok2 {
		return false, false, errors.New("can't convert to CSV")
	}

	foundDisableOperandDeletion := found.Annotations[components.DisableOperandDeletionAnnotation]
	csvDisableOperandDeletion := csv.Annotations[components.DisableOperandDeletionAnnotation]

	if csvDisableOperandDeletion != foundDisableOperandDeletion {
		if req.HCOTriggered {
			req.Logger.Info("Updating existing CSV disable-operand-delete annotation to new opinionated values")
		} else {
			req.Logger.Info("Reconciling an externally updated CSV disable-operand-delete annotation to its opinionated values")
		}

		patch := fmt.Sprintf(
			"[{\"op\": \"replace\",\"path\": \"/metadata/annotations/%s\",\"value\": \"%s\"}]\n",
			strings.ReplaceAll(components.DisableOperandDeletionAnnotation, "/", "~1"),
			csvDisableOperandDeletion,
		)
		err := cli.Patch(req.Ctx, found, client.RawPatch(types.JSONPatchType, []byte(patch)))
		if err != nil {
			req.Logger.Error(err, "Failed to update CSV disable-operand-delete annotation")
			return false, false, err
		}
		req.Logger.Info("Updated CSV disable-operand-delete annotation")
		return true, false, nil
	}

	return false, false, nil
}

func (c csvHooks) justBeforeComplete(_ *common.HcoRequest) { /* no implementation */ }
