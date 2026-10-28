package tlssecprofile

import (
	"context"
	"crypto/x509"
	"fmt"
	"sync/atomic"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

// clientCAPool holds the CA pool used to verify metrics client certificates.
// It is safe for concurrent use: the TLS server reads it once per new connection
// while a watch/reconciler swaps it atomically on ConfigMap changes.
type clientCAPool struct {
	pool atomic.Pointer[x509.CertPool]
}

var (
	metricsEndpointCAPool = new(clientCAPool)
)

// Get returns the current pool, or nil if none has been loaded yet.
func (c *clientCAPool) get() *x509.CertPool {
	pool := c.pool.Load()
	if pool == nil {
		return nil
	}

	return pool.Clone()
}

// Set parses the PEM-encoded CA bundle and atomically replaces the pool.
// It returns an error (leaving the previous pool intact) if no certificate can
// be parsed, so a malformed update never drops verification to an empty pool.
func (c *clientCAPool) set(caPEM []byte) error {
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		return fmt.Errorf("no valid certificates found in client CA bundle")
	}
	c.pool.Store(pool)
	return nil
}

var clientCAConfigMapKey = client.ObjectKey{
	Namespace: hcoutil.ClientCAConfigMapNamespace,
	Name:      hcoutil.ClientCAConfigMapName,
}

// RefreshClientCA reads the authoritative metrics client CA from the kube-system
// extension-apiserver-authentication ConfigMap and atomically updates the pool.
// It is used both to seed the pool at startup and to refresh it when a watch
// observes the ConfigMap change (CA rotation). Reads should use an authoritative
// (uncached) reader so a stale cache never narrows the trusted client set.
func RefreshClientCA(ctx context.Context, cli client.Reader, logger logr.Logger) error {
	cm := &corev1.ConfigMap{}

	logger = logger.WithValues("ConfigMap.Namespace", clientCAConfigMapKey.Namespace, "ConfigMap.Name", clientCAConfigMapKey.Name)

	logger.Info("reading the Client CA ConfigMap")
	if err := cli.Get(ctx, clientCAConfigMapKey, cm); err != nil {
		logger.Error(err, "can't get the Client CA ConfigMap")
		return err
	}
	caPEM, ok := cm.Data[hcoutil.ClientCAConfigMapKey]
	if !ok || caPEM == "" {
		logger.Error(nil, "can't find the client CA key in the ConfigMap", "keyName", hcoutil.ClientCAConfigMapKey)
		return fmt.Errorf("can't find the %q key in the ConfigMap %v", hcoutil.ClientCAConfigMapKey, clientCAConfigMapKey)
	}

	logger.Info("the Client CA cache was updated")
	return metricsEndpointCAPool.set([]byte(caPEM))
}
