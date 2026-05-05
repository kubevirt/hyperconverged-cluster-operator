package handlers

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"

	openshiftconfigv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/library-go/pkg/crypto"
	corev1 "k8s.io/api/core/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubevirtcorev1 "kubevirt.io/api/core/v1"
	"kubevirt.io/controller-lifecycle-operator-sdk/api"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/handlers/aie"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/kvfeaturegates"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/nodeinfo"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/patch"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/reformatobj"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/tlssecprofile"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

// env vars
const (
	kvmEmulationEnvName     = "KVM_EMULATION"
	smbiosEnvName           = "SMBIOS"
	machineTypeEnvName      = "MACHINETYPE"
	amd64MachineTypeEnvName = "AMD64_MACHINETYPE"
	arm64MachineTypeEnvName = "ARM64_MACHINETYPE"
	s390xMachineTypeEnvName = "S390X_MACHINETYPE"
)

const (
	DefaultAMD64OVMFPath             = "/usr/share/OVMF"
	DefaultARM64OVMFPath             = "/usr/share/AAVMF"
	DefaultS390xOVMFPath             = ""
	DefaultAMD64EmulatedQ35Machine   = "q35*"
	DefaultAMD64EmulatedPCQ35Machine = "pc-q35*"
	DefaultARM64EmulatedMachines     = "virt*"
	DefaultS390XEmulatedMachines     = "s390-ccw-virtio*"
)

const (
	primaryUDNNetworkBindingName = "l2bridge"
	deployPasstNetworkBindingAnn = hcoutil.HCOAnnotationPrefix + "deployPasstNetworkBinding"
)

const kvPriorityClass = "kubevirt-cluster-critical"

var (
	useKVMEmulation = false
)

func init() {
	kvmEmulationStr, varExists := os.LookupEnv(kvmEmulationEnvName)
	if varExists {
		isKVMEmulation, err := strconv.ParseBool(strings.ToLower(kvmEmulationStr))
		useKVMEmulation = err == nil && isKVMEmulation
	}

	mandatoryKvFeatureGates = getMandatoryKvFeatureGates(useKVMEmulation)
}

// KubeVirt hard coded FeatureGates
// These feature gates are set by HCO in the KubeVirt CR and can't be modified by the end user.
const (
	// Enables the CPUManager feature gate to label the nodes which have the Kubernetes CPUManager running. VMIs that
	// require dedicated CPU resources will automatically be scheduled on the labeled nodes
	kvCPUManagerGate = "CPUManager"

	// Enables the alpha offline snapshot functionality
	kvSnapshotGate = "Snapshot"

	// Allow attaching a data volume to a running VMI
	kvHotplugVolumesGate = "HotplugVolumes"

	// Allow attaching a data volume to a running VMI using declarative API
	kvDeclarativeHotplugVolumesGate = "DeclarativeHotplugVolumes"

	// Allow assigning host devices to virtual machines
	kvHostDevicesGate = "HostDevices"

	// Expand disks to the largest size
	kvExpandDisksGate = "ExpandDisks" // todo this FG is now GA in KV. Remove it after bumping KV to 1.9

	// Export VMs to outside of the cluster
	kvVMExportGate = "VMExport" // todo this FG is now GA in KV. Remove it after bumping KV to 1.9

	// Enable the installation of the KubeVirt seccomp profile
	kvKubevirtSeccompProfile = "KubevirtSeccompProfile"
)

// KubeVirt architecture dependant feature gates.
// These feature gates are set by HCO in the KubeVirt CR and can't be modified by the end user.
const (
	// Allow running IBM Z Secure Execution VMs
	kvSecureExecution = "SecureExecution"
)

var (
	hardCodeKvFgs = []string{
		kvCPUManagerGate,
		kvSnapshotGate,
		kvExpandDisksGate,
		kvHostDevicesGate,
		kvVMExportGate,
		kvKubevirtSeccompProfile,
	}

	// holds a list of mandatory KubeVirt feature gates. Some of them are the hard coded feature gates and some of
	// them are added according to conditions; e.g. if SSP is deployed.
	mandatoryKvFeatureGates []string
)

// These KubeVirt feature gates are automatically enabled in KubeVirt, unless emulation is enabled
const (
	// Enable HyperV strict host checking for HyperV enlightenments
	kvHypervStrictCheck = "HypervStrictCheck"
)

// KubeVirt feature gates that are exposed in HCO API
const (
	kvDownwardMetrics            = "DownwardMetrics"
	kvPersistentReservation      = "PersistentReservation"
	kvAlignCPUs                  = "AlignCPUs"
	kvDecentralizedLiveMigration = "DecentralizedLiveMigration"
	kvVideoConfig                = "VideoConfig"
	kvObjectGraph                = "ObjectGraph"
	kvUtilityVolumes             = "UtilityVolumes"
	kvIncrementalBackup          = "IncrementalBackup"
	kvPasstBinding               = "PasstBinding"
	kvConfigurableHypervisor     = "ConfigurableHypervisor"
	kvOptOutRoleAggregation      = "OptOutRoleAggregation"
	kvContainerPathVolumes       = "ContainerPathVolumes"
	kvPCINUMAAwareTopology       = "PCINUMAAwareTopology"
)

// CPU Plugin default values
var (
	hardcodedObsoleteCPUModels = []string{
		"486",
		"486-v1",
		"pentium",
		"pentium-v1",
		"pentium2",
		"pentium2-v1",
		"pentium3",
		"pentium3-v1",
		"pentiumpro",
		"pentiumpro-v1",
		"coreduo",
		"coreduo-v1",
		"n270",
		"n270-v1",
		"core2duo",
		"core2duo-v1",
		"Conroe",
		"Conroe-v1",
		"athlon",
		"athlon-v1",
		"phenom",
		"phenom-v1",
		"qemu64",
		"qemu64-v1",
		"qemu32",
		"qemu32-v1",
		"kvm64",
		"kvm64-v1",
		"kvm32",
		"kvm32-v1",
		"Opteron_G1",
		"Opteron_G1-v1",
		"Opteron_G2",
		"Opteron_G2-v1",
	}
)

