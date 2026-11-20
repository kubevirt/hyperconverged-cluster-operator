package virtimport

import (
	"maps"
	"os"
	"slices"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/virtimport"
)

func newDeploymentWithNameOnly(bundleDeployment *appsv1.Deployment) client.Object {
	return &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: appsv1.SchemeGroupVersion.String(),
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      bundleDeployment.Name,
			Namespace: hcoutil.GetOperatorNamespaceFromEnv(),
			Labels:    withHCOLabels(bundleDeployment.Labels),
		},
	}
}

func newDeployment(bundleDeployment *appsv1.Deployment, hc *hcov1.HyperConverged) *appsv1.Deployment {
	deployment := bundleDeployment.DeepCopy()
	setMetadata(&deployment.ObjectMeta, true)

	// Stable HCO selector labels, so bundle changes never hit the recreate path of the
	// Deployment handler for a changed (immutable) selector.
	deployment.Spec.Selector = &metav1.LabelSelector{MatchLabels: selectorLabels()}
	deployment.Spec.Template.Labels = withHCOLabels(deployment.Spec.Template.Labels)

	podSpec := &deployment.Spec.Template.Spec
	for i := range podSpec.Containers {
		container := &podSpec.Containers[i]
		if container.Name != virtimport.ManagerContainerName {
			continue
		}
		container.Image = os.Getenv(hcoutil.VirtImportControllerImageEnvV)
		if container.ImagePullPolicy == "" {
			container.ImagePullPolicy = virtimport.DefaultPullPolicy(container.Image)
		}
		for j := range container.Env {
			if container.Env[j].Name == virtimport.FetcherImageEnvName {
				container.Env[j].Value = os.Getenv(hcoutil.VirtImportFetcherImageEnvV)
			}
		}
	}

	if np := hc.Spec.Deployment.NodePlacements; np != nil && np.Infra != nil {
		if np.Infra.NodeSelector != nil {
			podSpec.NodeSelector = maps.Clone(np.Infra.NodeSelector)
		}
		if np.Infra.Affinity != nil {
			podSpec.Affinity = mergeAffinity(podSpec.Affinity, np.Infra.Affinity)
		}
		podSpec.Tolerations = slices.Clone(np.Infra.Tolerations)
	}

	return deployment
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
	infraRequired := out.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution
	if infraRequired == nil || len(infraRequired.NodeSelectorTerms) == 0 {
		out.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution = bundleRequired.DeepCopy()
		return out
	}

	terms := make([]corev1.NodeSelectorTerm, 0, len(infraRequired.NodeSelectorTerms)*len(bundleRequired.NodeSelectorTerms))
	for _, infraTerm := range infraRequired.NodeSelectorTerms {
		for _, bundleTerm := range bundleRequired.NodeSelectorTerms {
			terms = append(terms, corev1.NodeSelectorTerm{
				MatchExpressions: mergeRequirements(infraTerm.MatchExpressions, bundleTerm.MatchExpressions),
				MatchFields:      mergeRequirements(infraTerm.MatchFields, bundleTerm.MatchFields),
			})
		}
	}
	out.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution = &corev1.NodeSelector{NodeSelectorTerms: terms}
	return out
}

func mergeRequirements(a, b []corev1.NodeSelectorRequirement) []corev1.NodeSelectorRequirement {
	if len(a)+len(b) == 0 {
		return nil
	}
	out := make([]corev1.NodeSelectorRequirement, 0, len(a)+len(b))
	for _, r := range slices.Concat(a, b) {
		out = append(out, *r.DeepCopy())
	}
	return out
}
