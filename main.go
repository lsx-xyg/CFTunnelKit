package main

import (
	"embed"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/energye/systray"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"github.com/lsx-xyg/CFTunnelKit/internal/applog"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed internal/assets/tray-icon.ico
var trayIcon []byte

func main() {
	if _, err := applog.Init(); err != nil {
		println("warn: applog init failed:", err.Error())
	}
	slog.Info("=== CFTunnelKit starting ===")

	app := NewApp()

	// start systray in a goroutine
	go systray.Run(onReady(app), onExit)

	err := wails.Run(&options.App{
		Title:  "CFTunnelKit",
		Width:  1024,
		Height: 768,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		HideWindowOnClose: true,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}

func onReady(app *App) func() {
	return func() {
		systray.SetIcon(trayIcon)
		systray.SetTitle("CFTunnelKit")
		systray.SetTooltip("CFTunnelKit — Cloudflare Tunnel 管理器")

		mShow := systray.AddMenuItem("显示主窗口", "Show window")
		mAuto := systray.AddMenuItemCheckbox("开机自启", "Auto start", isAutoStart())
		systray.AddSeparator()
		mQuit := systray.AddMenuItem("退出", "Quit")

		mShow.Click(func() {
			wailsruntime.WindowShow(app.ctx)
			wailsruntime.WindowUnminimise(app.ctx)
		})
		mAuto.Click(func() {
			enabled := !mAuto.Checked()
			setAutoStart(enabled)
			if enabled { mAuto.Check() } else { mAuto.Uncheck() }
		})
		mQuit.Click(func() {
			systray.Quit()
		})

		// left-click restores window
		systray.SetOnClick(func(menu systray.IMenu) {
			wailsruntime.WindowShow(app.ctx)
			wailsruntime.WindowUnminimise(app.ctx)
		})
	}
}

func onExit() {}

func startupDir() string {
	appdata := os.Getenv("APPDATA")
	return filepath.Join(appdata, "Microsoft", "Windows", "Start Menu", "Programs", "Startup")
}

func shortcutPath() string {
	return filepath.Join(startupDir(), "CFTunnelKit.lnk")
}

func isAutoStart() bool {
	_, err := os.Stat(shortcutPath())
	return err == nil
}

func setAutoStart(enable bool) {
	if !enable {
		_ = os.Remove(shortcutPath())
		return
	}
	exe, _ := os.Executable()
	// create .lnk via PowerShell
	ps := fmt.Sprintf(`$ws = New-Object -ComObject WScript.Shell; $s = $ws.CreateShortcut('%s'); $s.TargetPath = '%s'; $s.Save()`, shortcutPath(), exe)
	_ = exec.Command("powershell", "-Command", ps).Run()
}