// KubeVirt containerDisk verification memory usage limit
var (
	kvDiskVerificationMemoryLimit = resource.MustParse("2G")
)

// ************  KubeVirt Handler  **************
func NewKubevirtHandler(Client client.Client, Scheme *runtime.Scheme) *operands.GenericOperand {
	return operands.NewGenericOperand(Client, Scheme, "KubeVirt", &kubevirtHooks{}, true)
}

type kubevirtHooks struct {
	sync.Mutex
	cache *kubevirtcorev1.KubeVirt
}

type rateLimits struct {
	QPS   float32 `json:"qps"`
	Burst int     `json:"burst"`
}

func (h *kubevirtHooks) GetFullCr(hc *hcov1.HyperConverged) (client.Object, error) {
	h.Lock()
	defer h.Unlock()

	if h.cache == nil {
		kv, err := NewKubeVirt(hc)
		if err != nil {
			return nil, err
		}
		h.cache = kv
	}
	return h.cache, nil
}

func (*kubevirtHooks) GetEmptyCr() client.Object { return &kubevirtcorev1.KubeVirt{} }
func (*kubevirtHooks) GetConditions(cr runtime.Object) []metav1.Condition {
	return translateKubeVirtConds(cr.(*kubevirtcorev1.KubeVirt).Status.Conditions)
}
func (*kubevirtHooks) CheckComponentVersion(cr runtime.Object) bool {
	found := cr.(*kubevirtcorev1.KubeVirt)
	return operands.CheckComponentVersion(hcoutil.KubevirtVersionEnvV, found.Status.ObservedKubeVirtVersion)
}
func (h *kubevirtHooks) Reset() {
	h.Lock()
	defer h.Unlock()
	h.cache = nil
}

func (*kubevirtHooks) UpdateCR(req *common.HcoRequest, Client client.Client, exists runtime.Object, required runtime.Object) (bool, bool, error) {
	virt, ok1 := required.(*kubevirtcorev1.KubeVirt)
	found, ok2 := exists.(*kubevirtcorev1.KubeVirt)
	if !ok1 || !ok2 {
		return false, false, errors.New("can't convert to KubeVirt")
	}

	if !reflect.DeepEqual(found.Spec, virt.Spec) ||
		!hcoutil.CompareLabels(virt, found) ||
		!isAnnotationStateMeetingRequirements(virt.Annotations, found.Annotations) {
		if req.HCOTriggered {
			req.Logger.Info("Updating existing KubeVirt's Spec to new opinionated values")
		} else {
			req.Logger.Info("Reconciling an externally updated KubeVirt's Spec to its opinionated values")
		}
		hcoutil.MergeLabels(&virt.ObjectMeta, &found.ObjectMeta)
		setAnnotationsToReqState(req.Instance, found)
		virt.Spec.DeepCopyInto(&found.Spec)
		err := Client.Update(req.Ctx, found)
		if err != nil {
			return false, false, err
		}
		return true, !req.HCOTriggered, nil
	}
	return false, false, nil
}

func NewKubeVirt(hc *hcov1.HyperConverged, opts ...string) (*kubevirtcorev1.KubeVirt, error) {
	config, err := getKVConfig(hc)
	if err != nil {
		return nil, err
	}

	kvCertConfig := hcoCertConfig2KvCertificateRotateStrategy(hc.Spec.Security.CertConfig)

	controlPlaneHighlyAvailable := nodeinfo.IsControlPlaneHighlyAvailable()
	controlPlaneNodeExists := nodeinfo.IsControlPlaneNodeExists()
	infraHighlyAvailable := nodeinfo.IsInfrastructureHighlyAvailable()

	uninstallStrategy := kubevirtcorev1.KubeVirtUninstallStrategyBlockUninstallIfWorkloadsExist
	if hc.Spec.Deployment.UninstallStrategy == hcov1.HyperConvergedUninstallStrategyRemoveWorkloads {
		uninstallStrategy = kubevirtcorev1.KubeVirtUninstallStrategyRemoveWorkloads
	}

	var infra, workload *api.NodePlacement
	if np := hc.Spec.Deployment.NodePlacements; np != nil {
		infra = np.Infra
		workload = np.Workload
	}
	kvInfra := hcoConfig2KvConfig(infra, infraHighlyAvailable, controlPlaneHighlyAvailable, controlPlaneNodeExists)
	kvWorkloads := hcoConfig2KvConfig(workload, true, true, true)

	spec := kubevirtcorev1.KubeVirtSpec{
		UninstallStrategy:           uninstallStrategy,
		Infra:                       kvInfra,
		Workloads:                   kvWorkloads,
		Configuration:               *config,
		CertificateRotationStrategy: *kvCertConfig,
		WorkloadUpdateStrategy:      hcWorkloadUpdateStrategyToKv(&hc.Spec.Virtualization.WorkloadUpdateStrategy),
		ProductName:                 hcoutil.HyperConvergedCluster,
		ProductVersion:              os.Getenv(hcoutil.HcoKvIoVersionName),
		ProductComponent:            string(hcoutil.AppComponentCompute),
		ServiceMonitorNamespace:     operands.GetNamespace(hc.Namespace, opts),
	}

	kv := NewKubeVirtWithNameOnly()
	setAnnotationsToReqState(hc, kv)
	kv.Spec = spec

	if err = operands.ApplyPatchToSpec(hc, common.JSONPatchKVAnnotationName, kv); err != nil {
		return nil, err
	}

	return reformatobj.ReformatObj(kv)
}

