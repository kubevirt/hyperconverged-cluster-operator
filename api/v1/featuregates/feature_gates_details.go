package featuregates

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

type featureGatesDetailsType map[string]featureGateDetails

var featureGatesDetails = featureGatesDetailsType{
	"downwardMetrics": {
		phase:       PhaseAlpha,
		description: "Allow to expose a limited set of host metrics to guests.",
	},
	"withHostPassthroughCPU": {
		phase: PhaseDeprecated,
	},
	"enableCommonBootImageImport": {
		phase: PhaseDeprecated,
		description: "This feature gate is ignored. Use spec.enableCommonBootImageImport field instead",
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
		description: "Enable persistent reservation of a LUN through the SCSI Persistent Reserve commands on Kubevirt." +
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
		description: "Enable KubeVirt to request up to two additional dedicated CPUs in order to complete the total CPU count to an even parity when using emulator thread isolation. " +
			"Note: this feature is in Developer Preview.",
	},
	"enableApplicationAwareQuota": {
		phase: PhaseDeprecated,
		description: "This feature gate is ignored. Use spec.enableApplicationAwareQuota field instead",
	},
	"primaryUserDefinedNetworkBinding": {
		phase: PhaseDeprecated,
	},
	"enableMultiArchBootImageImport": {
		phase: PhaseAlpha,
		description: "allows the HCO to run on heterogeneous clusters with different CPU architectures. " +
			"Enabling this feature gate will allow the HCO to create Golden Images for different CPU architectures.",
	},
	"decentralizedLiveMigration": {
		phase: PhaseAlpha,
		description: "enables the decentralized live migration (cross-cluster migration) feature." +
			"This feature allows live migration of VirtualMachineInstances between different clusters. " +
			"Note: This feature is in Developer Preview.",
	},
	"declarativeHotplugVolumes": {
		phase: PhaseAlpha,
		description: "enables the use of the declarative volume hotplug feature in KubeVirt. " +
			`When enabled, the "DeclarativeHotplugVolumes" feature gate is enabled in the KubeVirt CR, instead of the "HotplugVolumes" feature gate. ` +
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
			"This subresource returns a structured list of k8s objects that are related to the specified VM or VMI, enabling better dependency tracking.",
	},
}

func (fgs featureGatesDetailsType) isEnabled(name string) (isEnabled bool, isFinal bool) {
	details, ok := fgs[name]
	if !ok { // unsupported feature gate, even if it is in the featureGate list
		return false, true
	}

	switch details.phase {
	case PhaseGA:
		isFinal = true
		isEnabled = true
	case PhaseDeprecated:
		isFinal = true
		isEnabled = false
	case PhaseAlpha:
		isFinal = false
		isEnabled = false
	case PhaseBeta:
		isFinal = false
		isEnabled = true
	default:
		isFinal = true
		isEnabled = false
	}

	return isEnabled, isFinal
}