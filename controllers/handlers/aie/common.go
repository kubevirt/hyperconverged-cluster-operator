package aie

import (
	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

const (
	aieWebhookName               = "kubevirt-aie-webhook"
	aieWebhookServiceAccountName = "kubevirt-aie-webhook"
	aieWebhookClusterRoleName    = "kubevirt-aie-webhook"
	aieWebhookTLSSecretName      = "kubevirt-aie-webhook-tls"
	aieWebhookCertMountPath      = "/tmp/k8s-webhook-server/serving-certs"
	aieWebhookConfigMapName      = "kubevirt-aie-launcher-config"
	appComponent                 = hcoutil.AppComponentAIEWebhook

	DeployAIEAnnotation = hcoutil.HCOAnnotationPrefix + "deployAIE"
)

func shouldDeployAIE(hc *hcov1beta1.HyperConverged) bool {
	return hc.Annotations[DeployAIEAnnotation] == "true"
}
