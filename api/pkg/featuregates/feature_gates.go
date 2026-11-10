package featuregates

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"iter"
	"maps"
	"slices"
	"strings"
)

//go:embed feature-gates.json
var featureGateJson []byte

var (
	featureGatesDetails      map[string]FeatureGate
	configurableFeatureGates []FeatureGate
)

func GetFeatureGatePhase(fgName string) (enabled Phase, fgExists bool) {
	fgName = strings.ToLower(fgName)
	fg, ok := featureGatesDetails[fgName]
	if !ok {
		return PhaseUnknown, false
	}

	return fg.Phase, true
}

func ListBetaFeatureGates() []string {
	return slices.Sorted(filterFGByPhase(maps.All(featureGatesDetails), PhaseBeta))
}

func ListAlphaFeatureGates() []string {
	return slices.Sorted(filterFGByPhase(maps.All(featureGatesDetails), PhaseAlpha))
}

// ListConfigurableFeatureGates returns every catalogued feature gate that can
// still be configured through spec.featureGates (alpha and beta), sorted by name.
func ListConfigurableFeatureGates() []FeatureGate {
	return slices.Clone(configurableFeatureGates)
}

func buildConfigurableList() []FeatureGate {
	configurable := make([]FeatureGate, 0, len(featureGatesDetails))
	for _, fg := range featureGatesDetails {
		if fg.Phase.IsConfigurable() {
			configurable = append(configurable, fg)
		}
	}

	slices.SortFunc(configurable, func(a, b FeatureGate) int {
		return strings.Compare(a.Name, b.Name)
	})

	return configurable
}

func init() {
	if err := setup(featureGateJson); err != nil {
		panic("unable to setup v1 feature gates;" + err.Error())
	}
}

func setup(fgJson []byte) error {
	var fgs FeatureGates
	if len(fgJson) == 0 {
		return fmt.Errorf("the api/pkg/featuregates/feature_gates.json file is empty or missing")
	}
	err := json.Unmarshal(fgJson, &fgs)
	if err != nil {
		return err
	}

	featureGatesDetails = maps.Collect(fgsToMap(slices.Values(fgs)))
	configurableFeatureGates = buildConfigurableList()

	return nil
}

func fgsToMap(fgs iter.Seq[FeatureGate]) iter.Seq2[string, FeatureGate] {
	return func(yield func(string, FeatureGate) bool) {
		for fg := range fgs {
			if !yield(strings.ToLower(fg.Name), fg) {
				return
			}
		}
	}
}

func filterFGByPhase(fgs iter.Seq2[string, FeatureGate], phase Phase) iter.Seq[string] {
	return func(yield func(string) bool) {
		for _, fg := range fgs {
			if fg.Phase == phase {
				if !yield(fg.Name) {
					return
				}
			}
		}
	}
}
