package service

import (
	"context"

	"github.com/lsx-xyg/CFTunnelKit/internal/auth"
	"github.com/lsx-xyg/CFTunnelKit/internal/cloudflare"
)

// IngressHandler owns ingress and zone bindings.
type IngressHandler struct {
	svc *auth.Service
	ctx context.Context
}

func NewIngressHandler(svc *auth.Service, ctx context.Context) *IngressHandler {
	return &IngressHandler{svc: svc, ctx: ctx}
}

func (h *IngressHandler) ctxOrBackground() context.Context {
	if h.ctx != nil {
		return h.ctx
	}
	return context.Background()
}

// ListZones returns the account's zones.
func (h *IngressHandler) ListZones() ([]cloudflare.Zone, error) {
	return h.svc.ListZones(h.ctxOrBackground())
}

// GetIngressConfig returns the tunnel's ingress rules.
func (h *IngressHandler) GetIngressConfig(tunnelID string) ([]cloudflare.IngressRule, error) {
	return h.svc.GetIngressConfig(h.ctxOrBackground(), tunnelID)
}

// SaveIngressConfig saves ingress rules.
func (h *IngressHandler) SaveIngressConfig(tunnelID string, rules []cloudflare.IngressRule) ([]cloudflare.IngressRule, error) {
	return h.svc.SaveIngressConfig(h.ctxOrBackground(), tunnelID, rules)
}
