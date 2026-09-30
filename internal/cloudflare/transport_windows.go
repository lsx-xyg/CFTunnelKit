//go:build windows

package cloudflare

import (
	"crypto/tls"
	"crypto/x509"
	"log/slog"
	"net/http"
)

// newTransport returns an http.Transport for Windows. It explicitly loads
// the system root cert pool — cross-compiled Windows binaries sometimes
// fail to pick up newly-issued CA certs (e.g. Google Trust Services) from
// the default pool, causing spurious x509 name-mismatch errors.
func newTransport() *http.Transport {
	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		slog.Warn("SystemCertPool failed, falling back to empty", "err", err)
		pool = x509.NewCertPool()
	} else {
		slog.Debug("system cert pool loaded", "certs", len(pool.Subjects()))
	}
	return &http.Transport{
		TLSClientConfig: &tls.Config{
			RootCAs: pool,
		},
	}
}
