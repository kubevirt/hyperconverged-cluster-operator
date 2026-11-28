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

var _ = Describe("Service handler", func() {
	const servingCertAnnotation = "service.beta.openshift.io/serving-cert-secret-name"

	var (
		hco *hcov1.HyperConverged
		req *common.HcoRequest
	)

	BeforeEach(func() {
		hco = commontestutils.NewHco()
		req = commontestutils.NewReq(hco)
	})

	newService := func() *corev1.Service {
		return &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "test",
				Namespace:   commontestutils.Namespace,
				Labels:      GetLabels(hcoutil.AppComponentDeployment),
				Annotations: map[string]string{servingCertAnnotation: "test-cert"},
			},
			Spec: corev1.ServiceSpec{
				Selector: map[string]string{"app": "test"},
				Ports:    []corev1.ServicePort{{Name: "metrics", Port: 8443, Protocol: corev1.ProtocolTCP}},
			},
		}
	}

	It("should keep the annotations others added", func() {
		existing := newService()
		existing.Annotations["other"] = "value"
		cl := commontestutils.InitClient([]client.Object{hco, existing})

		res := NewServiceHandler(cl, commontestutils.GetScheme(), newService()).Ensure(req)
		Expect(res.Err).ToNot(HaveOccurred())
		Expect(res.Updated).To(BeFalse())
	})

	It("should restore a changed annotation", func() {
		existing := newService()
		existing.Annotations[servingCertAnnotation] = "old-cert"
		existing.Annotations["other"] = "value"
		cl := commontestutils.InitClient([]client.Object{hco, existing})

		res := NewServiceHandler(cl, commontestutils.GetScheme(), newService()).Ensure(req)
		Expect(res.Err).ToNot(HaveOccurred())
		Expect(res.Updated).To(BeTrue())

		found := &corev1.Service{}
		Expect(cl.Get(context.Background(), client.ObjectKeyFromObject(existing), found)).To(Succeed())
		Expect(found.Annotations).To(HaveKeyWithValue(servingCertAnnotation, "test-cert"))
		Expect(found.Annotations).To(HaveKeyWithValue("other", "value"))
	})

	It("should add a missing annotation", func() {
		existing := newService()
		existing.Annotations = nil
		cl := commontestutils.InitClient([]client.Object{hco, existing})

		res := NewServiceHandler(cl, commontestutils.GetScheme(), newService()).Ensure(req)
		Expect(res.Err).ToNot(HaveOccurred())
		Expect(res.Updated).To(BeTrue())

		found := &corev1.Service{}
		Expect(cl.Get(context.Background(), client.ObjectKeyFromObject(existing), found)).To(Succeed())
		Expect(found.Annotations).To(HaveKeyWithValue(servingCertAnnotation, "test-cert"))
	})
})
