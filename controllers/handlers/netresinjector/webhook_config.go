package netresinjector

import (
	"errors"
	"maps"
	"reflect"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubevirtcorev1 "kubevirt.io/api/core/v1"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

func NewMutatingWebhookConfigurationHandler(cli client.Client, scheme *runtime.Scheme) operands.Operand {
	return operands.NewConditionalHandler(
		operands.NewGenericOperand(cli, scheme, "MutatingWebhookConfiguration", &mwcHooks{required: newMutatingWebhookConfiguration()}, false),
		shouldDeploy,
		func(hc *hcov1.HyperConverged) client.Object {
			return NewMutatingWebhookConfigurationWithNameOnly()
		},
	)
}

type mwcHooks struct {
	required *admissionregistrationv1.MutatingWebhookConfiguration
}

func (h *mwcHooks) GetFullCr(_ *hcov1.HyperConverged) (client.Object, error) {
	return h.required.DeepCopy(), nil
}

func (h *mwcHooks) GetEmptyCr() client.Object {
	return &admissionregistrationv1.MutatingWebhookConfiguration{}
}

func (h *mwcHooks) UpdateCR(req *common.HcoRequest, Client client.Client, exists runtime.Object, required runtime.Object) (bool, bool, error) {
	mwc, ok1 := required.(*admissionregistrationv1.MutatingWebhookConfiguration)
	found, ok2 := exists.(*admissionregistrationv1.MutatingWebhookConfiguration)
	if !ok1 || !ok2 {
		return false, false, errors.New("can't convert to MutatingWebhookConfiguration")
	}

	preserveExistingCABundles(mwc, found)

	if !needsUpdate(mwc, found) {
		return false, false, nil
	}

	logUpdate(req, found.Name)
	applyRequiredChanges(mwc, found)

	if err := Client.Update(req.Ctx, found); err != nil {
		return false, false, err
	}
	return true, !req.HCOTriggered, nil
}

// preserveExistingCABundles copies CA bundle values from existing webhooks into required webhooks.
// This prevents wiping out CA bundles that were injected by the cluster (e.g., service-ca operator
// on OpenShift or cert-manager on non-OpenShift).
func preserveExistingCABundles(required, existing *admissionregistrationv1.MutatingWebhookConfiguration) {
	for i := range required.Webhooks {
		for j := range existing.Webhooks {
			if required.Webhooks[i].Name == existing.Webhooks[j].Name &&
				len(required.Webhooks[i].ClientConfig.CABundle) == 0 {
				required.Webhooks[i].ClientConfig.CABundle = existing.Webhooks[j].ClientConfig.CABundle
				break
			}
		}
	}
}

// needsUpdate determines if the existing webhook configuration needs to be updated.
func needsUpdate(required, existing *admissionregistrationv1.MutatingWebhookConfiguration) bool {
	return !hcoutil.CompareLabels(required, existing) ||
		!reflect.DeepEqual(required.Webhooks, existing.Webhooks) ||
		!hasRequiredAnnotations(existing.Annotations, required.Annotations)
}

// logUpdate logs the update action based on whether it was HCO-triggered or externally triggered.
func logUpdate(req *common.HcoRequest, name string) {
	if req.HCOTriggered {
		req.Logger.Info("Updating existing MutatingWebhookConfiguration to new opinionated values", "name", name)
	} else {
		req.Logger.Info("Reconciling an externally updated MutatingWebhookConfiguration to its opinionated values", "name", name)
	}
}

// applyRequiredChanges applies the required labels, webhooks, and annotations to the existing object.
func applyRequiredChanges(required, existing *admissionregistrationv1.MutatingWebhookConfiguration) {
	hcoutil.MergeLabels(&required.ObjectMeta, &existing.ObjectMeta)

	existing.Webhooks = make([]admissionregistrationv1.MutatingWebhook, len(required.Webhooks))
	for i := range required.Webhooks {
		required.Webhooks[i].DeepCopyInto(&existing.Webhooks[i])
	}

	mergeAnnotations(required, existing)
}

func mergeAnnotations(required, existing *admissionregistrationv1.MutatingWebhookConfiguration) {
	if required.Annotations == nil {
		return
	}

	if existing.Annotations == nil {
		existing.Annotations = make(map[string]string)
	}

	maps.Copy(existing.Annotations, required.Annotations)
}

func NewMutatingWebhookConfigurationWithNameOnly() *admissionregistrationv1.MutatingWebhookConfiguration {
	return &admissionregistrationv1.MutatingWebhookConfiguration{
		TypeMeta: metav1.TypeMeta{
			APIVersion: admissionregistrationv1.SchemeGroupVersion.String(),
			Kind:       "MutatingWebhookConfiguration",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   webhookConfigName,
			Labels: operands.GetLabels(hcoutil.AppComponentNetResInjector),
		},
	}
}

func newMutatingWebhookConfiguration() *admissionregistrationv1.MutatingWebhookConfiguration {
	mwc := NewMutatingWebhookConfigurationWithNameOnly()

	if hcoutil.GetClusterInfo().IsOpenshift() {
		mwc.Annotations = map[string]string{
			"service.beta.openshift.io/inject-cabundle": "true",
		}
	} else {
		mwc.Annotations = map[string]string{
			"cert-manager.io/inject-ca-from": hcoutil.GetOperatorNamespaceFromEnv() + "/" + tlsCertificateName,
		}
	}

	mwc.Webhooks = []admissionregistrationv1.MutatingWebhook{
		{
			Name:                    "cnv-network-resources-injector-config.k8s.io",
			AdmissionReviewVersions: []string{"v1", "v1beta1"},
			SideEffects:             new(admissionregistrationv1.SideEffectClassNone),
			FailurePolicy:           new(admissionregistrationv1.Ignore),
			TimeoutSeconds:          new(int32(10)),
			MatchPolicy:             new(admissionregistrationv1.Equivalent),
			ReinvocationPolicy:      new(admissionregistrationv1.NeverReinvocationPolicy),
			ObjectSelector:          &metav1.LabelSelector{},
			ClientConfig: admissionregistrationv1.WebhookClientConfig{
				Service: &admissionregistrationv1.ServiceReference{
					Name:      serviceName,
					Namespace: hcoutil.GetOperatorNamespaceFromEnv(),
					Path:      new("/mutate"),
					Port:      new(int32(443)),
				},
			},
			NamespaceSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "kubernetes.io/metadata.name",
						Operator: metav1.LabelSelectorOpNotIn,
						Values:   []string{"kube-system"},
					},
				},
			},
			MatchConditions: []admissionregistrationv1.MatchCondition{
				{
					Name:       "isVirtLauncherPod",
					Expression: `has(object.metadata.labels) && object.metadata.labels["` + kubevirtcorev1.AppLabel + `"] == "virt-launcher"`,
				},
				{
					Name:       "hasMultusAnnotation",
					Expression: `has(object.metadata.annotations) && "k8s.v1.cni.cncf.io/networks" in object.metadata.annotations`,
				},
			},

			Rules: []admissionregistrationv1.RuleWithOperations{
				{
					Operations: []admissionregistrationv1.OperationType{
						admissionregistrationv1.Create,
					},
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{""},
						APIVersions: []string{"v1"},
						Resources:   []string{"pods"},
						Scope:       new(admissionregistrationv1.AllScopes),
					},
				},
			},
		},
	}

	return mwc
}

func hasRequiredAnnotations(existing, required map[string]string) bool {
	for k, v := range required {
		if existing[k] != v {
			return false
		}
	}
	return true
}
