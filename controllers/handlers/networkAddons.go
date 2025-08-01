package handlers

import (
	"errors"
	"maps"
	"reflect"
	"sync"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/net"
	"sigs.k8s.io/controller-runtime/pkg/client"

	networkaddonsshared "github.com/kubevirt/cluster-network-addons-operator/pkg/apis/networkaddonsoperator/shared"
	networkaddonsv1 "github.com/kubevirt/cluster-network-addons-operator/pkg/apis/networkaddonsoperator/v1"
	networkaddonsnames "github.com/kubevirt/cluster-network-addons-operator/pkg/names"
	sdkapi "kubevirt.io/controller-lifecycle-operator-sdk/api"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/components"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/reformatobj"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

var defaultHco = components.GetOperatorCR()

func NewCnaHandler(Client client.Client, Scheme *runtime.Scheme) *operands.GenericOperand {
	return operands.NewGenericOperand(Client, Scheme, "NetworkAddonsConfig", &cnaHooks{}, false)
}

type cnaHooks struct {
	sync.Mutex
	cache *networkaddonsv1.NetworkAddonsConfig
}

func (h *cnaHooks) GetFullCr(hc *hcov1beta1.HyperConverged) (client.Object, error) {
	h.Lock()
	defer h.Unlock()

	if h.cache == nil {
		cna, err := NewNetworkAddons(hc)
		if err != nil {
			return nil, err
		}
		h.cache = cna
	}
	return h.cache, nil
}

func (h *cnaHooks) GetEmptyCr() client.Object { return &networkaddonsv1.NetworkAddonsConfig{} }
func (h *cnaHooks) GetConditions(cr runtime.Object) []metav1.Condition {
	return operands.OSConditionsToK8s(cr.(*networkaddonsv1.NetworkAddonsConfig).Status.Conditions)
}
func (h *cnaHooks) CheckComponentVersion(cr runtime.Object) bool {
	found := cr.(*networkaddonsv1.NetworkAddonsConfig)
	return operands.CheckComponentVersion(util.CnaoVersionEnvV, found.Status.ObservedVersion)
}
func (h *cnaHooks) Reset() {
	h.Lock()
	defer h.Unlock()

	h.cache = nil
}

func (h *cnaHooks) UpdateCR(req *common.HcoRequest, Client client.Client, exists runtime.Object, required runtime.Object) (bool, bool, error) {
	networkAddons, ok1 := required.(*networkaddonsv1.NetworkAddonsConfig)
	found, ok2 := exists.(*networkaddonsv1.NetworkAddonsConfig)

	if !ok1 || !ok2 {
		return false, false, errors.New("can't convert to CNA")
	}

	setDeployOvsAnnotation(req, found)

	changed := h.updateSpec(req, found, networkAddons)
	changed = h.updateLabels(found, networkAddons) || changed

	if changed {
		return h.updateCnaCr(req, Client, found)
	}

	return false, false, nil
}

func (*cnaHooks) JustBeforeComplete(_ *common.HcoRequest) { /* no implementation */ }

func (*cnaHooks) updateCnaCr(req *common.HcoRequest, Client client.Client, found *networkaddonsv1.NetworkAddonsConfig) (bool, bool, error) {
	err := Client.Update(req.Ctx, found)
	if err != nil {
		return false, false, err
	}
	return true, !req.HCOTriggered, nil
}

func (*cnaHooks) updateLabels(found *networkaddonsv1.NetworkAddonsConfig, networkAddons *networkaddonsv1.NetworkAddonsConfig) bool {
	if !util.CompareLabels(networkAddons, found) {
		util.MergeLabels(&networkAddons.ObjectMeta, &found.ObjectMeta)
		return true
	}
	return false
}

func (*cnaHooks) updateSpec(req *common.HcoRequest, found *networkaddonsv1.NetworkAddonsConfig, networkAddons *networkaddonsv1.NetworkAddonsConfig) bool {
	if !reflect.DeepEqual(found.Spec, networkAddons.Spec) && !req.UpgradeMode {
		if req.HCOTriggered {
			req.Logger.Info("Updating existing Network Addons's Spec to new opinionated values")
		} else {
			req.Logger.Info("Reconciling an externally updated Network Addons's Spec to its opinionated values")
		}
		networkAddons.Spec.DeepCopyInto(&found.Spec)
		return true
	}
	return false
}

// If deployOVS annotation doesn't exists prior the upgrade - set this annotation to true;
// Otherwise - remain the value as it is.
func setDeployOvsAnnotation(req *common.HcoRequest, found *networkaddonsv1.NetworkAddonsConfig) {
	if req.UpgradeMode {
		_, exists := req.Instance.Annotations["deployOVS"]
		if !exists {
			if req.Instance.Annotations == nil {
				req.Instance.Annotations = map[string]string{}
			}
			if found.Spec.Ovs != nil {
				req.Instance.Annotations["deployOVS"] = "true"
				req.Logger.Info("deployOVS annotation is set to true.")
			} else {
				req.Instance.Annotations["deployOVS"] = "false"
				req.Logger.Info("deployOVS annotation is set to false.")
			}

			req.Dirty = true
		}
	}
}

func NewNetworkAddons(hc *hcov1beta1.HyperConverged) (*networkaddonsv1.NetworkAddonsConfig, error) {
	ipam := &networkaddonsshared.KubevirtIpamController{}
	if util.GetClusterInfo().IsOpenshift() {
		ipam.DefaultNetworkNADNamespace = "openshift-ovn-kubernetes"
	}

	cnaoSpec := networkaddonsshared.NetworkAddonsConfigSpec{
		Multus:                 &networkaddonsshared.Multus{},
		LinuxBridge:            &networkaddonsshared.LinuxBridge{},
		KubeMacPool:            &networkaddonsshared.KubeMacPool{},
		KubevirtIpamController: ipam,
	}

	nameServerIP, err := getKSDNameServerIP(hc.Spec.KubeSecondaryDNSNameServerIP)
	if err != nil {
		return nil, err
	}

	if hc.Spec.FeatureGates.DeployKubeSecondaryDNS != nil && *hc.Spec.FeatureGates.DeployKubeSecondaryDNS {
		baseDomain := util.GetClusterInfo().GetBaseDomain()
		cnaoSpec.KubeSecondaryDNS = &networkaddonsshared.KubeSecondaryDNS{
			Domain:       baseDomain,
			NameServerIP: nameServerIP,
		}
	}

	cnaoSpec.Ovs = hcoAnnotation2CnaoSpec(hc.Annotations)
	cnaoInfra := hcoConfig2CnaoPlacement(hc.Spec.Infra.NodePlacement)
	cnaoWorkloads := hcoConfig2CnaoPlacement(hc.Spec.Workloads.NodePlacement)
	if cnaoInfra != nil || cnaoWorkloads != nil {
		cnaoSpec.PlacementConfiguration = &networkaddonsshared.PlacementConfiguration{
			Infra:     cnaoInfra,
			Workloads: cnaoWorkloads,
		}
	}
	cnaoSpec.SelfSignConfiguration = hcoCertConfig2CnaoSelfSignedConfig(&hc.Spec.CertConfig)

	cnaoSpec.TLSSecurityProfile = util.GetClusterInfo().GetTLSSecurityProfile(hc.Spec.TLSSecurityProfile)

	cna := NewNetworkAddonsWithNameOnly(hc)
	cna.Spec = cnaoSpec

	if err = operands.ApplyPatchToSpec(hc, common.JSONPatchCNAOAnnotationName, cna); err != nil {
		return nil, err
	}

	return reformatobj.ReformatObj(cna)
}

func getKSDNameServerIP(nameServerIPPtr *string) (string, error) {
	var nameServerIP string
	if nameServerIPPtr != nil {
		nameServerIP = *nameServerIPPtr
		if nameServerIP != "" && !net.IsIPv4String(nameServerIP) {
			return "", errors.New("kubeSecondaryDNSNameServerIP isn't a valid IPv4")
		}
	}

	return nameServerIP, nil
}

func NewNetworkAddonsWithNameOnly(hc *hcov1beta1.HyperConverged) *networkaddonsv1.NetworkAddonsConfig {
	return &networkaddonsv1.NetworkAddonsConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:   networkaddonsnames.OperatorConfig,
			Labels: operands.GetLabels(hc, util.AppComponentNetwork),
		},
	}
}

