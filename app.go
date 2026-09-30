package main

import (
	"context"

	"github.com/lsx-xyg/CFTunnelKit/internal/auth"
	"github.com/lsx-xyg/CFTunnelKit/internal/cloudflare"
	"github.com/lsx-xyg/CFTunnelKit/internal/config"
)

// App is the Wails application root. Its exported methods become the
// frontend bindings under window.go.main.App.*.
type App struct {
	ctx  context.Context
	auth *auth.Service
}

// NewApp creates the App with a config store at the default location.
func NewApp() *App {
	path, err := config.DefaultPath()
	if err != nil {
		// Practically unreachable (home dir always exists); fail loudly.
		panic(err)
	}
	return &App{
		auth: auth.NewService(config.NewStore(path)),
	}
}

// startup is called by Wails when the app boots. Only the persisted state is
// loaded synchronously; network verification is triggered by the frontend
// (RetryVerify) so the first GetAuthState is deterministic.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.auth.LoadPersisted()
}

func (a *App) ctxOrBackground() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}

// ---- Wails bindings ----

// GetAuthState returns the current authentication state for the frontend.
func (a *App) GetAuthState() auth.State {
	return a.auth.GetState()
}

// VerifyAndSaveToken validates a user-supplied API token and persists it on
// success. Errors are user-facing messages ("Token 无效或已失效" etc.).
func (a *App) VerifyAndSaveToken(token string) (cloudflare.TokenInfo, error) {
	return a.auth.VerifyAndSaveToken(a.ctxOrBackground(), token)
}

// RetryVerify re-verifies the persisted token (offline retry button and
// startup restore).
func (a *App) RetryVerify() (cloudflare.TokenInfo, error) {
	return a.auth.RetryVerify(a.ctxOrBackground())
}
