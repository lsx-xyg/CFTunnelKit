package service

import (
	"context"

	"github.com/lsx-xyg/CFTunnelKit/internal/auth"
	"github.com/lsx-xyg/CFTunnelKit/internal/process"
)

// ProcessHandler owns cloudflared process lifecycle bindings.
type ProcessHandler struct {
	svc *auth.Service
	pm  process.ProcessManager
	ctx context.Context
}

func NewProcessHandler(svc *auth.Service, pm process.ProcessManager, ctx context.Context) *ProcessHandler {
	return &ProcessHandler{svc: svc, pm: pm, ctx: ctx}
}

func (h *ProcessHandler) ctxOrBackground() context.Context {
	if h.ctx != nil {
		return h.ctx
	}
	return context.Background()
}

// StartTunnel fetches the run token and launches cloudflared.
func (h *ProcessHandler) StartTunnel(tunnelID string) error {
	tok, err := h.svc.GetTunnelToken(h.ctxOrBackground(), tunnelID)
	if err != nil {
		return err
	}
	return h.pm.Start(h.ctxOrBackground(), tunnelID, tok)
}

// StopTunnel gracefully stops one tunnel.
func (h *ProcessHandler) StopTunnel(tunnelID string) error {
	return h.pm.Stop(tunnelID)
}

// StopAllTunnels stops every running tunnel.
func (h *ProcessHandler) StopAllTunnels() error {
	return h.pm.StopAll()
}

// GetRunStates returns running tunnel IDs.
func (h *ProcessHandler) GetRunStates() map[string]string {
	return h.pm.RunStates()
}
