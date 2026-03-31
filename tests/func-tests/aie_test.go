package tests_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	tests "github.com/kubevirt/hyperconverged-cluster-operator/tests/func-tests"
)

const (
	aieWebhookName          = "kubevirt-aie-webhook"
	aieWebhookTLSSecretName = "kubevirt-aie-webhook-tls"
	aieWebhookConfigMapName = "kubevirt-aie-launcher-config"
	iommufdDevicePluginName = "iommufd-device-plugin"
	deployAIEAnnotationKey  = "hco.kubevirt.io/deployAIE"
)

var _ = Describe("Test AIE", Label("AIE"), Serial, Ordered, func() {
	tests.FlagParse()

	var cli client.Client

	BeforeAll(func(ctx context.Context) {
		cli = tests.GetControllerRuntimeClient()

		Expect(admissionregistrationv1.AddToScheme(cli.Scheme())).To(Succeed())
	})

	AfterAll(func(ctx context.Context) {
		disableAIEFeatureGate(ctx, cli)

		deleteAIEWebhookTLSSecret(ctx, cli)

		validateAIEDeleted(ctx, cli, getAIEWebhookDeploymentErr)
		validateAIEDeleted(ctx, cli, getAIEWebhookServiceErr)
		validateAIEDeleted(ctx, cli, getAIEWebhookServiceAccountErr)
		validateAIEDeleted(ctx, cli, getAIEWebhookConfigMapErr)
		validateAIEDeleted(ctx, cli, getAIEWebhookClusterRoleErr)
		validateAIEDeleted(ctx, cli, getAIEWebhookClusterRoleBindingErr)
		validateAIEDeleted(ctx, cli, getAIEWebhookMutatingWebhookConfigurationErr)
		validateAIEDeleted(ctx, cli, getIOMMUFDDevicePluginDaemonSetErr)
		validateAIEDeleted(ctx, cli, getIOMMUFDDevicePluginServiceAccountErr)
	})

	When("deployAIE annotation is set to true", func() {
		It("should deploy AIE webhook components", func(ctx context.Context) {
			enableAIEFeatureGate(ctx, cli)

			By("creating a self-signed TLS secret for the webhook")
			ensureAIEWebhookTLSSecret(ctx, cli)

			By("check the AIE webhook Deployment")
			Eventually(func(g Gomega, ctx context.Context) {
				dep := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      aieWebhookName,
						Namespace: tests.InstallNamespace,
					},
				}
				g.Expect(cli.Get(ctx, client.ObjectKeyFromObject(dep), dep)).To(Succeed())
				g.Expect(dep.Status.ReadyReplicas).To(Equal(*dep.Spec.Replicas))
			}).WithTimeout(5 * time.Minute).WithPolling(time.Second).WithContext(ctx).Should(Succeed())

			By("check the AIE webhook Service")
			Eventually(func(ctx context.Context) error {
				return getAIEWebhookServiceErr(ctx, cli)
			}).WithTimeout(1 * time.Minute).
				WithPolling(100 * time.Millisecond).
				WithContext(ctx).
				Should(Succeed())

			By("check the AIE webhook ServiceAccount")
			Eventually(func(ctx context.Context) error {
				return getAIEWebhookServiceAccountErr(ctx, cli)
			}).WithTimeout(1 * time.Minute).
				WithPolling(100 * time.Millisecond).
				WithContext(ctx).
				Should(Succeed())

			By("check the AIE webhook ConfigMap")
			Eventually(func(ctx context.Context) error {
				return getAIEWebhookConfigMapErr(ctx, cli)
			}).WithTimeout(1 * time.Minute).
				WithPolling(100 * time.Millisecond).
				WithContext(ctx).
				Should(Succeed())

			By("check the AIE webhook ClusterRole")
			Eventually(func(ctx context.Context) error {
				return getAIEWebhookClusterRoleErr(ctx, cli)
			}).WithTimeout(1 * time.Minute).
				WithPolling(100 * time.Millisecond).
				WithContext(ctx).
				Should(Succeed())

			By("check the AIE webhook ClusterRoleBinding")
			Eventually(func(ctx context.Context) error {
				return getAIEWebhookClusterRoleBindingErr(ctx, cli)
			}).WithTimeout(1 * time.Minute).
				WithPolling(100 * time.Millisecond).
				WithContext(ctx).
				Should(Succeed())

			By("check the AIE webhook MutatingWebhookConfiguration")
			Eventually(func(ctx context.Context) error {
				return getAIEWebhookMutatingWebhookConfigurationErr(ctx, cli)
			}).WithTimeout(1 * time.Minute).
				WithPolling(100 * time.Millisecond).
				WithContext(ctx).
				Should(Succeed())
		})

		It("should deploy iommufd-device-plugin DaemonSet", func(ctx context.Context) {
			enableAIEFeatureGate(ctx, cli)

			By("check the iommufd-device-plugin ServiceAccount")
			Eventually(func(ctx context.Context) error {
				return getIOMMUFDDevicePluginServiceAccountErr(ctx, cli)
			}).WithTimeout(1 * time.Minute).
				WithPolling(100 * time.Millisecond).
				WithContext(ctx).
				Should(Succeed())

			By("check the iommufd-device-plugin DaemonSet")
			Eventually(func(g Gomega, ctx context.Context) {
				ds := &appsv1.DaemonSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      iommufdDevicePluginName,
						Namespace: tests.InstallNamespace,
					},
				}
				g.Expect(cli.Get(ctx, client.ObjectKeyFromObject(ds), ds)).To(Succeed())
				g.Expect(ds.Status.DesiredNumberScheduled).To(BeNumerically(">", 0))
				g.Expect(ds.Status.NumberReady).To(Equal(ds.Status.DesiredNumberScheduled))
			}).WithTimeout(5 * time.Minute).WithPolling(time.Second).WithContext(ctx).Should(Succeed())
		})

		It("should preserve user edits to the ConfigMap across reconciliation", func(ctx context.Context) {
			enableAIEFeatureGate(ctx, cli)
			ensureAIEWebhookTLSSecret(ctx, cli)

			By("waiting for the ConfigMap to be created")
			Eventually(func(ctx context.Context) error {
				return getAIEWebhookConfigMapErr(ctx, cli)
			}).WithTimeout(2 * time.Minute).
				WithPolling(time.Second).
				WithContext(ctx).
				Should(Succeed())

			By("editing the ConfigMap with custom rules")
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      aieWebhookConfigMapName,
					Namespace: tests.InstallNamespace,
				},
			}
			Expect(cli.Get(ctx, client.ObjectKeyFromObject(cm), cm)).To(Succeed())
			customConfig := "rules:\n- name: \"test-rule\"\n  image: \"quay.io/test/virt-launcher:latest\"\n  selector:\n    deviceNames:\n    - \"nvidia.com/TEST_GPU\"\n"
			cm.Data["config.yaml"] = customConfig
			Expect(cli.Update(ctx, cm)).To(Succeed())

			By("triggering a reconcile by touching the HCO CR")
			patchBytes := []byte(fmt.Sprintf(`{"metadata":{"annotations":{"%s":"true"}}}`, deployAIEAnnotationKey))
			Eventually(tests.PatchMergeHCO).
				WithArguments(ctx, cli, patchBytes).
				WithTimeout(10 * time.Second).
				WithPolling(100 * time.Millisecond).
				WithContext(ctx).
				Should(Succeed())

			By("verifying user edits persist after reconciliation")
			Consistently(func(g Gomega, ctx context.Context) {
				updatedCM := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      aieWebhookConfigMapName,
						Namespace: tests.InstallNamespace,
					},
				}
				g.Expect(cli.Get(ctx, client.ObjectKeyFromObject(updatedCM), updatedCM)).To(Succeed())
				g.Expect(updatedCM.Data["config.yaml"]).To(Equal(customConfig))
			}).WithTimeout(30 * time.Second).
				WithPolling(time.Second).
				WithContext(ctx).
				Should(Succeed())
		})

		It("should remove AIE resources when feature gate is disabled", func(ctx context.Context) {
			enableAIEFeatureGate(ctx, cli)
			ensureAIEWebhookTLSSecret(ctx, cli)

			By("waiting for the Deployment to be created")
			Eventually(func(ctx context.Context) error {
				return getAIEWebhookDeploymentErr(ctx, cli)
			}).WithTimeout(2 * time.Minute).
				WithPolling(time.Second).
				WithContext(ctx).
				Should(Succeed())

			By("waiting for the DaemonSet to be created")
			Eventually(func(ctx context.Context) error {
				return getIOMMUFDDevicePluginDaemonSetErr(ctx, cli)
			}).WithTimeout(2 * time.Minute).
				WithPolling(time.Second).
				WithContext(ctx).
				Should(Succeed())

			By("disabling the AIE feature gate")
			disableAIEFeatureGate(ctx, cli)

			By("checking that all AIE resources are removed")
			validateAIEDeleted(ctx, cli, getAIEWebhookDeploymentErr)
			validateAIEDeleted(ctx, cli, getAIEWebhookServiceErr)
			validateAIEDeleted(ctx, cli, getAIEWebhookServiceAccountErr)
			validateAIEDeleted(ctx, cli, getAIEWebhookConfigMapErr)
			validateAIEDeleted(ctx, cli, getAIEWebhookClusterRoleErr)
			validateAIEDeleted(ctx, cli, getAIEWebhookClusterRoleBindingErr)
			validateAIEDeleted(ctx, cli, getAIEWebhookMutatingWebhookConfigurationErr)
			validateAIEDeleted(ctx, cli, getIOMMUFDDevicePluginDaemonSetErr)
			validateAIEDeleted(ctx, cli, getIOMMUFDDevicePluginServiceAccountErr)
		})
	})
})