func isAnnotationStateMeetingRequirements(requiredAnnotations, actualAnnotations map[string]string) bool {
	_, isRequired := requiredAnnotations[kubevirtcorev1.EmulatorThreadCompleteToEvenParity]
	_, exists := actualAnnotations[kubevirtcorev1.EmulatorThreadCompleteToEvenParity]

	return isRequired == exists
}

func setAnnotationsToReqState(hc *hcov1.HyperConverged, kv *kubevirtcorev1.KubeVirt) {
	if kv.Annotations == nil {
		kv.Annotations = map[string]string{}
	}

	if hc.Spec.FeatureGates.IsEnabled("alignCPUs") {
		kv.Annotations[kubevirtcorev1.EmulatorThreadCompleteToEvenParity] = ""
	} else {
		delete(kv.Annotations, kubevirtcorev1.EmulatorThreadCompleteToEvenParity)
	}
}

func getHcoAnnotationTuning(hc *hcov1.HyperConverged) (*kubevirtcorev1.ReloadableComponentConfiguration, error) {
	annotation, ok := hc.Annotations[common.TuningPolicyAnnotationName]
	if !ok {
		return nil, fmt.Errorf("tuning policy set but annotation not present or wrong")
	}

	var rates rateLimits
	err := json.Unmarshal([]byte(annotation), &rates)
	if err != nil {
		return nil, err
	}

	if rates.QPS <= 0 {
		return nil, fmt.Errorf("qps parameter not found in annotation")
	}
	if rates.Burst <= 0 {
		return nil, fmt.Errorf("burst parameter not found in annotation")
	}

	return &kubevirtcorev1.ReloadableComponentConfiguration{
		RestClient: &kubevirtcorev1.RESTClientConfiguration{
			RateLimiter: &kubevirtcorev1.RateLimiter{
				TokenBucketRateLimiter: &kubevirtcorev1.TokenBucketRateLimiter{
					QPS:   rates.QPS,
					Burst: rates.Burst,
				},
			},
		},
	}, nil
}

func hcoTuning2Kv(hc *hcov1.HyperConverged) (*kubevirtcorev1.ReloadableComponentConfiguration, error) {
	if hc.Spec.Virtualization.TuningPolicy == hcov1.HyperConvergedAnnotationTuningPolicy {
		return getHcoAnnotationTuning(hc)
	}

	return nil, nil
}

func hcWorkloadUpdateStrategyToKv(hcObject *hcov1.HyperConvergedWorkloadUpdateStrategy) kubevirtcorev1.KubeVirtWorkloadUpdateStrategy {
	kvObject := kubevirtcorev1.KubeVirtWorkloadUpdateStrategy{}
	if hcObject != nil {
		if hcObject.BatchEvictionInterval != nil {
			kvObject.BatchEvictionInterval = new(metav1.Duration)
			*kvObject.BatchEvictionInterval = *hcObject.BatchEvictionInterval
		}

		if hcObject.BatchEvictionSize != nil {
			kvObject.BatchEvictionSize = new(int)
			*kvObject.BatchEvictionSize = *hcObject.BatchEvictionSize
		}

		if size := len(hcObject.WorkloadUpdateMethods); size > 0 {
			kvObject.WorkloadUpdateMethods = make([]kubevirtcorev1.WorkloadUpdateMethod, size)
			for i, updateMethod := range hcObject.WorkloadUpdateMethods {
				kvObject.WorkloadUpdateMethods[i] = kubevirtcorev1.WorkloadUpdateMethod(updateMethod)
			}
		}
	}

	return kvObject
}

func getKVConfig(hc *hcov1.HyperConverged) (*kubevirtcorev1.KubeVirtConfiguration, error) {
	devConfig := getKVDevConfig(hc)

	kvLiveMigration, err := hcLiveMigrationToKv(hc.Spec.Virtualization.LiveMigrationConfig)
	if err != nil {
		return nil, err
	}

	obsoleteCPUs := getObsoleteCPUConfig(hc.Spec.Virtualization.ObsoleteCPUModels)

	rateLimiter, err := hcoTuning2Kv(hc)
	if err != nil {
		return nil, err
	}

	seccompConfig := getKVSeccompConfig()

	networkBindings := getNetworkBindings(hc.Spec.Networking)

	config := &kubevirtcorev1.KubeVirtConfiguration{
		DeveloperConfiguration: devConfig,
		NetworkConfiguration: &kubevirtcorev1.NetworkConfiguration{
			NetworkInterface: string(kubevirtcorev1.MasqueradeInterface),
			Binding:          networkBindings,
		},
		MigrationConfiguration:             kvLiveMigration,
		PermittedHostDevices:               toKvPermittedHostDevices(hc.Spec.Virtualization.PermittedHostDevices),
		MediatedDevicesConfiguration:       toKvMediatedDevicesConfiguration(hc),
		ObsoleteCPUModels:                  obsoleteCPUs,
		TLSConfiguration:                   hcTLSSecurityProfileToKv(tlssecprofile.GetTLSSecurityProfile(hc.Spec.Security.TLSSecurityProfile)),
		APIConfiguration:                   rateLimiter,
		WebhookConfiguration:               rateLimiter,
		ControllerConfiguration:            rateLimiter,
		HandlerConfiguration:               rateLimiter,
		SeccompConfiguration:               seccompConfig,
		EvictionStrategy:                   hc.Spec.Virtualization.EvictionStrategy,
		KSMConfiguration:                   hc.Spec.Virtualization.KSMConfiguration,
		ChangedBlockTrackingLabelSelectors: hc.Spec.Virtualization.ChangedBlockTrackingLabelSelectors,
		VMRolloutStrategy:                  ptr.To(kubevirtcorev1.VMRolloutStrategyLiveUpdate),
		LiveUpdateConfiguration:            hc.Spec.Virtualization.LiveUpdateConfiguration,
		ArchitectureConfiguration:          getArchConfiguration(),
	}

	smbiosConfig, err := getSMBConfig()
	if err != nil {
		return nil, err
	}
	config.SMBIOSConfig = smbiosConfig

	hcoVmOptionsToKV(hc.Spec.Virtualization.VirtualMachineOptions, config)

	if hc.Spec.Storage != nil {
		config.VMStateStorageClass = ptr.Deref(hc.Spec.Storage.VMStateStorageClass, "")
	}

	if hc.Spec.Virtualization.AutoCPULimitNamespaceLabelSelector != nil {
		config.AutoCPULimitNamespaceLabelSelector = hc.Spec.Virtualization.AutoCPULimitNamespaceLabelSelector.DeepCopy()
	}

	if hc.Spec.WorkloadSources.InstancetypeConfig != nil {
		config.Instancetype = hc.Spec.WorkloadSources.InstancetypeConfig.DeepCopy()
	}

	if hc.Spec.WorkloadSources.CommonInstancetypesDeployment != nil {
		config.CommonInstancetypesDeployment = hc.Spec.WorkloadSources.CommonInstancetypesDeployment.DeepCopy()
	}

	copyHypervisors(hc.Spec.Virtualization.Hypervisors, config)

	config.RoleAggregationStrategy = hc.Spec.Virtualization.RoleAggregationStrategy

	return config, nil
}

