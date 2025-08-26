package wasp_agent

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	securityv1 "github.com/openshift/api/security/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
)

func NewWaspAgentSCCHandler(Client client.Client, Scheme *runtime.Scheme) operands.Operand {
	return operands.NewConditionalHandler(
		operands.NewSecurityContextConstraintsHandler(Client, Scheme, newWaspAgentSCC),
		shouldDeployWaspAgent,
		func(hc *hcov1beta1.HyperConverged) client.Object {
			return NewWaspAgentSCCWithNameOnly(hc)
		},
	)
}

func newWaspAgentSCC(hc *hcov1beta1.HyperConverged) *securityv1.SecurityContextConstraints {
	scc := NewWaspAgentSCCWithNameOnly(hc)

	scc.AllowPrivilegedContainer = true
	scc.AllowHostDirVolumePlugin = true
	scc.AllowHostIPC = true
	scc.AllowHostNetwork = true
	scc.AllowHostPID = true
	scc.AllowHostPorts = true
	scc.ReadOnlyRootFilesystem = false
	scc.DefaultAddCapabilities = nil
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
		fmt.Sprintf("system:serviceaccount:%s:%s", hc.Namespace, waspAgentServiceAccountName),
	}
	scc.Volumes = []securityv1.FSType{
		securityv1.FSTypeAll,
	}
	scc.AllowedCapabilities = []corev1.Capability{
		"*",
	}
	scc.AllowedUnsafeSysctls = []string{
		"*",
	}
	scc.SeccompProfiles = []string{
		"*",
	}

	return scc
}

func NewWaspAgentSCCWithNameOnly(hc *hcov1beta1.HyperConverged) *securityv1.SecurityContextConstraints {
	return &securityv1.SecurityContextConstraints{
		ObjectMeta: metav1.ObjectMeta{
			Name:   waspAgentSCCName,
			Labels: operands.GetLabels(hc, AppComponentWaspAgent),
		},
	}
}
