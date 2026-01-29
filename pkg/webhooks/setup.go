package webhooks

import (
	"os"

	openshiftconfigv1 "github.com/openshift/api/config/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
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

func SetupWebhookWithManager(mgr ctrl.Manager, isOpenshift bool, hcoTLSSecurityProfile *openshiftconfigv1.TLSSecurityProfile) error {
	operatorNsEnv := hcoutil.GetOperatorNamespaceFromEnv()

	decoder := admission.NewDecoder(mgr.GetScheme())

	whHandler := validator.NewWebhookHandler(logger, mgr.GetClient(), decoder, operatorNsEnv, isOpenshift, hcoTLSSecurityProfile)
	whV1Beta1Handler := validator.NewV1Beta1WebhookHandler(logger, mgr.GetClient(), decoder, operatorNsEnv, isOpenshift, hcoTLSSecurityProfile)
	nsMutator := mutator.NewNsMutator(mgr.GetClient(), decoder, operatorNsEnv)
	hyperConvergedMutator := mutator.NewHyperConvergedMutator(mgr.GetClient(), decoder)
	hyperConvergedV1Beta1Mutator := mutator.NewHyperConvergedV1Beta1Mutator(mgr.GetClient(), decoder)

	// add the conversion webhook
	if err := ctrl.NewWebhookManagedBy(mgr).For(&hcov1beta1.HyperConverged{}).Complete(); err != nil {
		return err
	}

	srv := mgr.GetWebhookServer()

	srv.Register(hcoutil.HCONSWebhookPath, &webhook.Admission{Handler: nsMutator})
	srv.Register(hcoutil.HCOMutatingWebhookPath, &webhook.Admission{Handler: hyperConvergedMutator})
	srv.Register(hcoutil.HCOV1Beta1MutatingWebhookPath, &webhook.Admission{Handler: hyperConvergedV1Beta1Mutator})
	srv.Register(hcoutil.HCOWebhookPath, &webhook.Admission{Handler: whHandler})
	srv.Register(hcoutil.HCOWebhookV1Beta1Path, &webhook.Admission{Handler: whV1Beta1Handler})

	return nil
}

func GetWebhookCertDir() string {
	webhookCertDir := os.Getenv(webHookCertDirEnv)
	if webhookCertDir != "" {
		return webhookCertDir
	}

	return hcoutil.DefaultWebhookCertDir
}
