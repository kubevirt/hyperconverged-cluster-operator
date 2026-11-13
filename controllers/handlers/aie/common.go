package aie

import (
	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

const (
	policyName                    = "kubevirt-aie-launcher"
	bindingName                   = policyName + "-binding"
	alternativeLauncherAnnotation = "kubevirt.io/alternative-launcher-image"
	DeployAIEAnnotation           = hcoutil.HCOAnnotationPrefix + "deployAIE"
)

func shouldDeployAIE(hc *hcov1.HyperConverged) bool {
	return hc.Annotations[DeployAIEAnnotation] == "true"
}
