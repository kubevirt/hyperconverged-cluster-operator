package virtimport_test

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/yaml"

	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/virtimport"
)

const (
	testCRD = `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: tests.example.kubevirt.io
spec:
  group: example.kubevirt.io
  names:
    kind: Test
    plural: tests
  scope: Namespaced
  versions:
  - name: v1
    served: true
    storage: true
    schema:
      openAPIV3Schema:
        type: object
`
	testDeployment = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test
  namespace: kubevirt
spec:
  selector:
    matchLabels:
      app: test
  template:
    metadata:
      labels:
        app: test
    spec:
      containers:
      - name: manager
        image: controller:latest
        env:
        - name: FETCHER_IMAGE
          value: fetcher:latest
        ports:
        - containerPort: 8081
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8081
        readinessProbe:
          httpGet:
            port: 8081
      volumes:
      - name: metrics-certs
        secret:
          secretName: test-metrics-cert
`
	sha = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	testServiceAccount = `apiVersion: v1
kind: ServiceAccount
metadata:
  name: test
  namespace: kubevirt
`
	testService = `apiVersion: v1
kind: Service
metadata:
  name: test-metrics
  namespace: kubevirt
  labels:
    app: test
spec:
  selector:
    app: test
  ports:
  - name: http-metrics
    port: 8443
`
	testServiceMonitor = `apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: test
  namespace: kubevirt
spec:
  selector:
    matchLabels:
      app: test
  endpoints:
  - port: http-metrics
    path: /metrics
    scheme: https
`
	testNetworkPolicy = `apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: test
  namespace: kubevirt
spec:
  podSelector:
    matchLabels:
      app: test
  policyTypes:
  - Ingress
  ingress:
  - ports:
    - port: 8443
`
	testSecret = `apiVersion: v1
kind: Secret
type: kubernetes.io/service-account-token
metadata:
  name: test
  namespace: kubevirt
  annotations:
    kubernetes.io/service-account.name: test
`
	metricsCertsVolume = `      volumes:
      - name: metrics-certs
        secret:
          secretName: test-metrics-cert
`
)

// valid returns a bundle of the CRD, docs and the metrics Service and ServiceMonitor.
func valid(docs ...string) []byte {
	return join(append(append([]string{testCRD}, docs...), testService, testServiceMonitor)...)
}

func join(docs ...string) []byte {
	var out strings.Builder
	for _, doc := range docs {
		out.WriteString("---\n" + doc)
	}
	return []byte(out.String())
}

var _ = Describe("virt-import bundle", func() {
	Context("embedded bundle", func() {
		It("should parse and satisfy the contract", func() {
			b, err := virtimport.GetBundle()
			Expect(err).ToNot(HaveOccurred())
			Expect(b.Deployment).ToNot(BeNil())
		})

		It("should return deep copies", func() {
			b1, err := virtimport.GetBundle()
			Expect(err).ToNot(HaveOccurred())
			b1.Deployment.Name = "modified"

			b2, err := virtimport.GetBundle()
			Expect(err).ToNot(HaveOccurred())
			Expect(b2.Deployment.Name).ToNot(Equal("modified"))
		})

		It("should normalize the Deployment", func() {
			b, err := virtimport.GetBundle()
			Expect(err).ToNot(HaveOccurred())
			for _, c := range b.Deployment.Spec.Template.Spec.Containers {
				Expect(c.TerminationMessagePath).ToNot(BeEmpty())
				Expect(c.TerminationMessagePolicy).ToNot(BeEmpty())
			}
		})
	})

	// The bundle is embedded at build time, so its contract with HCO is checked here instead of
	// at runtime. See the HCO contract in the virt-import AGENTS.md.
	Context("embedded bundle contract", func() {
		var b *virtimport.Bundle

		BeforeEach(func() {
			var err error
			b, err = virtimport.GetBundle()
			Expect(err).ToNot(HaveOccurred())
		})

		It("should have a ServiceMonitor selecting the Service, without TLS and auth", func() {
			selector, err := metav1.LabelSelectorAsSelector(&b.ServiceMonitor.Spec.Selector)
			Expect(err).ToNot(HaveOccurred())
			Expect(selector.Empty()).To(BeFalse())
			Expect(selector.Matches(labels.Set(b.MetricsService.Labels))).To(BeTrue())

			endpoint := b.ServiceMonitor.Spec.Endpoints[0]
			Expect(endpoint.TLSConfig).To(BeNil())
			Expect(endpoint.Authorization).To(BeNil())
			Expect(endpoint.BearerTokenFile).To(BeEmpty()) //nolint:staticcheck // the bundle must not use it
		})

		It("should have the import authorization policy, bound with Deny", func() {
			Expect(b.ValidatingAdmissionPolicies).To(HaveLen(1))
			policy := b.ValidatingAdmissionPolicies[0].Name
			Expect(b.ValidatingAdmissionPolicyBindings).To(ContainElement(SatisfyAll(
				HaveField("Spec.PolicyName", policy),
				HaveField("Spec.ValidationActions", ContainElement(admissionregistrationv1.Deny)),
			)))
		})

		It("should have one service account token Secret without data for the scrape token", func() {
			Expect(b.Secrets).To(HaveLen(1))
			for _, secret := range b.Secrets {
				Expect(secret.Type).To(Equal(corev1.SecretTypeServiceAccountToken), secret.Name)
				Expect(secret.Data).To(BeEmpty(), secret.Name)
				Expect(secret.StringData).To(BeEmpty(), secret.Name)
			}
		})

		It("should only have ingress NetworkPolicies selecting the controller pods", func() {
			podLabels := labels.Set(b.Deployment.Spec.Template.Labels)
			for _, np := range b.NetworkPolicies {
				Expect(np.Spec.PolicyTypes).To(Equal([]networkingv1.PolicyType{networkingv1.PolicyTypeIngress}), np.Name)
				Expect(np.Spec.Egress).To(BeEmpty(), np.Name)
				selector, err := metav1.LabelSelectorAsSelector(&np.Spec.PodSelector)
				Expect(err).ToNot(HaveOccurred())
				Expect(selector.Empty()).To(BeFalse(), np.Name)
				Expect(selector.Matches(podLabels)).To(BeTrue(), np.Name)
			}
		})
	})

	Context("Parse", func() {
		It("should sort objects by kind", func() {
			b, err := virtimport.Parse(valid(testServiceAccount, testDeployment, testSecret, testNetworkPolicy))
			Expect(err).ToNot(HaveOccurred())
			Expect(b.ServiceAccounts).To(HaveLen(1))
			Expect(b.Deployment.Name).To(Equal("test"))
			Expect(b.Secrets).To(HaveLen(1))
			Expect(b.NetworkPolicies).To(HaveLen(1))
			Expect(b.MetricsService.Name).To(Equal("test-metrics"))
			Expect(b.ServiceMonitor.Name).To(Equal("test"))
			Expect(b.MetricsCertSecretName()).To(Equal("test-metrics-cert"))
		})

		It("should ignore empty documents", func() {
			b, err := virtimport.Parse(valid("", testDeployment, "\n"))
			Expect(err).ToNot(HaveOccurred())
			Expect(b.Deployment).ToNot(BeNil())
		})

		It("should reject unknown kinds", func() {
			_, err := virtimport.Parse(join(testCRD, testDeployment, `apiVersion: v1
kind: ConfigMap
metadata:
  name: test
`))
			Expect(err).To(MatchError(ContainSubstring("unsupported kind ConfigMap")))
		})

		It("should reject a bundle without a Deployment", func() {
			_, err := virtimport.Parse(join(testCRD))
			Expect(err).To(MatchError(ContainSubstring("exactly one Deployment")))
		})

		It("should reject a bundle with two Deployments", func() {
			_, err := virtimport.Parse(join(testCRD, testDeployment, testDeployment))
			Expect(err).To(MatchError(ContainSubstring("exactly one Deployment")))
		})

		It("should reject a Deployment without the manager container", func() {
			_, err := virtimport.Parse(join(testCRD, `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test
spec:
  selector:
    matchLabels:
      app: test
  template:
    spec:
      containers:
      - name: other
        image: controller:latest
`))
			Expect(err).To(MatchError(ContainSubstring(`exactly one container named "manager"`)))
		})

		It("should reject a Deployment with a second container", func() {
			_, err := virtimport.Parse(join(testCRD, `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test
spec:
  selector:
    matchLabels:
      app: test
  template:
    spec:
      containers:
      - name: manager
        image: controller:latest
        env:
        - name: FETCHER_IMAGE
          value: fetcher:latest
      - name: sidecar
        image: sidecar:latest
`))
			Expect(err).To(MatchError(ContainSubstring(`exactly one container named "manager"`)))
		})

		It("should reject a manager container without FETCHER_IMAGE", func() {
			_, err := virtimport.Parse(join(testCRD, `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test
spec:
  selector:
    matchLabels:
      app: test
  template:
    spec:
      containers:
      - name: manager
        image: controller:latest
`))
			Expect(err).To(MatchError(ContainSubstring("FETCHER_IMAGE")))
		})

		DescribeTable("should reject a bundle without the metrics objects HCO needs",
			func(bundle []byte, msg string) {
				_, err := virtimport.Parse(bundle)
				Expect(err).To(MatchError(ContainSubstring(msg)))
			},
			Entry("no metrics-certs volume",
				join(testCRD, strings.Replace(testDeployment, metricsCertsVolume, "", 1), testService, testServiceMonitor),
				`"metrics-certs" volume`),
			Entry("no Service", join(testCRD, testDeployment, testServiceMonitor), "exactly one Service"),
			Entry("two Services", valid(testDeployment, testService), "exactly one Service"),
			Entry("no ServiceMonitor", join(testCRD, testDeployment, testService), "exactly one ServiceMonitor"),
			Entry("two ServiceMonitors", valid(testDeployment, testServiceMonitor), "exactly one ServiceMonitor"),
			Entry("ServiceMonitor without an endpoint",
				join(testCRD, testDeployment, testService, testServiceMonitor[:strings.Index(testServiceMonitor, "  endpoints:")]),
				"with one endpoint"),
			Entry("two Secrets", valid(testDeployment, testSecret, testSecret), "at most one Secret"),
		)

		It("should fail on invalid YAML", func() {
			_, err := virtimport.Parse([]byte("kind: [unclosed"))
			Expect(err).To(HaveOccurred())
		})
	})

	Context("normalization", func() {
		It("should fill in the defaults the apiserver sets", func() {
			b, err := virtimport.Parse(valid(testDeployment))
			Expect(err).ToNot(HaveOccurred())

			c := b.Deployment.Spec.Template.Spec.Containers[0]
			Expect(c.ImagePullPolicy).To(BeEmpty())
			Expect(c.TerminationMessagePath).To(Equal(corev1.TerminationMessagePathDefault))
			Expect(c.LivenessProbe.TimeoutSeconds).To(Equal(int32(1)))
			Expect(c.LivenessProbe.SuccessThreshold).To(Equal(int32(1)))
			Expect(c.LivenessProbe.FailureThreshold).To(Equal(int32(3)))
			Expect(c.LivenessProbe.HTTPGet.Scheme).To(Equal(corev1.URISchemeHTTP))
		})

		It("should default the Secret volume mode", func() {
			b, err := virtimport.Parse(valid(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: test
spec:
  selector:
    matchLabels:
      app: test
  template:
    spec:
      containers:
      - name: manager
        image: controller:latest
        env:
        - name: FETCHER_IMAGE
          value: fetcher:latest
      volumes:
      - name: secret
        secret:
          secretName: test
      - name: metrics-certs
        secret:
          secretName: test-metrics-cert
`))
			Expect(err).ToNot(HaveOccurred())

			volumes := b.Deployment.Spec.Template.Spec.Volumes
			Expect(volumes[0].Secret.DefaultMode).To(HaveValue(Equal(corev1.SecretVolumeSourceDefaultMode)))
		})

		It("should drop empty lists, as the apiserver does", func() {
			b, err := virtimport.Parse(valid(strings.Replace(testDeployment, metricsCertsVolume,
				"        volumeMounts: []\n"+metricsCertsVolume, 1)))
			Expect(err).ToNot(HaveOccurred())
			Expect(b.Deployment.Spec.Template.Spec.Containers[0].VolumeMounts).To(BeNil())
		})

		It("should keep values that are set", func() {
			b, err := virtimport.Parse(valid(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: test
spec:
  selector:
    matchLabels:
      app: test
  template:
    spec:
      containers:
      - name: manager
        image: controller:latest
        imagePullPolicy: Always
        terminationMessagePolicy: FallbackToLogsOnError
        env:
        - name: FETCHER_IMAGE
          value: fetcher:latest
        livenessProbe:
          periodSeconds: 20
          failureThreshold: 15
          httpGet:
            path: /healthz
            port: 8081
` + metricsCertsVolume))
			Expect(err).ToNot(HaveOccurred())

			c := b.Deployment.Spec.Template.Spec.Containers[0]
			Expect(c.ImagePullPolicy).To(Equal(corev1.PullAlways))
			Expect(c.TerminationMessagePolicy).To(Equal(corev1.TerminationMessageFallbackToLogsOnError))
			Expect(c.LivenessProbe.PeriodSeconds).To(Equal(int32(20)))
			Expect(c.LivenessProbe.FailureThreshold).To(Equal(int32(15)))
		})

		It("should be idempotent", func() {
			b, err := virtimport.Parse(valid(testDeployment))
			Expect(err).ToNot(HaveOccurred())
			b.Deployment.APIVersion = "apps/v1"
			b.Deployment.Kind = "Deployment"
			normalized, err := yaml.Marshal(b.Deployment)
			Expect(err).ToNot(HaveOccurred())

			again, err := virtimport.Parse(valid(string(normalized)))
			Expect(err).ToNot(HaveOccurred())
			Expect(again.Deployment).To(Equal(b.Deployment))
		})
	})

	DescribeTable("DefaultPullPolicy should default like the apiserver",
		func(image string, expected corev1.PullPolicy) {
			Expect(virtimport.DefaultPullPolicy(image)).To(Equal(expected))
		},
		Entry("digest", "quay.io/kubevirt/virt-import-controller@sha256:"+sha, corev1.PullIfNotPresent),
		Entry("version tag", "quay.io/kubevirt/virt-import-controller:v1.0.0", corev1.PullIfNotPresent),
		Entry("version tag and digest", "quay.io/kubevirt/virt-import-controller:v1.0.0@sha256:"+sha, corev1.PullIfNotPresent),
		Entry("latest tag", "quay.io/kubevirt/virt-import-controller:latest", corev1.PullAlways),
		Entry("latest tag and digest", "quay.io/kubevirt/virt-import-controller:latest@sha256:"+sha, corev1.PullAlways),
		Entry("no tag", "quay.io/kubevirt/virt-import-controller", corev1.PullAlways),
		Entry("short name", "virt-import-controller", corev1.PullAlways),
		Entry("invalid", "Invalid Image", corev1.PullIfNotPresent),
	)
})
