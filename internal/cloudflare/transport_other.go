//go:build !windows

package cloudflare

import (
	"crypto/tls"
	"crypto/x509"
	"log/slog"
	"net/http"
)

// newTransport returns an http.Transport for non-Windows platforms.
func newTransport() *http.Transport {
	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		slog.Warn("SystemCertPool failed", "err", err)
		pool = x509.NewCertPool()
	}
	return &http.Transport{
		TLSClientConfig: &tls.Config{
			RootCAs: pool,
		},
	}
}
