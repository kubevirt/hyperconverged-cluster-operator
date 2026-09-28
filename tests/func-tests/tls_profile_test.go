package tests_test

import (
	"cmp"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	openshiftconfigv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/library-go/pkg/crypto"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sspapi "kubevirt.io/ssp-operator/api/v1beta3"

	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
	tests "github.com/kubevirt/hyperconverged-cluster-operator/tests/func-tests"
)

const (
	tlsDialTimeout  = 3 * time.Second
	sspReadyTimeout = 4 * time.Minute
	sspReadyPolling = 5 * time.Second
)

var _ = Describe("TLS Security Profile", Label("tls"), Serial, Ordered, func() {
	const tlsPath = "/spec/security/tlsSecurityProfile"

	DescribeTableSubtree("check TLS configurations", Serial, Ordered, func(targetPort int, podNameLabel string, serviceName, secretName string) {
		var (
			cli            client.Client
			k8sCli         *kubernetes.Clientset
			webhookAddr    string
			stopPF         func()
			shouldCheckSSP bool
			x509Alg        x509.PublicKeyAlgorithm
		)

		patchTLSProfile := func(ctx context.Context, profileJSON string) {
			GinkgoHelper()
			patch := fmt.Appendf(nil, `[{"op": "replace", "path": %q, "value": %s}]`, tlsPath, profileJSON)
			tests.PatchHCO(ctx, cli, patch)
		}

		BeforeAll(func(ctx context.Context) {
			cli = tests.GetControllerRuntimeClient()
			k8sCli = tests.GetK8sClientSet()

			if serviceName == "" || secretName == "" {
				deploymentName, err := getDeployment(ctx, k8sCli, podNameLabel)
				Expect(err).ToNot(HaveOccurred())

				serviceName = cmp.Or(serviceName, deploymentName+"-service")
				secretName = cmp.Or(secretName, deploymentName+"-service-cert")
			}

			var err error
			webhookAddr, stopPF, err = getEndpointToTest(ctx, k8sCli, targetPort, podNameLabel, serviceName)
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Printf("Using webhook endpoint: %s\n", webhookAddr)

			shouldCheckSSP, err = checkIfSSPDeployed(ctx, cli)
			Expect(err).NotTo(HaveOccurred())

			x509Alg, err = getCrtAlg(ctx, k8sCli, secretName)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterAll(func(ctx context.Context) {
			GinkgoLogr.Info("restoring TLS configuration")

			hc, err := tests.GetHCO(ctx, cli)
			Expect(err).NotTo(HaveOccurred())
			if hc.Spec.Security.TLSSecurityProfile != nil {
				removePatch := fmt.Appendf(nil, `[{"op": "remove", "path": %q}]`, tlsPath)
				tests.PatchHCO(ctx, cli, removePatch)

				expected, found := intermediateTLSResults.get(x509Alg)
				if !found {
					GinkgoLogr.Info("expected intermediate TLS result but found none; X509 public key algorithm = %v", x509Alg)
				} else {
					verifyStandardProfile(ctx, webhookAddr, openshiftconfigv1.TLSProfileIntermediateType, expected)
				}
			}

			if stopPF != nil {
				stopPF()
			}

			tests.RestoreDefaults(ctx, cli)
		})

		AfterEach(func(ctx context.Context) {
			if !shouldCheckSSP {
				return
			}

			waitForSSPReady(ctx, k8sCli)
		})

		It("should enforce Old TLS profile", func(ctx context.Context) {
			patchTLSProfile(ctx, `{"old": {}, "type": "Old"}`)
			expected, found := oldTLSResults.get(x509Alg)
			if !found {
				Skip(fmt.Sprintf("expected intermediate TLS result but found none; X509 public key algorithm = %v", x509Alg))
			}
			verifyStandardProfile(ctx, webhookAddr, openshiftconfigv1.TLSProfileOldType, expected)
		})

		It("should not change TLS profile on dry-run patch", func(ctx context.Context) {

			patch := fmt.Appendf(nil, `[{"op": "replace", "path": %q, "value": {"modern": {}, "type": "Modern"}}]`, tlsPath)
			hco := tests.HCOWithNameOnly()
			Expect(cli.Patch(ctx, hco, client.RawPatch(types.JSONPatchType, patch), client.DryRunAll)).To(Succeed())

			expected, found := oldTLSResults.get(x509Alg)
			if !found {
				Skip(fmt.Sprintf("expected intermediate TLS result but found none; X509 public key algorithm = %v", x509Alg))
			}

			verifyStandardProfile(ctx, webhookAddr, openshiftconfigv1.TLSProfileOldType, expected)
		})

		It("should enforce Intermediate TLS profile", func(ctx context.Context) {
			patchTLSProfile(ctx, `{"intermediate": {}, "type": "Intermediate"}`)

			expected, found := intermediateTLSResults.get(x509Alg)
			if !found {
				Skip(fmt.Sprintf("expected intermediate TLS result but found none; X509 public key algorithm = %v", x509Alg))
			}

			verifyStandardProfile(ctx, webhookAddr, openshiftconfigv1.TLSProfileIntermediateType, expected)
		})

		It("should enforce Modern TLS profile", func(ctx context.Context) {
			patchTLSProfile(ctx, `{"modern": {}, "type": "Modern"}`)

			expected, found := modernTLSResults.get(x509Alg)
			if !found {
				Skip(fmt.Sprintf("expected intermediate TLS result but found none; X509 public key algorithm = %v", x509Alg))
			}
			verifyStandardProfile(ctx, webhookAddr, openshiftconfigv1.TLSProfileModernType, expected)
		})

		It("should reject Custom profile missing HTTP/2-required cipher", func(ctx context.Context) {
			patch := fmt.Appendf(nil,
				`[{"op": "replace", "path": %q, "value": {"custom": {"minTLSVersion": "VersionTLS12", "ciphers": ["ECDHE-ECDSA-CHACHA20-POLY1305", "ECDHE-ECDSA-AES256-GCM-SHA384", "AES256-GCM-SHA384", "AES128-SHA256"]}, "type": "Custom"}}]`,
				tlsPath,
			)

			hco := tests.HCOWithNameOnly()
			err := cli.Patch(ctx, hco, client.RawPatch(types.JSONPatchType, patch))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("missing an HTTP/2-required"))
		})

		It("should enforce Custom TLS profile", func(ctx context.Context) {
			patchTLSProfile(ctx, `{"custom": {"minTLSVersion": "VersionTLS12", "ciphers": ["ECDHE-RSA-AES128-GCM-SHA256", "ECDHE-ECDSA-CHACHA20-POLY1305", "ECDHE-ECDSA-AES256-GCM-SHA384", "AES256-GCM-SHA384", "AES128-SHA256"]}, "type": "Custom"}`)

			expected, found := customTLSResults.get(x509Alg)
			if !found {
				Skip(fmt.Sprintf("expected intermediate TLS result but found none; X509 public key algorithm = %v", x509Alg))
			}

			verifyTLSConfig(ctx, webhookAddr, tls.VersionTLS12, []tls.CurveID{}, expected)
		})

		It("should enforce Custom TLS profile, with PQC", func(ctx context.Context) {
			patchTLSProfile(ctx, `{"custom": {"minTLSVersion": "VersionTLS12", "ciphers": ["ECDHE-ECDSA-AES256-GCM-SHA384", "ECDHE-RSA-AES128-GCM-SHA256"], "groups": ["X25519MLKEM768", "X25519"]}, "type": "Custom"}`)

			expected, found := customQPCTLSResults.get(x509Alg)
			if !found {
				Skip(fmt.Sprintf("expected intermediate TLS result but found none; X509 public key algorithm = %v", x509Alg))
			}

			verifyTLSConfig(ctx, webhookAddr, tls.VersionTLS12, []tls.CurveID{tls.X25519, tls.X25519MLKEM768}, expected)
		})
	},
		Entry("Check HCO webhook's webhook port (4343)", hcoutil.WebhookPort, "hyperconverged-cluster-webhook", "", ""),
		Entry("Check HCO webhook's metrics port (8443)", Label("tls-monitoring"), int(hcoutil.MetricsPort), "hyperconverged-cluster-webhook", "hyperconverged-cluster-webhook-operator-metrics", ""),
		// TODO: fix the secret name after PR #4625 is merged
		Entry("Check HCO operator's metrics port (8443)", Label("tls-monitoring"), int(hcoutil.MetricsPort), "hyperconverged-cluster-operator", "kubevirt-hyperconverged-operator-metrics", "hyperconverged-cluster-operator-service-cert"),
	)
})

func isTLSVersionAccepted(ctx context.Context, addr string, version uint16) (bool, error) {
	// Verify TCP connectivity first to distinguish transport errors from TLS rejections
	conn, err := net.DialTimeout("tcp", addr, tlsDialTimeout)
	if err != nil {
		return false, fmt.Errorf("transport error: %w", err)
	}

	conn.Close()

	dialer := &tls.Dialer{
		NetDialer: &net.Dialer{Timeout: tlsDialTimeout},
		Config: &tls.Config{
			InsecureSkipVerify: true,
			MinVersion:         version,
			MaxVersion:         version,
		},
	}

	conn, err = dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return false, nil
	}

	_, ok := conn.(*tls.Conn)
	if !ok {
		return false, fmt.Errorf("unexpected connection type: %T", conn)
	}

	conn.Close()
	return true, nil
}