type aieGetResourceFn func(ctx context.Context, cli client.Client) error

func validateAIEDeleted(ctx context.Context, cli client.Client, tryGetResource aieGetResourceFn) {
	GinkgoHelper()
	Eventually(func(ctx context.Context) error {
		return tryGetResource(ctx, cli)
	}).WithTimeout(2 * time.Minute).
		WithPolling(time.Second).
		WithContext(ctx).
		Should(MatchError(k8serrors.IsNotFound, "should be not-found error"))
}

func enableAIEFeatureGate(ctx context.Context, cli client.Client) {
	GinkgoHelper()
	patchBytes := []byte(fmt.Sprintf(`{"metadata":{"annotations":{"%s":"true"}}}`, deployAIEAnnotationKey))

	Eventually(tests.PatchMergeHCO).
		WithArguments(ctx, cli, patchBytes).
		WithTimeout(10 * time.Second).
		WithPolling(100 * time.Millisecond).
		WithContext(ctx).
		WithOffset(1).
		Should(Succeed())
}

func disableAIEFeatureGate(ctx context.Context, cli client.Client) {
	GinkgoHelper()
	patchBytes := []byte(fmt.Sprintf(`{"metadata":{"annotations":{"%s":null}}}`, deployAIEAnnotationKey))

	Eventually(tests.PatchMergeHCO).
		WithArguments(ctx, cli, patchBytes).
		WithTimeout(10 * time.Second).
		WithPolling(100 * time.Millisecond).
		WithContext(ctx).
		WithOffset(1).
		Should(Succeed())
}

