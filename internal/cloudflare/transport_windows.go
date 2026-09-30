//go:build windows

package cloudflare

import (
	"net/http"
	"net/url"
	"runtime"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// newTransport returns an http.Transport that first tries the Windows
// system proxy (set by Clash/v2rayN "system proxy" mode), then falls back
// to HTTP_PROXY/HTTPS_PROXY env vars. This way the app works the moment
// the user turns on their VPN/proxy without manually setting env vars.
func newTransport() *http.Transport {
	return &http.Transport{
		Proxy: func(req *http.Request) (*url.URL, error) {
			if p := windowsSystemProxy(); p != "" {
				return url.Parse(p)
			}
			return http.ProxyFromEnvironment(req)
		},
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
		return ""
	}
	defer k.Close()
	enable, _, err := k.GetIntegerValue("ProxyEnable")
	if err != nil || enable == 0 {
		return ""
	}
	server, _, err := k.GetStringValue("ProxyServer")
	if err != nil || server == "" {
		return ""
	}
	// ProxyServer is either "host:port" or "http=host:port;https=host:port".
	// Take the https entry if present, else the whole string.
	if strings.HasPrefix(server, "http=") || strings.Contains(server, ";") {
		if i := strings.Index(server, "https="); i >= 0 {
			s := server[i+len("https="):]
			if j := strings.Index(s, ";"); j >= 0 {
				s = s[:j]
			}
			return "http://" + s
		}
		for _, part := range strings.Split(server, ";") {
			if strings.HasPrefix(part, "http=") {
				return "http://" + strings.TrimPrefix(part, "http=")
			}
		}
		return ""
	}
	return "http://" + server
}