// enumerateServerCiphers discovers which TLS 1.2 cipher suites the server
// accepts by attempting a handshake with each known suite individually.
func enumerateServerCiphers(ctx context.Context, addr string, tlsVersion uint16, curves []tls.CurveID) []uint16 {
	allSuites := getAllTLSCipherSuite()

	var supported []uint16
	for _, suite := range allSuites {
		if !slices.Contains(suite.SupportedVersions, tlsVersion) {
			continue
		}

		if isCipherSetSupported(ctx, addr, tlsVersion, suite.ID, curves) {
			supported = append(supported, suite.ID)
		}
	}

	return supported
}

func isCipherSetSupported(ctx context.Context, addr string, tlsVersion, cipherSuitID uint16, curves []tls.CurveID) bool {
	dialer := &tls.Dialer{
		NetDialer: &net.Dialer{Timeout: tlsDialTimeout},
		Config: &tls.Config{
			InsecureSkipVerify: true,
			MinVersion:         tlsVersion,
			MaxVersion:         tlsVersion,
			CipherSuites:       []uint16{cipherSuitID},
			CurvePreferences:   curves,
		},
	}

	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return false
	}

	conn.Close()

	return true
}

// verifyTLSConfig checks that the server enforces the expected minimum TLS
// version and only accepts cipher suites from the allowed set.
func verifyTLSConfig(ctx context.Context, addr string, minVersion uint16, allowedCurves []tls.CurveID, expectedCiphers map[uint16][]uint16) {
	GinkgoHelper()

	for _, v := range []uint16{tls.VersionTLS10, tls.VersionTLS11, tls.VersionTLS12, tls.VersionTLS13} {
		vStr := tls.VersionName(v)
		minVerStr := tls.VersionName(minVersion)

		By(fmt.Sprintf("checking TLS version %s with minVersion = %s", vStr, minVerStr))

		if v >= minVersion {
			Eventually(func(g Gomega, ctx context.Context) {
				accepted, err := isTLSVersionAccepted(ctx, addr, v)
				g.Expect(err).NotTo(HaveOccurred(),
					"transport error while checking TLS version %s", vStr)
				g.Expect(accepted).To(BeTrue(),
					"TLS version %s should be accepted (min version is %s)", vStr, minVerStr)

			}).WithContext(ctx).WithTimeout(30 * time.Second).WithPolling(5 * time.Second).Should(Succeed())

			Eventually(enumerateServerCiphers).
				WithContext(ctx).
				WithArguments(addr, v, allowedCurves).
				WithTimeout(30 * time.Second).
				WithPolling(5 * time.Second).
				Should(ConsistOf(expectedCiphers[v]))

		} else {
			Eventually(func(g Gomega, ctx context.Context) {
				accepted, err := isTLSVersionAccepted(ctx, addr, v)
				g.Expect(err).NotTo(HaveOccurred(),
					"transport error while checking TLS version %s", vStr)
				g.Expect(accepted).To(BeFalse(),
					"TLS version %s should be rejected (min version is %s)", vStr, minVerStr)
			}).WithContext(ctx).WithTimeout(30 * time.Second).WithPolling(5 * time.Second).Should(Succeed())
		}
	}
}

