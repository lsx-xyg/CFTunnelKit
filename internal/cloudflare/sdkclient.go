package cloudflare

import (
	"net/http"

	cf "github.com/cloudflare/cloudflare-go/v2"
	"github.com/cloudflare/cloudflare-go/v2/option"
)

// NewSDKClient returns a Cloudflare SDK client wired with our custom
// transport (DNS resolver + embedded CA bundle). Not yet used by the app;
// this is the starting point for issue #38 migration.
func NewSDKClient(token string) *cf.Client {
	hc := &http.Client{Transport: newTransport()}
	return cf.NewClient(
		option.WithAPIToken(token),
		option.WithHTTPClient(hc),
	)
}
