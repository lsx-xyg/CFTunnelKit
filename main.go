package main

import (
	"context"
	"embed"
	"log/slog"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"github.com/lsx-xyg/CFTunnelKit/internal/applog"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	if _, err := applog.Init(); err != nil {
		println("warn: applog init failed:", err.Error())
	}
	slog.Info("=== CFTunnelKit starting ===")

	app := NewApp()

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
		OnBeforeClose: func(ctx context.Context) (prevent bool) {
			// hide to tray instead of exiting; return true prevents close
			app.HideWindow()
			return true
		},
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