func verifyStandardProfile(ctx context.Context, addr string, profileType openshiftconfigv1.TLSProfileType, expectedCiphers map[uint16][]uint16) {
	GinkgoHelper()

	spec := openshiftconfigv1.TLSProfiles[profileType]
	Expect(spec).NotTo(BeNil(), "unknown profile type: %s", profileType)

	minVersion := crypto.TLSVersionOrDie(string(spec.MinTLSVersion))
	curvesIDs, _ := crypto.TLSGroupsToCurveIDs(spec.Groups)
	verifyTLSConfig(ctx, addr, minVersion, curvesIDs, expectedCiphers)
}

func getEndpointToTest(ctx context.Context, k8sCli *kubernetes.Clientset, targetPort int, nameLabel string, serviceName string) (string, func(), error) {
	GinkgoHelper()
	_, err := rest.InClusterConfig()
	runningInPod := err == nil

	if runningInPod {
		return getAddressFromPodIp(ctx, k8sCli, targetPort, nameLabel)
	}

	return startPortForward(tests.InstallNamespace, serviceName, targetPort)
}

func getAddressFromPodIp(ctx context.Context, k8sCli *kubernetes.Clientset, targetPort int, nameLabel string) (string, func(), error) {
	pods, err := k8sCli.CoreV1().Pods(tests.InstallNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: "name=" + nameLabel,
	})
	if err != nil {
		return "", nil, fmt.Errorf("listing pods: %w", err)
	}

	podIP := ""
	for _, pod := range pods.Items {
		if pod.Status.PodIP != "" && pod.Status.Phase == corev1.PodRunning {
			podIP = pod.Status.PodIP
			break
		}
	}

	if podIP == "" {
		return "", nil, fmt.Errorf("no pod with an assigned IP found for label name=%s", nameLabel)
	}

	addr := net.JoinHostPort(podIP, strconv.Itoa(targetPort))
	Eventually(func() error {
		conn, dialErr := net.DialTimeout("tcp", addr, time.Second)
		if dialErr != nil {
			return dialErr
		}
		conn.Close()
		return nil
	}).WithTimeout(30*time.Second).WithPolling(2*time.Second).Should(Succeed(),
		"pod at %s did not become ready", addr)
	return addr, func() {}, nil
}

