package cloudflare

import "context"

// CFClient is the seam between the app and the Cloudflare API. Every HTTP
// call in the app flows through this interface so tests can inject an
// httptest fake server.
//
// This slice (01) deliberately defines only two methods; the rest
// (CreateTunnel / DeleteTunnel / GetToken / GetConfigurations /
// PutConfigurations) are added by later slices as needed.
type CFClient interface {
	// VerifyToken validates the token, resolves the default account and
	// probes the Tunnel:Edit / Zone:Read / DNS:Edit permissions. It returns
	// a TokenInfo on success.
	VerifyToken(ctx context.Context) (TokenInfo, error)
	// ListTunnels lists the tunnels under the given account.
	// An empty accountID returns an AuthError.
	ListTunnels(ctx context.Context, accountID string) ([]Tunnel, error)
}
