package virtimport

import (
	"os"
	"slices"
	"strings"

	openshiftconfigv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/library-go/pkg/crypto"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/tlssecprofile"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/virtimport"
)

func newDeployment(bundleDeployment *appsv1.Deployment, hc *hcov1.HyperConverged) *appsv1.Deployment {
	deployment := bundleDeployment.DeepCopy()
	setMetadata(&deployment.ObjectMeta, true)

	// Stable HCO selector labels, so bundle changes never hit the recreate path of the
	// Deployment handler for a changed (immutable) selector.
	deployment.Spec.Selector = &metav1.LabelSelector{MatchLabels: selectorLabels()}
	deployment.Spec.Template.Labels = withHCOLabels(deployment.Spec.Template.Labels)

	podSpec := &deployment.Spec.Template.Spec
	if i := slices.IndexFunc(podSpec.Containers, func(c corev1.Container) bool {
		return c.Name == virtimport.ManagerContainerName
	}); i >= 0 {
		container := &podSpec.Containers[i]
		container.Image = os.Getenv(hcoutil.VirtImportControllerImageEnvV)
		if container.ImagePullPolicy == "" {
			container.ImagePullPolicy = virtimport.DefaultPullPolicy(container.Image)
		}
		if j := slices.IndexFunc(container.Env, func(e corev1.EnvVar) bool {
			return e.Name == virtimport.FetcherImageEnvName
		}); j >= 0 {
			container.Env[j] = corev1.EnvVar{Name: virtimport.FetcherImageEnvName, Value: os.Getenv(hcoutil.VirtImportFetcherImageEnvV)}
		}
		container.Args = append(container.Args, tlsArgs(hc)...)
		container.Args = append(container.Args, metricsAuthArgs()...)
	}

	if np := hc.Spec.Deployment.NodePlacements; np != nil && np.Infra != nil {
		infra := np.Infra.DeepCopy()
		if infra.NodeSelector != nil {
			podSpec.NodeSelector = infra.NodeSelector
		}
		if infra.Affinity != nil {
			podSpec.Affinity = mergeAffinity(podSpec.Affinity, infra.Affinity)
		}
		podSpec.Tolerations = infra.Tolerations
	}

	return deployment
}

func selectorLabels() map[string]string {
	return map[string]string{
		hcoutil.AppLabel:          hcoutil.HyperConvergedName,
		hcoutil.AppLabelComponent: string(appComponent),
	}
}

// mergeAffinity returns the infra affinity, but keeps the required node affinity of the bundle
// (supported arch and OS). Terms are ORed, so each infra term is ANDed with each bundle term.
func mergeAffinity(bundle, infra *corev1.Affinity) *corev1.Affinity {
	out := infra.DeepCopy()
	if bundle == nil || bundle.NodeAffinity == nil || bundle.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil {
		return out
	}
	bundleRequired := bundle.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution

	if out.NodeAffinity == nil {
		out.NodeAffinity = &corev1.NodeAffinity{}
	}
	required := out.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution
	if required == nil || len(required.NodeSelectorTerms) == 0 {
		out.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution = bundleRequired.DeepCopy()
		return out
	}

	var terms []corev1.NodeSelectorTerm
	for _, infraTerm := range required.NodeSelectorTerms {
		// An empty term matches no node. Merging would turn it into the bundle term.
		if len(infraTerm.MatchExpressions)+len(infraTerm.MatchFields) == 0 {
			terms = append(terms, corev1.NodeSelectorTerm{})
			continue
		}
		for _, bundleTerm := range bundleRequired.NodeSelectorTerms {
			terms = append(terms, corev1.NodeSelectorTerm{
				MatchExpressions: slices.Concat(infraTerm.MatchExpressions, bundleTerm.MatchExpressions),
				MatchFields:      slices.Concat(infraTerm.MatchFields, bundleTerm.MatchFields),
			})
		}
	}
	required.NodeSelectorTerms = terms
	return out
}

// tlsArgs returns the TLS profile of the HyperConverged, which the controller applies to all its
// TLS servers.
func tlsArgs(hc *hcov1.HyperConverged) []string {
	profile := hc.Spec.Security.TLSSecurityProfile
	cipherNames, minTLSVersion := tlssecprofile.GetCipherSuitesAndMinTLSVersion(profile)

	var args []string
	if minTLSVersion != "" {
		args = append(args, "--tls-min-version="+string(minTLSVersion))
	}
	// Go does not allow configuring TLS 1.3 cipher suites.
	if ianaCiphers := crypto.OpenSSLToIANACipherSuites(cipherNames); minTLSVersion < openshiftconfigv1.VersionTLS13 && len(ianaCiphers) > 0 {
		args = append(args, "--tls-cipher-suites="+strings.Join(ianaCiphers, ","))
	}
	if curves := tlssecprofile.GetGroupsInStringFormat(profile); curves != "" {
		args = append(args, "--tls-curve-preferences="+curves)
	}
	return args
}

// metricsAuthArgs returns, on OpenShift, the flags that let the in-cluster Prometheus authenticate
// with its client certificate.
func metricsAuthArgs() []string {
	if !hcoutil.GetClusterInfo().IsOpenshift() {
		return nil
	}
	return []string{
		"--metrics-client-ca-configmap=" + hcoutil.ClientCAConfigMapNamespace + "/" + hcoutil.ClientCAConfigMapName,
		"--metrics-allowed-client-cn=" + hcoutil.PrometheusK8sCN,
	}
}