func getDeployment(ctx context.Context, k8sCli *kubernetes.Clientset, nameLabel string) (string, error) {
	deployments, err := k8sCli.AppsV1().Deployments(tests.InstallNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: "name=" + nameLabel,
	})

	if err != nil {
		return "", fmt.Errorf("can't find deployment for label name=%s; %w", nameLabel, err)
	}

	deployment := deployments.Items[0]
	if !slices.ContainsFunc(deployment.Status.Conditions, func(condition appsv1.DeploymentCondition) bool {
		return condition.Status == corev1.ConditionTrue && condition.Type == appsv1.DeploymentAvailable
	}) {
		return "", fmt.Errorf("deployment %s is not ready", deployment.Name)
	}

	// just for case
	if deployment.Status.Replicas > 0 && deployment.Status.Replicas != deployment.Status.AvailableReplicas {
		return "", fmt.Errorf("deployment %s is not ready (AvailableReplicas: %d, Required replicas: %d)", deployment.Name, deployment.Status.AvailableReplicas, deployment.Status.Replicas)
	}

	return deployment.Name, nil
}

func startPortForward(namespace, serviceName string, targetPort int) (string, func(), error) {
	GinkgoHelper()
	binary := "kubectl"
	if b := os.Getenv("KUBECTL_BINARY"); b != "" {
		binary = b
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", nil, fmt.Errorf("finding free port: %w", err)
	}
	localPort := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		return "", nil, fmt.Errorf("releasing port listener: %w", err)
	}

	cmd := exec.Command(binary, "port-forward", "-n", namespace,
		fmt.Sprintf("service/%s", serviceName),
		fmt.Sprintf("%d:%d", localPort, targetPort))
	cmd.Stderr = GinkgoWriter

	if err := cmd.Start(); err != nil {
		return "", nil, fmt.Errorf("starting port-forward: %w", err)
	}

	addr := fmt.Sprintf("127.0.0.1:%d", localPort)
	cleanup := func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	}
	ready := false
	defer func() {
		if !ready {
			cleanup()
		}
	}()

	Eventually(func() error {
		conn, dialErr := net.DialTimeout("tcp", addr, time.Second)
		if dialErr != nil {
			return dialErr
		}
		conn.Close()
		return nil
	}).WithTimeout(30*time.Second).WithPolling(2*time.Second).Should(Succeed(),
		"port-forward to %s did not become ready", serviceName)

	ready = true
	return addr, cleanup, nil
}