func getAIEWebhookDeploymentErr(ctx context.Context, cli client.Client) error {
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      aieWebhookName,
			Namespace: tests.InstallNamespace,
		},
	}
	return cli.Get(ctx, client.ObjectKeyFromObject(dep), dep)
}

func getAIEWebhookServiceErr(ctx context.Context, cli client.Client) error {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      aieWebhookName,
			Namespace: tests.InstallNamespace,
		},
	}
	return cli.Get(ctx, client.ObjectKeyFromObject(svc), svc)
}

func getAIEWebhookServiceAccountErr(ctx context.Context, cli client.Client) error {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      aieWebhookName,
			Namespace: tests.InstallNamespace,
		},
	}
	return cli.Get(ctx, client.ObjectKeyFromObject(sa), sa)
}

func getAIEWebhookConfigMapErr(ctx context.Context, cli client.Client) error {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      aieWebhookConfigMapName,
			Namespace: tests.InstallNamespace,
		},
	}
	return cli.Get(ctx, client.ObjectKeyFromObject(cm), cm)
}

func getAIEWebhookClusterRoleErr(ctx context.Context, cli client.Client) error {
	cr := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: aieWebhookName,
		},
	}
	return cli.Get(ctx, client.ObjectKeyFromObject(cr), cr)
}

