package service

import (
	"context"

	"github.com/lsx-xyg/CFTunnelKit/internal/auth"
	"github.com/lsx-xyg/CFTunnelKit/internal/cloudflare"
)

// DNSHandler owns DNS record bindings.
type DNSHandler struct {
	svc *auth.Service
	ctx context.Context
}

func NewDNSHandler(svc *auth.Service, ctx context.Context) *DNSHandler {
	return &DNSHandler{svc: svc, ctx: ctx}
}

func (h *DNSHandler) ctxOrBackground() context.Context {
	if h.ctx != nil {
		return h.ctx
	}
	return context.Background()
}

// ListDNSRecords returns the DNS records of a zone.
func (h *DNSHandler) ListDNSRecords(zoneID string) ([]cloudflare.DNSRecord, error) {
	return h.svc.ListDNSRecords(h.ctxOrBackground(), zoneID)
}

// EnsureCNAME idempotently creates a CNAME.
func (h *DNSHandler) EnsureCNAME(zoneID, name, target string) (cloudflare.DNSEnsureResult, error) {
	return h.svc.EnsureCNAME(h.ctxOrBackground(), zoneID, name, target)
}

// DeleteDNSByName removes the DNS record matching name in the zone.
func (h *DNSHandler) DeleteDNSByName(zoneID, name string) (bool, error) {
	return h.svc.DeleteDNSByName(h.ctxOrBackground(), zoneID, name)
}
