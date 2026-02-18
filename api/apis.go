package api

import (
	"k8s.io/apimachinery/pkg/runtime"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
)

// AddToSchemes may be used to add all resources defined in the project to a Scheme
var AddToSchemes = runtime.SchemeBuilder{
	hcov1beta1.SchemeBuilder.AddToScheme,
	hcov1.SchemeBuilder.AddToScheme,
}

// AddToScheme adds all Resources to the Scheme
func AddToScheme(s *runtime.Scheme) error {
	return AddToSchemes.AddToScheme(s)
}
