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

func (fgs *HyperConvergedFeatureGates) Enable(name string) {
	fgs.set(name, Enabled)
}

func (fgs *HyperConvergedFeatureGates) Disable(name string) {
	fgs.set(name, Disabled)
}

func (fgs *HyperConvergedFeatureGates) set(name string, enabled State) {
	idx := slices.IndexFunc(*fgs, func(item FeatureGate) bool {
		return item.Name == name
	})

	if idx == -1 {
		*fgs = append(*fgs, FeatureGate{Name: name, State: &enabled})
		return
	}

	(*fgs)[idx].State = &enabled
}

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
	case featuregates.PhaseAlpha:
		state = Disabled
	default:
		return false
	}

	idx := slices.IndexFunc(*fgs, func(fg FeatureGate) bool {
		return fg.Name == name
	})

	if idx > -1 {
		state = ptr.Deref((*fgs)[idx].State, Enabled)
	}

	return state == Enabled
}
