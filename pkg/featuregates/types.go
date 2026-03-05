package featuregates

import (
	"encoding/json"
	"fmt"
	"slices"
)

// Phase represents the lifecycle phase of a feature gate.
type Phase int

const (
	PhaseUnknown Phase = iota
	PhaseGA
	PhaseBeta
	PhaseAlpha
	PhaseDeprecated
)

func (p Phase) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%q", p.String())), nil
}

func (p *Phase) UnmarshalJSON(bytes []byte) error {
	var phase string
	if err := json.Unmarshal(bytes, &phase); err != nil {
		return err
	}

	switch phase {
	case "GA":
		*p = PhaseGA
	case "beta":
		*p = PhaseBeta
	case "alpha":
		*p = PhaseAlpha
	case "deprecated":
		*p = PhaseDeprecated
	default:
		*p = PhaseUnknown
	}

	return nil
}

func (p Phase) String() string {
	switch p {
	case PhaseGA:
		return "GA"
	case PhaseBeta:
		return "beta"
	case PhaseAlpha:
		return "alpha"
	case PhaseDeprecated:
		return "deprecated"
	default:
		return "unknown"
	}
}

// FeatureGate represents a single feature gate entry.
type FeatureGate struct {
	Name        string `json:"name"`
	Phase       Phase  `json:"phase"`
	Description string `json:"description,omitempty"`
}

type FeatureGates []FeatureGate

func (fgs FeatureGates) Sort() {
	slices.SortFunc(fgs, func(a, b FeatureGate) int {
		if delta := a.Phase - b.Phase; delta != 0 {
			return int(delta)
		}

		if a.Name < b.Name {
			return -1
		}

		return 1
	})

}
