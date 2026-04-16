package aie

import (
	"fmt"

	securityv1 "github.com/openshift/api/security/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

const (
	iommufdDevicePluginSCCName = "iommufd-device-plugin"
)

func NewIOMMUFDDevicePluginSCCHandler(cli client.Client, Scheme *runtime.Scheme) operands.Operand {
	return operands.NewConditionalHandler(
		operands.NewSecurityContextConstraintsHandler(cli, Scheme, newIOMMUFDDevicePluginSCC()),
		shouldDeployAIE,
		func(hc *hcov1beta1.HyperConverged) client.Object {
			return NewIOMMUFDDevicePluginSCCWithNameOnly()
		},
	)
}

func NewIOMMUFDDevicePluginSCCWithNameOnly() *securityv1.SecurityContextConstraints {
	return &securityv1.SecurityContextConstraints{
		ObjectMeta: metav1.ObjectMeta{
			Name:   iommufdDevicePluginSCCName,
			Labels: operands.GetLabels(iommufdDevicePluginAppComponent),
		},
	}
}

func newIOMMUFDDevicePluginSCC() *securityv1.SecurityContextConstraints {
	scc := NewIOMMUFDDevicePluginSCCWithNameOnly()

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
		fmt.Sprintf("system:serviceaccount:%s:%s", hcoutil.GetOperatorNamespaceFromEnv(), iommufdDevicePluginServiceAccountName),
	}
	scc.Volumes = []securityv1.FSType{
		securityv1.FSTypeHostPath,
		securityv1.FSProjected,
	}
	scc.AllowedCapabilities = []corev1.Capability{}

	return scc
}
