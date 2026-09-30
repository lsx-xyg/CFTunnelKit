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
		slog.Warn("SystemCertPool failed, using built-in roots", "err", err)
		return &http.Transport{}
	}
	if n := len(pool.Subjects()); n == 0 {
		slog.Warn("SystemCertPool returned 0 certs, using built-in roots")
		return &http.Transport{}
	}
	return &http.Transport{
		TLSClientConfig: &tls.Config{
			RootCAs: pool,
		},
	}
}