func waitForSSPReady(ctx context.Context, k8sCli *kubernetes.Clientset) {
	GinkgoHelper()

	Eventually(func(g Gomega, ctx context.Context) {
		deploy, err := k8sCli.AppsV1().Deployments(tests.InstallNamespace).Get(ctx, "ssp-operator", metav1.GetOptions{})
		g.Expect(err).NotTo(HaveOccurred())

		available := slices.ContainsFunc(deploy.Status.Conditions, func(c appsv1.DeploymentCondition) bool {
			return c.Type == appsv1.DeploymentAvailable && c.Status == corev1.ConditionTrue
		})
		g.Expect(available).To(BeTrue(), "ssp-operator deployment is not available")
	}).WithTimeout(sspReadyTimeout).WithPolling(sspReadyPolling).WithContext(ctx).Should(Succeed())
}

var (
	allCiphersOnce     = new(sync.Once)
	allTLSCipherSuites []*tls.CipherSuite
)

func getAllTLSCipherSuite() []*tls.CipherSuite {
	allCiphersOnce.Do(func() {
		allSuites := sets.New[*tls.CipherSuite](tls.CipherSuites()...).Insert(tls.InsecureCipherSuites()...)
		allTLSCipherSuites = allSuites.UnsortedList()
	})

	return slices.Clone(allTLSCipherSuites)
}

func checkIfSSPDeployed(ctx context.Context, cli client.Client) (bool, error) {
	ssps := new(sspapi.SSPList)

	err := cli.List(ctx, ssps, client.InNamespace(tests.InstallNamespace))
	if _, ok := errors.AsType[*meta.NoKindMatchError](err); ok {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	return len(ssps.Items) == 1, nil
}

func getCrtAlg(ctx context.Context, clientSet *kubernetes.Clientset, secretName string) (x509.PublicKeyAlgorithm, error) {
	secret, err := clientSet.CoreV1().Secrets(tests.InstallNamespace).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		return x509.UnknownPublicKeyAlgorithm, fmt.Errorf("can't get secret %s; %w", secretName, err)
	}

	crtBytes, ok := secret.Data["tls.crt"]
	if !ok {
		return x509.UnknownPublicKeyAlgorithm, fmt.Errorf("can't find tls.crt key in secret %s", secretName)
	}

	block, _ := pem.Decode(crtBytes)
	if block == nil {
		return x509.UnknownPublicKeyAlgorithm, fmt.Errorf("can't PEM decode the %s secret", secretName)
	}

	certificate, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return x509.UnknownPublicKeyAlgorithm, fmt.Errorf("can't parse the %s secret; %w", secretName, err)
	}

	return certificate.PublicKeyAlgorithm, nil
}
