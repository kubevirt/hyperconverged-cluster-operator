package operands

import (
	"context"
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	consolev1 "github.com/openshift/api/console/v1"
	routev1 "github.com/openshift/api/route/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/reference"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

var _ = Describe("CLI Download", func() {
	Context("ConsoleCLIDownload", func() {

		var hco *hcov1beta1.HyperConverged
		var req *common.HcoRequest

		BeforeEach(func() {
			hco = commontestutils.NewHco()
			req = commontestutils.NewReq(hco)
		})

		It("should create if not present", func() {
			expectedResource := NewConsoleCLIDownload(hco)
			cl := commontestutils.InitClient([]client.Object{})
			handler := (*genericOperand)(newCliDownloadHandler(cl, commontestutils.GetScheme()))
			res := handler.ensure(req)
			Expect(res.Err).ToNot(HaveOccurred())

			key := client.ObjectKeyFromObject(expectedResource)
			foundResource := &consolev1.ConsoleCLIDownload{}
			Expect(cl.Get(context.TODO(), key, foundResource)).ToNot(HaveOccurred())
			Expect(foundResource.Name).To(Equal(expectedResource.Name))
			Expect(foundResource.Labels).Should(HaveKeyWithValue(hcoutil.AppLabel, commontestutils.Name))
			Expect(foundResource.Spec.Links).Should(HaveLen(6))
			Expect(foundResource.Namespace).To(Equal(expectedResource.Namespace))
		})

		It("should find if present", func() {
			expectedResource := NewConsoleCLIDownload(hco)
			cl := commontestutils.InitClient([]client.Object{hco, expectedResource})
			handler := (*genericOperand)(newCliDownloadHandler(cl, commontestutils.GetScheme()))
			res := handler.ensure(req)
			Expect(res.Err).ToNot(HaveOccurred())

			// Check HCO's status
			Expect(hco.Status.RelatedObjects).To(Not(BeNil()))
			objectRef, err := reference.GetReference(handler.Scheme, expectedResource)
			Expect(err).ToNot(HaveOccurred())
			// ObjectReference should have been added
			Expect(hco.Status.RelatedObjects).To(ContainElement(*objectRef))
		})

		DescribeTable("should update if something changed", func(modify func(resource *consolev1.ConsoleCLIDownload)) {
			expectedResource := NewConsoleCLIDownload(hco)
			modifiedResource := &consolev1.ConsoleCLIDownload{}
			expectedResource.DeepCopyInto(modifiedResource)
			modify(modifiedResource)

			cl := commontestutils.InitClient([]client.Object{modifiedResource})
			handler := (*genericOperand)(newCliDownloadHandler(cl, commontestutils.GetScheme()))
			res := handler.ensure(req)
			Expect(res.Err).ToNot(HaveOccurred())

			key := client.ObjectKeyFromObject(expectedResource)
			foundResource := &consolev1.ConsoleCLIDownload{}
			Expect(cl.Get(context.TODO(), key, foundResource)).To(Succeed())
			Expect(reflect.DeepEqual(expectedResource.Spec, foundResource.Spec)).To(BeTrue())

			// ObjectReference should have been updated
			Expect(hco.Status.RelatedObjects).To(Not(BeNil()))
			objectRefOutdated, err := reference.GetReference(handler.Scheme, modifiedResource)
			Expect(err).ToNot(HaveOccurred())
			objectRefFound, err := reference.GetReference(handler.Scheme, foundResource)
			Expect(err).ToNot(HaveOccurred())
			Expect(hco.Status.RelatedObjects).To(Not(ContainElement(*objectRefOutdated)))
			Expect(hco.Status.RelatedObjects).To(ContainElement(*objectRefFound))
		},
			Entry("with modified description", func(resource *consolev1.ConsoleCLIDownload) {
				resource.Spec.Description = "different text"
			}),
			Entry("with modified display name", func(resource *consolev1.ConsoleCLIDownload) {
				resource.Spec.DisplayName = "different text"
			}),
			Entry("with modified links", func(resource *consolev1.ConsoleCLIDownload) {
				resource.Spec.Links = []consolev1.CLIDownloadLink{{Text: "text", Href: "href"}}
			}),
			Entry("with modified labels", func(resource *consolev1.ConsoleCLIDownload) {
				resource.Labels = map[string]string{"key": "value"}
			}),
		)

	})
})

var _ = Describe("Downloads Service", func() {
	Context("Downloads Service", func() {

		var hco *hcov1beta1.HyperConverged
		var req *common.HcoRequest

		BeforeEach(func() {
			hco = commontestutils.NewHco()
			req = commontestutils.NewReq(hco)
		})

		It("should create if not present", func() {
			expectedResource := NewCliDownloadsService(hco)
			cl := commontestutils.InitClient([]client.Object{})
			handler := (*genericOperand)(newServiceHandler(cl, commontestutils.GetScheme(), NewCliDownloadsService))
			res := handler.ensure(req)
			Expect(res.Err).ToNot(HaveOccurred())

			key := client.ObjectKeyFromObject(expectedResource)
			foundResource := &corev1.Service{}
			Expect(cl.Get(context.TODO(), key, foundResource)).ToNot(HaveOccurred())
			Expect(foundResource.Name).To(Equal(expectedResource.Name))
			Expect(foundResource.Labels).Should(HaveKeyWithValue(hcoutil.AppLabel, commontestutils.Name))
			Expect(foundResource.Namespace).To(Equal(expectedResource.Namespace))
		})

		It("should find if present", func() {
			expectedResource := NewCliDownloadsService(hco)
			cl := commontestutils.InitClient([]client.Object{hco, expectedResource})
			handler := (*genericOperand)(newServiceHandler(cl, commontestutils.GetScheme(), NewCliDownloadsService))
			res := handler.ensure(req)
			Expect(res.Err).ToNot(HaveOccurred())

			// Check HCO's status
			Expect(hco.Status.RelatedObjects).To(Not(BeNil()))
			objectRef, err := reference.GetReference(handler.Scheme, expectedResource)
			Expect(err).ToNot(HaveOccurred())
			// ObjectReference should have been added
			Expect(hco.Status.RelatedObjects).To(ContainElement(*objectRef))
		})

		DescribeTable("should update if something changed", func(modify func(resource *corev1.Service)) {
			expectedResource := NewCliDownloadsService(hco)
			modifiedResource := &corev1.Service{}
			expectedResource.DeepCopyInto(modifiedResource)
			modify(modifiedResource)

			cl := commontestutils.InitClient([]client.Object{modifiedResource})

			handler := (*genericOperand)(newServiceHandler(cl, commontestutils.GetScheme(), NewCliDownloadsService))
			res := handler.ensure(req)
			Expect(res.Err).ToNot(HaveOccurred())

			key := client.ObjectKeyFromObject(expectedResource)
			foundResource := &corev1.Service{}
			Expect(cl.Get(context.TODO(), key, foundResource)).To(Succeed())
			Expect(hasServiceRightFields(foundResource, expectedResource)).To(BeTrue())

			// ObjectReference should have been updated
			Expect(hco.Status.RelatedObjects).To(Not(BeNil()))
			objectRefOutdated, err := reference.GetReference(handler.Scheme, modifiedResource)
			Expect(err).ToNot(HaveOccurred())
			objectRefFound, err := reference.GetReference(handler.Scheme, foundResource)
			Expect(err).ToNot(HaveOccurred())
			Expect(hco.Status.RelatedObjects).To(Not(ContainElement(*objectRefOutdated)))
			Expect(hco.Status.RelatedObjects).To(ContainElement(*objectRefFound))
		},
			Entry("with modified selector", func(resource *corev1.Service) {
				resource.Spec.Selector = map[string]string{"key": "value"}
			}),
			Entry("with modified labels", func(resource *corev1.Service) {
				resource.Labels = map[string]string{"key": "value"}
			}),
			Entry("with modified ports", func(resource *corev1.Service) {
				resource.Spec.Ports = []corev1.ServicePort{{Port: 1111, Protocol: corev1.ProtocolUDP}}
			}),
		)

	})
})

var _ = Describe("Cli Downloads Route", func() {
	Context("Cli Downloads Route", func() {

		var hco *hcov1beta1.HyperConverged
		var req *common.HcoRequest

		BeforeEach(func() {
			hco = commontestutils.NewHco()
			req = commontestutils.NewReq(hco)
		})

		It("should create if not present", func() {
			expectedResource := NewCliDownloadsRoute(hco)
			cl := commontestutils.InitClient([]client.Object{})
			handler := (*genericOperand)(newCliDownloadsRouteHandler(cl, commontestutils.GetScheme()))
			res := handler.ensure(req)
			Expect(res.Err).ToNot(HaveOccurred())

			key := client.ObjectKeyFromObject(expectedResource)
			foundResource := &routev1.Route{}
			Expect(cl.Get(context.TODO(), key, foundResource)).ToNot(HaveOccurred())
			Expect(foundResource.Name).To(Equal(expectedResource.Name))
			Expect(foundResource.Labels).Should(HaveKeyWithValue(hcoutil.AppLabel, commontestutils.Name))
			Expect(foundResource.Namespace).To(Equal(expectedResource.Namespace))
		})

		It("should find if present", func() {
			expectedResource := NewCliDownloadsRoute(hco)
			cl := commontestutils.InitClient([]client.Object{hco, expectedResource})
			handler := (*genericOperand)(newCliDownloadsRouteHandler(cl, commontestutils.GetScheme()))
			res := handler.ensure(req)
			Expect(res.Err).ToNot(HaveOccurred())

			// Check HCO's status
			Expect(hco.Status.RelatedObjects).To(Not(BeNil()))
			objectRef, err := reference.GetReference(handler.Scheme, expectedResource)
			Expect(err).ToNot(HaveOccurred())
			// ObjectReference should have been added
			Expect(hco.Status.RelatedObjects).To(ContainElement(*objectRef))
		})

		DescribeTable("should update if something changed", func(modify func(resource *routev1.Route)) {
			expectedResource := NewCliDownloadsRoute(hco)
			modifiedResource := &routev1.Route{}
			expectedResource.DeepCopyInto(modifiedResource)
			modify(modifiedResource)

			cl := commontestutils.InitClient([]client.Object{modifiedResource})
			handler := (*genericOperand)(newCliDownloadsRouteHandler(cl, commontestutils.GetScheme()))
			res := handler.ensure(req)
			Expect(res.Err).ToNot(HaveOccurred())

			key := client.ObjectKeyFromObject(expectedResource)
			foundResource := &routev1.Route{}
			Expect(cl.Get(context.TODO(), key, foundResource)).To(Succeed())
			Expect(hasRouteRightFields(foundResource, expectedResource)).To(BeTrue())

			// ObjectReference should have been updated
			Expect(hco.Status.RelatedObjects).To(Not(BeNil()))
			objectRefOutdated, err := reference.GetReference(handler.Scheme, modifiedResource)
			Expect(err).ToNot(HaveOccurred())
			objectRefFound, err := reference.GetReference(handler.Scheme, foundResource)
			Expect(err).ToNot(HaveOccurred())
			Expect(hco.Status.RelatedObjects).To(Not(ContainElement(*objectRefOutdated)))
			Expect(hco.Status.RelatedObjects).To(ContainElement(*objectRefFound))
		},
			Entry("with modified labels", func(resource *routev1.Route) {
				resource.Labels = map[string]string{"key": "value"}
			}),
			Entry("with modified port", func(resource *routev1.Route) {
				resource.Spec.Port = &routev1.RoutePort{
					TargetPort: intstr.IntOrString{IntVal: 1111},
				}
			}),
			Entry("with modified tls", func(resource *routev1.Route) {
				resource.Spec.TLS = &routev1.TLSConfig{
					Termination: routev1.TLSTerminationReencrypt,
				}
			}),
			Entry("with modified target reference", func(resource *routev1.Route) {
				resource.Spec.To = routev1.RouteTargetReference{
					Kind: "Service",
					Name: "test-service",
				}
			}),
		)

	})
})
