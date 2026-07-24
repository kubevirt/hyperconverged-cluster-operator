package tests_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"testing"
	"time"

	openshiftconfigv1 "github.com/openshift/api/config/v1"
)

func generateTestCert(t *testing.T) tls.Certificate {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generating key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "tls-test"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1)},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("creating certificate: %v", err)
	}

	return tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  key,
	}
}

func startTestTLSServer(t *testing.T, cipherSuites []uint16, minVersion uint16) (addr string, cleanup func()) {
	t.Helper()

	cert := generateTestCert(t)

	listener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
		Certificates: []tls.Certificate{cert},
		CipherSuites: cipherSuites,
		MinVersion:   minVersion,
	})
	if err != nil {
		t.Fatalf("starting TLS listener: %v", err)
	}

	go func() {
		for {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				if tlsConn, ok := c.(*tls.Conn); ok {
					_ = tlsConn.Handshake()
				}
			}(conn)
		}
	}()

	return listener.Addr().String(), func() { listener.Close() }
}

func TestTLSCipherIDsFromSpec(t *testing.T) {
	t.Run("filters TLS 1.3 ciphers from Intermediate profile", func(t *testing.T) {
		spec := openshiftconfigv1.TLSProfiles[openshiftconfigv1.TLSProfileIntermediateType]
		cipherIDs := tls12CipherIDsFromSpec(spec)

		if len(cipherIDs) == 0 {
			t.Fatal("expected non-empty cipher list for Intermediate profile")
		}

		tls13IDs := map[uint16]bool{
			tls.TLS_AES_128_GCM_SHA256:       true,
			tls.TLS_AES_256_GCM_SHA384:       true,
			tls.TLS_CHACHA20_POLY1305_SHA256: true,
		}
		for _, id := range cipherIDs {
			if tls13IDs[id] {
				t.Errorf("TLS 1.3 cipher 0x%04x should not be in TLS 1.2 cipher list", id)
			}
		}
	})

	t.Run("returns nil for Modern profile", func(t *testing.T) {
		spec := openshiftconfigv1.TLSProfiles[openshiftconfigv1.TLSProfileModernType]
		if ids := tls12CipherIDsFromSpec(spec); ids != nil {
			t.Errorf("expected nil cipher list for Modern profile, got %d ciphers", len(ids))
		}
	})
}

func TestTLSEnumerateServerCiphers(t *testing.T) {
	expected := []uint16{
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
	}

	addr, cleanup := startTestTLSServer(t, expected, tls.VersionTLS12)
	defer cleanup()

	discovered := enumerateServerCiphers(addr, tls.VersionTLS12)
	if len(discovered) == 0 {
		t.Fatal("no ciphers discovered")
	}

	expectedSet := make(map[uint16]bool)
	for _, c := range expected {
		expectedSet[c] = true
	}

	discoveredSet := make(map[uint16]bool)
	for _, c := range discovered {
		if !expectedSet[c] {
			t.Errorf("discovered unexpected cipher: %s (0x%04x)", tls.CipherSuiteName(c), c)
		}
		discoveredSet[c] = true
	}
	for _, c := range expected {
		if !discoveredSet[c] {
			t.Errorf("expected cipher not discovered: %s (0x%04x)", tls.CipherSuiteName(c), c)
		}
	}
}
