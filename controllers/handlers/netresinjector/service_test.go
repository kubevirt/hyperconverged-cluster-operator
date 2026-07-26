package netresinjector

import (
	"context"
	"maps"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/util/fake/clusterinfo"
)

var _ = Describe("Network Resources Injector Service", func() {
	var (
		hco *hcov1.HyperConverged
		req *common.HcoRequest
		cl  client.Client
	)

	BeforeEach(func() {
		hco = commontestutils.NewHco()
		req = commontestutils.NewReq(hco)
	})

	Context("newService", func() {
		It("should have all default values", func() {
			svc := newService()
			Expect(svc.Name).To(Equal(serviceName))
			Expect(svc.Namespace).To(Equal(hco.Namespace))
			Expect(svc.Labels).To(HaveKeyWithValue(hcoutil.AppLabel, hcoutil.HyperConvergedName))
			Expect(svc.Labels).To(HaveKeyWithValue(hcoutil.AppLabelComponent, string(hcoutil.AppComponentNetResInjector)))

			Expect(svc.Spec.Ports).To(HaveLen(1))
			Expect(svc.Spec.Ports[0].Port).To(Equal(int32(443)))
			Expect(svc.Spec.Ports[0].TargetPort.IntValue()).To(Equal(6443))
			Expect(svc.Spec.Ports[0].Protocol).To(Equal(corev1.ProtocolTCP))

			Expect(svc.Spec.Selector).To(HaveKeyWithValue(hcoutil.AppLabel, hcoutil.HyperConvergedName))
			Expect(svc.Spec.Selector).To(HaveKeyWithValue(hcoutil.AppLabelComponent, string(hcoutil.AppComponentNetResInjector)))
		})

		It("should have OpenShift annotation on OpenShift clusters", func() {
			origGetClusterInfo := hcoutil.GetClusterInfo
			hcoutil.GetClusterInfo = clusterinfo.NewGetClusterInfo(clusterinfo.WithIsOpenshift(true))

			DeferCleanup(func() {
				hcoutil.GetClusterInfo = origGetClusterInfo
			})

			svc := newService()

			Expect(svc.Annotations).To(HaveKeyWithValue("service.beta.openshift.io/serving-cert-secret-name", tlsSecretName))
		})

		It("should not have OpenShift annotation on plain Kubernetes clusters", func() {
			origGetClusterInfo := hcoutil.GetClusterInfo
			hcoutil.GetClusterInfo = clusterinfo.NewGetClusterInfo(clusterinfo.WithIsOpenshift(false))

			DeferCleanup(func() {
				hcoutil.GetClusterInfo = origGetClusterInfo
			})

			svc := newService()

			Expect(svc.Annotations).To(BeEmpty())
		})
	})

	Context("Service handler", func() {
		It("should create Service if it does not exist", func() {
			cl = commontestutils.InitClient([]client.Object{hco})

			handler := NewServiceHandler(cl, commontestutils.GetScheme())
			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Created).To(BeTrue())

			foundSvcs := &corev1.ServiceList{}
			Expect(cl.List(context.Background(), foundSvcs)).To(Succeed())
			Expect(foundSvcs.Items).To(HaveLen(1))
			Expect(foundSvcs.Items[0].Name).To(Equal(serviceName))
		})
	})

	Context("Service update", func() {
		It("should reconcile labels if they are missing while preserving user labels", func() {
			svc := newService()
			expectedLabels := maps.Clone(svc.Labels)
			delete(svc.Labels, hcoutil.AppLabelComponent)
			svc.Labels["user-added-label"] = "user-value"
			cl = commontestutils.InitClient([]client.Object{hco, svc})

			handler := NewServiceHandler(cl, commontestutils.GetScheme())
			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Updated).To(BeTrue())

			foundSvc := &corev1.Service{}
			Expect(cl.Get(context.Background(), client.ObjectKey{Name: serviceName, Namespace: hco.Namespace}, foundSvc)).To(Succeed())

			for key, value := range expectedLabels {
				Expect(foundSvc.Labels).To(HaveKeyWithValue(key, value))
			}
			Expect(foundSvc.Labels).To(HaveKeyWithValue("user-added-label", "user-value"))
		})
	})
})
