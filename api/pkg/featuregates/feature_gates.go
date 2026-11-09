package featuregates

import (
	_ "embed"
	"encoding/json"
	"iter"
	"maps"
	"slices"
	"strings"
)

//go:embed feature-gates.json
var featureGateJson []byte

var featureGatesDetails map[string]FeatureGate

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

func init() {
	if err := setup(featureGateJson); err != nil {
		panic("unable to setup v1 feature gates;" + err.Error())
	}
}

func setup(fgJson []byte) error {
	var fgs FeatureGates
	err := json.Unmarshal(fgJson, &fgs)
	if err != nil {
		return err
	}

	featureGatesDetails = maps.Collect(fgsToMap(slices.Values(fgs)))

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
