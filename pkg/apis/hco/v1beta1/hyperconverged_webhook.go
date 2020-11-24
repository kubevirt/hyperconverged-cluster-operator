package v1beta1

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"net/http"
	"os"
	"path/filepath"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	WebhookCertDir  = "/apiserver.local.config/certificates"
	WebhookCertName = "apiserver.crt"
	WebhookKeyName  = "apiserver.key"
)

var (
	hcolog = logf.Log.WithName("hyperconverged-resource")
)

type WebhookHandlerIfs interface {
	Init(logger logr.Logger, cli client.Client, namespace string)
	ValidateCreate(hc *HyperConverged) error
	ValidateUpdate(requested *HyperConverged, exists *HyperConverged) error
	ValidateDelete(hc *HyperConverged) error
	HandleMutatingNsDelete(ns *corev1.Namespace, dryRun bool) error
}

var whHandler WebhookHandlerIfs

func (r *HyperConverged) SetupWebhookWithManager(ctx context.Context, mgr ctrl.Manager, handler WebhookHandlerIfs) error {
	operatorNsEnv, nserr := hcoutil.GetOperatorNamespaceFromEnv()
	if nserr != nil {
		hcolog.Error(nserr, "failed to get operator namespace from the environment")
		return nserr
	}

	// Make sure the certificates are mounted, this should be handled by the OLM
	whHandler = handler
	whHandler.Init(hcolog, mgr.GetClient(), operatorNsEnv)

	certs := []string{filepath.Join(WebhookCertDir, WebhookCertName), filepath.Join(WebhookCertDir, WebhookKeyName)}
	for _, fname := range certs {
		if _, err := os.Stat(fname); err != nil {
			hcolog.Error(err, "CSV certificates were not found, skipping webhook initialization")
			return err
		}
	}

	// The OLM limits the webhook scope to the namespaces that are defined in the OperatorGroup
	// by setting namespaceSelector in the ValidatingWebhookConfiguration.  We would like our webhook to intercept
	// requests from all namespaces, and fail them if they're not in the correct namespace for HCO (for CREATE).
	// Lucikly the OLM does not watch and reconcile the ValidatingWebhookConfiguration so we can simply reset the
	// namespaceSelector

	vwcList := &admissionregistrationv1.ValidatingWebhookConfigurationList{}
	err := mgr.GetAPIReader().List(ctx, vwcList, client.MatchingLabels{"olm.webhook-description-generate-name": hcoutil.HcoValidatingWebhook})
	if err != nil {
		hcolog.Error(err, "A validating webhook for the HCO was not found")
		return err
	}

	for _, vwc := range vwcList.Items {
		update := false

		for i, wh := range vwc.Webhooks {
			if wh.Name == hcoutil.HcoValidatingWebhook {
				vwc.Webhooks[i].NamespaceSelector = &metav1.LabelSelector{MatchLabels: map[string]string{}}
				update = true
			}
		}

		if update {
			hcolog.Info("Removing namespace scope from webhook", "webhook", vwc.Name)
			err = mgr.GetClient().Update(ctx, &vwc)
			if err != nil {
				hcolog.Error(err, "Failed updating webhook", "webhook", vwc.Name)
				return err
			}
		}
	}

	bldr := ctrl.NewWebhookManagedBy(mgr).For(r)

	srv := mgr.GetWebhookServer()
	srv.CertDir = WebhookCertDir
	srv.CertName = WebhookCertName
	srv.KeyName = WebhookKeyName
	srv.Port = hcoutil.WebhookPort
	srv.Register(hcoutil.HCONSWebhookPath, &webhook.Admission{Handler: &nsMutator{}})

	return bldr.Complete()
}

var _ webhook.Validator = &HyperConverged{}

func (r *HyperConverged) ValidateCreate() error {
	return whHandler.ValidateCreate(r)
}

func (r *HyperConverged) ValidateUpdate(old runtime.Object) error {
	oldR, ok := old.(*HyperConverged)
	if !ok {
		return fmt.Errorf("expect old object to be a %T instead of %T", oldR, old)
	}

	return whHandler.ValidateUpdate(r, oldR)
}

func (r *HyperConverged) ValidateDelete() error {
	return whHandler.ValidateDelete(r)
}

// nsMutator mutates Ns requests
type nsMutator struct {
	decoder *admission.Decoder
}

// nsMutator is trying to to delete HyperConverged CR before accepting the deletion request for the namespace
// if it accepts the deletion request, the namespace should be ready to be delete
// without getting stuck or without the risk of any leftovers.
// Please notice that the serviceAccounts used by other operator are also part of this namespace
// so once we accept the deletion of the namespace, all the components operator are going to quickly
// lose any power
func (a *nsMutator) Handle(ctx context.Context, req admission.Request) admission.Response {
	hcolog.Info("reaching nsMutator.Handle")
	ns := &corev1.Namespace{}

	if req.Operation == admissionv1beta1.Delete {

		// In reference to PR: https://github.com/kubernetes/kubernetes/pull/76346
		// OldObject contains the object being deleted
		err := a.decoder.DecodeRaw(req.OldObject, ns)
		if err != nil {
			hcolog.Error(err, "failed decoding namespace object")
			return admission.Errored(http.StatusBadRequest, err)
		}

		herr := whHandler.HandleMutatingNsDelete(ns, *req.DryRun)
		if herr != nil {
			hcolog.Error(herr, hcoutil.HCONSWebhookPath+" refused the request")
			return admission.Errored(http.StatusInternalServerError, herr)
		}

		hcolog.Info(hcoutil.HCONSWebhookPath + " admitted the delete request")
		return admission.Allowed(hcoutil.HCONSWebhookPath + " admitted the delete request")
	}

	// ignoring other operations
	return admission.Allowed(hcoutil.HCONSWebhookPath + " admitted the request")

}

// nsMutator implements admission.DecoderInjector.
// A decoder will be automatically injected.

// InjectDecoder injects the decoder.
func (a *nsMutator) InjectDecoder(d *admission.Decoder) error {
	a.decoder = d
	return nil
}
