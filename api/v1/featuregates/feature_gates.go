package featuregates

import (
	"encoding/json"
	"slices"
	"strings"

	"k8s.io/utils/ptr"
)

type Enablement string

const (
	Enabled  Enablement = "True"
	Disabled Enablement = "False"
)

// FeatureGate is an optional feature gate to enable or disable new feature that is not enabled
// by generally available yet.
// +k8s:conversion-gen=false
// +k8s:openapi-gen=true
type FeatureGate struct {
	Name string `json:"name"`
	// +kubebuilder:validation:Enum=True;False
	Enabled *Enablement `json:"enabled,omitempty"`
}

func (fg FeatureGate) MarshalJSON() ([]byte, error) {
	builder := &strings.Builder{}
	builder.WriteString(`{"name":"`)
	builder.WriteString(fg.Name)
	builder.WriteByte('"')

	if fg.Enabled != nil && *fg.Enabled == Disabled {
		builder.WriteString(`,"enabled":"`)
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

	if fg.Enabled == nil {
		fg.Enabled = ptr.To(Enabled)
	}

	return nil
}

// HyperConvergedFeatureGates is a set of optional feature gates to enable or disable new features that are not enabled
// by default yet.
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

	(*fgs)[idx].Enabled = fg.Enabled
}

func (fgs *HyperConvergedFeatureGates) Enable(name string) {
	fgs.set(name, Enabled)
}

func (fgs *HyperConvergedFeatureGates) Disable(name string) {
	fgs.set(name, Disabled)
}

func (fgs *HyperConvergedFeatureGates) set(name string, enabled Enablement) {
	idx := slices.IndexFunc(*fgs, func(item FeatureGate) bool {
		return item.Name == name
	})

	if idx == -1 {
		*fgs = append(*fgs, FeatureGate{Name: name, Enabled: &enabled})
		return
	}

	(*fgs)[idx].Enabled = &enabled
}

func (fgs *HyperConvergedFeatureGates) IsEnabled(name string) bool {
	details, ok := featureGatesDetails[name]
	if !ok { // unsupported feature gate, even if it is in the featureGate list
		return false
	}

	var isEnabled Enablement
	switch details.phase {
	case PhaseGA:
		return true
	case PhaseDeprecated:
		return false
	case PhaseAlpha:
		isEnabled = Disabled
	case PhaseBeta:
		isEnabled = Enabled
	default:
		return false
	}

	idx := slices.IndexFunc(*fgs, func(fg FeatureGate) bool {
		return fg.Name == name
	})

	if idx > -1 {
		isEnabled = ptr.Deref((*fgs)[idx].Enabled, Enabled)
	}

	return isEnabled == Enabled
}

type Phase int

const (
	UnknownFeatureGate Phase = iota
	PhaseAlpha
	PhaseBeta
	PhaseGA
	PhaseDeprecated
)

type featureGateDetails struct {
	phase       Phase
	description string
}

var featureGatesDetails = map[string]featureGateDetails{
	"downwardMetrics": {
		phase:       PhaseAlpha,
		description: "Allow to expose a limited set of host metrics to guests.",
	},
	"withHostPassthroughCPU": {
		phase: PhaseDeprecated,
	},
	"enableCommonBootImageImport": {
		phase:       PhaseDeprecated,
		description: "This field is ignored. Use spec.enableCommonBootImageImport instead",
	},
	"deployTektonTaskResources": {
		phase: PhaseDeprecated,
	},
	"deployVmConsoleProxy": {
		phase: PhaseDeprecated,
	},
	"deployKubeSecondaryDNS": {
		phase:       PhaseAlpha,
		description: "Deploy KubeSecondaryDNS by CNAO",
	},
	"deployKubevirtIpamController": {
		phase: PhaseDeprecated,
	},
	"nonRoot": {
		phase: PhaseDeprecated,
	},
	"disableMDevConfiguration": {
		phase:       PhaseAlpha,
		description: "Disable mediated devices handling on KubeVirt",
	},
	"persistentReservation": {
		phase: PhaseAlpha,
		description: "Enable persistent reservation of a LUN through the SCSI Persistent Reserve commands on Kubevirt. " +
			"In order to issue privileged SCSI ioctls, the VM requires activation of the persistent reservation flag. " +
			"Once this feature gate is enabled, then the additional container with the qemu-pr-helper is deployed inside the virt-handler pod. " +
			"Enabling (or removing) the feature gate causes the redeployment of the virt-handler pod.",
	},
	"enableManagedTenantQuota": {
		phase: PhaseDeprecated,
	},
	"autoResourceLimits": {
		phase: PhaseDeprecated,
	},
	"alignCPUs": {
		phase: PhaseAlpha,
		description: "Enable KubeVirt to request up to two additional dedicated CPUs " +
			"in order to complete the total CPU count to an even parity when using emulator thread isolation. " +
			"Note: this feature is in Developer Preview.",
	},
	"enableApplicationAwareQuota": {
		phase: PhaseDeprecated,
		description: "This featureGate ignored and will be removed on the next version of the API. " +
			"Use spec.enableApplicationAwareQuota instead",
	},
	"primaryUserDefinedNetworkBinding": {
		phase: PhaseDeprecated,
	},
	"enableMultiArchBootImageImport": {
		phase: PhaseAlpha,
		description: "allows the HCO to run on heterogeneous clusters with different CPU architectures. " +
			"Setting this field to true will allow the HCO to create Golden Images for different CPU architectures.",
	},
	"decentralizedLiveMigration": {
		phase: PhaseAlpha,
		description: "enables the decentralized live migration (cross-cluster migration) feature. " +
			"This feature allows live migration of VirtualMachineInstances between different clusters. " +
			"This feature is in Developer Preview.",
	},
	"declarativeHotplugVolumes": {
		phase: PhaseAlpha,
		description: "enables the use of the declarative volume hotplug feature in KubeVirt. " +
			`When enabled, the "DeclarativeHotplugVolumes" feature gate is enabled in the KubeVirt CR, instead of ` +
			`"HotplugVolumes". ` +
			`When disabled, the "HotplugVolumes" feature gate is enabled (default behavior).`,
	},
	"videoConfig": {
		phase: PhaseBeta,
		description: "allows users to configure video device types for their virtual machines. " +
			"This can be useful for workloads that require specific video capabilities or architectures.",
	},
	"objectGraph": {
		phase: PhaseAlpha,
		description: "enables the ObjectGraph VM and VMI subresource in KubeVirt. " +
			"This subresource returns a structured list of k8s objects that are related to the specified " +
			"VM or VMI, enabling better dependency tracking.",
	},
}
