package virtimport

import (
	"maps"
	"os"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

const (
	featureGateName = "deployVirtImport"
	appComponent    = hcoutil.AppComponentVirtImport
)

// Configured reports whether the virt-import image env vars are set.
func Configured() bool {
	return os.Getenv(hcoutil.VirtImportControllerImageEnvV) != "" && os.Getenv(hcoutil.VirtImportFetcherImageEnvV) != ""
}

func shouldDeploy(hc *hcov1.HyperConverged) bool {
	return hc.Spec.FeatureGates.IsEnabled(featureGateName)
}

// setMetadata moves a bundle object to the operator namespace (if namespaced) and adds the HCO
// labels, which the HCO cache and the HCO deletion protection rely on.
func setMetadata(meta *metav1.ObjectMeta, namespaced bool) {
	if namespaced {
		meta.Namespace = hcoutil.GetOperatorNamespaceFromEnv()
	}
	meta.Labels = withHCOLabels(meta.Labels)
}

func withHCOLabels(labels map[string]string) map[string]string {
	out := make(map[string]string, len(labels))
	maps.Copy(out, labels)
	maps.Copy(out, operands.GetLabels(appComponent))
	return out
}

func setSubjectNamespaces(subjects []rbacv1.Subject) {
	namespace := hcoutil.GetOperatorNamespaceFromEnv()
	for i := range subjects {
		if subjects[i].Kind == rbacv1.ServiceAccountKind {
			subjects[i].Namespace = namespace
		}
	}
}
