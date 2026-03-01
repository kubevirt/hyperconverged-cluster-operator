package aie_webhook

import (
	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

const (
	aieWebhookName               = "kubevirt-aie-webhook"
	aieWebhookServiceAccountName = "kubevirt-aie-webhook"
	aieWebhookClusterRoleName    = "kubevirt-aie-webhook"
	aieWebhookIssuerName         = "kubevirt-aie-webhook-selfsigned"
	aieWebhookCertificateName    = "kubevirt-aie-webhook-tls"
	aieWebhookConfigMapName      = "kubevirt-aie-launcher-config"
	appComponent                 = hcoutil.AppComponentAIEWebhook
)

func shouldDeployAIEWebhook(hc *hcov1beta1.HyperConverged) bool {
	return hc.Spec.FeatureGates.DeployAIEWebhook != nil && *hc.Spec.FeatureGates.DeployAIEWebhook
}
