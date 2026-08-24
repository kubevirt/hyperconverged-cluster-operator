package tests_test

import (
	"context"
	"encoding/json"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	tests "github.com/kubevirt/hyperconverged-cluster-operator/tests/func-tests"
)

const (
	hcoCRDName = "hyperconvergeds.hco.kubevirt.io"
	vmCRDName  = "virtualmachines.kubevirt.io"
)

var _ = Describe("KubeVirt documentation API compatibility", Label("documentationCompatibility"), func() {
	tests.FlagParse()

	It("should expose the eviction strategy fields used by the node maintenance howto", func(ctx context.Context) {
		cli := tests.GetControllerRuntimeClient()

		hcoSchema := getServedCRDSchema(ctx, cli, hcoCRDName)
		hcoEviction := schemaProperty(hcoSchema, "spec", "evictionStrategy")
		Expect(hcoEviction).ToNot(BeNil(),
			"the node maintenance howto uses hco.spec.evictionStrategy")
		Expect(documentationJSONEnum(hcoEviction.Enum)).To(ContainElement("LiveMigrate"),
			"hco.spec.evictionStrategy no longer accepts LiveMigrate")

		vmSchema := getServedCRDSchema(ctx, cli, vmCRDName)
		vmEviction := schemaProperty(vmSchema, "spec", "template", "spec", "evictionStrategy")
		Expect(vmEviction).ToNot(BeNil(),
			"the node maintenance howto uses vm.spec.template.spec.evictionStrategy")
		Expect(vmEviction.Description).To(ContainSubstring("LiveMigrate"),
			"vm.spec.template.spec.evictionStrategy no longer documents LiveMigrate")
	})
})

func getServedCRDSchema(ctx context.Context, cli client.Client, name string) *apiextensionsv1.JSONSchemaProps {
	crd := &apiextensionsv1.CustomResourceDefinition{}
	ExpectWithOffset(1, cli.Get(ctx, client.ObjectKey{Name: name}, crd)).To(Succeed(),
		fmt.Sprintf("CRD %q must be installed for KubeVirt documentation compatibility checks", name))
	for _, version := range crd.Spec.Versions {
		if version.Served && version.Schema != nil && version.Schema.OpenAPIV3Schema != nil {
			return version.Schema.OpenAPIV3Schema
		}
	}
	Fail(fmt.Sprintf("CRD %q has no served OpenAPI schema", name))
	return nil
}

func schemaProperty(schema *apiextensionsv1.JSONSchemaProps, path ...string) *apiextensionsv1.JSONSchemaProps {
	for _, field := range path {
		if schema == nil {
			return nil
		}
		next, ok := schema.Properties[field]
		if !ok {
			return nil
		}
		schema = &next
	}
	return schema
}

func documentationJSONEnum(values []apiextensionsv1.JSON) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		var decoded string
		if err := json.Unmarshal(value.Raw, &decoded); err == nil {
			result = append(result, decoded)
		}
	}
	return result
}
