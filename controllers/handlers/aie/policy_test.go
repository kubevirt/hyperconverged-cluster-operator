package aie

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcofg "github.com/kubevirt/hyperconverged-cluster-operator/api/v1/featuregates"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
)

const testLauncherImage = "registry.example/virt-launcher-4-nv@sha256:0123456789abcdef"

var _ = Describe("AIE launcher admission policy", func() {

	DescribeTable("ignores the retired annotation", func(annotation string) {
		hco := commontestutils.NewHco()
		hco.Annotations = map[string]string{"hco.kubevirt.io/deployAIE": annotation}
		cli := commontestutils.InitClient([]client.Object{hco})
		req := commontestutils.NewReq(hco)
		Expect(NewLauncherPolicyHandler(cli, commontestutils.GetScheme(), testLauncherImage).Ensure(req).Err).NotTo(HaveOccurred())
		Expect(NewLauncherBindingHandler(cli, commontestutils.GetScheme()).Ensure(req).Err).NotTo(HaveOccurred())
		policyErr := cli.Get(req.Ctx, client.ObjectKey{Name: policyName}, &admissionv1.MutatingAdmissionPolicy{})
		bindingErr := cli.Get(req.Ctx, client.ObjectKey{Name: bindingName}, &admissionv1.MutatingAdmissionPolicyBinding{})
		Expect(policyErr).To(MatchError(apierrors.IsNotFound, "not found"))
		Expect(bindingErr).To(MatchError(apierrors.IsNotFound, "not found"))
	}, Entry("true", "true"), Entry("false", "false"), Entry("missing", ""), Entry("other value", "TRUE"))

	It("uses nvidiaAIE for both policy and binding", func() {
		hco := commontestutils.NewHco()
		hco.Spec.FeatureGates.Enable(hcofg.NVIDIAAIE)
		cli := commontestutils.InitClient([]client.Object{hco})
		req := commontestutils.NewReq(hco)
		policy := NewLauncherPolicyHandler(cli, commontestutils.GetScheme(), testLauncherImage)
		binding := NewLauncherBindingHandler(cli, commontestutils.GetScheme())
		Expect(policy.Ensure(req).Err).NotTo(HaveOccurred())
		Expect(binding.Ensure(req).Err).NotTo(HaveOccurred())
		Expect(cli.Get(req.Ctx, client.ObjectKey{Name: policyName}, &admissionv1.MutatingAdmissionPolicy{})).To(Succeed())
		Expect(cli.Get(req.Ctx, client.ObjectKey{Name: bindingName}, &admissionv1.MutatingAdmissionPolicyBinding{})).To(Succeed())
		hco.Annotations = map[string]string{"hco.kubevirt.io/deployAIE": "true"}
		hco.Spec.FeatureGates.Disable(hcofg.NVIDIAAIE)
		Expect(binding.Ensure(req).Err).NotTo(HaveOccurred())
		Expect(policy.Ensure(req).Err).NotTo(HaveOccurred())
		Expect(cli.Get(req.Ctx, client.ObjectKey{Name: policyName}, &admissionv1.MutatingAdmissionPolicy{})).To(MatchError(apierrors.IsNotFound, "not found"))
		Expect(cli.Get(req.Ctx, client.ObjectKey{Name: bindingName}, &admissionv1.MutatingAdmissionPolicyBinding{})).To(MatchError(apierrors.IsNotFound, "not found"))
	})

	It("reconciles image changes and external drift while preserving unrelated labels", func() {
		hco := commontestutils.NewHco()
		hco.Spec.FeatureGates.Enable(hcofg.NVIDIAAIE)
		cli := commontestutils.InitClient([]client.Object{hco})
		req := commontestutils.NewReq(hco)
		handler := NewLauncherPolicyHandler(cli, commontestutils.GetScheme(), testLauncherImage)
		Expect(handler.Ensure(req).Err).NotTo(HaveOccurred())
		found := &admissionv1.MutatingAdmissionPolicy{}
		key := client.ObjectKey{Name: policyName}
		Expect(cli.Get(req.Ctx, key, found)).To(Succeed())
		original := found.Spec.DeepCopy()
		found.Labels["user.example/keep"] = "yes"
		found.Spec.MatchConstraints.ObjectSelector = &metav1.LabelSelector{}
		Expect(cli.Update(req.Ctx, found)).To(Succeed())
		result := handler.Ensure(req)
		Expect(result.Err).NotTo(HaveOccurred())
		Expect(result.Updated).To(BeTrue())
		Expect(cli.Get(req.Ctx, key, found)).To(Succeed())
		Expect(found.Spec).To(Equal(*original))
		Expect(found.Labels).To(HaveKeyWithValue("user.example/keep", "yes"))
		Expect(handler.Ensure(req).Updated).To(BeFalse())
		handler = NewLauncherPolicyHandler(cli, commontestutils.GetScheme(), "registry.example/virt-launcher-4-nv:next")
		Expect(handler.Ensure(req).Err).NotTo(HaveOccurred())
		Expect(cli.Get(req.Ctx, key, found)).To(Succeed())
		Expect(found.Spec.Mutations[0].ApplyConfiguration.Expression).To(ContainSubstring("registry.example/virt-launcher-4-nv:next"))
		Expect(found.Spec.Mutations[0].ApplyConfiguration.Expression).NotTo(ContainSubstring(testLauncherImage))
	})

	DescribeTable("removes the operand and related-object reference on disable", func(newHandler func(client.Client) operands.Operand, object client.Object) {
		hco := commontestutils.NewHco()
		hco.Spec.FeatureGates.Enable(hcofg.NVIDIAAIE)
		cli := commontestutils.InitClient([]client.Object{hco})
		req := commontestutils.NewReq(hco)
		handler := newHandler(cli)
		Expect(handler.Ensure(req).Err).NotTo(HaveOccurred())
		Expect(handler.Ensure(req).Err).NotTo(HaveOccurred())
		Expect(hco.Status.RelatedObjects).NotTo(BeEmpty())
		hco.Spec.FeatureGates.Disable(hcofg.NVIDIAAIE)
		Expect(handler.Ensure(req).Err).NotTo(HaveOccurred())
		Expect(cli.Get(req.Ctx, client.ObjectKeyFromObject(object), object)).To(MatchError(apierrors.IsNotFound, "not found"))
		Expect(hco.Status.RelatedObjects).To(BeEmpty())
		Expect(handler.Ensure(req).Err).NotTo(HaveOccurred())
	},
		Entry("policy", func(cli client.Client) operands.Operand {
			return NewLauncherPolicyHandler(cli, commontestutils.GetScheme(), testLauncherImage)
		}, client.Object(newLauncherPolicyWithNameOnly())),
		Entry("binding", func(cli client.Client) operands.Operand {
			return NewLauncherBindingHandler(cli, commontestutils.GetScheme())
		}, client.Object(newLauncherBinding())),
	)
})
