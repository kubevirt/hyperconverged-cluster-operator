package tests_test

import (
	"context"
	"encoding/json"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	tests "github.com/kubevirt/hyperconverged-cluster-operator/tests/func-tests"
)

const (
	deschedulerCRDName = "kubedeschedulers.operator.openshift.io"
	graduatedProfile   = "KubeVirtRelieveAndMigrate"
	legacyProfile      = "DevKubeVirtRelieveAndMigrate"
	deschedulerMode    = "Automatic"
)

var _ = Describe("KubeVirt Descheduler documentation compatibility", Label("deschedulerCompatibility"), func() {
	tests.FlagParse()

	It("should expose the documented KubeVirt profile and fields", func(ctx context.Context) {
		cli := tests.GetControllerRuntimeClient()
		crd := &apiextensionsv1.CustomResourceDefinition{}
		if err := cli.Get(ctx, client.ObjectKey{Name: deschedulerCRDName}, crd); apierrors.IsNotFound(err) {
			Skip(fmt.Sprintf("optional Descheduler plugin is not installed; CRD %q was not found", deschedulerCRDName))
		} else {
			Expect(err).ToNot(HaveOccurred())
		}

		schema := servedDeschedulerSchema(crd)
		Expect(schema).ToNot(BeNil(), "the Descheduler CRD has no served OpenAPI schema")

		spec, ok := schema.Properties["spec"]
		Expect(ok).To(BeTrue(), "KubeDescheduler.spec is missing from the served schema")
		profiles, ok := spec.Properties["profiles"]
		Expect(ok).To(BeTrue(), "KubeDescheduler.spec.profiles is missing from the served schema")
		Expect(profiles.Items).ToNot(BeNil(), "KubeDescheduler.spec.profiles has no item schema")
		Expect(profileEnum(profiles.Items)).To(Or(ContainElement(graduatedProfile), ContainElement(legacyProfile)),
			"neither the graduated nor the legacy KubeVirt profile is accepted")

		customizations, ok := spec.Properties["profileCustomizations"]
		Expect(ok).To(BeTrue(), "KubeDescheduler.spec.profileCustomizations is missing from the served schema")
		backgroundEvictions, ok := customizations.Properties["devEnableEvictionsInBackground"]
		Expect(ok).To(BeTrue(), "the documented background-eviction field is missing from the served schema")
		Expect(backgroundEvictions.Type).To(Equal("boolean"),
			"the documented background-eviction field is no longer boolean")

		mode, ok := spec.Properties["mode"]
		Expect(ok).To(BeTrue(), "KubeDescheduler.spec.mode is missing from the served schema")
		Expect(jsonEnum(mode.Enum)).To(ContainElement(deschedulerMode),
			"the documented Automatic Descheduler mode is no longer accepted")
	})
})

func servedDeschedulerSchema(crd *apiextensionsv1.CustomResourceDefinition) *apiextensionsv1.JSONSchemaProps {
	for _, version := range crd.Spec.Versions {
		if version.Served && version.Schema != nil && version.Schema.OpenAPIV3Schema != nil {
			return version.Schema.OpenAPIV3Schema
		}
	}
	return nil
}

func profileEnum(items *apiextensionsv1.JSONSchemaPropsOrArray) []string {
	if items == nil {
		return nil
	}
	if items.Schema != nil {
		return jsonEnum(items.Schema.Enum)
	}
	for i := range items.JSONSchemas {
		if len(items.JSONSchemas[i].Enum) > 0 {
			return jsonEnum(items.JSONSchemas[i].Enum)
		}
	}
	return nil
}

func jsonEnum(values []apiextensionsv1.JSON) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		var decoded string
		if err := json.Unmarshal(value.Raw, &decoded); err == nil {
			result = append(result, decoded)
		}
	}
	return result
}
