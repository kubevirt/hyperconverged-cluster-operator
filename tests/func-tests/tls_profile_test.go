package tests_test

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	openshiftconfigv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/library-go/pkg/crypto"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	tests "github.com/kubevirt/hyperconverged-cluster-operator/tests/func-tests"
)

const (
	webhookPort      = 4343
	tlsDialTimeout   = 3 * time.Second
	tlsVerifyTimeout = 2 * time.Minute
	tlsVerifyPolling = 5 * time.Second
	sspReadyTimeout  = 4 * time.Minute
	sspReadyPolling  = 5 * time.Second
)

var tls13CipherNames = map[string]bool{
	"TLS_AES_128_GCM_SHA256":       true,
	"TLS_AES_256_GCM_SHA384":       true,
	"TLS_CHACHA20_POLY1305_SHA256": true,
}

func goTLSVersion(v openshiftconfigv1.TLSProtocolVersion) uint16 {
	switch v {
	case openshiftconfigv1.VersionTLS10:
		return tls.VersionTLS10
	case openshiftconfigv1.VersionTLS11:
		return tls.VersionTLS11
	case openshiftconfigv1.VersionTLS12:
		return tls.VersionTLS12
	case openshiftconfigv1.VersionTLS13:
		return tls.VersionTLS13
	default:
		return tls.VersionTLS12
	}
}

// tls12CipherIDsFromSpec extracts TLS 1.2 cipher suite IDs from a profile spec,
// filtering out TLS 1.3 ciphers which are not configurable in Go's crypto/tls.
func tls12CipherIDsFromSpec(spec *openshiftconfigv1.TLSProfileSpec) []uint16 {
	var opensslNames []string
	for _, c := range spec.Ciphers {
		if !tls13CipherNames[c] {
			opensslNames = append(opensslNames, c)
		}
	}

	if len(opensslNames) == 0 {
		return nil
	}

	ianaCiphers := crypto.OpenSSLToIANACipherSuites(opensslNames)
	return crypto.CipherSuitesOrDie(ianaCiphers)
}

func isTLSVersionAccepted(addr string, version uint16) bool {
	conn, err := tls.DialWithDialer(
		&net.Dialer{Timeout: tlsDialTimeout},
		"tcp", addr,
		&tls.Config{
			InsecureSkipVerify: true,
			MinVersion:         version,
			MaxVersion:         version,
		},
	)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// enumerateServerCiphers discovers which TLS 1.2 cipher suites the server
// accepts by attempting a handshake with each known suite individually.
func enumerateServerCiphers(addr string, tlsVersion uint16) []uint16 {
	allSuites := append(tls.CipherSuites(), tls.InsecureCipherSuites()...)

	var supported []uint16
	for _, suite := range allSuites {
		supportsVersion := false
		for _, v := range suite.SupportedVersions {
			if v == tlsVersion {
				supportsVersion = true
				break
			}
		}
		if !supportsVersion {
			continue
		}

		conn, err := tls.DialWithDialer(
			&net.Dialer{Timeout: tlsDialTimeout},
			"tcp", addr,
			&tls.Config{
				InsecureSkipVerify: true,
				MinVersion:         tlsVersion,
				MaxVersion:         tlsVersion,
				CipherSuites:       []uint16{suite.ID},
			},
		)
		if err != nil {
			continue
		}
		supported = append(supported, conn.ConnectionState().CipherSuite)
		conn.Close()
	}

	return supported
}

// verifyTLSConfig checks that the server enforces the expected minimum TLS
// version and only accepts cipher suites from the allowed set.
func verifyTLSConfig(g Gomega, addr string, minVersion uint16, allowedTLS12Ciphers []uint16) {
	for _, v := range []uint16{tls.VersionTLS10, tls.VersionTLS11, tls.VersionTLS12} {
		if v < minVersion {
			g.Expect(isTLSVersionAccepted(addr, v)).To(BeFalse(),
				"TLS version 0x%04x should be rejected (min version is 0x%04x)", v, minVersion)
		}
	}

	g.Expect(isTLSVersionAccepted(addr, tls.VersionTLS13)).To(BeTrue(),
		"TLS 1.3 should always be accepted")

	if minVersion <= tls.VersionTLS12 {
		serverCiphers := enumerateServerCiphers(addr, tls.VersionTLS12)
		g.Expect(serverCiphers).NotTo(BeEmpty(),
			"server should accept at least one TLS 1.2 cipher")

		allowedSet := make(map[uint16]bool, len(allowedTLS12Ciphers))
		for _, c := range allowedTLS12Ciphers {
			allowedSet[c] = true
		}

		for _, c := range serverCiphers {
			g.Expect(allowedSet[c]).To(BeTrue(),
				"server accepts cipher %s (0x%04x) which is not in the allowed set for this profile",
				tls.CipherSuiteName(c), c)
		}
	}
}

func verifyStandardProfile(g Gomega, addr string, profileType openshiftconfigv1.TLSProfileType) {
	spec := openshiftconfigv1.TLSProfiles[profileType]
	g.Expect(spec).NotTo(BeNil(), "unknown profile type: %s", profileType)

	minVersion := goTLSVersion(spec.MinTLSVersion)
	cipherIDs := tls12CipherIDsFromSpec(spec)
	verifyTLSConfig(g, addr, minVersion, cipherIDs)
}

func getWebhookEndpoint(ctx context.Context, k8sCli *kubernetes.Clientset) (string, func(), error) {
	namespace := tests.InstallNamespace

	if os.Getenv("OPENSHIFT_BUILD_NAMESPACE") != "" {
		pods, err := k8sCli.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: "name=hyperconverged-cluster-webhook",
		})
		if err != nil {
			return "", nil, fmt.Errorf("listing webhook pods: %w", err)
		}

		podIP := ""
		for _, pod := range pods.Items {
			if pod.Status.PodIP != "" {
				podIP = pod.Status.PodIP
				break
			}
		}
		if podIP == "" {
			return "", nil, fmt.Errorf("no webhook pod with an assigned IP found for label name=hyperconverged-cluster-webhook")
		}

		addr := net.JoinHostPort(podIP, strconv.Itoa(webhookPort))
		Eventually(func() error {
			conn, dialErr := net.DialTimeout("tcp", addr, time.Second)
			if dialErr != nil {
				return dialErr
			}
			conn.Close()
			return nil
		}).WithTimeout(30 * time.Second).WithPolling(time.Second).Should(Succeed(),
			"webhook at %s did not become ready", addr)
		return addr, func() {}, nil
	}

	serviceName, err := getWebhookServiceName(ctx, k8sCli, namespace)
	if err != nil {
		return "", nil, err
	}

	return startPortForward(namespace, serviceName)
}

