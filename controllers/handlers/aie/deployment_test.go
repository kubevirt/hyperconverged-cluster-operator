package aie

import (
	"context"
	"maps"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	openshiftconfigv1 "github.com/openshift/api/config/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/tlssecprofile"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

var _ = Describe("AIE Webhook Deployment", func() {
	const testImage = "quay.io/kubevirt/aieimage:test"

	var (
		hco *hcov1beta1.HyperConverged
		req *common.HcoRequest
		cl  client.Client
	)

	BeforeEach(func() {
		hco = commontestutils.NewHco()
		hco.Annotations = make(map[string]string)
		req = commontestutils.NewReq(hco)
		Expect(os.Setenv(hcoutil.AIEWebhookImageEnvV, testImage)).To(Succeed())

		DeferCleanup(func() {
			Expect(os.Unsetenv(hcoutil.AIEWebhookImageEnvV)).To(Succeed())
		})
	})

	Context("newAIEWebhookDeployment", func() {
		It("should have all default values", func() {
			origFunc := tlssecprofile.GetCipherSuitesAndMinTLSVersion
			tlssecprofile.GetCipherSuitesAndMinTLSVersion = func(fromHC *openshiftconfigv1.TLSSecurityProfile) ([]string, openshiftconfigv1.TLSProtocolVersion) {
				return []string{"TLS_AES_128_GCM_SHA256", "TLS_AES_256_GCM_SHA384"}, openshiftconfigv1.VersionTLS11
			}

			DeferCleanup(func() {
				tlssecprofile.GetCipherSuitesAndMinTLSVersion = origFunc
			})

			deployment := newAIEWebhookDeployment(hco)

			Expect(deployment.Name).To(Equal(aieWebhookName))
			Expect(deployment.Namespace).To(Equal(hco.Namespace))
			Expect(deployment.Labels).To(HaveKeyWithValue(hcoutil.AppLabel, hcoutil.HyperConvergedName))
			Expect(deployment.Labels).To(HaveKeyWithValue(hcoutil.AppLabelComponent, string(hcoutil.AppComponentAIEWebhook)))

			Expect(deployment.Spec.Replicas).To(Equal(ptr.To[int32](1)))
			Expect(deployment.Spec.Selector.MatchLabels).To(HaveKeyWithValue(hcoutil.AppLabel, hcoutil.HyperConvergedName))
			Expect(deployment.Spec.Selector.MatchLabels).To(HaveKeyWithValue(hcoutil.AppLabelComponent, string(hcoutil.AppComponentAIEWebhook)))

			Expect(deployment.Spec.Template.Labels).To(HaveKeyWithValue(hcoutil.AppLabel, hcoutil.HyperConvergedName))
			Expect(deployment.Spec.Template.Labels).To(HaveKeyWithValue(hcoutil.AppLabelComponent, string(hcoutil.AppComponentAIEWebhook)))

			Expect(deployment.Spec.Template.Spec.ServiceAccountName).To(Equal(aieWebhookServiceAccountName))

			Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))
			container := deployment.Spec.Template.Spec.Containers[0]
			Expect(container.Name).To(Equal("webhook"))
			Expect(container.Image).To(Equal(testImage))
			Expect(container.ImagePullPolicy).To(Equal(corev1.PullIfNotPresent))
			Expect(container.Args).To(ConsistOf("--metrics-bind-address=:8443",
				"--metrics-secure=true",
				"--webhook-cert-path=/tmp/k8s-webhook-server/serving-certs",
				"--metrics-cert-path=/tmp/k8s-webhook-server/serving-certs",
				"--tls-min-version=VersionTLS11",
				"--tls-cipher-suites=TLS_AES_128_GCM_SHA256,TLS_AES_256_GCM_SHA384"))

			Expect(container.Ports).To(ConsistOf(
				corev1.ContainerPort{Name: "https", ContainerPort: 9443, Protocol: corev1.ProtocolTCP},
				corev1.ContainerPort{Name: "metrics", ContainerPort: 8443, Protocol: corev1.ProtocolTCP},
				corev1.ContainerPort{Name: "health", ContainerPort: 8081, Protocol: corev1.ProtocolTCP},
			))

			Expect(container.Env).To(HaveLen(1))
			Expect(container.Env[0].Name).To(Equal("NAMESPACE"))
			Expect(container.Env[0].ValueFrom.FieldRef.FieldPath).To(Equal("metadata.namespace"))
			Expect(container.Env[0].ValueFrom.FieldRef.APIVersion).To(Equal("v1"))

			Expect(container.LivenessProbe).ToNot(BeNil())
			Expect(container.LivenessProbe.HTTPGet.Path).To(Equal("/healthz"))
			Expect(container.LivenessProbe.HTTPGet.Port).To(Equal(intstr.FromInt32(8081)))
			Expect(container.LivenessProbe.HTTPGet.Scheme).To(Equal(corev1.URISchemeHTTP))
			Expect(container.LivenessProbe.InitialDelaySeconds).To(Equal(int32(5)))
			Expect(container.LivenessProbe.PeriodSeconds).To(Equal(int32(10)))

			Expect(container.ReadinessProbe).ToNot(BeNil())
			Expect(container.ReadinessProbe.HTTPGet.Path).To(Equal("/readyz"))
			Expect(container.ReadinessProbe.HTTPGet.Port).To(Equal(intstr.FromInt32(8081)))
			Expect(container.ReadinessProbe.HTTPGet.Scheme).To(Equal(corev1.URISchemeHTTP))
			Expect(container.ReadinessProbe.InitialDelaySeconds).To(Equal(int32(5)))
			Expect(container.ReadinessProbe.PeriodSeconds).To(Equal(int32(10)))

			Expect(container.Resources.Requests).To(HaveKeyWithValue(corev1.ResourceCPU, resourceCPURequest))
			Expect(container.Resources.Requests).To(HaveKeyWithValue(corev1.ResourceMemory, resourceMemoryRequest))
			Expect(container.Resources.Limits).To(HaveKeyWithValue(corev1.ResourceCPU, resourceCPULimit))
			Expect(container.Resources.Limits).To(HaveKeyWithValue(corev1.ResourceMemory, resourceMemoryLimit))

			Expect(container.VolumeMounts).To(HaveLen(1))
			Expect(container.VolumeMounts[0].Name).To(Equal("tls-cert"))
			Expect(container.VolumeMounts[0].MountPath).To(Equal(aieWebhookCertMountPath))
			Expect(container.VolumeMounts[0].ReadOnly).To(BeTrue())

			Expect(container.SecurityContext).ToNot(BeNil())
			Expect(container.SecurityContext.AllowPrivilegeEscalation).To(Equal(ptr.To(false)))
			Expect(container.SecurityContext.ReadOnlyRootFilesystem).To(Equal(ptr.To(true)))
			Expect(container.SecurityContext.RunAsNonRoot).To(Equal(ptr.To(true)))
			Expect(container.SecurityContext.Capabilities).ToNot(BeNil())
			Expect(container.SecurityContext.Capabilities.Drop).To(ConsistOf(corev1.Capability("ALL")))
			Expect(container.SecurityContext.SeccompProfile).ToNot(BeNil())
			Expect(container.SecurityContext.SeccompProfile.Type).To(Equal(corev1.SeccompProfileTypeRuntimeDefault))

			Expect(deployment.Spec.Template.Spec.Volumes).To(HaveLen(1))
			Expect(deployment.Spec.Template.Spec.Volumes[0].Name).To(Equal("tls-cert"))
			Expect(deployment.Spec.Template.Spec.Volumes[0].Secret.SecretName).To(Equal(aieWebhookTLSSecretName))
		})

		It("should not add ciphers for TLS 1.3", func() {
			origFunc := tlssecprofile.GetCipherSuitesAndMinTLSVersion
			tlssecprofile.GetCipherSuitesAndMinTLSVersion = func(fromHC *openshiftconfigv1.TLSSecurityProfile) ([]string, openshiftconfigv1.TLSProtocolVersion) {
				return []string{"TLS_AES_128_GCM_SHA256", "TLS_AES_256_GCM_SHA384"}, openshiftconfigv1.VersionTLS13
			}

			DeferCleanup(func() {
				tlssecprofile.GetCipherSuitesAndMinTLSVersion = origFunc
			})

			deployment := newAIEWebhookDeployment(hco)

			Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))
			container := deployment.Spec.Template.Spec.Containers[0]
			Expect(container.Args).To(ConsistOf("--metrics-bind-address=:8443",
				"--metrics-secure=true",
				"--webhook-cert-path=/tmp/k8s-webhook-server/serving-certs",
				"--metrics-cert-path=/tmp/k8s-webhook-server/serving-certs",
				"--tls-min-version=VersionTLS13"))
			Expect(container.Args).ToNot(ContainElement(ContainSubstring("--tls-cipher-suites")))
		})

		It("should not add TLS version or ciphers when version is empty", func() {
			origFunc := tlssecprofile.GetCipherSuitesAndMinTLSVersion
			tlssecprofile.GetCipherSuitesAndMinTLSVersion = func(fromHC *openshiftconfigv1.TLSSecurityProfile) ([]string, openshiftconfigv1.TLSProtocolVersion) {
				return nil, ""
			}

			DeferCleanup(func() {
				tlssecprofile.GetCipherSuitesAndMinTLSVersion = origFunc
			})

			deployment := newAIEWebhookDeployment(hco)

			Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))
			container := deployment.Spec.Template.Spec.Containers[0]
			Expect(container.Args).To(ConsistOf("--metrics-bind-address=:8443",
				"--metrics-secure=true",
				"--webhook-cert-path=/tmp/k8s-webhook-server/serving-certs",
				"--metrics-cert-path=/tmp/k8s-webhook-server/serving-certs"))
			Expect(container.Args).ToNot(ContainElement(ContainSubstring("--tls-min-version")))
			Expect(container.Args).ToNot(ContainElement(ContainSubstring("--tls-cipher-suites")))
		})

		It("should not add ciphers when cipher list is empty", func() {
			origFunc := tlssecprofile.GetCipherSuitesAndMinTLSVersion
			tlssecprofile.GetCipherSuitesAndMinTLSVersion = func(fromHC *openshiftconfigv1.TLSSecurityProfile) ([]string, openshiftconfigv1.TLSProtocolVersion) {
				return nil, openshiftconfigv1.VersionTLS12
			}

			DeferCleanup(func() {
				tlssecprofile.GetCipherSuitesAndMinTLSVersion = origFunc
			})

			deployment := newAIEWebhookDeployment(hco)

			Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))
			container := deployment.Spec.Template.Spec.Containers[0]
			Expect(container.Args).To(ConsistOf("--metrics-bind-address=:8443",
				"--metrics-secure=true",
				"--webhook-cert-path=/tmp/k8s-webhook-server/serving-certs",
				"--metrics-cert-path=/tmp/k8s-webhook-server/serving-certs",
				"--tls-min-version=VersionTLS12"))
			Expect(container.Args).ToNot(ContainElement(ContainSubstring("--tls-cipher-suites")))
		})
	})

	Context("AIE Webhook Deployment handler", func() {
		It("should not create if deploy-aie-webhook annotation is absent", func() {
			delete(hco.Annotations, DeployAIEAnnotation)
			cl = commontestutils.InitClient([]client.Object{hco})

			handler := NewAIEWebhookDeploymentHandler(cl, commontestutils.GetScheme())

			res := handler.Ensure(req)
			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Created).To(BeFalse())
			Expect(res.Updated).To(BeFalse())
			Expect(res.Deleted).To(BeFalse())

			foundDep := &appsv1.DeploymentList{}
			Expect(cl.List(context.Background(), foundDep)).To(Succeed())
			Expect(foundDep.Items).To(BeEmpty())
		})

		It("should delete Deployment when deploy-aie-webhook annotation is removed", func() {
			delete(hco.Annotations, DeployAIEAnnotation)
			dep := newAIEWebhookDeployment(hco)
			cl = commontestutils.InitClient([]client.Object{hco, dep})

			handler := NewAIEWebhookDeploymentHandler(cl, commontestutils.GetScheme())

			res := handler.Ensure(req)
			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Name).To(Equal(dep.Name))
			Expect(res.Created).To(BeFalse())
			Expect(res.Updated).To(BeFalse())
			Expect(res.Deleted).To(BeTrue())

			foundDep := &appsv1.DeploymentList{}
			Expect(cl.List(context.Background(), foundDep)).To(Succeed())
			Expect(foundDep.Items).To(BeEmpty())
		})

		It("should create Deployment when deploy-aie-webhook annotation is true", func() {
			hco.Annotations[DeployAIEAnnotation] = "true"
			cl = commontestutils.InitClient([]client.Object{hco})

			handler := NewAIEWebhookDeploymentHandler(cl, commontestutils.GetScheme())

			res := handler.Ensure(req)
			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Name).To(Equal(aieWebhookName))
			Expect(res.Created).To(BeTrue())
			Expect(res.Updated).To(BeFalse())
			Expect(res.Deleted).To(BeFalse())

			foundDep := &appsv1.DeploymentList{}
			Expect(cl.List(context.Background(), foundDep)).To(Succeed())
			Expect(foundDep.Items).To(HaveLen(1))
			Expect(foundDep.Items[0].Name).To(Equal(aieWebhookName))
		})
	})

	Context("AIE Webhook Deployment update", func() {
		It("should update Deployment fields if not matched to the requirements", func() {
			hco.Annotations[DeployAIEAnnotation] = "true"
			originalDep := newAIEWebhookDeployment(hco)
			modifiedDep := originalDep.DeepCopy()
			modifiedDep.Spec.Template.Spec.Containers[0].Image = "malicious:tag"
			modifiedDep.Spec.Template.Spec.Containers[0].SecurityContext.RunAsNonRoot = ptr.To(false)
			modifiedDep.Spec.Template.Spec.Volumes = nil
			cl = commontestutils.InitClient([]client.Object{hco, modifiedDep})

			handler := NewAIEWebhookDeploymentHandler(cl, commontestutils.GetScheme())

			res := handler.Ensure(req)
			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Created).To(BeFalse())
			Expect(res.Updated).To(BeTrue())
			Expect(res.Deleted).To(BeFalse())

			reconciledDep := &appsv1.Deployment{}
			Expect(cl.Get(context.Background(), client.ObjectKey{Name: res.Name, Namespace: hco.Namespace}, reconciledDep)).To(Succeed())
			Expect(reconciledDep.Spec.Template.Spec.Containers[0].Image).
				To(Equal(originalDep.Spec.Template.Spec.Containers[0].Image))
			Expect(reconciledDep.Spec.Template.Spec.Containers[0].SecurityContext.RunAsNonRoot).
				To(Equal(originalDep.Spec.Template.Spec.Containers[0].SecurityContext.RunAsNonRoot))
			Expect(reconciledDep.Spec.Template.Spec.Volumes).
				To(Equal(originalDep.Spec.Template.Spec.Volumes))
		})

		It("should reconcile labels if they are missing while preserving user labels", func() {
			hco.Annotations[DeployAIEAnnotation] = "true"
			dep := newAIEWebhookDeployment(hco)
			expectedLabels := maps.Clone(dep.Labels)
			delete(dep.Labels, "app.kubernetes.io/component")
			dep.Labels["user-added-label"] = "user-value"
			cl = commontestutils.InitClient([]client.Object{hco, dep})

			handler := NewAIEWebhookDeploymentHandler(cl, commontestutils.GetScheme())

			res := handler.Ensure(req)
			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Created).To(BeFalse())
			Expect(res.Updated).To(BeTrue())
			Expect(res.Deleted).To(BeFalse())

			foundDep := &appsv1.Deployment{}
			Expect(cl.Get(context.Background(), client.ObjectKey{Name: aieWebhookName, Namespace: hco.Namespace}, foundDep)).To(Succeed())

			for key, value := range expectedLabels {
				Expect(foundDep.Labels).To(HaveKeyWithValue(key, value))
			}
			Expect(foundDep.Labels).To(HaveKeyWithValue("user-added-label", "user-value"))
		})
	})
})
