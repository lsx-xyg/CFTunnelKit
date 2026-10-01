package cloudflare

import (
	"context"
	"fmt"
	"net/http"

	cf "github.com/cloudflare/cloudflare-go/v7"
	"github.com/cloudflare/cloudflare-go/v7/option"
	"github.com/cloudflare/cloudflare-go/v7/zero_trust"
)

// sdkClient implements CFClient using cloudflare-go v7 SDK.
type sdkClient struct {
	api *cf.Client
}

// NewSDKClient builds a CFClient backed by the official SDK, with our
// custom transport (DNS resolver + embedded CA bundle).
func NewSDKClient(token string) CFClient {
	hc := &http.Client{Transport: newTransport()}
	api := cf.NewClient(
		option.WithAPIToken(token),
		option.WithHTTPClient(hc),
	)
	return &sdkClient{api: api}
}

// --- Tunnel methods ---

func (c *sdkClient) ListTunnels(ctx context.Context, accountID string, page, perPage int) ([]Tunnel, error) {
	if accountID == "" {
		return nil, &APIError{Kind: KindAuth, Message: "账户未解析"}
	}
	resp, err := c.api.ZeroTrust.Tunnel.List(ctx, zero_trust.TunnelListParams{
		AccountID: accountID,
		Page:      int64(page),
		PerPage:   int64(perPage),
	})
	if err != nil {
		return nil, err
	}
	var out []Tunnel
	for _, t := range resp.Result {
		out = append(out, Tunnel{
			ID:   t.ID,
			Name: t.Name,
		})
	}
	return out, nil
}

func (c *sdkClient) CreateTunnel(ctx context.Context, accountID, name string) (Tunnel, error) {
	return Tunnel{}, fmt.Errorf("not yet migrated")
}

func (c *sdkClient) DeleteTunnel(ctx context.Context, accountID, tunnelID string) error {
	return fmt.Errorf("not yet migrated")
}

func (c *sdkClient) GetTunnelDetail(ctx context.Context, accountID, tunnelID string) (TunnelDetail, error) {
	return TunnelDetail{}, fmt.Errorf("not yet migrated")
}

func (c *sdkClient) GetIngressConfig(ctx context.Context, accountID, tunnelID string) ([]IngressRule, error) {
	return nil, fmt.Errorf("not yet migrated")
}

func (c *sdkClient) PutIngressConfig(ctx context.Context, accountID, tunnelID string, rules []IngressRule) error {
	return fmt.Errorf("not yet migrated")
}

// --- DNS methods ---

func (c *sdkClient) ListZones(ctx context.Context, accountID string) ([]Zone, error) {
	return nil, fmt.Errorf("not yet migrated")
}

func (c *sdkClient) ListDNSRecords(ctx context.Context, zoneID string) ([]DNSRecord, error) {
	return nil, fmt.Errorf("not yet migrated")
}

func (c *sdkClient) CreateCNAMERecord(ctx context.Context, zoneID, name, target string) (DNSRecord, error) {
	return DNSRecord{}, fmt.Errorf("not yet migrated")
}

func (c *sdkClient) DeleteDNSRecord(ctx context.Context, zoneID, recordID string) error {
	return fmt.Errorf("not yet migrated")
}

// --- Keep hand-written (SDK has no equivalent) ---

func (c *sdkClient) VerifyToken(ctx context.Context) (TokenInfo, error) {
	return TokenInfo{}, fmt.Errorf("not yet migrated")
}

func (c *sdkClient) GetTunnelToken(ctx context.Context, accountID, tunnelID string) (string, error) {
	return "", fmt.Errorf("not yet migrated")
}
