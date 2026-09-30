//go:build !windows

package cloudflare

import "net/http"

// newTransport on non-Windows platforms uses the default env-var proxy.
func newTransport() *http.Transport {
	return &http.Transport{Proxy: http.ProxyFromEnvironment}
}
