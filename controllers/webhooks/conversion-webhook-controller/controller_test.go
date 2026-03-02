package conversion_webhook_controller

import (
	"context"
	"os"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	fakeownresources "github.com/kubevirt/hyperconverged-cluster-operator/pkg/ownresources/fake"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/util/fake/clusterinfo"
)

const (
	testSecretName  = "hco-operator-service-cert"
	testServiceName = "hco-operator-service"
)

var _ = Describe("ConversionWebhookController", func() {

	BeforeEach(func() {
		origGetClusterInfo := hcoutil.GetClusterInfo
		hcoutil.GetClusterInfo = clusterinfo.NewGetClusterInfo(clusterinfo.WithIsOpenshift(true))

		DeferCleanup(func() {
			hcoutil.GetClusterInfo = origGetClusterInfo
		})
	})

	Describe("Reconcile", func() {

		var (
			request reconcile.Request
		)

		BeforeEach(func() {
			Expect(os.Setenv(hcoutil.OperatorNamespaceEnv, commontestutils.Namespace)).To(Succeed())
			fakeownresources.OLMV0OwnResourcesMock()

			request = reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name: hcoutil.HyperConvergedCRDName,
				},
			}

			DeferCleanup(func() {
				Expect(os.Unsetenv(hcoutil.OperatorNamespaceEnv)).To(Succeed())
				fakeownresources.ResetOwnResources()
			})
		})

		Context("CRD conversion configuration", func() {

			It("should update CRD when conversion is nil", func(ctx context.Context) {
				ctx = logr.NewContext(ctx, GinkgoLogr)
				crd := newHyperConvergedCRD()
				resources := []client.Object{crd}
				cl := commontestutils.InitClient(resources)

				reconciler := getReconciler(cl)

				result, err := reconciler.Reconcile(ctx, request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				updatedCRD := &apiextensionsv1.CustomResourceDefinition{}
				Expect(cl.Get(ctx, client.ObjectKey{Name: hcoutil.HyperConvergedCRDName}, updatedCRD)).To(Succeed())
				verifyConversionWebhookConfig(updatedCRD)

				Expect(updatedCRD.Annotations).To(HaveKeyWithValue(openshiftCertInjectionAnnotationName, "true"))
				Expect(updatedCRD.ResourceVersion).ToNot(Equal(crd.ResourceVersion))
			})

			It("should set k8s annotation when not running on openshift", func(ctx context.Context) {
				ctx = logr.NewContext(ctx, GinkgoLogr)
				crd := newHyperConvergedCRD()
				resources := []client.Object{crd}
				cl := commontestutils.InitClient(resources)

				hcoutil.GetClusterInfo = clusterinfo.NewGetClusterInfo(clusterinfo.WithIsOpenshift(false))

				reconciler := getReconciler(cl)

				result, err := reconciler.Reconcile(ctx, request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				updatedCRD := &apiextensionsv1.CustomResourceDefinition{}
				Expect(cl.Get(ctx, client.ObjectKey{Name: hcoutil.HyperConvergedCRDName}, updatedCRD)).To(Succeed())
				verifyConversionWebhookConfig(updatedCRD)

				Expect(updatedCRD.Annotations).To(HaveKeyWithValue(k8sCertInjectionAnnotationName, commontestutils.Namespace+"/"+testSecretName))
				Expect(updatedCRD.ResourceVersion).ToNot(Equal(crd.ResourceVersion))
			})

			It("should update CRD when strategy is not Webhook", func(ctx context.Context) {
				ctx = logr.NewContext(ctx, GinkgoLogr)
				crd := newHyperConvergedCRDWithConversion(&apiextensionsv1.CustomResourceConversion{
					Strategy: apiextensionsv1.NoneConverter,
				})

				resources := []client.Object{crd}
				cl := commontestutils.InitClient(resources)

				reconciler := getReconciler(cl)

				result, err := reconciler.Reconcile(ctx, request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				updatedCRD := &apiextensionsv1.CustomResourceDefinition{}
				Expect(cl.Get(ctx, client.ObjectKey{Name: hcoutil.HyperConvergedCRDName}, updatedCRD)).To(Succeed())
				verifyConversionWebhookConfig(updatedCRD)

				Expect(updatedCRD.Annotations).To(HaveKeyWithValue(openshiftCertInjectionAnnotationName, "true"))
			})

			It("should update CRD when webhook is nil", func(ctx context.Context) {
				ctx = logr.NewContext(ctx, GinkgoLogr)
				crd := newHyperConvergedCRDWithConversion(&apiextensionsv1.CustomResourceConversion{
					Strategy: apiextensionsv1.WebhookConverter,
					Webhook:  nil,
				})

				resources := []client.Object{crd}
				cl := commontestutils.InitClient(resources)

				reconciler := getReconciler(cl)

				result, err := reconciler.Reconcile(ctx, request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				updatedCRD := &apiextensionsv1.CustomResourceDefinition{}
				Expect(cl.Get(ctx, client.ObjectKey{Name: hcoutil.HyperConvergedCRDName}, updatedCRD)).To(Succeed())
				verifyConversionWebhookConfig(updatedCRD)

				Expect(updatedCRD.Annotations).To(HaveKeyWithValue(openshiftCertInjectionAnnotationName, "true"))
			})

			It("should update CRD when client config is nil", func(ctx context.Context) {
				ctx = logr.NewContext(ctx, GinkgoLogr)
				crd := newHyperConvergedCRDWithConversion(&apiextensionsv1.CustomResourceConversion{
					Strategy: apiextensionsv1.WebhookConverter,
					Webhook: &apiextensionsv1.WebhookConversion{
						ClientConfig: nil,
					},
				})
				resources := []client.Object{crd}
				cl := commontestutils.InitClient(resources)

				reconciler := getReconciler(cl)

				result, err := reconciler.Reconcile(ctx, request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				updatedCRD := &apiextensionsv1.CustomResourceDefinition{}
				Expect(cl.Get(ctx, client.ObjectKey{Name: hcoutil.HyperConvergedCRDName}, updatedCRD)).To(Succeed())
				verifyConversionWebhookConfig(updatedCRD)

				Expect(updatedCRD.Annotations).To(HaveKeyWithValue(openshiftCertInjectionAnnotationName, "true"))
			})

			It("should update CRD when service is nil", func(ctx context.Context) {
				ctx = logr.NewContext(ctx, GinkgoLogr)
				crd := newHyperConvergedCRDWithConversion(&apiextensionsv1.CustomResourceConversion{
					Strategy: apiextensionsv1.WebhookConverter,
					Webhook: &apiextensionsv1.WebhookConversion{
						ClientConfig: &apiextensionsv1.WebhookClientConfig{
							Service: nil,
						},
					},
				})

				resources := []client.Object{crd}
				cl := commontestutils.InitClient(resources)

				reconciler := getReconciler(cl)

				result, err := reconciler.Reconcile(ctx, request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				updatedCRD := &apiextensionsv1.CustomResourceDefinition{}
				Expect(cl.Get(ctx, client.ObjectKey{Name: hcoutil.HyperConvergedCRDName}, updatedCRD)).To(Succeed())
				verifyConversionWebhookConfig(updatedCRD)

				Expect(updatedCRD.Annotations).To(HaveKeyWithValue(openshiftCertInjectionAnnotationName, "true"))
			})

			It("should update CRD when service name is different", func(ctx context.Context) {
				ctx = logr.NewContext(ctx, GinkgoLogr)
				crd := newHyperConvergedCRDWithConversion(newValidConversion("wrong-service", commontestutils.Namespace))
				resources := []client.Object{crd}
				cl := commontestutils.InitClient(resources)

				reconciler := getReconciler(cl)

				result, err := reconciler.Reconcile(ctx, request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				updatedCRD := &apiextensionsv1.CustomResourceDefinition{}
				Expect(cl.Get(ctx, client.ObjectKey{Name: hcoutil.HyperConvergedCRDName}, updatedCRD)).To(Succeed())
				verifyConversionWebhookConfig(updatedCRD)

				Expect(updatedCRD.Annotations).To(HaveKeyWithValue(openshiftCertInjectionAnnotationName, "true"))
			})

			It("should update CRD when service namespace is different", func(ctx context.Context) {
				ctx = logr.NewContext(ctx, GinkgoLogr)
				crd := newHyperConvergedCRDWithConversion(newValidConversion(testServiceName, "wrong-namespace"))

				resources := []client.Object{crd}
				cl := commontestutils.InitClient(resources)

				reconciler := getReconciler(cl)

				result, err := reconciler.Reconcile(ctx, request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				updatedCRD := &apiextensionsv1.CustomResourceDefinition{}
				Expect(cl.Get(ctx, client.ObjectKey{Name: hcoutil.HyperConvergedCRDName}, updatedCRD)).To(Succeed())
				verifyConversionWebhookConfig(updatedCRD)

				Expect(updatedCRD.Annotations).To(HaveKeyWithValue(openshiftCertInjectionAnnotationName, "true"))
			})

			It("should update CRD when path is different", func(ctx context.Context) {
				ctx = logr.NewContext(ctx, GinkgoLogr)
				conversion := newValidConversion(testServiceName, commontestutils.Namespace)
				conversion.Webhook.ClientConfig.Service.Path = ptr.To("/wrong-path")
				crd := newHyperConvergedCRDWithConversion(conversion)

				resources := []client.Object{crd}
				cl := commontestutils.InitClient(resources)

				reconciler := getReconciler(cl)

				result, err := reconciler.Reconcile(ctx, request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				updatedCRD := &apiextensionsv1.CustomResourceDefinition{}
				Expect(cl.Get(ctx, client.ObjectKey{Name: hcoutil.HyperConvergedCRDName}, updatedCRD)).To(Succeed())
				verifyConversionWebhookConfig(updatedCRD)

				Expect(updatedCRD.Annotations).To(HaveKeyWithValue(openshiftCertInjectionAnnotationName, "true"))
			})

			It("should update CRD when port is different", func(ctx context.Context) {
				ctx = logr.NewContext(ctx, GinkgoLogr)
				conversion := newValidConversion(testServiceName, commontestutils.Namespace)
				conversion.Webhook.ClientConfig.Service.Port = ptr.To(int32(9999))
				crd := newHyperConvergedCRDWithConversion(conversion)

				resources := []client.Object{crd}
				cl := commontestutils.InitClient(resources)

				reconciler := getReconciler(cl)

				result, err := reconciler.Reconcile(ctx, request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				updatedCRD := &apiextensionsv1.CustomResourceDefinition{}
				Expect(cl.Get(ctx, client.ObjectKey{Name: hcoutil.HyperConvergedCRDName}, updatedCRD)).To(Succeed())
				verifyConversionWebhookConfig(updatedCRD)

				Expect(updatedCRD.Annotations).To(HaveKeyWithValue(openshiftCertInjectionAnnotationName, "true"))
			})

			It("should update CRD when caBundle was changed", func(ctx context.Context) {
				ctx = logr.NewContext(ctx, GinkgoLogr)

				hcoutil.GetClusterInfo = clusterinfo.NewGetClusterInfo(clusterinfo.WithIsManagedByOLM(true))

				conversion := newValidConversion(testServiceName, commontestutils.Namespace)
				conversion.Webhook.ClientConfig.CABundle = []byte("old-secret")
				crd := newHyperConvergedCRDWithConversion(conversion)
				crd.Annotations = nil

				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testSecretName,
						Namespace: commontestutils.Namespace,
					},
					Data: map[string][]byte{
						olmCaBundleKey: []byte("my-secret-content"),
					},
				}

				resources := []client.Object{crd, secret}
				cl := commontestutils.InitClient(resources)

				reconciler := getReconciler(cl)

				result, err := reconciler.Reconcile(ctx, request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				updatedCRD := &apiextensionsv1.CustomResourceDefinition{}
				Expect(cl.Get(ctx, client.ObjectKey{Name: hcoutil.HyperConvergedCRDName}, updatedCRD)).To(Succeed())
				verifyConversionWebhookConfig(updatedCRD)

				Expect(updatedCRD.Annotations).To(BeNil())
				Expect(updatedCRD.Spec.Conversion.Webhook.ClientConfig.CABundle).To(Equal([]byte("my-secret-content")))
			})

			It("should not update CRD when configuration is already correct", func(ctx context.Context) {
				ctx = logr.NewContext(ctx, GinkgoLogr)
				conversion := newValidConversion(testServiceName, commontestutils.Namespace)
				crd := newHyperConvergedCRDWithConversion(conversion)
				crd.Annotations = map[string]string{
					openshiftCertInjectionAnnotationName: "true",
				}
				resources := []client.Object{crd}
				cl := commontestutils.InitClient(resources)

				reconciler := getReconciler(cl)

				result, err := reconciler.Reconcile(ctx, request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				// Verify CRD was not updated (resource version should remain the same)
				updatedCRD := &apiextensionsv1.CustomResourceDefinition{}
				Expect(cl.Get(ctx, client.ObjectKey{Name: hcoutil.HyperConvergedCRDName}, updatedCRD)).To(Succeed())
				Expect(updatedCRD.ResourceVersion).To(Equal(crd.ResourceVersion))
			})

			It("Managed by OLM: should not update CRD when configuration is already correct", func(ctx context.Context) {
				ctx = logr.NewContext(ctx, GinkgoLogr)

				hcoutil.GetClusterInfo = clusterinfo.NewGetClusterInfo(clusterinfo.WithIsManagedByOLM(true))

				conversion := newValidConversion(testServiceName, commontestutils.Namespace)
				crd := newHyperConvergedCRDWithConversion(conversion)
				conversion.Webhook.ClientConfig.CABundle = []byte("my-secret-content")
				crd.Annotations = nil

				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testSecretName,
						Namespace: commontestutils.Namespace,
					},
					Data: map[string][]byte{
						olmCaBundleKey: []byte("my-secret-content"),
					},
				}

				resources := []client.Object{crd, secret}
				cl := commontestutils.InitClient(resources)

				reconciler := getReconciler(cl)

				result, err := reconciler.Reconcile(ctx, request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				// Verify CRD was not updated (resource version should remain the same)
				updatedCRD := &apiextensionsv1.CustomResourceDefinition{}
				Expect(cl.Get(ctx, client.ObjectKey{Name: hcoutil.HyperConvergedCRDName}, updatedCRD)).To(Succeed())
				Expect(updatedCRD.ResourceVersion).To(Equal(crd.ResourceVersion))
			})
		})

	})

	Describe("needsUpdate", func() {

		var reconciler *ReconcileConversionWebhook

		BeforeEach(func() {
			reconciler = &ReconcileConversionWebhook{
				namespace:          commontestutils.Namespace,
				serviceName:        testServiceName,
				certAnnotationName: openshiftCertInjectionAnnotationName,
				certAnnotationVal:  "true",
			}
		})

		It("should return true when conversion is nil", func() {
			crd := newHyperConvergedCRD()
			Expect(reconciler.needsUpdate(crd, nil)).To(BeTrue())
		})

		It("should return true when strategy is not Webhook", func() {
			crd := newHyperConvergedCRDWithConversion(&apiextensionsv1.CustomResourceConversion{
				Strategy: apiextensionsv1.NoneConverter,
			})
			Expect(reconciler.needsUpdate(crd, nil)).To(BeTrue())
		})

		It("should return true when webhook is nil", func() {
			crd := newHyperConvergedCRDWithConversion(&apiextensionsv1.CustomResourceConversion{
				Strategy: apiextensionsv1.WebhookConverter,
				Webhook:  nil,
			})
			Expect(reconciler.needsUpdate(crd, nil)).To(BeTrue())
		})

		It("should return true when client config is nil", func() {
			crd := newHyperConvergedCRDWithConversion(&apiextensionsv1.CustomResourceConversion{
				Strategy: apiextensionsv1.WebhookConverter,
				Webhook: &apiextensionsv1.WebhookConversion{
					ClientConfig: nil,
				},
			})
			Expect(reconciler.needsUpdate(crd, nil)).To(BeTrue())
		})

		It("should return true when service is nil", func() {
			crd := newHyperConvergedCRDWithConversion(&apiextensionsv1.CustomResourceConversion{
				Strategy: apiextensionsv1.WebhookConverter,
				Webhook: &apiextensionsv1.WebhookConversion{
					ClientConfig: &apiextensionsv1.WebhookClientConfig{
						Service: nil,
					},
				},
			})
			Expect(reconciler.needsUpdate(crd, nil)).To(BeTrue())
		})

		It("should return true when service name is different", func() {
			crd := newHyperConvergedCRDWithConversion(newValidConversion("wrong-name", commontestutils.Namespace))
			Expect(reconciler.needsUpdate(crd, nil)).To(BeTrue())
		})

		It("should return true when service namespace is different", func() {
			crd := newHyperConvergedCRDWithConversion(newValidConversion(testServiceName, "wrong-namespace"))
			Expect(reconciler.needsUpdate(crd, nil)).To(BeTrue())
		})

		It("should return true when path is different", func() {
			conversion := newValidConversion(testServiceName, commontestutils.Namespace)
			conversion.Webhook.ClientConfig.Service.Path = ptr.To("/wrong-path")
			crd := newHyperConvergedCRDWithConversion(conversion)
			Expect(reconciler.needsUpdate(crd, nil)).To(BeTrue())
		})

		It("should return true when port is different", func() {
			conversion := newValidConversion(testServiceName, commontestutils.Namespace)
			conversion.Webhook.ClientConfig.Service.Port = ptr.To(int32(9999))
			crd := newHyperConvergedCRDWithConversion(conversion)
			Expect(reconciler.needsUpdate(crd, nil)).To(BeTrue())
		})

		It("should return true when annotation value is missing", func() {
			crd := newHyperConvergedCRDWithConversion(newValidConversion(testServiceName, commontestutils.Namespace))
			delete(crd.Annotations, openshiftCertInjectionAnnotationName)
			Expect(reconciler.needsUpdate(crd, nil)).To(BeTrue())
		})

		It("should return true when annotation value is wrong", func() {
			crd := newHyperConvergedCRDWithConversion(newValidConversion(testServiceName, commontestutils.Namespace))
			crd.Annotations[openshiftCertInjectionAnnotationName] = "false"
			Expect(reconciler.needsUpdate(crd, nil)).To(BeTrue())
		})

		It("should return false when everything matches", func() {
			crd := newHyperConvergedCRDWithConversion(newValidConversion(testServiceName, commontestutils.Namespace))
			Expect(reconciler.needsUpdate(crd, nil)).To(BeFalse())
		})

		Context("Managed by OLM", func() {
			const secretContent = "a secret"

			var (
				crd         *apiextensionsv1.CustomResourceDefinition
				secretBytes []byte
			)

			BeforeEach(func() {
				crd = newHyperConvergedCRDWithConversion(newValidConversion(testServiceName, commontestutils.Namespace))
				delete(crd.Annotations, openshiftCertInjectionAnnotationName)

				secretBytes = []byte(secretContent)
				crd.Spec.Conversion.Webhook.ClientConfig.CABundle = secretBytes

				reconciler.managedByOLM = true
			})

			It("Managed by OLM: should return true caBundle was not set", func() {
				crd.Spec.Conversion.Webhook.ClientConfig.CABundle = nil
				Expect(reconciler.needsUpdate(crd, secretBytes)).To(BeTrue())
			})

			It("Managed by OLM: should return true when caBundle was changed", func() {
				Expect(reconciler.needsUpdate(crd, []byte("new secret"))).To(BeTrue())
			})

			It("should return false when everything matches", func() {
				Expect(reconciler.needsUpdate(crd, secretBytes)).To(BeFalse())
			})
		})
	})
})

// Helper functions

func newHyperConvergedCRD() *apiextensionsv1.CustomResourceDefinition {
	return &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: hcoutil.HyperConvergedCRDName,
		},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: "hco.kubevirt.io",
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Plural:   "hyperconvergeds",
				Singular: "hyperconverged",
				Kind:     "HyperConverged",
			},
			Scope: apiextensionsv1.ClusterScoped,
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
				{Name: "v1", Served: true, Storage: true},
				{Name: "v1beta1", Served: true, Storage: false},
			},
		},
	}
}

func newHyperConvergedCRDWithConversion(conversion *apiextensionsv1.CustomResourceConversion) *apiextensionsv1.CustomResourceDefinition {
	crd := newHyperConvergedCRD()
	crd.Spec.Conversion = conversion
	crd.Annotations = map[string]string{
		openshiftCertInjectionAnnotationName: "true",
	}
	return crd
}

func newValidConversion(serviceName, namespace string) *apiextensionsv1.CustomResourceConversion {
	return &apiextensionsv1.CustomResourceConversion{
		Strategy: apiextensionsv1.WebhookConverter,
		Webhook: &apiextensionsv1.WebhookConversion{
			ClientConfig: &apiextensionsv1.WebhookClientConfig{
				Service: &apiextensionsv1.ServiceReference{
					Namespace: namespace,
					Name:      serviceName,
					Path:      ptr.To(hcoutil.HCOConversionWebhookPath),
					Port:      ptr.To(int32(hcoutil.WebhookPort)),
				},
			},
			ConversionReviewVersions: []string{hcov1.APIVersionV1, hcov1beta1.APIVersionBeta},
		},
	}
}

func verifyConversionWebhookConfig(crd *apiextensionsv1.CustomResourceDefinition) {
	GinkgoHelper()

	Expect(crd.Spec.Conversion).ToNot(BeNil())
	Expect(crd.Spec.Conversion.Strategy).To(Equal(apiextensionsv1.WebhookConverter))
	Expect(crd.Spec.Conversion.Webhook).ToNot(BeNil())
	Expect(crd.Spec.Conversion.Webhook.ClientConfig).ToNot(BeNil())
	Expect(crd.Spec.Conversion.Webhook.ClientConfig.Service).ToNot(BeNil())
	Expect(crd.Spec.Conversion.Webhook.ClientConfig.Service.Namespace).To(Equal(commontestutils.Namespace))
	Expect(crd.Spec.Conversion.Webhook.ClientConfig.Service.Name).To(Equal(testServiceName))
	Expect(crd.Spec.Conversion.Webhook.ClientConfig.Service.Path).To(HaveValue(Equal(hcoutil.HCOConversionWebhookPath)))
	Expect(crd.Spec.Conversion.Webhook.ClientConfig.Service.Port).To(HaveValue(Equal(int32(hcoutil.WebhookPort))))
	Expect(crd.Spec.Conversion.Webhook.ConversionReviewVersions).To(ContainElements(hcov1.APIVersionV1, hcov1beta1.APIVersionBeta))
}

func getReconciler(cl client.Client) reconcile.Reconciler {
	mgr, err := commontestutils.NewManagerMock(&rest.Config{}, manager.Options{Scheme: commontestutils.GetScheme()}, cl, GinkgoLogr)
	Expect(err).ToNot(HaveOccurred())

	return newReconciler(mgr, commontestutils.Namespace, testSecretName, testServiceName)
}
