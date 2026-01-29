// NOTE: Boilerplate only.  Ignore this file.

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	APIVersionV1      = "v1"
	CurrentAPIVersion = APIVersionV1
	APIVersionGroup   = "hco.kubevirt.io"
	APIVersion        = APIVersionGroup + "/" + CurrentAPIVersion
)

var (
	// SchemeGroupVersion is group version used to register these objects
	SchemeGroupVersion = schema.GroupVersion{Group: APIVersionGroup, Version: APIVersionV1}

	// schemeBuilder is used to add go types to the GroupVersionKind scheme
	schemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)

	// AddToScheme tbd
	AddToScheme = schemeBuilder.AddToScheme
)

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion, &HyperConverged{}, &HyperConvergedList{})
	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}
