package observabilitycontroller

import (
	"context"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	openshiftconfigv1 "github.com/openshift/api/config/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/tlssecprofile"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

func mockTLSSecProfile(profile *openshiftconfigv1.TLSSecurityProfile) {
	origFunc := tlssecprofile.GetTLSSecurityProfile
	tlssecprofile.GetTLSSecurityProfile = func(_ *openshiftconfigv1.TLSSecurityProfile) *openshiftconfigv1.TLSSecurityProfile {
		return profile
	}
	DeferCleanup(func() {
		tlssecprofile.GetTLSSecurityProfile = origFunc
	})
}

var _ = Describe("Observability Controller Deployment", func() {
	const testImage = "quay.io/kubevirt/virt-observability-controller:test"

	BeforeEach(func() {
		origImage, origImageSet := os.LookupEnv(hcoutil.ObservabilityControllerImageEnvV)
		Expect(os.Setenv(hcoutil.ObservabilityControllerImageEnvV, testImage)).To(Succeed())
		DeferCleanup(func() {
			if origImageSet {
				Expect(os.Setenv(hcoutil.ObservabilityControllerImageEnvV, origImage)).To(Succeed())
			} else {
				Expect(os.Unsetenv(hcoutil.ObservabilityControllerImageEnvV)).To(Succeed())
			}
		})
	})

	Context("newDeployment", func() {
		It("should have all default values", func() {
			hco := commontestutils.NewHco()
			dep := newDeployment(hco)

			Expect(dep.Name).To(Equal(deploymentName))
			Expect(dep.Namespace).To(Equal(hco.Namespace))
			Expect(dep.Labels).To(HaveKeyWithValue(hcoutil.AppLabel, hcoutil.HyperConvergedName))
			Expect(dep.Labels).To(HaveKeyWithValue(hcoutil.AppLabelComponent, string(hcoutil.AppComponentObservability)))

			Expect(dep.Spec.Replicas).To(HaveValue(Equal(int32(1))))
			Expect(dep.Spec.Selector.MatchLabels).To(HaveKeyWithValue(hcoutil.AppLabel, hcoutil.HyperConvergedName))
			Expect(dep.Spec.Selector.MatchLabels).To(HaveKeyWithValue(hcoutil.AppLabelComponent, string(hcoutil.AppComponentObservability)))

			Expect(dep.Spec.Template.Spec.ServiceAccountName).To(Equal(serviceAccountName))
			Expect(dep.Spec.Template.Spec.PriorityClassName).To(Equal("system-cluster-critical"))

			Expect(dep.Spec.Template.Spec.Tolerations).To(HaveLen(1))
			Expect(dep.Spec.Template.Spec.Tolerations[0].Key).To(Equal("CriticalAddonsOnly"))

			Expect(dep.Spec.Template.Spec.Containers).To(HaveLen(1))
			container := dep.Spec.Template.Spec.Containers[0]
			Expect(container.Name).To(Equal("manager"))
			Expect(container.Image).To(Equal(testImage))
			Expect(container.ImagePullPolicy).To(Equal(corev1.PullIfNotPresent))

			Expect(container.Args).To(ContainElements(
				"--metrics-bind-address=:8443",
				"--metrics-secure=true",
				"--health-probe-bind-address=:8081",
				"--leader-elect=false",
			))

			Expect(container.Env).To(HaveLen(2))
			Expect(container.Env[0].Name).To(Equal("POD_NAMESPACE"))
			Expect(container.Env[0].ValueFrom.FieldRef.FieldPath).To(Equal("metadata.namespace"))
			Expect(container.Env[1].Name).To(Equal("POD_SERVICE_ACCOUNT"))
			Expect(container.Env[1].ValueFrom.FieldRef.FieldPath).To(Equal("spec.serviceAccountName"))

			Expect(container.Ports).To(HaveLen(2))
			Expect(container.Ports[0].ContainerPort).To(Equal(int32(8443)))
			Expect(container.Ports[0].Name).To(Equal("metrics"))
			Expect(container.Ports[1].ContainerPort).To(Equal(int32(8081)))
			Expect(container.Ports[1].Name).To(Equal("health"))

			Expect(container.LivenessProbe).ToNot(BeNil())
			Expect(container.LivenessProbe.HTTPGet.Path).To(Equal("/healthz"))
			Expect(container.ReadinessProbe).ToNot(BeNil())
			Expect(container.ReadinessProbe.HTTPGet.Path).To(Equal("/readyz"))

			Expect(container.SecurityContext.AllowPrivilegeEscalation).To(HaveValue(Equal(false)))
			Expect(container.SecurityContext.ReadOnlyRootFilesystem).To(HaveValue(Equal(true)))
			Expect(container.SecurityContext.RunAsNonRoot).To(HaveValue(Equal(true)))
			Expect(container.SecurityContext.Capabilities.Drop).To(ConsistOf(corev1.Capability("ALL")))
			Expect(container.SecurityContext.SeccompProfile.Type).To(Equal(corev1.SeccompProfileTypeRuntimeDefault))

			Expect(container.Resources.Requests).To(HaveKey(corev1.ResourceCPU))
			Expect(container.Resources.Requests).To(HaveKey(corev1.ResourceMemory))
			Expect(container.Resources.Limits).To(HaveKey(corev1.ResourceMemory))
		})
	})

	Context("TLS configuration", func() {
		var containerArgs func() []string

		BeforeEach(func() {
			containerArgs = func() []string {
				hco := commontestutils.NewHco()
				dep := newDeployment(hco)
				return dep.Spec.Template.Spec.Containers[0].Args
			}
		})

		It("should pass tls-security-profile type for Intermediate profile", func() {
			mockTLSSecProfile(&openshiftconfigv1.TLSSecurityProfile{
				Type: openshiftconfigv1.TLSProfileIntermediateType,
			})
			args := containerArgs()
			Expect(args).To(ContainElement("--tls-security-profile=Intermediate"))
			Expect(args).ToNot(ContainElement(ContainSubstring("--tls-min-version")))
			Expect(args).ToNot(ContainElement(ContainSubstring("--tls-cipher-suites")))
		})

		It("should add tls-min-version and cipher suites for Custom profile with TLS 1.2", func() {
			mockTLSSecProfile(&openshiftconfigv1.TLSSecurityProfile{
				Type: openshiftconfigv1.TLSProfileCustomType,
				Custom: &openshiftconfigv1.CustomTLSProfile{
					TLSProfileSpec: openshiftconfigv1.TLSProfileSpec{
						Ciphers:       []string{"TLS_AES_128_GCM_SHA256", "TLS_AES_256_GCM_SHA384"},
						MinTLSVersion: openshiftconfigv1.VersionTLS12,
					},
				},
			})
			args := containerArgs()
			Expect(args).To(ContainElement("--tls-security-profile=Custom"))
			Expect(args).To(ContainElement("--tls-min-version=VersionTLS12"))
			Expect(args).To(ContainElement("--tls-cipher-suites=TLS_AES_128_GCM_SHA256,TLS_AES_256_GCM_SHA384"))
		})

		It("should not add cipher suites for Custom profile with TLS 1.3", func() {
			mockTLSSecProfile(&openshiftconfigv1.TLSSecurityProfile{
				Type: openshiftconfigv1.TLSProfileCustomType,
				Custom: &openshiftconfigv1.CustomTLSProfile{
					TLSProfileSpec: openshiftconfigv1.TLSProfileSpec{
						Ciphers:       []string{"TLS_AES_128_GCM_SHA256"},
						MinTLSVersion: openshiftconfigv1.VersionTLS13,
					},
				},
			})
			args := containerArgs()
			Expect(args).To(ContainElement("--tls-security-profile=Custom"))
			Expect(args).To(ContainElement("--tls-min-version=VersionTLS13"))
			Expect(args).ToNot(ContainElement(ContainSubstring("--tls-cipher-suites")))
		})

		It("should not add cipher suites for Custom profile when cipher list is empty", func() {
			mockTLSSecProfile(&openshiftconfigv1.TLSSecurityProfile{
				Type: openshiftconfigv1.TLSProfileCustomType,
				Custom: &openshiftconfigv1.CustomTLSProfile{
					TLSProfileSpec: openshiftconfigv1.TLSProfileSpec{
						MinTLSVersion: openshiftconfigv1.VersionTLS12,
					},
				},
			})
			args := containerArgs()
			Expect(args).To(ContainElement("--tls-security-profile=Custom"))
			Expect(args).To(ContainElement("--tls-min-version=VersionTLS12"))
			Expect(args).ToNot(ContainElement(ContainSubstring("--tls-cipher-suites")))
		})
	})

	Context("Deployment spec drift", func() {
		It("should restore fields if they are modified", func() {
			hco := commontestutils.NewHco()
			hco.Spec.FeatureGates.Enable(featureGateName)
			req := commontestutils.NewReq(hco)

			expected := newDeployment(hco)
			modifiedDep := newDeployment(hco)
			modifiedDep.Spec.Template.Spec.Containers[0].Image = "malicious:tag"
			modifiedDep.Spec.Template.Spec.Containers[0].SecurityContext.RunAsNonRoot = new(false)
			modifiedDep.Labels = map[string]string{"tampered": "true"}
			cl := commontestutils.InitClient([]client.Object{hco, modifiedDep})

			handler := NewDeploymentHandler(cl, commontestutils.GetScheme())
			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Updated).To(BeTrue())

			reconciledDep := &appsv1.Deployment{}
			Expect(cl.Get(context.Background(), client.ObjectKey{Name: res.Name, Namespace: hco.Namespace}, reconciledDep)).To(Succeed())
			Expect(reconciledDep.Spec.Template.Spec.Containers[0].Image).To(Equal(testImage))
			Expect(reconciledDep.Spec.Template.Spec.Containers[0].SecurityContext.RunAsNonRoot).To(HaveValue(Equal(true)))
			for k, v := range expected.Labels {
				Expect(reconciledDep.Labels).To(HaveKeyWithValue(k, v))
			}
		})
	})

	Context("Observability allowlist flags", func() {
		It("should not add any allowlist flags when observability is nil (means all)", func() {
			hco := commontestutils.NewHco()
			hco.Spec.Observability = nil
			dep := newDeployment(hco)
			args := dep.Spec.Template.Spec.Containers[0].Args
			Expect(args).ToNot(ContainElement(ContainSubstring("--metrics-allowlist")))
			Expect(args).ToNot(ContainElement(ContainSubstring("--alerts-allowlist")))
			Expect(args).ToNot(ContainElement(ContainSubstring("--recording-rules-allowlist")))
		})

		It("should not add flags when all lists are empty", func() {
			hco := commontestutils.NewHco()
			hco.Spec.Observability = &hcov1.ObservabilityConfig{
				Workloads: &hcov1.ObservabilityWorkloadsConfig{
					AllowedMetrics: []string{},
				},
				AllowedAlerts:         []string{},
				AllowedRecordingRules: []string{},
			}
			dep := newDeployment(hco)
			args := dep.Spec.Template.Spec.Containers[0].Args
			Expect(args).ToNot(ContainElement(ContainSubstring("--metrics-allowlist")))
			Expect(args).ToNot(ContainElement(ContainSubstring("--alerts-allowlist")))
			Expect(args).ToNot(ContainElement(ContainSubstring("--recording-rules-allowlist")))
		})

		It("should pass 'none' flag when value is 'none' (means no metrics)", func() {
			hco := commontestutils.NewHco()
			hco.Spec.Observability = &hcov1.ObservabilityConfig{
				Workloads: &hcov1.ObservabilityWorkloadsConfig{
					AllowedMetrics: []string{"none"},
				},
				AllowedAlerts:         []string{"none"},
				AllowedRecordingRules: []string{"none"},
			}
			dep := newDeployment(hco)
			args := dep.Spec.Template.Spec.Containers[0].Args
			Expect(args).To(ContainElement("--metrics-allowlist=none"))
			Expect(args).To(ContainElement("--alerts-allowlist=none"))
			Expect(args).To(ContainElement("--recording-rules-allowlist=none"))
		})

		It("should pass specific metrics when listed", func() {
			hco := commontestutils.NewHco()
			hco.Spec.Observability = &hcov1.ObservabilityConfig{
				Workloads: &hcov1.ObservabilityWorkloadsConfig{
					AllowedMetrics: []string{"kubevirt_vmi_memory_used_bytes", "kubevirt_vmi_cpu_usage_seconds_total"},
				},
				AllowedAlerts:         []string{"KubeVirtVMDown", "KubeVirtVMIExcessiveMigrations"},
				AllowedRecordingRules: []string{"kubevirt_vmi_phase_count:sum"},
			}
			dep := newDeployment(hco)
			args := dep.Spec.Template.Spec.Containers[0].Args
			Expect(args).To(ContainElement("--metrics-allowlist=kubevirt_vmi_memory_used_bytes,kubevirt_vmi_cpu_usage_seconds_total"))
			Expect(args).To(ContainElement("--alerts-allowlist=KubeVirtVMDown,KubeVirtVMIExcessiveMigrations"))
			Expect(args).To(ContainElement("--recording-rules-allowlist=kubevirt_vmi_phase_count:sum"))
		})

		It("should not add metrics-allowlist when workloads is nil", func() {
			hco := commontestutils.NewHco()
			hco.Spec.Observability = &hcov1.ObservabilityConfig{
				AllowedAlerts: []string{"none"},
			}
			dep := newDeployment(hco)
			args := dep.Spec.Template.Spec.Containers[0].Args
			Expect(args).ToNot(ContainElement(ContainSubstring("--metrics-allowlist")))
			Expect(args).To(ContainElement("--alerts-allowlist=none"))
		})
	})
})
