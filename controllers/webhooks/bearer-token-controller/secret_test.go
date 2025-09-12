package bearer_token_controller

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
)

var _ = Describe("newSecret", func() {
	It("should builds the expected Secret object", func() {
		owner := metav1.OwnerReference{Name: "o"}
		sec := newSecret(commontestutils.Namespace, owner, "token content")
		Expect(sec.Name).To(Equal(secretName))
		Expect(sec.Namespace).To(Equal(commontestutils.Namespace))
		Expect(sec.Labels).To(Equal(getLabels()))
		Expect(sec.StringData).To(HaveKeyWithValue("token", "token content"))
		Expect(sec.OwnerReferences).To(ContainElement(owner))
	})
})
