package cloudflare

import "context"

// CFClient is the seam between the app and the Cloudflare API. Every HTTP
// call in the app flows through this interface so tests can inject an
// httptest fake server.
//
// Slice 01 defined VerifyToken and ListTunnels; slice 02 extended
// ListTunnels with page/perPage; slice 03 added GetTunnelToken. The rest
// (CreateTunnel / DeleteTunnel / GetConfigurations / PutConfigurations)
// are added by later slices as needed.
type CFClient interface {
	// VerifyToken validates the token, resolves the default account and
	// probes the Tunnel:Edit / Zone:Read / DNS:Edit permissions. It returns
	// a TokenInfo on success.
	VerifyToken(ctx context.Context) (TokenInfo, error)
	// ListTunnels lists the tunnels under the given account, using the
	// page/perPage query parameters (the UI currently pins page=1,
	// perPage=50). An empty accountID returns an AuthError.
	ListTunnels(ctx context.Context, accountID string, page, perPage int) ([]Tunnel, error)
	// GetTunnelToken returns the run token for a tunnel
	// (GET /accounts/{id}/cfd_tunnel/{tunnel_id}/token → result.token).
	// 403 maps to a permission error, 404 to an API error.
	GetTunnelToken(ctx context.Context, accountID, tunnelID string) (string, error)
	// CreateTunnel creates a remotely-managed tunnel
	// (POST body {name, config_src: "cloudflare"}). 409 maps to
	// "同名 Tunnel 已存在", 403 to a permission error.
	CreateTunnel(ctx context.Context, accountID, name string) (Tunnel, error)
	// DeleteTunnel deletes a tunnel. Deleting a tunnel with active
	// connections yields "该 Tunnel 有活跃连接，请先停止隧道".
	DeleteTunnel(ctx context.Context, accountID, tunnelID string) error
	// GetTunnelDetail returns the full record for one tunnel, including
	// the number of active connections.
	GetTunnelDetail(ctx context.Context, accountID, tunnelID string) (TunnelDetail, error)
	// GetIngressConfig returns the ingress rules of a tunnel, with the
	// catch-all 404 rule stripped (issue #5). A never-configured tunnel
	// (config: null) yields an empty list.
	GetIngressConfig(ctx context.Context, accountID, tunnelID string) ([]IngressRule, error)
	// PutIngressConfig pushes ingress rules. The catch-all 404 rule is
	// appended automatically; the body is wrapped as {"config":{"ingress":…}}.
	PutIngressConfig(ctx context.Context, accountID, tunnelID string, rules []IngressRule) error
	// ListZones returns the zones of an account (issue #5: hostname
	// root-domain validation).
	ListZones(ctx context.Context, accountID string) ([]Zone, error)
}
