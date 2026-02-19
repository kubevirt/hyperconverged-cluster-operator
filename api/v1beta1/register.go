// NOTE: Boilerplate only.  Ignore this file.

// package v1beta1 contains API Schema definitions for the hco vbeta1 API group
// +k8s:deepcopy-gen=package,register
// +k8s:defaulter-gen=TypeMeta
// +groupName=hco.kubevirt.io
package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	APIVersionBeta  = "v1beta1"
	APIVersionGroup = "hco.kubevirt.io"
)

var (
	// SchemeGroupVersion is group version used to register these objects
	SchemeGroupVersion = schema.GroupVersion{Group: APIVersionGroup, Version: APIVersionBeta}

	APIVersion = SchemeGroupVersion.String()

	// schemeBuilder is used to add go types to the GroupVersionKind scheme
	schemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)

	// localSchemeBuilder is used for type conversions.
	localSchemeBuilder = &schemeBuilder

	// AddToScheme tbd
	AddToScheme = schemeBuilder.AddToScheme
)

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion, &HyperConverged{}, &HyperConvergedList{})
	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}
