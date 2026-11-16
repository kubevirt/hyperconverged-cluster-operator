package virtimport_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
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
`
	sha = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	testServiceAccount = `apiVersion: v1
kind: ServiceAccount
metadata:
  name: test
  namespace: kubevirt
`
)

func join(docs ...string) []byte {
	var out string
	for _, doc := range docs {
		out += "---\n" + doc
	}
	return []byte(out)
}

var _ = Describe("virt-import bundle", func() {
	Context("embedded bundle", func() {
		It("should parse and satisfy the contract", func() {
			b, err := virtimport.GetBundle()
			Expect(err).ToNot(HaveOccurred())
			Expect(b.CRDs).ToNot(BeEmpty())
			Expect(b.Deployment).ToNot(BeNil())
		})

		It("should return deep copies", func() {
			b1, err := virtimport.GetBundle()
			Expect(err).ToNot(HaveOccurred())
			b1.Deployment.Name = "modified"
			b1.CRDs[0].Name = "modified"

			b2, err := virtimport.GetBundle()
			Expect(err).ToNot(HaveOccurred())
			Expect(b2.Deployment.Name).ToNot(Equal("modified"))
			Expect(b2.CRDs[0].Name).ToNot(Equal("modified"))
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

	Context("Parse", func() {
		It("should sort objects by kind", func() {
			b, err := virtimport.Parse(join(testCRD, testServiceAccount, testDeployment))
			Expect(err).ToNot(HaveOccurred())
			Expect(b.CRDs).To(HaveLen(1))
			Expect(b.CRDs[0].Name).To(Equal("tests.example.kubevirt.io"))
			Expect(b.ServiceAccounts).To(HaveLen(1))
			Expect(b.Deployment.Name).To(Equal("test"))
		})

		It("should ignore empty documents", func() {
			b, err := virtimport.Parse(join(testCRD, "", testDeployment, "\n"))
			Expect(err).ToNot(HaveOccurred())
			Expect(b.CRDs).To(HaveLen(1))
		})

		It("should reject unknown kinds", func() {
			_, err := virtimport.Parse(join(testCRD, testDeployment, `apiVersion: v1
kind: ConfigMap
metadata:
  name: test
`))
			Expect(err).To(MatchError(ContainSubstring("unsupported kind ConfigMap")))
		})

		It("should reject a bundle without a CRD", func() {
			_, err := virtimport.Parse(join(testDeployment))
			Expect(err).To(MatchError(ContainSubstring("at least one CustomResourceDefinition")))
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

		It("should fail on invalid YAML", func() {
			_, err := virtimport.Parse([]byte("kind: [unclosed"))
			Expect(err).To(HaveOccurred())
		})
	})

	Context("normalization", func() {
		It("should fill in the defaults the apiserver sets", func() {
			b, err := virtimport.Parse(join(testCRD, testDeployment))
			Expect(err).ToNot(HaveOccurred())

			c := b.Deployment.Spec.Template.Spec.Containers[0]
			Expect(c.ImagePullPolicy).To(BeEmpty())
			Expect(c.TerminationMessagePath).To(Equal(corev1.TerminationMessagePathDefault))
			Expect(c.TerminationMessagePolicy).To(Equal(corev1.TerminationMessageReadFile))
			Expect(c.Ports[0].Protocol).To(Equal(corev1.ProtocolTCP))
			Expect(c.LivenessProbe.TimeoutSeconds).To(Equal(int32(1)))
			Expect(c.LivenessProbe.PeriodSeconds).To(Equal(int32(10)))
			Expect(c.LivenessProbe.SuccessThreshold).To(Equal(int32(1)))
			Expect(c.LivenessProbe.FailureThreshold).To(Equal(int32(3)))
			Expect(c.LivenessProbe.HTTPGet.Scheme).To(Equal(corev1.URISchemeHTTP))
		})

		It("should default replicas, env field refs and volume modes", func() {
			b, err := virtimport.Parse(join(testCRD, `apiVersion: apps/v1
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
        - name: POD_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
      volumes:
      - name: secret
        secret:
          secretName: test
      - name: config
        configMap:
          name: test
      - name: projected
        projected:
          sources: []
      - name: downward
        downwardAPI:
          items: []
`))
			Expect(err).ToNot(HaveOccurred())

			spec := b.Deployment.Spec
			Expect(spec.Replicas).To(HaveValue(Equal(int32(1))))
			Expect(spec.Template.Spec.Containers[0].Env[1].ValueFrom.FieldRef.APIVersion).To(Equal("v1"))
			volumes := spec.Template.Spec.Volumes
			Expect(volumes[0].Secret.DefaultMode).To(HaveValue(Equal(corev1.SecretVolumeSourceDefaultMode)))
			Expect(volumes[1].ConfigMap.DefaultMode).To(HaveValue(Equal(corev1.ConfigMapVolumeSourceDefaultMode)))
			Expect(volumes[2].Projected.DefaultMode).To(HaveValue(Equal(corev1.ProjectedVolumeSourceDefaultMode)))
			Expect(volumes[3].DownwardAPI.DefaultMode).To(HaveValue(Equal(corev1.DownwardAPIVolumeSourceDefaultMode)))
		})

		It("should drop empty lists, as the apiserver does", func() {
			b, err := virtimport.Parse(join(testCRD, testDeployment+`        volumeMounts: []
      volumes: []
`))
			Expect(err).ToNot(HaveOccurred())
			Expect(b.Deployment.Spec.Template.Spec.Containers[0].VolumeMounts).To(BeNil())
			Expect(b.Deployment.Spec.Template.Spec.Volumes).To(BeNil())
		})

		It("should keep values that are set", func() {
			b, err := virtimport.Parse(join(testCRD, `apiVersion: apps/v1
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
`))
			Expect(err).ToNot(HaveOccurred())

			c := b.Deployment.Spec.Template.Spec.Containers[0]
			Expect(c.ImagePullPolicy).To(Equal(corev1.PullAlways))
			Expect(c.TerminationMessagePolicy).To(Equal(corev1.TerminationMessageFallbackToLogsOnError))
			Expect(c.LivenessProbe.PeriodSeconds).To(Equal(int32(20)))
			Expect(c.LivenessProbe.FailureThreshold).To(Equal(int32(15)))
		})

		It("should be idempotent", func() {
			b, err := virtimport.Parse(join(testCRD, testDeployment))
			Expect(err).ToNot(HaveOccurred())
			b.Deployment.APIVersion = "apps/v1"
			b.Deployment.Kind = "Deployment"
			normalized, err := yaml.Marshal(b.Deployment)
			Expect(err).ToNot(HaveOccurred())

			again, err := virtimport.Parse(join(testCRD, string(normalized)))
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
