package main

import (
	"embed"
	"log/slog"

	"github.com/energye/systray"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"github.com/lsx-xyg/CFTunnelKit/internal/applog"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed internal/assets/tray-icon.png
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
		systray.AddSeparator()
		mQuit := systray.AddMenuItem("退出", "Quit")

		mShow.Click(func() {
			wailsruntime.WindowShow(app.ctx)
			wailsruntime.WindowUnminimise(app.ctx)
		})
		mQuit.Click(func() {
			systray.Quit()
		})
	}
}

func onExit() {}
