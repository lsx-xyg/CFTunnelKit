//go:build !windows

package cloudflare

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
)

// newTransport on non-Windows platforms uses the explicit proxyURL if set,
// otherwise env-var proxy. Includes TLS diagnostics.
func newTransport(explicitProxy string) *http.Transport {
	dialer := &net.Dialer{}
	return &http.Transport{
		Proxy: func(req *http.Request) (*url.URL, error) {
			if explicitProxy != "" {
				slog.Debug("using explicit proxy", "addr", explicitProxy)
				return url.Parse(explicitProxy)
			}
			return http.ProxyFromEnvironment(req)
		},
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			conn, err := dialer.DialContext(ctx, network, addr)
			if err != nil {
				slog.Warn("dial failed", "addr", addr, "err", err)
				return nil, err
			}
			slog.Debug("dialed", "addr", addr, "remote", conn.RemoteAddr().String())
			return conn, nil
		},
		TLSClientConfig: &tls.Config{
			VerifyConnection: func(cs tls.ConnectionState) error {
				for i, cert := range cs.PeerCertificates {
					slog.Debug("peer cert", "i", i, "subject", cert.Subject.CommonName, "issuer", cert.Issuer.CommonName, "dns_names", cert.DNSNames)
				}
				return nil
			},
		},
	}
}

func tlsVersionName(v uint16) string {
	switch v {
	case tls.VersionTLS12:
		return "TLS1.2"
	case tls.VersionTLS13:
		return "TLS1.3"
	default:
		return fmt.Sprintf("0x%x", v)
	}
}
