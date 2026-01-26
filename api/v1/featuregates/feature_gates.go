package featuregates

import (
	"encoding/json"
	"slices"
	"strings"

	"k8s.io/utils/ptr"
)

type FeatureGateState string

const (
	Enabled  FeatureGateState = "Enabled"
	Disabled FeatureGateState = "Disabled"
)

// FeatureGate is an optional feature gate to enable or disable a new feature that is not generally available yet.
// +k8s:conversion-gen=false
// +k8s:openapi-gen=true
type FeatureGate struct {
	// Name is the feature gate name
	Name string `json:"name"`

	// State determines if the feature gate is enabled ("Enabled"), or disabled ("False"). The default value is "Disabled".
	// +kubebuilder:validation:Enum="Enabled";"Disabled"
	State *FeatureGateState `json:"state,omitempty"`
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

func (fgs *HyperConvergedFeatureGates) Add(fg FeatureGate) {
	idx := slices.IndexFunc(*fgs, func(item FeatureGate) bool {
		return item.Name == fg.Name
	})

	if idx == -1 {
		*fgs = append(*fgs, fg)
		return
	}

	(*fgs)[idx].State = fg.State
}

func (fgs *HyperConvergedFeatureGates) Enable(name string) {
	fgs.set(name, Enabled)
}

func (fgs *HyperConvergedFeatureGates) Disable(name string) {
	fgs.set(name, Disabled)
}

func (fgs *HyperConvergedFeatureGates) set(name string, enabled FeatureGateState) {
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
	isEnabled, isFinal := featureGatesDetails.isEnabled(name)

	if isFinal {
		return isEnabled
	}

	enabled := Disabled
	if isEnabled {
		enabled = Enabled
	}

	idx := slices.IndexFunc(*fgs, func(fg FeatureGate) bool {
		return fg.Name == name
	})

	if idx > -1 {
		enabled = ptr.Deref((*fgs)[idx].State, Enabled)
	}

	return enabled == Enabled
}
