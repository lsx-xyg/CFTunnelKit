package service

import (
	"context"

	"github.com/lsx-xyg/CFTunnelKit/internal/auth"
	"github.com/lsx-xyg/CFTunnelKit/internal/cloudflare"
)

// AuthHandler owns authentication bindings.
type AuthHandler struct {
	svc *auth.Service
	ctx context.Context
}

func NewAuthHandler(svc *auth.Service, ctx context.Context) *AuthHandler {
	return &AuthHandler{svc: svc, ctx: ctx}
}

func (h *AuthHandler) ctxOrBackground() context.Context {
	if h.ctx != nil {
		return h.ctx
	}
	return context.Background()
}

// GetAuthState returns the current authentication state.
func (h *AuthHandler) GetAuthState() auth.State {
	return h.svc.GetState()
}

// VerifyAndSaveToken validates a user-supplied API token and persists it.
func (h *AuthHandler) VerifyAndSaveToken(token string) (cloudflare.TokenInfo, error) {
	return h.svc.VerifyAndSaveToken(h.ctxOrBackground(), token)
}

// RetryVerify re-verifies the persisted token.
func (h *AuthHandler) RetryVerify() (cloudflare.TokenInfo, error) {
	return h.svc.RetryVerify(h.ctxOrBackground())
}
