package cloudflare

import (
	"context"
	"net/http"

	cf "github.com/cloudflare/cloudflare-go/v7"
	"github.com/cloudflare/cloudflare-go/v7/option"
	"github.com/cloudflare/cloudflare-go/v7/zero_trust"
)

// sdkClient implements CFClient using cloudflare-go v7 SDK where available,
// falling back to the hand-written HTTP client for endpoints the SDK doesn't
// cover well.
type sdkClient struct {
	sdk      *cf.Client
	fallback CFClient
	token    string
}

func NewSDKClient(token string, fallback CFClient) CFClient {
	hc := &http.Client{Transport: newTransport()}
	api := cf.NewClient(
		option.WithAPIToken(token),
		option.WithHTTPClient(hc),
	)
	return &sdkClient{sdk: api, fallback: fallback, token: token}
}

// --- Tunnel CRUD (SDK) ---

func (c *sdkClient) ListTunnels(ctx context.Context, accountID string, page, perPage int) ([]Tunnel, error) {
	resp, err := c.sdk.ZeroTrust.Tunnels.Cloudflared.List(ctx, zero_trust.TunnelCloudflaredListParams{
		AccountID: cf.F(accountID),
		Page:      cf.F(float64(page)),
		PerPage:   cf.F(float64(perPage)),
	})
	if err != nil {
		return nil, err
	}
	var out []Tunnel
	for _, t := range resp.Result {
		out = append(out, Tunnel{ID: t.ID, Name: t.Name})
	}
	return out, nil
}

func (c *sdkClient) CreateTunnel(ctx context.Context, accountID, name string) (Tunnel, error) {
	t, err := c.sdk.ZeroTrust.Tunnels.Cloudflared.New(ctx, zero_trust.TunnelCloudflaredNewParams{
		AccountID: cf.F(accountID),
		Name:      cf.F(name),
	})
	if err != nil {
		return Tunnel{}, err
	}
	return Tunnel{ID: t.ID, Name: t.Name}, nil
}

func (c *sdkClient) DeleteTunnel(ctx context.Context, accountID, tunnelID string) error {
	_, err := c.sdk.ZeroTrust.Tunnels.Cloudflared.Delete(ctx, tunnelID, zero_trust.TunnelCloudflaredDeleteParams{
		AccountID: cf.F(accountID),
	})
	return err
}

func (c *sdkClient) GetTunnelDetail(ctx context.Context, accountID, tunnelID string) (TunnelDetail, error) {
	t, err := c.sdk.ZeroTrust.Tunnels.Cloudflared.Get(ctx, tunnelID, zero_trust.TunnelCloudflaredGetParams{
		AccountID: cf.F(accountID),
	})
	if err != nil {
		return TunnelDetail{}, err
	}
	return TunnelDetail{ID: t.ID, Name: t.Name}, nil
}

// --- Ingress config (SDK) ---

func (c *sdkClient) GetIngressConfig(ctx context.Context, accountID, tunnelID string) ([]IngressRule, error) {
	svc := zero_trust.NewTunnelCloudflaredConfigurationService(
		option.WithBaseURL("https://api.cloudflare.com/client/v4"),
		option.WithHTTPClient(&http.Client{Transport: newTransport()}),
		option.WithAPIToken(c.token),
	)
	resp, err := svc.Get(ctx, tunnelID, zero_trust.TunnelCloudflaredConfigurationGetParams{
		AccountID: cf.F(accountID),
	})
	if err != nil {
		return nil, err
	}
	var out []IngressRule
	for _, ing := range resp.Config.Ingress {
		if ing.Service == "http_status:404" || ing.Hostname == "" {
			continue
		}
		out = append(out, IngressRule{Hostname: ing.Hostname, Service: ing.Service})
	}
	return out, nil
}

func (c *sdkClient) PutIngressConfig(ctx context.Context, accountID, tunnelID string, rules []IngressRule) error {
	var ingress []zero_trust.TunnelCloudflaredConfigurationUpdateParamsConfigIngress
	for _, r := range rules {
		ingress = append(ingress, zero_trust.TunnelCloudflaredConfigurationUpdateParamsConfigIngress{
			Hostname: cf.F(r.Hostname),
			Service:  cf.F(r.Service),
		})
	}
	svc := zero_trust.NewTunnelCloudflaredConfigurationService(
		option.WithBaseURL("https://api.cloudflare.com/client/v4"),
		option.WithHTTPClient(&http.Client{Transport: newTransport()}),
		option.WithAPIToken(c.token),
	)
	_, err := svc.Update(ctx, tunnelID, zero_trust.TunnelCloudflaredConfigurationUpdateParams{
		AccountID: cf.F(accountID),
		Config: cf.F(zero_trust.TunnelCloudflaredConfigurationUpdateParamsConfig{
			Ingress: cf.F(ingress),
		}),
	})
	return err
}

// --- Fallback to hand-written ---

func (c *sdkClient) VerifyToken(ctx context.Context) (TokenInfo, error) {
	return c.fallback.VerifyToken(ctx)
}

func (c *sdkClient) GetTunnelToken(ctx context.Context, accountID, tunnelID string) (string, error) {
	return c.fallback.GetTunnelToken(ctx, accountID, tunnelID)
}

func (c *sdkClient) ListZones(ctx context.Context, accountID string) ([]Zone, error) {
	return c.fallback.ListZones(ctx, accountID)
}

func (c *sdkClient) ListDNSRecords(ctx context.Context, zoneID string) ([]DNSRecord, error) {
	return c.fallback.ListDNSRecords(ctx, zoneID)
}

func (c *sdkClient) CreateCNAMERecord(ctx context.Context, zoneID, name, target string) (DNSRecord, error) {
	return c.fallback.CreateCNAMERecord(ctx, zoneID, name, target)
}

func (c *sdkClient) DeleteDNSRecord(ctx context.Context, zoneID, recordID string) error {
	return c.fallback.DeleteDNSRecord(ctx, zoneID, recordID)
}