func copyHypervisors(hvs []kubevirtcorev1.HypervisorConfiguration, config *kubevirtcorev1.KubeVirtConfiguration) {
	if len(hvs) == 0 {
		return
	}

	config.Hypervisors = make([]kubevirtcorev1.HypervisorConfiguration, len(hvs))

	for i := range hvs {
		config.Hypervisors[i] = *(hvs[i].DeepCopy())
	}
}

func hcoVmOptionsToKV(vmOpts *hcov1.VirtualMachineOptions, config *kubevirtcorev1.KubeVirtConfiguration) {
	if vmOpts == nil {
		return
	}
	config.CPUModel = ptr.Deref(vmOpts.DefaultCPUModel, "")

	if disableFPR, disableSCL := ptr.Deref(vmOpts.DisableFreePageReporting, false), ptr.Deref(vmOpts.DisableSerialConsoleLog, false); disableFPR || disableSCL {
		config.VirtualMachineOptions = &kubevirtcorev1.VirtualMachineOptions{}
		if disableFPR {
			config.VirtualMachineOptions.DisableFreePageReporting = &kubevirtcorev1.DisableFreePageReporting{}
		}

		if disableSCL {
			config.VirtualMachineOptions.DisableSerialConsoleLog = &kubevirtcorev1.DisableSerialConsoleLog{}
		}
	}

	config.DefaultRuntimeClass = ptr.Deref(vmOpts.DefaultRuntimeClass, "")
}

var (
	archConfigOnce            = &sync.Once{}
	architectureConfiguration *kubevirtcorev1.ArchConfiguration
)

func getArchConfiguration() *kubevirtcorev1.ArchConfiguration {
	archConfigOnce.Do(func() {
		amd64Comfig := getAMD64ArchConfig()
		arm64Config := getARM64ArchConfig()
		s390xConfig := getS390xArchConfig()
		if amd64Comfig == nil && arm64Config == nil && s390xConfig == nil {
			return
		}

		architectureConfiguration = &kubevirtcorev1.ArchConfiguration{
			Amd64: amd64Comfig,
			Arm64: arm64Config,
			S390x: s390xConfig,
		}
	})

	return architectureConfiguration.DeepCopy()
}

func getAMD64ArchConfig() *kubevirtcorev1.ArchSpecificConfiguration {
	amd64MachineType := cmp.Or(
		strings.TrimSpace(os.Getenv(machineTypeEnvName)),
		strings.TrimSpace(os.Getenv(amd64MachineTypeEnvName)),
	)

	if amd64MachineType == "" {
		return nil
	}

	return &kubevirtcorev1.ArchSpecificConfiguration{
		MachineType: amd64MachineType,
		OVMFPath:    DefaultAMD64OVMFPath,
		EmulatedMachines: []string{
			DefaultAMD64EmulatedQ35Machine,
			DefaultAMD64EmulatedPCQ35Machine,
		},
	}
}

func getARM64ArchConfig() *kubevirtcorev1.ArchSpecificConfiguration {
	armMachineType := strings.TrimSpace(os.Getenv(arm64MachineTypeEnvName))
	if armMachineType == "" {
		return nil
	}

	return &kubevirtcorev1.ArchSpecificConfiguration{
		MachineType:      armMachineType,
		OVMFPath:         DefaultARM64OVMFPath,
		EmulatedMachines: []string{DefaultARM64EmulatedMachines},
	}
}

func getS390xArchConfig() *kubevirtcorev1.ArchSpecificConfiguration {
	s390xMachineType := strings.TrimSpace(os.Getenv(s390xMachineTypeEnvName))
	if s390xMachineType == "" {
		return nil
	}

	return &kubevirtcorev1.ArchSpecificConfiguration{
		MachineType:      s390xMachineType,
		OVMFPath:         DefaultS390xOVMFPath,
		EmulatedMachines: []string{DefaultS390XEmulatedMachines},
	}
}

func getNetworkBindings(networkingCfg *hcov1.NetworkingConfig) map[string]kubevirtcorev1.InterfaceBindingPlugin {
	var networkBindings map[string]kubevirtcorev1.InterfaceBindingPlugin
	if networkingCfg != nil {
		networkBindings = maps.Clone(networkingCfg.NetworkBinding)
	}

	if networkBindings == nil {
		networkBindings = make(map[string]kubevirtcorev1.InterfaceBindingPlugin)
	}

	networkBindings[primaryUDNNetworkBindingName] = primaryUserDefinedNetworkBinding()

	return networkBindings
}

