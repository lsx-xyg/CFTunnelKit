//go:build windows

package cloudflare

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"runtime"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// newTransport returns an http.Transport that uses the explicit proxyURL if
// provided, otherwise tries the Windows system proxy, then falls back to
// env vars. It also logs the remote IP and peer cert for TLS debugging.
func newTransport(explicitProxy string) *http.Transport {
	dialer := &net.Dialer{}
	return &http.Transport{
		Proxy: func(req *http.Request) (*url.URL, error) {
			if explicitProxy != "" {
				slog.Debug("using explicit proxy", "addr", explicitProxy)
				return url.Parse(explicitProxy)
			}
			if p := windowsSystemProxy(); p != "" {
				return url.Parse(p)
			}
			return http.ProxyFromEnvironment(req)
		},
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			conn, err := dialer.DialContext(ctx, network, addr)
			if err != nil {
				slog.Warn("dial failed", "addr", addr, "err", err)
				return nil, err
			}
			slog.Debug("dialed", "addr", addr, "local", conn.LocalAddr().String(), "remote", conn.RemoteAddr().String())
			return conn, nil
		},
		TLSClientConfig: &tls.Config{
			VerifyConnection: func(cs tls.ConnectionState) error {
				slog.Debug("TLS handshake",
					"server", cs.ServerName,
					"version", tlsVersionName(cs.Version),
					"cipher", tls.CipherSuiteName(cs.CipherSuite),
					"peer_certs", len(cs.PeerCertificates),
				)
				for i, cert := range cs.PeerCertificates {
					slog.Debug("peer cert",
						"i", i,
						"subject", cert.Subject.CommonName,
						"issuer", cert.Issuer.CommonName,
						"dns_names", cert.DNSNames,
					)
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

func windowsSystemProxy() string {
	if runtime.GOOS != "windows" {
		return ""
	}
	k, err := registry.OpenKey(registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Internet Settings`,
		registry.QUERY_VALUE)
	if err != nil {
		slog.Debug("system proxy: cannot open registry", "err", err)
		return ""
	}
	defer k.Close()
	enable, _, err := k.GetIntegerValue("ProxyEnable")
	if err != nil || enable == 0 {
		slog.Debug("system proxy: disabled")
		return ""
	}
	server, _, err := k.GetStringValue("ProxyServer")
	if err != nil || server == "" {
		slog.Debug("system proxy: ProxyServer empty")
		return ""
	}
	// ProxyServer is either "host:port" or "http=host:port;https=host:port".
	// Take the https entry if present, else the whole string.
	var result string
	if strings.HasPrefix(server, "http=") || strings.Contains(server, ";") {
		if i := strings.Index(server, "https="); i >= 0 {
			s := server[i+len("https="):]
			if j := strings.Index(s, ";"); j >= 0 {
				s = s[:j]
			}
			result = "http://" + s
		} else {
			for _, part := range strings.Split(server, ";") {
				if strings.HasPrefix(part, "http=") {
					result = "http://" + strings.TrimPrefix(part, "http=")
					break
				}
			}
		}
	} else {
		result = "http://" + server
	}
	slog.Info("system proxy detected", "addr", result, "raw", server)
	return result
}
