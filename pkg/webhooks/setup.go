package webhooks

import (
	"os"

	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/webhooks/mutator"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/webhooks/validator"
)

const (
	webHookCertDirEnv = "WEBHOOK_CERT_DIR"
)

var (
	logger = logf.Log.WithName("webhook-setup")
)

func SetupWebhookWithManager(mgr ctrl.Manager, isOpenshift bool) error {
	operatorNsEnv := hcoutil.GetOperatorNamespaceFromEnv()

	decoder := admission.NewDecoder(mgr.GetScheme())

	whHandler := validator.NewWebhookHandler(logger, mgr.GetClient(), decoder, operatorNsEnv, isOpenshift)
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
