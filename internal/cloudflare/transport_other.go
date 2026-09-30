//go:build !windows

package cloudflare

import (
	"log/slog"
	"net/http"
	"net/url"
)

// newTransport on non-Windows platforms uses the explicit proxyURL if set,
// otherwise env-var proxy.
func newTransport(explicitProxy string) *http.Transport {
	return &http.Transport{
		Proxy: func(req *http.Request) (*url.URL, error) {
			if explicitProxy != "" {
				slog.Debug("using explicit proxy", "addr", explicitProxy)
				return url.Parse(explicitProxy)
			}
			return http.ProxyFromEnvironment(req)
		},
	}
}
