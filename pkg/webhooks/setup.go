package webhooks

import (
	"os"

	openshiftconfigv1 "github.com/openshift/api/config/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/webhooks/mutator"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/webhooks/validator"

	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

const (
	webHookCertDirEnv = "WEBHOOK_CERT_DIR"
)

var (
	logger = logf.Log.WithName("webhook-setup")
)

func SetupWebhookWithManager(mgr ctrl.Manager, isOpenshift bool, hcoTLSSecurityProfile *openshiftconfigv1.TLSSecurityProfile) error {
	operatorNsEnv := hcoutil.GetOperatorNamespaceFromEnv()

	decoder := admission.NewDecoder(mgr.GetScheme())

	whHandler := validator.NewWebhookHandler(logger, mgr.GetClient(), decoder, operatorNsEnv, isOpenshift, hcoTLSSecurityProfile)
	nsMutator := mutator.NewNsMutator(mgr.GetClient(), decoder, operatorNsEnv)
	hyperConvergedMutator := mutator.NewHyperConvergedMutator(mgr.GetClient(), decoder)

	srv := mgr.GetWebhookServer()

	srv.Register(hcoutil.HCONSWebhookPath, &webhook.Admission{Handler: nsMutator})
	srv.Register(hcoutil.HCOMutatingWebhookPath, &webhook.Admission{Handler: hyperConvergedMutator})
	srv.Register(hcoutil.HCOWebhookPath, &webhook.Admission{Handler: whHandler})

	return nil
}

func GetWebhookCertDir() string {
	webhookCertDir := os.Getenv(webHookCertDirEnv)
	if webhookCertDir != "" {
		return webhookCertDir
	}

	return hcoutil.DefaultWebhookCertDir
}
