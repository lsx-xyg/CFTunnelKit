package service

import (
	"context"

	"github.com/lsx-xyg/CFTunnelKit/internal/auth"
	"github.com/lsx-xyg/CFTunnelKit/internal/cloudflare"
)

// TunnelHandler owns tunnel CRUD bindings.
type TunnelHandler struct {
	svc *auth.Service
	ctx context.Context
}

func NewTunnelHandler(svc *auth.Service, ctx context.Context) *TunnelHandler {
	return &TunnelHandler{svc: svc, ctx: ctx}
}

func (h *TunnelHandler) ctxOrBackground() context.Context {
	if h.ctx != nil {
		return h.ctx
	}
	return context.Background()
}

// ListTunnels returns the account's tunnels.
func (h *TunnelHandler) ListTunnels() ([]cloudflare.Tunnel, error) {
	return h.svc.ListTunnels(h.ctxOrBackground())
}

// CreateTunnel creates a remotely-managed tunnel.
func (h *TunnelHandler) CreateTunnel(name string) (cloudflare.Tunnel, error) {
	return h.svc.CreateTunnel(h.ctxOrBackground(), name)
}

// DeleteTunnel deletes a tunnel.
func (h *TunnelHandler) DeleteTunnel(tunnelID string) error {
	return h.svc.DeleteTunnel(h.ctxOrBackground(), tunnelID)
}

// GetTunnelDetail returns one tunnel's full record.
func (h *TunnelHandler) GetTunnelDetail(tunnelID string) (cloudflare.TunnelDetail, error) {
	return h.svc.GetTunnelDetail(h.ctxOrBackground(), tunnelID)
}

// GetTunnelToken returns the run token for a tunnel.
func (h *TunnelHandler) GetTunnelToken(tunnelID string) (string, error) {
	return h.svc.GetTunnelToken(h.ctxOrBackground(), tunnelID)
}
