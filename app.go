package main

import (
	"context"

	"github.com/lsx-xyg/CFTunnelKit/internal/auth"
	"github.com/lsx-xyg/CFTunnelKit/internal/cloudflare"
	"github.com/lsx-xyg/CFTunnelKit/internal/config"
	"github.com/lsx-xyg/CFTunnelKit/internal/process"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the Wails application root. Its exported methods become the
// frontend bindings under window.go.main.App.*.
type App struct {
	ctx  context.Context
	auth *auth.Service
	pm   process.ProcessManager
}

// NewApp creates the App with a config store and the process manager at the
// default locations. Events emitted by the manager are forwarded to the
// frontend via Wails Events (cloudflared:log / cloudflared:status /
// cloudflared:download).
func NewApp() *App {
	path, err := config.DefaultPath()
	if err != nil {
		// Practically unreachable (home dir always exists); fail loudly.
		panic(err)
	}
	binDir, err := config.DefaultBinDir()
	if err != nil {
		panic(err)
	}
	a := &App{auth: auth.NewService(config.NewStore(path))}
	a.pm = process.NewManager(binDir, func(event string, payload interface{}) {
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, event, payload)
		}
	})
	return a
}

// startup is called by Wails when the app boots. Only the persisted state is
// loaded synchronously; network verification is triggered by the frontend
// (RetryVerify) so the first GetAuthState is deterministic.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.auth.LoadPersisted()
}

// shutdown is wired to Wails OnShutdown: every running cloudflared process
// is stopped gracefully before the app exits (issue #5).
func (a *App) shutdown(ctx context.Context) {
	if a.pm != nil {
		_ = a.pm.StopAll()
	}
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

// ListTunnels returns the account's tunnels (page=1, per_page=50).
// An auth failure clears the persisted config and resets the state, so the
// frontend can detect the session expiry via GetAuthState and jump to the
// auth page; permission/network errors keep the state and are rendered
// inline with a retry button.
func (a *App) ListTunnels() ([]cloudflare.Tunnel, error) {
	return a.auth.ListTunnels(a.ctxOrBackground())
}

// ---- slice 03: cloudflared process bindings ----

// StartTunnel fetches the tunnel run token and launches cloudflared for it.
// The binary is auto-downloaded on first use (progress via
// cloudflared:download events). A duplicate start returns
// "该 Tunnel 已在运行".
func (a *App) StartTunnel(tunnelID string) error {
	tok, err := a.auth.GetTunnelToken(a.ctxOrBackground(), tunnelID)
	if err != nil {
		return err
	}
	return a.pm.Start(a.ctxOrBackground(), tunnelID, tok)
}

// StopTunnel gracefully stops one tunnel process.
func (a *App) StopTunnel(tunnelID string) error {
	return a.pm.Stop(tunnelID)
}

// StopAllTunnels stops every running tunnel (frontend "全部停止" / shutdown).
func (a *App) StopAllTunnels() error {
	return a.pm.StopAll()
}

// GetRunStates returns a snapshot of currently running tunnel IDs
// (tunnelID → "running").
func (a *App) GetRunStates() map[string]string {
	return a.pm.RunStates()
}
