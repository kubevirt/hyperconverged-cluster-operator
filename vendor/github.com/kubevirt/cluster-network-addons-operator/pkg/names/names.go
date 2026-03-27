package names

// OperatorConfig is the name of the CRD that defines the complete
// operator configuration
const OperatorConfig = "cluster"

// AppliedPrefix is the prefix applied to the config maps
// where we store previously applied configuration
const AppliedPrefix = "cluster-networks-addons-operator-applied-"

// RejectOwnerAnnotation can be set on objects under data/ that should not be
// assigned with NetworkAddonsConfig as their owner. This can be used to prevent
// garbage collection deletion upon NetworkAddonsConfig removal.
const RejectOwnerAnnotation = "networkaddonsoperator.network.kubevirt.io/rejectOwner"

const PrometheusLabelKey = "prometheus.cnao.io"
const KubeMacPoolPrometheusLabelKey = "prometheus.kubemacpool.io"

const PrometheusLabelValueTrue = "true"
const PrometheusLabelValueFalse = "false"

// Relationship labels

const ComponentLabelKey = "app.kubernetes.io/component"
const PartOfLabelKey = "app.kubernetes.io/part-of"
const VersionLabelKey = "app.kubernetes.io/version"
const ManagedByLabelKey = "app.kubernetes.io/managed-by"
const ComponentLabelDefaultValue = "network"
const ManagedByLabelDefaultValue = "cnao-operator"

const KubemacpoolControlPlaneKey = "control-plane"
const KubemacpoolMacControllerManagerValue = "mac-controller-manager"