func getObsoleteCPUConfig(hcObsoleteCPUModels []string) map[string]bool {
	obsoleteCPUModels := make(map[string]bool)
	for _, cpu := range hardcodedObsoleteCPUModels {
		obsoleteCPUModels[cpu] = true
	}

	for _, cpu := range hcObsoleteCPUModels {
		obsoleteCPUModels[cpu] = true
	}

	return obsoleteCPUModels
}

func toKvMediatedDevicesConfiguration(hc *hcov1.HyperConverged) *kubevirtcorev1.MediatedDevicesConfiguration {
	mdevsConfig := hc.Spec.Virtualization.MediatedDevicesConfiguration
	disabled := hc.Spec.FeatureGates.IsEnabled("disableMDevConfiguration")

	if mdevsConfig == nil && !disabled {
		return nil
	}

	kvMdev := &kubevirtcorev1.MediatedDevicesConfiguration{}

	if mdevsConfig != nil {
		var mediatedDeviceTypes []string
		if len(mdevsConfig.MediatedDeviceTypes) > 0 {
			mediatedDeviceTypes = mdevsConfig.MediatedDeviceTypes
		}

		kvMdev.MediatedDeviceTypes = mediatedDeviceTypes
		kvMdev.NodeMediatedDeviceTypes = toKvNodeMediatedDevicesConfiguration(mdevsConfig.NodeMediatedDeviceTypes)
	}

	if disabled {
		kvMdev.Enabled = ptr.To(false)
	}

	return kvMdev
}

func toKvNodeMediatedDevicesConfiguration(hcoNodeMdevTypesConf []hcov1.NodeMediatedDeviceTypesConfig) []kubevirtcorev1.NodeMediatedDeviceTypesConfig {
	if len(hcoNodeMdevTypesConf) > 0 {
		nodeMdevTypesConf := make([]kubevirtcorev1.NodeMediatedDeviceTypesConfig, 0, len(hcoNodeMdevTypesConf))
		for _, hcoNodeMdevTypeConf := range hcoNodeMdevTypesConf {

			var mediatedDeviceTypes []string
			if len(hcoNodeMdevTypeConf.MediatedDeviceTypes) > 0 {
				mediatedDeviceTypes = hcoNodeMdevTypeConf.MediatedDeviceTypes
			}

			nodeMdevTypesConf = append(nodeMdevTypesConf, kubevirtcorev1.NodeMediatedDeviceTypesConfig{
				NodeSelector:        hcoNodeMdevTypeConf.NodeSelector,
				MediatedDeviceTypes: mediatedDeviceTypes,
			})
		}
		return nodeMdevTypesConf
	}

	return nil
}

func toKvPermittedHostDevices(permittedDevices *hcov1.PermittedHostDevices) *kubevirtcorev1.PermittedHostDevices {
	if permittedDevices == nil {
		return nil
	}

	return &kubevirtcorev1.PermittedHostDevices{
		PciHostDevices:  toKvPciHostDevices(permittedDevices.PciHostDevices),
		MediatedDevices: toKvMediatedDevices(permittedDevices.MediatedDevices),
		USB:             toKvUSBHostDevices(permittedDevices.USBHostDevices),
	}
}

func toKvPciHostDevices(hcoPciHostdevices []hcov1.PciHostDevice) []kubevirtcorev1.PciHostDevice {
	if len(hcoPciHostdevices) > 0 {
		pciHostDevices := make([]kubevirtcorev1.PciHostDevice, 0, len(hcoPciHostdevices))
		for _, hcoPciHostDevice := range hcoPciHostdevices {
			if !hcoPciHostDevice.Disabled {
				pciHostDevices = append(pciHostDevices, kubevirtcorev1.PciHostDevice{
					PCIVendorSelector:        hcoPciHostDevice.PCIDeviceSelector,
					ResourceName:             hcoPciHostDevice.ResourceName,
					ExternalResourceProvider: hcoPciHostDevice.ExternalResourceProvider,
				})
			}
		}

		return pciHostDevices
	}
	return nil
}

func toKvUSBHostDevices(hcoUSBHostdevices []hcov1.USBHostDevice) []kubevirtcorev1.USBHostDevice {
	if len(hcoUSBHostdevices) > 0 {
		usbHostDevices := make([]kubevirtcorev1.USBHostDevice, 0, len(hcoUSBHostdevices))
		for _, hcoUSBHostDevice := range hcoUSBHostdevices {
			if !hcoUSBHostDevice.Disabled {
				kvUSBHostDevice := kubevirtcorev1.USBHostDevice{
					ResourceName:             hcoUSBHostDevice.ResourceName,
					ExternalResourceProvider: hcoUSBHostDevice.ExternalResourceProvider,
				}
				kvUSBHostDevice.Selectors = make([]kubevirtcorev1.USBSelector, 0, len(hcoUSBHostDevice.Selectors))
				for _, selector := range hcoUSBHostDevice.Selectors {
					kvUSBHostDevice.Selectors = append(kvUSBHostDevice.Selectors, kubevirtcorev1.USBSelector{
						Vendor:  selector.Vendor,
						Product: selector.Product,
					})
				}
				usbHostDevices = append(usbHostDevices, kvUSBHostDevice)
			}
		}

		return usbHostDevices
	}
	return nil
}

