// NOTE: Boilerplate only.  Ignore this file.

package v1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
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

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme
	SchemeBuilder = &scheme.Builder{GroupVersion: SchemeGroupVersion}

	// AddToScheme tbd
	AddToScheme = SchemeBuilder.AddToScheme
)
