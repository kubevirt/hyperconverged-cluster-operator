package handlers

import (
	"context"
	"encoding/json"

	jsonpatch "github.com/evanphx/json-patch/v5"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

var _ = Describe("VMExport PUDN MutatingAdmissionPolicy", func() {
	It("has tightly scoped matching and a conditional JSONPatch mutation", func() {
		policy := newVMExportOpenDefaultPortsPolicy()

		Expect(policy.Name).To(Equal(vmExportOpenDefaultPortsPolicyName))
		Expect(policy.Labels).To(Equal(operands.GetLabels(hcoutil.AppComponentDeployment)))
		Expect(policy.Spec.MatchConstraints).NotTo(BeNil())
		Expect(policy.Spec.MatchConstraints.MatchPolicy).To(HaveValue(Equal(admissionregistrationv1.Equivalent)))
		Expect(policy.Spec.MatchConstraints.NamespaceSelector).To(Equal(&metav1.LabelSelector{}))
		Expect(policy.Spec.MatchConstraints.ObjectSelector).To(Equal(&metav1.LabelSelector{}))
		Expect(policy.Spec.MatchConstraints.ResourceRules).To(HaveLen(1))

		rule := policy.Spec.MatchConstraints.ResourceRules[0]
		Expect(rule.Operations).To(ConsistOf(admissionregistrationv1.Create))
		Expect(rule.APIGroups).To(ConsistOf("export.kubevirt.io"))
		Expect(rule.APIVersions).To(ConsistOf("v1", "v1beta1"))
		Expect(rule.Resources).To(ConsistOf("virtualmachineexports"))
		Expect(rule.Scope).To(HaveValue(Equal(admissionregistrationv1.NamespacedScope)))

		Expect(policy.Spec.MatchConditions).To(HaveLen(1))
		Expect(policy.Spec.MatchConditions[0]).To(Equal(admissionregistrationv1.MatchCondition{
			Name:       "vmexport-open-default-ports-absent",
			Expression: `!has(object.metadata.annotations) || !("k8s.ovn.org/open-default-ports" in object.metadata.annotations)`,
		}))
		Expect(policy.Spec.Mutations).To(HaveLen(1))
		Expect(policy.Spec.Mutations[0]).To(Equal(admissionregistrationv1.Mutation{
			PatchType: admissionregistrationv1.PatchTypeJSONPatch,
			JSONPatch: &admissionregistrationv1.JSONPatch{
				Expression: `has(object.metadata.annotations) ?
  [JSONPatch{
    op: "add",
    path: "/metadata/annotations/" + jsonpatch.escapeKey("k8s.ovn.org/open-default-ports"),
    value: "[{\"protocol\":\"tcp\",\"port\":8443}]"
  }] :
  [JSONPatch{
    op: "add",
    path: "/metadata/annotations",
    value: {"k8s.ovn.org/open-default-ports": "[{\"protocol\":\"tcp\",\"port\":8443}]"}
  }]`,
			},
		}))
	})

	It("binds only namespaces labeled as primary User Defined Network namespaces", func() {
		binding := newVMExportOpenDefaultPortsBinding()

		Expect(binding.Name).To(Equal(vmExportOpenDefaultPortsBindingName))
		Expect(binding.Labels).To(Equal(operands.GetLabels(hcoutil.AppComponentDeployment)))
		Expect(binding.Spec.PolicyName).To(Equal(vmExportOpenDefaultPortsPolicyName))
		Expect(binding.Spec.MatchResources).NotTo(BeNil())
		Expect(binding.Spec.MatchResources.MatchPolicy).To(HaveValue(Equal(admissionregistrationv1.Equivalent)))
		Expect(binding.Spec.MatchResources.ObjectSelector).To(Equal(&metav1.LabelSelector{}))
		Expect(binding.Spec.MatchResources.NamespaceSelector.MatchExpressions).To(ConsistOf(
			metav1.LabelSelectorRequirement{
				Key:      primaryUserDefinedNetworkLabel,
				Operator: metav1.LabelSelectorOpExists,
			},
		))
	})

	DescribeTable("creates and reconciles the cluster-scoped operand", func(
		newHandler func(client.Client) operands.Operand,
		newObject func() client.Object,
		getObject func() client.Object,
	) {
		hco := commontestutils.NewHco()
		cli := commontestutils.InitClient([]client.Object{hco})
		req := commontestutils.NewReq(hco)

		result := newHandler(cli).Ensure(req)
		Expect(result.Err).NotTo(HaveOccurred())
		Expect(result.Created).To(BeTrue())

		found := getObject()
		Expect(cli.Get(context.Background(), client.ObjectKeyFromObject(newObject()), found)).To(Succeed())

		// External changes are overwritten on the next HCO reconciliation while
		// unrelated labels are preserved.
		existing := found
		existing.SetLabels(map[string]string{"user.example/test": "preserve"})
		Expect(cli.Update(context.Background(), existing)).To(Succeed())

		result = newHandler(cli).Ensure(req)
		Expect(result.Err).NotTo(HaveOccurred())
		Expect(result.Updated).To(BeTrue())

		Expect(cli.Get(context.Background(), client.ObjectKeyFromObject(existing), found)).To(Succeed())
		Expect(hcoutil.CompareLabels(existing, found)).To(BeTrue())
	},
		Entry("policy", func(cli client.Client) operands.Operand {
			return NewVMExportOpenDefaultPortsPolicyHandler(cli, commontestutils.GetScheme())
		}, func() client.Object { return newVMExportOpenDefaultPortsPolicy() }, func() client.Object {
			return &admissionregistrationv1.MutatingAdmissionPolicy{}
		}),
		Entry("binding", func(cli client.Client) operands.Operand {
			return NewVMExportOpenDefaultPortsBindingHandler(cli, commontestutils.GetScheme())
		}, func() client.Object { return newVMExportOpenDefaultPortsBinding() }, func() client.Object {
			return &admissionregistrationv1.MutatingAdmissionPolicyBinding{}
		}),
	)

	DescribeTable("applies the intended JSONPatch branch", func(object map[string]any, patch []byte, expected map[string]any) {
		objectJSON, err := json.Marshal(object)
		Expect(err).NotTo(HaveOccurred())
		decoded, err := jsonpatch.DecodePatch(patch)
		Expect(err).NotTo(HaveOccurred())
		patchedJSON, err := decoded.Apply(objectJSON)
		Expect(err).NotTo(HaveOccurred())

		var actual map[string]any
		Expect(json.Unmarshal(patchedJSON, &actual)).To(Succeed())
		Expect(actual).To(Equal(expected))
	},
		Entry("creates metadata.annotations when absent",
			map[string]any{"metadata": map[string]any{}},
			[]byte(`[{"op":"add","path":"/metadata/annotations","value":{"k8s.ovn.org/open-default-ports":"[{\"protocol\":\"tcp\",\"port\":8443}]"}}]`),
			map[string]any{"metadata": map[string]any{"annotations": map[string]any{vmExportOpenDefaultPortsAnnotation: vmExportOpenDefaultPortsValue}}},
		),
		Entry("adds the escaped annotation key without replacing other annotations",
			map[string]any{"metadata": map[string]any{"annotations": map[string]any{"user.example/test": "keep"}}},
			[]byte(`[{"op":"add","path":"/metadata/annotations/k8s.ovn.org~1open-default-ports","value":"[{\"protocol\":\"tcp\",\"port\":8443}]"}]`),
			map[string]any{"metadata": map[string]any{"annotations": map[string]any{"user.example/test": "keep", vmExportOpenDefaultPortsAnnotation: vmExportOpenDefaultPortsValue}}},
		),
	)
})