func getAIEWebhookClusterRoleBindingErr(ctx context.Context, cli client.Client) error {
	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: aieWebhookName,
		},
	}
	return cli.Get(ctx, client.ObjectKeyFromObject(crb), crb)
}

func getAIEWebhookMutatingWebhookConfigurationErr(ctx context.Context, cli client.Client) error {
	mwc := &admissionregistrationv1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: aieWebhookName,
		},
	}
	return cli.Get(ctx, client.ObjectKeyFromObject(mwc), mwc)
}

func getIOMMUFDDevicePluginDaemonSetErr(ctx context.Context, cli client.Client) error {
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      iommufdDevicePluginName,
			Namespace: tests.InstallNamespace,
		},
	}
	return cli.Get(ctx, client.ObjectKeyFromObject(ds), ds)
}

func getIOMMUFDDevicePluginServiceAccountErr(ctx context.Context, cli client.Client) error {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      iommufdDevicePluginName,
			Namespace: tests.InstallNamespace,
		},
	}
	return cli.Get(ctx, client.ObjectKeyFromObject(sa), sa)
}

func ensureAIEWebhookTLSSecret(ctx context.Context, cli client.Client) {
	GinkgoHelper()

	// On OpenShift the service-ca operator provisions the TLS secret
	// automatically via the Service annotation. Wait for it if so.
	isOpenShift, err := tests.IsOpenShift(ctx, cli)
	Expect(err).ToNot(HaveOccurred())
	if isOpenShift {
		Eventually(func(ctx context.Context) error {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      aieWebhookTLSSecretName,
					Namespace: tests.InstallNamespace,
				},
			}
			return cli.Get(ctx, client.ObjectKeyFromObject(secret), secret)
		}).WithTimeout(2 * time.Minute).
			WithPolling(time.Second).
			WithContext(ctx).
			Should(Succeed())
		return
	}

	// On non-OpenShift clusters generate a self-signed certificate
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      aieWebhookTLSSecretName,
			Namespace: tests.InstallNamespace,
		},
	}
	if err := cli.Get(ctx, client.ObjectKeyFromObject(secret), secret); err == nil {
		return
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	Expect(err).ToNot(HaveOccurred())

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: fmt.Sprintf("%s.%s.svc", aieWebhookName, tests.InstallNamespace),
		},
		DNSNames: []string{
			aieWebhookName,
			fmt.Sprintf("%s.%s", aieWebhookName, tests.InstallNamespace),
			fmt.Sprintf("%s.%s.svc", aieWebhookName, tests.InstallNamespace),
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	Expect(err).ToNot(HaveOccurred())

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	keyDER, err := x509.MarshalECPrivateKey(key)
	Expect(err).ToNot(HaveOccurred())
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	secret = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      aieWebhookTLSSecretName,
			Namespace: tests.InstallNamespace,
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			corev1.TLSCertKey:       certPEM,
			corev1.TLSPrivateKeyKey: keyPEM,
		},
	}
	Expect(cli.Create(ctx, secret)).To(Succeed())
}

func deleteAIEWebhookTLSSecret(ctx context.Context, cli client.Client) {
	GinkgoHelper()

	// On OpenShift the service-ca operator owns the TLS secret; don't delete it.
	isOpenShift, err := tests.IsOpenShift(ctx, cli)
	Expect(err).ToNot(HaveOccurred())
	if isOpenShift {
		return
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      aieWebhookTLSSecretName,
			Namespace: tests.InstallNamespace,
		},
	}
	if err := cli.Delete(ctx, secret); err != nil && !k8serrors.IsNotFound(err) {
		Expect(err).ToNot(HaveOccurred())
	}
}
