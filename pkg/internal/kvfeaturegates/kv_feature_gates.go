package kvfeaturegates

import (
	_ "embed"
	"encoding/json"
	"slices"
)

//go:embed kv-beta-feature-gates.json
var kvBetaFeatureGatesJSON []byte

type featureGateEntry struct {
	Name  string `json:"name"`
	State string `json:"state"`
}

var kvBetaFeatureGates []string

func init() {
	var kvFG []featureGateEntry
	if err := json.Unmarshal(kvBetaFeatureGatesJSON, &kvFG); err != nil {
		panic("failed to parse kv-beta-feature-gates.json: " + err.Error())
	}

	for _, fg := range kvFG {
		if fg.State != "Beta" {
			continue
		}

		kvBetaFeatureGates = append(kvBetaFeatureGates, fg.Name)
	}

	slices.Sort(kvBetaFeatureGates)
}

func GetBetaFeatureGates() []string {
	return slices.Clone(kvBetaFeatureGates)
}
