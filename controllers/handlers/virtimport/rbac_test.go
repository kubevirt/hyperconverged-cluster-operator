package virtimport

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	rbacv1 "k8s.io/api/rbac/v1"
	rbacvalidation "k8s.io/component-helpers/auth/rbac/validation"

	"github.com/kubevirt/hyperconverged-cluster-operator/tools/manifests"
)

var _ = Describe("virt-import RBAC", func() {
	// Kubernetes prevents privilege escalation: HCO can only create the virt-import roles if it
	// holds all their rules itself. A bundle bump that adds rules fails here instead of at runtime.
	It("should be covered by the HCO ClusterRole", func() {
		b := getBundle()

		var rules []rbacv1.PolicyRule
		for _, role := range b.ClusterRoles {
			rules = append(rules, role.Rules...)
		}
		for _, role := range b.Roles {
			rules = append(rules, role.Rules...)
		}
		Expect(rules).ToNot(BeEmpty())

		covered, missing := rbacvalidation.Covers(manifests.GetClusterPermissions(), rules)
		Expect(covered).To(BeTrue(), "rules missing in tools/manifests GetClusterPermissions(): %v", missing)
	})
})