func toKvMediatedDevices(hcoMediatedDevices []hcov1.MediatedHostDevice) []kubevirtcorev1.MediatedHostDevice {
	if len(hcoMediatedDevices) > 0 {
		mediatedDevices := make([]kubevirtcorev1.MediatedHostDevice, 0, len(hcoMediatedDevices))
		for _, hcoMediatedHostDevice := range hcoMediatedDevices {
			if !hcoMediatedHostDevice.Disabled {
				mediatedDevices = append(mediatedDevices, kubevirtcorev1.MediatedHostDevice{
					MDEVNameSelector:         hcoMediatedHostDevice.MDEVNameSelector,
					ResourceName:             hcoMediatedHostDevice.ResourceName,
					ExternalResourceProvider: hcoMediatedHostDevice.ExternalResourceProvider,
				})
			}
		}

		return mediatedDevices
	}
	return nil
}

func hcLiveMigrationToKv(lm hcov1.LiveMigrationConfigurations) (*kubevirtcorev1.MigrationConfiguration, error) {
	var bandwidthPerMigration *resource.Quantity
	if lm.BandwidthPerMigration != nil {
		bandwidthPerMigrationObject, err := resource.ParseQuantity(*lm.BandwidthPerMigration)
		if err != nil {
			return nil, fmt.Errorf("failed to parse the LiveMigrationConfig.bandwidthPerMigration field; %w", err)
		}
		bandwidthPerMigration = &bandwidthPerMigrationObject
	}

	return &kubevirtcorev1.MigrationConfiguration{
		BandwidthPerMigration:             bandwidthPerMigration,
		CompletionTimeoutPerGiB:           lm.CompletionTimeoutPerGiB,
		ParallelOutboundMigrationsPerNode: lm.ParallelOutboundMigrationsPerNode,
		ParallelMigrationsPerCluster:      lm.ParallelMigrationsPerCluster,
		ProgressTimeout:                   lm.ProgressTimeout,
		Network:                           lm.Network,
		AllowAutoConverge:                 lm.AllowAutoConverge,
		AllowPostCopy:                     lm.AllowPostCopy,
	}, nil
}

func hcTLSProtocolVersionToKv(profileMinVersion openshiftconfigv1.TLSProtocolVersion) kubevirtcorev1.TLSProtocolVersion {
	switch profileMinVersion {
	case openshiftconfigv1.VersionTLS10:
		return kubevirtcorev1.VersionTLS10

	case openshiftconfigv1.VersionTLS11:
		return kubevirtcorev1.VersionTLS11

	case openshiftconfigv1.VersionTLS12:
		return kubevirtcorev1.VersionTLS12

	case openshiftconfigv1.VersionTLS13:
		return kubevirtcorev1.VersionTLS13

	default:
		return kubevirtcorev1.VersionTLS12
	}
}

func hcTLSSecurityProfileToKv(profile *openshiftconfigv1.TLSSecurityProfile) *kubevirtcorev1.TLSConfiguration {
	var profileCiphers []string
	var profileMinVersion openshiftconfigv1.TLSProtocolVersion

	if profile == nil {
		return nil
	}

	if profile.Custom != nil {
		profileCiphers = profile.Custom.Ciphers
		profileMinVersion = profile.Custom.MinTLSVersion
	} else if profile.Modern != nil {
		profileMinVersion = openshiftconfigv1.TLSProfiles[profile.Type].MinTLSVersion
	} else {
		profileCiphers = openshiftconfigv1.TLSProfiles[profile.Type].Ciphers
		profileMinVersion = openshiftconfigv1.TLSProfiles[profile.Type].MinTLSVersion
	}

	return &kubevirtcorev1.TLSConfiguration{
		MinTLSVersion: hcTLSProtocolVersionToKv(profileMinVersion),
		Ciphers:       crypto.OpenSSLToIANACipherSuites(profileCiphers),
	}
}

func getKVDevConfig(hc *hcov1.HyperConverged) *kubevirtcorev1.DeveloperConfiguration {
	devConf := &kubevirtcorev1.DeveloperConfiguration{
		DiskVerification: &kubevirtcorev1.DiskVerification{
			MemoryLimit: &kvDiskVerificationMemoryLimit,
		},
	}

	if hc.Spec.Virtualization.HigherWorkloadDensity != nil {
		devConf.MemoryOvercommit = hc.Spec.Virtualization.HigherWorkloadDensity.MemoryOvercommitPercentage
	}

	fgs := getKvFeatureGateList(hc)
	if len(fgs) > 0 {
		devConf.FeatureGates = fgs
	}
	if disabledFGs := getKvDisabledFeatureGateList(fgs); len(disabledFGs) > 0 {
		devConf.DisabledFeatureGates = disabledFGs
	}
	if useKVMEmulation {
		devConf.UseEmulation = useKVMEmulation
	}
	if lv := hc.Spec.Deployment.LogVerbosityConfig; lv != nil && lv.Kubevirt != nil {
		devConf.LogVerbosity = lv.Kubevirt.DeepCopy()
	}

	devConf.CPUAllocationRatio = ptr.Deref(hc.Spec.Virtualization.VmiCPUAllocationRatio, 0)

	return devConf
}

func primaryUserDefinedNetworkBinding() kubevirtcorev1.InterfaceBindingPlugin {
	return kubevirtcorev1.InterfaceBindingPlugin{
		DomainAttachmentType: kubevirtcorev1.ManagedTap,
		Migration:            &kubevirtcorev1.InterfaceBindingMigration{},
	}
}

// Static for now, could be configured in the HCO CR in the future
func getKVSeccompConfig() *kubevirtcorev1.SeccompConfiguration {
	return &kubevirtcorev1.SeccompConfiguration{
		VirtualMachineInstanceProfile: &kubevirtcorev1.VirtualMachineInstanceProfile{
			CustomProfile: &kubevirtcorev1.CustomProfile{
				LocalhostProfile: ptr.To("kubevirt/kubevirt.json"),
			},
		},
	}
}

