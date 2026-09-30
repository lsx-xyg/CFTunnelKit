package cloudflare

import "context"

// CFClient is the seam between the app and the Cloudflare API. Every HTTP
// call in the app flows through this interface so tests can inject an
// httptest fake server.
//
// Slice 01 defined VerifyToken and ListTunnels; slice 02 extended
// ListTunnels with page/perPage. The rest (CreateTunnel / DeleteTunnel /
// GetToken / GetConfigurations / PutConfigurations) are added by later
// slices as needed.
type CFClient interface {
	// VerifyToken validates the token, resolves the default account and
	// probes the Tunnel:Edit / Zone:Read / DNS:Edit permissions. It returns
	// a TokenInfo on success.
	VerifyToken(ctx context.Context) (TokenInfo, error)
	// ListTunnels lists the tunnels under the given account, using the
	// page/perPage query parameters (the UI currently pins page=1,
	// perPage=50). An empty accountID returns an AuthError.
	ListTunnels(ctx context.Context, accountID string, page, perPage int) ([]Tunnel, error)
}