func getWebhookServiceName(ctx context.Context, k8sCli *kubernetes.Clientset, namespace string) (string, error) {
	for _, name := range []string{"hyperconverged-cluster-webhook-service", "hco-webhook-service"} {
		_, err := k8sCli.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
		if err == nil {
			return name, nil
		}
		if !apierrors.IsNotFound(err) {
			return "", fmt.Errorf("checking service %s: %w", name, err)
		}
	}
	return "", fmt.Errorf("unable to find HCO webhook service in namespace %s", tests.InstallNamespace)
}

func startPortForward(namespace, serviceName string) (string, func(), error) {
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

	cmd := exec.Command(binary, "port-forward", "-n", namespace, //nolint:gosec
		fmt.Sprintf("service/%s", serviceName),
		fmt.Sprintf("%d:%d", localPort, webhookPort))
	cmd.Stdout = GinkgoWriter
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
	}).WithTimeout(30 * time.Second).WithPolling(time.Second).Should(Succeed(),
		"port-forward to %s did not become ready", serviceName)

	ready = true
	return addr, cleanup, nil
}

func waitForSSPReady(ctx context.Context, k8sCli *kubernetes.Clientset) {
	GinkgoHelper()

	_, err := k8sCli.AppsV1().Deployments(tests.InstallNamespace).Get(ctx, "ssp-operator", metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return
	}
	Expect(err).NotTo(HaveOccurred())

	Eventually(func(g Gomega, ctx context.Context) {
		deploy, err := k8sCli.AppsV1().Deployments(tests.InstallNamespace).Get(ctx, "ssp-operator", metav1.GetOptions{})
		g.Expect(err).NotTo(HaveOccurred())

		found := false
		for _, c := range deploy.Status.Conditions {
			if c.Type == appsv1.DeploymentAvailable {
				g.Expect(c.Status).To(Equal(corev1.ConditionTrue),
					"ssp-operator deployment is not available")
				found = true
				break
			}
		}
		g.Expect(found).To(BeTrue(), "ssp-operator deployment missing Available condition")
	}).WithTimeout(sspReadyTimeout).WithPolling(sspReadyPolling).WithContext(ctx).Should(Succeed())
}

