package cloudflare

import (
	"context"
	"fmt"
	"net/http"

	cf "github.com/cloudflare/cloudflare-go/v7"
	"github.com/cloudflare/cloudflare-go/v7/option"
	"github.com/cloudflare/cloudflare-go/v7/zero_trust"
)

type sdkClient struct {
	api *cf.Client
}

func NewSDKClient(token string) CFClient {
	hc := &http.Client{Transport: newTransport()}
	api := cf.NewClient(
		option.WithAPIToken(token),
		option.WithHTTPClient(hc),
	)
	return &sdkClient{api: api}
}

// --- Tunnel CRUD ---

func (c *sdkClient) ListTunnels(ctx context.Context, accountID string, page, perPage int) ([]Tunnel, error) {
	if accountID == "" {
		return nil, &APIError{Kind: KindAuth, Message: "账户未解析"}
	}
	resp, err := c.api.ZeroTrust.Tunnels.Cloudflared.List(ctx, zero_trust.TunnelCloudflaredListParams{
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
	t, err := c.api.ZeroTrust.Tunnels.Cloudflared.New(ctx, zero_trust.TunnelCloudflaredNewParams{
		AccountID: cf.F(accountID),
		Name:      cf.F(name),
	})
	if err != nil {
		return Tunnel{}, err
	}
	return Tunnel{ID: t.ID, Name: t.Name}, nil
}

func (c *sdkClient) DeleteTunnel(ctx context.Context, accountID, tunnelID string) error {
	_, err := c.api.ZeroTrust.Tunnels.Cloudflared.Delete(ctx, tunnelID, zero_trust.TunnelCloudflaredDeleteParams{
		AccountID: cf.F(accountID),
	})
	return err
}

func (c *sdkClient) GetTunnelDetail(ctx context.Context, accountID, tunnelID string) (TunnelDetail, error) {
	t, err := c.api.ZeroTrust.Tunnels.Cloudflared.Get(ctx, tunnelID, zero_trust.TunnelCloudflaredGetParams{
		AccountID: cf.F(accountID),
	})
	if err != nil {
		return TunnelDetail{}, err
	}
	return TunnelDetail{ID: t.ID, Name: t.Name}, nil
}

// --- Ingress config ---

func (c *sdkClient) GetIngressConfig(ctx context.Context, accountID, tunnelID string) ([]IngressRule, error) {
	return nil, fmt.Errorf("not migrated")
}

func (c *sdkClient) PutIngressConfig(ctx context.Context, accountID, tunnelID string, rules []IngressRule) error {
	return fmt.Errorf("not migrated")
}

// --- DNS ---

func (c *sdkClient) ListZones(ctx context.Context, accountID string) ([]Zone, error) {
	return nil, fmt.Errorf("not migrated")
}

func (c *sdkClient) ListDNSRecords(ctx context.Context, zoneID string) ([]DNSRecord, error) {
	return nil, fmt.Errorf("not migrated")
}

func (c *sdkClient) CreateCNAMERecord(ctx context.Context, zoneID, name, target string) (DNSRecord, error) {
	return DNSRecord{}, fmt.Errorf("not migrated")
}

func (c *sdkClient) DeleteDNSRecord(ctx context.Context, zoneID, recordID string) error {
	return fmt.Errorf("not migrated")
}

// --- Keep hand-written ---

func (c *sdkClient) VerifyToken(ctx context.Context) (TokenInfo, error) {
	return TokenInfo{}, fmt.Errorf("not migrated")
}

func (c *sdkClient) GetTunnelToken(ctx context.Context, accountID, tunnelID string) (string, error) {
	return "", fmt.Errorf("not migrated")
}
