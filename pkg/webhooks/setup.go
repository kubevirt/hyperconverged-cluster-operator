package webhooks

import (
	"os"

	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

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

func SetupWebhookWithManager(mgr ctrl.Manager, isOpenshift bool) error {
	operatorNsEnv := hcoutil.GetOperatorNamespaceFromEnv()

	decoder := admission.NewDecoder(mgr.GetScheme())

	v1Beta1WHHandler := validator.NewWebhookV1Beta1Handler(logger, mgr.GetClient(), decoder, operatorNsEnv, isOpenshift)
	v1WHHandler := validator.NewWebhookHandler(logger, mgr.GetClient(), decoder, operatorNsEnv, isOpenshift, v1Beta1WHHandler)
	nsMutator := mutator.NewNsMutator(mgr.GetClient(), decoder, operatorNsEnv)
	v1HCMutator := mutator.NewHyperConvergedMutator(mgr.GetClient(), decoder)
	v1Beta1HCMutator := mutator.NewHyperConvergedV1Beta1Mutator(mgr.GetClient(), decoder)

	// add the conversion webhook
	// this is working because hcov1beta1.HyperConverged implements the
	// sigs.k8s.io/controller-runtime/pkg/conversion.Convertible interface, and hcov1.HyperConverged
	// implements the sigs.k8s.io/controller-runtime/pkg/conversion.Hub interface.
	if err := ctrl.NewWebhookManagedBy[*hcov1beta1.HyperConverged](mgr, &hcov1beta1.HyperConverged{}).Complete(); err != nil {
		return err
	}

	srv := mgr.GetWebhookServer()

	srv.Register(hcoutil.HCONSWebhookPath, &webhook.Admission{Handler: nsMutator})
	srv.Register(hcoutil.HCOV1Beta1MutatingWebhookPath, &webhook.Admission{Handler: v1Beta1HCMutator})
	srv.Register(hcoutil.HCOV1MutatingWebhookPath, &webhook.Admission{Handler: v1HCMutator})
	srv.Register(hcoutil.HCOV1Beta1WebhookPath, &webhook.Admission{Handler: v1Beta1WHHandler})
	srv.Register(hcoutil.HCOV1WebhookPath, &webhook.Admission{Handler: v1WHHandler})

	return nil
}

func GetWebhookCertDir() string {
	webhookCertDir := os.Getenv(webHookCertDirEnv)
	if webhookCertDir != "" {
		return webhookCertDir
	}

	return hcoutil.DefaultWebhookCertDir
}
