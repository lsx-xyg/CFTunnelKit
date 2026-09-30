//go:build windows

package cloudflare

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	_ "embed"
	"log/slog"
	"net"
	"net/http"
	"time"
)

//go:embed ca-bundle.crt
var embeddedCABundle []byte

func newTransport() *http.Transport {
	// Custom resolver: bypass router DNS hijacking by forcing Alibaba DNS.
	dnsServers := []string{"223.5.5.5:53", "223.6.6.6:53"}
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{Timeout: 3 * time.Second}
			var lastErr error
			for _, srv := range dnsServers {
				conn, err := d.DialContext(ctx, "udp", srv)
				if err == nil {
					return conn, nil
				}
				lastErr = err
			}
			return nil, lastErr
		},
	}
	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
		Resolver:  resolver,
	}
	dialContext := func(ctx context.Context, network, addr string) (net.Conn, error) {
		return dialer.DialContext(ctx, "tcp4", addr)
	}

	// Use embedded Mozilla CA bundle — system cert pool is often empty on
	// wails dev (CGO disabled) or trimmed Windows installs.
	pool := x509.NewCertPool()
	if ok := pool.AppendCertsFromPEM(embeddedCABundle); !ok {
		slog.Error("embedded CA bundle parse failed")
	} else {
		slog.Debug("embedded CA bundle loaded", "certs", len(pool.Subjects()))
	}

	return &http.Transport{
		DialContext: dialContext,
		TLSClientConfig: &tls.Config{
			RootCAs: pool,
		},
	}
}
