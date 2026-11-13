package aie

import (
	"context"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

const testLauncherImage = "registry.example/virt-launcher-4-nv@sha256:0123456789abcdef"

var _ = Describe("AIE launcher admission policy", func() {
	BeforeEach(func() {
		original, present := os.LookupEnv(hcoutil.AIELauncherImageEnvV)
		Expect(os.Setenv(hcoutil.AIELauncherImageEnvV, testLauncherImage)).To(Succeed())
		DeferCleanup(func() {
			if present {
				Expect(os.Setenv(hcoutil.AIELauncherImageEnvV, original)).To(Succeed())
			} else {
				Expect(os.Unsetenv(hcoutil.AIELauncherImageEnvV)).To(Succeed())
			}
		})
	})

	DescribeTable("only activates with the explicit true annotation", func(annotation string, enabled bool) {
		hco := commontestutils.NewHco()
		hco.Annotations = map[string]string{DeployAIEAnnotation: annotation}
		cli := commontestutils.InitClient([]client.Object{hco})
		req := commontestutils.NewReq(hco)
		Expect(NewLauncherPolicyHandler(cli, commontestutils.GetScheme()).Ensure(req).Err).NotTo(HaveOccurred())
		Expect(NewLauncherBindingHandler(cli, commontestutils.GetScheme()).Ensure(req).Err).NotTo(HaveOccurred())
		policyErr := cli.Get(req.Ctx, client.ObjectKey{Name: policyName}, &admissionv1.MutatingAdmissionPolicy{})
		bindingErr := cli.Get(req.Ctx, client.ObjectKey{Name: bindingName}, &admissionv1.MutatingAdmissionPolicyBinding{})
		if enabled {
			Expect(policyErr).NotTo(HaveOccurred())
			Expect(bindingErr).NotTo(HaveOccurred())
		} else {
			Expect(policyErr).To(MatchError(apierrors.IsNotFound, "not found"))
			Expect(bindingErr).To(MatchError(apierrors.IsNotFound, "not found"))
		}
	}, Entry("true", "true", true), Entry("false", "false", false), Entry("missing", "", false), Entry("other value", "TRUE", false))

	It("fails before activating a policy if the bundle has no launcher image", func() {
		Expect(os.Unsetenv(hcoutil.AIELauncherImageEnvV)).To(Succeed())
		hco := commontestutils.NewHco()
		hco.Annotations = map[string]string{DeployAIEAnnotation: "true"}
		cli := commontestutils.InitClient([]client.Object{hco})
		result := NewLauncherPolicyHandler(cli, commontestutils.GetScheme()).Ensure(commontestutils.NewReq(hco))
		Expect(result.Err).To(MatchError(ContainSubstring(hcoutil.AIELauncherImageEnvV)))
		Expect(cli.Get(context.Background(), client.ObjectKey{Name: policyName}, &admissionv1.MutatingAdmissionPolicy{})).To(MatchError(apierrors.IsNotFound, "not found"))
	})

	It("reconciles image changes and external drift while preserving unrelated labels", func() {
		hco := commontestutils.NewHco()
		hco.Annotations = map[string]string{DeployAIEAnnotation: "true"}
		cli := commontestutils.InitClient([]client.Object{hco})
		req := commontestutils.NewReq(hco)
		handler := NewLauncherPolicyHandler(cli, commontestutils.GetScheme())
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
		Expect(os.Setenv(hcoutil.AIELauncherImageEnvV, "registry.example/virt-launcher-4-nv:next")).To(Succeed())
		Expect(handler.Ensure(req).Err).NotTo(HaveOccurred())
		Expect(cli.Get(req.Ctx, key, found)).To(Succeed())
		Expect(found.Spec.Mutations[0].ApplyConfiguration.Expression).To(ContainSubstring("registry.example/virt-launcher-4-nv:next"))
		Expect(found.Spec.Mutations[0].ApplyConfiguration.Expression).NotTo(ContainSubstring(testLauncherImage))
	})

	DescribeTable("removes the operand and related-object reference on disable", func(newHandler func(client.Client) operands.Operand, object client.Object) {
		hco := commontestutils.NewHco()
		hco.Annotations = map[string]string{DeployAIEAnnotation: "true"}
		cli := commontestutils.InitClient([]client.Object{hco})
		req := commontestutils.NewReq(hco)
		handler := newHandler(cli)
		Expect(handler.Ensure(req).Err).NotTo(HaveOccurred())
		Expect(handler.Ensure(req).Err).NotTo(HaveOccurred())
		Expect(hco.Status.RelatedObjects).NotTo(BeEmpty())
		delete(hco.Annotations, DeployAIEAnnotation)
		Expect(handler.Ensure(req).Err).NotTo(HaveOccurred())
		Expect(cli.Get(req.Ctx, client.ObjectKeyFromObject(object), object)).To(MatchError(apierrors.IsNotFound, "not found"))
		Expect(hco.Status.RelatedObjects).To(BeEmpty())
		Expect(handler.Ensure(req).Err).NotTo(HaveOccurred())
	},
		Entry("policy", func(cli client.Client) operands.Operand {
			return NewLauncherPolicyHandler(cli, commontestutils.GetScheme())
		}, client.Object(newLauncherPolicy())),
		Entry("binding", func(cli client.Client) operands.Operand {
			return NewLauncherBindingHandler(cli, commontestutils.GetScheme())
		}, client.Object(newLauncherBinding())),
	)
})