func hcoConfig2CnaoPlacement(hcoConf *sdkapi.NodePlacement) *networkaddonsshared.Placement {
	if hcoConf == nil {
		return nil
	}
	empty := true
	cnaoPlacement := &networkaddonsshared.Placement{}
	if hcoConf.Affinity != nil {
		empty = false
		hcoConf.Affinity.DeepCopyInto(&cnaoPlacement.Affinity)
	}

	for _, hcoTol := range hcoConf.Tolerations {
		empty = false
		cnaoTol := corev1.Toleration{}
		hcoTol.DeepCopyInto(&cnaoTol)
		cnaoPlacement.Tolerations = append(cnaoPlacement.Tolerations, cnaoTol)
	}

	if len(hcoConf.NodeSelector) > 0 {
		empty = false
		cnaoPlacement.NodeSelector = maps.Clone(hcoConf.NodeSelector)
	}

	if empty {
		return nil
	}
	return cnaoPlacement
}

func hcoAnnotation2CnaoSpec(hcoAnnotations map[string]string) *networkaddonsshared.Ovs {
	val, exists := hcoAnnotations["deployOVS"]
	if exists && val == "true" {
		return &networkaddonsshared.Ovs{}
	}
	return nil
}

func hcoCertConfig2CnaoSelfSignedConfig(hcoCertConfig *hcov1beta1.HyperConvergedCertConfig) *networkaddonsshared.SelfSignConfiguration {
	caRotateInterval := defaultHco.Spec.CertConfig.CA.Duration.Duration.String()
	caOverlapInterval := defaultHco.Spec.CertConfig.CA.RenewBefore.Duration.String()
	certRotateInterval := defaultHco.Spec.CertConfig.Server.Duration.Duration.String()
	certOverlapInterval := defaultHco.Spec.CertConfig.Server.RenewBefore.Duration.String()
	if hcoCertConfig.CA.Duration != nil {
		caRotateInterval = hcoCertConfig.CA.Duration.Duration.String()
	}
	if hcoCertConfig.CA.RenewBefore != nil {
		caOverlapInterval = hcoCertConfig.CA.RenewBefore.Duration.String()
	}
	if hcoCertConfig.Server.Duration != nil {
		certRotateInterval = hcoCertConfig.Server.Duration.Duration.String()
	}
	if hcoCertConfig.Server.RenewBefore != nil {
		certOverlapInterval = hcoCertConfig.Server.RenewBefore.Duration.String()
	}

	return &networkaddonsshared.SelfSignConfiguration{
		CARotateInterval:    caRotateInterval,
		CAOverlapInterval:   caOverlapInterval,
		CertRotateInterval:  certRotateInterval,
		CertOverlapInterval: certOverlapInterval,
	}
}
