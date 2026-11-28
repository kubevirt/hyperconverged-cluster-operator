package virtimport

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/util/fake/clusterinfo"
)

const clientCAFlag = "--metrics-client-ca-configmap=kube-system/extension-apiserver-authentication"

func setClusterInfo(opts ...clusterinfo.MockOption) {
	orig := hcoutil.GetClusterInfo
	hcoutil.GetClusterInfo = clusterinfo.NewGetClusterInfo(opts...)
	DeferCleanup(func() { hcoutil.GetClusterInfo = orig })
}

// deployedObjects returns the objects of all handlers, as they would be deployed.
func deployedObjects() []client.Object {
	var objs []client.Object
	for _, op := range newOperands(nil) {
		obj, err := op.(operands.CRGetter).GetFullCr(commontestutils.NewHco())
		Expect(err).ToNot(HaveOccurred())
		objs = append(objs, obj)
	}
	return objs
}

func findObject[T client.Object](objs []client.Object) T {
	var found T
	for _, obj := range objs {
		if o, ok := obj.(T); ok {
			found = o
		}
	}
	return found
}

var _ = Describe("virt-import metrics", func() {
	var serverName string

	BeforeEach(func() {
		setupImageEnv()
		serverName = getBundle().MetricsService.Name + "." + hcoutil.GetOperatorNamespaceFromEnv() + ".svc"
	})

	Context("on Kubernetes", func() {
		It("should issue the serving certificate with cert-manager", func() {
			objs := deployedObjects()

			svc := findObject[*corev1.Service](objs)
			Expect(svc).ToNot(BeNil())
			Expect(svc.Annotations).ToNot(HaveKey(servingCertSecretAnnotation))

			cert := findObject[*unstructured.Unstructured](objs)
			Expect(cert).ToNot(BeNil())
			Expect(cert.GetNamespace()).To(Equal(hcoutil.GetOperatorNamespaceFromEnv()))
			Expect(cert.GetLabels()).To(HaveKeyWithValue(hcoutil.AppLabelComponent, string(appComponent)))
			secretName, _, _ := unstructured.NestedString(cert.Object, "spec", "secretName")
			Expect(secretName).To(Equal(getBundle().MetricsCertSecretName()))
			dnsNames, _, _ := unstructured.NestedStringSlice(cert.Object, "spec", "dnsNames")
			Expect(dnsNames).To(ConsistOf(serverName))
		})

		It("should deploy the token Secret", func() {
			Expect(findObject[*corev1.Secret](deployedObjects())).ToNot(BeNil())
		})

		It("should not deploy the ServiceMonitor without monitoring", func() {
			Expect(findObject[*monitoringv1.ServiceMonitor](deployedObjects())).To(BeNil())
		})

		It("should scrape with the token and verify the certificate", func() {
			setClusterInfo(clusterinfo.WithMonitoringAvailable(true))

			sm := findObject[*monitoringv1.ServiceMonitor](deployedObjects())
			Expect(sm).ToNot(BeNil())
			Expect(sm.Spec.ScrapeClassName).To(BeNil())
			Expect(sm.Spec.NamespaceSelector.MatchNames).To(ConsistOf(hcoutil.GetOperatorNamespaceFromEnv()))

			endpoint := sm.Spec.Endpoints[0]
			Expect(endpoint.Authorization).ToNot(BeNil())
			Expect(endpoint.Authorization.Credentials.Name).To(Equal(tokenSecretName(getBundle())))
			Expect(endpoint.Authorization.Credentials.Name).ToNot(BeEmpty())
			Expect(endpoint.Authorization.Credentials.Key).To(Equal(tokenSecretKey))
			Expect(endpoint.TLSConfig.CA.Secret.Name).To(Equal(getBundle().MetricsCertSecretName()))
			Expect(endpoint.TLSConfig.CA.Secret.Key).To(Equal(caCertKey))
			Expect(endpoint.TLSConfig.ServerName).To(HaveValue(Equal(serverName)))
			Expect(endpoint.TLSConfig.InsecureSkipVerify).To(HaveValue(BeFalse()))
		})

		It("should not pass the client certificate flags", func() {
			Expect(newDeployment(getBundle().Deployment, commontestutils.NewHco()).Spec.Template.Spec.Containers[0].Args).
				ToNot(ContainElement(clientCAFlag))
		})
	})

	Context("on OpenShift", func() {
		BeforeEach(func() {
			setClusterInfo(clusterinfo.WithIsOpenshift(true), clusterinfo.WithMonitoringAvailable(true))
		})

		It("should issue the serving certificate with service-ca", func() {
			objs := deployedObjects()

			svc := findObject[*corev1.Service](objs)
			Expect(svc.Annotations).To(HaveKeyWithValue(servingCertSecretAnnotation, getBundle().MetricsCertSecretName()))
			Expect(findObject[*unstructured.Unstructured](objs)).To(BeNil())
		})

		It("should not deploy the token Secret", func() {
			Expect(findObject[*corev1.Secret](deployedObjects())).To(BeNil())
		})

		It("should scrape with the Prometheus client certificate", func() {
			sm := findObject[*monitoringv1.ServiceMonitor](deployedObjects())
			Expect(sm).ToNot(BeNil())
			Expect(sm.Spec.ScrapeClassName).To(HaveValue(Equal(hcoutil.OpenShiftScrapeClassName)))
			Expect(sm.Spec.NamespaceSelector.MatchNames).To(ConsistOf(hcoutil.GetOperatorNamespaceFromEnv()))

			endpoint := sm.Spec.Endpoints[0]
			Expect(endpoint.Authorization).To(BeNil())
			Expect(endpoint.TLSConfig.ServerName).To(HaveValue(Equal(serverName)))
			Expect(endpoint.TLSConfig.InsecureSkipVerify).To(HaveValue(BeFalse()))
		})

		It("should pass the client certificate flags", func() {
			Expect(newDeployment(getBundle().Deployment, commontestutils.NewHco()).Spec.Template.Spec.Containers[0].Args).
				To(ContainElements(clientCAFlag, "--metrics-allowed-client-cn="+hcoutil.PrometheusK8sCN))
		})
	})

	It("should keep the ServiceMonitor selecting the deployed Service", func() {
		setClusterInfo(clusterinfo.WithMonitoringAvailable(true))
		objs := deployedObjects()

		svc := findObject[*corev1.Service](objs)
		sm := findObject[*monitoringv1.ServiceMonitor](objs)
		Expect(labels.SelectorFromSet(sm.Spec.Selector.MatchLabels).Matches(labels.Set(svc.Labels))).To(BeTrue())
		Expect(svc.Namespace).To(Equal(hcoutil.GetOperatorNamespaceFromEnv()))
	})

	It("should deploy the bundle network policies selecting the deployed pods", func() {
		hco := commontestutils.NewHco()
		podLabels := labels.Set(newDeployment(getBundle().Deployment, hco).Spec.Template.Labels)

		var bundlePolicies int
		for _, obj := range deployedObjects() {
			np, ok := obj.(*networkingv1.NetworkPolicy)
			if !ok || np.Name == newNetworkPolicy(getBundle().Deployment).Name {
				continue
			}
			bundlePolicies++
			Expect(np.Namespace).To(Equal(hcoutil.GetOperatorNamespaceFromEnv()))
			Expect(np.Labels).To(HaveKeyWithValue(hcoutil.AppLabelComponent, string(appComponent)))
			Expect(labels.SelectorFromSet(np.Spec.PodSelector.MatchLabels).Matches(podLabels)).To(BeTrue())
		}
		Expect(bundlePolicies).To(Equal(len(getBundle().NetworkPolicies)))
	})
})
