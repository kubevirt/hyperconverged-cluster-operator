package aie

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/reference"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

var _ = Describe("Retired AIE webhook cleanup", func() {
	DescribeTable("removes legacy resources and stale status independently of the annotation", func(enabled bool) {
		hco := commontestutils.NewHco()
		if enabled {
			hco.Annotations = map[string]string{DeployAIEAnnotation: "true"}
		}
		objects := legacyWebhookObjects(hco.Namespace)
		for _, object := range objects {
			object.SetLabels(operands.GetLabels(hcoutil.AppComponentAIE))
			ref, err := reference.GetReference(commontestutils.GetScheme(), object)
			Expect(err).NotTo(HaveOccurred())
			hco.Status.RelatedObjects = append(hco.Status.RelatedObjects, *ref)
		}
		hco.Status.RelatedObjects = append(hco.Status.RelatedObjects, corev1.ObjectReference{Kind: "Service", Name: "keep", Namespace: hco.Namespace})
		meta.SetStatusCondition(&hco.Status.Conditions, metav1.Condition{Type: "KubevirtAieWebhookReady", Status: metav1.ConditionTrue, Reason: "OldDeploymentReady"})
		cli := commontestutils.InitClient(append(objects, hco))
		req := commontestutils.NewReq(hco)
		cleaner := NewLegacyWebhookCleaner(cli, commontestutils.GetScheme())
		Expect(cleaner.Ensure(req).Err).NotTo(HaveOccurred())
		for _, object := range objects {
			Expect(cli.Get(req.Ctx, client.ObjectKeyFromObject(object), object)).To(MatchError(apierrors.IsNotFound, "not found"))
		}
		Expect(hco.Status.RelatedObjects).To(ConsistOf(corev1.ObjectReference{Kind: "Service", Name: "keep", Namespace: hco.Namespace}))
		Expect(meta.FindStatusCondition(hco.Status.Conditions, "KubevirtAieWebhookReady")).To(BeNil())
		Expect(req.StatusDirty).To(BeTrue())
		Expect(cleaner.Ensure(req).Err).NotTo(HaveOccurred())
	}, Entry("enabled", true), Entry("disabled", false))

	It("preserves resources that are not managed by this HCO", func() {
		hco := commontestutils.NewHco()
		foreign := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "kubevirt-aie-launcher-config", Namespace: hco.Namespace}, Data: map[string]string{"keep": "yes"}}
		cli := commontestutils.InitClient([]client.Object{hco, foreign})
		req := commontestutils.NewReq(hco)
		Expect(NewLegacyWebhookCleaner(cli, commontestutils.GetScheme()).Ensure(req).Err).NotTo(HaveOccurred())
		found := &corev1.ConfigMap{}
		Expect(cli.Get(req.Ctx, client.ObjectKeyFromObject(foreign), found)).To(Succeed())
		Expect(found.Data).To(Equal(foreign.Data))
	})

	It("keeps the server available if its admission registration cannot be removed", func() {
		hco := commontestutils.NewHco()
		webhook := &admissionv1.MutatingWebhookConfiguration{ObjectMeta: metav1.ObjectMeta{Name: legacyWebhookName, Labels: operands.GetLabels(hcoutil.AppComponentAIE)}}
		deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: legacyWebhookName, Namespace: hco.Namespace, Labels: operands.GetLabels(hcoutil.AppComponentAIE)}}
		cli := commontestutils.InitClient([]client.Object{hco, webhook, deployment})
		expected := errors.New("cannot delete admission registration")
		cli.InitiateDeleteErrors(func(obj client.Object) error {
			if obj.GetName() == legacyWebhookName && obj.GetNamespace() == "" {
				return expected
			}
			return nil
		})
		req := commontestutils.NewReq(hco)
		Expect(NewLegacyWebhookCleaner(cli, commontestutils.GetScheme()).Ensure(req).Err).To(MatchError(expected))
		Expect(cli.Get(req.Ctx, client.ObjectKeyFromObject(deployment), &appsv1.Deployment{})).To(Succeed())
	})
	It("removes managed webhook NetworkPolicies with alternate names", func() {
		hco := commontestutils.NewHco()
		policy := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "legacy-aie-ingress", Namespace: hco.Namespace,
			Labels: map[string]string{hcoutil.AppLabel: hco.Name, hcoutil.AppLabelComponent: "aie-webhook"}}}
		foreign := policy.DeepCopy()
		foreign.Name = "other-component"
		foreign.Labels = map[string]string{hcoutil.AppLabel: hco.Name, hcoutil.AppLabelComponent: "keep"}
		cli := commontestutils.InitClient([]client.Object{hco, policy, foreign})
		req := commontestutils.NewReq(hco)
		Expect(NewLegacyWebhookCleaner(cli, commontestutils.GetScheme()).Ensure(req).Err).NotTo(HaveOccurred())
		Expect(cli.Get(req.Ctx, client.ObjectKeyFromObject(policy), &networkingv1.NetworkPolicy{})).To(MatchError(apierrors.IsNotFound, "not found"))
		Expect(cli.Get(req.Ctx, client.ObjectKeyFromObject(foreign), &networkingv1.NetworkPolicy{})).To(Succeed())
	})

	It("preserves the server while an unowned registration remains", func() {
		hco := commontestutils.NewHco()
		webhook := &admissionv1.MutatingWebhookConfiguration{ObjectMeta: metav1.ObjectMeta{Name: legacyWebhookName}}
		deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: legacyWebhookName, Namespace: hco.Namespace, Labels: operands.GetLabels(hcoutil.AppComponentAIE)}}
		cli := commontestutils.InitClient([]client.Object{hco, webhook, deployment})
		req := commontestutils.NewReq(hco)
		Expect(NewLegacyWebhookCleaner(cli, commontestutils.GetScheme()).Ensure(req).Err).To(MatchError(ContainSubstring("refusing to remove its server")))
		Expect(cli.Get(req.Ctx, client.ObjectKeyFromObject(deployment), &appsv1.Deployment{})).To(Succeed())
	})

})
