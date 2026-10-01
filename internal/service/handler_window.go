package service

import (
	"context"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// WindowHandler owns window show/hide bindings (tray behavior).
type WindowHandler struct {
	ctx context.Context
}

func NewWindowHandler(ctx context.Context) *WindowHandler {
	return &WindowHandler{ctx: ctx}
}

// HideWindow hides the main window (tray behavior).
func (h *WindowHandler) HideWindow() {
	wailsruntime.WindowHide(h.ctx)
}

// ShowWindow restores the main window from tray.
func (h *WindowHandler) ShowWindow() {
	wailsruntime.WindowShow(h.ctx)
	wailsruntime.WindowUnminimise(h.ctx)
}
