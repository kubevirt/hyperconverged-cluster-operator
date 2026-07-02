package featuregates

import (
	"encoding/json"
	"slices"
	"strings"

	"k8s.io/utils/ptr"

	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/featuregatedetails"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/featuregates"
)

type State string

const (
	Enabled  State = "Enabled"
	Disabled State = "Disabled"
)

// FeatureGate is an optional feature gate to enable or disable a new feature that is not generally available yet.
// +k8s:conversion-gen=false
// +k8s:openapi-gen=true
type FeatureGate struct {
	// Name is the feature gate name
	Name string `json:"name"`

	// State determines if the feature gate is Enabled, or Disabled. The default value is Enabled.
	// +kubebuilder:validation:Enum=Enabled;Disabled
	State *State `json:"state,omitempty"`
}

func (fg FeatureGate) MarshalJSON() ([]byte, error) {
	builder := &strings.Builder{}
	builder.WriteString(`{"name":"`)
	builder.WriteString(fg.Name)
	builder.WriteByte('"')

	if fg.State != nil && *fg.State == Disabled {
		builder.WriteString(`,"state":"`)
		builder.WriteString(string(Disabled))
		builder.WriteByte('"')
	}
	builder.WriteByte('}')

	return []byte(builder.String()), nil
}

func (fg *FeatureGate) UnmarshalJSON(bytes []byte) error {
	type plain FeatureGate
	err := json.Unmarshal(bytes, (*plain)(fg))
	if err != nil {
		return err
	}

	if fg.State == nil {
		fg.State = ptr.To(Enabled)
	}

	return nil
}

// HyperConvergedFeatureGates is a set of optional feature gates to enable or disable new features that are not
// generally available yet.
// Add a new FeatureGate Object to this set, to enable a feature that is disabled by default, or to disable a feature
// that is enabled by default.
//
// +k8s:openapi-gen=true
// +k8s:conversion-gen=false
// +k8s:deepcopy-gen=false
type HyperConvergedFeatureGates []FeatureGate

// Enable enables a feature gate by its name
func (fgs *HyperConvergedFeatureGates) Enable(name string) {
	fgs.set(name, Enabled)
}

// Disable disables a feature gate by its name
func (fgs *HyperConvergedFeatureGates) Disable(name string) {
	fgs.set(name, Disabled)
}

func (fgs *HyperConvergedFeatureGates) set(name string, enabled State) {
	idx := fgs.index(name)

	if idx == -1 {
		*fgs = append(*fgs, FeatureGate{Name: name, State: &enabled})
		return
	}

	(*fgs)[idx].State = &enabled
}

// IsEnabled return true if the feature gate is enabled
//   - If the feature gate is GA, it's always enabled.
//   - If the feature gate is discontinued, it's always disabled.
//   - If the feature gate is in beta, alpha or deprecated phases, then
//     if the feature gate is in the list, it's enabled if its state
//     is missing, or equal to "Enabled".
//     if the feature gate is not in the list, then the default state is used:
//     alpha and deprecated feature gates are disabled by default
//     beta feature gates are enabled by default
func (fgs *HyperConvergedFeatureGates) IsEnabled(name string) bool {
	phase, fgExist := featuregatedetails.GetFeatureGatePhase(name)
	if !fgExist { // unsupported feature gate, even if it is in the featureGate list
		return false
	}

	var state State
	switch phase {
	case featuregates.PhaseGA:
		return true
	case featuregates.PhaseBeta:
		state = Enabled
	case featuregates.PhaseAlpha, featuregates.PhaseDeprecated:
		state = Disabled
	default:
		return false
	}

	if idx := fgs.index(name); idx > -1 {
		state = ptr.Deref((*fgs)[idx].State, Enabled)
	}

	return state == Enabled
}

// IsExplicitlyEnabled checks if a feature gate is explicitly set in the feature gate list
func (fgs *HyperConvergedFeatureGates) IsExplicitlyEnabled(name string) (enabled bool, found bool) {
	idx := fgs.index(name)

	if idx < 0 {
		return false, false
	}

	return ptr.Deref((*fgs)[idx].State, Enabled) == Enabled, true
}

func (fgs *HyperConvergedFeatureGates) index(name string) int {
	return slices.IndexFunc(*fgs, func(fg FeatureGate) bool {
		return fg.Name == name
	})
}
