package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lsx-xyg/CFTunnelKit/internal/applog"
	"github.com/lsx-xyg/CFTunnelKit/internal/auth"
	"github.com/lsx-xyg/CFTunnelKit/internal/cloudflare"
	"github.com/lsx-xyg/CFTunnelKit/internal/config"
	"github.com/lsx-xyg/CFTunnelKit/internal/process"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"gopkg.in/natefinch/lumberjack.v2"
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
	if err := a.initRollingLog(); err != nil {
		// Non-fatal: log persistence is best-effort (slice 07b).
		fmt.Printf("warning: rolling log unavailable: %v\n", err)
	}
	return a
}

// initRollingLog wires the ~/.cftunnelkit/logs/cloudflared.log roller into
// the process manager (issue #9: MaxSize 1MB, MaxBackups 3, no compress).
func (a *App) initRollingLog() error {
	logPath, err := config.DefaultLogPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return err
	}
	if pm, ok := a.pm.(*process.Manager); ok {
		pm.SetLogWriter(&lumberjack.Logger{
			Filename:   logPath,
			MaxSize:    1, // MB
			MaxBackups: 3,
			MaxAge:     0,
			Compress:   false,
		})
	}
	return nil
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

// Quit exits the app completely (not just hiding the window).
func (a *App) Quit() {
	if a.ctx != nil {
		runtime.Quit(a.ctx)
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

// ---- slice 04: tunnel create / delete / detail bindings ----

// CreateTunnel creates a remotely-managed tunnel (issue #6). The name is
// validated frontend-side; 409 surfaces as "同名 Tunnel 已存在".
func (a *App) CreateTunnel(name string) (cloudflare.Tunnel, error) {
	return a.auth.CreateTunnel(a.ctxOrBackground(), name)
}

// DeleteTunnel deletes a tunnel. Active-connection errors surface as
// "该 Tunnel 有活跃连接，请先停止隧道".
func (a *App) DeleteTunnel(tunnelID string) error {
	return a.auth.DeleteTunnel(a.ctxOrBackground(), tunnelID)
}

// GetTunnelDetail returns one tunnel's full record (metadata + connection
// count) for the detail dialog.
func (a *App) GetTunnelDetail(tunnelID string) (cloudflare.TunnelDetail, error) {
	return a.auth.GetTunnelDetail(a.ctxOrBackground(), tunnelID)
}

// GetTunnelToken returns the run token for a tunnel (create/detail dialogs,
// issue #6). Token failures never block create.
func (a *App) GetTunnelToken(tunnelID string) (string, error) {
	return a.auth.GetTunnelToken(a.ctxOrBackground(), tunnelID)
}

// ---- slice 05: ingress editor bindings ----

// ListZones returns the account's zones (hostname root-domain validation).
func (a *App) ListZones() ([]cloudflare.Zone, error) {
	return a.auth.ListZones(a.ctxOrBackground())
}

// GetIngressConfig returns the tunnel's ingress rules (catch-all stripped).
func (a *App) GetIngressConfig(tunnelID string) ([]cloudflare.IngressRule, error) {
	return a.auth.GetIngressConfig(a.ctxOrBackground(), tunnelID)
}

// SaveIngressConfig runs the issue #5 save flow (PUT + read-back compare)
// and returns the read-back rules on success.
func (a *App) SaveIngressConfig(tunnelID string, rules []cloudflare.IngressRule) ([]cloudflare.IngressRule, error) {
	return a.auth.SaveIngressConfig(a.ctxOrBackground(), tunnelID, rules)
}

// ---- slice 06: DNS link bindings ----

// ListDNSRecords returns the DNS records of a zone (issue #6).
func (a *App) ListDNSRecords(zoneID string) ([]cloudflare.DNSRecord, error) {
	return a.auth.ListDNSRecords(a.ctxOrBackground(), zoneID)
}

// EnsureCNAME idempotently creates a CNAME name → target (issue #6).
// Conflicts surface as "域名 xxx 已被占用，请手动处理" and never overwrite.
func (a *App) EnsureCNAME(zoneID, name, target string) (cloudflare.DNSEnsureResult, error) {
	return a.auth.EnsureCNAME(a.ctxOrBackground(), zoneID, name, target)
}

// DeleteDNSByName removes the DNS record matching name in the zone
// (issue #6 delete link). Missing records are an idempotent success.
func (a *App) DeleteDNSByName(zoneID, name string) (bool, error) {
	return a.auth.DeleteDNSByName(a.ctxOrBackground(), zoneID, name)
}

// LogDir returns the absolute path of the directory containing app.log and
// cloudflared.log, so the user can open it and send logs for debugging.
func (a *App) LogDir() string {
	return applog.Dir()
}

// OpenLogDir opens the logs folder in the system file manager.
func (a *App) OpenLogDir() {
	d := applog.Dir()
	_ = os.MkdirAll(d, 0o700)
	runtime.BrowserOpenURL(a.ctx, "file://"+filepath.ToSlash(d))
}