func NewKubeVirtWithNameOnly() *kubevirtcorev1.KubeVirt {
	return &kubevirtcorev1.KubeVirt{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubevirt-" + hcoutil.HyperConvergedName,
			Labels:    operands.GetLabels(hcoutil.AppComponentCompute),
			Namespace: hcoutil.GetOperatorNamespaceFromEnv(),
		},
	}
}

func hcoConfig2KvConfig(
	nodePlacement *api.NodePlacement, infraHighlyAvailable, controlPlaneHighlyAvailable, controlPlaneNodeExists bool) *kubevirtcorev1.ComponentConfig {
	if nodePlacement == nil && controlPlaneHighlyAvailable {
		return nil
	}

	kvConfig := &kubevirtcorev1.ComponentConfig{}

	// In case there are no control plane / master nodes in the cluster, we're setting
	// an empty struct for NodePlacement so that kubevirt control plane pods won't have
	// any affinity rules, and they could get scheduled onto worker nodes.
	if nodePlacement == nil && !controlPlaneNodeExists {
		kvConfig.NodePlacement = &kubevirtcorev1.NodePlacement{}
		if !infraHighlyAvailable {
			// if there is only one worker node and no control plane nodes,
			// set the kubevirt control plane replica count to 1.
			kvConfig.Replicas = ptr.To[uint8](1)
		}
		return kvConfig
	}

	if !controlPlaneHighlyAvailable {
		kvConfig.Replicas = ptr.To[uint8](1)
	}

	if nodePlacement == nil {
		return kvConfig
	}

	kvConfig.NodePlacement = &kubevirtcorev1.NodePlacement{}

	if nodePlacement.Affinity != nil {
		kvConfig.NodePlacement.Affinity = &corev1.Affinity{}
		nodePlacement.Affinity.DeepCopyInto(kvConfig.NodePlacement.Affinity)
	}

	if nodePlacement.NodeSelector != nil {
		kvConfig.NodePlacement.NodeSelector = maps.Clone(nodePlacement.NodeSelector)
	}

	for _, hcoTolr := range nodePlacement.Tolerations {
		kvTolr := corev1.Toleration{}
		hcoTolr.DeepCopyInto(&kvTolr)
		kvConfig.NodePlacement.Tolerations = append(kvConfig.NodePlacement.Tolerations, kvTolr)
	}

	return kvConfig
}

func getFeatureGateChecks(hc *hcov1.HyperConverged) []string {
	featureGates := &hc.Spec.FeatureGates
	fgs := make([]string, 0, 2)

	if featureGates.IsEnabled("downwardMetrics") {
		fgs = append(fgs, kvDownwardMetrics)
	}
	if featureGates.IsEnabled("persistentReservation") {
		fgs = append(fgs, kvPersistentReservation)
	}
	if featureGates.IsEnabled("alignCPUs") {
		fgs = append(fgs, kvAlignCPUs)
	}
	if featureGates.IsEnabled("videoConfig") {
		fgs = append(fgs, kvVideoConfig)
	}

	if featureGates.IsEnabled("objectGraph") {
		fgs = append(fgs, kvObjectGraph)
	}

	if featureGates.IsEnabled("decentralizedLiveMigration") {
		fgs = append(fgs, kvDecentralizedLiveMigration)
	}

	// Add the appropriate volume hotplug featuregate based on DeclarativeHotplugVolumes setting
	if featureGates.IsEnabled("declarativeHotplugVolumes") {
		fgs = append(fgs, kvDeclarativeHotplugVolumesGate)
	} else {
		// Default behavior: use the original HotplugVolumes featuregate
		fgs = append(fgs, kvHotplugVolumesGate)
	}

	if hc.Annotations[deployPasstNetworkBindingAnn] == "true" {
		fgs = append(fgs, kvPasstBinding)
	}

	if hc.Annotations[aie.DeployAIEAnnotation] == "true" {
		fgs = append(fgs, kvPCINUMAAwareTopology)
	}

	if slices.Contains(nodeinfo.GetWorkloadsArchitectures(), nodeinfo.S390X) {
		fgs = append(fgs, kvSecureExecution)
	}

	if featureGates.IsEnabled("incrementalBackup") {
		fgs = append(fgs, kvIncrementalBackup)
		fgs = append(fgs, kvUtilityVolumes)
	}

	if len(hc.Spec.Virtualization.Hypervisors) > 0 {
		fgs = append(fgs, kvConfigurableHypervisor)
	}

	if hc.Spec.Virtualization.RoleAggregationStrategy != nil {
		fgs = append(fgs, kvOptOutRoleAggregation)
	}

	if featureGates.IsEnabled("containerPathVolumes") {
		fgs = append(fgs, kvContainerPathVolumes)
	}

	return fgs
}

// ***********  KubeVirt Priority Class  ************
func NewKvPriorityClassHandler(Client client.Client, Scheme *runtime.Scheme) *operands.GenericOperand {
	return operands.NewGenericOperand(Client, Scheme, "PriorityClass", &kvPriorityClassHooks{}, false)
}

type kvPriorityClassHooks struct{}

func (kvPriorityClassHooks) GetFullCr(_ *hcov1.HyperConverged) (client.Object, error) {
	return NewKubeVirtPriorityClass(), nil
}
func (kvPriorityClassHooks) GetEmptyCr() client.Object { return &schedulingv1.PriorityClass{} }

