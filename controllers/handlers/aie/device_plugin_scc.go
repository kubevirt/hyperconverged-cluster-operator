package aie

import (
	"fmt"

	log "github.com/go-logr/logr"
	securityv1 "github.com/openshift/api/security/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
)

const (
	iommufdDevicePluginSCCName = "iommufd-device-plugin"
)

func NewIOMMUFDDevicePluginSCCHandler(
	_ log.Logger, Client client.Client, Scheme *runtime.Scheme, hc *hcov1beta1.HyperConverged,
) (operands.Operand, error) {
	return operands.NewConditionalHandler(
		operands.NewSecurityContextConstraintsHandler(Client, Scheme, newIOMMUFDDevicePluginSCC),
		shouldDeployAIE,
		func(hc *hcov1beta1.HyperConverged) client.Object {
			return NewIOMMUFDDevicePluginSCCWithNameOnly(hc)
		},
	), nil
}

func NewIOMMUFDDevicePluginSCCWithNameOnly(hc *hcov1beta1.HyperConverged) *securityv1.SecurityContextConstraints {
	return &securityv1.SecurityContextConstraints{
		ObjectMeta: metav1.ObjectMeta{
			Name:   iommufdDevicePluginSCCName,
			Labels: operands.GetLabels(hc, iommufdDevicePluginAppComponent),
		},
	}
}

func newIOMMUFDDevicePluginSCC(hc *hcov1beta1.HyperConverged) *securityv1.SecurityContextConstraints {
	scc := NewIOMMUFDDevicePluginSCCWithNameOnly(hc)

	scc.AllowPrivilegedContainer = true
	scc.AllowHostDirVolumePlugin = true
	scc.AllowHostIPC = false
	scc.AllowHostNetwork = false
	scc.AllowHostPID = false
	scc.AllowHostPorts = false
	scc.ReadOnlyRootFilesystem = false
	scc.RunAsUser = securityv1.RunAsUserStrategyOptions{
		Type: securityv1.RunAsUserStrategyRunAsAny,
	}
	scc.SupplementalGroups = securityv1.SupplementalGroupsStrategyOptions{
		Type: securityv1.SupplementalGroupsStrategyRunAsAny,
	}
	scc.SELinuxContext = securityv1.SELinuxContextStrategyOptions{
		Type: securityv1.SELinuxStrategyRunAsAny,
	}
	scc.Users = []string{
		fmt.Sprintf("system:serviceaccount:%s:%s", hc.Namespace, iommufdDevicePluginServiceAccountName),
	}
	scc.Volumes = []securityv1.FSType{
		securityv1.FSTypeHostPath,
		securityv1.FSProjected,
	}
	scc.AllowedCapabilities = []corev1.Capability{}

	return scc
}
