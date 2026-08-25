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

		hcoSchema, hcoVersion, err := getStorageServedCRDSchema(ctx, cli, hcoCRDName)
		Expect(err).ToNot(HaveOccurred())
		Expect(hcoVersion).ToNot(BeEmpty(), "the documented kubectl explain command requires a served HCO API version")
		// The howto discovers the storage+served version at runtime. Keep this
		// check version-agnostic so an upstream HCO storage-version change does
		// not fail the compatibility suite before the documented field is tested.
		hcoEviction := schemaProperty(hcoSchema, "spec", "evictionStrategy")
		if hcoEviction == nil {
			hcoEviction = schemaProperty(hcoSchema, "spec", "virtualization", "evictionStrategy")
		}
		Expect(hcoEviction).ToNot(BeNil(),
			"the node maintenance howto has no supported HCO evictionStrategy path")
		Expect(documentationJSONEnum(hcoEviction.Enum)).To(ContainElement("LiveMigrate"),
			"the documented HCO evictionStrategy path no longer accepts LiveMigrate")

		vmSchema, _, err := getStorageServedCRDSchema(ctx, cli, vmCRDName)
		Expect(err).ToNot(HaveOccurred())
		vmEviction := schemaProperty(vmSchema, "spec", "template", "spec", "evictionStrategy")
		Expect(vmEviction).ToNot(BeNil(),
			"the node maintenance howto uses vm.spec.template.spec.evictionStrategy")
		if vmEviction.Enum != nil {
			Expect(documentationJSONEnum(vmEviction.Enum)).To(ContainElement("LiveMigrate"),
				"vm.spec.template.spec.evictionStrategy enum no longer accepts LiveMigrate")
		}
		Expect(vmEviction.Description).To(ContainSubstring("LiveMigrate"),
			"vm.spec.template.spec.evictionStrategy no longer documents LiveMigrate")
	})
})

func getStorageServedCRDSchema(ctx context.Context, cli client.Client, name string) (*apiextensionsv1.JSONSchemaProps, string, error) {
	GinkgoHelper()
	crd := new(apiextensionsv1.CustomResourceDefinition)
	if err := cli.Get(ctx, client.ObjectKey{Name: name}, crd); err != nil {
		return nil, "", fmt.Errorf("CRD %q must be installed for KubeVirt documentation compatibility checks: %w", name, err)
	}
	for _, version := range crd.Spec.Versions {
		if version.Storage && version.Served && version.Schema != nil && version.Schema.OpenAPIV3Schema != nil {
			return version.Schema.OpenAPIV3Schema, version.Name, nil
		}
	}
	return nil, "", fmt.Errorf("CRD %q has no storage=true, served=true OpenAPI schema", name)
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
