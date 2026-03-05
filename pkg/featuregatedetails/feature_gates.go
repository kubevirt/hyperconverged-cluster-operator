package featuregatedetails

import (
	_ "embed"
	"encoding/json"
	"iter"
	"maps"
	"slices"

	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/featuregates"
)

//go:embed feature-gates.json
var featureGateJson []byte

var featureGatesDetails map[string]featuregates.FeatureGate

func GetFeatureGatePhase(fgName string) (enabled featuregates.Phase, fgExists bool) {
	fg, ok := featureGatesDetails[fgName]
	if !ok {
		return featuregates.PhaseUnknown, false
	}

	return fg.Phase, true
}

func init() {
	if err := setup(featureGateJson); err != nil {
		panic("unable to setup v1 feature gates;" + err.Error())
	}
}

func setup(fgJson []byte) error {
	var fgs featuregates.FeatureGates
	err := json.Unmarshal(fgJson, &fgs)
	if err != nil {
		return err
	}

	featureGatesDetails = maps.Collect(fgsToMap(slices.Values(fgs)))

	return nil
}

func fgsToMap(fgs iter.Seq[featuregates.FeatureGate]) iter.Seq2[string, featuregates.FeatureGate] {
	return func(yield func(string, featuregates.FeatureGate) bool) {
		for fg := range fgs {
			if !yield(fg.Name, fg) {
				return
			}
		}
	}
}