func (kvPriorityClassHooks) UpdateCR(req *common.HcoRequest, Client client.Client, exists runtime.Object, required runtime.Object) (bool, bool, error) {
	pc, ok1 := required.(*schedulingv1.PriorityClass)
	found, ok2 := exists.(*schedulingv1.PriorityClass)
	if !ok1 || !ok2 {
		return false, false, errors.New("can't convert to PriorityClass")
	}

	// at this point we found the object in the cache and we check if something was changed
	specEquals := (pc.Value == found.Value) && (pc.Description == found.Description)
	if (pc.Name == found.Name) && specEquals && hcoutil.CompareLabels(&pc.ObjectMeta, &found.ObjectMeta) {
		return false, false, nil
	}

	if req.HCOTriggered {
		req.Logger.Info("Updating existing PriorityClass's Spec to new opinionated values")
	} else {
		req.Logger.Info("Reconciling an externally updated PriorityClass's Spec to its opinionated values")
	}

	// make sure req labels are in place, while allowing user defined labels
	labels := maps.Clone(found.Labels)
	if labels == nil {
		labels = make(map[string]string)
	}
	if len(pc.Labels) > 0 {
		maps.Copy(labels, pc.Labels)
	}

	if !specEquals {
		// something was changed but since we can't patch a priority class object, we remove it
		err := Client.Delete(req.Ctx, found)
		if err != nil {
			return false, false, err
		}

		// create the new object
		pc.Labels = labels
		err = Client.Create(req.Ctx, pc)
		if err != nil {
			return false, false, err
		}
	} else {
		p, err := getLabelPatch(found.Labels, labels)
		if err != nil {
			return false, false, err
		}

		err = Client.Patch(req.Ctx, found, client.RawPatch(types.JSONPatchType, p))
		if err != nil {
			return false, false, err
		}
	}

	pc.DeepCopyInto(found)

	return true, !req.HCOTriggered, nil
}

func NewKubeVirtPriorityClass() *schedulingv1.PriorityClass {
	return &schedulingv1.PriorityClass{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "scheduling.k8s.io/v1",
			Kind:       "PriorityClass",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   kvPriorityClass,
			Labels: operands.GetLabels(hcoutil.AppComponentCompute),
		},
		// 1 billion is the highest value we can set
		// https://kubernetes.io/docs/concepts/configuration/pod-priority-preemption/#priorityclass
		Value:         1000000000,
		GlobalDefault: false,
		Description:   "This priority class should be used for KubeVirt core components only.",
	}
}

// translateKubeVirtConds translates list of KubeVirt conditions to a list of custom resource
// conditions.
func translateKubeVirtConds(orig []kubevirtcorev1.KubeVirtCondition) []metav1.Condition {
	translated := make([]metav1.Condition, len(orig))

	for i, origCond := range orig {
		translated[i] = metav1.Condition{
			Type:    string(origCond.Type),
			Status:  metav1.ConditionStatus(origCond.Status),
			Reason:  origCond.Reason,
			Message: origCond.Message,
		}
	}

	return translated
}

func getMandatoryKvFeatureGates(isKVMEmulation bool) []string {
	mandatoryFeatureGates := slices.Clone(hardCodeKvFgs)
	if !isKVMEmulation {
		mandatoryFeatureGates = append(mandatoryFeatureGates, kvHypervStrictCheck)
	}

	return mandatoryFeatureGates
}

// get list of feature gates or KV FG list
func getKvFeatureGateList(hc *hcov1.HyperConverged) []string {
	checks := getFeatureGateChecks(hc)
	res := make([]string, 0, len(checks)+len(mandatoryKvFeatureGates))
	res = append(res, mandatoryKvFeatureGates...)
	res = append(res, checks...)

	slices.Sort(res)

	return res
}

func getKvDisabledFeatureGateList(enabledFGs []string) []string {
	betaFGs := kvfeaturegates.GetBetaFeatureGates()

	disabled := make([]string, 0, len(betaFGs))
	for _, fg := range betaFGs {
		if !slices.Contains(enabledFGs, fg) {
			disabled = append(disabled, fg)
		}
	}

	return disabled
}

func hcoCertConfig2KvCertificateRotateStrategy(hcoCertConfig hcov1.HyperConvergedCertConfig) *kubevirtcorev1.KubeVirtCertificateRotateStrategy {
	return &kubevirtcorev1.KubeVirtCertificateRotateStrategy{
		SelfSigned: &kubevirtcorev1.KubeVirtSelfSignConfiguration{
			CA: &kubevirtcorev1.CertConfig{
				Duration:    hcoCertConfig.CA.Duration.DeepCopy(),
				RenewBefore: hcoCertConfig.CA.RenewBefore.DeepCopy(),
			},
			Server: &kubevirtcorev1.CertConfig{
				Duration:    hcoCertConfig.Server.Duration.DeepCopy(),
				RenewBefore: hcoCertConfig.Server.RenewBefore.DeepCopy(),
			},
		},
	}
}

func getLabelPatch(dest, src map[string]string) ([]byte, error) {
	const labelPath = "/metadata/labels/"
	var patches []patch.JSONPatchAction

	for k, v := range src {
		op := "replace"
		lbl, ok := dest[k]

		if !ok {
			op = "add"
		} else if lbl == v {
			continue
		}

		patches = append(patches, patch.JSONPatchAction{
			Op:    op,
			Path:  labelPath + patch.EscapeJSONPointer(k),
			Value: v,
		})
	}

	return json.Marshal(patches)
}

var (
	smbiosCfg  *kubevirtcorev1.SMBiosConfiguration
	smbiosOnce = &sync.Once{}
)

func getSMBConfig() (*kubevirtcorev1.SMBiosConfiguration, error) {
	var err error
	smbiosOnce.Do(func() {
		smbiosConfig := strings.TrimSpace(os.Getenv(smbiosEnvName))
		if smbiosConfig == "" {
			return
		}

		smbiosCfg = &kubevirtcorev1.SMBiosConfiguration{}
		err = yaml.NewYAMLOrJSONDecoder(strings.NewReader(smbiosConfig), 1024).Decode(smbiosCfg)
		if err != nil {
			smbiosCfg = nil
		}
	})

	return smbiosCfg, err
}
