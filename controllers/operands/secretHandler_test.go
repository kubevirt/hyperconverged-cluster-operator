package operands

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

var _ = Describe("Secret handler", func() {
	var (
		hco *hcov1.HyperConverged
		req *common.HcoRequest
	)

	BeforeEach(func() {
		hco = commontestutils.NewHco()
		req = commontestutils.NewReq(hco)
	})

	newSecret := func() *corev1.Secret {
		return &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "test",
				Namespace:   commontestutils.Namespace,
				Labels:      GetLabels(hcoutil.AppComponentDeployment),
				Annotations: map[string]string{corev1.ServiceAccountNameKey: "test"},
			},
			Type: corev1.SecretTypeServiceAccountToken,
		}
	}

	It("should create the Secret", func() {
		cl := commontestutils.InitClient([]client.Object{hco})
		res := NewSecretHandler(cl, commontestutils.GetScheme(), newSecret()).Ensure(req)
		Expect(res.Err).ToNot(HaveOccurred())
		Expect(res.Created).To(BeTrue())
	})

	It("should keep the data and annotations others added", func() {
		existing := newSecret()
		existing.Annotations[corev1.ServiceAccountUIDKey] = "uid"
		existing.Data = map[string][]byte{"token": []byte("token")}
		cl := commontestutils.InitClient([]client.Object{hco, existing})

		res := NewSecretHandler(cl, commontestutils.GetScheme(), newSecret()).Ensure(req)
		Expect(res.Err).ToNot(HaveOccurred())
		Expect(res.Updated).To(BeFalse())
	})

	It("should restore a changed annotation", func() {
		existing := newSecret()
		existing.Annotations[corev1.ServiceAccountNameKey] = "other"
		cl := commontestutils.InitClient([]client.Object{hco, existing})

		res := NewSecretHandler(cl, commontestutils.GetScheme(), newSecret()).Ensure(req)
		Expect(res.Err).ToNot(HaveOccurred())
		Expect(res.Updated).To(BeTrue())

		found := &corev1.Secret{}
		Expect(cl.Get(context.Background(), client.ObjectKeyFromObject(existing), found)).To(Succeed())
		Expect(found.Annotations).To(HaveKeyWithValue(corev1.ServiceAccountNameKey, "test"))
	})
})
