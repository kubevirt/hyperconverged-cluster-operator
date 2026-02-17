package wasp_agent

import (
	_ "embed"

	mcfgv1 "github.com/openshift/api/machineconfiguration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"errors"
	"reflect"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

const (
	machineConfigName = "90-worker-swap-online"
)

//go:embed machineconfig.yaml
var machineConfigBytes []byte

func NewWaspAgentMachineConfigHandler(Client client.Client, Scheme *runtime.Scheme) operands.Operand {
	mc := newWaspAgentMachineConfig(nil)
	return newConditionalMachineConfigHandler(
		Client,
		Scheme,
		mc,
		shouldDeployWaspAgent,
		func(hc *hcov1beta1.HyperConverged) client.Object {
			return NewWaspAgentMachineConfigWithNameOnly(hc)
		},
	)
}

func newConditionalMachineConfigHandler(
	Client client.Client,
	Scheme *runtime.Scheme,
	required *mcfgv1.MachineConfig,
	shouldDeploy operands.ConditionFunc,
	getCRWithName operands.GetCRWithNameFunc,
) operands.Operand {
	return operands.NewConditionalHandler(
		NewMachineConfigHandler(Client, Scheme, required),
		shouldDeploy,
		getCRWithName,
	)
}

func NewWaspAgentMachineConfigWithNameOnly(hc *hcov1beta1.HyperConverged) *mcfgv1.MachineConfig {
	return &mcfgv1.MachineConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:   machineConfigName,
			Labels: operands.GetLabels(hc, AppComponentWaspAgent),
		},
	}
}

func newWaspAgentMachineConfig(hc *hcov1beta1.HyperConverged) *mcfgv1.MachineConfig {
	mc, err := getMachineConfig()
	if err != nil {
		// This should never happen since the YAML is embedded at compile time
		panic(err)
	}

	if hc != nil {
		mc.Labels = operands.GetLabels(hc, AppComponentWaspAgent)
	}

	return mc
}

func getMachineConfig() (*mcfgv1.MachineConfig, error) {
	mc := &mcfgv1.MachineConfig{}
	if err := yaml.Unmarshal(machineConfigBytes, mc); err != nil {
		return nil, err
	}
	return mc, nil
}

func NewMachineConfigHandler(Client client.Client, Scheme *runtime.Scheme, required *mcfgv1.MachineConfig) *operands.GenericOperand {
	return operands.NewGenericOperand(Client, Scheme, "MachineConfig", &machineConfigHooks{required: required}, false)
}

type machineConfigHooks struct {
	required *mcfgv1.MachineConfig
}

func (h *machineConfigHooks) GetFullCr(_ *hcov1beta1.HyperConverged) (client.Object, error) {
	return h.required.DeepCopy(), nil
}

func (h *machineConfigHooks) GetEmptyCr() client.Object {
	return &mcfgv1.MachineConfig{}
}

func (h *machineConfigHooks) UpdateCR(req *common.HcoRequest, Client client.Client, exists runtime.Object, _ runtime.Object) (bool, bool, error) {
	machineConfig := h.required
	found, ok := exists.(*mcfgv1.MachineConfig)
	if !ok {
		return false, false, errors.New("can't convert to a MachineConfig")
	}

	if !util.CompareLabels(machineConfig, found) ||
		!reflect.DeepEqual(machineConfig.Spec, found.Spec) {

		req.Logger.Info("Updating existing MachineConfig to its default values", "name", found.Name)

		found.Spec = machineConfig.Spec
		util.MergeLabels(&machineConfig.ObjectMeta, &found.ObjectMeta)

		err := Client.Update(req.Ctx, found)
		if err != nil {
			return false, false, err
		}
		return true, !req.HCOTriggered, nil
	}

	return false, false, nil
}
