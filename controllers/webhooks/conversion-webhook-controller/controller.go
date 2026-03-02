package conversion_webhook_controller

import (
	"context"
	"errors"
	"slices"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/ownresources"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

const (
	k8sCertInjectionAnnotationName = "cert-manager.io/inject-ca-from"

	openshiftCertInjectionAnnotationName = "service.beta.openshift.io/inject-cabundle"

	olmCaBundleKey = "olmCAKey"
)

var (
	setupLogger = logf.Log.WithName("conversion-webhook-controller")
)

// ReconcileConversionWebhook reconciles the HyperConverged CRD conversion webhook configuration
type ReconcileConversionWebhook struct {
	client             client.Client
	namespace          string
	secretName         string
	serviceName        string
	certAnnotationName string
	certAnnotationVal  string
	managedByOLM       bool
}

// RegisterReconciler creates a new conversion webhook controller and registers it into manager.
func RegisterReconciler(mgr manager.Manager) error {
	deploymentName := ownresources.GetDeploymentRef().Name
	if deploymentName == "" {
		setupLogger.Info("Deployment reference not available, skipping conversion webhook controller registration")
		return nil
	}

	secretName := deploymentName + "-service-cert"
	serviceName := deploymentName + "-service"
	namespace := hcoutil.GetOperatorNamespaceFromEnv()

	return add(mgr, newReconciler(mgr, namespace, secretName, serviceName))
}

func newReconciler(mgr manager.Manager, namespace, secretName, serviceName string) *ReconcileConversionWebhook {
	r := &ReconcileConversionWebhook{
		client:      mgr.GetClient(),
		namespace:   namespace,
		secretName:  secretName,
		serviceName: serviceName,
	}

	if hcoutil.GetClusterInfo().IsOpenshift() {
		r.certAnnotationName = openshiftCertInjectionAnnotationName
		r.certAnnotationVal = "true"
	} else {
		r.certAnnotationName = k8sCertInjectionAnnotationName
		r.certAnnotationVal = namespace + "/" + secretName
	}

	r.managedByOLM = hcoutil.GetClusterInfo().IsManagedByOLM()

	return r
}

func add(mgr manager.Manager, r *ReconcileConversionWebhook) error {
	setupLogger.Info("Setting up conversion webhook controller")

	c, err := controller.New("conversion-webhook-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	err = c.Watch(
		source.Kind[*apiextensionsv1.CustomResourceDefinition](
			mgr.GetCache(),
			&apiextensionsv1.CustomResourceDefinition{},
			&handler.TypedEnqueueRequestForObject[*apiextensionsv1.CustomResourceDefinition]{},
			predicate.And(
				predicate.NewTypedPredicateFuncs(func(crd *apiextensionsv1.CustomResourceDefinition) bool {
					return crd.Name == hcoutil.HyperConvergedCRDName
				}),
				predicate.Or[*apiextensionsv1.CustomResourceDefinition](
					predicate.TypedGenerationChangedPredicate[*apiextensionsv1.CustomResourceDefinition]{},
					predicate.TypedAnnotationChangedPredicate[*apiextensionsv1.CustomResourceDefinition]{}),
			),
		),
	)

	if err != nil {
		return err
	}

	if r.managedByOLM {
		return c.Watch(
			source.Kind[*corev1.Secret](
				mgr.GetCache(),
				&corev1.Secret{},
				&handler.TypedEnqueueRequestForObject[*corev1.Secret]{},
				predicate.NewTypedPredicateFuncs(func(secret *corev1.Secret) bool {
					return secret.Name == r.secretName
				}),
			),
		)
	}

	return nil
}

func (r *ReconcileConversionWebhook) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	logger := logf.FromContext(ctx).WithName("conversion-webhook-controller").WithValues("Request.Name", request.Name)
	if request.Name == hcoutil.HyperConvergedCRDName {
		logger.Info("Reconciling CRD")
	} else {
		logger = logger.WithValues("Request.Namespace", request.Namespace)
		logger.Info("Reconciling Secret")
	}

	// Get the HyperConverged CRD
	crd := &apiextensionsv1.CustomResourceDefinition{}
	if err := r.client.Get(ctx, client.ObjectKey{Name: hcoutil.HyperConvergedCRDName}, crd); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("HyperConverged CRD not found, skipping reconciliation")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	var caBundle []byte
	var err error
	if r.managedByOLM {
		caBundle, err = r.getOLMCaBundle(ctx)
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	// Check if update is needed
	if !r.needsUpdate(crd, caBundle) {
		logger.Info("CRD conversion webhook configuration is up to date")
		return reconcile.Result{}, nil
	}

	// Update CRD with conversion webhook configuration
	if err = r.updateCRDConversion(ctx, crd, caBundle); err != nil {
		logger.Error(err, "Failed to update CRD conversion webhook configuration")
		return reconcile.Result{}, err
	}

	logger.Info("Successfully updated CRD conversion webhook configuration")
	return reconcile.Result{}, nil
}

func (r *ReconcileConversionWebhook) needsUpdate(crd *apiextensionsv1.CustomResourceDefinition, caBundle []byte) bool {
	if !r.managedByOLM && crd.Annotations[r.certAnnotationName] != r.certAnnotationVal {
		return true
	}

	if crd.Spec.Conversion == nil {
		return true
	}

	if crd.Spec.Conversion.Strategy != apiextensionsv1.WebhookConverter {
		return true
	}

	if crd.Spec.Conversion.Webhook == nil {
		return true
	}

	webhook := crd.Spec.Conversion.Webhook
	if webhook.ClientConfig == nil {
		return true
	}

	clientConfig := webhook.ClientConfig
	if clientConfig.Service == nil {
		return true
	}

	if r.managedByOLM && slices.Compare(caBundle, clientConfig.CABundle) != 0 {
		return true
	}

	service := clientConfig.Service
	return service.Name != r.serviceName ||
		service.Namespace != r.namespace ||
		ptr.Deref(service.Path, "") != hcoutil.HCOConversionWebhookPath ||
		ptr.Deref(service.Port, 0) != int32(hcoutil.WebhookPort)
}

func (r *ReconcileConversionWebhook) updateCRDConversion(ctx context.Context, crd *apiextensionsv1.CustomResourceDefinition, caBundle []byte) error {
	if !r.managedByOLM {
		if crd.Annotations == nil {
			crd.Annotations = map[string]string{}
		}

		crd.Annotations[r.certAnnotationName] = r.certAnnotationVal
	}

	crd.Spec.Conversion = &apiextensionsv1.CustomResourceConversion{
		Strategy: apiextensionsv1.WebhookConverter,
		Webhook: &apiextensionsv1.WebhookConversion{
			ClientConfig: &apiextensionsv1.WebhookClientConfig{
				Service: &apiextensionsv1.ServiceReference{
					Namespace: r.namespace,
					Name:      r.serviceName,
					Path:      ptr.To(hcoutil.HCOConversionWebhookPath),
					Port:      ptr.To(int32(hcoutil.WebhookPort)),
				},
			},
			ConversionReviewVersions: []string{hcov1.APIVersionV1, hcov1beta1.APIVersionBeta},
		},
	}

	if caBundle != nil {
		crd.Spec.Conversion.Webhook.ClientConfig.CABundle = caBundle
	}

	return r.client.Update(ctx, crd)
}

func (r *ReconcileConversionWebhook) getOLMCaBundle(ctx context.Context) ([]byte, error) {
	secret := &corev1.Secret{}
	err := r.client.Get(ctx, client.ObjectKey{Name: r.secretName, Namespace: r.namespace}, secret)
	if err != nil {
		return nil, err
	}

	caBundle, ok := secret.Data[olmCaBundleKey]
	if !ok {
		return nil, errors.New("can't find caBundle")
	}

	return caBundle, nil
}
