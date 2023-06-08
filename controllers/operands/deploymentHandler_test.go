package operands

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
)

var _ = Describe("Deployment Handler", func() {
	Context("update or recreate the Deployment as required", func() {
		var hco *hcov1beta1.HyperConverged
		var req *common.HcoRequest
		var originalDeployment *appsv1.Deployment
		var modifiedDeployment *appsv1.Deployment

		BeforeEach(func() {
			hco = commontestutils.NewHco()
			req = commontestutils.NewReq(hco)
			originalDeployment = &appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Deployment",
					APIVersion: "apps/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:   "modifiedDeployment",
					Labels: map[string]string{"key1": "value1"},
					UID:    "testuid",
				},
				Spec: appsv1.DeploymentSpec{},
			}
			modifiedDeployment = &appsv1.Deployment{}
			originalDeployment.DeepCopyInto(modifiedDeployment)
		})

		It("should recreate the Deployment as LabelSelector has changed", func() {
			// create a fake client using original deployment
			cl := commontestutils.InitClient([]client.Object{originalDeployment})
			foundResource := &appsv1.Deployment{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Namespace: originalDeployment.GetNamespace(), Name: originalDeployment.GetName()},
					foundResource),
			).ToNot(HaveOccurred())

			// modify the LabelSelector
			modifiedDeployment.Spec.Selector = &metav1.LabelSelector{
				MatchLabels: map[string]string{"key2": "value2"},
			}
			modifiedDeployment.SetUID("")

			handler := newDeploymentHandler(cl, commontestutils.GetScheme(), modifiedDeployment)
			res := handler.ensure(req)
			Expect(res.Updated).To(BeTrue())
			Expect(res.Err).ToNot(HaveOccurred())

			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Namespace: modifiedDeployment.GetNamespace(), Name: modifiedDeployment.GetName()},
					foundResource),
			).ToNot(HaveOccurred())

			Expect(foundResource.Spec.Selector).Should(Equal(modifiedDeployment.Spec.Selector))
			Expect(foundResource.GetUID()).ToNot(Equal(originalDeployment.GetUID()))
		})

		It("should only update, not recreate, the Deployment since LabelSelector hasn't changed", func() {
			cl := commontestutils.InitClient([]client.Object{originalDeployment})
			foundResource := &appsv1.Deployment{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Namespace: originalDeployment.GetNamespace(), Name: originalDeployment.GetName()},
					foundResource),
			).ToNot(HaveOccurred())

			// modify only the labels
			gotLabels := originalDeployment.GetLabels()
			gotLabels["key2"] = "value2"
			modifiedDeployment.SetLabels(gotLabels)

			handler := newDeploymentHandler(cl, commontestutils.GetScheme(), modifiedDeployment)
			res := handler.ensure(req)
			Expect(res.Updated).To(BeTrue())
			Expect(res.Err).ToNot(HaveOccurred())

			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Namespace: modifiedDeployment.GetNamespace(), Name: modifiedDeployment.GetName()},
					foundResource),
			).ToNot(HaveOccurred())

			Expect(foundResource.Spec.Selector).To(BeNil())
			Expect(foundResource.Labels).Should(Equal(gotLabels))
			Expect(foundResource.GetUID()).To(Equal(originalDeployment.GetUID()))
		})
	})
})
