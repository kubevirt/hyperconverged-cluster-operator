package v1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

const (
	APIVersionV1    = "v1"
	APIVersionGroup = "hco.kubevirt.io"
)

var (
	// SchemeGroupVersion is group version used to register these objects
	SchemeGroupVersion = schema.GroupVersion{Group: APIVersionGroup, Version: APIVersionV1}

	APIVersion = SchemeGroupVersion.String()

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme
	SchemeBuilder = &scheme.Builder{GroupVersion: SchemeGroupVersion}

	// AddToScheme tbd
	AddToScheme = SchemeBuilder.AddToScheme
)
