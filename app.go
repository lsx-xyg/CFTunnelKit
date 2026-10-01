package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/lsx-xyg/CFTunnelKit/internal/auth"
	"github.com/lsx-xyg/CFTunnelKit/internal/cloudflare"
	"github.com/lsx-xyg/CFTunnelKit/internal/config"
	"github.com/lsx-xyg/CFTunnelKit/internal/process"
	"github.com/lsx-xyg/CFTunnelKit/internal/service"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"gopkg.in/natefinch/lumberjack.v2"
)

// App is the Wails application root. Its exported methods become the
// frontend bindings under window.go.main.App.*.
type App struct {
	ctx  context.Context
	auth *auth.Service
	pm   process.ProcessManager
	sys  *service.SystemHandler
	win  *service.WindowHandler
	ah   *service.AuthHandler
	th   *service.TunnelHandler
	ih   *service.IngressHandler
	dh   *service.DNSHandler
	ph   *service.ProcessHandler
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
			wailsruntime.EventsEmit(a.ctx, event, payload)
		}
	})
	a.sys = service.NewSystemHandler()
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
	a.win = service.NewWindowHandler(ctx)
	a.ah = service.NewAuthHandler(a.auth, ctx)
	a.th = service.NewTunnelHandler(a.auth, ctx)
	a.ih = service.NewIngressHandler(a.auth, ctx)
	a.dh = service.NewDNSHandler(a.auth, ctx)
	a.ph = service.NewProcessHandler(a.auth, a.pm, ctx)
	a.auth.LoadPersisted()
	go func() {
		time.Sleep(1 * time.Second)
		a.RestoreRunning()
	}()
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
	return a.ah.GetAuthState()
}

// VerifyAndSaveToken validates a user-supplied API token and persists it on
// success.
func (a *App) VerifyAndSaveToken(token string) (cloudflare.TokenInfo, error) {
	return a.ah.VerifyAndSaveToken(token)
}

// RetryVerify re-verifies the persisted token.
func (a *App) RetryVerify() (cloudflare.TokenInfo, error) {
	return a.ah.RetryVerify()
}

// ListTunnels returns the account's tunnels.
func (a *App) ListTunnels() ([]cloudflare.Tunnel, error) {
	return a.th.ListTunnels()
}

// ---- slice 03: cloudflared process bindings ----

// StartTunnel fetches the tunnel run token and launches cloudflared.
func (a *App) StartTunnel(tunnelID string) error {
	if err := a.ph.StartTunnel(tunnelID); err != nil {
		return err
	}
	a.addRunning(tunnelID)
	return nil
}

// StopTunnel gracefully stops one tunnel process.
func (a *App) StopTunnel(tunnelID string) error {
	err := a.ph.StopTunnel(tunnelID)
	if err == nil {
		a.removeRunning(tunnelID)
	}
	return err
}

// StopAllTunnels stops every running tunnel.
func (a *App) StopAllTunnels() error {
	return a.ph.StopAllTunnels()
}

// GetRunStates returns running tunnel IDs.
func (a *App) GetRunStates() map[string]string {
	return a.ph.GetRunStates()
}

// ---- slice 04: tunnel create / delete / detail bindings ----

// CreateTunnel creates a remotely-managed tunnel.
func (a *App) CreateTunnel(name string) (cloudflare.Tunnel, error) {
	return a.th.CreateTunnel(name)
}

// DeleteTunnel deletes a tunnel.
func (a *App) DeleteTunnel(tunnelID string) error {
	return a.th.DeleteTunnel(tunnelID)
}

// GetTunnelDetail returns one tunnel's full record.
func (a *App) GetTunnelDetail(tunnelID string) (cloudflare.TunnelDetail, error) {
	return a.th.GetTunnelDetail(tunnelID)
}

// GetTunnelToken returns the run token for a tunnel.
func (a *App) GetTunnelToken(tunnelID string) (string, error) {
	return a.th.GetTunnelToken(tunnelID)
}

// ---- slice 05: ingress editor bindings ----

// ListZones returns the account's zones.
func (a *App) ListZones() ([]cloudflare.Zone, error) {
	return a.ih.ListZones()
}

// GetIngressConfig returns the tunnel's ingress rules.
func (a *App) GetIngressConfig(tunnelID string) ([]cloudflare.IngressRule, error) {
	return a.ih.GetIngressConfig(tunnelID)
}

// SaveIngressConfig saves ingress rules.
func (a *App) SaveIngressConfig(tunnelID string, rules []cloudflare.IngressRule) ([]cloudflare.IngressRule, error) {
	return a.ih.SaveIngressConfig(tunnelID, rules)
}

// ---- slice 06: DNS link bindings ----

// ListDNSRecords returns the DNS records of a zone.
func (a *App) ListDNSRecords(zoneID string) ([]cloudflare.DNSRecord, error) {
	return a.dh.ListDNSRecords(zoneID)
}

// EnsureCNAME idempotently creates a CNAME name → target.
func (a *App) EnsureCNAME(zoneID, name, target string) (cloudflare.DNSEnsureResult, error) {
	return a.dh.EnsureCNAME(zoneID, name, target)
}

// DeleteDNSByName removes the DNS record matching name in the zone.
func (a *App) DeleteDNSByName(zoneID, name string) (bool, error) {
	return a.dh.DeleteDNSByName(zoneID, name)
}

// LogDir returns the absolute path of the directory containing app.log and
// cloudflared.log, so the user can open it and send logs for debugging.
func (a *App) LogDir() string {
	return a.sys.LogDir()
}

// HideWindow hides the main window (tray behavior)
func (a *App) HideWindow() {
	a.win.HideWindow()
}

// ShowWindow restores the main window from tray
func (a *App) ShowWindow() {
	a.win.ShowWindow()
}

// OpenLogDir opens the logs folder in the system file manager.
func (a *App) OpenLogDir() {
	a.sys.OpenLogDir()
}

// WriteOpLog appends an operation record to operation.log.
func (a *App) WriteOpLog(action, result string) {
	a.sys.WriteOpLog(action, result)
}

// GetVersion returns the build version.
func (a *App) GetVersion() string {
	return a.sys.GetVersion()
}

func (a *App) addRunning(id string) {
	cfg := a.auth.Config()
	for _, x := range cfg.LastRunning {
		if x == id { return }
	}
	cfg.LastRunning = append(cfg.LastRunning, id)
	a.auth.Save(cfg)
}

func (a *App) removeRunning(id string) {
	cfg := a.auth.Config()
	out := cfg.LastRunning[:0]
	for _, x := range cfg.LastRunning {
		if x != id { out = append(out, x) }
	}
	cfg.LastRunning = out
	a.auth.Save(cfg)
}

// RestoreRunning starts tunnels that were running on last exit.
func (a *App) RestoreRunning() {
	cfg := a.auth.Config()
	for _, id := range cfg.LastRunning {
		tok, err := a.auth.GetTunnelToken(a.ctxOrBackground(), id)
		if err != nil { continue }
		_ = a.pm.Start(a.ctxOrBackground(), id, tok)
	}
}
