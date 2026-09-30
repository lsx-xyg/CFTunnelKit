//go:build windows

package cloudflare

import (
	"crypto/tls"
	"crypto/x509"
	"log/slog"
	"net/http"
)

// newTransport returns an http.Transport for Windows. It tries to load the
// system root cert pool. If that yields zero certs (common when CGO is
// disabled, e.g. wails dev), we leave RootCAs nil so Go falls back to its
// built-in Mozilla root bundle instead of trusting nothing.
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
	slog.Debug("system cert pool loaded", "certs", len(pool.Subjects()))
	return &http.Transport{
		TLSClientConfig: &tls.Config{
			RootCAs: pool,
		},
	}
}