var _ = Describe("TLS Security Profile", Label("tls"), Serial, Ordered, func() {
	const tlsPath = "/spec/security/tlsSecurityProfile"

	var (
		cli         client.Client
		k8sCli      *kubernetes.Clientset
		webhookAddr string
		stopPF      func()
	)

	patchTLSProfile := func(ctx context.Context, profileJSON string) {
		GinkgoHelper()
		patch := fmt.Appendf(nil, `[{"op": "replace", "path": %q, "value": %s}]`, tlsPath, profileJSON)
		tests.PatchHCO(ctx, cli, patch)
	}

	BeforeAll(func(ctx context.Context) {
		cli = tests.GetControllerRuntimeClient()
		k8sCli = tests.GetK8sClientSet()

		var err error
		webhookAddr, stopPF, err = getWebhookEndpoint(ctx, k8sCli)
		Expect(err).NotTo(HaveOccurred())
		GinkgoWriter.Printf("Using webhook endpoint: %s\n", webhookAddr)
	})

	AfterAll(func(ctx context.Context) {
		if stopPF != nil {
			stopPF()
		}
		tests.RestoreDefaults(ctx, cli)
	})

	It("should enforce Old TLS profile", func(ctx context.Context) {
		patchTLSProfile(ctx, `{"old": {}, "type": "Old"}`)

		Eventually(func(g Gomega) {
			verifyStandardProfile(g, webhookAddr, openshiftconfigv1.TLSProfileOldType)
		}).WithTimeout(tlsVerifyTimeout).WithPolling(tlsVerifyPolling).Should(Succeed())

		waitForSSPReady(ctx, k8sCli)
	})

	It("should not change TLS profile on dry-run patch", func(ctx context.Context) {
		patch := []byte(fmt.Sprintf(`[{"op": "replace", "path": %q, "value": {"modern": {}, "type": "Modern"}}]`, tlsPath))
		hco := tests.HCOWithNameOnly()
		Expect(cli.Patch(ctx, hco, client.RawPatch(types.JSONPatchType, patch), client.DryRunAll)).To(Succeed())

		verifyStandardProfile(Default, webhookAddr, openshiftconfigv1.TLSProfileOldType)

		waitForSSPReady(ctx, k8sCli)
	})

	It("should enforce Intermediate TLS profile", func(ctx context.Context) {
		patchTLSProfile(ctx, `{"intermediate": {}, "type": "Intermediate"}`)

		Eventually(func(g Gomega) {
			verifyStandardProfile(g, webhookAddr, openshiftconfigv1.TLSProfileIntermediateType)
		}).WithTimeout(tlsVerifyTimeout).WithPolling(tlsVerifyPolling).Should(Succeed())

		waitForSSPReady(ctx, k8sCli)
	})

	It("should enforce Modern TLS profile", func(ctx context.Context) {
		patchTLSProfile(ctx, `{"modern": {}, "type": "Modern"}`)

		Eventually(func(g Gomega) {
			verifyStandardProfile(g, webhookAddr, openshiftconfigv1.TLSProfileModernType)
		}).WithTimeout(tlsVerifyTimeout).WithPolling(tlsVerifyPolling).Should(Succeed())

		waitForSSPReady(ctx, k8sCli)
	})

	It("should reject Custom profile missing HTTP/2-required cipher", func(ctx context.Context) {
		patch := []byte(fmt.Sprintf(
			`[{"op": "replace", "path": %q, "value": {"custom": {"minTLSVersion": "VersionTLS12", "ciphers": ["ECDHE-ECDSA-CHACHA20-POLY1305", "ECDHE-ECDSA-AES256-GCM-SHA384", "AES256-GCM-SHA384", "AES128-SHA256"]}, "type": "Custom"}}]`,
			tlsPath,
		))

		hco := tests.HCOWithNameOnly()
		err := cli.Patch(ctx, hco, client.RawPatch(types.JSONPatchType, patch))
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("missing an HTTP/2-required"))

		waitForSSPReady(ctx, k8sCli)
	})

	It("should enforce Custom TLS profile", func(ctx context.Context) {
		customCiphers := []string{
			"ECDHE-RSA-AES128-GCM-SHA256",
			"ECDHE-ECDSA-CHACHA20-POLY1305",
			"ECDHE-ECDSA-AES256-GCM-SHA384",
			"AES256-GCM-SHA384",
			"AES128-SHA256",
		}

		patchTLSProfile(ctx, `{"custom": {"minTLSVersion": "VersionTLS12", "ciphers": ["ECDHE-RSA-AES128-GCM-SHA256", "ECDHE-ECDSA-CHACHA20-POLY1305", "ECDHE-ECDSA-AES256-GCM-SHA384", "AES256-GCM-SHA384", "AES128-SHA256"]}, "type": "Custom"}`)

		customSpec := &openshiftconfigv1.TLSProfileSpec{
			MinTLSVersion: openshiftconfigv1.VersionTLS12,
			Ciphers:       customCiphers,
		}
		minVersion := goTLSVersion(customSpec.MinTLSVersion)
		cipherIDs := tls12CipherIDsFromSpec(customSpec)

		Eventually(func(g Gomega) {
			verifyTLSConfig(g, webhookAddr, minVersion, cipherIDs)
		}).WithTimeout(tlsVerifyTimeout).WithPolling(tlsVerifyPolling).Should(Succeed())

		waitForSSPReady(ctx, k8sCli)
	})

	It("should default to Intermediate when TLS profile is removed", func(ctx context.Context) {
		removePatch := []byte(fmt.Sprintf(`[{"op": "remove", "path": %q}]`, tlsPath))
		tests.PatchHCO(ctx, cli, removePatch)

		Eventually(func(g Gomega) {
			verifyStandardProfile(g, webhookAddr, openshiftconfigv1.TLSProfileIntermediateType)
		}).WithTimeout(tlsVerifyTimeout).WithPolling(tlsVerifyPolling).Should(Succeed())

		waitForSSPReady(ctx, k8sCli)
	})
})
